package normalize

import (
	"encoding/json"
	"fmt"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/arpingblue/AIStat/internal/collector"
	"github.com/arpingblue/AIStat/internal/model"
)

func Empty(now time.Time, toolVersion, profile string) model.Snapshot {
	return model.Snapshot{
		Meta: model.Meta{SchemaVersion: "0.1", ToolVersion: toolVersion, CollectedAt: now.UTC(), OS: runtime.GOOS, Arch: runtime.GOARCH, Profile: profile}, Host: model.Host{State: model.StateUnknown},
		CPU: model.CPU{State: model.StateUnknown}, Memory: model.Memory{State: model.StateUnknown}, NUMA: model.NUMAState{State: model.StateUnknown}, PCI: model.PCIState{State: model.StateUnknown}, GPUs: model.GPUState{State: model.StateUnknown}, Network: model.NetworkState{State: model.StateUnknown}, RDMA: model.RDMAState{State: model.StateUnknown}, Storage: model.StorageState{State: model.StateUnknown}, NVIDIA: model.NVIDIAStack{State: model.StateUnknown, XIDState: model.StateUnknown}, Containers: model.ContainerState{State: model.StateUnknown, DaemonState: model.StateUnknown}, Processes: model.ProcessState{State: model.StateUnknown}, Runtimes: model.RuntimeState{State: model.StateUnknown},
	}
}

func Results(snapshot model.Snapshot, results []collector.Result, statuses []model.CollectorStatus) (model.Snapshot, error) {
	for _, result := range results {
		for _, fact := range result.Facts {
			if err := apply(&snapshot, fact); err != nil {
				return snapshot, fmt.Errorf("normalize %s/%s: %w", result.Collector, fact.Key, err)
			}
		}
	}
	snapshot.Collectors = statuses
	enrich(&snapshot)
	return snapshot, nil
}

func enrich(snapshot *model.Snapshot) {
	pci := map[string]model.PCIDevice{}
	for _, device := range snapshot.PCI.Devices {
		pci[normalizeBDF(device.Address)] = device
	}
	for i := range snapshot.GPUs.Devices {
		if device, ok := pci[normalizeBDF(snapshot.GPUs.Devices[i].PCIAddress)]; ok {
			if device.NUMANode != nil {
				snapshot.GPUs.Devices[i].NUMANode = device.NUMANode
			}
			if snapshot.GPUs.Devices[i].PCIELinkWidth == nil && device.LinkWidth > 0 {
				value := device.LinkWidth
				snapshot.GPUs.Devices[i].PCIELinkWidth = &value
			}
			if snapshot.GPUs.Devices[i].PCIEMaxLinkWidth == nil && device.MaxWidth > 0 {
				value := device.MaxWidth
				snapshot.GPUs.Devices[i].PCIEMaxLinkWidth = &value
			}
		}
	}
	logicalByID := map[int]model.LogicalCPU{}
	for _, cpu := range snapshot.CPU.Logical {
		logicalByID[cpu.ID] = cpu
	}
	for _, node := range snapshot.NUMA.Nodes {
		for _, id := range node.CPUList {
			cpu := logicalByID[id]
			cpu.ID = id
			cpu.NUMANode = model.Int(node.ID)
			logicalByID[id] = cpu
		}
	}
	if len(logicalByID) > 0 {
		logical := make([]model.LogicalCPU, 0, len(logicalByID))
		for _, cpu := range logicalByID {
			logical = append(logical, cpu)
		}
		sort.Slice(logical, func(i, j int) bool { return logical[i].ID < logical[j].ID })
		snapshot.CPU.Logical = logical
	}
}
func normalizeBDF(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	parts := strings.Split(value, ":")
	if len(parts) == 3 && len(parts[0]) > 4 {
		parts[0] = parts[0][len(parts[0])-4:]
		return strings.Join(parts, ":")
	}
	return value
}

func apply(snapshot *model.Snapshot, fact model.Fact) error {
	if fact.State != model.StateAvailable {
		setState(snapshot, fact.Key, fact.State)
		return nil
	}
	var target any
	switch fact.Key {
	case "host":
		target = &snapshot.Host
	case "cpu":
		target = &snapshot.CPU
	case "memory":
		target = &snapshot.Memory
	case "numa":
		target = &snapshot.NUMA
	case "pci":
		target = &snapshot.PCI
	case "gpus":
		target = &snapshot.GPUs
	case "p2p":
		target = &snapshot.P2P
	case "topology":
		target = &snapshot.Topology
	case "network":
		target = &snapshot.Network
	case "rdma":
		target = &snapshot.RDMA
	case "storage":
		target = &snapshot.Storage
	case "nvidia":
		target = &snapshot.NVIDIA
	case "containers":
		target = &snapshot.Containers
	case "processes":
		target = &snapshot.Processes
	case "runtimes":
		target = &snapshot.Runtimes
	default:
		return fmt.Errorf("unknown fact key %q", fact.Key)
	}
	if err := json.Unmarshal(fact.Value, target); err != nil {
		setState(snapshot, fact.Key, model.StateParseError)
		return err
	}
	return nil
}

func setState(s *model.Snapshot, key string, state model.FactState) {
	switch key {
	case "host":
		s.Host.State = state
	case "cpu":
		s.CPU.State = state
	case "memory":
		s.Memory.State = state
	case "numa":
		s.NUMA.State = state
	case "pci":
		s.PCI.State = state
	case "gpus":
		s.GPUs.State = state
	case "network":
		s.Network.State = state
	case "rdma":
		s.RDMA.State = state
	case "storage":
		s.Storage.State = state
	case "nvidia":
		s.NVIDIA.State = state
	case "containers":
		s.Containers.State = state
	case "processes":
		s.Processes.State = state
	case "runtimes":
		s.Runtimes.State = state
	}
}
