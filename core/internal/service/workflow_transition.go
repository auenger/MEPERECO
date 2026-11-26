package service

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/auenger/MEPERECO/core/internal/util"
	db "github.com/auenger/MEPERECO/core/pkg/db/generated"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

// WorkflowStateToken binds a manual decision to the exact observed Run state.
func WorkflowStateToken(row db.WorkflowInstance) string {
	basis := fmt.Sprintf("%x|%x|%d|%s|%s", row.ID.Bytes, row.CurrentStageID.Bytes, row.Revision, row.Status, row.UpdatedAt.Time.UTC().Format(time.RFC3339Nano))
	return fmt.Sprintf("%x", sha256.Sum256([]byte(basis)))
}

// WorkflowTransitionError carries domain failure categories, not HTTP status.
type WorkflowTransitionError struct{ Kind, Message string }

func (e *WorkflowTransitionError) Error() string { return e.Message }
func transitionError(kind, message string) error { return &WorkflowTransitionError{kind, message} }

type WorkflowTransitionPlan struct {
	Status     string
	StageID    pgtype.UUID
	Target     db.WorkflowInstanceStage
	EnterStage bool
}

// planWorkflowTransition is shared by both accepted-result and manual decisions.
func planWorkflowTransition(run db.WorkflowInstance, current, target db.WorkflowInstanceStage, outcome string) (WorkflowTransitionPlan, error) {
	if run.ArchivedAt.Valid || (run.Status != "active" && run.Status != "waiting") || run.CurrentStageID != current.ID || !current.ID.Valid || current.WorkflowInstanceID != run.ID || current.WorkspaceID != run.WorkspaceID {
		return WorkflowTransitionPlan{}, ErrWorkflowExecutionConflict
	}
	if target.ID.Valid && (target.WorkflowInstanceID != run.ID || target.WorkspaceID != run.WorkspaceID) {
		return WorkflowTransitionPlan{}, ErrWorkflowExecutionConflict
	}
	switch outcome {
	case "approved":
		if !target.ID.Valid {
			return WorkflowTransitionPlan{Status: "completed", StageID: current.ID}, nil
		}
		if target.Position <= current.Position {
			return WorkflowTransitionPlan{}, transitionError("invalid", "target stage must be the next stage")
		}
	case "rejected":
		if !target.ID.Valid {
			return WorkflowTransitionPlan{Status: "waiting", StageID: current.ID}, nil
		}
		if target.Position >= current.Position || current.RollbackStageKey.String != target.StableKey {
			return WorkflowTransitionPlan{}, transitionError("invalid", "target stage is not the configured rollback stage")
		}
	default:
		return WorkflowTransitionPlan{}, transitionError("invalid", "outcome must be approved or rejected")
	}
	return WorkflowTransitionPlan{Status: "active", StageID: target.ID, Target: target, EnterStage: true}, nil
}
func resolveWorkflowTransitionInTx(ctx context.Context, q *db.Queries, run db.WorkflowInstance, current db.WorkflowInstanceStage, outcome string, requested pgtype.UUID) (WorkflowTransitionPlan, error) {
	var target db.WorkflowInstanceStage
	var err error
	if outcome == "approved" {
		target, err = q.GetNextWorkflowInstanceStage(ctx, db.GetNextWorkflowInstanceStageParams{WorkflowInstanceID: run.ID, WorkspaceID: run.WorkspaceID, Position: current.Position})
		if errors.Is(err, pgx.ErrNoRows) {
			target = db.WorkflowInstanceStage{}
		} else if err != nil {
			return WorkflowTransitionPlan{}, err
		}
		if requested.Valid && requested != target.ID {
			return WorkflowTransitionPlan{}, transitionError("invalid", "target stage must be the next stage")
		}
	} else if outcome == "rejected" && requested.Valid {
		target, err = q.GetWorkflowInstanceStage(ctx, db.GetWorkflowInstanceStageParams{ID: requested, WorkflowInstanceID: run.ID, WorkspaceID: run.WorkspaceID})
		if errors.Is(err, pgx.ErrNoRows) {
			return WorkflowTransitionPlan{}, transitionError("invalid", "target stage is not the configured rollback stage")
		}
		if err != nil {
			return WorkflowTransitionPlan{}, err
		}
	}
	return planWorkflowTransition(run, current, target, outcome)
}
func (s *WorkflowExecutionService) applyWorkflowTransitionInTx(ctx context.Context, q *db.Queries, run db.WorkflowInstance, plan WorkflowTransitionPlan) (db.WorkflowInstance, error) {
	updated, err := writeWorkflowStateInTx(ctx, q, run, plan.Status, plan.StageID)
	if err != nil {
		return db.WorkflowInstance{}, err
	}
	if plan.EnterStage && plan.Target.ExecutionMode == "agent" {
		_, err = s.EnsureStageExecutionInTx(ctx, q, EnsureWorkflowExecutionParams{WorkspaceID: run.WorkspaceID, InstanceID: run.ID, StageID: plan.Target.ID, ExpectedRevision: updated.Revision, ExecutorID: plan.Target.ExecutorID, InputSnapshot: updated.Inputs})
		if err != nil {
			return db.WorkflowInstance{}, fmt.Errorf("cannot enter stage execution: %w", err)
		}
	}
	return updated, nil
}

type ManualWorkflowTransitionParams struct {
	WorkspaceID, InstanceID, ActorID, TargetStageID pgtype.UUID
	ActorType, Outcome, ExpectedStateToken          string
	Note                                            *string
}
type ManualWorkflowTransitionResult struct {
	Instance db.WorkflowInstance
	Rework   WorkflowReworkEffects
}
type manualWorkflowGate struct {
	Type         string `json:"type"`
	Decider      string `json:"decider"`
	HumanDecider string `json:"human_decider"`
}

func validateManualWorkflowGate(raw []byte, actorType, actorID string) (manualWorkflowGate, error) {
	var gate manualWorkflowGate
	if err := json.Unmarshal(raw, &gate); err != nil {
		return gate, transitionError("conflict", "invalid stage gate configuration")
	}
	if gate.Type == "" {
		gate.Type = "none"
	}
	switch gate.Type {
	case "none":
	case "human", "hybrid":
		if actorType != "member" {
			return gate, transitionError("forbidden", "this stage requires a human decision")
		}
		expected := gate.HumanDecider
		if expected == "" && gate.Type == "human" {
			expected = gate.Decider
		}
		if expected != "" && expected != actorID {
			return gate, transitionError("forbidden", "only the configured human decider can review this stage")
		}
	case "agent":
		if actorType != "agent" {
			return gate, transitionError("forbidden", "this stage requires an agent decision")
		}
		if gate.Decider != "" && gate.Decider != "@self" && gate.Decider != actorID {
			return gate, transitionError("forbidden", "only the configured agent can review this stage")
		}
	default:
		return gate, transitionError("conflict", "unsupported stage gate")
	}
	return gate, nil
}

// TransitionManual owns the entire decision transaction. Actor authorization
// from the HTTP layer is retained; membership and gate are rechecked under lock.
func (s *WorkflowExecutionService) TransitionManual(ctx context.Context, p ManualWorkflowTransitionParams) (ManualWorkflowTransitionResult, error) {
	out := ManualWorkflowTransitionResult{}
	if p.Outcome == "" {
		p.Outcome = "approved"
	}
	if p.Outcome != "approved" && p.Outcome != "rejected" {
		return out, transitionError("invalid", "outcome must be approved or rejected")
	}
	if !p.ActorID.Valid || (p.ActorType != "member" && p.ActorType != "agent") {
		return out, transitionError("invalid", "invalid actor id")
	}
	tx, err := s.TxStarter.Begin(ctx)
	if err != nil {
		return out, err
	}
	defer tx.Rollback(ctx)
	q := s.Queries.WithTx(tx)
	run, err := lockWorkflowExecutionRun(ctx, q, p.WorkspaceID, p.InstanceID)
	if errors.Is(err, pgx.ErrNoRows) {
		return out, transitionError("not_found", "workflow instance not found")
	}
	if err != nil {
		return out, err
	}
	if run.ArchivedAt.Valid || !run.CurrentStageID.Valid || (run.Status != "active" && run.Status != "waiting") {
		return out, transitionError("conflict", "workflow instance is not active")
	}
	if len(p.ExpectedStateToken) != 64 || WorkflowStateToken(run) != p.ExpectedStateToken {
		return out, transitionError("conflict", "workflow state changed or review state is missing; reopen the stage review")
	}
	if p.ActorType == "member" {
		if _, err := q.GetMemberByUserAndWorkspace(ctx, db.GetMemberByUserAndWorkspaceParams{WorkspaceID: p.WorkspaceID, UserID: p.ActorID}); err != nil {
			return out, transitionError("forbidden", "workspace membership is required")
		}
	}
	current, err := q.GetWorkflowInstanceStage(ctx, db.GetWorkflowInstanceStageParams{ID: run.CurrentStageID, WorkflowInstanceID: run.ID, WorkspaceID: p.WorkspaceID})
	if err != nil {
		return out, err
	}
	if current.ExecutionMode == "agent" {
		return out, transitionError("conflict", "agent stages require accepted execution results before transition")
	}
	gate, err := validateManualWorkflowGate(current.Gate, p.ActorType, util.UUIDToString(p.ActorID))
	if err != nil {
		return out, err
	}
	plan, err := resolveWorkflowTransitionInTx(ctx, q, run, current, p.Outcome, p.TargetStageID)
	if err != nil {
		return out, err
	}
	if p.Outcome == "approved" {
		tasks, err := q.LockWorkflowStageTasks(ctx, db.LockWorkflowStageTasksParams{WorkspaceID: p.WorkspaceID, WorkflowInstanceID: p.InstanceID, WorkflowStageID: current.ID})
		if err != nil {
			return out, err
		}
		statuses := make([]string, 0, len(tasks))
		for _, task := range tasks {
			statuses = append(statuses, task.Status)
		}
		evaluation := EvaluateManualStage(current, statuses, time.Now().UTC())
		if !evaluation.Ready {
			if evaluation.IncompleteTasks == 0 {
				return out, transitionError("conflict", "invalid stage completion policy")
			}
			return out, transitionError("conflict", fmt.Sprintf("%d required task(s) have not been completed; cancellation is not successful delivery", evaluation.IncompleteTasks))
		}
	}
	if p.Outcome == "rejected" && plan.Target.ID.Valid {
		if p.Note == nil || strings.TrimSpace(*p.Note) == "" || len(*p.Note) > 4000 {
			return out, transitionError("invalid", "a rollback reason of 1 to 4000 bytes is required")
		}
		out.Rework, err = InvalidateWorkflowForReworkInTx(ctx, q, WorkflowReworkInvalidationParams{Run: run, Source: current, Target: plan.Target, SourceStateToken: pgtype.Text{String: p.ExpectedStateToken, Valid: true}, ActorType: p.ActorType, ActorID: p.ActorID, Reason: *p.Note})
		if err != nil {
			return out, transitionError("conflict", "cannot invalidate rollback work: "+err.Error())
		}
	}
	if gate.Type != "none" || p.Outcome == "rejected" || p.Note != nil {
		_, err = q.CreateWorkflowGateDecision(ctx, db.CreateWorkflowGateDecisionParams{WorkspaceID: p.WorkspaceID, WorkflowInstanceID: p.InstanceID, FromStageID: current.ID, ToStageID: plan.Target.ID, Outcome: p.Outcome, ActorType: p.ActorType, ActorID: p.ActorID, Note: util.PtrToText(p.Note)})
		if err != nil {
			return out, err
		}
	}
	out.Instance, err = s.applyWorkflowTransitionInTx(ctx, q, run, plan)
	if err != nil {
		return out, err
	}
	if err = tx.Commit(ctx); err != nil {
		return out, err
	}
	return out, nil
}

func writeWorkflowStateInTx(ctx context.Context, q *db.Queries, run db.WorkflowInstance, status string, stageID pgtype.UUID) (db.WorkflowInstance, error) {
	return q.UpdateWorkflowInstanceState(ctx, db.UpdateWorkflowInstanceStateParams{ID: run.ID, WorkspaceID: run.WorkspaceID, Status: pgtype.Text{String: status, Valid: status != ""}, CurrentStageID: stageID})
}

// StartWorkflowInstanceInTx initializes the first stage and its execution intent with the same persistence path as subsequent transitions.
func (s *WorkflowExecutionService) StartWorkflowInstanceInTx(ctx context.Context, q *db.Queries, run db.WorkflowInstance, stage db.WorkflowInstanceStage) (db.WorkflowInstance, error) {
	if run.ArchivedAt.Valid || run.Status != "active" || run.CurrentStageID.Valid || stage.WorkflowInstanceID != run.ID || stage.WorkspaceID != run.WorkspaceID || stage.Position != 0 {
		return db.WorkflowInstance{}, ErrWorkflowExecutionConflict
	}
	return s.applyWorkflowTransitionInTx(ctx, q, run, WorkflowTransitionPlan{Status: "active", StageID: stage.ID, Target: stage, EnterStage: true})
}
