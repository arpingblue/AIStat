package python

import (
	"context"
	"strings"
	"testing"

	"github.com/arpingblue/AIStat/internal/collector"
	"github.com/arpingblue/AIStat/internal/execx"
	"github.com/arpingblue/AIStat/internal/model"
)

type fakeRunner struct{}

func (fakeRunner) Run(context.Context, execx.CommandSpec) (execx.Result, error) {
	return execx.Result{Stdout: `{"python_version":"3.12.3","version":"2.8.0","cuda_available":true,"cuda_version":"12.8","gpu_count":2}`}, nil
}

func TestResolveVLLMProcess(t *testing.T) {
	processes := model.ProcessState{Processes: []model.Process{{PID: 42, Executable: "python", Command: "--tensor-parallel-size 2", CPUSet: []int{0, 1}, AllowedArgs: map[string]string{"--tensor-parallel-size": "2", "--device-ids": "1,0", "--nnodes": "2", "--node-rank": "0", "--distributed-executor-backend": "ray", "--numa-bind-cpus": "4-7", "--model": "/models/[REDACTED]"}, AllowedEnv: map[string]string{"CUDA_VISIBLE_DEVICES": "0", "NCCL_SOCKET_IFNAME": "eth0"}}}}
	processes.Processes[0].Command = "vllm " + processes.Processes[0].Command
	gpus := model.GPUState{Devices: []model.GPU{{Index: 0, UUID: "g0"}, {Index: 1, UUID: "g1"}}}
	got := resolveProcesses(processes, gpus, model.ContainerState{})
	if len(got) != 1 || got[0].Kind != "vllm" || got[0].LocalWorldSize == nil || *got[0].LocalWorldSize != 2 || len(got[0].GPUs) != 2 || got[0].GPUs[0] != "g1" || got[0].NNodes == nil || *got[0].NNodes != 2 || got[0].NodeRank == nil || *got[0].NodeRank != 0 || got[0].DistributedBackend != "ray" || len(got[0].NUMABindCPUs) != 4 || got[0].ModelPath != "/models/[REDACTED]" {
		t.Fatalf("unexpected runtime: %#v", got)
	}
}

func TestResolveSGLangSteppedGPUSelection(t *testing.T) {
	processes := model.ProcessState{Processes: []model.Process{{PID: 43, Executable: "python", Command: "sglang", AllowedArgs: map[string]string{"--tp-size": "2", "--base-gpu-id": "1", "--gpu-id-step": "2"}, AllowedEnv: map[string]string{}}}}
	gpus := model.GPUState{Devices: []model.GPU{{Index: 1, UUID: "g1"}, {Index: 3, UUID: "g3"}}}
	got := resolveProcesses(processes, gpus, model.ContainerState{})
	if len(got) != 1 || strings.Join(got[0].GPUDeviceRefs, ",") != "1,3" || strings.Join(got[0].GPUs, ",") != "g1,g3" {
		t.Fatalf("unexpected runtime: %#v", got)
	}
}

func TestCollectEnrichesRuntimeWithFixedProbe(t *testing.T) {
	processes := model.ProcessState{State: model.StateAvailable, Processes: []model.Process{{PID: 7, Executable: "python3", Command: "vllm", AllowedArgs: map[string]string{"--tensor-parallel-size": "2"}, AllowedEnv: map[string]string{}}}}
	gpus := model.GPUState{State: model.StateAvailable, Devices: []model.GPU{{Index: 0, UUID: "g0"}, {Index: 1, UUID: "g1"}}}
	containers := model.ContainerState{State: model.StateAvailable}
	facts := map[string]model.Fact{"processes": model.NewFact("processes", model.StateAvailable, processes, model.ConfidenceHigh), "gpus": model.NewFact("gpus", model.StateAvailable, gpus, model.ConfidenceHigh), "containers": model.NewFact("containers", model.StateAvailable, containers, model.ConfidenceHigh)}
	result := Collector{}.Collect(context.Background(), collector.Env{Runner: fakeRunner{}, Facts: facts, Platform: "linux"})
	state, ok := collector.DecodeFact[model.RuntimeState](collector.Env{Facts: map[string]model.Fact{"runtimes": result.Facts[0]}}, "runtimes")
	if !ok || len(state.Instances) != 1 || state.Instances[0].CUDAAvailable == nil || !*state.Instances[0].CUDAAvailable || state.Instances[0].GPUCount == nil || *state.Instances[0].GPUCount != 2 {
		t.Fatalf("unexpected runtime: %#v", state)
	}
}
