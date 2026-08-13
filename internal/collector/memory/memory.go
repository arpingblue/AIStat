package memory

import (
	"bufio"
	"context"
	"strconv"
	"strings"

	"github.com/arpingblue/AIStat/internal/collector"
	"github.com/arpingblue/AIStat/internal/model"
)

type Collector struct{}

func (Collector) ID() collector.ID                 { return "memory" }
func (Collector) Provides() []collector.Capability { return []collector.Capability{"memory"} }
func (Collector) Requires() []collector.Capability { return nil }

func (c Collector) Collect(_ context.Context, env collector.Env) collector.Result {
	if env.Platform != "linux" && !env.Fixture {
		return collector.Unsupported(c.ID(), "memory")
	}
	raw, err := env.FileSystem.ReadFile("/proc/meminfo")
	if err != nil {
		return failed(c.ID(), "memory", collector.FileErrorState(err), err)
	}
	mem, err := Parse(string(raw))
	if err != nil {
		return failed(c.ID(), "memory", model.StateParseError, err)
	}
	if raw, readErr := env.FileSystem.ReadFile("/sys/kernel/mm/transparent_hugepage/enabled"); readErr == nil {
		mem.THPMode = selected(strings.TrimSpace(string(raw)))
	}
	if raw, readErr := env.FileSystem.ReadFile("/proc/sys/kernel/numa_balancing"); readErr == nil {
		value := strings.TrimSpace(string(raw)) == "1"
		mem.NUMABalancing = &value
	}
	if raw, readErr := env.FileSystem.ReadFile("/proc/self/limits"); readErr == nil {
		mem.MemlockSoftBytes, mem.MemlockHardBytes, mem.MemlockSoftUnlimited, mem.MemlockHardUnlimited = parseMemlock(string(raw))
	}
	return collector.Result{Collector: c.ID(), State: model.StateAvailable, Facts: []model.Fact{model.NewFact("memory", model.StateAvailable, mem, model.ConfidenceHigh, model.SourceRef{Collector: "memory", Source: "/proc/meminfo"})}}
}

func Parse(raw string) (model.Memory, error) {
	values := map[string]uint64{}
	scanner := bufio.NewScanner(strings.NewReader(raw))
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 2 {
			continue
		}
		value, err := strconv.ParseUint(fields[1], 10, 64)
		if err != nil {
			return model.Memory{}, err
		}
		values[strings.TrimSuffix(fields[0], ":")] = value * 1024
	}
	if values["MemTotal"] == 0 {
		return model.Memory{}, strconv.ErrSyntax
	}
	return model.Memory{State: model.StateAvailable, TotalBytes: values["MemTotal"], FreeBytes: values["MemAvailable"], SwapTotalBytes: values["SwapTotal"], SwapFreeBytes: values["SwapFree"], HugePagesTotal: values["HugePages_Total"] / 1024, HugePagesFree: values["HugePages_Free"] / 1024, HugePageSizeBytes: values["Hugepagesize"]}, nil
}

func selected(raw string) string {
	for _, field := range strings.Fields(raw) {
		if strings.HasPrefix(field, "[") && strings.HasSuffix(field, "]") {
			return strings.Trim(field, "[]")
		}
	}
	return "unknown"
}
func parseMemlock(raw string) (soft, hard *uint64, softUnlimited, hardUnlimited bool) {
	for _, line := range strings.Split(raw, "\n") {
		if !strings.HasPrefix(strings.TrimSpace(line), "Max locked memory") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 5 {
			return
		}
		softRaw, hardRaw := fields[len(fields)-3], fields[len(fields)-2]
		if softRaw == "unlimited" {
			softUnlimited = true
		} else if value, err := strconv.ParseUint(softRaw, 10, 64); err == nil {
			soft = &value
		}
		if hardRaw == "unlimited" {
			hardUnlimited = true
		} else if value, err := strconv.ParseUint(hardRaw, 10, 64); err == nil {
			hard = &value
		}
		return
	}
	return
}

func failed(id collector.ID, key string, state model.FactState, err error) collector.Result {
	return collector.Result{Collector: id, State: state, Facts: []model.Fact{{Key: key, State: state, Confidence: model.ConfidenceLow}}, Diagnostics: []model.Diagnostic{{Level: "warn", Code: "read_or_parse", Message: err.Error()}}}
}
