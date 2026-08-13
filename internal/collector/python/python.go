package python

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"
	"time"

	"github.com/arpingblue/AIStat/internal/collector"
	"github.com/arpingblue/AIStat/internal/collector/numa"
	"github.com/arpingblue/AIStat/internal/execx"
	"github.com/arpingblue/AIStat/internal/model"
)

type Collector struct{}

func (Collector) ID() collector.ID                 { return "runtime" }
func (Collector) Provides() []collector.Capability { return []collector.Capability{"runtimes"} }
func (Collector) Requires() []collector.Capability {
	return []collector.Capability{"processes", "gpus", "containers"}
}

const probe = `import json,sys
r={"python_version":sys.version.split()[0]}
try:
 import torch
 r.update({"version":torch.__version__,"cuda_available":bool(torch.cuda.is_available()),"cuda_version":torch.version.cuda,"gpu_count":int(torch.cuda.device_count())})
except Exception as e:
 r["probe_error"]=type(e).__name__
print(json.dumps(r,separators=(",",":")))`

type probeResult struct {
	PythonVersion string `json:"python_version"`
	Version       string `json:"version"`
	CUDAAvailable *bool  `json:"cuda_available"`
	CUDAVersion   string `json:"cuda_version"`
	GPUCount      *int   `json:"gpu_count"`
	ProbeError    string `json:"probe_error"`
}

func (c Collector) Collect(ctx context.Context, env collector.Env) collector.Result {
	if env.Platform != "linux" && !env.Fixture {
		return collector.Unsupported(c.ID(), "runtimes")
	}
	processes, _ := collector.DecodeFact[model.ProcessState](env, "processes")
	gpus, _ := collector.DecodeFact[model.GPUState](env, "gpus")
	containers, _ := collector.DecodeFact[model.ContainerState](env, "containers")
	instances := resolveProcesses(processes, gpus, containers)
	for i := range instances {
		if !strings.HasPrefix(instances[i].Executable, "python") {
			continue
		}
		name := instances[i].Executable
		if env.FileSystem != nil && instances[i].PID > 0 {
			name = "/proc/" + strconv.Itoa(instances[i].PID) + "/exe"
			if _, err := env.FileSystem.Readlink(name); err != nil {
				continue
			}
		}
		dir := ""
		if env.Platform == "linux" {
			dir = "/"
		}
		result, err := env.Runner.Run(ctx, execx.CommandSpec{Name: name, Args: []string{"-I", "-c", probe}, Env: probeEnvironment(instances[i]), Dir: dir, Timeout: 5 * time.Second, OutputLimit: 64 << 10})
		if err != nil {
			continue
		}
		var parsed probeResult
		if json.Unmarshal([]byte(result.Stdout), &parsed) != nil || parsed.ProbeError != "" {
			continue
		}
		instances[i].Version = parsed.Version
		instances[i].PythonVersion = parsed.PythonVersion
		instances[i].CUDAAvailable = parsed.CUDAAvailable
		instances[i].CUDAVersion = parsed.CUDAVersion
		instances[i].GPUCount = parsed.GPUCount
	}
	state := model.StateAvailable
	if len(instances) == 0 {
		state = model.StateNotDetected
	}
	value := model.RuntimeState{State: state, Instances: instances}
	fact := model.Fact{Key: "runtimes", State: state, Confidence: model.ConfidenceMedium, Sources: []model.SourceRef{{Collector: "runtime", Source: "sanitized process facts and fixed Python probe"}}}
	if state == model.StateAvailable {
		fact = model.NewFact("runtimes", model.StateAvailable, value, model.ConfidenceMedium, fact.Sources...)
	}
	return collector.Result{Collector: c.ID(), State: state, Facts: []model.Fact{fact}}
}
func probeEnvironment(runtime model.RuntimeInstance) []string {
	result := []string{}
	if value := runtime.Details["CUDA_VISIBLE_DEVICES"]; value != "" {
		result = append(result, "CUDA_VISIBLE_DEVICES="+value)
	}
	return result
}
func resolveProcesses(processes model.ProcessState, gpus model.GPUState, containers model.ContainerState) []model.RuntimeInstance {
	out := []model.RuntimeInstance{}
	for _, process := range processes.Processes {
		joined := strings.ToLower(process.Executable + " " + process.Command)
		kind := ""
		if strings.Contains(joined, "vllm") {
			kind = "vllm"
		} else if strings.Contains(joined, "sglang") {
			kind = "sglang"
		} else if strings.Contains(joined, "torchrun") {
			kind = "pytorch"
		} else {
			continue
		}
		instance := model.RuntimeInstance{Kind: kind, PID: process.PID, ContainerID: process.ContainerID, Executable: process.Executable, CPUSet: process.CPUSet, NUMAMems: process.NUMAMems, Details: map[string]string{}}
		for key, value := range process.AllowedEnv {
			instance.Details[key] = value
		}
		for key, value := range process.AllowedArgs {
			instance.Details[key] = value
		}
		if process.ContainerID != "" {
			for _, container := range containers.Devices {
				if container.ID == process.ContainerID || strings.HasPrefix(container.ID, process.ContainerID) || strings.HasPrefix(process.ContainerID, container.ID) {
					if len(container.GPUUUIDs) > 0 && process.AllowedEnv["CUDA_VISIBLE_DEVICES"] == "" {
						instance.GPUs = append([]string(nil), container.GPUUUIDs...)
					}
					if len(instance.CPUSet) == 0 {
						instance.CPUSet = append([]int(nil), container.EffectiveCPUs...)
					}
					if len(instance.NUMAMems) == 0 {
						instance.NUMAMems = append([]int(nil), container.EffectiveMems...)
					}
					break
				}
			}
		}
		instance.TensorParallel = parseInt(process.AllowedArgs, "--tensor-parallel-size", "--tp-size", "--tp", "-tp")
		instance.PipelineParallel = parseInt(process.AllowedArgs, "--pipeline-parallel-size", "--pp", "-pp")
		instance.DataParallel = parseInt(process.AllowedArgs, "--data-parallel-size", "--dp-size", "--dp", "-dp")
		instance.NNodes = parseInt(process.AllowedArgs, "--nnodes")
		instance.NodeRank = parseNonNegativeInt(process.AllowedArgs, "--node-rank")
		instance.DistributedBackend = process.AllowedArgs["--distributed-executor-backend"]
		instance.NUMABindCPUs, _ = numa.ParseList(process.AllowedArgs["--numa-bind-cpus"])
		instance.ModelPath = firstNonEmpty(process.AllowedArgs["--model"], process.AllowedArgs["--model-path"])
		tp, pp, dp := one(instance.TensorParallel), one(instance.PipelineParallel), one(instance.DataParallel)
		world := tp * pp * dp
		instance.LocalWorldSize = &world
		selection := firstNonEmpty(process.AllowedArgs["--device-ids"], process.AllowedEnv["CUDA_VISIBLE_DEVICES"], process.AllowedEnv["NVIDIA_VISIBLE_DEVICES"])
		if selection == "" {
			selection = sglangGPUSelection(process.AllowedArgs, world)
		}
		instance.GPUDeviceRefs = split(selection)
		instance.GPUs = resolveGPUs(selection, gpus)
		instance.Disaggregation = process.AllowedArgs["--disaggregation-mode"] != ""
		if raw := process.AllowedArgs["--disaggregation-ib-device"]; raw != "" {
			instance.SelectedHCAs = split(raw)
		}
		instance.SelectedNICs = split(process.AllowedEnv["NCCL_SOCKET_IFNAME"])
		instance.SelectedHCAs = append(instance.SelectedHCAs, split(process.AllowedEnv["NCCL_IB_HCA"])...)
		out = append(out, instance)
	}
	return out
}
func resolveGPUs(raw string, gpus model.GPUState) []string {
	if raw == "" || strings.EqualFold(strings.TrimSpace(raw), "all") {
		result := []string{}
		for _, gpu := range gpus.Devices {
			id := gpu.UUID
			if id == "" {
				id = strconv.Itoa(gpu.Index)
			}
			result = append(result, id)
		}
		return result
	}
	result := []string{}
	for _, ref := range split(raw) {
		for _, gpu := range gpus.Devices {
			if ref == gpu.UUID || ref == strconv.Itoa(gpu.Index) {
				id := gpu.UUID
				if id == "" {
					id = strconv.Itoa(gpu.Index)
				}
				result = append(result, id)
				break
			}
		}
	}
	return result
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func sglangGPUSelection(values map[string]string, count int) string {
	base := parseNonNegativeInt(values, "--base-gpu-id")
	if base == nil {
		return ""
	}
	step := one(parseInt(values, "--gpu-id-step"))
	refs := make([]string, 0, count)
	for i := 0; i < count; i++ {
		refs = append(refs, strconv.Itoa(*base+i*step))
	}
	return strings.Join(refs, ",")
}
func parseInt(values map[string]string, keys ...string) *int {
	for _, key := range keys {
		if raw := values[key]; raw != "" {
			value, err := strconv.Atoi(raw)
			if err == nil && value > 0 {
				return &value
			}
		}
	}
	return nil
}
func parseNonNegativeInt(values map[string]string, keys ...string) *int {
	for _, key := range keys {
		if raw := values[key]; raw != "" {
			value, err := strconv.Atoi(raw)
			if err == nil && value >= 0 {
				return &value
			}
		}
	}
	return nil
}
func one(value *int) int {
	if value == nil {
		return 1
	}
	return *value
}
func split(raw string) []string {
	parts := strings.Split(raw, ",")
	result := []string{}
	for _, part := range parts {
		part = strings.TrimSpace(strings.TrimLeft(part, "=^"))
		if part != "" {
			result = append(result, part)
		}
	}
	return result
}
