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
