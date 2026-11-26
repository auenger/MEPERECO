package workflowcontract

import (
	"encoding/json"
	"reflect"
	"testing"
	"time"
)

func TestManualCompletionPolicy(t *testing.T) {
	for _, raw := range []string{"", `{}`, `{"type":"all_runs_terminal"}`, `{"type":"all_tasks_terminal","allowed_task_statuses":["todo","in_progress","done"],"evaluation_mode":"on_task_change"}`} {
		p, err := ParseCompletionPolicy([]byte(raw), "manual")
		if err != nil {
			t.Fatal(err)
		}
		for _, tc := range []struct {
			statuses   []string
			ready      bool
			incomplete int
		}{{nil, true, 0}, {[]string{"done", "done"}, true, 0}, {[]string{"done", "cancelled"}, false, 1}, {[]string{"todo", "in_progress", "blocked"}, false, 3}} {
			got := p.EvaluateManual(tc.statuses)
			if got.Ready != tc.ready || got.IncompleteTasks != tc.incomplete {
				t.Fatalf("policy %s evidence %v: %+v", raw, tc.statuses, got)
			}
		}
		encoded, _ := json.Marshal(p)
		round, err := ParseCompletionPolicy(encoded, "manual")
		if err != nil || !reflect.DeepEqual(p, round) {
			t.Fatalf("round trip: %+v %v", round, err)
		}
	}
	for _, raw := range []string{`null`, `[]`, `{"type":"any_task"}`, `{"type":"validated_result"}`, `{"type":"all_tasks_terminal","evaluation_mode":"invalid"}`, `{"type":"all_tasks_terminal","type":"x"}`} {
		if _, err := ParseCompletionPolicy([]byte(raw), "manual"); err == nil {
			t.Fatalf("accepted %s", raw)
		}
	}
}
func TestAgentCompletionEvidence(t *testing.T) {
	p, err := ParseCompletionPolicy([]byte(`{"type":"validated_result"}`), "agent")
	if err != nil || p.Type != CompletionValidatedResult {
		t.Fatal(p, err)
	}
	if _, err := ParseCompletionPolicy([]byte(`{"type":"all_tasks_terminal"}`), "agent"); err == nil {
		t.Fatal("agent accepted manual rule")
	}
	e := AgentCompletionEvidence{Current: true, AwaitingReview: true, HasResult: true, MainTaskDone: true, MainRunCompleted: true}
	if got := EvaluateAgentCompletion(e); len(got) != 0 {
		t.Fatal(got)
	}
	e.Current = false
	e.ActiveCalls = 1
	e.IncompleteTasks = 2
	if got := EvaluateAgentCompletion(e); !reflect.DeepEqual(got, []string{"execution_not_current", "required_tasks_not_done", "calls_still_active"}) {
		t.Fatal(got)
	}
}
func TestRetryAndReworkPolicy(t *testing.T) {
	p, err := ParseRetryPolicy([]byte(`{"max_attempts":3,"backoff_seconds":2,"backoff_multiplier":2,"max_backoff_seconds":5}`))
	if err != nil {
		t.Fatal(err)
	}
	if p.AttemptCeiling(8) != 3 || p.RetryDelay(1, time.Second) != 2*time.Second || p.RetryDelay(3, time.Second) != 5*time.Second {
		t.Fatal("incorrect retry budget/delay")
	}
	for _, raw := range []string{`{"max_attempts":0}`, `{"backoff_multiplier":2}`, `{"backoff_seconds":3,"max_backoff_seconds":2}`} {
		if _, err := ParseRetryPolicy([]byte(raw)); err == nil {
			t.Fatalf("accepted %s", raw)
		}
	}
	if p, err := ParseReworkPolicy([]byte(`{"max_reworks":0}`)); err != nil || p.MaxReworks == nil || *p.MaxReworks != 0 {
		t.Fatal(p, err)
	}
	if _, err := ParseReworkPolicy([]byte(`{"max_reworks":101}`)); err == nil {
		t.Fatal("accepted invalid rework budget")
	}
}
func TestContractValidation(t *testing.T) {
	c, err := Parse([]byte(`{"schema_version":"silieco.stage/v1","data":{"type":"object","properties":{"summary":{"type":"string"}},"required":["summary"]}}`))
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		raw   string
		valid bool
	}{{`{"summary":"done"}`, true}, {`{}`, false}, {`{"summary":1}`, false}, {`{"summary":"ok","unknown":true}`, false}} {
		v, err := c.ValidateData([]byte(tc.raw))
		if err != nil || (len(v) == 0) != tc.valid {
			t.Fatalf("%s: %v %v", tc.raw, v, err)
		}
	}
}

func TestScheduledEvaluation(t *testing.T) {
	p, err := ParseCompletionPolicy([]byte(`{"type":"all_tasks_terminal","evaluation_mode":"scheduled","schedule":"0 18 * * *","timezone":"Asia/Shanghai"}`), "manual")
	if err != nil {
		t.Fatal(err)
	}
	next, err := p.NextEvaluationAfter(time.Date(2026, 11, 26, 9, 0, 0, 0, time.UTC))
	if err != nil || !next.Equal(time.Date(2026, 11, 26, 10, 0, 0, 0, time.UTC)) {
		t.Fatal(next, err)
	}
	p.Schedule = "每天18点"
	if p.ValidateWrite() == nil {
		t.Fatal("unsupported schedule accepted")
	}
}
