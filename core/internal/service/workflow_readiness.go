package service

import (
	"context"

	db "github.com/auenger/MEPERECO/core/pkg/db/generated"
	"github.com/auenger/MEPERECO/core/pkg/workflowcontract"
	"github.com/jackc/pgx/v5/pgtype"
)

// WorkflowExecutionReadiness is diagnostic evidence, not an acceptance token.
// Review/advance must recheck these conditions in their mutation transaction.
type WorkflowExecutionReadiness struct {
	Scope           string   `json:"scope"`
	ResultRevision  int32    `json:"result_revision"`
	ReadyForReview  bool     `json:"ready_for_review"`
	Blockers        []string `json:"blockers"`
	IncompleteTasks int64    `json:"incomplete_tasks"`
	ActiveCalls     int64    `json:"active_calls"`
}

func (s *WorkflowExecutionService) InspectReadiness(ctx context.Context, workspaceID, instanceID, executionID pgtype.UUID) (WorkflowExecutionReadiness, error) {
	row, err := s.Queries.GetWorkflowExecutionReadiness(ctx, db.GetWorkflowExecutionReadinessParams{WorkspaceID: workspaceID, WorkflowInstanceID: instanceID, ID: executionID})
	if err != nil {
		return WorkflowExecutionReadiness{}, err
	}
	return workflowCompletionConditions(row), nil
}

func workflowCompletionConditions(row db.GetWorkflowExecutionReadinessRow) WorkflowExecutionReadiness {
	result := WorkflowExecutionReadiness{Scope: "readiness_snapshot", ResultRevision: row.ResultRevision, Blockers: []string{}, IncompleteTasks: row.IncompleteTasks, ActiveCalls: row.ActiveCalls}
	result.Blockers = workflowcontract.EvaluateAgentCompletion(workflowcontract.AgentCompletionEvidence{
		Current: row.CurrentExecution.Valid && row.CurrentExecution.Bool, AwaitingReview: row.Status == "awaiting_review", HasResult: row.HasResult, MainTaskDone: row.MainTaskDone, MainRunCompleted: row.MainRunCompleted, IncompleteTasks: row.IncompleteTasks, ActiveCalls: row.ActiveCalls,
	})
	result.ReadyForReview = len(result.Blockers) == 0
	return result
}
