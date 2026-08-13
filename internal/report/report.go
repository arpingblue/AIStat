package report

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/arpingblue/AIStat/internal/model"
)

type Format string

type Options struct {
	Color bool
}

const (
	FormatHuman Format = "human"
	FormatJSON  Format = "json"
)

func ParseFormat(value string) (Format, error) {
	switch Format(strings.ToLower(value)) {
	case FormatHuman:
		return FormatHuman, nil
	case FormatJSON:
		return FormatJSON, nil
	default:
		return "", fmt.Errorf("unsupported format %q (want human or json)", value)
	}
}
func Write(w io.Writer, format Format, value model.Report) error {
	return WriteWithOptions(w, format, value, Options{})
}

func WriteWithOptions(w io.Writer, format Format, value model.Report, options Options) error {
	if err := Validate(value); err != nil {
		return fmt.Errorf("invalid report: %w", err)
	}
	if format == FormatJSON {
		encoder := json.NewEncoder(w)
		encoder.SetIndent("", "  ")
		return encoder.Encode(value)
	}
	return writeHuman(w, value, options)
}
func writeHuman(w io.Writer, value model.Report, options Options) error {
	paint := painter{enabled: options.Color}
	if _, err := fmt.Fprintf(w, "AIStat %s\n\nNVIDIA AI Node Check\n\nDeployment Readiness    %s\nPerformance Readiness   %s\n\n%d blocker(s), %d warning(s), %d unknown\n", value.AIStatVersion, paint.readiness(value.Readiness.Deployment), paint.readiness(value.Readiness.Performance), value.Summary.Fail, value.Summary.Warn, value.Summary.Unknown); err != nil {
		return err
	}
	writeInspectionGaps(w, value, paint)
	findings := append([]model.Finding(nil), value.Findings...)
	sort.SliceStable(findings, func(i, j int) bool { return findings[i].RuleID < findings[j].RuleID })
	for _, finding := range findings {
		if finding.Status != model.StatusFail && finding.Status != model.StatusWarn && finding.Status != model.StatusUnknown {
			continue
		}
		if _, err := fmt.Fprintf(w, "\n%s %s — %s\n%s\n", paint.status(finding.Status), finding.RuleID, finding.Title, finding.CurrentState); err != nil {
			return err
		}
		if _, err := fmt.Fprintln(w, "Evidence:"); err != nil {
			return err
		}
		for _, evidence := range finding.Evidence {
			encoded, err := json.Marshal(evidence.Value)
			if err != nil {
				encoded = []byte(fmt.Sprint(evidence.Value))
			}
			if _, err := fmt.Fprintf(w, "  - %s=%s", evidence.Fact, encoded); err != nil {
				return err
			}
			if evidence.Source != "" {
				if _, err := fmt.Fprintf(w, " (%s)", evidence.Source); err != nil {
					return err
				}
			}
			if _, err := fmt.Fprintln(w); err != nil {
				return err
			}
		}
		if finding.Why != "" {
			if _, err := fmt.Fprintf(w, "Why: %s\n", finding.Why); err != nil {
				return err
			}
		}
		if finding.Impact != "" {
			if _, err := fmt.Fprintf(w, "Impact: %s\n", finding.Impact); err != nil {
				return err
			}
		}
		if finding.Recommendation != "" {
			if _, err := fmt.Fprintf(w, "Recommendation: %s\n", finding.Recommendation); err != nil {
				return err
			}
		}
		if len(finding.Verification) > 0 {
			if _, err := fmt.Fprintln(w, "Verification:"); err != nil {
				return err
			}
			for _, step := range finding.Verification {
				if _, err := fmt.Fprintf(w, "  - %s\n", step); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func WriteStatus(w io.Writer, format Format, value model.Report, options Options) error {
	if err := Validate(value); err != nil {
		return fmt.Errorf("invalid report: %w", err)
	}
	if format == FormatJSON {
		encoder := json.NewEncoder(w)
		encoder.SetIndent("", "  ")
		return encoder.Encode(value)
	}
	if value.Node == nil {
		return errors.New("report node is nil")
	}
	s, paint := *value.Node, painter{enabled: options.Color}
	fmt.Fprintf(w, "AIStat %s — Node Status\nHost: %s  %s/%s  Kernel %s\n\n", value.AIStatVersion, empty(s.Meta.Hostname), s.Meta.OS, s.Meta.Arch, empty(s.Host.Kernel))
	fmt.Fprintln(w, "Hardware")
	fmt.Fprintf(w, "  GPUs       %-18s %s\n", paint.fact(s.GPUs.State), gpuSummary(s.GPUs))
	fmt.Fprintf(w, "  CPU        %-18s %s, %d sockets, %d cores, %d logical\n", paint.fact(s.CPU.State), empty(s.CPU.Model), s.CPU.Sockets, s.CPU.PhysicalCores, s.CPU.LogicalCores)
	fmt.Fprintf(w, "  Memory     %-18s %.1f GiB\n", paint.fact(s.Memory.State), float64(s.Memory.TotalBytes)/(1<<30))
	fmt.Fprintf(w, "  NUMA       %-18s %d nodes\n", paint.fact(s.NUMA.State), len(s.NUMA.Nodes))
	fmt.Fprintf(w, "  GPU P2P    %-18s %d links\n", paint.fact(p2pState(s)), len(s.P2P))

	fmt.Fprintln(w, "\nNVIDIA Stack")
	fmt.Fprintf(w, "  Driver     %-18s %s\n", paint.boolFact(s.NVIDIA.DriverUsable, s.NVIDIA.State), empty(s.NVIDIA.DriverVersion))
	fmt.Fprintf(w, "  CUDA       %-18s driver capability %s; selected toolkit %s\n", paint.fact(s.NVIDIA.State), empty(s.NVIDIA.CUDADriver), empty(s.NVIDIA.CUDAToolkit))
	fmt.Fprintf(w, "  NCCL       %-18s %s\n", paint.detected(s.NVIDIA.NCCLVersion != "", s.NVIDIA.State), empty(s.NVIDIA.NCCLVersion))
	fmt.Fprintf(w, "  Xid log    %-18s %s\n", paint.fact(s.NVIDIA.XIDState), xidSummary(s.NVIDIA))

	fmt.Fprintln(w, "\nContainers")
	fmt.Fprintf(w, "  Client     %-18s %s %s\n", paint.fact(nonEmptyState(s.Containers.ClientState, s.Containers.State)), empty(s.Containers.Engine), empty(s.Containers.ClientVersion))
	fmt.Fprintf(w, "  Daemon     %-18s %s\n", paint.fact(s.Containers.DaemonState), dockerReason(s.Containers))
	fmt.Fprintf(w, "  NVIDIA CTK %-18s %s\n", toolkitLabel(paint, s.Containers), toolkitSummary(s.Containers))
	fmt.Fprintf(w, "  Docker GPU %-18s %s\n", gpuContainerLabel(paint, s.Containers), gpuContainerSummary(s.Containers))

	fmt.Fprintln(w, "\nAI Runtimes")
	for _, product := range s.Runtimes.Products {
		fmt.Fprintf(w, "  %-10s installed=%-18s running=%-18s instances=%d\n", displayProduct(product.Name), paint.fact(product.InstallationState), paint.fact(product.ExecutionState), product.InstanceCount)
		if product.InstallationState == model.StateUnknown || product.InstallationState == model.StatePermissionDenied || product.InstallationState == model.StateTimeout || product.InstallationState == model.StateParseError {
			fmt.Fprintf(w, "             install reason: %s\n", empty(product.InstallationReason))
		}
		if product.ExecutionState == model.StateUnknown || product.ExecutionState == model.StatePermissionDenied || product.ExecutionState == model.StateTimeout || product.ExecutionState == model.StateParseError {
			fmt.Fprintf(w, "             runtime reason: %s\n", empty(product.ExecutionReason))
		}
		for _, installation := range product.Installations {
			location := installation.Path
			if installation.ContainerID != "" {
				location = "container " + installation.ContainerID + ":" + location
			}
			fmt.Fprintf(w, "             %s  %s  %s\n", empty(installation.Version), installation.Scope, location)
		}
	}

	fmt.Fprintln(w, "\nReadiness")
	fmt.Fprintf(w, "  Deployment  %-18s %s\n", paint.readiness(value.Readiness.Deployment), readinessReason(value.Findings, model.DimensionDeployment))
	fmt.Fprintf(w, "  Performance %-18s %s\n", paint.readiness(value.Readiness.Performance), readinessReason(value.Findings, model.DimensionPerformance))

	fmt.Fprintln(w, "\nTop Actions")
	actions := actionableFindings(value.Findings)
	if len(actions) == 0 {
		fmt.Fprintln(w, "  No blocking, warning, or unknown findings.")
	}
	for index, finding := range actions {
		fmt.Fprintf(w, "  %d. %s %s — %s\n", index+1, paint.status(finding.Status), finding.RuleID, finding.CurrentState)
	}
	if s.Containers.DaemonState == model.StatePermissionDenied {
		fmt.Fprintln(w, "\nDocker inspection is incomplete: the current user cannot access the Docker API.")
		fmt.Fprintln(w, "Verify with: docker version")
		fmt.Fprintln(w, "Ask an administrator for an appropriate inspection method; Docker group access is effectively privileged.")
	}
	return nil
}
func WriteInfo(w io.Writer, format Format, value model.Report) error {
	return WriteInfoWithOptions(w, format, value, Options{})
}
func WriteInfoWithOptions(w io.Writer, format Format, value model.Report, options Options) error {
	if value.Node == nil {
		return errors.New("report node is nil")
	}
	if format == FormatJSON {
		encoder := json.NewEncoder(w)
		encoder.SetIndent("", "  ")
		return encoder.Encode(value.Node)
	}
	s := *value.Node
	paint := painter{enabled: options.Color}
	fmt.Fprintln(w, "Node Inventory")
	fmt.Fprintf(w, "  Host       %s  %s/%s  kernel=%s\n", empty(s.Meta.Hostname), s.Host.Distro, s.Meta.Arch, empty(s.Host.Kernel))
	fmt.Fprintf(w, "  CPU        %s  sockets=%d cores=%d logical=%d SMT=%s\n", empty(s.CPU.Model), s.CPU.Sockets, s.CPU.PhysicalCores, s.CPU.LogicalCores, boolPointer(s.CPU.SMT))
	fmt.Fprintf(w, "  Memory     %.1f GiB total  %.1f GiB free\n", float64(s.Memory.TotalBytes)/(1<<30), float64(s.Memory.FreeBytes)/(1<<30))
	fmt.Fprintf(w, "  NUMA       %s  nodes=%d\n", paint.fact(s.NUMA.State), len(s.NUMA.Nodes))
	for _, node := range s.NUMA.Nodes {
		fmt.Fprintf(w, "    node%-3d CPUs=%s memory=%.1f GiB\n", node.ID, CompressCPUSet(node.CPUList), float64(node.MemoryBytes)/(1<<30))
	}
	fmt.Fprintf(w, "  GPUs       %s  count=%d\n", paint.fact(s.GPUs.State), len(s.GPUs.Devices))
	for _, gpu := range s.GPUs.Devices {
		fmt.Fprintf(w, "    GPU%-3d %-22s PCI=%-14s NUMA=%s memory=%.1f GiB\n", gpu.Index, empty(gpu.Name), empty(gpu.PCIAddress), numaText(gpu.NUMANode), float64(gpu.MemoryTotalBytes)/(1<<30))
	}
	physical := physicalNICs(s.Network.NICs)
	fmt.Fprintf(w, "  Network    %s  physical NICs=%d RDMA=%d\n", paint.fact(s.Network.State), len(physical), len(s.RDMA.Devices))
	for _, nic := range physical {
		fmt.Fprintf(w, "    %-12s state=%-8s PCI=%-14s NUMA=%s speed=%d Mbps\n", nic.Name, empty(nic.OperState), empty(nic.PCIAddress), numaText(nic.NUMANode), nic.SpeedMbps)
	}
	fmt.Fprintf(w, "  Storage    %s  devices=%d mounts=%d\n", paint.fact(s.Storage.State), len(s.Storage.Devices), len(s.Storage.Mounts))
	return nil
}
func WriteStack(w io.Writer, format Format, value model.Report) error {
	return WriteStackWithOptions(w, format, value, Options{})
}
func WriteStackWithOptions(w io.Writer, format Format, value model.Report, options Options) error {
	if value.Node == nil {
		return errors.New("report node is nil")
	}
	if format == FormatJSON {
		encoder := json.NewEncoder(w)
		encoder.SetIndent("", "  ")
		return encoder.Encode(struct {
			NVIDIA     model.NVIDIAStack    `json:"nvidia"`
			Runtimes   model.RuntimeState   `json:"runtimes"`
			Containers model.ContainerState `json:"containers"`
		}{value.Node.NVIDIA, value.Node.Runtimes, value.Node.Containers})
	}
	s := *value.Node
	paint := painter{enabled: options.Color}
	fmt.Fprintln(w, "NVIDIA Software Stack")
	fmt.Fprintf(w, "  Driver              %-18s %s\n", paint.boolFact(s.NVIDIA.DriverUsable, s.NVIDIA.State), empty(s.NVIDIA.DriverVersion))
	fmt.Fprintf(w, "  Driver CUDA support %-18s %s\n", paint.fact(s.NVIDIA.State), empty(s.NVIDIA.CUDADriver))
	fmt.Fprintf(w, "  Selected toolkit    %-18s %s\n", paint.detected(s.NVIDIA.CUDAToolkit != "", s.NVIDIA.State), empty(s.NVIDIA.CUDAToolkit))
	if len(s.NVIDIA.CUDAToolkits) == 0 {
		fmt.Fprintln(w, "  Installed toolkits  NOT DETECTED")
	} else {
		fmt.Fprintf(w, "  Installed toolkits  %s\n", strings.Join(s.NVIDIA.CUDAToolkits, ", "))
	}
	fmt.Fprintf(w, "  NCCL                %-18s %s\n", paint.detected(s.NVIDIA.NCCLVersion != "", s.NVIDIA.State), empty(s.NVIDIA.NCCLVersion))
	fmt.Fprintln(w, "\nContainer Stack")
	fmt.Fprintf(w, "  Docker client       %-18s %s\n", paint.fact(nonEmptyState(s.Containers.ClientState, s.Containers.State)), empty(s.Containers.ClientVersion))
	fmt.Fprintf(w, "  Docker daemon       %-18s %s\n", paint.fact(s.Containers.DaemonState), dockerReason(s.Containers))
	fmt.Fprintf(w, "  NVIDIA CTK          %-18s %s\n", toolkitLabel(paint, s.Containers), toolkitSummary(s.Containers))
	fmt.Fprintf(w, "  Toolkit packages    %-18s %s\n", paint.fact(nonEmptyState(s.Containers.ToolkitPackageState, model.StateUnknown)), joinOrNone(s.Containers.ToolkitPackages))
	fmt.Fprintf(w, "  Toolkit commands    %-18s %s\n", paint.fact(nonEmptyState(s.Containers.ToolkitCLIState, model.StateUnknown)), toolkitCommandSummary(s.Containers))
	fmt.Fprintf(w, "  NVIDIA runtime      %-18s registered=%t\n", paint.fact(nonEmptyState(s.Containers.NVIDIARuntimeState, model.StateUnknown)), s.Containers.NVIDIARuntime)
	fmt.Fprintf(w, "  NVIDIA CDI          %-18s %s\n", paint.fact(nonEmptyState(s.Containers.CDIState, model.StateUnknown)), joinOrNone(s.Containers.CDISpecs))
	fmt.Fprintf(w, "  Docker GPU support  %-18s %s\n", gpuContainerLabel(paint, s.Containers), gpuContainerSummary(s.Containers))
	fmt.Fprintf(w, "  Default runtime     %s\n  cgroup              %s\n", empty(s.Containers.DefaultRuntime), empty(s.Containers.CgroupVersion))
	if s.Containers.DaemonState == model.StatePermissionDenied {
		fmt.Fprintln(w, "\nPermission guidance")
		fmt.Fprintln(w, "  The current user cannot inspect the Docker API, so containers, GPU mappings, shared memory, and toolkit configuration remain unknown.")
		fmt.Fprintln(w, "  Verify: docker version")
		fmt.Fprintln(w, "  Ask an administrator for an approved inspection path; Docker group membership is effectively privileged.")
	}
	return nil
}
func WriteRuntime(w io.Writer, format Format, value model.Report) error {
	return WriteRuntimeWithOptions(w, format, value, Options{})
}
func WriteRuntimeWithOptions(w io.Writer, format Format, value model.Report, options Options) error {
	if value.Node == nil {
		return errors.New("report node is nil")
	}
	if format == FormatJSON {
		encoder := json.NewEncoder(w)
		encoder.SetIndent("", "  ")
		return encoder.Encode(value.Node.Runtimes)
	}
	paint := painter{enabled: options.Color}
	for _, product := range value.Node.Runtimes.Products {
		fmt.Fprintf(w, "%s\n", displayProduct(product.Name))
		fmt.Fprintf(w, "  Installation  %s\n  Execution     %s\n  Host          %s\n  Containers    %s\n  Instances     %d\n", paint.fact(product.InstallationState), paint.fact(product.ExecutionState), paint.fact(product.HostState), paint.fact(product.ContainerState), product.InstanceCount)
		fmt.Fprintf(w, "  Install why   %s\n  Execution why %s\n", empty(product.InstallationReason), empty(product.ExecutionReason))
		for _, installation := range product.Installations {
			container := ""
			if installation.ContainerID != "" {
				container = " container=" + installation.ContainerID
			}
			fmt.Fprintf(w, "  - version=%s scope=%s%s env=%s path=%s\n", empty(installation.Version), installation.Scope, container, empty(installation.PythonEnvironment), installation.Path)
		}
		for _, runtime := range value.Node.Runtimes.Instances {
			if runtime.Kind != product.Name {
				continue
			}
			fmt.Fprintf(w, "  - pid=%d version=%s pytorch=%s GPUs=%s CPU=%s NUMA=%s TP=%s PP=%s DP=%s\n", runtime.PID, empty(runtime.Version), empty(runtime.PyTorchVersion), joinOrUnknown(runtime.GPUs), CompressCPUSet(runtime.CPUSet), CompressCPUSet(runtime.NUMAMems), intPointer(runtime.TensorParallel), intPointer(runtime.PipelineParallel), intPointer(runtime.DataParallel))
		}
		fmt.Fprintln(w)
	}
	return nil
}
func empty(value string) string {
	if value == "" {
		return "unknown"
	}
	return value
}

type painter struct{ enabled bool }

const (
	ansiReset     = "\x1b[0m"
	ansiRed       = "\x1b[31m"
	ansiBrightRed = "\x1b[91m"
	ansiGreen     = "\x1b[32m"
	ansiYellow    = "\x1b[33m"
	ansiCyan      = "\x1b[36m"
	ansiGray      = "\x1b[90m"
)

func (p painter) color(value, code string) string {
	if !p.enabled {
		return value
	}
	return code + value + ansiReset
}
func (p painter) fact(state model.FactState) string {
	label := strings.ToUpper(strings.ReplaceAll(string(state), "_", " "))
	switch state {
	case model.StateAvailable:
		return p.color(label, ansiGreen)
	case model.StatePermissionDenied, model.StateParseError, model.StateTimeout, model.StateUnknown:
		return p.color(label, ansiBrightRed)
	case model.StateNotDetected, model.StateUnsupported:
		return p.color(label, ansiGray)
	default:
		return p.color("UNKNOWN", ansiBrightRed)
	}
}
func (p painter) status(status model.Status) string {
	label := strings.ToUpper(string(status))
	switch status {
	case model.StatusPass:
		return p.color(label, ansiGreen)
	case model.StatusFail:
		return p.color(label, ansiRed)
	case model.StatusWarn:
		return p.color(label, ansiYellow)
	case model.StatusUnknown:
		return p.color(label, ansiBrightRed)
	case model.StatusSkip:
		return p.color(label, ansiGray)
	default:
		return p.color(label, ansiCyan)
	}
}
func (p painter) readiness(value string) string {
	label := strings.ToUpper(strings.ReplaceAll(value, "_", " "))
	switch value {
	case "ready":
		return p.color(label, ansiGreen)
	case "warn":
		return p.color(label, ansiYellow)
	case "not_ready":
		return p.color(label, ansiRed)
	default:
		return p.color(label, ansiBrightRed)
	}
}
func (p painter) boolFact(value *bool, state model.FactState) string {
	if state != model.StateAvailable || value == nil {
		return p.fact(state)
	}
	if *value {
		return p.color("PASS", ansiGreen)
	}
	return p.color("FAIL", ansiRed)
}
func (p painter) detected(found bool, state model.FactState) string {
	if state != model.StateAvailable {
		return p.fact(state)
	}
	if found {
		return p.color("DETECTED", ansiGreen)
	}
	return p.color("NOT DETECTED", ansiGray)
}
func writeInspectionGaps(w io.Writer, value model.Report, paint painter) {
	if value.Node == nil {
		return
	}
	gaps := []string{}
	for _, status := range value.Node.Collectors {
		if status.State == model.StatePermissionDenied || status.State == model.StateTimeout || status.State == model.StateParseError || status.State == model.StateUnknown {
			message := strings.TrimSpace(status.Error)
			if message == "" {
				message = strings.ReplaceAll(string(status.State), "_", " ")
			}
			gaps = append(gaps, fmt.Sprintf("%s: %s", status.ID, message))
		}
	}
	if len(gaps) == 0 {
		return
	}
	fmt.Fprintf(w, "\n%s\n", paint.color("Inspection Gaps", ansiBrightRed))
	for _, gap := range gaps {
		fmt.Fprintf(w, "  - %s\n", gap)
	}
}

func actionableFindings(findings []model.Finding) []model.Finding {
	result := []model.Finding{}
	for _, finding := range findings {
		if finding.Status == model.StatusFail || finding.Status == model.StatusWarn || finding.Status == model.StatusUnknown {
			result = append(result, finding)
		}
	}
	sort.SliceStable(result, func(i, j int) bool {
		rank := func(status model.Status) int {
			if status == model.StatusFail {
				return 0
			}
			if status == model.StatusWarn {
				return 1
			}
			return 2
		}
		if rank(result[i].Status) != rank(result[j].Status) {
			return rank(result[i].Status) < rank(result[j].Status)
		}
		return result[i].RuleID < result[j].RuleID
	})
	if len(result) > 5 {
		result = result[:5]
	}
	return result
}

func readinessReason(findings []model.Finding, dimension model.Dimension) string {
	failures, warnings, unknowns := 0, 0, 0
	for _, finding := range findings {
		if finding.Dimension != dimension {
			continue
		}
		switch finding.Status {
		case model.StatusFail:
			failures++
		case model.StatusWarn:
			warnings++
		case model.StatusUnknown:
			unknowns++
		}
	}
	parts := []string{}
	if failures > 0 {
		parts = append(parts, fmt.Sprintf("%d blocker(s)", failures))
	}
	if warnings > 0 {
		parts = append(parts, fmt.Sprintf("%d warning(s)", warnings))
	}
	if unknowns > 0 {
		parts = append(parts, fmt.Sprintf("%d evidence gap(s)", unknowns))
	}
	if len(parts) == 0 {
		return "no blocking findings"
	}
	return strings.Join(parts, ", ")
}

func gpuSummary(state model.GPUState) string {
	if len(state.Devices) == 0 {
		return "no NVIDIA GPU detected"
	}
	counts := map[string]int{}
	for _, gpu := range state.Devices {
		counts[gpu.Name]++
	}
	parts := []string{}
	for name, count := range counts {
		parts = append(parts, fmt.Sprintf("%d × %s", count, empty(name)))
	}
	sort.Strings(parts)
	return strings.Join(parts, ", ")
}
func p2pState(snapshot model.Snapshot) model.FactState {
	if len(snapshot.GPUs.Devices) < 2 {
		return model.StateNotDetected
	}
	expected := len(snapshot.GPUs.Devices) * (len(snapshot.GPUs.Devices) - 1) / 2
	if len(snapshot.P2P) == expected {
		return model.StateAvailable
	}
	return model.StateUnknown
}
func xidSummary(stack model.NVIDIAStack) string {
	if stack.XIDState != model.StateAvailable {
		return "kernel log could not be fully inspected"
	}
	if len(stack.XIDEvents) == 0 {
		return "no recent Xid events"
	}
	return fmt.Sprintf("%d recent event(s)", len(stack.XIDEvents))
}
func dockerReason(state model.ContainerState) string {
	switch state.DaemonState {
	case model.StateAvailable:
		return empty(state.EngineVersion)
	case model.StatePermissionDenied:
		return "current user cannot access the Docker API"
	case model.StateNotDetected:
		return "daemon not detected or unreachable"
	case model.StateTimeout:
		return "inspection timed out"
	default:
		return "state could not be determined"
	}
}
func toolkitState(state model.ContainerState) model.FactState {
	if state.ToolkitState != "" {
		return state.ToolkitState
	}
	if state.ToolkitDetected != nil {
		if *state.ToolkitDetected {
			return model.StateAvailable
		}
		return model.StateNotDetected
	}
	return model.StateUnknown
}
func toolkitLabel(paint painter, state model.ContainerState) string {
	value := toolkitState(state)
	if value == model.StateNotDetected {
		return paint.color("NOT INSTALLED", ansiRed)
	}
	return paint.fact(value)
}
func toolkitSummary(state model.ContainerState) string {
	switch toolkitState(state) {
	case model.StateAvailable:
		if state.ToolkitVersion != "" {
			return state.ToolkitVersion
		}
		if len(state.ToolkitEvidence) > 0 {
			return state.ToolkitEvidence[0]
		}
		return "installation evidence found"
	case model.StateNotDetected:
		return "no package, toolkit command, NVIDIA runtime, or standard CDI specification found"
	case model.StatePermissionDenied:
		return "installation evidence is permission-blocked; absence was not inferred"
	case model.StateTimeout:
		return "installation inspection timed out; absence was not inferred"
	case model.StateParseError:
		return "installation metadata could not be parsed"
	default:
		return "available evidence is insufficient to prove presence or absence"
	}
}
func gpuContainerState(state model.ContainerState) model.FactState {
	if state.GPUContainerState != "" {
		return state.GPUContainerState
	}
	if state.NVIDIARuntime {
		return model.StateAvailable
	}
	return model.StateUnknown
}
func gpuContainerLabel(paint painter, state model.ContainerState) string {
	value := gpuContainerState(state)
	if value == model.StateNotDetected {
		return paint.color("NOT CONFIGURED", ansiRed)
	}
	return paint.fact(value)
}
func gpuContainerSummary(state model.ContainerState) string {
	switch gpuContainerState(state) {
	case model.StateAvailable:
		if len(state.GPUContainerModes) > 0 {
			return "mode=" + strings.Join(state.GPUContainerModes, ",")
		}
		return "NVIDIA GPU integration path detected"
	case model.StateNotDetected:
		return "Docker exposes neither an NVIDIA runtime nor an NVIDIA CDI specification"
	case model.StatePermissionDenied:
		return "Docker GPU configuration is permission-blocked"
	case model.StateTimeout:
		return "Docker GPU configuration inspection timed out"
	case model.StateParseError:
		return "Docker GPU configuration could not be parsed"
	default:
		return "Docker GPU integration could not be conclusively determined"
	}
}
func toolkitCommandSummary(state model.ContainerState) string {
	values := []string{}
	for _, evidence := range state.ToolkitEvidence {
		if strings.Contains(evidence, " is executable") {
			values = append(values, strings.TrimSuffix(evidence, " is executable"))
		}
	}
	return joinOrNone(values)
}
func joinOrNone(values []string) string {
	if len(values) == 0 {
		return "none"
	}
	return strings.Join(values, ", ")
}
func nonEmptyState(value, fallback model.FactState) model.FactState {
	if value == "" {
		return fallback
	}
	return value
}
func displayProduct(value string) string {
	if value == "vllm" {
		return "vLLM"
	}
	if value == "sglang" {
		return "SGLang"
	}
	if value == "pytorch" {
		return "PyTorch"
	}
	return value
}
func boolPointer(value *bool) string {
	if value == nil {
		return "unknown"
	}
	if *value {
		return "enabled"
	}
	return "disabled"
}
func intPointer(value *int) string {
	if value == nil {
		return "unknown"
	}
	return fmt.Sprint(*value)
}
func numaText(value *int) string {
	if value == nil {
		return "unknown"
	}
	return fmt.Sprint(*value)
}
func joinOrUnknown(values []string) string {
	if len(values) == 0 {
		return "unknown"
	}
	return strings.Join(values, ",")
}
func physicalNICs(values []model.NIC) []model.NIC {
	result := []model.NIC{}
	for _, value := range values {
		if value.PCIAddress != "" {
			result = append(result, value)
		}
	}
	return result
}

func CompressCPUSet(values []int) string {
	if len(values) == 0 {
		return "unknown"
	}
	copyValues := append([]int(nil), values...)
	sort.Ints(copyValues)
	unique := copyValues[:0]
	for _, value := range copyValues {
		if len(unique) == 0 || unique[len(unique)-1] != value {
			unique = append(unique, value)
		}
	}
	parts := []string{}
	for index := 0; index < len(unique); {
		end := index
		for end+1 < len(unique) && unique[end+1] == unique[end]+1 {
			end++
		}
		if end == index {
			parts = append(parts, fmt.Sprint(unique[index]))
		} else {
			parts = append(parts, fmt.Sprintf("%d-%d", unique[index], unique[end]))
		}
		index = end + 1
	}
	return strings.Join(parts, ",")
}
