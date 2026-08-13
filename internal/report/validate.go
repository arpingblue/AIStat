package report

import (
	"errors"
	"fmt"
	"regexp"

	"github.com/arpingblue/AIStat/internal/model"
)

var ruleIDPattern = regexp.MustCompile(`^[A-Z]+[0-9]{3}$`)

func Validate(value model.Report) error {
	if value.SchemaVersion != "0.1" {
		return fmt.Errorf("schema_version must be 0.1")
	}
	if value.AIStatVersion == "" {
		return errors.New("aistat_version is required")
	}
	if value.CollectedAt.IsZero() {
		return errors.New("collected_at is required")
	}
	if value.Profile != "general" && value.Profile != "llm-inference" {
		return fmt.Errorf("invalid profile %q", value.Profile)
	}
	if value.CompatibilityVersion == "" {
		return errors.New("compatibility_version is required")
	}
	if value.Node == nil {
		return errors.New("node is required")
	}
	if value.Node.Meta.SchemaVersion != "0.1" || value.Node.Meta.ToolVersion == "" || value.Node.Meta.CollectedAt.IsZero() || value.Node.Meta.OS == "" || value.Node.Meta.Arch == "" {
		return errors.New("node meta is incomplete")
	}
	if !oneOf(value.Readiness.Deployment, "ready", "not_ready", "unknown") {
		return fmt.Errorf("invalid deployment readiness %q", value.Readiness.Deployment)
	}
	if !oneOf(value.Readiness.Performance, "ready", "warn", "unknown") {
		return fmt.Errorf("invalid performance readiness %q", value.Readiness.Performance)
	}
	states := []model.FactState{value.Node.Host.State, value.Node.CPU.State, value.Node.Memory.State, value.Node.NUMA.State, value.Node.PCI.State, value.Node.GPUs.State, value.Node.Network.State, value.Node.RDMA.State, value.Node.Storage.State, value.Node.NVIDIA.State, value.Node.NVIDIA.XIDState, value.Node.Containers.State, value.Node.Containers.DaemonState, value.Node.Processes.State, value.Node.Runtimes.State}
	for _, state := range states {
		if !validState(state) {
			return fmt.Errorf("invalid fact state %q", state)
		}
	}
	for _, finding := range value.Findings {
		if !ruleIDPattern.MatchString(finding.RuleID) {
			return fmt.Errorf("invalid rule ID %q", finding.RuleID)
		}
		if finding.Title == "" || finding.Domain == "" || finding.CurrentState == "" {
			return fmt.Errorf("finding %s lacks title, domain, or current state", finding.RuleID)
		}
		if !validStatus(finding.Status) {
			return fmt.Errorf("finding %s has invalid status %q", finding.RuleID, finding.Status)
		}
		if finding.Dimension != model.DimensionDeployment && finding.Dimension != model.DimensionPerformance {
			return fmt.Errorf("finding %s has invalid dimension %q", finding.RuleID, finding.Dimension)
		}
		if !oneOf(string(finding.Confidence), "high", "medium", "low") {
			return fmt.Errorf("finding %s has invalid confidence", finding.RuleID)
		}
		if len(finding.Evidence) == 0 {
			return fmt.Errorf("finding %s has no evidence", finding.RuleID)
		}
		if len(finding.References) == 0 {
			return fmt.Errorf("finding %s has no references", finding.RuleID)
		}
		if finding.Why == "" || finding.Recommendation == "" || len(finding.Verification) == 0 {
			return fmt.Errorf("finding %s lacks why, recommendation, or verification", finding.RuleID)
		}
	}
	return nil
}
func validState(value model.FactState) bool {
	return oneOf(string(value), "available", "not_detected", "unsupported", "permission_denied", "timeout", "parse_error", "unknown")
}
func validStatus(value model.Status) bool {
	return oneOf(string(value), "pass", "warn", "fail", "info", "unknown", "skip")
}
func oneOf(value string, allowed ...string) bool {
	for _, candidate := range allowed {
		if value == candidate {
			return true
		}
	}
	return false
}
