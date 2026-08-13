package app

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/arpingblue/AIStat/internal/clock"
	basecollector "github.com/arpingblue/AIStat/internal/collector"
	"github.com/arpingblue/AIStat/internal/collector/cpu"
	"github.com/arpingblue/AIStat/internal/collector/docker"
	"github.com/arpingblue/AIStat/internal/collector/memory"
	"github.com/arpingblue/AIStat/internal/collector/network"
	"github.com/arpingblue/AIStat/internal/collector/numa"
	"github.com/arpingblue/AIStat/internal/collector/nvidia"
	"github.com/arpingblue/AIStat/internal/collector/pci"
	processcollector "github.com/arpingblue/AIStat/internal/collector/process"
	pythoncollector "github.com/arpingblue/AIStat/internal/collector/python"
	"github.com/arpingblue/AIStat/internal/collector/storage"
	systemcollector "github.com/arpingblue/AIStat/internal/collector/system"
	"github.com/arpingblue/AIStat/internal/compat"
	"github.com/arpingblue/AIStat/internal/execx"
	"github.com/arpingblue/AIStat/internal/fsx"
	"github.com/arpingblue/AIStat/internal/model"
	"github.com/arpingblue/AIStat/internal/normalize"
	"github.com/arpingblue/AIStat/internal/report"
	"github.com/arpingblue/AIStat/internal/rules"
	"github.com/arpingblue/AIStat/internal/topology"
	"github.com/arpingblue/AIStat/internal/version"
)

const usageText = `AIStat helps AI infrastructure engineers prepare and optimize Linux NVIDIA nodes for large-model deployment and high-performance inference.
It unifies hardware topology, the CUDA stack, containers, and inference runtimes to expose deployment blockers and performance bottlenecks, then provides verifiable optimization guidance.

Usage:
  aistat status [options]
  aistat check [options]
  aistat info [options]
  aistat topology [options]
  aistat stack [options]
  aistat runtime [options]
  aistat explain RULE_ID [--format human|json]
  aistat version [--format human|json]

Command roles:
  status      fast operator overview (also the default command)
  check       detailed findings, evidence, and recommendations
  info        hardware inventory
  topology    compact NUMA/GPU/NIC topology
  stack       NVIDIA and container software stack
  runtime     PyTorch, vLLM, and SGLang installations and instances

Options:
  --format human|json
  --profile general|llm-inference
  --timeout 10s
  --no-color
  --fail-on fail|warn
`

type options struct {
	format  string
	profile string
	timeout time.Duration
	noColor bool
	failOn  string
	view    string
}

func Run(args []string, stdout, stderr io.Writer) int {
	return RunContext(context.Background(), args, stdout, stderr)
}

func RunContext(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		args = []string{"status"}
	}
	command := args[0]
	if command == "help" || command == "-h" || command == "--help" {
		fmt.Fprint(stdout, usageText)
		return 0
	}
	if command == "version" {
		return runVersion(args[1:], stdout, stderr)
	}
	if command == "explain" {
		return runExplain(args[1:], stdout, stderr)
	}
	if command != "status" && command != "check" && command != "info" && command != "topology" && command != "stack" && command != "runtime" {
		fmt.Fprintf(stderr, "unknown command %q\n\n%s", command, usageText)
		return 2
	}
	opts, err := parseOptions(command, args[1:], stderr)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	format, err := report.ParseFormat(opts.format)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	profile, err := profileFor(opts.profile)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	ctx, cancel := context.WithTimeout(ctx, opts.timeout)
	defer cancel()
	value, graph, err := inspect(ctx, profile)
	if err != nil {
		fmt.Fprintln(stderr, "inspection failed:", err)
		return 2
	}
	reportOptions := report.Options{Color: colorEnabled(stdout, opts.noColor, format)}
	switch command {
	case "status":
		err = report.WriteStatus(stdout, format, value, reportOptions)
	case "check":
		err = report.WriteWithOptions(stdout, format, value, reportOptions)
	case "info":
		err = report.WriteInfoWithOptions(stdout, format, value, reportOptions)
	case "stack":
		err = report.WriteStackWithOptions(stdout, format, value, reportOptions)
	case "runtime":
		err = report.WriteRuntimeWithOptions(stdout, format, value, reportOptions)
	case "topology":
		err = writeTopology(stdout, format, value, graph, opts.view, reportOptions)
	}
	if err != nil {
		fmt.Fprintln(stderr, "write report:", err)
		return 2
	}
	if command == "check" || command == "status" {
		if value.Summary.Fail > 0 {
			return 1
		}
		if opts.failOn == "warn" && value.Summary.Warn > 0 {
			return 1
		}
	}
	return 0
}

func parseOptions(command string, args []string, stderr io.Writer) (options, error) {
	opts := options{format: "human", profile: "llm-inference", timeout: 10 * time.Second, failOn: "fail", view: "tree"}
	set := flag.NewFlagSet(command, flag.ContinueOnError)
	set.SetOutput(stderr)
	set.StringVar(&opts.format, "format", opts.format, "human or json")
	set.StringVar(&opts.profile, "profile", opts.profile, "general or llm-inference")
	set.DurationVar(&opts.timeout, "timeout", opts.timeout, "overall inspection timeout")
	set.BoolVar(&opts.noColor, "no-color", false, "disable color output")
	set.StringVar(&opts.failOn, "fail-on", opts.failOn, "fail or warn")
	if command == "topology" {
		set.StringVar(&opts.view, "view", opts.view, "tree, gpu, or gpu-nic")
	}
	if err := set.Parse(args); err != nil {
		return opts, err
	}
	if set.NArg() != 0 {
		return opts, fmt.Errorf("unexpected arguments: %s", strings.Join(set.Args(), " "))
	}
	if opts.timeout <= 0 {
		return opts, errors.New("timeout must be positive")
	}
	if opts.failOn != "fail" && opts.failOn != "warn" {
		return opts, errors.New("fail-on must be fail or warn")
	}
	if command == "topology" && opts.view != "tree" && opts.view != "gpu" && opts.view != "gpu-nic" {
		return opts, fmt.Errorf("view must be tree, gpu, or gpu-nic (got %q)", opts.view)
	}
	return opts, nil
}
func profileFor(name string) (model.Profile, error) {
	switch name {
	case "general":
		return model.Profile{Name: name}, nil
	case "llm-inference":
		return model.Profile{Name: name, GPURequired: true}, nil
	default:
		return model.Profile{}, fmt.Errorf("unknown profile %q", name)
	}
}

func inspect(ctx context.Context, profile model.Profile) (model.Report, *topology.Graph, error) {
	now := time.Now().UTC()
	registry, err := basecollector.NewRegistry(systemcollector.Collector{}, cpu.Collector{}, memory.Collector{}, numa.Collector{}, pci.Collector{}, network.Collector{}, storage.Collector{}, nvidia.Collector{}, processcollector.Collector{}, docker.Collector{}, pythoncollector.Collector{})
	if err != nil {
		return model.Report{}, nil, err
	}
	home, _ := os.UserHomeDir()
	environment := map[string]string{}
	for _, key := range []string{"PATH", "CONDA_PREFIX", "VIRTUAL_ENV"} {
		environment[key] = os.Getenv(key)
	}
	env := basecollector.Env{Runner: execx.SafeRunner{Resolver: execx.NewResolver("nvidia-smi", "docker", "python3", "nvidia-ctk", "nvidia-container-cli", "nvidia-container-runtime", "dpkg-query", "rpm", "dmesg", "lspci")}, FileSystem: fsx.OS{}, Clock: clock.Real{}, Platform: runtime.GOOS, HomeDir: home, Environment: environment}
	results, statuses, err := registry.Run(ctx, env)
	if err != nil {
		return model.Report{}, nil, err
	}
	snapshot := normalize.Empty(now, version.Version, profile.Name)
	snapshot.Meta.Hostname, _ = os.Hostname()
	snapshot, err = normalize.Results(snapshot, results, statuses)
	if err != nil {
		return model.Report{}, nil, err
	}
	snapshot.Meta.DurationMS = time.Since(now).Milliseconds()
	base := topology.BuildBase(&snapshot)
	graph := topology.Enrich(base, &snapshot)
	profile = resolveProfile(profile, &snapshot)
	engine := rules.Default()
	findings, readiness, summary := engine.Evaluate(rules.RuleContext{Snapshot: &snapshot, Graph: graph, Profile: profile, Now: now})
	value := model.Report{SchemaVersion: "0.1", AIStatVersion: version.Version, CollectedAt: now, Profile: profile.Name, CompatibilityVersion: compat.DatasetVersion, Node: &snapshot, Findings: findings, Readiness: readiness, Summary: summary}
	return value, graph, nil
}

func resolveProfile(profile model.Profile, snapshot *model.Snapshot) model.Profile {
	for _, process := range snapshot.Processes.Processes {
		if process.ContainerID != "" {
			profile.DockerRequired = true
		}
	}
	for _, container := range snapshot.Containers.Devices {
		if container.GPURequired || strings.EqualFold(container.Runtime, "nvidia") {
			profile.DockerRequired = true
			break
		}
	}
	for _, runtime := range snapshot.Runtimes.Instances {
		if len(runtime.GPUs) > 0 || runtime.CUDAVersion != "" {
			profile.GPURequired = true
		}
		if runtime.LocalWorldSize != nil && *runtime.LocalWorldSize > 1 {
			profile.MultiProcess = true
		}
		if len(runtime.SelectedHCAs) > 0 || runtime.Details["NCCL_IB_HCA"] != "" {
			profile.RDMARequired = true
		}
	}
	return profile
}

func runVersion(args []string, stdout, stderr io.Writer) int {
	format := "human"
	set := flag.NewFlagSet("version", flag.ContinueOnError)
	set.SetOutput(stderr)
	set.StringVar(&format, "format", format, "human or json")
	if err := set.Parse(args); err != nil {
		return 2
	}
	if format == "json" {
		_ = json.NewEncoder(stdout).Encode(map[string]string{"version": version.Version, "commit": version.Commit, "date": version.Date})
		return 0
	}
	if format != "human" {
		fmt.Fprintln(stderr, "format must be human or json")
		return 2
	}
	fmt.Fprintf(stdout, "aistat %s (commit %s, built %s)\n", version.Version, version.Commit, version.Date)
	return 0
}
func runExplain(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "explain requires RULE_ID")
		return 2
	}
	id := strings.ToUpper(args[0])
	format := "human"
	set := flag.NewFlagSet("explain", flag.ContinueOnError)
	set.SetOutput(stderr)
	set.StringVar(&format, "format", format, "human or json")
	if err := set.Parse(args[1:]); err != nil {
		return 2
	}
	for _, item := range rules.Default().Rules() {
		if string(item.ID()) != id {
			continue
		}
		meta := item.Meta()
		if format == "json" {
			_ = json.NewEncoder(stdout).Encode(struct {
				ID   string         `json:"id"`
				Meta rules.RuleMeta `json:"meta"`
			}{id, meta})
			return 0
		}
		fmt.Fprintf(stdout, "%s — %s\n\nDomain: %s\nDimension: %s\nPriority: %s\nConfidence: %s\n%s\n", id, meta.Title, meta.Domain, meta.Dimension, meta.Priority, meta.Confidence, meta.Description)
		for _, ref := range meta.References {
			fmt.Fprintf(stdout, "Reference: %s — %s\n", ref.Title, ref.URL)
		}
		return 0
	}
	fmt.Fprintf(stderr, "unknown rule %q\n", id)
	return 2
}
func writeTopology(w io.Writer, format report.Format, value model.Report, graph *topology.Graph, view string, options report.Options) error {
	if format == report.FormatJSON {
		encoder := json.NewEncoder(w)
		encoder.SetIndent("", "  ")
		return encoder.Encode(graph)
	}
	valid := map[string]bool{"tree": true, "gpu": true, "gpu-nic": true}
	if !valid[view] {
		return fmt.Errorf("unsupported topology view %q", view)
	}
	if value.Node == nil {
		return errors.New("report node is nil")
	}
	snapshot := *value.Node
	fmt.Fprintf(w, "Topology — %s\n", emptyText(snapshot.Meta.Hostname))
	numaIDs := make([]int, 0, len(snapshot.NUMA.Nodes))
	for _, node := range snapshot.NUMA.Nodes {
		numaIDs = append(numaIDs, node.ID)
	}
	sort.Ints(numaIDs)
	unknownGPUs := []model.GPU{}
	for _, gpu := range sortedGPUs(snapshot.GPUs.Devices) {
		if gpu.NUMANode == nil {
			unknownGPUs = append(unknownGPUs, gpu)
		}
	}
	unknownNICs := []model.NIC{}
	if view != "gpu" {
		for _, nic := range physicalNICs(snapshot.Network.NICs) {
			if nic.NUMANode == nil {
				unknownNICs = append(unknownNICs, nic)
			}
		}
	}
	hasUnknown := len(unknownGPUs)+len(unknownNICs) > 0
	for position, numaID := range numaIDs {
		branch, indent := "├──", "│   "
		if position == len(numaIDs)-1 && !hasUnknown {
			branch, indent = "└──", "    "
		}
		fmt.Fprintf(w, "%s NUMA %d\n", branch, numaID)
		if view == "tree" {
			for _, node := range snapshot.NUMA.Nodes {
				if node.ID == numaID {
					fmt.Fprintf(w, "%s├── CPUs: %s\n", indent, report.CompressCPUSet(node.CPUList))
					break
				}
			}
		}
		for _, gpu := range sortedGPUs(snapshot.GPUs.Devices) {
			if gpu.NUMANode != nil && *gpu.NUMANode == numaID {
				fmt.Fprintf(w, "%s├── GPU %d: %s  PCI %s  %.1f GiB\n", indent, gpu.Index, emptyText(gpu.Name), emptyText(gpu.PCIAddress), float64(gpu.MemoryTotalBytes)/(1<<30))
			}
		}
		if view != "gpu" {
			for _, nic := range physicalNICs(snapshot.Network.NICs) {
				if nic.NUMANode != nil && *nic.NUMANode == numaID {
					fmt.Fprintf(w, "%s├── NIC %s: state=%s PCI=%s speed=%d Mbps\n", indent, nic.Name, emptyText(nic.OperState), emptyText(nic.PCIAddress), nic.SpeedMbps)
				}
			}
			for _, rdma := range snapshot.RDMA.Devices {
				if rdma.NUMANode != nil && *rdma.NUMANode == numaID {
					fmt.Fprintf(w, "%s└── RDMA %s: netdev=%s state=%s\n", indent, rdma.Name, emptyText(rdma.NetDevice), emptyText(rdma.State))
				}
			}
		}
	}
	if hasUnknown {
		fmt.Fprintln(w, "└── Unassigned / Unknown NUMA")
		for _, gpu := range unknownGPUs {
			fmt.Fprintf(w, "    ├── GPU %d: %s  PCI %s\n", gpu.Index, emptyText(gpu.Name), emptyText(gpu.PCIAddress))
		}
		for _, nic := range unknownNICs {
			fmt.Fprintf(w, "    └── NIC %s: state=%s PCI=%s\n", nic.Name, emptyText(nic.OperState), emptyText(nic.PCIAddress))
		}
	}
	writeP2PMatrix(w, snapshot.GPUs.Devices, snapshot.P2P)
	_ = graph
	_ = options
	return nil
}

func writeP2PMatrix(w io.Writer, gpus []model.GPU, links []model.P2PLink) {
	gpus = sortedGPUs(gpus)
	if len(gpus) < 2 {
		return
	}
	index := map[string]int{}
	for _, gpu := range gpus {
		index[gpu.UUID] = gpu.Index
		if gpu.UUID == "" {
			index[fmt.Sprint(gpu.Index)] = gpu.Index
		}
	}
	matrix := map[[2]int]string{}
	for _, link := range links {
		from, fromOK := index[link.FromGPU]
		to, toOK := index[link.ToGPU]
		if !fromOK || !toOK {
			continue
		}
		matrix[[2]int{from, to}], matrix[[2]int{to, from}] = link.Kind, link.Kind
	}
	fmt.Fprintln(w, "\nGPU P2P")
	fmt.Fprint(w, "       ")
	for _, gpu := range gpus {
		fmt.Fprintf(w, "%-7s", fmt.Sprintf("GPU%d", gpu.Index))
	}
	fmt.Fprintln(w)
	for _, from := range gpus {
		fmt.Fprintf(w, "GPU%-4d", from.Index)
		for _, to := range gpus {
			value := "X"
			if from.Index != to.Index {
				value = matrix[[2]int{from.Index, to.Index}]
				if value == "" {
					value = "?"
				}
			}
			fmt.Fprintf(w, "%-7s", value)
		}
		fmt.Fprintln(w)
	}
}

func sortedGPUs(values []model.GPU) []model.GPU {
	result := append([]model.GPU(nil), values...)
	sort.Slice(result, func(i, j int) bool { return result[i].Index < result[j].Index })
	return result
}
func physicalNICs(values []model.NIC) []model.NIC {
	result := []model.NIC{}
	for _, value := range values {
		if value.PCIAddress != "" {
			result = append(result, value)
		}
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Name < result[j].Name })
	return result
}
func emptyText(value string) string {
	if strings.TrimSpace(value) == "" {
		return "unknown"
	}
	return value
}

func colorEnabled(w io.Writer, disabled bool, format report.Format) bool {
	file, ok := w.(*os.File)
	if !ok {
		return colorEnabledFor(false, disabled, format, os.Getenv("NO_COLOR"))
	}
	info, err := file.Stat()
	return colorEnabledFor(err == nil && info.Mode()&os.ModeCharDevice != 0, disabled, format, os.Getenv("NO_COLOR"))
}

func colorEnabledFor(terminal, disabled bool, format report.Format, noColorEnvironment string) bool {
	return terminal && !disabled && format == report.FormatHuman && noColorEnvironment == ""
}
