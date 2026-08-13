package normalize

import (
	"testing"
	"time"

	"github.com/arpingblue/AIStat/internal/collector"
	"github.com/arpingblue/AIStat/internal/model"
)

func TestResultsPreservesUnavailableState(t *testing.T) {
	snapshot := Empty(time.Unix(0, 0), "test", "general")
	got, err := Results(snapshot, []collector.Result{{Collector: "memory", Facts: []model.Fact{{Key: "memory", State: model.StatePermissionDenied, Confidence: model.ConfidenceHigh}}}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got.Memory.State != model.StatePermissionDenied {
		t.Fatalf("got %s", got.Memory.State)
	}
}
func TestResultsRejectsUnknownFact(t *testing.T) {
	snapshot := Empty(time.Unix(0, 0), "test", "general")
	_, err := Results(snapshot, []collector.Result{{Facts: []model.Fact{model.NewFact("bad", model.StateAvailable, map[string]string{}, model.ConfidenceHigh)}}}, nil)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestResultsJoinsPCIAndNUMALocality(t *testing.T) {
	snapshot := Empty(time.Unix(0, 0), "test", "general")
	pci := model.PCIState{State: model.StateAvailable, Devices: []model.PCIDevice{{Address: "0000:3b:00.0", NUMANode: model.Int(1), LinkWidth: 8, MaxWidth: 16}}}
	gpus := model.GPUState{State: model.StateAvailable, Devices: []model.GPU{{Index: 0, PCIAddress: "00000000:3b:00.0"}}}
	numa := model.NUMAState{State: model.StateAvailable, Nodes: []model.NUMANode{{ID: 1, CPUList: []int{4, 5}}}}
	got, err := Results(snapshot, []collector.Result{{Facts: []model.Fact{model.NewFact("pci", model.StateAvailable, pci, model.ConfidenceHigh), model.NewFact("gpus", model.StateAvailable, gpus, model.ConfidenceHigh), model.NewFact("numa", model.StateAvailable, numa, model.ConfidenceHigh)}}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	gpu := got.GPUs.Devices[0]
	if gpu.NUMANode == nil || *gpu.NUMANode != 1 || gpu.PCIELinkWidth == nil || *gpu.PCIELinkWidth != 8 || len(got.CPU.Logical) != 2 {
		t.Fatalf("enrichment failed: gpu=%#v logical=%#v", gpu, got.CPU.Logical)
	}
}
