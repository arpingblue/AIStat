package python

import (
	"context"
	"encoding/json"
	"path"
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
	processes, processOK := collector.DecodeFact[model.ProcessState](env, "processes")
	if !processOK {
		processes.State = sourceState(env, "processes")
	}
	gpus, gpuOK := collector.DecodeFact[model.GPUState](env, "gpus")
	if !gpuOK {
		gpus.State = sourceState(env, "gpus")
	}
	containers, containerOK := collector.DecodeFact[model.ContainerState](env, "containers")
	if !containerOK {
		containers.State, containers.DaemonState = sourceState(env, "containers"), sourceState(env, "containers")
	}
	instances := resolveProcesses(processes, gpus, containers)
	scan := discoverInstallations(ctx, env.FileSystem, env.HomeDir, env.Environment, processes, containers)
	for i := range instances {
		if !strings.HasPrefix(instances[i].Executable, "python") {
			continue
		}
		name := instances[i].Executable
		if env.FileSystem != nil && instances[i].PID > 0 {
			name = "/proc/" + strconv.Itoa(instances[i].PID) + "/exe"
			target, err := env.FileSystem.Readlink(name)
			if err != nil {
				continue
			}
			instances[i].PythonEnvironment = path.Dir(path.Dir(target))
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
		instances[i].PyTorchVersion = parsed.Version
		instances[i].PythonVersion = parsed.PythonVersion
		instances[i].CUDAAvailable = parsed.CUDAAvailable
		instances[i].CUDAVersion = parsed.CUDAVersion
		instances[i].GPUCount = parsed.GPUCount
	}
	applyRuntimeVersions(instances, scan.Installations)
	products := buildProducts(instances, scan, processes, containers)
	state := model.StateAvailable
	if len(instances) == 0 && len(scan.Installations) == 0 {
		switch {
		case scan.HostState == model.StatePermissionDenied || scan.ContainerState == model.StatePermissionDenied || processes.State == model.StatePermissionDenied:
			state = model.StatePermissionDenied
		case scan.HostState == model.StateUnknown || scan.ContainerState == model.StateUnknown:
			state = model.StateUnknown
		default:
			state = model.StateNotDetected
		}
	}
	value := model.RuntimeState{State: state, Instances: instances, Products: products}
	fact := model.NewFact("runtimes", model.StateAvailable, value, model.ConfidenceMedium, model.SourceRef{Collector: "runtime", Source: "sanitized process facts, bounded package metadata scan, and fixed Python probe"})
	return collector.Result{Collector: c.ID(), State: state, Facts: []model.Fact{fact}}
}

func sourceState(env collector.Env, key string) model.FactState {
	if fact, ok := env.Facts[key]; ok && fact.State != "" {
		return fact.State
	}
	return model.StateUnknown
}

func buildProducts(instances []model.RuntimeInstance, scan installationScan, processes model.ProcessState, containers model.ContainerState) []model.RuntimeProduct {
	products := make([]model.RuntimeProduct, 0, 3)
	for _, name := range []string{"pytorch", "vllm", "sglang"} {
		product := model.RuntimeProduct{Name: name, HostState: scan.HostState, ContainerState: scan.ContainerState, ExecutionState: model.StateNotDetected}
		for _, installation := range scan.Installations {
			if installation.Product != name {
				continue
			}
			product.Installations = append(product.Installations, installation)
			if installation.Scope == "container" {
				if product.ContainerState == model.StateNotDetected {
					product.ContainerState = model.StateAvailable
				}
			} else {
				if product.HostState == model.StateNotDetected {
					product.HostState = model.StateAvailable
				}
			}
		}
		for _, instance := range instances {
			if instance.Kind == name {
				product.InstanceCount++
				product.ExecutionState = model.StateAvailable
			}
		}
		if product.InstanceCount > 0 && len(product.Installations) == 0 {
			for _, instance := range instances {
				if instance.Kind != name {
					continue
				}
				scope, location := "host", instance.PythonEnvironment
				if location == "" {
					location = instance.Executable
				}
				if instance.ContainerID != "" {
					scope = "container"
				}
				product.Installations = append(product.Installations, model.RuntimeInstallation{Product: name, Version: instance.Version, Path: location, PythonEnvironment: instance.PythonEnvironment, Scope: scope, ContainerID: instance.ContainerID, Source: "active runtime process", Confidence: model.ConfidenceMedium})
				if scope == "host" && product.HostState == model.StateNotDetected {
					product.HostState = model.StateAvailable
				}
				if scope == "container" && product.ContainerState == model.StateNotDetected {
					product.ContainerState = model.StateAvailable
				}
				break
			}
		}
		if len(product.Installations) > 0 || product.InstanceCount > 0 {
			product.InstallationState = model.StateAvailable
			product.InstallationReason = strconv.Itoa(len(product.Installations)) + " installation record(s) confirmed"
		} else if product.HostState == model.StatePermissionDenied || product.ContainerState == model.StatePermissionDenied {
			product.InstallationState = model.StatePermissionDenied
			product.InstallationReason = joinReasons(scan.HostReason, scan.ContainerReason)
		} else if product.HostState == model.StateUnknown || product.ContainerState == model.StateUnknown {
			product.InstallationState = model.StateUnknown
			product.InstallationReason = joinReasons(scan.HostReason, scan.ContainerReason)
		} else {
			product.InstallationState = model.StateNotDetected
			product.InstallationReason = "selected host and running-container package paths were fully inspected; no matching metadata or active instance was found"
		}
		containerVisibilityUnknown := containers.DaemonState == model.StatePermissionDenied || containers.DaemonState == model.StateUnknown || containers.DaemonState == model.StateTimeout || containers.DaemonState == model.StateParseError
		processVisibilityUnknown := processes.State == model.StatePermissionDenied || processes.State == model.StateUnknown || processes.State == model.StateTimeout || processes.State == model.StateParseError
		if product.ExecutionState == model.StateNotDetected && (containerVisibilityUnknown || processVisibilityUnknown) {
			product.ExecutionState = model.StateUnknown
			product.ExecutionReason = "process or running-container visibility is incomplete; absence was not inferred"
		} else if product.ExecutionState == model.StateNotDetected {
			product.ExecutionReason = "process and running-container inventory was inspected; no active instance was found"
		} else {
			product.ExecutionReason = strconv.Itoa(product.InstanceCount) + " active instance(s) confirmed"
		}
		products = append(products, product)
	}
	return products
}

func joinReasons(values ...string) string {
	result := []string{}
	seen := map[string]bool{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" && !seen[value] {
			seen[value] = true
			result = append(result, value)
		}
	}
	return strings.Join(result, "; ")
}

func applyRuntimeVersions(instances []model.RuntimeInstance, installations []model.RuntimeInstallation) {
	for index := range instances {
		matches := []model.RuntimeInstallation{}
		for _, installation := range installations {
			if installation.Product != instances[index].Kind {
				continue
			}
			if instances[index].ContainerID != "" {
				if installation.ContainerID == instances[index].ContainerID || strings.HasPrefix(installation.ContainerID, instances[index].ContainerID) || strings.HasPrefix(instances[index].ContainerID, installation.ContainerID) {
					matches = append(matches, installation)
				}
			} else if installation.Scope == "host" {
				matches = append(matches, installation)
			}
		}
		for _, installation := range matches {
			if instances[index].PythonEnvironment != "" && installation.PythonEnvironment == instances[index].PythonEnvironment {
				instances[index].Version = installation.Version
				break
			}
		}
		if instances[index].Version == "" && len(matches) == 1 {
			instances[index].Version = matches[0].Version
		}
		if instances[index].Kind == "pytorch" && instances[index].Version == "" {
			instances[index].Version = instances[index].PyTorchVersion
		}
	}
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
		kind := process.RuntimeKind
		if kind == "" {
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
