package rules

import (
	"sort"

	"github.com/arpingblue/AIStat/internal/model"
)

type Engine struct{ rules []Rule }

func NewEngine(items ...Rule) Engine { return Engine{rules: append([]Rule(nil), items...)} }
func (e Engine) Rules() []Rule       { return append([]Rule(nil), e.rules...) }

func (e Engine) Evaluate(ctx RuleContext) ([]model.Finding, model.Readiness, model.Summary) {
	findings := []model.Finding{}
	for _, item := range e.rules {
		findings = append(findings, item.Evaluate(ctx)...)
	}
	sort.SliceStable(findings, func(i, j int) bool {
		a, b := findings[i], findings[j]
		if a.Dimension != b.Dimension {
			return dimensionRank(a.Dimension) < dimensionRank(b.Dimension)
		}
		if a.Status != b.Status {
			return statusRank(a.Status) < statusRank(b.Status)
		}
		if a.Priority != b.Priority {
			return a.Priority < b.Priority
		}
		if a.RuleID != b.RuleID {
			return a.RuleID < b.RuleID
		}
		return a.Subject.ID < b.Subject.ID
	})
	readiness := model.Readiness{Deployment: "ready", Performance: "ready"}
	summary := model.Summary{}
	for _, f := range findings {
		switch f.Status {
		case model.StatusPass:
			summary.Pass++
		case model.StatusWarn:
			summary.Warn++
		case model.StatusFail:
			summary.Fail++
		case model.StatusInfo:
			summary.Info++
		case model.StatusUnknown:
			summary.Unknown++
		case model.StatusSkip:
			summary.Skip++
		}
		if f.Dimension == model.DimensionDeployment {
			if f.Status == model.StatusFail {
				readiness.Deployment = "not_ready"
			} else if f.Status == model.StatusUnknown && readiness.Deployment != "not_ready" {
				readiness.Deployment = "unknown"
			}
		}
		if f.Dimension == model.DimensionPerformance {
			if f.Status == model.StatusWarn || f.Status == model.StatusFail {
				readiness.Performance = "warn"
			} else if f.Status == model.StatusUnknown && readiness.Performance != "warn" {
				readiness.Performance = "unknown"
			}
		}
	}
	return findings, readiness, summary
}

func dimensionRank(value model.Dimension) int {
	if value == model.DimensionDeployment {
		return 0
	}
	return 1
}
func statusRank(value model.Status) int {
	switch value {
	case model.StatusFail:
		return 0
	case model.StatusWarn:
		return 1
	case model.StatusUnknown:
		return 2
	case model.StatusInfo:
		return 3
	case model.StatusPass:
		return 4
	default:
		return 5
	}
}
