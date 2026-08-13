package topology

import (
	"sort"
)

type NodeKind string
type EdgeKind string

const (
	NodeHost       NodeKind = "host"
	NodeNUMA       NodeKind = "numa"
	NodeCPUPackage NodeKind = "cpu_package"
	NodeCPU        NodeKind = "cpu"
	NodePCIRoot    NodeKind = "pci_root"
	NodePCIBridge  NodeKind = "pci_bridge"
	NodePCI        NodeKind = "pci"
	NodeGPU        NodeKind = "gpu"
	NodeNIC        NodeKind = "nic"
	NodeRDMA       NodeKind = "rdma"
	NodeStorage    NodeKind = "block_device"
	NodeProcess    NodeKind = "process"
	NodeContainer  NodeKind = "container"
)

const (
	EdgeContains  EdgeKind = "contains"
	EdgeAttached  EdgeKind = "attached"
	EdgeLocalTo   EdgeKind = "local_to"
	EdgeUses      EdgeKind = "uses"
	EdgeRunsIn    EdgeKind = "runs_in"
	EdgeP2P       EdgeKind = "p2p"
	EdgeConnected EdgeKind = "connected_to"
)

type Node struct {
	ID         string            `json:"id"`
	Kind       NodeKind          `json:"kind"`
	Label      string            `json:"label,omitempty"`
	Attributes map[string]string `json:"attributes,omitempty"`
}

type Edge struct {
	From       string            `json:"from"`
	To         string            `json:"to"`
	Kind       EdgeKind          `json:"kind"`
	Weight     int               `json:"weight,omitempty"`
	Attributes map[string]string `json:"attributes,omitempty"`
}

type Graph struct {
	Nodes map[string]Node `json:"nodes"`
	Edges []Edge          `json:"edges"`
}

func New() *Graph { return &Graph{Nodes: map[string]Node{}} }
func (g *Graph) AddNode(node Node) {
	if g.Nodes == nil {
		g.Nodes = map[string]Node{}
	}
	g.Nodes[node.ID] = node
}
func (g *Graph) AddEdge(edge Edge) {
	if _, ok := g.Nodes[edge.From]; !ok {
		return
	}
	if _, ok := g.Nodes[edge.To]; !ok {
		return
	}
	g.Edges = append(g.Edges, edge)
}

func (g *Graph) Neighbors(id string) []Node {
	ids := map[string]bool{}
	for _, edge := range g.Edges {
		if edge.From == id {
			ids[edge.To] = true
		}
		if edge.To == id {
			ids[edge.From] = true
		}
	}
	result := make([]Node, 0, len(ids))
	for other := range ids {
		result = append(result, g.Nodes[other])
	}
	sort.Slice(result, func(i, j int) bool { return result[i].ID < result[j].ID })
	return result
}

func (g *Graph) Distance(from, to string) (int, bool) {
	if _, ok := g.Nodes[from]; !ok {
		return 0, false
	}
	if _, ok := g.Nodes[to]; !ok {
		return 0, false
	}
	if from == to {
		return 0, true
	}
	type item struct {
		id       string
		distance int
	}
	queue := []item{{from, 0}}
	seen := map[string]bool{from: true}
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		for _, next := range g.Neighbors(current.id) {
			if seen[next.ID] {
				continue
			}
			if next.ID == to {
				return current.distance + 1, true
			}
			seen[next.ID] = true
			queue = append(queue, item{next.ID, current.distance + 1})
		}
	}
	return 0, false
}

func (g *Graph) LocalNUMA(id string) (int, bool) {
	node, ok := g.Nodes[id]
	if !ok {
		return 0, false
	}
	raw, ok := node.Attributes["numa_node"]
	if !ok {
		return 0, false
	}
	var result int
	sign := 1
	if len(raw) > 0 && raw[0] == '-' {
		sign = -1
		raw = raw[1:]
	}
	if raw == "" {
		return 0, false
	}
	for _, char := range raw {
		if char < '0' || char > '9' {
			return 0, false
		}
		result = result*10 + int(char-'0')
	}
	result *= sign
	return result, result >= 0
}
