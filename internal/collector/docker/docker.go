package docker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"sort"
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
	state := model.ContainerState{
		State: model.StateAvailable, ClientState: model.StateUnknown, DaemonState: model.StateUnknown,
		NVIDIARuntimeState: model.StateUnknown, ToolkitState: model.StateUnknown,
		ToolkitPackageState: model.StateUnknown, ToolkitCLIState: model.StateUnknown,
		CDIState: model.StateUnknown, GPUContainerState: model.StateUnknown, Engine: "docker",
	}
	probeToolkitHost(ctx, env, &state)
	client, clientErr := env.Runner.Run(ctx, execx.CommandSpec{Name: "docker", Args: []string{"--version"}, Timeout: 2 * time.Second, OutputLimit: 64 << 10})
	if clientErr != nil {
		clientState := classify(client, clientErr)
		state.State = clientState
		state.ClientState = clientState
		state.DaemonState = model.StateUnknown
		if clientState == model.StateNotDetected {
			state.DaemonState = model.StateNotDetected
			state.NVIDIARuntimeState = model.StateNotDetected
			state.GPUContainerState = model.StateNotDetected
		}
		finalizeToolkitState(&state)
		return collector.Result{Collector: c.ID(), State: clientState, Facts: []model.Fact{model.NewFact("containers", model.StateAvailable, state, model.ConfidenceMedium)}, Diagnostics: []model.Diagnostic{{Level: "warn", Code: "docker_client", Message: clientErr.Error()}}}
	}
	state.ClientState = model.StateAvailable
	state.ClientVersion = firstVersion(client.Stdout)
	version, err := env.Runner.Run(ctx, execx.CommandSpec{Name: "docker", Args: []string{"version", "--format", "{{json .}}"}, Timeout: 3 * time.Second, OutputLimit: 256 << 10})
	if err != nil {
		commandState := classify(version, err)
		state.State = commandState
		state.DaemonState = commandState
		state.NVIDIARuntimeState = commandState
		state.GPUContainerState = commandState
		finalizeToolkitState(&state)
		return collector.Result{Collector: c.ID(), State: commandState, Facts: []model.Fact{model.NewFact("containers", model.StateAvailable, state, model.ConfidenceMedium)}, Diagnostics: []model.Diagnostic{{Level: "warn", Code: "docker_daemon", Message: err.Error()}}}
	}
	state.DaemonState = model.StateAvailable
	var parsedVersion versionResponse
	if json.Unmarshal([]byte(version.Stdout), &parsedVersion) == nil {
		if parsedVersion.Server != nil {
			state.EngineVersion = parsedVersion.Server.Version
		} else {
			state.EngineVersion = parsedVersion.Client.Version
		}
		if state.ClientVersion == "" {
			state.ClientVersion = parsedVersion.Client.Version
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
				if state.NVIDIARuntime {
					state.NVIDIARuntimeState = model.StateAvailable
					state.ToolkitEvidence = append(state.ToolkitEvidence, "Docker runtime 'nvidia' is registered")
					state.GPUContainerModes = append(state.GPUContainerModes, "nvidia-runtime")
				} else {
					state.NVIDIARuntimeState = model.StateNotDetected
				}
			}
			if state.NVIDIARuntimeState == model.StateUnknown {
				state.NVIDIARuntimeState = model.StateNotDetected
			}
			probeDockerCDIDirs(env, raw, &state)
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
		} else {
			state.NVIDIARuntimeState = model.StateParseError
		}
	} else {
		state.NVIDIARuntimeState = classify(info, infoErr)
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
			if container.GPURequired && container.GPUAccess {
				state.GPUContainerState = model.StateAvailable
				state.GPUContainerModes = append(state.GPUContainerModes, "active-gpu-container")
				state.ToolkitEvidence = append(state.ToolkitEvidence, "running container has an NVIDIA GPU device request")
			}
		}
	}
	finalizeToolkitState(&state)
	return collector.Result{Collector: c.ID(), State: model.StateAvailable, Facts: []model.Fact{model.NewFact("containers", model.StateAvailable, state, model.ConfidenceHigh, model.SourceRef{Collector: "docker", Source: "docker version/info/ps/inspect"})}}
}

var toolkitPackageNames = []string{
	"nvidia-container-toolkit",
	"nvidia-container-toolkit-base",
	"libnvidia-container-tools",
	"libnvidia-container1",
}

func probeToolkitHost(ctx context.Context, env collector.Env, state *model.ContainerState) {
	cliStates := []model.FactState{}
	ctkAvailable := false
	for _, name := range []string{"nvidia-ctk", "nvidia-container-cli", "nvidia-container-runtime"} {
		result, err := env.Runner.Run(ctx, execx.CommandSpec{Name: name, Args: []string{"--version"}, Timeout: 2 * time.Second, OutputLimit: 64 << 10})
		probeState := commandProbeState(result, err)
		cliStates = append(cliStates, probeState)
		if probeState != model.StateAvailable {
			continue
		}
		state.ToolkitEvidence = append(state.ToolkitEvidence, name+" is executable")
		if name == "nvidia-ctk" {
			ctkAvailable = true
		}
		if state.ToolkitVersion == "" && name != "nvidia-container-runtime" {
			state.ToolkitVersion = firstVersion(result.Stdout + " " + result.Stderr)
		}
	}
	state.ToolkitCLIState = combineProbeStates(cliStates...)
	state.ToolkitPackageState, state.ToolkitPackages = probeToolkitPackages(ctx, env)
	if state.ToolkitVersion == "" {
		for _, item := range state.ToolkitPackages {
			if _, version, ok := strings.Cut(item, "="); ok {
				state.ToolkitVersion = version
				break
			}
		}
	}
	for _, item := range state.ToolkitPackages {
		state.ToolkitEvidence = append(state.ToolkitEvidence, "installed package "+item)
	}
	state.CDIState, state.CDISpecs = scanCDIDirs(env, []string{"/etc/cdi", "/var/run/cdi"})
	if ctkAvailable {
		listed, err := env.Runner.Run(ctx, execx.CommandSpec{Name: "nvidia-ctk", Args: []string{"cdi", "list"}, Timeout: 2 * time.Second, OutputLimit: 256 << 10})
		listedState := commandProbeState(listed, err)
		if listedState == model.StateAvailable {
			names := cdiDeviceNames(listed.Stdout)
			if len(names) > 0 {
				state.CDIState = model.StateAvailable
				state.CDISpecs = append(state.CDISpecs, names...)
				state.ToolkitEvidence = append(state.ToolkitEvidence, "NVIDIA CDI devices are resolvable")
			} else if state.CDIState == model.StateUnknown {
				state.CDIState = model.StateNotDetected
			}
		} else if state.CDIState != model.StateAvailable {
			state.CDIState = combineProbeStates(state.CDIState, listedState)
		}
	}
}

func probeToolkitPackages(ctx context.Context, env collector.Env) (model.FactState, []string) {
	dpkgArgs := []string{"-W", "-f=${binary:Package}\t${Version}\t${db:Status-Abbrev}\n"}
	dpkgArgs = append(dpkgArgs, toolkitPackageNames...)
	if state, packages, checked := queryPackages(ctx, env, "dpkg-query", dpkgArgs, parseDPKGPackages); checked {
		return state, packages
	}
	rpmArgs := []string{"-q", "--qf", "%{NAME}\t%{VERSION}-%{RELEASE}\n"}
	rpmArgs = append(rpmArgs, toolkitPackageNames...)
	if state, packages, checked := queryPackages(ctx, env, "rpm", rpmArgs, parseRPMPackages); checked {
		return state, packages
	}
	return model.StateUnknown, nil
}

func queryPackages(ctx context.Context, env collector.Env, command string, args []string, parse func(string) []string) (model.FactState, []string, bool) {
	result, err := env.Runner.Run(ctx, execx.CommandSpec{Name: command, Args: args, Timeout: 2 * time.Second, OutputLimit: 256 << 10})
	state := commandProbeState(result, err)
	if state == model.StateNotDetected && result.ExitCode == 0 && result.Stdout == "" && result.Stderr == "" {
		return model.StateUnknown, nil, false
	}
	if strings.Contains(strings.ToLower(errorText(err)), "executable file not found") || strings.Contains(strings.ToLower(errorText(err)), "executable not found") {
		return model.StateUnknown, nil, false
	}
	if state == model.StatePermissionDenied || state == model.StateTimeout {
		return state, nil, true
	}
	packages := parse(result.Stdout)
	if len(packages) > 0 {
		return model.StateAvailable, packages, true
	}
	// A package manager that ran successfully (including its normal
	// "package is not installed" exit) is authoritative for that database.
	if err == nil || result.ExitCode > 0 {
		return model.StateNotDetected, nil, true
	}
	return model.StateUnknown, nil, true
}

func parseDPKGPackages(raw string) []string {
	packages := []string{}
	for _, line := range strings.Split(raw, "\n") {
		fields := strings.Split(strings.TrimSpace(line), "\t")
		if len(fields) >= 3 && strings.HasPrefix(strings.TrimSpace(fields[2]), "ii") {
			packages = append(packages, strings.TrimSpace(fields[0])+"="+strings.TrimSpace(fields[1]))
		}
	}
	return packages
}

func parseRPMPackages(raw string) []string {
	packages := []string{}
	for _, line := range strings.Split(raw, "\n") {
		fields := strings.Split(strings.TrimSpace(line), "\t")
		if len(fields) >= 2 && fields[0] != "" && fields[1] != "" {
			packages = append(packages, fields[0]+"="+fields[1])
		}
	}
	return packages
}

func probeDockerCDIDirs(env collector.Env, raw map[string]any, state *model.ContainerState) {
	dirs := []string{}
	if values, ok := raw["CDISpecDirs"].([]any); ok {
		for _, value := range values {
			if dir := strings.TrimSpace(fmt.Sprint(value)); dir != "" {
				dirs = append(dirs, dir)
			}
		}
	}
	if len(dirs) == 0 {
		return
	}
	probeState, specs := scanCDIDirs(env, dirs)
	state.CDIState = combineProbeStates(state.CDIState, probeState)
	state.CDISpecs = append(state.CDISpecs, specs...)
}

func scanCDIDirs(env collector.Env, dirs []string) (model.FactState, []string) {
	if env.FileSystem == nil {
		return model.StateUnknown, nil
	}
	states := []model.FactState{}
	specs := []string{}
	entriesSeen := 0
	for _, dir := range uniqueStrings(dirs) {
		entries, err := env.FileSystem.ReadDir(dir)
		if err != nil {
			switch {
			case errors.Is(err, fs.ErrNotExist):
				states = append(states, model.StateNotDetected)
			case errors.Is(err, fs.ErrPermission):
				states = append(states, model.StatePermissionDenied)
			default:
				states = append(states, model.StateUnknown)
			}
			continue
		}
		states = append(states, model.StateNotDetected)
		for _, entry := range entries {
			if entriesSeen >= 128 {
				states = append(states, model.StateUnknown)
				break
			}
			entriesSeen++
			if entry.IsDir() {
				continue
			}
			ext := strings.ToLower(filepath.Ext(entry.Name()))
			if ext != ".yaml" && ext != ".yml" && ext != ".json" {
				continue
			}
			path := filepath.ToSlash(filepath.Join(dir, entry.Name()))
			if info, statErr := env.FileSystem.Stat(path); statErr != nil || info.Size() > 256<<10 {
				states = append(states, model.StateUnknown)
				continue
			}
			raw, readErr := env.FileSystem.ReadFile(path)
			if readErr != nil {
				if errors.Is(readErr, fs.ErrPermission) {
					states = append(states, model.StatePermissionDenied)
				} else {
					states = append(states, model.StateUnknown)
				}
				continue
			}
			if strings.Contains(strings.ToLower(string(raw)), "nvidia.com/gpu") {
				states = append(states, model.StateAvailable)
				specs = append(specs, path)
			}
		}
	}
	return combineProbeStates(states...), uniqueStrings(specs)
}

func cdiDeviceNames(raw string) []string {
	result := []string{}
	for _, field := range strings.Fields(raw) {
		field = strings.Trim(field, " ,[]")
		if strings.HasPrefix(field, "nvidia.com/gpu=") {
			result = append(result, field)
		}
	}
	return uniqueStrings(result)
}

func commandProbeState(result execx.Result, err error) model.FactState {
	if err == nil {
		return model.StateAvailable
	}
	return classify(result, err)
}

func combineProbeStates(states ...model.FactState) model.FactState {
	if len(states) == 0 {
		return model.StateUnknown
	}
	allNotDetected := true
	for _, state := range states {
		if state == model.StateAvailable {
			return model.StateAvailable
		}
		allNotDetected = allNotDetected && state == model.StateNotDetected
	}
	if allNotDetected {
		return model.StateNotDetected
	}
	for _, preferred := range []model.FactState{model.StatePermissionDenied, model.StateTimeout, model.StateParseError, model.StateUnknown} {
		for _, state := range states {
			if state == preferred {
				return preferred
			}
		}
	}
	return model.StateUnknown
}

func finalizeToolkitState(state *model.ContainerState) {
	if state.GPUContainerState != model.StateAvailable {
		state.GPUContainerState = combineProbeStates(state.NVIDIARuntimeState, state.CDIState)
	}
	if state.GPUContainerState == model.StateAvailable {
		if state.NVIDIARuntimeState == model.StateAvailable {
			state.GPUContainerModes = append(state.GPUContainerModes, "nvidia-runtime")
		}
		if state.CDIState == model.StateAvailable {
			state.GPUContainerModes = append(state.GPUContainerModes, "cdi")
			state.ToolkitEvidence = append(state.ToolkitEvidence, "NVIDIA CDI specification is present")
		}
	}
	state.ToolkitState = combineProbeStates(state.ToolkitCLIState, state.ToolkitPackageState, state.NVIDIARuntimeState, state.CDIState)
	if state.ToolkitState == model.StateAvailable {
		detected := true
		state.ToolkitDetected = &detected
	} else if state.ToolkitState == model.StateNotDetected {
		detected := false
		state.ToolkitDetected = &detected
	} else {
		state.ToolkitDetected = nil
	}
	state.ToolkitPackages = uniqueStrings(state.ToolkitPackages)
	state.CDISpecs = uniqueStrings(state.CDISpecs)
	state.GPUContainerModes = uniqueStrings(state.GPUContainerModes)
	state.ToolkitEvidence = uniqueStrings(state.ToolkitEvidence)
}

func uniqueStrings(values []string) []string {
	seen := map[string]bool{}
	result := []string{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}

func normalize(raw inspectResponse) model.Container {
	c := model.Container{ID: shortID(raw.ID), InitPID: raw.State.PID, Name: strings.TrimPrefix(raw.Name, "/"), Image: raw.Config.Image, Runtime: raw.HostConfig.Runtime, GPUUUIDs: []string{}}
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
	message := strings.ToLower(errorText(err) + " " + result.Stderr)
	if strings.Contains(message, "permission denied") || strings.Contains(message, "not permitted") || strings.Contains(message, "access is denied") {
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

func errorText(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
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
