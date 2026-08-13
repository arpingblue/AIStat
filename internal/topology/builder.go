package topology

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/arpingblue/AIStat/internal/model"
)

func BuildBase(snapshot *model.Snapshot) *Graph {
	g := New()
	g.AddNode(Node{ID: "host", Kind: NodeHost, Label: snapshot.Meta.Hostname})
	for _, numa := range snapshot.NUMA.Nodes {
		id := fmt.Sprintf("numa:%d", numa.ID)
		g.AddNode(Node{ID: id, Kind: NodeNUMA, Label: id, Attributes: map[string]string{"numa_node": strconv.Itoa(numa.ID)}})
		g.AddEdge(Edge{From: "host", To: id, Kind: EdgeContains})
		for _, cpu := range numa.CPUList {
			cpuID := fmt.Sprintf("cpu:%d", cpu)
			g.AddNode(Node{ID: cpuID, Kind: NodeCPU, Label: cpuID, Attributes: map[string]string{"numa_node": strconv.Itoa(numa.ID)}})
			g.AddEdge(Edge{From: id, To: cpuID, Kind: EdgeContains})
		}
	}
	packages := map[int]bool{}
	for _, cpu := range snapshot.CPU.Logical {
		packageID := fmt.Sprintf("cpu_package:%d", cpu.PackageID)
		if !packages[cpu.PackageID] {
			packages[cpu.PackageID] = true
			g.AddNode(Node{ID: packageID, Kind: NodeCPUPackage, Label: packageID})
			g.AddEdge(Edge{From: "host", To: packageID, Kind: EdgeContains})
		}
		cpuID := fmt.Sprintf("cpu:%d", cpu.ID)
		if _, ok := g.Nodes[cpuID]; !ok {
			g.AddNode(Node{ID: cpuID, Kind: NodeCPU, Label: cpuID})
		}
		g.AddEdge(Edge{From: packageID, To: cpuID, Kind: EdgeContains})
	}
	for _, pci := range snapshot.PCI.Devices {
		kind := NodePCI
		if strings.HasPrefix(pci.Class, "0x0604") {
			kind = NodePCIBridge
		}
		id := "pci:" + strings.ToLower(pci.Address)
		g.AddNode(Node{ID: id, Kind: kind, Label: pci.Address, Attributes: map[string]string{"numa_node": numaAttribute(pci.NUMANode), "parent": pci.Parent}})
		if pci.NUMANode != nil {
			g.AddEdge(Edge{From: id, To: fmt.Sprintf("numa:%d", *pci.NUMANode), Kind: EdgeLocalTo})
		}
	}
	for _, pci := range snapshot.PCI.Devices {
		id := "pci:" + strings.ToLower(pci.Address)
		parent := "pci:" + strings.ToLower(pci.Parent)
		if pci.Parent != "" {
			if _, ok := g.Nodes[parent]; ok {
				g.AddEdge(Edge{From: parent, To: id, Kind: EdgeContains})
				continue
			}
		}
		rootID := "pci_root:" + pciRootLabel(pci.Address)
		if _, ok := g.Nodes[rootID]; !ok {
			g.AddNode(Node{ID: rootID, Kind: NodePCIRoot, Label: pciRootLabel(pci.Address)})
			g.AddEdge(Edge{From: "host", To: rootID, Kind: EdgeContains})
		}
		g.AddEdge(Edge{From: rootID, To: id, Kind: EdgeContains})
	}
	for _, gpu := range snapshot.GPUs.Devices {
		key := gpu.UUID
		if key == "" {
			key = strconv.Itoa(gpu.Index)
		}
		id := "gpu:" + key
		g.AddNode(Node{ID: id, Kind: NodeGPU, Label: gpu.Name, Attributes: map[string]string{"numa_node": numaAttribute(gpu.NUMANode), "pci": strings.ToLower(gpu.PCIAddress)}})
		if _, ok := g.Nodes["pci:"+strings.ToLower(gpu.PCIAddress)]; ok {
			g.AddEdge(Edge{From: "pci:" + strings.ToLower(gpu.PCIAddress), To: id, Kind: EdgeAttached})
		}
		if gpu.NUMANode != nil {
			g.AddEdge(Edge{From: id, To: fmt.Sprintf("numa:%d", *gpu.NUMANode), Kind: EdgeLocalTo})
		}
	}
	for _, nic := range snapshot.Network.NICs {
		id := "nic:" + nic.Name
		g.AddNode(Node{ID: id, Kind: NodeNIC, Label: nic.Name, Attributes: map[string]string{"numa_node": numaAttribute(nic.NUMANode), "pci": strings.ToLower(nic.PCIAddress)}})
		if _, ok := g.Nodes["pci:"+strings.ToLower(nic.PCIAddress)]; ok {
			g.AddEdge(Edge{From: "pci:" + strings.ToLower(nic.PCIAddress), To: id, Kind: EdgeAttached})
		}
		if nic.NUMANode != nil {
			g.AddEdge(Edge{From: id, To: fmt.Sprintf("numa:%d", *nic.NUMANode), Kind: EdgeLocalTo})
		}
	}
	for _, rdma := range snapshot.RDMA.Devices {
		id := "rdma:" + rdma.Name
		g.AddNode(Node{ID: id, Kind: NodeRDMA, Label: rdma.Name, Attributes: map[string]string{"numa_node": numaAttribute(rdma.NUMANode), "pci": strings.ToLower(rdma.PCIAddress)}})
		if rdma.NetDevice != "" {
			g.AddEdge(Edge{From: "nic:" + rdma.NetDevice, To: id, Kind: EdgeAttached})
		}
	}
	for _, link := range snapshot.P2P {
		g.AddEdge(Edge{From: "gpu:" + link.FromGPU, To: "gpu:" + link.ToGPU, Kind: EdgeP2P, Weight: link.Distance, Attributes: map[string]string{"kind": link.Kind, "status": link.Status}})
	}
	for _, connection := range snapshot.Topology {
		from := topologyNodeID(connection.FromKind, connection.From)
		to := topologyNodeID(connection.ToKind, connection.To)
		g.AddEdge(Edge{From: from, To: to, Kind: EdgeConnected, Weight: connection.Distance, Attributes: map[string]string{"path": connection.Path, "status": connection.Status}})
	}
	for index, mount := range snapshot.Storage.Mounts {
		id := fmt.Sprintf("storage:mount:%d", index)
		g.AddNode(Node{ID: id, Kind: NodeStorage, Label: mount.Target, Attributes: map[string]string{"source": mount.Source, "fs_type": mount.FSType}})
		g.AddEdge(Edge{From: "host", To: id, Kind: EdgeContains})
	}
	for _, device := range snapshot.Storage.Devices {
		id := "storage:" + device.Name
		g.AddNode(Node{ID: id, Kind: NodeStorage, Label: device.Model, Attributes: map[string]string{"pci": device.PCIAddress, "numa_node": numaAttribute(device.NUMANode)}})
		if _, ok := g.Nodes["pci:"+device.PCIAddress]; ok {
			g.AddEdge(Edge{From: "pci:" + device.PCIAddress, To: id, Kind: EdgeAttached})
		} else {
			g.AddEdge(Edge{From: "host", To: id, Kind: EdgeContains})
		}
	}
	return g
}

func topologyNodeID(kind, id string) string {
	switch strings.ToLower(kind) {
	case "gpu":
		return "gpu:" + id
	case "nic":
		return "nic:" + id
	case "storage":
		return "storage:" + id
	default:
		return strings.ToLower(kind) + ":" + id
	}
}

func pciRootLabel(address string) string {
	parts := strings.Split(strings.ToLower(address), ":")
	if len(parts) >= 2 {
		return parts[0] + ":" + parts[1]
	}
	return strings.ToLower(address)
}

func numaAttribute(value *int) string {
	if value == nil {
		return "unknown"
	}
	return strconv.Itoa(*value)
}

func Enrich(base *Graph, snapshot *model.Snapshot) *Graph {
	g := &Graph{Nodes: map[string]Node{}, Edges: append([]Edge(nil), base.Edges...)}
	for key, value := range base.Nodes {
		g.Nodes[key] = value
	}
	for _, container := range snapshot.Containers.Devices {
		id := "container:" + container.ID
		g.AddNode(Node{ID: id, Kind: NodeContainer, Label: container.Name})
		g.AddEdge(Edge{From: "host", To: id, Kind: EdgeContains})
	}
	for _, process := range snapshot.Processes.Processes {
		id := fmt.Sprintf("process:%d", process.PID)
		g.AddNode(Node{ID: id, Kind: NodeProcess, Label: process.Executable})
		if process.ContainerID != "" {
			g.AddEdge(Edge{From: id, To: "container:" + process.ContainerID, Kind: EdgeRunsIn})
		} else {
			g.AddEdge(Edge{From: "host", To: id, Kind: EdgeContains})
		}
		for _, cpu := range process.CPUSet {
			g.AddEdge(Edge{From: id, To: fmt.Sprintf("cpu:%d", cpu), Kind: EdgeUses})
		}
		for _, gpu := range process.GPUUUIDs {
			g.AddEdge(Edge{From: id, To: "gpu:" + gpu, Kind: EdgeUses})
		}
	}
	return g
}

func RootForPCI(snapshot *model.Snapshot, address string) string {
	parents := map[string]string{}
	for _, device := range snapshot.PCI.Devices {
		parents[strings.ToLower(device.Address)] = strings.ToLower(device.Parent)
	}
	current := strings.ToLower(address)
	seen := map[string]bool{}
	for current != "" && !seen[current] {
		seen[current] = true
		parent := parents[current]
		if parent == "" {
			return current
		}
		current = parent
	}
	return current
}
