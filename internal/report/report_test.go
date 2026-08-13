package report

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/arpingblue/AIStat/internal/model"
)

func validReport() model.Report {
	s := &model.Snapshot{Meta: model.Meta{SchemaVersion: "0.1", ToolVersion: "test", CollectedAt: time.Unix(1, 0).UTC(), OS: "linux", Arch: "amd64"}, Host: model.Host{State: model.StateAvailable}, CPU: model.CPU{State: model.StateAvailable}, Memory: model.Memory{State: model.StateAvailable}, NUMA: model.NUMAState{State: model.StateAvailable}, PCI: model.PCIState{State: model.StateAvailable}, GPUs: model.GPUState{State: model.StateAvailable}, Network: model.NetworkState{State: model.StateAvailable}, RDMA: model.RDMAState{State: model.StateAvailable}, Storage: model.StorageState{State: model.StateAvailable}, NVIDIA: model.NVIDIAStack{State: model.StateAvailable, XIDState: model.StateAvailable}, Containers: model.ContainerState{State: model.StateAvailable, DaemonState: model.StateNotDetected}, Processes: model.ProcessState{State: model.StateAvailable}, Runtimes: model.RuntimeState{State: model.StateAvailable}}
	return model.Report{SchemaVersion: "0.1", AIStatVersion: "test", CollectedAt: time.Unix(1, 0).UTC(), Profile: "general", CompatibilityVersion: "test", Readiness: model.Readiness{Deployment: "ready", Performance: "ready"}, Node: s, Findings: []model.Finding{}}
}

func testFinding(id string, status model.Status, current string) model.Finding {
	return model.Finding{RuleID: id, Title: "test", Domain: "test", Status: status, Severity: model.SeverityHigh, Dimension: model.DimensionDeployment, Priority: model.PriorityP0, Subject: model.Subject{Kind: "node"}, CurrentState: current, Why: "test", Recommendation: "test", Confidence: model.ConfidenceHigh, Evidence: []model.Evidence{{Fact: "test.fact", Value: true, Source: "test"}}, Verification: []string{"test"}, References: []model.Reference{{Title: "test", URL: "https://example.com"}}}
}

func TestWriteJSONUsesLowercaseEnums(t *testing.T) {
	value := validReport()
	value.Readiness.Deployment = "unknown"
	value.Findings = []model.Finding{testFinding("TEST001", model.StatusUnknown, "unknown")}
	var buffer bytes.Buffer
	if err := Write(&buffer, FormatJSON, value); err != nil {
		t.Fatal(err)
	}
	var parsed map[string]any
	if err := json.Unmarshal(buffer.Bytes(), &parsed); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(buffer.String(), "UNKNOWN") {
		t.Fatal("JSON leaked uppercase enum")
	}
}
func TestWriteHumanShowsActionableOnly(t *testing.T) {
	value := validReport()
	value.Readiness.Deployment = "not_ready"
	value.Summary.Fail = 1
	fail := testFinding("TEST001", model.StatusFail, "broken")
	fail.Title = "bad"
	pass := testFinding("TEST002", model.StatusPass, "good")
	pass.Title = "good"
	value.Findings = []model.Finding{fail, pass}
	var buffer bytes.Buffer
	if err := Write(&buffer, FormatHuman, value); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buffer.String(), "FAIL TEST001") || strings.Contains(buffer.String(), "PASS TEST002") {
		t.Fatalf("unexpected human output: %s", buffer.String())
	}
}

func TestHumanGolden(t *testing.T) {
	value := validReport()
	value.Readiness.Deployment = "not_ready"
	value.Summary.Fail = 1
	fail := testFinding("TEST001", model.StatusFail, "broken")
	fail.Title = "bad"
	value.Findings = []model.Finding{fail}
	var buffer bytes.Buffer
	if err := Write(&buffer, FormatHuman, value); err != nil {
		t.Fatal(err)
	}
	want, err := os.ReadFile(filepath.Join("..", "..", "testdata", "golden", "check-broken.txt"))
	if err != nil {
		t.Fatal(err)
	}
	expected := strings.ReplaceAll(string(want), "\r\n", "\n")
	if buffer.String() != expected {
		t.Fatalf("golden mismatch\n--- got ---\n%s\n--- want ---\n%s", buffer.String(), want)
	}
}

func TestJSONGolden(t *testing.T) {
	var buffer bytes.Buffer
	if err := Write(&buffer, FormatJSON, validReport()); err != nil {
		t.Fatal(err)
	}
	want, err := os.ReadFile(filepath.Join("..", "..", "testdata", "golden", "check-valid.json"))
	if err != nil {
		t.Fatalf("read golden: %v\n--- got ---\n%s", err, buffer.String())
	}
	expected := strings.ReplaceAll(string(want), "\r\n", "\n")
	if buffer.String() != expected {
		t.Fatalf("golden mismatch\n--- got ---\n%s\n--- want ---\n%s", buffer.String(), want)
	}
}

func TestHumanColorCanBeEnabledWithoutLeakingIntoJSON(t *testing.T) {
	value := validReport()
	value.Readiness.Deployment = "unknown"
	value.Summary.Unknown = 1
	value.Findings = []model.Finding{testFinding("TEST001", model.StatusUnknown, "permission denied")}
	var human bytes.Buffer
	if err := WriteWithOptions(&human, FormatHuman, value, Options{Color: true}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(human.String(), "\x1b[91mUNKNOWN\x1b[0m") {
		t.Fatalf("missing status color: %q", human.String())
	}
	var plain bytes.Buffer
	if err := WriteWithOptions(&plain, FormatHuman, value, Options{}); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(plain.String(), "\x1b[") {
		t.Fatal("plain output contains ANSI")
	}
	var jsonOutput bytes.Buffer
	if err := WriteWithOptions(&jsonOutput, FormatJSON, value, Options{Color: true}); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(jsonOutput.String(), "\x1b[") {
		t.Fatal("JSON contains ANSI")
	}
}

func TestStatusJSONMatchesCheckJSON(t *testing.T) {
	value := validReport()
	var check, status bytes.Buffer
	if err := Write(&check, FormatJSON, value); err != nil {
		t.Fatal(err)
	}
	if err := WriteStatus(&status, FormatJSON, value, Options{}); err != nil {
		t.Fatal(err)
	}
	if check.String() != status.String() {
		t.Fatalf("status JSON differs from check JSON")
	}
}

func TestCompressCPUSet(t *testing.T) {
	if got := CompressCPUSet([]int{3, 2, 1, 8, 10, 9, 3}); got != "1-3,8-10" {
		t.Fatalf("got %q", got)
	}
}

func TestOperatorViewsExposePermissionAndRuntimePartitions(t *testing.T) {
	value := validReport()
	value.Node.Meta.Hostname = "gpu-node"
	value.Node.Containers = model.ContainerState{State: model.StatePermissionDenied, ClientState: model.StateAvailable, DaemonState: model.StatePermissionDenied, Engine: "docker", ClientVersion: "28.0.0"}
	value.Node.Runtimes = model.RuntimeState{State: model.StateAvailable, Products: []model.RuntimeProduct{{Name: "vllm", InstallationState: model.StateAvailable, ExecutionState: model.StateAvailable, HostState: model.StateNotDetected, ContainerState: model.StatePermissionDenied, InstanceCount: 1, Installations: []model.RuntimeInstallation{{Product: "vllm", Version: "0.9.0", Path: "/opt/conda/lib/python3.12/site-packages/vllm.dist-info", Scope: "container", ContainerID: "abc123", Source: "python package metadata", Confidence: model.ConfidenceHigh}}}}, Instances: []model.RuntimeInstance{{Kind: "vllm", Version: "0.9.0", PyTorchVersion: "2.8.0", PID: 42, GPUs: []string{"GPU-0"}, CPUSet: []int{0, 1, 2, 3}}}}
	var status bytes.Buffer
	if err := WriteStatus(&status, FormatHuman, value, Options{}); err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{"Hardware", "NVIDIA Stack", "Containers", "AI Runtimes", "vLLM", "PERMISSION DENIED", "Docker group access is effectively privileged"} {
		if !strings.Contains(status.String(), expected) {
			t.Fatalf("status missing %q:\n%s", expected, status.String())
		}
	}
	var runtime bytes.Buffer
	if err := WriteRuntime(&runtime, FormatHuman, value); err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{"Installation  AVAILABLE", "Execution     AVAILABLE", "version=0.9.0", "pid=42", "pytorch=2.8.0", "CPU=0-3"} {
		if !strings.Contains(runtime.String(), expected) {
			t.Fatalf("runtime missing %q:\n%s", expected, runtime.String())
		}
	}
	var stack bytes.Buffer
	if err := WriteStack(&stack, FormatHuman, value); err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{"Docker client", "28.0.0", "current user cannot access", "Docker group membership is effectively privileged"} {
		if !strings.Contains(stack.String(), expected) {
			t.Fatalf("stack missing %q:\n%s", expected, stack.String())
		}
	}
}

func TestStatusExplainsToolkitEvidenceAndReadiness(t *testing.T) {
	value := validReport()
	value.Readiness = model.Readiness{Deployment: "not_ready", Performance: "unknown"}
	deploymentFail := testFinding("CTR002", model.StatusFail, "toolkit missing")
	performanceUnknown := testFinding("PCIE002", model.StatusUnknown, "ACS evidence unavailable")
	performanceUnknown.Dimension = model.DimensionPerformance
	value.Findings = []model.Finding{deploymentFail, performanceUnknown}
	value.Node.Containers = model.ContainerState{
		State: model.StateAvailable, ClientState: model.StateAvailable, DaemonState: model.StateAvailable,
		ToolkitState: model.StateNotDetected, ToolkitPackageState: model.StateNotDetected,
		ToolkitCLIState: model.StateNotDetected, NVIDIARuntimeState: model.StateNotDetected,
		CDIState: model.StateNotDetected, GPUContainerState: model.StateNotDetected,
	}
	var output bytes.Buffer
	if err := WriteStatus(&output, FormatHuman, value, Options{}); err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{
		"NVIDIA CTK NOT INSTALLED",
		"no package, toolkit command, NVIDIA runtime, or standard CDI specification found",
		"Docker GPU NOT CONFIGURED",
		"1 blocker(s)",
		"1 evidence gap(s)",
	} {
		if !strings.Contains(output.String(), expected) {
			t.Fatalf("status missing %q:\n%s", expected, output.String())
		}
	}
}
