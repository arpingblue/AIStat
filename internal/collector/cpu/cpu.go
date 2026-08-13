package cpu

import (
	"bufio"
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/arpingblue/AIStat/internal/collector"
	"github.com/arpingblue/AIStat/internal/fsx"
	"github.com/arpingblue/AIStat/internal/model"
)

type Collector struct{}

func (Collector) ID() collector.ID                 { return "cpu" }
func (Collector) Provides() []collector.Capability { return []collector.Capability{"cpu"} }
func (Collector) Requires() []collector.Capability { return nil }

func (c Collector) Collect(_ context.Context, env collector.Env) collector.Result {
	if env.Platform != "linux" && !env.Fixture {
		return collector.Unsupported(c.ID(), "cpu")
	}
	raw, err := env.FileSystem.ReadFile("/proc/cpuinfo")
	if err != nil {
		return failed(c.ID(), collector.FileErrorState(err), err)
	}
	value, err := Parse(string(raw))
	if err != nil {
		return failed(c.ID(), model.StateParseError, err)
	}
	if raw, readErr := env.FileSystem.ReadFile("/sys/devices/system/cpu/cpu0/cpufreq/scaling_governor"); readErr == nil {
		value.Governor = strings.TrimSpace(string(raw))
	}
	if raw, readErr := env.FileSystem.ReadFile("/sys/devices/system/cpu/cpu0/cpufreq/scaling_cur_freq"); readErr == nil {
		value.FrequencyKHz, _ = strconv.ParseUint(strings.TrimSpace(string(raw)), 10, 64)
	}
	value.CacheBytes = readCaches(env.FileSystem)
	return collector.Result{Collector: c.ID(), State: model.StateAvailable, Facts: []model.Fact{model.NewFact("cpu", model.StateAvailable, value, model.ConfidenceHigh, model.SourceRef{Collector: "cpu", Source: "/proc/cpuinfo"})}}
}

func readCaches(fileSystem fsx.FileSystem) map[string]uint64 {
	const root = "/sys/devices/system/cpu/cpu0/cache"
	entries, err := fileSystem.ReadDir(root)
	if err != nil {
		return nil
	}
	result := map[string]uint64{}
	for _, entry := range entries {
		if !entry.IsDir() || !strings.HasPrefix(entry.Name(), "index") {
			continue
		}
		base := root + "/" + entry.Name()
		levelRaw, levelErr := fileSystem.ReadFile(base + "/level")
		typeRaw, typeErr := fileSystem.ReadFile(base + "/type")
		sizeRaw, sizeErr := fileSystem.ReadFile(base + "/size")
		if levelErr != nil || typeErr != nil || sizeErr != nil {
			continue
		}
		size, parseErr := parseCacheSize(strings.TrimSpace(string(sizeRaw)))
		if parseErr != nil {
			continue
		}
		kind := strings.ToLower(strings.TrimSpace(string(typeRaw)))
		suffix := ""
		if kind == "data" {
			suffix = "d"
		} else if kind == "instruction" {
			suffix = "i"
		}
		key := "L" + strings.TrimSpace(string(levelRaw)) + suffix
		result[key] += size
	}
	if len(result) == 0 {
		return nil
	}
	return result
}

func parseCacheSize(raw string) (uint64, error) {
	raw = strings.TrimSpace(strings.ToUpper(raw))
	if raw == "" {
		return 0, strconv.ErrSyntax
	}
	multiplier := uint64(1)
	switch raw[len(raw)-1] {
	case 'K':
		multiplier, raw = 1<<10, raw[:len(raw)-1]
	case 'M':
		multiplier, raw = 1<<20, raw[:len(raw)-1]
	case 'G':
		multiplier, raw = 1<<30, raw[:len(raw)-1]
	}
	value, err := strconv.ParseUint(raw, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("parse cache size: %w", err)
	}
	return value * multiplier, nil
}

func Parse(raw string) (model.CPU, error) {
	sockets, cores := map[string]bool{}, map[string]bool{}
	logical := 0
	logicalCPUs := []model.LogicalCPU{}
	modelName, vendor := "", ""
	physical, core, processor := "0", "", -1
	flush := func() {
		if processor < 0 {
			return
		}
		coreKey := core
		if coreKey == "" {
			coreKey = strconv.Itoa(processor)
		}
		packageID, _ := strconv.Atoi(physical)
		coreID, _ := strconv.Atoi(coreKey)
		sockets[physical] = true
		cores[physical+":"+coreKey] = true
		logicalCPUs = append(logicalCPUs, model.LogicalCPU{ID: processor, PackageID: packageID, CoreID: coreID})
		logical++
		physical, core, processor = "0", "", -1
	}
	scanner := bufio.NewScanner(strings.NewReader(raw))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			flush()
			continue
		}
		key, value, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		key, value = strings.TrimSpace(key), strings.TrimSpace(value)
		switch key {
		case "model name", "Processor":
			if modelName == "" {
				modelName = value
			}
		case "vendor_id", "CPU implementer":
			if vendor == "" {
				vendor = value
			}
		case "physical id":
			physical = value
		case "core id":
			core = value
		case "processor":
			processor, _ = strconv.Atoi(value)
		}
	}
	flush()
	if logical == 0 {
		return model.CPU{}, strconv.ErrSyntax
	}
	smt := logical > len(cores)
	threadsPerCore := 0
	if len(cores) > 0 {
		threadsPerCore = logical / len(cores)
	}
	return model.CPU{State: model.StateAvailable, Model: modelName, Vendor: vendor, Sockets: len(sockets), PhysicalCores: len(cores), LogicalCores: logical, Logical: logicalCPUs, ThreadsPerCore: threadsPerCore, SMT: &smt}, nil
}

func ParseCPUList(raw string) ([]int, error) {
	if strings.TrimSpace(raw) == "" {
		return nil, nil
	}
	result := []int{}
	seen := map[int]bool{}
	for _, part := range strings.Split(raw, ",") {
		bounds := strings.SplitN(strings.TrimSpace(part), "-", 2)
		first, err := strconv.Atoi(bounds[0])
		if err != nil || first < 0 {
			return nil, fmt.Errorf("invalid CPU range %q", part)
		}
		last := first
		if len(bounds) == 2 {
			last, err = strconv.Atoi(bounds[1])
			if err != nil || last < first {
				return nil, fmt.Errorf("invalid CPU range %q", part)
			}
		}
		for value := first; value <= last; value++ {
			if !seen[value] {
				seen[value] = true
				result = append(result, value)
			}
		}
	}
	return result, nil
}

func failed(id collector.ID, state model.FactState, err error) collector.Result {
	return collector.Result{Collector: id, State: state, Facts: []model.Fact{{Key: "cpu", State: state, Confidence: model.ConfidenceLow}}, Diagnostics: []model.Diagnostic{{Level: "warn", Code: "read_or_parse", Message: err.Error()}}}
}
