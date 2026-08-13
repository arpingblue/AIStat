package system

import (
	"bufio"
	"context"
	"strconv"
	"strings"

	"github.com/arpingblue/AIStat/internal/collector"
	"github.com/arpingblue/AIStat/internal/model"
)

type Collector struct{}

func (Collector) ID() collector.ID                 { return "system" }
func (Collector) Provides() []collector.Capability { return []collector.Capability{"host"} }
func (Collector) Requires() []collector.Capability { return nil }

func (c Collector) Collect(_ context.Context, env collector.Env) collector.Result {
	if env.Platform != "linux" && !env.Fixture {
		return collector.Unsupported(c.ID(), "host")
	}
	host := model.Host{State: model.StateAvailable}
	primarySuccess := false
	var primaryErr error
	if data, err := env.FileSystem.ReadFile("/etc/os-release"); err == nil {
		primarySuccess = true
		values := parseOSRelease(string(data))
		host.Distro = values["PRETTY_NAME"]
	} else {
		primaryErr = err
	}
	if data, err := env.FileSystem.ReadFile("/proc/sys/kernel/osrelease"); err == nil {
		primarySuccess = true
		host.Kernel = strings.TrimSpace(string(data))
	} else if primaryErr == nil || collector.FileErrorState(err) == model.StatePermissionDenied {
		primaryErr = err
	}
	if data, err := env.FileSystem.ReadFile("/proc/uptime"); err == nil {
		fields := strings.Fields(string(data))
		if len(fields) > 0 {
			host.UptimeSeconds, _ = strconv.ParseFloat(fields[0], 64)
		}
	}
	host.BIOSVendor = readTrim(env, "/sys/class/dmi/id/bios_vendor")
	host.BIOSVersion = readTrim(env, "/sys/class/dmi/id/bios_version")
	host.Machine = readTrim(env, "/sys/class/dmi/id/product_name")
	host.Virtualization = detectVirtualization(env, host.Machine)
	if !primarySuccess {
		state := collector.FileErrorState(primaryErr)
		return collector.Result{Collector: c.ID(), State: state, Facts: []model.Fact{{Key: "host", State: state, Confidence: model.ConfidenceLow}}, Diagnostics: []model.Diagnostic{{Level: "warn", Code: "host_sources", Message: primaryErr.Error()}}}
	}
	return collector.Result{Collector: c.ID(), State: model.StateAvailable, Facts: []model.Fact{model.NewFact("host", model.StateAvailable, host, model.ConfidenceHigh, model.SourceRef{Collector: "system", Source: "/etc/os-release,/proc/sys/kernel/osrelease"})}}
}

func readTrim(env collector.Env, path string) string {
	raw, err := env.FileSystem.ReadFile(path)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(raw))
}
func detectVirtualization(env collector.Env, machine string) string {
	if raw, err := env.FileSystem.ReadFile("/proc/1/cgroup"); err == nil {
		lower := strings.ToLower(string(raw))
		if strings.Contains(lower, "docker") || strings.Contains(lower, "containerd") || strings.Contains(lower, "kubepods") {
			return "container"
		}
	}
	if hypervisor := readTrim(env, "/sys/hypervisor/type"); hypervisor != "" {
		return strings.ToLower(hypervisor)
	}
	lower := strings.ToLower(machine)
	for _, marker := range []string{"virtual", "vmware", "kvm", "qemu", "hyper-v"} {
		if strings.Contains(lower, marker) {
			return "vm"
		}
	}
	if machine != "" {
		return "bare_metal"
	}
	return "unknown"
}

func parseOSRelease(raw string) map[string]string {
	result := map[string]string{}
	scanner := bufio.NewScanner(strings.NewReader(raw))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if ok {
			result[key] = strings.Trim(strings.TrimSpace(value), "\"")
		}
	}
	return result
}
