package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/auenger/MEPERECO/core/internal/logger"
	"github.com/auenger/MEPERECO/core/internal/service"
	"github.com/auenger/MEPERECO/core/internal/util"
	db "github.com/auenger/MEPERECO/core/pkg/db/generated"
	"github.com/auenger/MEPERECO/core/pkg/protocol"
	"github.com/auenger/MEPERECO/core/pkg/workflowcontract"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

const maxWorkflowStages = 50

var validWorkflowInstanceStatuses = map[string]bool{
	"draft": true, "active": true, "waiting": true,
	"completed": true, "cancelled": true,
}

type WorkflowStageInput struct {
	ID                  *string         `json:"id,omitempty"`
	StableKey           string          `json:"stable_key"`
	Name                string          `json:"name"`
	Description         *string         `json:"description"`
	ExecutionMode       string          `json:"execution_mode"`
	ExecutorID          *string         `json:"executor_id"`
	CompletionRule      json.RawMessage `json:"completion_rule"`
	ReworkPolicy        json.RawMessage `json:"rework_policy"`
	RetryPolicy         json.RawMessage `json:"retry_policy"`
	InputSpec           json.RawMessage `json:"input_spec"`
	InputBindings       json.RawMessage `json:"input_bindings"`
	OutputSpec          json.RawMessage `json:"output_spec"`
	RequiredSkills      []string        `json:"required_skills"`
	AllowedMCPServerIDs []string        `json:"allowed_mcp_server_ids"`
	Gate                json.RawMessage `json:"gate"`
	RollbackStageKey    *string         `json:"rollback_stage_key"`
}

type CreateWorkflowRequest struct {
	ProjectID   *string              `json:"project_id"`
	Name        string               `json:"name"`
	Description *string              `json:"description"`
	Publish     bool                 `json:"publish"`
	Stages      []WorkflowStageInput `json:"stages"`
}

type UpdateWorkflowRequest struct {
	ProjectID   *string `json:"project_id"`
	Name        *string `json:"name"`
	Description *string `json:"description"`
}

type CreateWorkflowVersionRequest struct {
	Publish bool                 `json:"publish"`
	Stages  []WorkflowStageInput `json:"stages"`
}

type CreateWorkflowInstanceRequest struct {
	Title       string          `json:"title"`
	Description *string         `json:"description"`
	VersionID   *string         `json:"version_id"`
	ProjectID   *string         `json:"project_id"`
	Start       *bool           `json:"start"`
	Inputs      json.RawMessage `json:"inputs"`
}

type TransitionWorkflowInstanceRequest struct {
	ExpectedStateToken string  `json:"expected_state_token"`
	TargetStageID      *string `json:"target_stage_id"`
	Outcome            string  `json:"outcome"`
	Note               *string `json:"note"`
}

type AttachWorkflowTaskRequest struct {
	StageID string `json:"stage_id"`
}

type WorkflowResponse struct {
	ID               string                   `json:"id"`
	WorkspaceID      string                   `json:"workspace_id"`
	ProjectID        *string                  `json:"project_id"`
	Name             string                   `json:"name"`
	Description      *string                  `json:"description"`
	Status           string                   `json:"status"`
	CurrentVersionID *string                  `json:"current_version_id"`
	CurrentVersion   *WorkflowVersionResponse `json:"current_version,omitempty"`
	CreatedBy        string                   `json:"created_by"`
	CreatedAt        string                   `json:"created_at"`
	UpdatedAt        string                   `json:"updated_at"`
}

type WorkflowVersionResponse struct {
	ID          string                  `json:"id"`
	WorkspaceID string                  `json:"workspace_id"`
	WorkflowID  string                  `json:"workflow_id"`
	Version     int32                   `json:"version"`
	Status      string                  `json:"status"`
	CreatedBy   string                  `json:"created_by"`
	PublishedAt *string                 `json:"published_at"`
	CreatedAt   string                  `json:"created_at"`
	Stages      []WorkflowStageResponse `json:"stages"`
}

type WorkflowStageResponse struct {
	ID                  string          `json:"id"`
	WorkspaceID         string          `json:"workspace_id"`
	WorkflowVersionID   string          `json:"workflow_version_id"`
	StableKey           string          `json:"stable_key"`
	Name                string          `json:"name"`
	Description         *string         `json:"description"`
	Position            int32           `json:"position"`
	ExecutionMode       string          `json:"execution_mode"`
	ExecutorID          *string         `json:"executor_id"`
	CompletionRule      json.RawMessage `json:"completion_rule"`
	ReworkPolicy        json.RawMessage `json:"rework_policy"`
	RetryPolicy         json.RawMessage `json:"retry_policy"`
	InputSpec           json.RawMessage `json:"input_spec"`
	InputBindings       json.RawMessage `json:"input_bindings"`
	OutputSpec          json.RawMessage `json:"output_spec"`
	RequiredSkills      []string        `json:"required_skills"`
	AllowedMCPServerIDs []string        `json:"allowed_mcp_server_ids"`
	Gate                json.RawMessage `json:"gate"`
	RollbackStageKey    *string         `json:"rollback_stage_key"`
	CreatedAt           string          `json:"created_at"`
}

type WorkflowReworkResponse struct {
	ID                string  `json:"id"`
	SourceStageID     string  `json:"source_stage_id"`
	TargetStageID     string  `json:"target_stage_id"`
	SourceExecutionID *string `json:"source_execution_id"`
	PlanRevision      int32   `json:"plan_revision"`
	ActorType         string  `json:"actor_type"`
	ActorID           string  `json:"actor_id"`
	Reason            string  `json:"reason"`
	CreatedAt         string  `json:"created_at"`
}

type WorkflowInstanceResponse struct {
	ManualReadiness    *service.ManualReadiness         `json:"manual_readiness,omitempty"`
	AutomaticReadiness *service.ManualReadiness         `json:"automatic_readiness,omitempty"`
	Reworks            []WorkflowReworkResponse         `json:"reworks,omitempty"`
	StateToken         string                           `json:"state_token"`
	ID                 string                           `json:"id"`
	WorkspaceID        string                           `json:"workspace_id"`
	WorkflowID         string                           `json:"workflow_id"`
	WorkflowVersionID  string                           `json:"workflow_version_id"`
	Title              string                           `json:"title"`
	Description        *string                          `json:"description"`
	Inputs             json.RawMessage                  `json:"inputs"`
	Revision           int32                            `json:"revision"`
	AcceptsNewTasks    bool                             `json:"accepts_new_tasks"`
	CanEdit            bool                             `json:"can_edit"`
	SourceVersion      int32                            `json:"source_version"`
	Status             string                           `json:"status"`
	CurrentStageID     *string                          `json:"current_stage_id"`
	CurrentStageName   *string                          `json:"current_stage_name"`
	CurrentStageIndex  *int32                           `json:"current_stage_index"`
	StageCount         int32                            `json:"stage_count"`
	ProjectID          *string                          `json:"project_id"`
	CreatedBy          string                           `json:"created_by"`
	StartedAt          *string                          `json:"started_at"`
	CompletedAt        *string                          `json:"completed_at"`
	TaskCount          int64                            `json:"task_count"`
	ArchivedAt         *string                          `json:"archived_at"`
	ArchivedBy         *string                          `json:"archived_by"`
	CreatedAt          string                           `json:"created_at"`
	UpdatedAt          string                           `json:"updated_at"`
	Stages             []WorkflowInstanceStageResponse  `json:"stages,omitempty"`
	Tasks              []TaskResponse                   `json:"tasks,omitempty"`
	Decisions          []WorkflowGateDecisionResponse   `json:"decisions,omitempty"`
	Changes            []WorkflowInstanceChangeResponse `json:"changes,omitempty"`
}

type WorkflowInstanceStageResponse struct {
	ID                     string            `json:"id"`
	WorkspaceID            string            `json:"workspace_id"`
	WorkflowInstanceID     string            `json:"workflow_instance_id"`
	SourceStageID          *string           `json:"source_stage_id"`
	StableKey              string            `json:"stable_key"`
	Name                   string            `json:"name"`
	Description            *string           `json:"description"`
	Position               int32             `json:"position"`
	ExecutionMode          string            `json:"execution_mode"`
	ExecutorID             *string           `json:"executor_id"`
	CompletionRule         json.RawMessage   `json:"completion_rule"`
	ReworkPolicy           json.RawMessage   `json:"rework_policy"`
	RetryPolicy            json.RawMessage   `json:"retry_policy"`
	InputSpec              json.RawMessage   `json:"input_spec"`
	InputBindings          json.RawMessage   `json:"input_bindings"`
	OutputSpec             json.RawMessage   `json:"output_spec"`
	RequiredSkills         []string          `json:"required_skills"`
	RequiredSkillRevisions map[string]string `json:"required_skill_revisions"`
	AllowedMCPServerIDs    []string          `json:"allowed_mcp_server_ids"`
	Gate                   json.RawMessage   `json:"gate"`
	RollbackStageKey       *string           `json:"rollback_stage_key"`
	CreatedAt              string            `json:"created_at"`
	UpdatedAt              string            `json:"updated_at"`
}

type WorkflowInstanceChangeResponse struct {
	ID         string          `json:"id"`
	Revision   int32           `json:"revision"`
	ChangedBy  string          `json:"changed_by"`
	ChangeNote *string         `json:"change_note"`
	BeforePlan json.RawMessage `json:"before_plan"`
	AfterPlan  json.RawMessage `json:"after_plan"`
	CreatedAt  string          `json:"created_at"`
}

type WorkflowGateDecisionResponse struct {
	InvalidatedByReworkID *string `json:"invalidated_by_rework_id"`
	ExecutionID           *string `json:"execution_id"`
	ResultRevision        *int32  `json:"result_revision"`
	ReviewStep            *string `json:"review_step"`
	BasisHash             *string `json:"basis_hash"`
	ID                    string  `json:"id"`
	WorkflowInstanceID    string  `json:"workflow_instance_id"`
	FromStageID           string  `json:"from_stage_id"`
	ToStageID             *string `json:"to_stage_id"`
	Outcome               string  `json:"outcome"`
	ActorType             string  `json:"actor_type"`
	ActorID               string  `json:"actor_id"`
	Note                  *string `json:"note"`
	CreatedAt             string  `json:"created_at"`
}

func workflowToResponse(row db.Workflow) WorkflowResponse {
	return WorkflowResponse{
		ID:               uuidToString(row.ID),
		WorkspaceID:      uuidToString(row.WorkspaceID),
		ProjectID:        uuidToPtr(row.ProjectID),
		Name:             row.Name,
		Description:      textToPtr(row.Description),
		Status:           row.Status,
		CurrentVersionID: uuidToPtr(row.CurrentVersionID),
		CreatedBy:        uuidToString(row.CreatedBy),
		CreatedAt:        timestampToString(row.CreatedAt),
		UpdatedAt:        timestampToString(row.UpdatedAt),
	}
}

func workflowVersionToResponse(row db.WorkflowVersion, stages []db.WorkflowStage) WorkflowVersionResponse {
	resp := WorkflowVersionResponse{
		ID:          uuidToString(row.ID),
		WorkspaceID: uuidToString(row.WorkspaceID),
		WorkflowID:  uuidToString(row.WorkflowID),
		Version:     row.Version,
		Status:      row.Status,
		CreatedBy:   uuidToString(row.CreatedBy),
		PublishedAt: timestampToPtr(row.PublishedAt),
		CreatedAt:   timestampToString(row.CreatedAt),
		Stages:      make([]WorkflowStageResponse, len(stages)),
	}
	for i, stage := range stages {
		resp.Stages[i] = workflowStageToResponse(stage)
	}
	return resp
}

func workflowStageToResponse(row db.WorkflowStage) WorkflowStageResponse {
	requiredSkills := row.RequiredSkills
	if requiredSkills == nil {
		requiredSkills = []string{}
	}
	return WorkflowStageResponse{
		ID:                uuidToString(row.ID),
		WorkspaceID:       uuidToString(row.WorkspaceID),
		WorkflowVersionID: uuidToString(row.WorkflowVersionID),
		StableKey:         row.StableKey,
		Name:              row.Name,
		Description:       textToPtr(row.Description),
		Position:          row.Position,
		ExecutionMode:     row.ExecutionMode, ExecutorID: uuidToPtr(row.ExecutorID),
		CompletionRule: jsonOrEmptyObject(row.CompletionRule), ReworkPolicy: jsonOrEmptyObject(row.ReworkPolicy), RetryPolicy: jsonOrEmptyObject(row.RetryPolicy),
		InputSpec: jsonOrEmptyObject(row.InputSpec), InputBindings: jsonOrEmptyObject(row.InputBindings),
		OutputSpec:          jsonOrEmptyObject(row.OutputSpec),
		RequiredSkills:      requiredSkills,
		AllowedMCPServerIDs: uuidsToStrings(row.AllowedMcpServerIds),
		Gate:                jsonOrEmptyObject(row.Gate),
		RollbackStageKey:    textToPtr(row.RollbackStageKey),
		CreatedAt:           timestampToString(row.CreatedAt),
	}
}

// workflowInstanceStateToken binds manual decisions to the observed Run state.
// Use full database timestamp precision; display timestamps may omit fractions.
func workflowInstanceStateToken(row db.WorkflowInstance) string {
	return service.WorkflowStateToken(row)
}

func workflowInstanceToResponse(row db.WorkflowInstance) WorkflowInstanceResponse {
	return WorkflowInstanceResponse{
		StateToken:        workflowInstanceStateToken(row),
		ID:                uuidToString(row.ID),
		WorkspaceID:       uuidToString(row.WorkspaceID),
		WorkflowID:        uuidToString(row.WorkflowID),
		WorkflowVersionID: uuidToString(row.WorkflowVersionID),
		Title:             row.Title,
		Description:       textToPtr(row.Description),
		Inputs:            jsonOrEmptyObject(row.Inputs),
		Revision:          row.Revision,
		Status:            row.Status,
		CurrentStageID:    uuidToPtr(row.CurrentStageID),
		ProjectID:         uuidToPtr(row.ProjectID),
		CreatedBy:         uuidToString(row.CreatedBy),
		StartedAt:         timestampToPtr(row.StartedAt),
		CompletedAt:       timestampToPtr(row.CompletedAt),
		ArchivedAt:        timestampToPtr(row.ArchivedAt),
		ArchivedBy:        uuidToPtr(row.ArchivedBy),
		CreatedAt:         timestampToString(row.CreatedAt),
		UpdatedAt:         timestampToString(row.UpdatedAt),
	}
}

func workflowInstanceStageToResponse(row db.WorkflowInstanceStage) WorkflowInstanceStageResponse {
	requiredSkills := row.RequiredSkills
	if requiredSkills == nil {
		requiredSkills = []string{}
	}
	revisions := map[string]string{}
	var snapshots []service.AgentSkillData
	if json.Unmarshal(row.RequiredSkillSnapshots, &snapshots) == nil {
		_, refs := service.BuildAgentSkillBundles(snapshots)
		for _, ref := range refs {
			revisions[ref.ID] = ref.Hash
		}
	}
	return WorkflowInstanceStageResponse{
		ID: uuidToString(row.ID), WorkspaceID: uuidToString(row.WorkspaceID),
		WorkflowInstanceID: uuidToString(row.WorkflowInstanceID), SourceStageID: uuidToPtr(row.SourceStageID),
		StableKey: row.StableKey, Name: row.Name, Description: textToPtr(row.Description), Position: row.Position,
		ExecutionMode: row.ExecutionMode, ExecutorID: uuidToPtr(row.ExecutorID),
		CompletionRule: jsonOrEmptyObject(row.CompletionRule), ReworkPolicy: jsonOrEmptyObject(row.ReworkPolicy), RetryPolicy: jsonOrEmptyObject(row.RetryPolicy), InputSpec: jsonOrEmptyObject(row.InputSpec), InputBindings: jsonOrEmptyObject(row.InputBindings),
		OutputSpec: jsonOrEmptyObject(row.OutputSpec), RequiredSkills: requiredSkills, RequiredSkillRevisions: revisions,
		AllowedMCPServerIDs: uuidsToStrings(row.AllowedMcpServerIds),
		Gate:                jsonOrEmptyObject(row.Gate), RollbackStageKey: textToPtr(row.RollbackStageKey),
		CreatedAt: timestampToString(row.CreatedAt), UpdatedAt: timestampToString(row.UpdatedAt),
	}
}

func workflowInstanceChangeToResponse(row db.WorkflowInstanceChange) WorkflowInstanceChangeResponse {
	return WorkflowInstanceChangeResponse{
		ID: uuidToString(row.ID), Revision: row.Revision, ChangedBy: uuidToString(row.ChangedBy),
		ChangeNote: textToPtr(row.ChangeNote), BeforePlan: jsonOrEmptyObject(row.BeforePlan),
		AfterPlan: jsonOrEmptyObject(row.AfterPlan), CreatedAt: timestampToString(row.CreatedAt),
	}
}

func workflowDecisionToResponse(row db.WorkflowGateDecision) WorkflowGateDecisionResponse {
	var revision *int32
	if row.ResultRevision.Valid {
		value := row.ResultRevision.Int32
		revision = &value
	}
	return WorkflowGateDecisionResponse{
		InvalidatedByReworkID: uuidToPtr(row.InvalidatedByReworkID), ExecutionID: uuidToPtr(row.ExecutionID), ResultRevision: revision, ReviewStep: textToPtr(row.ReviewStep), BasisHash: textToPtr(row.BasisHash),
		ID:                 uuidToString(row.ID),
		WorkflowInstanceID: uuidToString(row.WorkflowInstanceID),
		FromStageID:        uuidToString(row.FromStageID),
		ToStageID:          uuidToPtr(row.ToStageID),
		Outcome:            row.Outcome,
		ActorType:          row.ActorType,
		ActorID:            uuidToString(row.ActorID),
		Note:               textToPtr(row.Note),
		CreatedAt:          timestampToString(row.CreatedAt),
	}
}

func jsonOrEmptyObject(raw []byte) json.RawMessage {
	if len(raw) == 0 || !json.Valid(raw) {
		return json.RawMessage(`{}`)
	}
	return json.RawMessage(raw)
}

func normalizeObject(raw json.RawMessage, fallback string) ([]byte, error) {
	if len(raw) == 0 {
		return []byte(fallback), nil
	}
	var value map[string]any
	if err := json.Unmarshal(raw, &value); err != nil || value == nil {
		return nil, errors.New("must be a JSON object")
	}
	return raw, nil
}

func normalizeWorkflowStages(stages []WorkflowStageInput) ([]WorkflowStageInput, error) {
	if len(stages) == 0 {
		return nil, errors.New("at least one stage is required")
	}
	if len(stages) > maxWorkflowStages {
		return nil, fmt.Errorf("a workflow may contain at most %d stages", maxWorkflowStages)
	}

	seen := make(map[string]int, len(stages))
	for i := range stages {
		stages[i].Name = strings.TrimSpace(stages[i].Name)
		if stages[i].Name == "" {
			return nil, fmt.Errorf("stages[%d].name is required", i)
		}
		stages[i].StableKey = strings.TrimSpace(stages[i].StableKey)
		if stages[i].StableKey == "" {
			stages[i].StableKey = fmt.Sprintf("stage_%d", i+1)
		}
		if previous, ok := seen[stages[i].StableKey]; ok {
			return nil, fmt.Errorf("stages[%d].stable_key duplicates stages[%d]", i, previous)
		}
		seen[stages[i].StableKey] = i

		bindings, err := workflowcontract.OptionalInputBindings(stages[i].InputBindings)
		if err != nil {
			return nil, fmt.Errorf("stages[%d].input_bindings: %w", i, err)
		}
		if bindings != nil {
			contract, err := workflowcontract.FromSpec(stages[i].InputSpec, false)
			if err != nil {
				return nil, fmt.Errorf("stages[%d].input_spec: %w", i, err)
			}
			if contract == nil {
				return nil, fmt.Errorf("stages[%d].input_spec.contract is required for input bindings", i)
			}
			if err := bindings.CheckContract(*contract); err != nil {
				return nil, fmt.Errorf("stages[%d].input_bindings: %w", i, err)
			}
			for _, binding := range bindings.Data {
				if binding.Source.Kind == "stage_result" {
					previous, exists := seen[binding.Source.StageKey]
					if !exists || previous >= i {
						return nil, fmt.Errorf("stages[%d].input_bindings must reference a preceding stage", i)
					}
				}
			}
		}
		policy, policyErr := workflowcontract.ParseReworkPolicy(stages[i].ReworkPolicy)
		if policyErr != nil {
			return nil, fmt.Errorf("stages[%d]: %w", i, policyErr)
		}
		stages[i].ReworkPolicy, _ = json.Marshal(policy)
		retryPolicy, retryErr := workflowcontract.ParseRetryPolicy(stages[i].RetryPolicy)
		if retryErr != nil {
			return nil, fmt.Errorf("stages[%d]: %w", i, retryErr)
		}
		stages[i].RetryPolicy, _ = json.Marshal(retryPolicy)
		if stages[i].ExecutionMode == "" {
			stages[i].ExecutionMode = "manual"
		}
		switch stages[i].ExecutionMode {
		case "manual":
			if stages[i].ExecutorID != nil {
				return nil, fmt.Errorf("stages[%d].executor_id requires agent execution_mode", i)
			}
		case "agent":
			if stages[i].ExecutorID == nil {
				return nil, fmt.Errorf("stages[%d].executor_id is required", i)
			}
			if _, err := util.ParseUUID(*stages[i].ExecutorID); err != nil {
				return nil, fmt.Errorf("stages[%d].executor_id is invalid", i)
			}
			if _, err := workflowcontract.FromSpec(stages[i].OutputSpec, true); err != nil {
				return nil, fmt.Errorf("stages[%d].output_spec: %w", i, err)
			}
			inputContract, err := workflowcontract.FromSpec(stages[i].InputSpec, false)
			if err != nil {
				return nil, fmt.Errorf("stages[%d].input_spec: %w", i, err)
			}
			if inputContract != nil && len(inputContract.Artifacts) > 0 {
				return nil, fmt.Errorf("stages[%d].input_spec artifact bindings are not supported yet", i)
			}

		default:
			return nil, fmt.Errorf("stages[%d].execution_mode must be manual or agent", i)
		}
		completion, err := workflowcontract.ParseCompletionPolicy(stages[i].CompletionRule, stages[i].ExecutionMode)
		if err != nil {
			return nil, fmt.Errorf("stages[%d]: %w", i, err)
		}
		if err := completion.ValidateWrite(); err != nil {
			return nil, fmt.Errorf("stages[%d]: %w", i, err)
		}
		stages[i].CompletionRule, _ = json.Marshal(completion)

		if _, err := normalizeObject(stages[i].InputSpec, `{}`); err != nil {
			return nil, fmt.Errorf("stages[%d].input_spec %w", i, err)
		}
		if _, err := normalizeObject(stages[i].OutputSpec, `{}`); err != nil {
			return nil, fmt.Errorf("stages[%d].output_spec %w", i, err)
		}
		gate, err := normalizeObject(stages[i].Gate, `{"type":"none"}`)
		if err != nil {
			return nil, fmt.Errorf("stages[%d].gate %w", i, err)
		}
		var gateValue struct {
			Type string `json:"type"`
		}
		if err := json.Unmarshal(gate, &gateValue); err != nil {
			return nil, fmt.Errorf("stages[%d].gate is invalid", i)
		}
		switch gateValue.Type {
		case "", "none", "human", "agent", "hybrid":
		default:
			return nil, fmt.Errorf("stages[%d].gate.type must be none, human, agent, or hybrid", i)
		}
		if stages[i].RollbackStageKey != nil {
			key := strings.TrimSpace(*stages[i].RollbackStageKey)
			if key == "" {
				stages[i].RollbackStageKey = nil
			} else {
				stages[i].RollbackStageKey = &key
			}
		}
		seenMCP := make(map[string]struct{}, len(stages[i].AllowedMCPServerIDs))
		for j, id := range stages[i].AllowedMCPServerIDs {
			id = strings.TrimSpace(id)
			if _, err := util.ParseUUID(id); err != nil {
				return nil, fmt.Errorf("stages[%d].allowed_mcp_server_ids[%d] is invalid", i, j)
			}
			if _, duplicate := seenMCP[id]; duplicate {
				return nil, fmt.Errorf("stages[%d].allowed_mcp_server_ids contains a duplicate", i)
			}
			seenMCP[id] = struct{}{}
			stages[i].AllowedMCPServerIDs[j] = id
		}
	}

	for i, stage := range stages {
		if stage.RollbackStageKey == nil {
			continue
		}
		target, ok := seen[*stage.RollbackStageKey]
		if !ok {
			return nil, fmt.Errorf("stages[%d].rollback_stage_key does not exist", i)
		}
		if target >= i {
			return nil, fmt.Errorf("stages[%d].rollback_stage_key must point to an earlier stage", i)
		}
	}
	return stages, nil
}

func parseUUIDArray(ids []string) []pgtype.UUID {
	if ids == nil {
		return nil
	}
	out := make([]pgtype.UUID, len(ids))
	for i, id := range ids {
		out[i] = parseUUID(id)
	}
	return out
}

func createWorkflowStages(ctx context.Context, q *db.Queries, workspaceID, versionID pgtype.UUID, stages []WorkflowStageInput) ([]db.WorkflowStage, error) {
	rows := make([]db.WorkflowStage, 0, len(stages))
	for i, stage := range stages {
		completionRule, _ := normalizeObject(stage.CompletionRule, `{"type":"all_tasks_terminal"}`)
		inputSpec, _ := normalizeObject(stage.InputSpec, `{}`)
		inputBindings, _ := normalizeObject(stage.InputBindings, `{}`)
		outputSpec, _ := normalizeObject(stage.OutputSpec, `{}`)
		gate, _ := normalizeObject(stage.Gate, `{"type":"none"}`)
		requiredSkills := stage.RequiredSkills
		if requiredSkills == nil {
			requiredSkills = []string{}
		}
		row, err := q.CreateWorkflowStage(ctx, db.CreateWorkflowStageParams{
			WorkspaceID:       workspaceID,
			WorkflowVersionID: versionID,
			StableKey:         stage.StableKey,
			Name:              stage.Name,
			Description:       ptrToText(stage.Description),
			Position:          int32(i),
			ExecutionMode:     pgtype.Text{String: stage.ExecutionMode, Valid: true}, ExecutorID: workflowExecutorUUID(stage),
			CompletionRule: completionRule, ReworkPolicy: stage.ReworkPolicy, RetryPolicy: stage.RetryPolicy,
			InputSpec: inputSpec, InputBindings: inputBindings,
			OutputSpec:          outputSpec,
			RequiredSkills:      requiredSkills,
			Gate:                gate,
			RollbackStageKey:    ptrToText(stage.RollbackStageKey),
			AllowedMcpServerIds: parseUUIDArray(stage.AllowedMCPServerIDs),
		})
		if err != nil {
			return nil, err
		}
		rows = append(rows, row)
	}
	return rows, nil
}

func (h *Handler) requireWorkflowAdmin(w http.ResponseWriter, r *http.Request) (string, string, pgtype.UUID, pgtype.UUID, bool) {
	workspaceID := h.resolveWorkspaceID(r)
	userID, ok := requireUserID(w, r)
	if !ok {
		return "", "", pgtype.UUID{}, pgtype.UUID{}, false
	}
	if actorType, _ := h.resolveActor(r, userID, workspaceID); actorType == "agent" {
		writeError(w, http.StatusForbidden, "agents cannot manage workflow definitions")
		return "", "", pgtype.UUID{}, pgtype.UUID{}, false
	}
	if _, roleOK := h.requireWorkspaceRole(w, r, workspaceID, "workspace not found", "owner", "admin"); !roleOK {
		return "", "", pgtype.UUID{}, pgtype.UUID{}, false
	}
	wsUUID, ok := parseUUIDOrBadRequest(w, workspaceID, "workspace id")
	if !ok {
		return "", "", pgtype.UUID{}, pgtype.UUID{}, false
	}
	userUUID, ok := parseUUIDOrBadRequest(w, userID, "user id")
	if !ok {
		return "", "", pgtype.UUID{}, pgtype.UUID{}, false
	}
	return workspaceID, userID, wsUUID, userUUID, true
}

func (h *Handler) requireWorkflowMember(w http.ResponseWriter, r *http.Request) (string, string, pgtype.UUID, bool) {
	workspaceID := h.resolveWorkspaceID(r)
	userID, ok := requireUserID(w, r)
	if !ok {
		return "", "", pgtype.UUID{}, false
	}
	if _, memberOK := h.workspaceMember(w, r, workspaceID); !memberOK {
		return "", "", pgtype.UUID{}, false
	}
	wsUUID, ok := parseUUIDOrBadRequest(w, workspaceID, "workspace id")
	if !ok {
		return "", "", pgtype.UUID{}, false
	}
	return workspaceID, userID, wsUUID, true
}

func (h *Handler) loadWorkflowCurrentVersion(ctx context.Context, workflow db.Workflow) (*WorkflowVersionResponse, error) {
	if !workflow.CurrentVersionID.Valid {
		return nil, nil
	}
	version, err := h.Queries.GetWorkflowVersionInWorkspace(ctx, db.GetWorkflowVersionInWorkspaceParams{
		ID: workflow.CurrentVersionID, WorkflowID: workflow.ID, WorkspaceID: workflow.WorkspaceID,
	})
	if err != nil {
		return nil, err
	}
	stages, err := h.Queries.ListWorkflowStages(ctx, db.ListWorkflowStagesParams{
		WorkflowVersionID: version.ID, WorkspaceID: workflow.WorkspaceID,
	})
	if err != nil {
		return nil, err
	}
	resp := workflowVersionToResponse(version, stages)
	return &resp, nil
}

func (h *Handler) ListWorkflows(w http.ResponseWriter, r *http.Request) {
	_, _, wsUUID, ok := h.requireWorkflowMember(w, r)
	if !ok {
		return
	}
	var projectID pgtype.UUID
	if value := strings.TrimSpace(r.URL.Query().Get("project_id")); value != "" {
		projectID, ok = parseUUIDOrBadRequest(w, value, "project_id")
		if !ok {
			return
		}
	}
	rows, err := h.Queries.ListWorkflows(r.Context(), db.ListWorkflowsParams{
		WorkspaceID: wsUUID, ProjectID: projectID,
		IncludeArchived: r.URL.Query().Get("include_archived") == "true",
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list workflows")
		return
	}
	visible := rows[:0]
	for _, row := range rows {
		if h.projectAccessible(r, wsUUID, row.ProjectID) {
			visible = append(visible, row)
		}
	}
	rows = visible
	resp := make([]WorkflowResponse, len(rows))
	for i, row := range rows {
		resp[i] = workflowToResponse(row)
		current, loadErr := h.loadWorkflowCurrentVersion(r.Context(), row)
		if loadErr != nil {
			slog.Warn("failed to load current workflow version", "workflow_id", resp[i].ID, "error", loadErr)
		} else {
			resp[i].CurrentVersion = current
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"workflows": resp, "total": len(resp)})
}

func (h *Handler) GetWorkflow(w http.ResponseWriter, r *http.Request) {
	_, _, wsUUID, ok := h.requireWorkflowMember(w, r)
	if !ok {
		return
	}
	workflowID, ok := parseUUIDOrBadRequest(w, chi.URLParam(r, "id"), "workflow id")
	if !ok {
		return
	}
	row, err := h.Queries.GetWorkflowInWorkspace(r.Context(), db.GetWorkflowInWorkspaceParams{
		ID: workflowID, WorkspaceID: wsUUID,
	})
	if err != nil {
		writeError(w, http.StatusNotFound, "workflow not found")
		return
	}
	resp := workflowToResponse(row)
	current, err := h.loadWorkflowCurrentVersion(r.Context(), row)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load workflow version")
		return
	}
	resp.CurrentVersion = current
	writeJSON(w, http.StatusOK, resp)
}

func (h *Handler) CreateWorkflow(w http.ResponseWriter, r *http.Request) {
	workspaceID, userID, wsUUID, userUUID, ok := h.requireWorkflowAdmin(w, r)
	if !ok {
		return
	}
	var req CreateWorkflowRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}
	var projectID pgtype.UUID
	if req.ProjectID != nil && strings.TrimSpace(*req.ProjectID) != "" {
		projectID, ok = parseUUIDOrBadRequest(w, strings.TrimSpace(*req.ProjectID), "project_id")
		if !ok {
			return
		}
		if _, err := h.Queries.GetProjectInWorkspace(r.Context(), db.GetProjectInWorkspaceParams{
			ID: projectID, WorkspaceID: wsUUID,
		}); err != nil {
			writeError(w, http.StatusBadRequest, "project not found")
			return
		}
	}
	stages, err := normalizeWorkflowStages(req.Stages)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := validateWorkflowStageExecutors(r.Context(), h.Queries, wsUUID, stages); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	tx, err := h.TxStarter.Begin(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to start transaction")
		return
	}
	defer tx.Rollback(r.Context())
	qtx := h.Queries.WithTx(tx)

	workflow, err := qtx.CreateWorkflow(r.Context(), db.CreateWorkflowParams{
		WorkspaceID: wsUUID, ProjectID: projectID,
		Name: req.Name, Description: ptrToText(req.Description),
		Status: "draft", CreatedBy: userUUID,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create workflow")
		return
	}
	version, err := qtx.CreateWorkflowVersion(r.Context(), db.CreateWorkflowVersionParams{
		WorkspaceID: wsUUID, WorkflowID: workflow.ID, Version: 1,
		Status: "draft", CreatedBy: userUUID,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create workflow version")
		return
	}
	stageRows, err := createWorkflowStages(r.Context(), qtx, wsUUID, version.ID, stages)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create workflow stages")
		return
	}
	if req.Publish {
		version, err = qtx.PublishWorkflowVersion(r.Context(), db.PublishWorkflowVersionParams{
			ID: version.ID, WorkflowID: workflow.ID, WorkspaceID: wsUUID,
		})
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to publish workflow")
			return
		}
		workflow, err = qtx.SetWorkflowCurrentVersion(r.Context(), db.SetWorkflowCurrentVersionParams{
			ID: workflow.ID, WorkspaceID: wsUUID, CurrentVersionID: version.ID, Status: "published",
		})
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to activate workflow version")
			return
		}
	}
	if err := tx.Commit(r.Context()); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to commit workflow")
		return
	}
	resp := workflowToResponse(workflow)
	versionResp := workflowVersionToResponse(version, stageRows)
	resp.CurrentVersion = &versionResp
	if !req.Publish {
		resp.CurrentVersionID = nil
	}
	h.publish(protocol.EventWorkflowCreated, workspaceID, "member", userID, map[string]any{"workflow": resp})
	writeJSON(w, http.StatusCreated, resp)
}

func (h *Handler) UpdateWorkflow(w http.ResponseWriter, r *http.Request) {
	workspaceID, userID, wsUUID, _, ok := h.requireWorkflowAdmin(w, r)
	if !ok {
		return
	}
	workflowID, ok := parseUUIDOrBadRequest(w, chi.URLParam(r, "id"), "workflow id")
	if !ok {
		return
	}
	current, err := h.Queries.GetWorkflowInWorkspace(r.Context(), db.GetWorkflowInWorkspaceParams{
		ID: workflowID, WorkspaceID: wsUUID,
	})
	if err != nil {
		writeError(w, http.StatusNotFound, "workflow not found")
		return
	}
	var req UpdateWorkflowRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Name != nil {
		name := strings.TrimSpace(*req.Name)
		if name == "" {
			writeError(w, http.StatusBadRequest, "name cannot be empty")
			return
		}
		req.Name = &name
	}
	projectID := current.ProjectID
	if req.ProjectID != nil {
		value := strings.TrimSpace(*req.ProjectID)
		if value == "" {
			writeError(w, http.StatusBadRequest, "project_id cannot be empty")
			return
		}
		projectID, ok = parseUUIDOrBadRequest(w, value, "project_id")
		if !ok {
			return
		}
		if _, err := h.Queries.GetProjectInWorkspace(r.Context(), db.GetProjectInWorkspaceParams{
			ID: projectID, WorkspaceID: wsUUID,
		}); err != nil {
			writeError(w, http.StatusBadRequest, "project not found")
			return
		}
		if current.ProjectID.Valid && current.ProjectID != projectID {
			instanceCount, countErr := h.Queries.CountWorkflowInstances(r.Context(), db.CountWorkflowInstancesParams{
				WorkflowID: workflowID, WorkspaceID: wsUUID,
			})
			if countErr != nil {
				writeError(w, http.StatusInternalServerError, "failed to validate workflow project")
				return
			}
			if instanceCount > 0 {
				writeError(w, http.StatusConflict, "workflow cannot move projects after a run has been created")
				return
			}
		}
	}
	description := textToPtr(current.Description)
	if req.Description != nil {
		description = req.Description
	}
	row, err := h.Queries.UpdateWorkflowMetadata(r.Context(), db.UpdateWorkflowMetadataParams{
		ID: workflowID, WorkspaceID: wsUUID, ProjectID: projectID,
		Name: ptrToText(req.Name), Description: ptrToText(description),
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update workflow")
		return
	}
	resp := workflowToResponse(row)
	resp.CurrentVersion, _ = h.loadWorkflowCurrentVersion(r.Context(), row)
	h.publish(protocol.EventWorkflowUpdated, workspaceID, "member", userID, map[string]any{"workflow": resp})
	writeJSON(w, http.StatusOK, resp)
}

func (h *Handler) ArchiveWorkflow(w http.ResponseWriter, r *http.Request) {
	workspaceID, userID, wsUUID, _, ok := h.requireWorkflowAdmin(w, r)
	if !ok {
		return
	}
	workflowID, ok := parseUUIDOrBadRequest(w, chi.URLParam(r, "id"), "workflow id")
	if !ok {
		return
	}
	tx, err := h.TxStarter.Begin(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to start workflow deletion")
		return
	}
	defer tx.Rollback(r.Context())
	qtx := h.Queries.WithTx(tx)
	if _, err := qtx.LockWorkflowForVersion(r.Context(), db.LockWorkflowForVersionParams{ID: workflowID, WorkspaceID: wsUUID}); err != nil {
		writeError(w, http.StatusNotFound, "workflow not found")
		return
	}
	if r.URL.Query().Get("unused_only") == "true" {
		count, err := qtx.CountWorkflowInstances(r.Context(), db.CountWorkflowInstancesParams{WorkflowID: workflowID, WorkspaceID: wsUUID})
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to check workflow runs")
			return
		}
		if count > 0 {
			writeError(w, http.StatusConflict, "workflow has existing runs")
			return
		}
	}
	row, err := qtx.ArchiveWorkflow(r.Context(), db.ArchiveWorkflowParams{
		ID: workflowID, WorkspaceID: wsUUID,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete workflow")
		return
	}
	if err := tx.Commit(r.Context()); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to commit workflow deletion")
		return
	}
	resp := workflowToResponse(row)
	h.publish(protocol.EventWorkflowUpdated, workspaceID, "member", userID, map[string]any{"workflow": resp})
	writeJSON(w, http.StatusOK, resp)
}

func (h *Handler) ListWorkflowVersions(w http.ResponseWriter, r *http.Request) {
	_, _, wsUUID, ok := h.requireWorkflowMember(w, r)
	if !ok {
		return
	}
	workflowID, ok := parseUUIDOrBadRequest(w, chi.URLParam(r, "id"), "workflow id")
	if !ok {
		return
	}
	if _, err := h.Queries.GetWorkflowInWorkspace(r.Context(), db.GetWorkflowInWorkspaceParams{
		ID: workflowID, WorkspaceID: wsUUID,
	}); err != nil {
		writeError(w, http.StatusNotFound, "workflow not found")
		return
	}
	rows, err := h.Queries.ListWorkflowVersions(r.Context(), db.ListWorkflowVersionsParams{
		WorkflowID: workflowID, WorkspaceID: wsUUID,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list workflow versions")
		return
	}
	resp := make([]WorkflowVersionResponse, 0, len(rows))
	for _, row := range rows {
		stages, loadErr := h.Queries.ListWorkflowStages(r.Context(), db.ListWorkflowStagesParams{
			WorkflowVersionID: row.ID, WorkspaceID: wsUUID,
		})
		if loadErr != nil {
			writeError(w, http.StatusInternalServerError, "failed to load workflow stages")
			return
		}
		resp = append(resp, workflowVersionToResponse(row, stages))
	}
	writeJSON(w, http.StatusOK, map[string]any{"versions": resp, "total": len(resp)})
}

func (h *Handler) CreateWorkflowVersion(w http.ResponseWriter, r *http.Request) {
	workspaceID, userID, wsUUID, userUUID, ok := h.requireWorkflowAdmin(w, r)
	if !ok {
		return
	}
	workflowID, ok := parseUUIDOrBadRequest(w, chi.URLParam(r, "id"), "workflow id")
	if !ok {
		return
	}
	var req CreateWorkflowVersionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	stages, err := normalizeWorkflowStages(req.Stages)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := validateWorkflowStageExecutors(r.Context(), h.Queries, wsUUID, stages); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	tx, err := h.TxStarter.Begin(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to start transaction")
		return
	}
	defer tx.Rollback(r.Context())
	qtx := h.Queries.WithTx(tx)
	workflow, err := qtx.LockWorkflowForVersion(r.Context(), db.LockWorkflowForVersionParams{
		ID: workflowID, WorkspaceID: wsUUID,
	})
	if err != nil || workflow.Status == "archived" {
		writeError(w, http.StatusNotFound, "workflow not found")
		return
	}
	next, err := qtx.NextWorkflowVersionNumber(r.Context(), db.NextWorkflowVersionNumberParams{
		WorkflowID: workflowID, WorkspaceID: wsUUID,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to allocate workflow version")
		return
	}
	version, err := qtx.CreateWorkflowVersion(r.Context(), db.CreateWorkflowVersionParams{
		WorkspaceID: wsUUID, WorkflowID: workflowID, Version: next,
		Status: "draft", CreatedBy: userUUID,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create workflow version")
		return
	}
	stageRows, err := createWorkflowStages(r.Context(), qtx, wsUUID, version.ID, stages)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create workflow stages")
		return
	}
	if req.Publish {
		if err := qtx.SupersedeOtherWorkflowVersions(r.Context(), db.SupersedeOtherWorkflowVersionsParams{
			WorkflowID: workflowID, WorkspaceID: wsUUID, ID: version.ID,
		}); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to supersede workflow version")
			return
		}
		version, err = qtx.PublishWorkflowVersion(r.Context(), db.PublishWorkflowVersionParams{
			ID: version.ID, WorkflowID: workflowID, WorkspaceID: wsUUID,
		})
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to publish workflow version")
			return
		}
		if _, err = qtx.SetWorkflowCurrentVersion(r.Context(), db.SetWorkflowCurrentVersionParams{
			ID: workflowID, WorkspaceID: wsUUID, CurrentVersionID: version.ID, Status: "published",
		}); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to activate workflow version")
			return
		}
	}
	if err := tx.Commit(r.Context()); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to commit workflow version")
		return
	}
	resp := workflowVersionToResponse(version, stageRows)
	h.publish(protocol.EventWorkflowUpdated, workspaceID, "member", userID, map[string]any{
		"workflow_id": uuidToString(workflowID), "version": resp,
	})
	writeJSON(w, http.StatusCreated, resp)
}

func (h *Handler) PublishWorkflowVersion(w http.ResponseWriter, r *http.Request) {
	workspaceID, userID, wsUUID, _, ok := h.requireWorkflowAdmin(w, r)
	if !ok {
		return
	}
	workflowID, ok := parseUUIDOrBadRequest(w, chi.URLParam(r, "id"), "workflow id")
	if !ok {
		return
	}
	versionID, ok := parseUUIDOrBadRequest(w, chi.URLParam(r, "versionId"), "workflow version id")
	if !ok {
		return
	}
	tx, err := h.TxStarter.Begin(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to start transaction")
		return
	}
	defer tx.Rollback(r.Context())
	qtx := h.Queries.WithTx(tx)
	if _, err := qtx.LockWorkflowForVersion(r.Context(), db.LockWorkflowForVersionParams{
		ID: workflowID, WorkspaceID: wsUUID,
	}); err != nil {
		writeError(w, http.StatusNotFound, "workflow not found")
		return
	}
	stages, err := qtx.ListWorkflowStages(r.Context(), db.ListWorkflowStagesParams{
		WorkflowVersionID: versionID, WorkspaceID: wsUUID,
	})
	if err != nil || len(stages) == 0 {
		writeError(w, http.StatusBadRequest, "workflow version has no stages")
		return
	}
	version, err := qtx.PublishWorkflowVersion(r.Context(), db.PublishWorkflowVersionParams{
		ID: versionID, WorkflowID: workflowID, WorkspaceID: wsUUID,
	})
	if err != nil {
		writeError(w, http.StatusNotFound, "workflow version not found")
		return
	}
	if err := qtx.SupersedeOtherWorkflowVersions(r.Context(), db.SupersedeOtherWorkflowVersionsParams{
		WorkflowID: workflowID, WorkspaceID: wsUUID, ID: versionID,
	}); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to supersede workflow version")
		return
	}
	if _, err := qtx.SetWorkflowCurrentVersion(r.Context(), db.SetWorkflowCurrentVersionParams{
		ID: workflowID, WorkspaceID: wsUUID, CurrentVersionID: versionID, Status: "published",
	}); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to activate workflow version")
		return
	}
	if err := tx.Commit(r.Context()); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to commit workflow publication")
		return
	}
	resp := workflowVersionToResponse(version, stages)
	h.publish(protocol.EventWorkflowUpdated, workspaceID, "member", userID, map[string]any{
		"workflow_id": uuidToString(workflowID), "version": resp,
	})
	writeJSON(w, http.StatusOK, resp)
}

func (h *Handler) ListWorkflowInstances(w http.ResponseWriter, r *http.Request) {
	_, userID, wsUUID, ok := h.requireWorkflowMember(w, r)
	if !ok {
		return
	}
	var workflowID pgtype.UUID
	if value := r.URL.Query().Get("workflow_id"); value != "" {
		workflowID, ok = parseUUIDOrBadRequest(w, value, "workflow_id")
		if !ok {
			return
		}
	}
	var projectID pgtype.UUID
	if value := r.URL.Query().Get("project_id"); value != "" {
		projectID, ok = parseUUIDOrBadRequest(w, value, "project_id")
		if !ok {
			return
		}
	}
	var status pgtype.Text
	if value := r.URL.Query().Get("status"); value != "" {
		if !validWorkflowInstanceStatuses[value] {
			writeError(w, http.StatusBadRequest, "invalid workflow instance status")
			return
		}
		status = pgtype.Text{String: value, Valid: true}
	}
	rows, err := h.Queries.ListWorkflowInstances(r.Context(), db.ListWorkflowInstancesParams{
		WorkspaceID: wsUUID, WorkflowID: workflowID, ProjectID: projectID, Status: status,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list workflow instances")
		return
	}
	visible := rows[:0]
	for _, row := range rows {
		if h.projectAccessible(r, wsUUID, row.ProjectID) {
			visible = append(visible, row)
		}
	}
	rows = visible
	resp := make([]WorkflowInstanceResponse, len(rows))
	for i, row := range rows {
		resp[i] = workflowInstanceToResponse(row)
		if err := h.decorateWorkflowInstanceResponse(r.Context(), r, wsUUID, userID, row, &resp[i]); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to load workflow instance plan")
			return
		}
		count, countErr := h.Queries.CountTasksInWorkflowInstance(r.Context(), db.CountTasksInWorkflowInstanceParams{
			WorkspaceID: wsUUID, WorkflowInstanceID: row.ID,
		})
		if countErr != nil {
			writeError(w, http.StatusInternalServerError, "failed to count workflow tasks")
			return
		}
		resp[i].TaskCount = count
	}
	writeJSON(w, http.StatusOK, map[string]any{"instances": resp, "total": len(resp)})
}

func (h *Handler) CreateWorkflowInstance(w http.ResponseWriter, r *http.Request) {
	workspaceID, userID, wsUUID, ok := h.requireWorkflowMember(w, r)
	if !ok {
		return
	}
	workflowID, ok := parseUUIDOrBadRequest(w, chi.URLParam(r, "id"), "workflow id")
	if !ok {
		return
	}
	var req CreateWorkflowInstanceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	inputs, err := normalizeObject(req.Inputs, `{}`)
	if err != nil {
		writeError(w, http.StatusBadRequest, "inputs must be a JSON object")
		return
	}
	req.Title = strings.TrimSpace(req.Title)
	if req.Title == "" {
		writeError(w, http.StatusBadRequest, "title is required")
		return
	}
	workflow, err := h.Queries.GetWorkflowInWorkspace(r.Context(), db.GetWorkflowInWorkspaceParams{
		ID: workflowID, WorkspaceID: wsUUID,
	})
	if err != nil || workflow.Status == "archived" {
		writeError(w, http.StatusNotFound, "workflow not found")
		return
	}
	projectID := workflow.ProjectID
	if workflow.ProjectID.Valid {
		if req.ProjectID != nil && strings.TrimSpace(*req.ProjectID) != "" {
			requestProjectID, parsed := parseUUIDOrBadRequest(w, *req.ProjectID, "project_id")
			if !parsed {
				return
			}
			if requestProjectID != workflow.ProjectID {
				writeError(w, http.StatusBadRequest, "project_id must match the workflow project")
				return
			}
		}
	} else {
		if req.ProjectID == nil || strings.TrimSpace(*req.ProjectID) == "" {
			writeError(w, http.StatusBadRequest, "project_id is required when starting a Space SOP")
			return
		}
		var parsed bool
		projectID, parsed = parseUUIDOrBadRequest(w, strings.TrimSpace(*req.ProjectID), "project_id")
		if !parsed {
			return
		}
	}
	if !h.requireProjectAccess(w, r, wsUUID, projectID) {
		return
	}
	if _, err := h.Queries.GetProjectInWorkspace(r.Context(), db.GetProjectInWorkspaceParams{
		ID: projectID, WorkspaceID: wsUUID,
	}); err != nil {
		writeError(w, http.StatusBadRequest, "project not found")
		return
	}
	versionID := workflow.CurrentVersionID
	if req.VersionID != nil {
		versionID, ok = parseUUIDOrBadRequest(w, *req.VersionID, "version_id")
		if !ok {
			return
		}
	}
	if !versionID.Valid {
		writeError(w, http.StatusConflict, "workflow has no published version")
		return
	}
	version, err := h.Queries.GetWorkflowVersionInWorkspace(r.Context(), db.GetWorkflowVersionInWorkspaceParams{
		ID: versionID, WorkflowID: workflowID, WorkspaceID: wsUUID,
	})
	if err != nil || version.Status == "draft" {
		writeError(w, http.StatusConflict, "workflow version is not published")
		return
	}
	templateStages, err := h.Queries.ListWorkflowStages(r.Context(), db.ListWorkflowStagesParams{
		WorkflowVersionID: versionID, WorkspaceID: wsUUID,
	})
	if err != nil || len(templateStages) == 0 {
		writeError(w, http.StatusConflict, "workflow version has no stages")
		return
	}
	start := true
	if req.Start != nil {
		start = *req.Start
	}
	status := "draft"
	if start {
		status = "active"
	}
	userUUID, err := util.ParseUUID(userID)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid user id")
		return
	}
	tx, err := h.TxStarter.Begin(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to start workflow instance transaction")
		return
	}
	defer tx.Rollback(r.Context())
	qtx := h.Queries.WithTx(tx)
	if _, err := qtx.LockWorkspaceForWorkflowExecution(r.Context(), wsUUID); err != nil {
		writeError(w, http.StatusConflict, "workspace is unavailable")
		return
	}
	if _, err := qtx.LockProjectForWorkflowExecution(r.Context(), db.LockProjectForWorkflowExecutionParams{ID: projectID, WorkspaceID: wsUUID}); err != nil {
		writeError(w, http.StatusConflict, "project is unavailable")
		return
	}
	lockedWorkflow, err := qtx.LockWorkflowForVersion(r.Context(), db.LockWorkflowForVersionParams{ID: workflowID, WorkspaceID: wsUUID})
	if err != nil || lockedWorkflow.Status == "archived" {
		writeError(w, http.StatusConflict, "workflow is unavailable")
		return
	}
	row, err := qtx.CreateWorkflowInstance(r.Context(), db.CreateWorkflowInstanceParams{
		WorkspaceID: wsUUID, WorkflowID: workflowID, WorkflowVersionID: versionID,
		Title: req.Title, Description: ptrToText(req.Description), Status: status, Inputs: inputs,
		CurrentStageID: pgtype.UUID{}, ProjectID: projectID, CreatedBy: userUUID,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create workflow instance")
		return
	}
	instanceStages := make([]db.WorkflowInstanceStage, 0, len(templateStages))
	for _, templateStage := range templateStages {
		required, missing := h.AgentRunService.ResolveRequiredSkills(r.Context(), wsUUID, templateStage.RequiredSkills)
		if len(missing) > 0 {
			writeError(w, http.StatusConflict, "required SOP skill is unavailable: "+strings.Join(missing, ", "))
			return
		}
		snapshots, marshalErr := json.Marshal(required)
		if marshalErr != nil {
			writeError(w, http.StatusInternalServerError, "failed to snapshot workflow skills")
			return
		}
		instanceStage, createErr := qtx.CreateWorkflowInstanceStage(r.Context(), db.CreateWorkflowInstanceStageParams{
			WorkspaceID: wsUUID, WorkflowInstanceID: row.ID, SourceStageID: templateStage.ID,
			StableKey: templateStage.StableKey, Name: templateStage.Name, Description: templateStage.Description,
			ExecutionMode: pgtype.Text{String: templateStage.ExecutionMode, Valid: true}, ExecutorID: templateStage.ExecutorID,
			Position: templateStage.Position, CompletionRule: templateStage.CompletionRule, ReworkPolicy: templateStage.ReworkPolicy, RetryPolicy: templateStage.RetryPolicy,
			InputSpec: templateStage.InputSpec, InputBindings: templateStage.InputBindings, OutputSpec: templateStage.OutputSpec,
			RequiredSkills: templateStage.RequiredSkills, RequiredSkillSnapshots: snapshots, Gate: templateStage.Gate,
			RollbackStageKey: templateStage.RollbackStageKey, AllowedMcpServerIds: templateStage.AllowedMcpServerIds,
		})
		if createErr != nil {
			writeError(w, http.StatusInternalServerError, "failed to create workflow instance plan")
			return
		}
		instanceStages = append(instanceStages, instanceStage)
	}
	if start {
		executionSvc := &service.WorkflowExecutionService{Queries: h.Queries, TxStarter: h.TxStarter, AgentRunService: h.AgentRunService, Storage: h.Storage}
		row, err = executionSvc.StartWorkflowInstanceInTx(r.Context(), qtx, row, instanceStages[0])
		if err != nil {
			writeError(w, http.StatusConflict, "cannot start stage execution: "+err.Error())
			return
		}
	}
	if err := tx.Commit(r.Context()); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to commit workflow instance")
		return
	}
	resp := workflowInstanceToResponse(row)
	resp.Stages = make([]WorkflowInstanceStageResponse, len(instanceStages))
	for i, stage := range instanceStages {
		resp.Stages[i] = workflowInstanceStageToResponse(stage)
	}
	resp.StageCount = int32(len(instanceStages))
	resp.SourceVersion = version.Version
	resp.CanEdit = h.canManageWorkflowInstance(r, userID, projectID) && !(start && instanceStages[0].ExecutionMode == "agent")
	if start {
		resp.CurrentStageName = &instanceStages[0].Name
		position := instanceStages[0].Position
		resp.CurrentStageIndex = &position
	}
	h.publish(protocol.EventWorkflowInstanceCreated, workspaceID, "member", userID, map[string]any{"instance": resp})
	writeJSON(w, http.StatusCreated, resp)
}

func (h *Handler) GetWorkflowInstance(w http.ResponseWriter, r *http.Request) {
	_, userID, wsUUID, ok := h.requireWorkflowMember(w, r)
	if !ok {
		return
	}
	instanceID, ok := parseUUIDOrBadRequest(w, chi.URLParam(r, "id"), "workflow instance id")
	if !ok {
		return
	}
	row, err := h.Queries.GetWorkflowInstanceInWorkspace(r.Context(), db.GetWorkflowInstanceInWorkspaceParams{
		ID: instanceID, WorkspaceID: wsUUID,
	})
	if err != nil {
		writeError(w, http.StatusNotFound, "workflow instance not found")
		return
	}
	resp := workflowInstanceToResponse(row)
	stages, err := h.Queries.ListWorkflowInstanceStages(r.Context(), db.ListWorkflowInstanceStagesParams{
		WorkflowInstanceID: row.ID, WorkspaceID: wsUUID,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load workflow stages")
		return
	}
	resp.Stages = make([]WorkflowInstanceStageResponse, len(stages))
	for i, stage := range stages {
		resp.Stages[i] = workflowInstanceStageToResponse(stage)
	}
	if err := h.decorateWorkflowInstanceResponse(r.Context(), r, wsUUID, userID, row, &resp); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load workflow instance plan")
		return
	}
	tasks, err := h.Queries.ListTasksForWorkflowInstance(r.Context(), db.ListTasksForWorkflowInstanceParams{
		WorkspaceID: wsUUID, WorkflowInstanceID: instanceID,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load workflow tasks")
		return
	}
	workspace, err := h.Queries.GetWorkspace(r.Context(), wsUUID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load workspace")
		return
	}
	resp.Tasks = make([]TaskResponse, len(tasks))
	resp.TaskCount = int64(len(tasks))
	for i, task := range tasks {
		resp.Tasks[i] = taskToResponse(task, workspace.TaskPrefix)
	}
	// Readiness is diagnostic; the transition transaction still checks locked rows.
	for _, stage := range stages {
		if stage.ID == row.CurrentStageID && stage.ExecutionMode == "manual" {
			statuses := make([]string, 0, len(tasks))
			for _, task := range tasks {
				if task.WorkflowStageID == stage.ID && !task.WorkflowInvalidatedByReworkID.Valid {
					statuses = append(statuses, task.Status)
				}
			}
			evaluated := service.EvaluateManualStage(stage, statuses, time.Now().UTC())
			resp.ManualReadiness = &evaluated
			if evaluated.EvaluationMode == workflowcontract.EvaluationScheduled || evaluated.EvaluationMode == workflowcontract.EvaluationOnTaskChange {
				svc := &service.WorkflowExecutionService{Queries: h.Queries, TxStarter: h.TxStarter}
				if scheduled, err := svc.ReadAutomaticReadiness(r.Context(), row); err == nil {
					resp.AutomaticReadiness = scheduled
				} else {
					h.logWorkflowError(r, "scheduled readiness", err)
				}
			}
			break
		}
	}
	reworks, err := h.Queries.ListWorkflowReworks(r.Context(), db.ListWorkflowReworksParams{WorkspaceID: wsUUID, WorkflowInstanceID: instanceID})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load workflow rework history")
		return
	}
	resp.Reworks = make([]WorkflowReworkResponse, len(reworks))
	for i, record := range reworks {
		resp.Reworks[i] = WorkflowReworkResponse{ID: uuidToString(record.ID), SourceStageID: uuidToString(record.SourceStageID), TargetStageID: uuidToString(record.TargetStageID), SourceExecutionID: uuidToPtr(record.SourceExecutionID), PlanRevision: record.PlanRevision, ActorType: record.ActorType, ActorID: uuidToString(record.ActorID), Reason: record.Reason, CreatedAt: timestampToString(record.CreatedAt)}
	}
	decisions, err := h.Queries.ListWorkflowGateDecisions(r.Context(), db.ListWorkflowGateDecisionsParams{
		WorkflowInstanceID: instanceID, WorkspaceID: wsUUID,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load workflow decisions")
		return
	}
	resp.Decisions = make([]WorkflowGateDecisionResponse, len(decisions))
	for i, decision := range decisions {
		resp.Decisions[i] = workflowDecisionToResponse(decision)
	}
	changes, err := h.Queries.ListWorkflowInstanceChanges(r.Context(), db.ListWorkflowInstanceChangesParams{
		WorkflowInstanceID: instanceID, WorkspaceID: wsUUID,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load workflow changes")
		return
	}
	resp.Changes = make([]WorkflowInstanceChangeResponse, len(changes))
	for i, change := range changes {
		resp.Changes[i] = workflowInstanceChangeToResponse(change)
	}
	writeJSON(w, http.StatusOK, resp)
}

func (h *Handler) TransitionWorkflowInstance(w http.ResponseWriter, r *http.Request) {
	workspaceID, userID, wsUUID, ok := h.requireWorkflowMember(w, r)
	if !ok {
		return
	}
	instanceID, ok := parseUUIDOrBadRequest(w, chi.URLParam(r, "id"), "workflow instance id")
	if !ok {
		return
	}
	var req TransitionWorkflowInstanceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	var target pgtype.UUID
	if req.TargetStageID != nil {
		target, ok = parseUUIDOrBadRequest(w, *req.TargetStageID, "target_stage_id")
		if !ok {
			return
		}
	}
	actorType, actorID := h.resolveActor(r, userID, workspaceID)
	actorUUID, err := util.ParseUUID(actorID)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid actor id")
		return
	}
	svc := &service.WorkflowExecutionService{Queries: h.Queries, TxStarter: h.TxStarter, AgentRunService: h.AgentRunService, Storage: h.Storage}
	result, err := svc.TransitionManual(r.Context(), service.ManualWorkflowTransitionParams{WorkspaceID: wsUUID, InstanceID: instanceID, ActorID: actorUUID, ActorType: actorType, TargetStageID: target, Outcome: req.Outcome, ExpectedStateToken: req.ExpectedStateToken, Note: req.Note})
	if err != nil {
		var domain *service.WorkflowTransitionError
		if errors.As(err, &domain) {
			status := http.StatusConflict
			switch domain.Kind {
			case "invalid":
				status = http.StatusBadRequest
			case "forbidden":
				status = http.StatusForbidden
			case "not_found":
				status = http.StatusNotFound
			}
			writeError(w, status, domain.Message)
		} else if errors.Is(err, service.ErrWorkflowExecutionConflict) {
			writeError(w, http.StatusConflict, err.Error())
		} else {
			h.logWorkflowError(r, "transition", err)
			writeError(w, http.StatusInternalServerError, "failed to transition workflow instance")
		}
		return
	}
	if h.AgentRunService != nil {
		h.AgentRunService.BroadcastCancelledRuns(r.Context(), result.Rework.CancelledRuns)
	}
	resp := workflowInstanceToResponse(result.Instance)
	h.publish(protocol.EventWorkflowInstanceUpdated, workspaceID, actorType, actorID, map[string]any{"instance": resp})
	writeJSON(w, http.StatusOK, resp)
}

func (h *Handler) ArchiveWorkflowInstance(w http.ResponseWriter, r *http.Request) {
	workspaceID, userID, wsUUID, ok := h.requireWorkflowMember(w, r)
	if !ok {
		return
	}
	instanceID, ok := parseUUIDOrBadRequest(w, chi.URLParam(r, "id"), "workflow instance id")
	if !ok {
		return
	}
	instance, err := h.Queries.GetWorkflowInstanceInWorkspace(r.Context(), db.GetWorkflowInstanceInWorkspaceParams{
		ID: instanceID, WorkspaceID: wsUUID,
	})
	if err != nil {
		writeError(w, http.StatusNotFound, "workflow instance not found")
		return
	}
	if instance.ArchivedAt.Valid {
		writeError(w, http.StatusConflict, "workflow instance is already archived")
		return
	}
	taskCount, err := h.Queries.CountTasksInWorkflowInstance(r.Context(), db.CountTasksInWorkflowInstanceParams{
		WorkspaceID: wsUUID, WorkflowInstanceID: instanceID,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to count workflow tasks")
		return
	}
	if taskCount > 0 {
		writeError(w, http.StatusConflict, "workflow instances with tasks cannot be archived")
		return
	}
	archivedBy, err := util.ParseUUID(userID)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid user id")
		return
	}
	archived, err := h.Queries.ArchiveWorkflowInstance(r.Context(), db.ArchiveWorkflowInstanceParams{
		ID: instanceID, WorkspaceID: wsUUID, ArchivedBy: archivedBy,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to archive workflow instance")
		return
	}
	resp := workflowInstanceToResponse(archived)
	resp.TaskCount = taskCount
	h.publish(protocol.EventWorkflowInstanceUpdated, workspaceID, "member", userID, map[string]any{"instance": resp})
	writeJSON(w, http.StatusOK, resp)
}

func (h *Handler) ListWorkflowTasks(w http.ResponseWriter, r *http.Request) {
	_, _, wsUUID, ok := h.requireWorkflowMember(w, r)
	if !ok {
		return
	}
	instanceID, ok := parseUUIDOrBadRequest(w, chi.URLParam(r, "id"), "workflow instance id")
	if !ok {
		return
	}
	if _, err := h.Queries.GetWorkflowInstanceInWorkspace(r.Context(), db.GetWorkflowInstanceInWorkspaceParams{
		ID: instanceID, WorkspaceID: wsUUID,
	}); err != nil {
		writeError(w, http.StatusNotFound, "workflow instance not found")
		return
	}
	rows, err := h.Queries.ListTasksForWorkflowInstance(r.Context(), db.ListTasksForWorkflowInstanceParams{
		WorkspaceID: wsUUID, WorkflowInstanceID: instanceID,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list workflow tasks")
		return
	}
	workspace, err := h.Queries.GetWorkspace(r.Context(), wsUUID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load workspace")
		return
	}
	resp := make([]TaskResponse, len(rows))
	for i, row := range rows {
		resp[i] = taskToResponse(row, workspace.TaskPrefix)
	}
	writeJSON(w, http.StatusOK, map[string]any{"tasks": resp, "total": len(resp)})
}

func (h *Handler) AttachWorkflowTask(w http.ResponseWriter, r *http.Request) {
	workspaceID, userID, wsUUID, ok := h.requireWorkflowMember(w, r)
	if !ok {
		return
	}
	instanceID, ok := parseUUIDOrBadRequest(w, chi.URLParam(r, "id"), "workflow instance id")
	if !ok {
		return
	}
	taskID, ok := parseUUIDOrBadRequest(w, chi.URLParam(r, "taskId"), "task id")
	if !ok {
		return
	}
	var req AttachWorkflowTaskRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	stageID, ok := parseUUIDOrBadRequest(w, req.StageID, "stage_id")
	if !ok {
		return
	}
	instance, err := h.Queries.GetWorkflowInstanceInWorkspace(r.Context(), db.GetWorkflowInstanceInWorkspaceParams{
		ID: instanceID, WorkspaceID: wsUUID,
	})
	if err != nil {
		writeError(w, http.StatusNotFound, "workflow instance not found")
		return
	}
	if instance.Status == "cancelled" {
		writeError(w, http.StatusConflict, "workflow instance is closed")
		return
	}
	if instance.ArchivedAt.Valid {
		writeError(w, http.StatusConflict, "archived workflow instances cannot accept tasks")
		return
	}
	if !instance.ProjectID.Valid {
		writeError(w, http.StatusConflict, "workflow run is not assigned to a project")
		return
	}
	if _, err := h.Queries.GetWorkflowInstanceStage(r.Context(), db.GetWorkflowInstanceStageParams{
		ID: stageID, WorkflowInstanceID: instance.ID, WorkspaceID: wsUUID,
	}); err != nil {
		writeError(w, http.StatusBadRequest, "stage does not belong to this workflow instance")
		return
	}
	task, err := h.Queries.GetTaskInWorkspace(r.Context(), db.GetTaskInWorkspaceParams{
		ID: taskID, WorkspaceID: wsUUID,
	})
	if err != nil {
		writeError(w, http.StatusNotFound, "task not found")
		return
	}
	if task.ProjectID.Valid && task.ProjectID != instance.ProjectID {
		writeError(w, http.StatusConflict, "task belongs to a different project")
		return
	}
	executionSvc := service.WorkflowExecutionService{Queries: h.Queries, TxStarter: h.TxStarter}
	row, err := executionSvc.MoveTaskStage(r.Context(), wsUUID, taskID, instanceID, stageID)
	if err != nil {
		if errors.Is(err, service.ErrWorkflowExecutionConflict) {
			writeError(w, http.StatusConflict, "workflow task membership changed or Run is closed")
		} else if isNotFound(err) {
			writeError(w, http.StatusNotFound, "task not found")
		} else {
			writeError(w, http.StatusInternalServerError, "failed to attach workflow task")
		}
		return
	}
	workspace, _ := h.Queries.GetWorkspace(r.Context(), wsUUID)
	resp := taskToResponse(row, workspace.TaskPrefix)
	actorType, actorID := h.resolveActor(r, userID, workspaceID)
	h.publish(protocol.EventWorkflowTaskChanged, workspaceID, actorType, actorID, map[string]any{
		"instance_id": uuidToString(instanceID), "task": resp,
	})
	writeJSON(w, http.StatusOK, resp)
}

func (h *Handler) DetachWorkflowTask(w http.ResponseWriter, r *http.Request) {
	workspaceID, userID, wsUUID, ok := h.requireWorkflowMember(w, r)
	if !ok {
		return
	}
	instanceID, ok := parseUUIDOrBadRequest(w, chi.URLParam(r, "id"), "workflow instance id")
	if !ok {
		return
	}
	taskID, ok := parseUUIDOrBadRequest(w, chi.URLParam(r, "taskId"), "task id")
	if !ok {
		return
	}
	executionSvc := service.WorkflowExecutionService{Queries: h.Queries, TxStarter: h.TxStarter}
	row, err := executionSvc.MoveTaskStage(r.Context(), wsUUID, taskID, instanceID, pgtype.UUID{})
	if err != nil {
		if errors.Is(err, service.ErrWorkflowExecutionConflict) {
			writeError(w, http.StatusConflict, "workflow task membership changed or Run is closed")
			return
		}
		writeError(w, http.StatusNotFound, "workflow task not found")
		return
	}
	workspace, _ := h.Queries.GetWorkspace(r.Context(), wsUUID)
	resp := taskToResponse(row, workspace.TaskPrefix)
	actorType, actorID := h.resolveActor(r, userID, workspaceID)
	h.publish(protocol.EventWorkflowTaskChanged, workspaceID, actorType, actorID, map[string]any{
		"instance_id": uuidToString(instanceID), "task": resp,
	})
	writeJSON(w, http.StatusOK, resp)
}

func (h *Handler) logWorkflowError(r *http.Request, action string, err error) {
	slog.Error("workflow "+action+" failed", append(logger.RequestAttrs(r), "error", err)...)
}

// workflowExecutorUUID consumes a UUID already validated by normalizeWorkflowStages.
func workflowExecutorUUID(stage WorkflowStageInput) pgtype.UUID {
	if stage.ExecutorID == nil {
		return pgtype.UUID{}
	}
	return parseUUID(*stage.ExecutorID)
}

func validateWorkflowStageExecutors(ctx context.Context, q *db.Queries, workspaceID pgtype.UUID, stages []WorkflowStageInput) error {
	for i, stage := range stages {
		if stage.ExecutionMode != "agent" {
			continue
		}
		agent, err := q.GetAgentInWorkspace(ctx, db.GetAgentInWorkspaceParams{ID: workflowExecutorUUID(stage), WorkspaceID: workspaceID})
		if err != nil || agent.ArchivedAt.Valid {
			return fmt.Errorf("stages[%d].executor_id is unavailable in this workspace", i)
		}
	}
	return nil
}
