package storage

import (
	"bufio"
	"context"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/arpingblue/AIStat/internal/collector"
	"github.com/arpingblue/AIStat/internal/model"
	"github.com/arpingblue/AIStat/internal/redact"
)

type Collector struct{}

func (Collector) ID() collector.ID                 { return "storage" }
func (Collector) Provides() []collector.Capability { return []collector.Capability{"storage"} }
func (Collector) Requires() []collector.Capability { return nil }
func (c Collector) Collect(_ context.Context, env collector.Env) collector.Result {
	if env.Platform != "linux" && !env.Fixture {
		return collector.Unsupported(c.ID(), "storage")
	}
	raw, err := env.FileSystem.ReadFile("/proc/mounts")
	if err != nil {
		state := collector.FileErrorState(err)
		return collector.Result{Collector: c.ID(), State: state, Facts: []model.Fact{{Key: "storage", State: state, Confidence: model.ConfidenceLow}}, Diagnostics: []model.Diagnostic{{Level: "warn", Code: "read_or_parse", Message: err.Error()}}}
	}
	mounts := []model.Mount{}
	scanner := bufio.NewScanner(strings.NewReader(string(raw)))
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) >= 3 {
			mount := model.Mount{Source: redact.Path(fields[0]), Target: redact.Path(unescape(fields[1])), FSType: fields[2]}
			if !env.Fixture {
				fillCapacity(&mount)
			}
			mounts = append(mounts, mount)
		}
	}
	value := model.StorageState{State: model.StateAvailable, Mounts: mounts, Devices: collectDevices(env)}
	return collector.Result{Collector: c.ID(), State: model.StateAvailable, Facts: []model.Fact{model.NewFact("storage", model.StateAvailable, value, model.ConfidenceMedium, model.SourceRef{Collector: "storage", Source: "/proc/mounts"})}}
}

func collectDevices(env collector.Env) []model.BlockDevice {
	entries, err := env.FileSystem.ReadDir("/sys/block")
	if err != nil {
		return nil
	}
	result := []model.BlockDevice{}
	for _, entry := range entries {
		name := entry.Name()
		if strings.HasPrefix(name, "loop") || strings.HasPrefix(name, "ram") {
			continue
		}
		base := "/sys/block/" + name
		device := model.BlockDevice{Name: name}
		if raw, err := env.FileSystem.ReadFile(base + "/device/model"); err == nil {
			device.Model = strings.TrimSpace(string(raw))
		}
		if raw, err := env.FileSystem.ReadFile(base + "/size"); err == nil {
			sectors, _ := strconv.ParseUint(strings.TrimSpace(string(raw)), 10, 64)
			device.SizeBytes = sectors * 512
		}
		if raw, err := env.FileSystem.ReadFile(base + "/queue/rotational"); err == nil {
			value := strings.TrimSpace(string(raw)) == "1"
			device.Rotational = &value
		}
		if link, err := env.FileSystem.Readlink(base + "/device"); err == nil {
			for _, part := range strings.Split(filepath.ToSlash(link), "/") {
				if strings.Count(part, ":") == 2 && strings.Contains(part, ".") {
					device.PCIAddress = strings.ToLower(part)
				}
			}
		}
		if raw, err := env.FileSystem.ReadFile(base + "/device/numa_node"); err == nil {
			if value, parseErr := strconv.Atoi(strings.TrimSpace(string(raw))); parseErr == nil && value >= 0 {
				device.NUMANode = model.Int(value)
			}
		}
		result = append(result, device)
	}
	return result
}
func unescape(value string) string {
	replacements := map[string]string{"\\040": " ", "\\011": "\t", "\\012": "\n", "\\134": "\\"}
	for from, to := range replacements {
		value = strings.ReplaceAll(value, from, to)
	}
	return value
}
