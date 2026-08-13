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

const usageText = `AIStat is a read-only NVIDIA AI node readiness inspector.

Usage:
  aistat check [options]
  aistat info [options]
  aistat topology [options]
  aistat stack [options]
  aistat runtime [options]
  aistat explain RULE_ID [--format human|json]
  aistat version [--format human|json]

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
		args = []string{"check"}
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
	if command != "check" && command != "info" && command != "topology" && command != "stack" && command != "runtime" {
		fmt.Fprintf(stderr, "unknown command %q\n\n%s", command, usageText)
		return 2
	}
	opts, err := parseOptions(command, args[1:], stderr)
	if err != nil {
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
	switch command {
	case "check":
		err = report.Write(stdout, format, value)
	case "info":
		err = report.WriteInfo(stdout, format, value)
	case "stack":
		err = report.WriteStack(stdout, format, value)
	case "runtime":
		err = report.WriteRuntime(stdout, format, value)
	case "topology":
		err = writeTopology(stdout, format, value, graph, opts.view)
	}
	if err != nil {
		fmt.Fprintln(stderr, "write report:", err)
		return 2
	}
	if command == "check" {
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
	env := basecollector.Env{Runner: execx.SafeRunner{Resolver: execx.NewResolver("nvidia-smi", "docker", "python3", "nvidia-ctk", "dmesg", "lspci")}, FileSystem: fsx.OS{}, Clock: clock.Real{}, Platform: runtime.GOOS}
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
	if len(snapshot.Containers.Devices) > 0 {
		profile.DockerRequired = true
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
func writeTopology(w io.Writer, format report.Format, value model.Report, graph *topology.Graph, view string) error {
	if format == report.FormatJSON {
		encoder := json.NewEncoder(w)
		encoder.SetIndent("", "  ")
		return encoder.Encode(graph)
	}
	valid := map[string]bool{"tree": true, "gpu": true, "gpu-nic": true}
	if !valid[view] {
		return fmt.Errorf("unsupported topology view %q", view)
	}
	ids := make([]string, 0, len(graph.Nodes))
	for id := range graph.Nodes {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	fmt.Fprintln(w, "Topology")
	for _, id := range ids {
		node := graph.Nodes[id]
		if view == "gpu" && node.Kind != topology.NodeGPU {
			continue
		}
		if view == "gpu-nic" && node.Kind != topology.NodeGPU && node.Kind != topology.NodeNIC && node.Kind != topology.NodeRDMA {
			continue
		}
		fmt.Fprintf(w, "%-16s %-12s %s\n", node.Kind, node.ID, node.Label)
	}
	fmt.Fprintln(w, "\nLinks")
	for _, edge := range graph.Edges {
		if view != "tree" && edge.Kind != topology.EdgeP2P && edge.Kind != topology.EdgeLocalTo {
			continue
		}
		fmt.Fprintf(w, "%s -> %s [%s]\n", edge.From, edge.To, edge.Kind)
	}
	_ = value
	return nil
}
