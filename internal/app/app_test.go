package app

import (
	"bytes"
	"encoding/json"
	"runtime"
	"strings"
	"testing"

	"github.com/arpingblue/AIStat/internal/model"
	"github.com/arpingblue/AIStat/internal/report"
	"github.com/arpingblue/AIStat/internal/topology"
)

func TestVersionJSON(t *testing.T) {
	var out, errOut bytes.Buffer
	if code := Run([]string{"version", "--format", "json"}, &out, &errOut); code != 0 {
		t.Fatalf("code=%d stderr=%s", code, errOut.String())
	}
	var parsed map[string]string
	if err := json.Unmarshal(out.Bytes(), &parsed); err != nil {
		t.Fatal(err)
	}
	if parsed["version"] == "" {
		t.Fatal("missing version")
	}
}
func TestCheckJSONPortable(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("portable unsupported-state behavior is Windows-specific")
	}
	var out, errOut bytes.Buffer
	code := Run([]string{"check", "--format", "json", "--profile", "general"}, &out, &errOut)
	if code != 0 {
		t.Fatalf("code=%d stderr=%s", code, errOut.String())
	}
	var parsed map[string]any
	if err := json.Unmarshal(out.Bytes(), &parsed); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, out.String())
	}
	if parsed["schema_version"] != "0.1" {
		t.Fatalf("bad schema: %v", parsed["schema_version"])
	}
}
func TestUnknownCommand(t *testing.T) {
	var out, errOut bytes.Buffer
	if code := Run([]string{"wat"}, &out, &errOut); code != 2 || !strings.Contains(errOut.String(), "unknown command") {
		t.Fatalf("code/output: %d %s", code, errOut.String())
	}
}

func TestHelpDescribesDeploymentAndOptimizationMission(t *testing.T) {
	var out, errOut bytes.Buffer
	if code := Run([]string{"help"}, &out, &errOut); code != 0 {
		t.Fatalf("code=%d stderr=%s", code, errOut.String())
	}
	for _, expected := range []string{
		"large-model deployment and high-performance inference",
		"deployment blockers and performance bottlenecks",
		"verifiable optimization guidance",
	} {
		if !strings.Contains(out.String(), expected) {
			t.Fatalf("help is missing %q:\n%s", expected, out.String())
		}
	}
}

func TestDefaultCommandIsStatus(t *testing.T) {
	var out, errOut bytes.Buffer
	if code := Run(nil, &out, &errOut); code > 1 {
		t.Fatalf("code=%d stderr=%s", code, errOut.String())
	}
	if !strings.Contains(out.String(), "Node Status") {
		t.Fatalf("default output is not status: %s", out.String())
	}
}

func TestCustomOptionErrorsArePrinted(t *testing.T) {
	var out, errOut bytes.Buffer
	if code := Run([]string{"check", "--timeout", "0s"}, &out, &errOut); code != 2 || !strings.Contains(errOut.String(), "timeout must be positive") {
		t.Fatalf("code=%d stderr=%q", code, errOut.String())
	}
	errOut.Reset()
	if code := Run([]string{"topology", "--view", "wat"}, &out, &errOut); code != 2 || !strings.Contains(errOut.String(), "view must be") {
		t.Fatalf("code=%d stderr=%q", code, errOut.String())
	}
}

func TestNoColorDisablesANSI(t *testing.T) {
	var out, errOut bytes.Buffer
	code := Run([]string{"status", "--profile", "general", "--no-color"}, &out, &errOut)
	if code > 1 {
		t.Fatalf("code=%d stderr=%s", code, errOut.String())
	}
	if strings.Contains(out.String(), "\x1b[") {
		t.Fatal("--no-color output contains ANSI")
	}
}

func TestColorPolicy(t *testing.T) {
	if !colorEnabledFor(true, false, report.FormatHuman, "") {
		t.Fatal("TTY human output should use color")
	}
	for _, test := range []struct {
		terminal, disabled bool
		format             report.Format
		noColor            string
	}{{false, false, report.FormatHuman, ""}, {true, true, report.FormatHuman, ""}, {true, false, report.FormatJSON, ""}, {true, false, report.FormatHuman, "1"}} {
		if colorEnabledFor(test.terminal, test.disabled, test.format, test.noColor) {
			t.Fatalf("unexpected color for %#v", test)
		}
	}
}

func TestResolveProfileDoesNotRequireNVIDIAToolkitForUnrelatedContainers(t *testing.T) {
	profile := model.Profile{Name: "llm-inference", GPURequired: true}
	snapshot := &model.Snapshot{Containers: model.ContainerState{Devices: []model.Container{{ID: "web", Runtime: "runc"}}}}
	resolved := resolveProfile(profile, snapshot)
	if resolved.DockerRequired {
		t.Fatal("an unrelated runc container must not turn Docker GPU support into a deployment requirement")
	}
	snapshot.Containers.Devices = append(snapshot.Containers.Devices, model.Container{ID: "gpu", Runtime: "runc", GPURequired: true})
	resolved = resolveProfile(profile, snapshot)
	if !resolved.DockerRequired {
		t.Fatal("a GPU-requesting container must require Docker GPU support")
	}
}

func TestTopologyHumanIsCompactTreeAndMatrix(t *testing.T) {
	numa := 0
	node := &model.Snapshot{Meta: model.Meta{Hostname: "node"}, NUMA: model.NUMAState{Nodes: []model.NUMANode{{ID: 0, CPUList: []int{0, 1, 2, 3}}}}, GPUs: model.GPUState{Devices: []model.GPU{{Index: 0, UUID: "g0", Name: "L20", PCIAddress: "0000:01:00.0", NUMANode: &numa}, {Index: 1, UUID: "g1", Name: "L20", PCIAddress: "0000:02:00.0", NUMANode: &numa}}}, Network: model.NetworkState{NICs: []model.NIC{{Name: "eth0", PCIAddress: "0000:03:00.0", NUMANode: &numa}, {Name: "veth0"}}}, P2P: []model.P2PLink{{FromGPU: "g0", ToGPU: "g1", Kind: "NODE"}}, Processes: model.ProcessState{Processes: []model.Process{{PID: 42, CPUSet: []int{0, 1, 2, 3}}}}}
	var out bytes.Buffer
	if err := writeTopology(&out, report.FormatHuman, model.Report{Node: node}, topology.New(), "tree", report.Options{}); err != nil {
		t.Fatal(err)
	}
	text := out.String()
	for _, expected := range []string{"NUMA 0", "CPUs: 0-3", "GPU 0", "NIC eth0", "GPU P2P", "NODE"} {
		if !strings.Contains(text, expected) {
			t.Fatalf("missing %q in %s", expected, text)
		}
	}
	for _, unwanted := range []string{"process:42", "cpu:0", "veth0"} {
		if strings.Contains(text, unwanted) {
			t.Fatalf("unexpected %q in %s", unwanted, text)
		}
	}
	out.Reset()
	if err := writeTopology(&out, report.FormatHuman, model.Report{Node: node}, topology.New(), "gpu", report.Options{}); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out.String(), "CPUs:") || strings.Contains(out.String(), "NIC eth0") {
		t.Fatalf("gpu view leaked CPU/NIC details: %s", out.String())
	}
	out.Reset()
	if err := writeTopology(&out, report.FormatHuman, model.Report{Node: node}, topology.New(), "gpu-nic", report.Options{}); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out.String(), "CPUs:") || !strings.Contains(out.String(), "NIC eth0") {
		t.Fatalf("gpu-nic view filtering failed: %s", out.String())
	}
}
