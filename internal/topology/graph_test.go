package topology

import (
	"testing"

	"github.com/arpingblue/AIStat/internal/model"
)

func TestBuildAndEnrich(t *testing.T) {
	snapshot := model.Snapshot{CPU: model.CPU{Logical: []model.LogicalCPU{{ID: 0, PackageID: 0, CoreID: 0}, {ID: 1, PackageID: 0, CoreID: 0}}}, NUMA: model.NUMAState{Nodes: []model.NUMANode{{ID: 0, CPUList: []int{0, 1}}}}, PCI: model.PCIState{Devices: []model.PCIDevice{{Address: "0000:3b:00.0", NUMANode: model.Int(0)}}}, GPUs: model.GPUState{Devices: []model.GPU{{Index: 0, UUID: "GPU-0", PCIAddress: "0000:3b:00.0", NUMANode: model.Int(0)}}}, Processes: model.ProcessState{Processes: []model.Process{{PID: 42, GPUUUIDs: []string{"GPU-0"}, CPUSet: []int{0}}}}}
	base := BuildBase(&snapshot)
	graph := Enrich(base, &snapshot)
	if _, ok := graph.Nodes["gpu:GPU-0"]; !ok {
		t.Fatal("missing GPU node")
	}
	if node, ok := graph.Nodes["cpu_package:0"]; !ok || node.Kind != NodeCPUPackage {
		t.Fatal("missing CPU package node")
	}
	if distance, ok := graph.Distance("process:42", "gpu:GPU-0"); !ok || distance != 1 {
		t.Fatalf("distance=%d ok=%v", distance, ok)
	}
	if node, ok := graph.LocalNUMA("gpu:GPU-0"); !ok || node != 0 {
		t.Fatalf("NUMA=%d ok=%v", node, ok)
	}
}
func TestRootForPCI(t *testing.T) {
	snapshot := model.Snapshot{PCI: model.PCIState{Devices: []model.PCIDevice{{Address: "0000:01:00.0"}, {Address: "0000:02:00.0", Parent: "0000:01:00.0"}}}}
	if got := RootForPCI(&snapshot, "0000:02:00.0"); got != "0000:01:00.0" {
		t.Fatalf("got %s", got)
	}
}
