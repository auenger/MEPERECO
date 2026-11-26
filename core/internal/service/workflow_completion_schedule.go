package service

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	db "github.com/auenger/MEPERECO/core/pkg/db/generated"
	"github.com/auenger/MEPERECO/core/pkg/workflowcontract"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

type ManualReadiness struct {
	workflowcontract.CompletionEvaluation
	EvaluationMode string    `json:"evaluation_mode"`
	EvaluatedAt    time.Time `json:"evaluated_at"`
	ScheduleError  string    `json:"schedule_error,omitempty"`
}

func EvaluateManualStage(stage db.WorkflowInstanceStage, statuses []string, now time.Time) ManualReadiness {
	p, err := workflowcontract.ParseCompletionPolicy(stage.CompletionRule, "manual")
	if err != nil {
		return ManualReadiness{CompletionEvaluation: workflowcontract.CompletionEvaluation{Blockers: []string{"invalid_completion_policy"}}, EvaluatedAt: now}
	}
	out := ManualReadiness{CompletionEvaluation: p.EvaluateManual(statuses), EvaluationMode: p.EvaluationMode, EvaluatedAt: now}
	if p.EvaluationMode == workflowcontract.EvaluationScheduled {
		if _, err := p.NextEvaluationAfter(now); err != nil {
			out.ScheduleError = err.Error()
		}
	}
	return out
}
func (s *WorkflowExecutionService) ReadAutomaticReadiness(ctx context.Context, run db.WorkflowInstance) (*ManualReadiness, error) {
	tx, err := s.TxStarter.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	var raw []byte
	err = tx.QueryRow(ctx, `SELECT evaluation FROM workflow_stage_readiness WHERE workflow_instance_id=$1 AND workspace_id=$2 AND stage_id=$3 AND state_token=$4`, run.ID, run.WorkspaceID, run.CurrentStageID, WorkflowStateToken(run)).Scan(&raw)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var out ManualReadiness
	if err = json.Unmarshal(raw, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// EvaluateAutomaticReadiness evaluates diagnostics only. Run lock and task locks
// use the same ordering as transitions, and state tokens reset after rework.
func (s *WorkflowExecutionService) EvaluateAutomaticReadiness(ctx context.Context, ws, instance pgtype.UUID) error {
	tx, err := s.TxStarter.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	q := s.Queries.WithTx(tx)
	run, err := lockWorkflowExecutionRun(ctx, q, ws, instance)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	if run.ArchivedAt.Valid || (run.Status != "active" && run.Status != "waiting") || !run.CurrentStageID.Valid {
		return nil
	}
	stage, err := q.GetWorkflowInstanceStage(ctx, db.GetWorkflowInstanceStageParams{WorkspaceID: ws, WorkflowInstanceID: instance, ID: run.CurrentStageID})
	if err != nil {
		return err
	}
	if stage.ExecutionMode != "manual" {
		return nil
	}
	policy, err := workflowcontract.ParseCompletionPolicy(stage.CompletionRule, "manual")
	if err != nil || (policy.EvaluationMode != workflowcontract.EvaluationScheduled && policy.EvaluationMode != workflowcontract.EvaluationOnTaskChange) {
		return nil
	}
	var now time.Time
	if err = tx.QueryRow(ctx, `SELECT now()`).Scan(&now); err != nil {
		return err
	}
	anchor := run.UpdatedAt.Time
	token := WorkflowStateToken(run)
	var previous time.Time
	var previousHash string
	err = tx.QueryRow(ctx, `SELECT evaluated_at,evidence_hash FROM workflow_stage_readiness WHERE workflow_instance_id=$1 AND workspace_id=$2 AND stage_id=$3 AND state_token=$4`, instance, ws, stage.ID, token).Scan(&previous, &previousHash)
	if err == nil {
		anchor = previous
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return err
	}
	if policy.EvaluationMode == workflowcontract.EvaluationScheduled {
		next, err := policy.NextEvaluationAfter(anchor)
		if err != nil || next.After(now) {
			return nil
		}
	}

	tasks, err := q.LockWorkflowStageTasks(ctx, db.LockWorkflowStageTasksParams{WorkspaceID: ws, WorkflowInstanceID: instance, WorkflowStageID: stage.ID})
	if err != nil {
		return err
	}
	statuses := make([]string, 0, len(tasks))
	hash := sha256.New()
	for _, task := range tasks {
		statuses = append(statuses, task.Status)
		fmt.Fprintf(hash, "%x:%s;", task.ID.Bytes, task.Status)
	}
	evidenceHash := fmt.Sprintf("%x", hash.Sum(nil))
	if policy.EvaluationMode == workflowcontract.EvaluationOnTaskChange && previousHash == evidenceHash {
		return nil
	}

	evaluation := EvaluateManualStage(stage, statuses, now)
	raw, err := json.Marshal(evaluation)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `INSERT INTO workflow_stage_readiness(workflow_instance_id,workspace_id,stage_id,state_token,evaluated_at,evaluation,evidence_hash) VALUES($1,$2,$3,$4,$5,$6,$7) ON CONFLICT(workflow_instance_id) DO UPDATE SET stage_id=excluded.stage_id,state_token=excluded.state_token,evaluated_at=excluded.evaluated_at,evaluation=excluded.evaluation,evidence_hash=excluded.evidence_hash`, instance, ws, stage.ID, token, now, raw, evidenceHash)
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}
func (s *WorkflowExecutionService) scanAutomaticReadiness(ctx context.Context, after pgtype.UUID) (pgtype.UUID, error) {
	tx, err := s.TxStarter.Begin(ctx)
	if err != nil {
		return after, err
	}
	defer tx.Rollback(ctx)
	rows, err := tx.Query(ctx, `SELECT i.workspace_id,i.id FROM workflow_instance i JOIN workflow_instance_stage s ON s.id=i.current_stage_id AND s.workflow_instance_id=i.id AND s.workspace_id=i.workspace_id WHERE i.archived_at IS NULL AND i.status IN ('active','waiting') AND s.execution_mode='manual' AND s.completion_rule->>'evaluation_mode' IN ('scheduled','on_task_change') AND ($1::uuid IS NULL OR i.id>$1) ORDER BY i.id LIMIT 100`, after)
	if err != nil {
		return after, err
	}
	type candidate struct{ ws, id pgtype.UUID }
	var list []candidate
	for rows.Next() {
		var c candidate
		if err = rows.Scan(&c.ws, &c.id); err != nil {
			rows.Close()
			return after, err
		}
		list = append(list, c)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return after, err
	}
	if err = tx.Commit(ctx); err != nil {
		return after, err
	}
	for _, c := range list {
		if err = s.EvaluateAutomaticReadiness(ctx, c.ws, c.id); err != nil {
			slog.Warn("workflow automatic evaluation failed", "instance_id", c.id, "error", err)
		}
	}
	if len(list) < 100 {
		return pgtype.UUID{}, nil
	}
	return list[len(list)-1].id, nil
}

// WorkflowEvaluationTarget is enqueued by task events. The periodic scan
// recovers missed or dropped events and scheduled evaluations across restarts.
type WorkflowEvaluationTarget struct{ WorkspaceID, InstanceID pgtype.UUID }

func (s *WorkflowExecutionService) RunAutomaticReadinessWorker(ctx context.Context, work <-chan WorkflowEvaluationTarget) {
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()
	var cursor pgtype.UUID
	scan := func() {
		next, err := s.scanAutomaticReadiness(ctx, cursor)
		if err != nil && ctx.Err() == nil {
			slog.Warn("workflow evaluation scan failed", "error", err)
		} else {
			cursor = next
		}
	}
	scan()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			scan()
		case item, ok := <-work:
			if !ok {
				work = nil
				continue
			}
			if err := s.EvaluateAutomaticReadiness(ctx, item.WorkspaceID, item.InstanceID); err != nil && ctx.Err() == nil {
				slog.Warn("workflow event evaluation failed", "error", err)
			}
		}
	}
}
