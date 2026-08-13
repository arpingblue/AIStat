package docker

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/arpingblue/AIStat/internal/collector"
	"github.com/arpingblue/AIStat/internal/execx"
	"github.com/arpingblue/AIStat/internal/model"
)

type Collector struct{}

func (Collector) ID() collector.ID                 { return "docker" }
func (Collector) Provides() []collector.Capability { return []collector.Capability{"containers"} }
func (Collector) Requires() []collector.Capability { return nil }

type versionResponse struct {
	Server *struct {
		Version string `json:"Version"`
	} `json:"Server"`
	Client struct {
		Version string `json:"Version"`
	} `json:"Client"`
}
type inspectResponse struct {
	ID    string `json:"Id"`
	Name  string `json:"Name"`
	State struct {
		PID int `json:"Pid"`
	} `json:"State"`
	Config struct {
		Image string   `json:"Image"`
		Env   []string `json:"Env"`
	} `json:"Config"`
	HostConfig struct {
		Runtime        string `json:"Runtime"`
		ShmSize        int64  `json:"ShmSize"`
		Memory         int64  `json:"Memory"`
		DeviceRequests []struct {
			Driver       string     `json:"Driver"`
			Count        int        `json:"Count"`
			DeviceIDs    []string   `json:"DeviceIDs"`
			Capabilities [][]string `json:"Capabilities"`
		} `json:"DeviceRequests"`
		Ulimits []struct {
			Name string `json:"Name"`
			Soft int64  `json:"Soft"`
			Hard int64  `json:"Hard"`
		} `json:"Ulimits"`
		CpusetCpus string `json:"CpusetCpus"`
		CpusetMems string `json:"CpusetMems"`
	} `json:"HostConfig"`
}

func (c Collector) Collect(ctx context.Context, env collector.Env) collector.Result {
	if env.Platform != "linux" && !env.Fixture {
		return collector.Unsupported(c.ID(), "containers")
	}
	state := model.ContainerState{State: model.StateAvailable, DaemonState: model.StateUnknown, Engine: "docker"}
	if toolkit, toolkitErr := env.Runner.Run(ctx, execx.CommandSpec{Name: "nvidia-ctk", Args: []string{"--version"}, Timeout: 2 * time.Second, OutputLimit: 64 << 10}); toolkitErr == nil {
		detected := true
		state.ToolkitDetected = &detected
		state.ToolkitVersion = firstVersion(toolkit.Stdout)
	} else {
		detected := false
		state.ToolkitDetected = &detected
	}
	version, err := env.Runner.Run(ctx, execx.CommandSpec{Name: "docker", Args: []string{"version", "--format", "{{json .}}"}, Timeout: 3 * time.Second, OutputLimit: 256 << 10})
	if err != nil {
		commandState := classify(version, err)
		state.State = commandState
		state.DaemonState = commandState
		return collector.Result{Collector: c.ID(), State: commandState, Facts: []model.Fact{model.NewFact("containers", model.StateAvailable, state, model.ConfidenceMedium)}, Diagnostics: []model.Diagnostic{{Level: "warn", Code: "docker_version", Message: err.Error()}}}
	}
	state.DaemonState = model.StateAvailable
	var parsedVersion versionResponse
	if json.Unmarshal([]byte(version.Stdout), &parsedVersion) == nil {
		if parsedVersion.Server != nil {
			state.EngineVersion = parsedVersion.Server.Version
		} else {
			state.EngineVersion = parsedVersion.Client.Version
		}
	}
	info, infoErr := env.Runner.Run(ctx, execx.CommandSpec{Name: "docker", Args: []string{"info", "--format", "{{json .}}"}, Timeout: 3 * time.Second, OutputLimit: 512 << 10})
	if infoErr == nil {
		var raw map[string]any
		if json.Unmarshal([]byte(info.Stdout), &raw) == nil {
			if value, ok := raw["DefaultRuntime"].(string); ok {
				state.DefaultRuntime = value
			}
			if runtimes, ok := raw["Runtimes"].(map[string]any); ok {
				_, state.NVIDIARuntime = runtimes["nvidia"]
			}
			if value, ok := raw["CgroupVersion"].(string); ok {
				state.CgroupVersion = value
			}
			if options, ok := raw["SecurityOptions"].([]any); ok {
				rootless := false
				for _, option := range options {
					if strings.Contains(strings.ToLower(fmt.Sprint(option)), "rootless") {
						rootless = true
					}
				}
				state.Rootless = &rootless
			}
		}
	}
	ps, psErr := env.Runner.Run(ctx, execx.CommandSpec{Name: "docker", Args: []string{"ps", "--quiet"}, Timeout: 3 * time.Second, OutputLimit: 1 << 20})
	if psErr == nil {
		ids := strings.Fields(ps.Stdout)
		for _, id := range ids {
			inspected, inspectErr := env.Runner.Run(ctx, execx.CommandSpec{Name: "docker", Args: []string{"inspect", id}, Timeout: 3 * time.Second, OutputLimit: 2 << 20})
			if inspectErr != nil {
				continue
			}
			var rows []inspectResponse
			if json.Unmarshal([]byte(inspected.Stdout), &rows) != nil || len(rows) == 0 {
				continue
			}
			container := normalize(rows[0])
			enrichEffectiveSets(env, rows[0].State.PID, &container)
			state.Devices = append(state.Devices, container)
		}
	}
	return collector.Result{Collector: c.ID(), State: model.StateAvailable, Facts: []model.Fact{model.NewFact("containers", model.StateAvailable, state, model.ConfidenceHigh, model.SourceRef{Collector: "docker", Source: "docker version/info/ps/inspect"})}}
}

func normalize(raw inspectResponse) model.Container {
	c := model.Container{ID: shortID(raw.ID), Name: strings.TrimPrefix(raw.Name, "/"), Image: raw.Config.Image, Runtime: raw.HostConfig.Runtime, GPUUUIDs: []string{}}
	if raw.HostConfig.ShmSize >= 0 {
		value := uint64(raw.HostConfig.ShmSize)
		c.SHMSize = &value
	}
	if raw.HostConfig.Memory > 0 {
		value := uint64(raw.HostConfig.Memory)
		c.MemoryLimit = &value
	}
	for _, request := range raw.HostConfig.DeviceRequests {
		gpu := strings.EqualFold(request.Driver, "nvidia")
		for _, caps := range request.Capabilities {
			for _, capability := range caps {
				gpu = gpu || strings.EqualFold(capability, "gpu")
			}
		}
		if gpu {
			c.GPURequired = true
			c.GPUAccess = request.Count != 0 || len(request.DeviceIDs) > 0
			c.GPUUUIDs = append(c.GPUUUIDs, request.DeviceIDs...)
		}
	}
	for _, item := range raw.HostConfig.Ulimits {
		if item.Name == "memlock" {
			soft, hard := uint64(max(item.Soft, 0)), uint64(max(item.Hard, 0))
			c.MemlockSoft = &soft
			c.MemlockHard = &hard
		}
	}
	c.EffectiveCPUs = parseList(raw.HostConfig.CpusetCpus)
	c.EffectiveMems = parseList(raw.HostConfig.CpusetMems)
	return c
}
func shortID(value string) string {
	if len(value) > 12 {
		return value[:12]
	}
	return value
}
func enrichEffectiveSets(env collector.Env, pid int, container *model.Container) {
	if pid <= 0 || env.FileSystem == nil {
		return
	}
	raw, err := env.FileSystem.ReadFile(fmt.Sprintf("/proc/%d/status", pid))
	if err != nil {
		return
	}
	for _, line := range strings.Split(string(raw), "\n") {
		key, value, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		switch strings.TrimSpace(key) {
		case "Cpus_allowed_list":
			if parsed := parseList(strings.TrimSpace(value)); len(parsed) > 0 {
				container.EffectiveCPUs = parsed
			}
		case "Mems_allowed_list":
			if parsed := parseList(strings.TrimSpace(value)); len(parsed) > 0 {
				container.EffectiveMems = parsed
			}
		}
	}
}
func classify(result execx.Result, err error) model.FactState {
	if result.TimedOut {
		return model.StateTimeout
	}
	message := strings.ToLower(err.Error() + " " + result.Stderr)
	if strings.Contains(message, "permission denied") || strings.Contains(message, "access is denied") {
		return model.StatePermissionDenied
	}
	if strings.Contains(message, "executable file not found") || strings.Contains(message, "not found") || strings.Contains(message, "cannot find") {
		return model.StateNotDetected
	}
	if strings.Contains(message, "cannot connect") || strings.Contains(message, "daemon") {
		return model.StateNotDetected
	}
	return model.StateUnknown
}
func parseList(raw string) []int {
	result := []int{}
	for _, part := range strings.Split(raw, ",") {
		if part == "" {
			continue
		}
		var start, end int
		if _, err := fmt.Sscanf(part, "%d-%d", &start, &end); err == nil {
			for value := start; value <= end; value++ {
				result = append(result, value)
			}
			continue
		}
		if _, err := fmt.Sscanf(part, "%d", &start); err == nil {
			result = append(result, start)
		}
	}
	return result
}

func firstVersion(raw string) string {
	for _, field := range strings.Fields(raw) {
		trimmed := strings.Trim(field, "v,;()[]")
		if len(trimmed) > 0 && trimmed[0] >= '0' && trimmed[0] <= '9' && strings.Contains(trimmed, ".") {
			return trimmed
		}
	}
	return ""
}
