package main

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/auenger/MEPERECO/core/internal/events"
	"github.com/auenger/MEPERECO/core/internal/util"
	db "github.com/auenger/MEPERECO/core/pkg/db/generated"
	"github.com/jackc/pgx/v5/pgtype"
)

type projectEventScope struct {
	project  pgtype.UUID
	chat     *db.ChatSession
	onlyUser pgtype.UUID
	resolved bool
}

func projectSensitiveEvent(eventType string) bool {
	prefix, _, _ := strings.Cut(eventType, ":")
	switch prefix {
	case "project", "project_resource", "task", "task_metadata", "task_labels", "task_properties", "task_reaction", "comment", "reaction", "activity", "subscriber", "run", "chat", "workflow", "workflow_instance", "workflow_run", "workflow_task", "autopilot", "pull_request", "pin":
		return true
	}
	return false
}

func eventString(obj map[string]any, key string) string {
	val, _ := obj[key].(string)
	return val
}
func eventUUID(obj map[string]any, key string) pgtype.UUID {
	id, _ := util.ParseUUID(eventString(obj, key))
	return id
}

// Resolve authoritative task/chat/project lineage, not client-supplied metadata.
// If a deleted resource or legacy event has no resolvable scope, never broadcast
// its content; only issue a content-free cache invalidation.
func resolveProjectEventScope(ctx context.Context, q *db.Queries, e events.Event, ws pgtype.UUID) projectEventScope {
	data, err := json.Marshal(e.Payload)
	if err != nil {
		return projectEventScope{}
	}
	var payload map[string]any
	if json.Unmarshal(data, &payload) != nil {
		return projectEventScope{}
	}
	nested := func(key string) map[string]any { val, _ := payload[key].(map[string]any); return val }
	lookupChat := func(id pgtype.UUID) projectEventScope {
		cs, err := q.GetChatSessionInWorkspace(ctx, db.GetChatSessionInWorkspaceParams{ID: id, WorkspaceID: ws})
		if err != nil {
			return projectEventScope{}
		}
		return projectEventScope{project: cs.ProjectID, chat: &cs, resolved: true}
	}
	lookupTask := func(id pgtype.UUID) projectEventScope {
		t, err := q.GetTaskInWorkspace(ctx, db.GetTaskInWorkspaceParams{ID: id, WorkspaceID: ws})
		if err != nil {
			return projectEventScope{}
		}
		return projectEventScope{project: t.ProjectID, resolved: true}
	}
	chatID, _ := util.ParseUUID(e.ChatSessionID)
	if !chatID.Valid {
		chatID = eventUUID(payload, "chat_session_id")
	}
	if chatID.Valid {
		return lookupChat(chatID)
	}
	taskID := eventUUID(payload, "task_id")
	if !taskID.Valid {
		taskID = eventUUID(nested("task"), "id")
	}
	if !taskID.Valid {
		for _, key := range []string{"comment", "reaction", "activity", "item", "run"} {
			if id := eventUUID(nested(key), "task_id"); id.Valid {
				taskID = id
				break
			}
		}
	}
	if taskID.Valid {
		return lookupTask(taskID)
	}
	runID, _ := util.ParseUUID(e.RunID)
	if !runID.Valid {
		runID = eventUUID(payload, "run_id")
	}
	if !runID.Valid {
		runID = eventUUID(nested("run"), "id")
	}
	if runID.Valid {
		run, err := q.GetAgentRun(ctx, runID)
		if err != nil {
			return projectEventScope{}
		}
		if run.ChatSessionID.Valid {
			return lookupChat(run.ChatSessionID)
		}
		if run.TaskID.Valid {
			return lookupTask(run.TaskID)
		}
		if run.AutopilotRunID.Valid {
			ar, err := q.GetAutopilotRun(ctx, run.AutopilotRunID)
			if err != nil {
				return projectEventScope{}
			}
			ap, err := q.GetAutopilotInWorkspace(ctx, db.GetAutopilotInWorkspaceParams{ID: ar.AutopilotID, WorkspaceID: ws})
			if err != nil {
				return projectEventScope{}
			}
			return projectEventScope{project: ap.ProjectID, resolved: true}
		}
		var qc struct {
			Type      string `json:"type"`
			ProjectID string `json:"project_id"`
		}
		if json.Unmarshal(run.Context, &qc) == nil && qc.Type == "quick_create" && run.InitiatorUserID.Valid {
			pid, _ := util.ParseUUID(qc.ProjectID)
			return projectEventScope{project: pid, onlyUser: run.InitiatorUserID, resolved: true}
		}
		return projectEventScope{}
	}
	projectID := eventUUID(payload, "project_id")
	if !projectID.Valid {
		projectID = eventUUID(nested("project"), "id")
	}
	if !projectID.Valid {
		projectID = eventUUID(nested("resource"), "project_id")
	}
	if !projectID.Valid {
		for _, key := range []string{"workflow", "instance", "autopilot"} {
			if id := eventUUID(nested(key), "project_id"); id.Valid {
				projectID = id
				break
			}
		}
	}
	if projectID.Valid {
		return projectEventScope{project: projectID, resolved: true}
	}
	return projectEventScope{}
}

func projectEventAllowed(ctx context.Context, q *db.Queries, scope projectEventScope, ws, user pgtype.UUID) bool {
	if !scope.resolved {
		return false
	}
	if scope.onlyUser.Valid && scope.onlyUser != user {
		return false
	}
	allowed, err := q.CanAccessProject(ctx, ws, user, scope.project)
	if err != nil || !allowed {
		return false
	}
	if scope.chat == nil {
		return true
	}
	if scope.chat.Kind != "group" {
		return scope.chat.CreatorID == user
	}
	allowed, err = q.IsChatSessionMemberParticipant(ctx, db.IsChatSessionMemberParticipantParams{ChatSessionID: scope.chat.ID, ParticipantID: user})
	return err == nil && allowed
}

// Resolve current audience once per event, not once per workspace member.
func projectEventAudience(ctx context.Context, q *db.Queries, scope projectEventScope, ws pgtype.UUID) ([]pgtype.UUID, error) {
	users, err := q.ListProjectAudience(ctx, ws, scope.project)
	if err != nil {
		return nil, err
	}
	if scope.chat == nil && !scope.onlyUser.Valid {
		return users, nil
	}
	allowed := map[pgtype.UUID]bool{}
	if scope.onlyUser.Valid {
		allowed[scope.onlyUser] = true
	}
	if scope.chat != nil {
		if scope.chat.Kind != "group" {
			allowed[scope.chat.CreatorID] = true
		} else {
			rows, err := q.ListChatSessionParticipants(ctx, scope.chat.ID)
			if err != nil {
				return nil, err
			}
			for _, row := range rows {
				if row.ParticipantType == "member" {
					allowed[row.ParticipantID] = true
				}
			}
		}
	}
	out := users[:0]
	for _, user := range users {
		if allowed[user] {
			out = append(out, user)
		}
	}
	return out, nil
}
