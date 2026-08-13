package docker

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/arpingblue/AIStat/internal/collector"
	"github.com/arpingblue/AIStat/internal/execx"
	"github.com/arpingblue/AIStat/internal/fsx"
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
	if !ok || state.ClientState != model.StateAvailable || state.DaemonState != model.StateAvailable || !state.NVIDIARuntime || state.ToolkitDetected == nil || !*state.ToolkitDetected {
		t.Fatalf("unexpected state: %#v", state)
	}
}

type permissionRunner struct{}

func (permissionRunner) Run(_ context.Context, spec execx.CommandSpec) (execx.Result, error) {
	joined := spec.Name + " " + strings.Join(spec.Args, " ")
	if joined == "docker --version" {
		return execx.Result{Stdout: "Docker version 28.0.0"}, nil
	}
	if strings.HasPrefix(joined, "docker version") {
		return execx.Result{Stderr: "permission denied while connecting to /var/run/docker.sock"}, errors.New("exit status 1")
	}
	return execx.Result{}, errors.New("executable file not found")
}

func TestCollectSeparatesClientFromDeniedDaemon(t *testing.T) {
	result := Collector{}.Collect(context.Background(), collector.Env{Runner: permissionRunner{}, Platform: "linux"})
	state, ok := collector.DecodeFact[model.ContainerState](collector.Env{Facts: map[string]model.Fact{"containers": result.Facts[0]}}, "containers")
	if !ok || state.ClientState != model.StateAvailable || state.ClientVersion != "28.0.0" || state.DaemonState != model.StatePermissionDenied || result.State != model.StatePermissionDenied {
		t.Fatalf("unexpected state: result=%#v state=%#v", result, state)
	}
}

func TestEffectiveSetParser(t *testing.T) {
	if got := parseList("0-2,4"); len(got) != 4 || got[3] != 4 {
		t.Fatalf("unexpected cpuset: %v", got)
	}
}

type evidenceRunner struct {
	packageOutput string
}

func (r evidenceRunner) Run(_ context.Context, spec execx.CommandSpec) (execx.Result, error) {
	joined := spec.Name + " " + strings.Join(spec.Args, " ")
	switch {
	case spec.Name == "nvidia-ctk", spec.Name == "nvidia-container-cli", spec.Name == "nvidia-container-runtime":
		return execx.Result{}, errors.New("executable file not found")
	case spec.Name == "dpkg-query":
		if r.packageOutput != "" {
			return execx.Result{Stdout: r.packageOutput}, nil
		}
		return execx.Result{ExitCode: 1, Stderr: "no packages found"}, errors.New("exit status 1")
	case joined == "docker --version":
		return execx.Result{Stdout: "Docker version 29.5.2"}, nil
	case strings.HasPrefix(joined, "docker version"):
		return execx.Result{Stdout: `{"Client":{"Version":"29.5.2"},"Server":{"Version":"29.5.2"}}`}, nil
	case strings.HasPrefix(joined, "docker info"):
		return execx.Result{Stdout: `{"DefaultRuntime":"runc","Runtimes":{"io.containerd.runc.v2":{}}}`}, nil
	case strings.HasPrefix(joined, "docker ps"):
		return execx.Result{}, nil
	default:
		return execx.Result{}, errors.New("executable file not found")
	}
}

func TestToolkitAbsenceRequiresConclusiveIndependentEvidence(t *testing.T) {
	root := t.TempDir()
	result := Collector{}.Collect(context.Background(), collector.Env{Runner: evidenceRunner{}, FileSystem: fsx.Rooted{Root: root}, Platform: "linux"})
	state, ok := collector.DecodeFact[model.ContainerState](collector.Env{Facts: map[string]model.Fact{"containers": result.Facts[0]}}, "containers")
	if !ok || state.ToolkitPackageState != model.StateNotDetected || state.ToolkitCLIState != model.StateNotDetected || state.NVIDIARuntimeState != model.StateNotDetected || state.CDIState != model.StateNotDetected || state.ToolkitState != model.StateNotDetected || state.GPUContainerState != model.StateNotDetected {
		t.Fatalf("unexpected state: %#v", state)
	}
	if state.ToolkitDetected == nil || *state.ToolkitDetected {
		t.Fatalf("conclusive absence was not preserved: %#v", state.ToolkitDetected)
	}
}

func TestInstalledToolkitIsSeparatedFromDockerConfiguration(t *testing.T) {
	runner := evidenceRunner{packageOutput: "nvidia-container-toolkit\t1.18.0-1\tii \nlibnvidia-container1\t1.18.0-1\tii \n"}
	result := Collector{}.Collect(context.Background(), collector.Env{Runner: runner, FileSystem: fsx.Rooted{Root: t.TempDir()}, Platform: "linux"})
	state, _ := collector.DecodeFact[model.ContainerState](collector.Env{Facts: map[string]model.Fact{"containers": result.Facts[0]}}, "containers")
	if state.ToolkitState != model.StateAvailable || state.ToolkitPackageState != model.StateAvailable || state.GPUContainerState != model.StateNotDetected || len(state.ToolkitPackages) != 2 {
		t.Fatalf("installed and configured states were conflated: %#v", state)
	}
}

func TestNVIDIACDISpecProvidesDockerGPUIntegrationEvidence(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "etc", "cdi")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "nvidia.yaml"), []byte("kind: nvidia.com/gpu\ndevices: []\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	result := Collector{}.Collect(context.Background(), collector.Env{Runner: evidenceRunner{}, FileSystem: fsx.Rooted{Root: root}, Platform: "linux"})
	state, _ := collector.DecodeFact[model.ContainerState](collector.Env{Facts: map[string]model.Fact{"containers": result.Facts[0]}}, "containers")
	if state.CDIState != model.StateAvailable || state.GPUContainerState != model.StateAvailable || state.ToolkitState != model.StateAvailable || len(state.CDISpecs) != 1 {
		t.Fatalf("CDI integration was not recognized: %#v", state)
	}
}
