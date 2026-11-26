package service

import (
	"context"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/auenger/MEPERECO/core/internal/util"
	db "github.com/auenger/MEPERECO/core/pkg/db/generated"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

func testUUID(t *testing.T, raw string) pgtype.UUID {
	t.Helper()
	id, err := util.ParseUUID(raw)
	if err != nil {
		t.Fatal(err)
	}
	return id
}
func TestWorkflowTransitionPlan(t *testing.T) {
	ws := testUUID(t, "00000000-0000-0000-0000-000000000001")
	id := testUUID(t, "00000000-0000-0000-0000-000000000002")
	stageID := testUUID(t, "00000000-0000-0000-0000-000000000003")
	targetID := testUUID(t, "00000000-0000-0000-0000-000000000004")
	run := db.WorkflowInstance{ID: id, WorkspaceID: ws, CurrentStageID: stageID, Status: "active", UpdatedAt: pgtype.Timestamptz{Time: time.Now(), Valid: true}}
	current := db.WorkflowInstanceStage{ID: stageID, WorkspaceID: ws, WorkflowInstanceID: id, Position: 1, RollbackStageKey: pgtype.Text{String: "first", Valid: true}}
	target := db.WorkflowInstanceStage{ID: targetID, WorkspaceID: ws, WorkflowInstanceID: id, Position: 2, StableKey: "next"}
	if p, err := planWorkflowTransition(run, current, target, "approved"); err != nil || !p.EnterStage || p.StageID != targetID {
		t.Fatal(p, err)
	}
	if p, err := planWorkflowTransition(run, current, db.WorkflowInstanceStage{}, "approved"); err != nil || p.Status != "completed" || p.StageID != stageID {
		t.Fatal(p, err)
	}
	if p, err := planWorkflowTransition(run, current, db.WorkflowInstanceStage{}, "rejected"); err != nil || p.Status != "waiting" {
		t.Fatal(p, err)
	}
	target.Position = 0
	target.StableKey = "first"
	if p, err := planWorkflowTransition(run, current, target, "rejected"); err != nil || !p.EnterStage {
		t.Fatal(p, err)
	}
	target.StableKey = "incorrect"
	if _, err := planWorkflowTransition(run, current, target, "rejected"); err == nil {
		t.Fatal("invalid rollback")
	}
	old := WorkflowStateToken(run)
	run.UpdatedAt.Time = run.UpdatedAt.Time.Add(time.Microsecond)
	if old == WorkflowStateToken(run) {
		t.Fatal("state token ignored timestamp precision")
	}
	run.Status = "completed"
	if _, err := planWorkflowTransition(run, current, target, "approved"); err == nil {
		t.Fatal("closed run transitioned")
	}
}
func TestWorkflowManualGate(t *testing.T) {
	for _, tc := range []struct {
		raw, kind, id string
		ok            bool
	}{
		{`{"type":"none"}`, "member", "a", true}, {`{"type":"human","decider":"a"}`, "member", "a", true}, {`{"type":"human","decider":"a"}`, "member", "b", false}, {`{"type":"human"}`, "agent", "a", false}, {`{"type":"agent","decider":"a"}`, "agent", "a", true}, {`{"type":"unknown"}`, "member", "a", false},
	} {
		_, err := validateManualWorkflowGate([]byte(tc.raw), tc.kind, tc.id)
		if (err == nil) != tc.ok {
			t.Fatal(tc, err)
		}
	}
}

// Requires a disposable database with all repository migrations applied. Never
// falls back to DATABASE_URL; a normal unit test run does not write to any DB.
func TestWorkflowManualTransitionPostgres(t *testing.T) {
	url := os.Getenv("MEPERSTUDIO_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("MEPERSTUDIO_TEST_DATABASE_URL (isolated migrated database) not set")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	var ws, user, wf, version, run, first, second, task pgtype.UUID
	row := func(sql string, dest *pgtype.UUID, args ...any) {
		t.Helper()
		if err := pool.QueryRow(ctx, sql, args...).Scan(dest); err != nil {
			t.Fatal(err)
		}
	}
	row(`INSERT INTO workspace(name,slug,task_prefix) VALUES('Workflow test','workflow-test-'||gen_random_uuid()::text,'WF') RETURNING id`, &ws)
	row(`INSERT INTO "user"(name,email) VALUES('Workflow owner',gen_random_uuid()::text||'@example.test') RETURNING id`, &user)
	t.Cleanup(func() { // isolated fixture rows; cleanup best effort, disposable DB is removed by runner.
		_, _ = pool.Exec(ctx, `DELETE FROM workspace WHERE id=$1`, ws)
		_, _ = pool.Exec(ctx, `DELETE FROM "user" WHERE id=$1`, user)
	})
	if _, err := pool.Exec(ctx, `INSERT INTO member(workspace_id,user_id,role) VALUES($1,$2,'owner')`, ws, user); err != nil {
		t.Fatal(err)
	}
	row(`INSERT INTO workflow(workspace_id,name,created_by) VALUES($1,'test SOP',$2) RETURNING id`, &wf, ws, user)
	row(`INSERT INTO workflow_version(workspace_id,workflow_id,version,created_by) VALUES($1,$2,1,$3) RETURNING id`, &version, ws, wf, user)
	row(`INSERT INTO workflow_instance(workspace_id,workflow_id,workflow_version_id,title,status,created_by) VALUES($1,$2,$3,'test Run','active',$4) RETURNING id`, &run, ws, wf, version, user)
	row(`INSERT INTO workflow_instance_stage(workspace_id,workflow_instance_id,stable_key,name,position) VALUES($1,$2,'first','First',0) RETURNING id`, &first, ws, run)
	row(`INSERT INTO workflow_instance_stage(workspace_id,workflow_instance_id,stable_key,name,position,rollback_stage_key) VALUES($1,$2,'second','Second',1,'first') RETURNING id`, &second, ws, run)
	if _, err := pool.Exec(ctx, `UPDATE workflow_instance SET current_stage_id=$1 WHERE id=$2`, first, run); err != nil {
		t.Fatal(err)
	}
	row(`INSERT INTO task(workspace_id,title,status,creator_type,creator_id,workflow_instance_id,workflow_stage_id,number) VALUES($1,'Required','cancelled','member',$2,$3,$4,1) RETURNING id`, &task, ws, user, run, first)
	q := db.New(pool)
	svc := &WorkflowExecutionService{Queries: q, TxStarter: pool}
	get := func() db.WorkflowInstance {
		t.Helper()
		r, err := q.GetWorkflowInstanceInWorkspace(ctx, db.GetWorkflowInstanceInWorkspaceParams{WorkspaceID: ws, ID: run})
		if err != nil {
			t.Fatal(err)
		}
		return r
	}
	p := ManualWorkflowTransitionParams{WorkspaceID: ws, InstanceID: run, ActorID: user, ActorType: "member", Outcome: "approved", ExpectedStateToken: WorkflowStateToken(get())}
	if _, err := svc.TransitionManual(ctx, p); err == nil {
		t.Fatal("cancelled task accepted")
	}
	if get().CurrentStageID != first {
		t.Fatal("failed decision modified run")
	}
	if _, err := pool.Exec(ctx, `UPDATE task SET status='done' WHERE id=$1`, task); err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	results := make(chan error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); _, err := svc.TransitionManual(ctx, p); results <- err }()
	}
	wg.Wait()
	close(results)
	success := 0
	for err := range results {
		if err == nil {
			success++
		}
	}
	if success != 1 || get().CurrentStageID != second {
		t.Fatalf("duplicate decision: successes=%d state=%+v", success, get())
	}
	p.ExpectedStateToken = WorkflowStateToken(get())
	p.Outcome = "rejected"
	p.TargetStageID = first
	note := "Needs rework"
	p.Note = &note
	if _, err := svc.TransitionManual(ctx, p); err != nil {
		t.Fatal(err)
	}
	if get().CurrentStageID != first {
		t.Fatal("rollback did not return to first stage")
	}
	// Scheduled evaluation runs against the locked current task state but does not advance it.
	if _, err := pool.Exec(ctx, `UPDATE workflow_instance_stage SET completion_rule='{"type":"all_tasks_terminal","evaluation_mode":"scheduled","schedule":"* * * * *"}' WHERE id=$1`, first); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `UPDATE workflow_instance SET updated_at=now()-interval '2 minutes' WHERE id=$1`, run); err != nil {
		t.Fatal(err)
	}
	if err := svc.EvaluateAutomaticReadiness(ctx, ws, run); err != nil {
		t.Fatal(err)
	}
	snap, err := svc.ReadAutomaticReadiness(ctx, get())
	if err != nil || snap == nil || !snap.Ready {
		t.Fatal(snap, err)
	}
	if get().CurrentStageID != first {
		t.Fatal("evaluation advanced the stage")
	}
	if _, err := pool.Exec(ctx, `UPDATE workflow_instance SET revision=revision+1,updated_at=now() WHERE id=$1`, run); err != nil {
		t.Fatal(err)
	}
	snap, err = svc.ReadAutomaticReadiness(ctx, get())
	if err != nil || snap != nil {
		t.Fatal("stale schedule snapshot", snap, err)
	}

	// Task-change evaluation updates evidence only when it changes; it must
	// neither advance the stage nor repeatedly overwrite unchanged snapshots.
	var currentTask pgtype.UUID
	row(`INSERT INTO task(workspace_id,title,status,creator_type,creator_id,workflow_instance_id,workflow_stage_id,number) VALUES($1,'Current work','done','member',$2,$3,$4,50) RETURNING id`, &currentTask, ws, user, run, first)
	if _, err := pool.Exec(ctx, `UPDATE workflow_instance_stage SET completion_rule='{"type":"all_tasks_terminal","evaluation_mode":"on_task_change"}' WHERE id=$1`, first); err != nil {
		t.Fatal(err)
	}
	if err := svc.EvaluateAutomaticReadiness(ctx, ws, run); err != nil {
		t.Fatal(err)
	}
	snap, err = svc.ReadAutomaticReadiness(ctx, get())
	if err != nil || snap == nil || !snap.Ready {
		t.Fatal(snap, err)
	}
	firstEvaluation := snap.EvaluatedAt
	if err := svc.EvaluateAutomaticReadiness(ctx, ws, run); err != nil {
		t.Fatal(err)
	}
	snap, err = svc.ReadAutomaticReadiness(ctx, get())
	if err != nil || snap == nil || !snap.EvaluatedAt.Equal(firstEvaluation) {
		t.Fatal("unchanged evidence was rewritten", snap, err)
	}
	if _, err := pool.Exec(ctx, `UPDATE task SET status='in_progress' WHERE id=$1`, currentTask); err != nil {
		t.Fatal(err)
	}
	if err := svc.EvaluateAutomaticReadiness(ctx, ws, run); err != nil {
		t.Fatal(err)
	}
	snap, err = svc.ReadAutomaticReadiness(ctx, get())
	if err != nil || snap == nil || snap.Ready || get().CurrentStageID != first {
		t.Fatal("task change incorrectly evaluated", snap, err)
	}

	// Real Agent delivery and human review contracts share the same advancement path.
	var executor, runtimeID pgtype.UUID
	row(`INSERT INTO agent_runtime(workspace_id,name,runtime_mode,provider,owner_id) VALUES($1,'Test runtime','local','codex',$2) RETURNING id`, &runtimeID, ws, user)
	row(`INSERT INTO agent(workspace_id,name,runtime_mode,owner_id,runtime_id) VALUES($1,'Workflow executor','local',$2,$3) RETURNING id`, &executor, ws, user, runtimeID)
	for gateIndex, gateKind := range []string{"none", "human", "agent", "hybrid"} {
		var instance, execution, stage, mainTask, mainRun, result, next pgtype.UUID
		row(`INSERT INTO workflow_instance(workspace_id,workflow_id,workflow_version_id,title,status,created_by) VALUES($1,$2,$3,'Agent run','active',$4) RETURNING id`, &instance, ws, wf, version, user)
		row(`INSERT INTO workflow_instance_stage(workspace_id,workflow_instance_id,stable_key,name,position,execution_mode,executor_id,completion_rule) VALUES($1,$2,'agent','Agent',0,'agent',$3,'{"type":"validated_result"}') RETURNING id`, &stage, ws, instance, executor)
		row(`INSERT INTO workflow_instance_stage(workspace_id,workflow_instance_id,stable_key,name,position) VALUES($1,$2,'next','Next',1) RETURNING id`, &next, ws, instance)
		if _, err := pool.Exec(ctx, `UPDATE workflow_instance SET current_stage_id=$1 WHERE id=$2`, stage, instance); err != nil {
			t.Fatal(err)
		}
		guard := map[string]any{"type": gateKind}
		if gateKind == "human" {
			guard["decider"] = util.UUIDToString(user)
		} else if gateKind == "agent" || gateKind == "hybrid" {
			guard["decider"] = util.UUIDToString(executor)
		}
		if gateKind == "human" || gateKind == "hybrid" {
			guard["human_decider"] = util.UUIDToString(user)
		}
		cfg := map[string]any{"gate": guard, "completion_rule": map[string]any{"type": "validated_result"}, "output_spec": map[string]any{"contract": map[string]any{"schema_version": "silieco.stage/v1", "data": map[string]any{"type": "object", "properties": map[string]any{"summary": map[string]any{"type": "string"}}, "required": []string{"summary"}}}}, "retry_policy": map[string]any{"max_attempts": 3, "backoff_seconds": 2}}
		row(`INSERT INTO workflow_stage_execution(workspace_id,workflow_instance_id,workflow_stage_id,attempt,plan_revision,executor_id,config_snapshot,input_snapshot,status,result_revision) VALUES($1,$2,$3,1,1,$4,$5,'{}','awaiting_review',1) RETURNING id`, &execution, ws, instance, stage, executor, cfg)
		num := int32(2 + gateIndex)
		row(`INSERT INTO task(workspace_id,title,status,creator_type,creator_id,workflow_instance_id,workflow_stage_id,workflow_stage_execution_id,number) VALUES($1,'Agent delivery','done','member',$2,$3,$4,$5,$6) RETURNING id`, &mainTask, ws, user, instance, stage, execution, num)
		row(`INSERT INTO agent_run(agent_id,task_id,status,completed_at,workflow_stage_execution_id) VALUES($1,$2,'completed',now(),$3) RETURNING id`, &mainRun, executor, mainTask, execution)
		if _, err := pool.Exec(ctx, `UPDATE workflow_stage_execution SET main_task_id=$1,main_run_id=$2 WHERE id=$3`, mainTask, mainRun, execution); err != nil {
			t.Fatal(err)
		}
		row(`INSERT INTO workflow_execution_result(workspace_id,workflow_instance_id,execution_id,revision,data,submitted_by_type,submitted_by_id) VALUES($1,$2,$3,1,'{"summary":1}','agent',$4) RETURNING id`, &result, ws, instance, execution, executor)
		if gateKind == "none" {
			if err := svc.AcceptAutomaticExecution(ctx, ws, instance, execution); err == nil {
				t.Fatal("invalid output accepted")
			}
		} else if gateKind == "human" {
			if _, err := svc.PrepareHumanReview(ctx, ws, instance, execution, user); err == nil {
				t.Fatal("invalid output reviewed")
			}
		} else {
			if _, err := svc.PrepareAgentReview(ctx, ws, instance, execution); err == nil {
				t.Fatal("invalid output reviewed by agent")
			}
		}
		if _, err := pool.Exec(ctx, `UPDATE workflow_execution_result SET data='{"summary":"done"}' WHERE id=$1`, result); err != nil {
			t.Fatal(err)
		}
		if gateKind == "agent" || gateKind == "hybrid" {
			request, err := svc.PrepareAgentReview(ctx, ws, instance, execution)
			if err != nil {
				t.Fatal(err)
			}
			// Bind a local fixture call; no real provider/account or external dispatch.
			var reviewRun pgtype.UUID
			row(`INSERT INTO agent_run(agent_id,task_id,status,runtime_id) VALUES($1,$2,'queued',$3) RETURNING id`, &reviewRun, executor, mainTask, runtimeID)
			if _, err := q.BindWorkflowReviewCall(ctx, db.BindWorkflowReviewCallParams{WorkspaceID: ws, RequestID: request.ID, RunID: reviewRun}); err != nil {
				t.Fatal(err)
			}
			if _, err := pool.Exec(ctx, `UPDATE agent_run SET status='running' WHERE id=$1`, reviewRun); err != nil {
				t.Fatal(err)
			}
			decision := RecordWorkflowAgentDecisionParams{WorkspaceID: ws, RequestID: request.ID, ActorID: executor, RunID: reviewRun, ExpectedResultRevision: 1, ExpectedBasisHash: request.BasisHash, Outcome: "approved"}
			if _, err := svc.RecordAgentDecision(ctx, decision); err != nil {
				t.Fatal(err)
			}
			if gateKind == "agent" {
				if _, err := svc.AcceptAgentReviewedExecution(ctx, ws, instance, execution); err == nil {
					t.Fatal("unfinished reviewer accepted")
				}
			} else if _, err := svc.PrepareHumanReview(ctx, ws, instance, execution, user); err == nil {
				t.Fatal("unfinished hybrid reviewer accepted")
			}
			if _, err := pool.Exec(ctx, `UPDATE agent_run SET status='completed',completed_at=now() WHERE id=$1`, reviewRun); err != nil {
				t.Fatal(err)
			}
		}
		if gateKind == "none" {
			if err := svc.AcceptAutomaticExecution(ctx, ws, instance, execution); err != nil {
				t.Fatal(err)
			}
		} else if gateKind == "agent" {
			if _, err := svc.AcceptAgentReviewedExecution(ctx, ws, instance, execution); err != nil {
				t.Fatal(err)
			}
		} else {
			basis, err := svc.PrepareHumanReview(ctx, ws, instance, execution, user)
			if err != nil {
				t.Fatal(err)
			}
			bad := RecordWorkflowHumanDecisionParams{WorkspaceID: ws, InstanceID: instance, ExecutionID: execution, ActorID: user, ExpectedResultRevision: 1, ExpectedBasisHash: strings.Repeat("0", 64), Outcome: "approved"}
			if _, err := svc.DecideHumanExecution(ctx, bad); err == nil {
				t.Fatal("stale review evidence accepted")
			}
			bad.ExpectedBasisHash = basis.Hash
			decision, err := svc.DecideHumanExecution(ctx, bad)
			if err != nil {
				t.Fatal(err)
			}
			repeated, err := svc.DecideHumanExecution(ctx, bad)
			if err != nil || repeated.ID != decision.ID {
				t.Fatal("decision replay not idempotent", err)
			}
		}
		actual, err := q.GetWorkflowInstanceInWorkspace(ctx, db.GetWorkflowInstanceInWorkspaceParams{WorkspaceID: ws, ID: instance})
		if err != nil || actual.CurrentStageID != next {
			t.Fatal("agent did not advance", actual, err)
		}
		exec, err := q.GetWorkflowStageExecution(ctx, db.GetWorkflowStageExecutionParams{WorkspaceID: ws, ID: execution})
		if err != nil || exec.Status != "succeeded" || len(exec.Acceptance) == 0 {
			t.Fatal("acceptance not persisted", exec.Status, err)
		}
		call, err := q.GetAgentRun(ctx, mainRun)
		if err != nil {
			t.Fatal(err)
		}
		ceiling, delay, err := runRetrySettings(ctx, q, call, "provider_network")
		if err != nil || ceiling != 3 || delay != 2*time.Second {
			t.Fatal("frozen retry settings", ceiling, delay, err)
		}
	}

}
