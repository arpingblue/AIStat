package process

import (
	"context"
	"errors"
	"io/fs"
	"path"
	"strconv"
	"strings"

	"github.com/arpingblue/AIStat/internal/collector"
	"github.com/arpingblue/AIStat/internal/collector/numa"
	"github.com/arpingblue/AIStat/internal/fsx"
	"github.com/arpingblue/AIStat/internal/model"
	"github.com/arpingblue/AIStat/internal/redact"
)

type Collector struct{}

func (Collector) ID() collector.ID                 { return "process" }
func (Collector) Provides() []collector.Capability { return []collector.Capability{"processes"} }
func (Collector) Requires() []collector.Capability { return []collector.Capability{"nvidia"} }

var allowedEnv = map[string]struct{}{
	"CUDA_VISIBLE_DEVICES": {}, "NVIDIA_VISIBLE_DEVICES": {},
	"NCCL_P2P_DISABLE": {}, "NCCL_P2P_LEVEL": {}, "NCCL_SHM_DISABLE": {},
	"NCCL_SOCKET_IFNAME": {}, "NCCL_IB_HCA": {}, "NCCL_NET": {},
	"NCCL_NET_GDR_LEVEL": {}, "NCCL_IGNORE_CPU_AFFINITY": {},
	"NCCL_DEBUG": {}, "NCCL_DEBUG_SUBSYS": {},
}
var allowedFlags = map[string]bool{
	"--tensor-parallel-size": true, "--pipeline-parallel-size": true,
	"--data-parallel-size": true, "--tp-size": true, "--dp-size": true,
	"--tp": true, "--pp": true, "--dp": true, "-tp": true, "-pp": true, "-dp": true,
	"--device-ids": true, "--numa-bind-cpus": true,
	"--distributed-executor-backend": true, "--nnodes": true, "--node-rank": true,
	"--base-gpu-id": true, "--gpu-id-step": true,
	"--model": true, "--model-path": true,
	"--disaggregation-ib-device": true, "--disaggregation-mode": true,
}

func (c Collector) Collect(_ context.Context, env collector.Env) collector.Result {
	if env.Platform != "linux" && !env.Fixture {
		return collector.Unsupported(c.ID(), "processes")
	}
	entries, err := env.FileSystem.ReadDir("/proc")
	if err != nil {
		return failure(c.ID(), err)
	}
	processes := []model.Process{}
	permissionDenied := false
	gpuProcesses := map[int][]string{}
	if stack, ok := collector.DecodeFact[model.NVIDIAStack](env, "nvidia"); ok {
		for _, item := range stack.ComputeProcesses {
			gpuProcesses[item.PID] = append(gpuProcesses[item.PID], item.GPUUUID)
		}
	}
	for _, entry := range entries {
		pid, err := strconv.Atoi(entry.Name())
		if err != nil || !entry.IsDir() {
			continue
		}
		base := "/proc/" + entry.Name()
		cmdRaw, err := env.FileSystem.ReadFile(base + "/cmdline")
		if err != nil {
			permissionDenied = permissionDenied || errors.Is(err, fs.ErrPermission) || strings.Contains(strings.ToLower(err.Error()), "not permitted")
			continue
		}
		args := splitNUL(cmdRaw)
		kind := detectRuntimeKind(args)
		if len(args) == 0 || kind == "" {
			continue
		}
		process := model.Process{PID: pid, RuntimeKind: kind, Executable: path.Base(args[0]), Command: strings.Join(redactArgs(args), " "), AllowedArgs: parseAllowedArgs(args), AllowedEnv: map[string]string{}}
		process.GPUUUIDs = append(process.GPUUUIDs, gpuProcesses[pid]...)
		if status, err := env.FileSystem.ReadFile(base + "/status"); err == nil {
			process.CPUSet = parseStatusList(string(status), "Cpus_allowed_list")
			process.NUMAMems = parseStatusList(string(status), "Mems_allowed_list")
		}
		if values, err := ReadAllowedEnv(env.FileSystem, pid, allowedEnv); err == nil {
			process.AllowedEnv = values
		}
		if raw, err := env.FileSystem.ReadFile(base + "/cgroup"); err == nil {
			process.ContainerID = shortID(containerID(string(raw)))
		}
		processes = append(processes, process)
	}
	state := model.StateAvailable
	if permissionDenied {
		state = model.StatePermissionDenied
	}
	value := model.ProcessState{State: state, Processes: processes}
	return collector.Result{Collector: c.ID(), State: state, Facts: []model.Fact{model.NewFact("processes", model.StateAvailable, value, model.ConfidenceMedium, model.SourceRef{Collector: "process", Source: "/proc/*/{cmdline,status,environ,cgroup}"})}}
}
func ReadAllowedEnv(fileSystem fsx.FileSystem, pid int, allowed map[string]struct{}) (map[string]string, error) {
	raw, err := fileSystem.ReadFile("/proc/" + strconv.Itoa(pid) + "/environ")
	if err != nil {
		return nil, err
	}
	result := map[string]string{}
	for _, item := range splitNUL(raw) {
		key, value, ok := strings.Cut(item, "=")
		if _, permitted := allowed[key]; ok && permitted {
			result[key] = value
		}
	}
	return result, nil
}
func shortID(value string) string {
	if len(value) > 12 {
		return value[:12]
	}
	return value
}
func splitNUL(raw []byte) []string {
	parts := strings.Split(strings.TrimRight(string(raw), "\x00"), "\x00")
	if len(parts) == 1 && parts[0] == "" {
		return nil
	}
	return parts
}
func detectRuntimeKind(args []string) string {
	for index, raw := range args {
		token := strings.ToLower(strings.TrimSpace(raw))
		base := strings.TrimSuffix(path.Base(token), ".py")
		switch {
		case base == "vllm" || base == "vllm.entrypoints" || strings.HasPrefix(token, "vllm.") || strings.Contains(token, "/vllm/"):
			return "vllm"
		case base == "sglang" || base == "sglang.launch_server" || strings.HasPrefix(token, "sglang.") || strings.Contains(token, "/sglang/"):
			return "sglang"
		case base == "torchrun" || token == "torch.distributed.run" || strings.HasSuffix(token, "/torch/distributed/run.py"):
			return "pytorch"
		case token == "-m" && index+1 < len(args):
			module := strings.ToLower(args[index+1])
			if module == "vllm" || strings.HasPrefix(module, "vllm.") {
				return "vllm"
			}
			if module == "sglang" || strings.HasPrefix(module, "sglang.") {
				return "sglang"
			}
			if module == "torch.distributed.run" {
				return "pytorch"
			}
		}
	}
	return ""
}
func redactArgs(args []string) []string {
	result := []string{}
	for i := 0; i < len(args); i++ {
		key, value, inline := strings.Cut(args[i], "=")
		if inline && allowedFlags[key] {
			result = append(result, key+"="+sanitizeFlagValue(key, value))
			continue
		}
		if allowedFlags[args[i]] {
			result = append(result, args[i])
			if i+1 < len(args) {
				result = append(result, sanitizeFlagValue(args[i], args[i+1]))
				i++
			}
		}
	}
	return result
}
func parseAllowedArgs(args []string) map[string]string {
	result := map[string]string{}
	for i := 0; i < len(args); i++ {
		key, value, ok := strings.Cut(args[i], "=")
		if ok && allowedFlags[key] {
			result[key] = sanitizeFlagValue(key, value)
			continue
		}
		if allowedFlags[args[i]] && i+1 < len(args) {
			result[args[i]] = sanitizeFlagValue(args[i], args[i+1])
			i++
		}
	}
	return result
}
func sanitizeFlagValue(key, value string) string {
	if key == "--model" || key == "--model-path" {
		return redact.ModelPath(value)
	}
	return value
}
func parseStatusList(raw, key string) []int {
	for _, line := range strings.Split(raw, "\n") {
		left, right, ok := strings.Cut(line, ":")
		if ok && strings.TrimSpace(left) == key {
			values, _ := numa.ParseList(strings.TrimSpace(right))
			return values
		}
	}
	return nil
}
func containerID(raw string) string {
	for _, line := range strings.Split(raw, "\n") {
		_, path, ok := strings.Cut(line, "::")
		if ok {
			parts := strings.FieldsFunc(path, func(r rune) bool { return r == '/' || r == '-' })
			for i := len(parts) - 1; i >= 0; i-- {
				if len(parts[i]) >= 12 {
					return strings.TrimSuffix(parts[i], ".scope")
				}
			}
		}
	}
	return ""
}
func failure(id collector.ID, err error) collector.Result {
	state := collector.FileErrorState(err)
	return collector.Result{Collector: id, State: state, Facts: []model.Fact{{Key: "processes", State: state, Confidence: model.ConfidenceLow}}, Diagnostics: []model.Diagnostic{{Level: "warn", Code: "proc_access", Message: err.Error()}}}
}
