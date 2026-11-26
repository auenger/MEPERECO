package workflowcontract

import (
	"bytes"
	"fmt"
	"strings"
	"time"

	"github.com/robfig/cron/v3"
)

const (
	CompletionVersion         = "meperstudio.completion/v1"
	CompletionTasksDone       = "all_tasks_terminal" // historical wire name; success means done, not merely terminal.
	CompletionValidatedResult = "validated_result"
	EvaluationManual          = "manual"
	EvaluationOnTaskChange    = "on_task_change"
	EvaluationScheduled       = "scheduled"
)

// CompletionCapabilities is the public, static contract advertised to designers.
type CompletionCapabilities struct {
	SchemaVersion          string   `json:"schema_version"`
	ManualEvaluations      []string `json:"manual_evaluations"`
	ManualOutputValidation bool     `json:"manual_output_validation"`
	AgentOutputValidation  bool     `json:"agent_output_validation"`
}

func SupportedCompletionCapabilities() CompletionCapabilities {
	return CompletionCapabilities{SchemaVersion: CompletionVersion, ManualEvaluations: []string{EvaluationOnTaskChange, EvaluationManual, EvaluationScheduled}, AgentOutputValidation: true}
}

// CompletionPolicy separates readiness from authorization and advancement.
// AllowedTaskStatuses describes the designer's task vocabulary, never success.
// Historical schedules are retained for viewing, not silently interpreted.
type CompletionPolicy struct {
	SchemaVersion       string   `json:"schema_version,omitempty"`
	Type                string   `json:"type"`
	EvaluationMode      string   `json:"evaluation_mode,omitempty"`
	Schedule            string   `json:"schedule,omitempty"`
	Timezone            string   `json:"timezone,omitempty"`
	AllowedTaskStatuses []string `json:"allowed_task_statuses,omitempty"`
}

// ParseCompletionPolicy reads both historic defaults and the current contract.
// It never treats a cancelled or unfinished task as delivery.
func ParseCompletionPolicy(raw []byte, executionMode string) (CompletionPolicy, error) {
	p := CompletionPolicy{}
	if len(bytes.TrimSpace(raw)) != 0 {
		if bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
			return p, fmt.Errorf("completion_rule must be an object")
		}
		if err := strictDecode(raw, &p); err != nil {
			return p, fmt.Errorf("invalid completion_rule: %w", err)
		}
	}
	if p.SchemaVersion != "" && p.SchemaVersion != CompletionVersion {
		return p, fmt.Errorf("unsupported completion schema %q", p.SchemaVersion)
	}
	legacy := p.SchemaVersion == ""
	p.SchemaVersion = CompletionVersion
	if executionMode == "agent" {
		if p.Type == "" || (legacy && p.Type == "all_runs_terminal") {
			p.Type = CompletionValidatedResult
		}
		if p.Type != CompletionValidatedResult || p.EvaluationMode != "" || p.Schedule != "" || len(p.AllowedTaskStatuses) > 0 {
			return p, fmt.Errorf("agent execution requires validated_result completion")
		}
		return p, nil
	}
	if executionMode != "" && executionMode != "manual" {
		return p, fmt.Errorf("unsupported execution mode %q", executionMode)
	}
	if p.Type == "" || (legacy && p.Type == "all_runs_terminal") {
		p.Type = CompletionTasksDone
	}
	if p.Type != CompletionTasksDone {
		return p, fmt.Errorf("unsupported manual completion rule %q", p.Type)
	}
	if p.EvaluationMode == "" {
		p.EvaluationMode = EvaluationManual
	}
	switch p.EvaluationMode {
	case EvaluationManual, EvaluationOnTaskChange, EvaluationScheduled:
	default:
		return p, fmt.Errorf("unsupported evaluation mode %q", p.EvaluationMode)
	}
	seen := map[string]bool{}
	for _, status := range p.AllowedTaskStatuses {
		if strings.TrimSpace(status) == "" || seen[status] {
			return p, fmt.Errorf("invalid or duplicate task status %q", status)
		}
		seen[status] = true
	}
	return p, nil
}

// ManualCompletion evaluates the strict successful-delivery invariant; zero
// required tasks retains the historical ability to complete a manual stage.
// The caller must supply current non-invalidated task rows and recheck under lock.
type CompletionEvaluation struct {
	Ready           bool     `json:"ready"`
	IncompleteTasks int      `json:"incomplete_tasks"`
	Blockers        []string `json:"blockers"`
}

func (p CompletionPolicy) EvaluateManual(statuses []string) CompletionEvaluation {
	r := CompletionEvaluation{Blockers: []string{}}
	if p.Type != CompletionTasksDone {
		r.Blockers = append(r.Blockers, "unsupported_completion_policy")
		return r
	}
	for _, status := range statuses {
		if status != "done" {
			r.IncompleteTasks++
		}
	}
	if r.IncompleteTasks > 0 {
		r.Blockers = append(r.Blockers, "required_tasks_not_done")
	}
	r.Ready = len(r.Blockers) == 0
	return r
}

// Agent evidence is assembled from the authoritative execution/readiness query.
// Contract and artifact validation still occurs in the transactional review.
type AgentCompletionEvidence struct {
	Current, AwaitingReview, HasResult, MainTaskDone, MainRunCompleted bool
	IncompleteTasks, ActiveCalls                                       int64
}

func EvaluateAgentCompletion(e AgentCompletionEvidence) []string {
	blockers := []string{}
	for _, check := range []struct {
		ok   bool
		code string
	}{
		{e.Current, "execution_not_current"}, {e.AwaitingReview, "execution_not_awaiting_review"}, {e.HasResult, "result_not_submitted"},
		{e.MainTaskDone, "main_task_not_done"}, {e.MainRunCompleted, "main_call_not_completed"},
		{e.IncompleteTasks == 0, "required_tasks_not_done"}, {e.ActiveCalls == 0, "calls_still_active"},
	} {
		if !check.ok {
			blockers = append(blockers, check.code)
		}
	}
	return blockers
}

// NextEvaluationAfter accepts a five-field cron and an explicit IANA zone.
// Invalid legacy free-text schedules remain readable, but writes must repair them.
func (p CompletionPolicy) NextEvaluationAfter(after time.Time) (time.Time, error) {
	if p.EvaluationMode != EvaluationScheduled {
		return time.Time{}, fmt.Errorf("policy is not scheduled")
	}
	zone := p.Timezone
	if zone == "" {
		zone = "UTC"
	}
	loc, err := time.LoadLocation(zone)
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid evaluation timezone: %w", err)
	}
	if len(strings.Fields(p.Schedule)) != 5 {
		return time.Time{}, fmt.Errorf("evaluation schedule must be a five-field cron expression")
	}
	sched, err := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow).Parse(p.Schedule)
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid evaluation schedule: %w", err)
	}
	next := sched.Next(after.In(loc)).UTC()
	if next.IsZero() {
		return time.Time{}, fmt.Errorf("schedule has no future evaluation")
	}
	return next, nil
}
func (p CompletionPolicy) ValidateWrite() error {
	if p.EvaluationMode == EvaluationScheduled {
		_, err := p.NextEvaluationAfter(time.Now())
		return err
	}
	if p.Schedule != "" || p.Timezone != "" {
		return fmt.Errorf("schedule and timezone require scheduled evaluation")
	}
	return nil
}
