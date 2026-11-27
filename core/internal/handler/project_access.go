package handler

import (
	"encoding/json"
	db "github.com/auenger/MEPERECO/core/pkg/db/generated"
	"github.com/auenger/MEPERECO/core/pkg/protocol"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"net/http"
)

// Agent run credentials represent the runtime owner, but project authorization
// follows the human who initiated the delegated run whenever known.
func (h *Handler) projectViewer(r *http.Request) pgtype.UUID {
	if r.Header.Get("X-Actor-Source") == "agent_run_token" {
		if run, ok := h.runFromRequestHeader(r); ok {
			if run.OriginatorUserID.Valid {
				return run.OriginatorUserID
			}
			if run.InitiatorUserID.Valid {
				return run.InitiatorUserID
			}
		}
	}
	return parseUUID(requestUserID(r))
}

func (h *Handler) projectAccessible(r *http.Request, workspaceID, projectID pgtype.UUID) bool {
	if !projectID.Valid {
		return true
	}
	allowed, err := h.Queries.CanAccessProject(r.Context(), workspaceID, h.projectViewer(r), projectID)
	return err == nil && allowed
}

func (h *Handler) requireProjectAccess(w http.ResponseWriter, r *http.Request, workspaceID, projectID pgtype.UUID) bool {
	if !h.projectAccessible(r, workspaceID, projectID) {
		writeError(w, http.StatusForbidden, "project access required")
		return false
	}
	return true
}

func (h *Handler) accessibleProjectIDs(w http.ResponseWriter, r *http.Request, workspaceID pgtype.UUID) ([]pgtype.UUID, bool) {
	ids, err := h.Queries.ListAccessibleProjectIDs(r.Context(), workspaceID, h.projectViewer(r))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to resolve project access")
		return nil, false
	}
	return ids, true
}

func projectIDSet(ids []pgtype.UUID) map[pgtype.UUID]bool {
	out := make(map[pgtype.UUID]bool, len(ids))
	for _, id := range ids {
		out[id] = true
	}
	return out
}

// RequireProjectAccess is applied to the project's entire resource subtree.
func (h *Handler) RequireProjectAccess(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id, ok := parseUUIDOrBadRequest(w, chi.URLParam(r, "id"), "project id")
		if !ok || !h.requireProjectAccess(w, r, parseUUID(h.resolveWorkspaceID(r)), id) {
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (h *Handler) ListProjectMembers(w http.ResponseWriter, r *http.Request) {
	workspaceID := h.resolveWorkspaceID(r)
	if _, ok := h.requireWorkspaceRole(w, r, workspaceID, "workspace not found", "owner", "admin"); !ok {
		return
	}
	id, ok := parseUUIDOrBadRequest(w, chi.URLParam(r, "id"), "project id")
	if !ok {
		return
	}
	ids, err := h.Queries.ListProjectMemberUserIDs(r.Context(), parseUUID(workspaceID), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list project members")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"user_ids": uuidStringsOrEmpty(ids)})
}

// ListMemberProjectAccess is the employee-oriented view of the same grants.
func (h *Handler) ListMemberProjectAccess(w http.ResponseWriter, r *http.Request) {
	workspaceID := h.resolveWorkspaceID(r)
	if _, ok := h.requireWorkspaceRole(w, r, workspaceID, "workspace not found", "owner", "admin"); !ok {
		return
	}
	uid, ok := parseUUIDOrBadRequest(w, chi.URLParam(r, "userId"), "user id")
	if !ok {
		return
	}
	ws := parseUUID(workspaceID)
	if _, err := h.Queries.GetMemberByUserAndWorkspace(r.Context(), db.GetMemberByUserAndWorkspaceParams{WorkspaceID: ws, UserID: uid}); err != nil {
		writeError(w, http.StatusNotFound, "workspace member not found")
		return
	}
	ids, err := h.Queries.ListAccessibleProjectIDs(r.Context(), ws, uid)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list member projects")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"project_ids": uuidStringsOrEmpty(ids)})
}

// Grant/revoke one employee at a time, preserving concurrent edits by admins.
func (h *Handler) SetProjectMember(w http.ResponseWriter, r *http.Request) {
	workspaceID := h.resolveWorkspaceID(r)
	if _, ok := h.requireWorkspaceRole(w, r, workspaceID, "workspace not found", "owner", "admin"); !ok {
		return
	}
	projectID, ok := parseUUIDOrBadRequest(w, chi.URLParam(r, "id"), "project id")
	if !ok {
		return
	}
	userID, ok := parseUUIDOrBadRequest(w, chi.URLParam(r, "userId"), "user id")
	if !ok {
		return
	}
	workspaceUUID := parseUUID(workspaceID)
	member, err := h.Queries.GetMemberByUserAndWorkspace(r.Context(), db.GetMemberByUserAndWorkspaceParams{WorkspaceID: workspaceUUID, UserID: userID})
	if err != nil {
		writeError(w, http.StatusNotFound, "workspace member not found")
		return
	}
	if roleAllowed(member.Role, "owner", "admin") {
		writeError(w, http.StatusBadRequest, "workspace administrators already have full access")
		return
	}
	var req struct {
		Granted *bool `json:"granted"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Granted == nil {
		writeError(w, http.StatusBadRequest, "granted is required")
		return
	}
	if *req.Granted {
		err = h.Queries.GrantProjectMember(r.Context(), workspaceUUID, projectID, userID)
	} else {
		err = h.Queries.RevokeProjectMember(r.Context(), workspaceUUID, projectID, userID)
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update project membership")
		return
	}
	h.publish(protocol.EventProjectAccessChanged, workspaceID, "member", requestUserID(r), map[string]any{"recipient_id": uuidToString(userID), "workspace_id": workspaceID, "project_id": uuidToString(projectID), "granted": *req.Granted})
	writeJSON(w, http.StatusOK, map[string]bool{"granted": *req.Granted})
}

// requireTaskProjectAccess is used by secondary resource endpoints that receive
// a task UUID rather than the normal /tasks/{id} route.
func (h *Handler) taskProjectAccessible(r *http.Request, workspaceID, taskID pgtype.UUID) bool {
	if !taskID.Valid {
		return true
	}
	task, err := h.Queries.GetTaskInWorkspace(r.Context(), db.GetTaskInWorkspaceParams{ID: taskID, WorkspaceID: workspaceID})
	return err == nil && h.projectAccessible(r, workspaceID, task.ProjectID)
}

func (h *Handler) RequireCommentProjectAccess(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id, ok := parseUUIDOrBadRequest(w, chi.URLParam(r, "commentId"), "comment id")
		if !ok {
			return
		}
		ws := parseUUID(h.resolveWorkspaceID(r))
		comment, err := h.Queries.GetCommentInWorkspace(r.Context(), db.GetCommentInWorkspaceParams{ID: id, WorkspaceID: ws})
		if err != nil || !h.taskProjectAccessible(r, ws, comment.TaskID) {
			writeError(w, http.StatusNotFound, "comment not found")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (h *Handler) RequireWorkflowProjectAccess(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id, ok := parseUUIDOrBadRequest(w, chi.URLParam(r, "id"), "workflow id")
		if !ok {
			return
		}
		ws := parseUUID(h.resolveWorkspaceID(r))
		row, err := h.Queries.GetWorkflowInWorkspace(r.Context(), db.GetWorkflowInWorkspaceParams{ID: id, WorkspaceID: ws})
		if err != nil {
			writeError(w, http.StatusNotFound, "workflow not found")
			return
		}
		if !h.requireProjectAccess(w, r, ws, row.ProjectID) {
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (h *Handler) RequireWorkflowInstanceProjectAccess(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id, ok := parseUUIDOrBadRequest(w, chi.URLParam(r, "id"), "workflow instance id")
		if !ok {
			return
		}
		ws := parseUUID(h.resolveWorkspaceID(r))
		row, err := h.Queries.GetWorkflowInstanceInWorkspace(r.Context(), db.GetWorkflowInstanceInWorkspaceParams{ID: id, WorkspaceID: ws})
		if err != nil {
			writeError(w, http.StatusNotFound, "workflow instance not found")
			return
		}
		if !h.requireProjectAccess(w, r, ws, row.ProjectID) {
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (h *Handler) RequireAutopilotProjectAccess(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id, ok := parseUUIDOrBadRequest(w, chi.URLParam(r, "id"), "autopilot id")
		if !ok {
			return
		}
		ws := parseUUID(h.resolveWorkspaceID(r))
		row, err := h.Queries.GetAutopilotInWorkspace(r.Context(), db.GetAutopilotInWorkspaceParams{ID: id, WorkspaceID: ws})
		if err != nil {
			writeError(w, http.StatusNotFound, "autopilot not found")
			return
		}
		if !h.requireProjectAccess(w, r, ws, row.ProjectID) {
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (h *Handler) runProjectAccessible(r *http.Request, workspaceID pgtype.UUID, run db.AgentRun) bool {
	if run.TaskID.Valid {
		return h.taskProjectAccessible(r, workspaceID, run.TaskID)
	}
	if run.ChatSessionID.Valid {
		session, err := h.Queries.GetChatSessionInWorkspace(r.Context(), db.GetChatSessionInWorkspaceParams{ID: run.ChatSessionID, WorkspaceID: workspaceID})
		if err != nil || !h.projectAccessible(r, workspaceID, session.ProjectID) {
			return false
		}
		if session.Kind == "group" {
			allowed, err := h.Queries.IsChatSessionMemberParticipant(r.Context(), db.IsChatSessionMemberParticipantParams{ChatSessionID: session.ID, ParticipantID: parseUUID(requestUserID(r))})
			return err == nil && allowed
		}
		return session.CreatorID == parseUUID(requestUserID(r))
	}
	if run.AutopilotRunID.Valid {
		ar, err := h.Queries.GetAutopilotRun(r.Context(), run.AutopilotRunID)
		if err != nil {
			return false
		}
		ap, err := h.Queries.GetAutopilotInWorkspace(r.Context(), db.GetAutopilotInWorkspaceParams{ID: ar.AutopilotID, WorkspaceID: workspaceID})
		return err == nil && h.projectAccessible(r, workspaceID, ap.ProjectID)
	}
	var qc struct {
		Type      string `json:"type"`
		ProjectID string `json:"project_id"`
	}
	if json.Unmarshal(run.Context, &qc) == nil && qc.Type == "quick_create" && qc.ProjectID != "" {
		return h.projectAccessible(r, workspaceID, parseUUID(qc.ProjectID))
	}
	return true
}

func (h *Handler) attachmentProjectAccessible(r *http.Request, workspaceID, attachmentID pgtype.UUID) bool {
	allowed, err := h.Queries.CanAccessAttachment(r.Context(), workspaceID, h.projectViewer(r), attachmentID)
	return err == nil && allowed
}
