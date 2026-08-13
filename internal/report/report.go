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
	if err := Validate(value); err != nil {
		return fmt.Errorf("invalid report: %w", err)
	}
	if format == FormatJSON {
		encoder := json.NewEncoder(w)
		encoder.SetIndent("", "  ")
		return encoder.Encode(value)
	}
	return writeHuman(w, value)
}
func writeHuman(w io.Writer, value model.Report) error {
	if _, err := fmt.Fprintf(w, "AIStat %s\n\nNVIDIA AI Node Check\n\nDeployment Readiness    %s\nPerformance Readiness   %s\n\n%d blocker(s), %d warning(s), %d unknown\n", value.AIStatVersion, strings.ToUpper(value.Readiness.Deployment), strings.ToUpper(value.Readiness.Performance), value.Summary.Fail, value.Summary.Warn, value.Summary.Unknown); err != nil {
		return err
	}
	findings := append([]model.Finding(nil), value.Findings...)
	sort.SliceStable(findings, func(i, j int) bool { return findings[i].RuleID < findings[j].RuleID })
	for _, finding := range findings {
		if finding.Status != model.StatusFail && finding.Status != model.StatusWarn && finding.Status != model.StatusUnknown {
			continue
		}
		if _, err := fmt.Fprintf(w, "\n%s %s — %s\n%s\n", strings.ToUpper(string(finding.Status)), finding.RuleID, finding.Title, finding.CurrentState); err != nil {
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
func WriteInfo(w io.Writer, format Format, value model.Report) error {
	if value.Node == nil {
		return errors.New("report node is nil")
	}
	if format == FormatJSON {
		encoder := json.NewEncoder(w)
		encoder.SetIndent("", "  ")
		return encoder.Encode(value.Node)
	}
	s := *value.Node
	_, err := fmt.Fprintf(w, "Host: %s %s/%s\nCPU: %s (%d logical cores)\nMemory: %.1f GiB\nNUMA nodes: %d\nGPUs: %d\nNICs: %d\n", s.Host.Distro, s.Meta.OS, s.Meta.Arch, s.CPU.Model, s.CPU.LogicalCores, float64(s.Memory.TotalBytes)/(1<<30), len(s.NUMA.Nodes), len(s.GPUs.Devices), len(s.Network.NICs))
	return err
}
func WriteStack(w io.Writer, format Format, value model.Report) error {
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
	_, err := fmt.Fprintf(w, "NVIDIA driver: %s\nCUDA driver: %s\nCUDA toolkit: %s\nNCCL: %s\nDocker: %s %s\nRuntime instances: %d\n", empty(s.NVIDIA.DriverVersion), empty(s.NVIDIA.CUDADriver), empty(s.NVIDIA.CUDAToolkit), empty(s.NVIDIA.NCCLVersion), empty(s.Containers.Engine), empty(s.Containers.EngineVersion), len(s.Runtimes.Instances))
	return err
}
func WriteRuntime(w io.Writer, format Format, value model.Report) error {
	if value.Node == nil {
		return errors.New("report node is nil")
	}
	if format == FormatJSON {
		encoder := json.NewEncoder(w)
		encoder.SetIndent("", "  ")
		return encoder.Encode(value.Node.Runtimes)
	}
	if len(value.Node.Runtimes.Instances) == 0 {
		_, err := fmt.Fprintln(w, "No active supported AI runtime detected.")
		return err
	}
	for _, runtime := range value.Node.Runtimes.Instances {
		fmt.Fprintf(w, "%s pid=%d gpus=%s cpu_count=%d\n", runtime.Kind, runtime.PID, strings.Join(runtime.GPUs, ","), len(runtime.CPUSet))
	}
	return nil
}
func empty(value string) string {
	if value == "" {
		return "unknown"
	}
	return value
}
