package rules

import (
	"time"

	"github.com/arpingblue/AIStat/internal/model"
	"github.com/arpingblue/AIStat/internal/topology"
)

type RuleID string

type Rule interface {
	ID() RuleID
	Meta() RuleMeta
	Evaluate(RuleContext) []model.Finding
}

type RuleContext struct {
	Snapshot *model.Snapshot
	Graph    *topology.Graph
	Profile  model.Profile
	Now      time.Time
}

type RuleMeta struct {
	Title       string
	Domain      string
	Dimension   model.Dimension
	Priority    model.Priority
	Confidence  model.Confidence
	Description string
	References  []model.Reference
}

type rule struct {
	id       RuleID
	meta     RuleMeta
	evaluate func(RuleContext, rule) []model.Finding
}

func (r rule) ID() RuleID                               { return r.id }
func (r rule) Meta() RuleMeta                           { return r.meta }
func (r rule) Evaluate(ctx RuleContext) []model.Finding { return r.evaluate(ctx, r) }

func finding(r rule, status model.Status, severity model.Severity, subject model.Subject, current, expected, impact, remediation string, evidence ...model.Evidence) model.Finding {
	if len(evidence) == 0 {
		evidence = []model.Evidence{{Fact: "rule.evaluation", Value: current, Source: "normalized snapshot"}}
	}
	if impact == "" {
		impact = "No negative readiness impact was established."
	}
	if remediation == "" {
		remediation = "No change is recommended for this rule result."
	}
	return model.Finding{RuleID: string(r.id), Title: r.meta.Title, Domain: r.meta.Domain, Status: status, Severity: severity, Dimension: r.meta.Dimension, Priority: r.meta.Priority, Subject: subject, CurrentState: current, ExpectedState: expected, Impact: impact, Why: impact, Recommendation: remediation, Verification: []string{"Re-run AIStat in the same workload context after the change."}, Confidence: r.meta.Confidence, References: r.meta.References, Evidence: evidence}
}

func pass(r rule, current string) []model.Finding {
	return []model.Finding{finding(r, model.StatusPass, model.SeverityInfo, model.Subject{Kind: "node", ID: "host"}, current, "No trigger condition detected.", "", "")}
}
func skip(r rule, current string) []model.Finding {
	return []model.Finding{finding(r, model.StatusSkip, model.SeverityInfo, model.Subject{Kind: "node", ID: "host"}, current, "Applicable workload context.", "", "")}
}
func unknown(r rule, current string) []model.Finding {
	return []model.Finding{finding(r, model.StatusUnknown, model.SeverityMedium, model.Subject{Kind: "node", ID: "host"}, current, "Sufficient evidence to evaluate the rule.", "The rule cannot establish readiness from incomplete evidence.", "Restore read permissions or the required inspection tool, then retry.")}
}
