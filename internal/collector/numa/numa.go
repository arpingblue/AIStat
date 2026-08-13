package numa

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/arpingblue/AIStat/internal/collector"
	"github.com/arpingblue/AIStat/internal/model"
)

type Collector struct{}

func (Collector) ID() collector.ID                 { return "numa" }
func (Collector) Provides() []collector.Capability { return []collector.Capability{"numa"} }
func (Collector) Requires() []collector.Capability { return []collector.Capability{"cpu", "memory"} }

func (c Collector) Collect(_ context.Context, env collector.Env) collector.Result {
	if env.Platform != "linux" && !env.Fixture {
		return collector.Unsupported(c.ID(), "numa")
	}
	entries, err := env.FileSystem.ReadDir("/sys/devices/system/node")
	if err != nil {
		return fail(c.ID(), collector.FileErrorState(err), err)
	}
	nodes := []model.NUMANode{}
	for _, entry := range entries {
		if !entry.IsDir() || !strings.HasPrefix(entry.Name(), "node") {
			continue
		}
		id, err := strconv.Atoi(strings.TrimPrefix(entry.Name(), "node"))
		if err != nil {
			continue
		}
		base := "/sys/devices/system/node/" + entry.Name()
		if online, onlineErr := env.FileSystem.ReadFile(base + "/online"); onlineErr == nil && strings.TrimSpace(string(online)) == "0" {
			continue
		}
		cpusRaw, err := env.FileSystem.ReadFile(base + "/cpulist")
		if err != nil {
			return fail(c.ID(), collector.FileErrorState(err), err)
		}
		cpus, err := ParseList(strings.TrimSpace(string(cpusRaw)))
		if err != nil {
			return fail(c.ID(), model.StateParseError, err)
		}
		node := model.NUMANode{ID: id, CPUList: cpus}
		if distance, err := env.FileSystem.ReadFile(base + "/distance"); err == nil {
			values, parseErr := ParseDistance(string(distance))
			if parseErr == nil {
				for _, value := range values {
					node.Distances = append(node.Distances, int(value))
				}
			}
		}
		if meminfo, err := env.FileSystem.ReadFile(base + "/meminfo"); err == nil {
			node.MemoryBytes = parseMemTotal(string(meminfo))
		}
		nodes = append(nodes, node)
	}
	if len(nodes) == 0 {
		return collector.Result{Collector: c.ID(), State: model.StateNotDetected, Facts: []model.Fact{{Key: "numa", State: model.StateNotDetected, Confidence: model.ConfidenceHigh}}}
	}
	value := model.NUMAState{State: model.StateAvailable, Nodes: nodes}
	return collector.Result{Collector: c.ID(), State: model.StateAvailable, Facts: []model.Fact{model.NewFact("numa", model.StateAvailable, value, model.ConfidenceHigh, model.SourceRef{Collector: "numa", Source: "/sys/devices/system/node"})}}
}

func ParseDistance(raw string) ([]uint32, error) {
	result := []uint32{}
	for _, field := range strings.Fields(raw) {
		value, err := strconv.ParseUint(field, 10, 32)
		if err != nil {
			return nil, err
		}
		result = append(result, uint32(value))
	}
	return result, nil
}

func ParseList(raw string) ([]int, error) {
	if strings.TrimSpace(raw) == "" {
		return nil, nil
	}
	result := []int{}
	seen := map[int]bool{}
	for _, part := range strings.Split(raw, ",") {
		bounds := strings.SplitN(strings.TrimSpace(part), "-", 2)
		first, err := strconv.Atoi(bounds[0])
		if err != nil {
			return nil, err
		}
		last := first
		if len(bounds) == 2 {
			last, err = strconv.Atoi(bounds[1])
			if err != nil || last < first {
				return nil, fmt.Errorf("invalid range %q", part)
			}
		}
		for value := first; value <= last; value++ {
			if !seen[value] {
				result = append(result, value)
				seen[value] = true
			}
		}
	}
	return result, nil
}

func parseMemTotal(raw string) uint64 {
	for _, line := range strings.Split(raw, "\n") {
		if strings.Contains(line, "MemTotal") {
			fields := strings.Fields(line)
			for i, f := range fields {
				if strings.HasPrefix(f, "MemTotal") && i+1 < len(fields) {
					value, _ := strconv.ParseUint(fields[i+1], 10, 64)
					return value * 1024
				}
			}
		}
	}
	return 0
}
func fail(id collector.ID, state model.FactState, err error) collector.Result {
	return collector.Result{Collector: id, State: state, Facts: []model.Fact{{Key: "numa", State: state, Confidence: model.ConfidenceLow}}, Diagnostics: []model.Diagnostic{{Level: "warn", Code: "read_or_parse", Message: err.Error()}}}
}
