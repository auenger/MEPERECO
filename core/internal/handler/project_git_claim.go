package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/auenger/MEPERECO/core/internal/projectgit"
	db "github.com/auenger/MEPERECO/core/pkg/db/generated"
	"github.com/auenger/MEPERECO/core/pkg/runfailure"
)

var errProjectRepoDaemonUpgrade = errors.New("project repository mode requires an upgraded daemon")

// applyClaimProjectRepos resolves all task, chat, quick-create and run-only
// autopilot repositories through the same workspace-scoped policy. A DB read
// failure is not an empty project and must not silently grant default repos.
func (h *Handler) applyClaimProjectRepos(ctx context.Context, resp *AgentRunResponse, workspaceID, projectID pgtype.UUID, scopedDaemon bool) error {
	resp.Repos = nil
	resp.ProjectResources = nil
	resp.ProjectID, resp.ProjectTitle, resp.ProjectDescription = "", "", ""
	resp.RepoMode = "inherit"
	resp.RepoScopeVersion = 0
	if scopedDaemon {
		resp.RepoScopeVersion = 1
	}
	if projectID.Valid {
		project, err := h.Queries.GetProjectInWorkspace(ctx, db.GetProjectInWorkspaceParams{ID: projectID, WorkspaceID: workspaceID})
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("load project repository policy: %w", err)
		}
		// Existing soft project references can disappear after a delete.
		if err == nil {
			resp.ProjectID = uuidToString(project.ID)
			resp.ProjectTitle = project.Title
			resp.ProjectDescription = project.Description.String
			if project.RepoMode != "" {
				resp.RepoMode = project.RepoMode
			}
			if resp.RepoMode != "inherit" && !scopedDaemon {
				return errProjectRepoDaemonUpgrade
			}
			resources, err := h.Queries.ListProjectResources(ctx, project.ID)
			if err != nil {
				return fmt.Errorf("load project repositories: %w", err)
			}
			for _, row := range resources {
				if row.WorkspaceID != workspaceID {
					continue
				}
				ref := json.RawMessage(row.ResourceRef)
				if len(ref) == 0 {
					ref = json.RawMessage("{}")
				}
				resp.ProjectResources = append(resp.ProjectResources, ProjectResourceData{ID: uuidToString(row.ID), ResourceType: row.ResourceType, ResourceRef: ref, Label: row.Label.String})
				if row.ResourceType != "github_repo" || resp.RepoMode == "none" {
					continue
				}
				var repo projectgit.Ref
				if err := json.Unmarshal(ref, &repo); err != nil || strings.TrimSpace(repo.URL) == "" {
					return errors.New("invalid project repository configuration")
				}
				resp.Repos = append(resp.Repos, RepoData{URL: repo.URL, Ref: strings.TrimSpace(repo.Ref)})
			}
			if resp.RepoMode != "inherit" || len(resp.Repos) > 0 {
				return nil
			}
		}
	}
	ws, err := h.Queries.GetWorkspace(ctx, workspaceID)
	if err != nil {
		return fmt.Errorf("load default repositories: %w", err)
	}
	if len(ws.Repos) > 0 {
		if err = json.Unmarshal(ws.Repos, &resp.Repos); err != nil {
			return fmt.Errorf("invalid default repository configuration: %w", err)
		}
	}
	return nil
}
func (h *Handler) claimProjectRepoFailure(r *http.Request, agentRun *db.AgentRun, err error) *claimBuildFailure {
	if errors.Is(err, errProjectRepoDaemonUpgrade) {
		return h.failClaimedRunBeforeLaunch(r.Context(), agentRun,
			"Project repository policy requires an updated daemon. Upgrade the local runtime or choose compatibility mode.",
			runfailure.ReasonInvalidRunIdentity, "error_repo_mode_upgrade", http.StatusConflict, err.Error())
	}
	slog.Warn("claim: repository configuration unavailable", "run_id", uuidToString(agentRun.ID), "err", err)
	if _, requeueErr := h.AgentRunService.RequeueRunAfterClaimFailure(r.Context(), *agentRun); requeueErr != nil {
		slog.Error("claim: failed to requeue after repository configuration error", "run_id", uuidToString(agentRun.ID), "err", requeueErr)
	}
	return &claimBuildFailure{outcome: "error_repo_config", status: http.StatusServiceUnavailable, message: "failed to resolve project repository configuration; retry"}
}
