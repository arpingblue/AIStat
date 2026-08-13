package docker

import (
	"context"
	"strings"
	"testing"

	"github.com/arpingblue/AIStat/internal/collector"
	"github.com/arpingblue/AIStat/internal/execx"
	"github.com/arpingblue/AIStat/internal/model"
)

type fakeRunner struct{}

func (fakeRunner) Run(_ context.Context, spec execx.CommandSpec) (execx.Result, error) {
	joined := spec.Name + " " + strings.Join(spec.Args, " ")
	switch {
	case strings.HasPrefix(joined, "nvidia-ctk"):
		return execx.Result{Stdout: "NVIDIA Container Toolkit CLI version 1.17.8\n"}, nil
	case strings.Contains(joined, "docker version"):
		return execx.Result{Stdout: `{"Client":{"Version":"28.0.0"},"Server":{"Version":"28.0.0"}}`}, nil
	case strings.Contains(joined, "docker info"):
		return execx.Result{Stdout: `{"DefaultRuntime":"runc","Runtimes":{"nvidia":{"path":"nvidia-container-runtime"}}}`}, nil
	case strings.Contains(joined, "docker ps"):
		return execx.Result{}, nil
	}
	return execx.Result{}, nil
}

func TestNormalizeGPUContainer(t *testing.T) {
	var raw inspectResponse
	raw.ID = "0123456789abcdef"
	raw.Name = "/worker"
	raw.HostConfig.ShmSize = 64 << 20
	raw.HostConfig.Memory = 8 << 30
	raw.HostConfig.CpusetCpus = "0-3,8"
	raw.HostConfig.CpusetMems = "0"
	raw.HostConfig.DeviceRequests = append(raw.HostConfig.DeviceRequests, struct {
		Driver       string     `json:"Driver"`
		Count        int        `json:"Count"`
		DeviceIDs    []string   `json:"DeviceIDs"`
		Capabilities [][]string `json:"Capabilities"`
	}{Driver: "nvidia", Count: 1, DeviceIDs: []string{"GPU-0"}, Capabilities: [][]string{{"gpu"}}})
	got := normalize(raw)
	if got.ID != "0123456789ab" || !got.GPURequired || !got.GPUAccess || len(got.GPUUUIDs) != 1 || len(got.EffectiveCPUs) != 5 || got.SHMSize == nil {
		t.Fatalf("unexpected container: %#v", got)
	}
}

func TestCollectHealthyDaemon(t *testing.T) {
	result := Collector{}.Collect(context.Background(), collector.Env{Runner: fakeRunner{}, Platform: "linux"})
	if result.State != model.StateAvailable {
		t.Fatalf("unexpected result: %#v", result)
	}
	state, ok := collector.DecodeFact[model.ContainerState](collector.Env{Facts: map[string]model.Fact{"containers": result.Facts[0]}}, "containers")
	if !ok || state.DaemonState != model.StateAvailable || !state.NVIDIARuntime || state.ToolkitDetected == nil || !*state.ToolkitDetected {
		t.Fatalf("unexpected state: %#v", state)
	}
}

func TestEffectiveSetParser(t *testing.T) {
	if got := parseList("0-2,4"); len(got) != 4 || got[3] != 4 {
		t.Fatalf("unexpected cpuset: %v", got)
	}
}
