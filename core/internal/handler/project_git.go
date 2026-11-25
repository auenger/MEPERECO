package handler

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/silieco-ai/silieco/core/internal/integrations/vcs"
	"github.com/silieco-ai/silieco/core/internal/projectgit"
	db "github.com/silieco-ai/silieco/core/pkg/db/generated"
)

type GitBindingResponse struct {
	Resource       ProjectResourceResponse `json:"resource"`
	ProjectTitle   string                  `json:"project_title"`
	ProjectStatus  string                  `json:"project_status"`
	RepositoryPath *string                 `json:"repository_path"`
	Provider       *string                 `json:"provider"`
	InstanceURL    *string                 `json:"instance_url"`
	SyncState      string                  `json:"sync_state"`
}
type GitTaskReference struct {
	ID            string  `json:"id"`
	ProjectID     *string `json:"project_id"`
	Title         string  `json:"title"`
	Number        int32   `json:"number"`
	ReferenceOnly bool    `json:"reference_only"`
	CloseIntent   bool    `json:"close_intent"`
}
type GitPullRequestItem struct {
	GitHubPullRequestResponse
	InstanceURL    string             `json:"instance_url"`
	ExternalRepoID *string            `json:"external_repo_id"`
	ProjectIDs     []string           `json:"project_ids"`
	Tasks          []GitTaskReference `json:"tasks"`
}

func providerRepositoryID(id int64) pgtype.Text {
	if id <= 0 {
		return pgtype.Text{}
	}
	return pgtype.Text{String: strconv.FormatInt(id, 10), Valid: true}
}
func gitLimit(r *http.Request) int {
	n, e := strconv.Atoi(r.URL.Query().Get("limit"))
	if e != nil || n < 1 {
		return 50
	}
	if n > 100 {
		return 100
	}
	return n
}
func gitUUID(raw string) (pgtype.UUID, error) {
	if raw == "" {
		return pgtype.UUID{}, nil
	}
	return parseUUIDLoose(raw)
}
func (h *Handler) gitScope(w http.ResponseWriter, r *http.Request, projectScoped bool) (pgtype.UUID, pgtype.UUID, bool) {
	if projectScoped {
		p, ok := h.loadProjectForResource(w, r, chi.URLParam(r, "id"))
		return p.WorkspaceID, p.ID, ok
	}
	ws, ok := parseUUIDOrBadRequest(w, chi.URLParam(r, "id"), "workspace id")
	if !ok {
		return ws, pgtype.UUID{}, false
	}
	p, e := gitUUID(r.URL.Query().Get("project_id"))
	if e != nil {
		writeError(w, http.StatusBadRequest, "invalid project_id")
		return ws, p, false
	}
	if p.Valid {
		if _, e = h.Queries.GetProjectInWorkspace(r.Context(), db.GetProjectInWorkspaceParams{ID: p, WorkspaceID: ws}); e != nil {
			writeError(w, http.StatusNotFound, "project not found")
			return ws, p, false
		}
	}
	return ws, p, true
}

const gitBindingsSQL = `
SELECT r.id,r.project_id,r.workspace_id,r.resource_type,r.resource_ref,r.label,r.position,r.created_at,r.created_by,p.title,p.status,b.repository_path,
 CASE WHEN c.id IS NOT NULL THEN c.provider WHEN i.id IS NOT NULL THEN 'github' END,
 CASE WHEN c.id IS NOT NULL THEN c.instance_url WHEN i.id IS NOT NULL THEN 'https://github.com' END,
 CASE WHEN b.resource_id IS NULL THEN 'pending_projection' WHEN b.connection_id IS NULL THEN 'git_only'
 WHEN c.id IS NULL AND i.id IS NULL THEN 'disconnected'
 WHEN c.id IS NOT NULL AND EXISTS(SELECT 1 FROM vcs_pull_request pr WHERE pr.workspace_id=r.workspace_id AND pr.connection_id=c.id AND ((b.external_repo_id IS NOT NULL AND pr.external_repo_id IS NOT NULL AND b.external_repo_id=pr.external_repo_id) OR ((b.external_repo_id IS NULL OR pr.external_repo_id IS NULL) AND pr.repo_owner||'/'||pr.repo_name=b.repository_path))) THEN 'received'
 WHEN i.id IS NOT NULL AND EXISTS(SELECT 1 FROM github_pull_request pr WHERE pr.workspace_id=r.workspace_id AND pr.installation_id=i.installation_id AND ((b.external_repo_id IS NOT NULL AND pr.external_repo_id IS NOT NULL AND b.external_repo_id=pr.external_repo_id) OR ((b.external_repo_id IS NULL OR pr.external_repo_id IS NULL) AND lower(pr.repo_owner||'/'||pr.repo_name)=lower(b.repository_path)))) THEN 'received'
 ELSE 'waiting_events' END
FROM project_resource r JOIN project p ON p.id=r.project_id AND p.workspace_id=r.workspace_id
LEFT JOIN project_repo_binding b ON b.resource_id=r.id AND b.workspace_id=r.workspace_id AND b.project_id=r.project_id
LEFT JOIN vcs_connection c ON b.connection_kind='vcs' AND c.id=b.connection_id AND c.workspace_id=r.workspace_id
LEFT JOIN github_installation i ON b.connection_kind='github_app' AND i.id=b.connection_id AND i.workspace_id=r.workspace_id
WHERE r.workspace_id=$1 AND r.resource_type='github_repo' AND ($2::uuid IS NULL OR r.project_id=$2) AND ($3::uuid IS NULL OR r.id>$3)
ORDER BY r.id LIMIT $4`

func (h *Handler) listGitBindings(ctx context.Context, ws, p, after pgtype.UUID, limit int) ([]GitBindingResponse, error) {
	rows, e := h.DB.Query(ctx, gitBindingsSQL, ws, p, after, limit)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []GitBindingResponse{}
	for rows.Next() {
		var res db.ProjectResource
		var item GitBindingResponse
		var path, provider, instance pgtype.Text
		if e = rows.Scan(&res.ID, &res.ProjectID, &res.WorkspaceID, &res.ResourceType, &res.ResourceRef, &res.Label, &res.Position, &res.CreatedAt, &res.CreatedBy, &item.ProjectTitle, &item.ProjectStatus, &path, &provider, &instance, &item.SyncState); e != nil {
			return nil, e
		}
		item.Resource = projectResourceToResponse(res)
		item.RepositoryPath = textToPtr(path)
		item.Provider = textToPtr(provider)
		item.InstanceURL = textToPtr(instance)
		out = append(out, item)
	}
	return out, rows.Err()
}
func (h *Handler) ListWorkspaceGitBindings(w http.ResponseWriter, r *http.Request) {
	ws, p, ok := h.gitScope(w, r, false)
	if !ok {
		return
	}
	after, e := gitUUID(r.URL.Query().Get("cursor"))
	if e != nil {
		writeError(w, http.StatusBadRequest, "invalid cursor")
		return
	}
	limit := gitLimit(r)
	items, e := h.listGitBindings(r.Context(), ws, p, after, limit+1)
	if e != nil {
		writeError(w, http.StatusInternalServerError, "failed to list Git bindings")
		return
	}
	var next *string
	if len(items) > limit {
		items = items[:limit]
		v := items[len(items)-1].Resource.ID
		next = &v
	}
	writeJSON(w, http.StatusOK, map[string]any{"bindings": items, "next_cursor": next})
}

const gitMatchedSQL = `
WITH matched AS (
 SELECT pr.id,b.project_id,'vcs'::text AS source,pr.provider,c.instance_url,
 COALESCE('id:'||pr.external_repo_id,'path:'||pr.repo_owner||'/'||pr.repo_name) AS repository_key,
 pr.external_repo_id,pr.repo_owner,pr.repo_name,pr.pr_number,pr.title,pr.state,pr.html_url,pr.branch,pr.author_login,pr.author_avatar_url,
 pr.merged_at,pr.closed_at,pr.pr_created_at,pr.pr_updated_at,pr.additions,pr.deletions,pr.changed_files,pr.head_sha,pr.connection_id,
 NULL::text AS checks_rollup_state,NULL::text AS snapshot_head_sha,NULL::timestamptz AS snapshot_fetched_at
 FROM project_repo_binding b JOIN project_resource r ON r.id=b.resource_id AND r.workspace_id=b.workspace_id AND r.project_id=b.project_id AND r.resource_type='github_repo'
 JOIN project p ON p.id=b.project_id AND p.workspace_id=b.workspace_id
 JOIN vcs_connection c ON b.connection_kind='vcs' AND c.id=b.connection_id AND c.workspace_id=b.workspace_id
 JOIN vcs_pull_request pr ON pr.connection_id=c.id AND pr.workspace_id=b.workspace_id
 AND ((b.external_repo_id IS NOT NULL AND pr.external_repo_id IS NOT NULL AND b.external_repo_id=pr.external_repo_id) OR ((b.external_repo_id IS NULL OR pr.external_repo_id IS NULL) AND pr.repo_owner||'/'||pr.repo_name=b.repository_path))
 WHERE b.workspace_id=$1
 UNION ALL
 SELECT pr.id,b.project_id,'github','github','https://github.com',
 COALESCE('id:'||pr.external_repo_id,'path:'||lower(pr.repo_owner||'/'||pr.repo_name)),pr.external_repo_id,pr.repo_owner,pr.repo_name,pr.pr_number,pr.title,pr.state,pr.html_url,pr.branch,pr.author_login,pr.author_avatar_url,
 pr.merged_at,pr.closed_at,pr.pr_created_at,pr.pr_updated_at,pr.additions,pr.deletions,pr.changed_files,pr.head_sha,i.id,pr.checks_rollup_state,pr.snapshot_head_sha,pr.snapshot_fetched_at
 FROM project_repo_binding b JOIN project_resource r ON r.id=b.resource_id AND r.workspace_id=b.workspace_id AND r.project_id=b.project_id AND r.resource_type='github_repo'
 JOIN project p ON p.id=b.project_id AND p.workspace_id=b.workspace_id
 JOIN github_installation i ON b.connection_kind='github_app' AND i.id=b.connection_id AND i.workspace_id=b.workspace_id
 JOIN github_pull_request pr ON pr.installation_id=i.installation_id AND pr.workspace_id=b.workspace_id
 AND ((b.external_repo_id IS NOT NULL AND pr.external_repo_id IS NOT NULL AND b.external_repo_id=pr.external_repo_id) OR ((b.external_repo_id IS NULL OR pr.external_repo_id IS NULL) AND lower(pr.repo_owner||'/'||pr.repo_name)=lower(b.repository_path)))
 WHERE b.workspace_id=$1
), membership AS (
 SELECT provider,instance_url,repository_key,pr_number,array_agg(DISTINCT project_id ORDER BY project_id) AS project_ids, array_agg(DISTINCT id) AS mirror_ids FROM matched GROUP BY provider,instance_url,repository_key,pr_number
), latest AS (
 SELECT DISTINCT ON(provider,instance_url,repository_key,pr_number) * FROM matched ORDER BY provider,instance_url,repository_key,pr_number,pr_updated_at DESC,id DESC
), dedup AS (
 SELECT l.*,m.project_ids,m.mirror_ids FROM latest l JOIN membership m USING(provider,instance_url,repository_key,pr_number) WHERE ($2::uuid IS NULL OR $2=ANY(m.project_ids))
)`

func (h *Handler) ProjectGitOverview(w http.ResponseWriter, r *http.Request) {
	h.gitOverview(w, r, true)
}
func (h *Handler) WorkspaceGitOverview(w http.ResponseWriter, r *http.Request) {
	h.gitOverview(w, r, false)
}
func (h *Handler) gitOverview(w http.ResponseWriter, r *http.Request, projectScoped bool) {
	ws, p, ok := h.gitScope(w, r, projectScoped)
	if !ok {
		return
	}
	var count, projects, connected, prs int64
	e := h.DB.QueryRow(r.Context(), `SELECT count(*),count(DISTINCT r.project_id),count(*) FILTER(WHERE c.id IS NOT NULL OR i.id IS NOT NULL)
 FROM project_resource r JOIN project p ON p.id=r.project_id AND p.workspace_id=r.workspace_id
 LEFT JOIN project_repo_binding b ON b.resource_id=r.id AND b.workspace_id=r.workspace_id AND b.project_id=r.project_id
 LEFT JOIN vcs_connection c ON b.connection_kind='vcs' AND c.id=b.connection_id AND c.workspace_id=r.workspace_id
 LEFT JOIN github_installation i ON b.connection_kind='github_app' AND i.id=b.connection_id AND i.workspace_id=r.workspace_id
 WHERE r.resource_type='github_repo' AND r.workspace_id=$1 AND ($2::uuid IS NULL OR r.project_id=$2)`, ws, p).Scan(&count, &projects, &connected)
	if e != nil {
		writeError(w, http.StatusInternalServerError, "failed to load Git overview")
		return
	}
	if e = h.DB.QueryRow(r.Context(), gitMatchedSQL+` SELECT count(*) FROM dedup`, ws, p).Scan(&prs); e != nil {
		writeError(w, http.StatusInternalServerError, "failed to count Git pull requests")
		return
	}
	resp := map[string]any{"binding_count": count, "project_count": projects, "connected_binding_count": connected, "pull_request_count": prs, "repo_mode": "inherit"}
	if projectScoped {
		project, e := h.Queries.GetProjectInWorkspace(r.Context(), db.GetProjectInWorkspaceParams{ID: p, WorkspaceID: ws})
		if e != nil {
			writeError(w, http.StatusNotFound, "project not found")
			return
		}
		resp["repo_mode"] = project.RepoMode
		bindings, e := h.listGitBindings(r.Context(), ws, p, pgtype.UUID{}, 100)
		if e != nil {
			writeError(w, http.StatusInternalServerError, "failed to load Git repositories")
			return
		}
		resp["bindings"] = bindings
	}
	if workspace, e := h.Queries.GetWorkspace(r.Context(), ws); e == nil {
		resp["default_repos"] = parseWorkspaceRepos(workspace.Repos)
	}
	writeJSON(w, http.StatusOK, resp)
}

const gitPRListSQL = gitMatchedSQL + `,page AS (
 SELECT * FROM dedup d WHERE ($3::text='' OR state=$3) AND ($4::text='' OR strpos(lower(repo_owner||'/'||repo_name),lower($4))>0)
 AND ($5::timestamptz IS NULL OR (pr_updated_at,id)<($5,$6::uuid))
 AND (NOT $7::boolean OR EXISTS(SELECT 1 FROM task_vcs_pull_request l JOIN task t ON t.id=l.task_id AND t.workspace_id=$1 WHERE d.source='vcs' AND l.pull_request_id=ANY(d.mirror_ids) AND ($2::uuid IS NULL OR t.project_id=$2)
 UNION ALL SELECT 1 FROM task_pull_request l JOIN task t ON t.id=l.task_id AND t.workspace_id=$1 WHERE d.source='github' AND l.pull_request_id=ANY(d.mirror_ids) AND ($2::uuid IS NULL OR t.project_id=$2)))
 AND (($9::uuid IS NULL AND $10::text='') OR EXISTS(
   SELECT 1 FROM task_vcs_pull_request l JOIN task t ON t.id=l.task_id AND t.workspace_id=$1 WHERE d.source='vcs' AND l.pull_request_id=ANY(d.mirror_ids) AND ($9::uuid IS NULL OR t.id=$9) AND ($10::text='' OR strpos(lower(t.title),lower($10))>0 OR t.number::text=ltrim($10,'#'))
   UNION ALL SELECT 1 FROM task_pull_request l JOIN task t ON t.id=l.task_id AND t.workspace_id=$1 WHERE d.source='github' AND l.pull_request_id=ANY(d.mirror_ids) AND ($9::uuid IS NULL OR t.id=$9) AND ($10::text='' OR strpos(lower(t.title),lower($10))>0 OR t.number::text=ltrim($10,'#'))))
 ORDER BY pr_updated_at DESC,id DESC LIMIT $8
)
SELECT p.id,p.source,p.provider,p.instance_url,p.external_repo_id,p.repo_owner,p.repo_name,p.pr_number,p.title,p.state,p.html_url,p.branch,p.author_login,p.author_avatar_url,p.merged_at,p.closed_at,p.pr_created_at,p.pr_updated_at,p.additions,p.deletions,p.changed_files,p.project_ids,p.mirror_ids,p.checks_rollup_state,p.snapshot_head_sha,p.snapshot_fetched_at,p.head_sha,
 COALESCE(cs.total,0),COALESCE(cs.passed,0),COALESCE(cs.failed,0),COALESCE(cs.pending,0)
FROM page p LEFT JOIN LATERAL (
 SELECT count(*)::bigint total,count(*) FILTER(WHERE state='passed')::bigint passed,count(*) FILTER(WHERE state='failed')::bigint failed,count(*) FILTER(WHERE state='pending')::bigint pending
 FROM (
   SELECT s.state FROM vcs_repo_commit_status s WHERE p.source='vcs' AND s.connection_id=p.connection_id AND s.repository_path=p.repo_owner||'/'||p.repo_name AND s.sha=p.head_sha AND p.head_sha<>''
   UNION ALL
   SELECT CASE WHEN cr.status='completed' AND cr.conclusion IN ('success','neutral','skipped') THEN 'passed'
               WHEN cr.status='completed' AND cr.conclusion IN ('failure','cancelled','timed_out','action_required','startup_failure','stale','error') THEN 'failed'
               ELSE 'pending' END
   FROM github_pull_request_check_run cr WHERE p.source='github' AND cr.pr_id=p.id AND cr.head_sha=p.head_sha AND p.snapshot_head_sha=p.head_sha AND p.head_sha<>''
 ) states
) cs ON true ORDER BY p.pr_updated_at DESC,p.id DESC`

func parseGitPRCursor(raw string) (pgtype.Timestamptz, pgtype.UUID, error) {
	if raw == "" {
		return pgtype.Timestamptz{}, pgtype.UUID{}, nil
	}
	data, e := base64.RawURLEncoding.DecodeString(raw)
	if e != nil {
		return pgtype.Timestamptz{}, pgtype.UUID{}, e
	}
	a, b, ok := strings.Cut(string(data), "|")
	if !ok {
		return pgtype.Timestamptz{}, pgtype.UUID{}, errors.New("invalid cursor")
	}
	t, e := time.Parse(time.RFC3339Nano, a)
	if e != nil {
		return pgtype.Timestamptz{}, pgtype.UUID{}, e
	}
	id, e := gitUUID(b)
	if e != nil || !id.Valid {
		return pgtype.Timestamptz{}, pgtype.UUID{}, errors.New("invalid cursor")
	}
	return pgtype.Timestamptz{Time: t, Valid: true}, id, nil
}
func (h *Handler) ListProjectGitPullRequests(w http.ResponseWriter, r *http.Request) {
	h.gitPRList(w, r, true)
}
func (h *Handler) ListWorkspaceGitPullRequests(w http.ResponseWriter, r *http.Request) {
	h.gitPRList(w, r, false)
}
func (h *Handler) gitPRList(w http.ResponseWriter, r *http.Request, projectScoped bool) {
	ws, p, ok := h.gitScope(w, r, projectScoped)
	if !ok {
		return
	}
	t, cursor, e := parseGitPRCursor(r.URL.Query().Get("cursor"))
	if e != nil {
		writeError(w, http.StatusBadRequest, "invalid cursor")
		return
	}
	state := r.URL.Query().Get("state")
	switch state {
	case "", "open", "draft", "merged", "closed":
	default:
		writeError(w, http.StatusBadRequest, "invalid PR state")
		return
	}
	taskID, e := gitUUID(r.URL.Query().Get("task_id"))
	if e != nil {
		writeError(w, http.StatusBadRequest, "invalid task_id")
		return
	}
	limit := gitLimit(r)
	rows, e := h.DB.Query(r.Context(), gitPRListSQL, ws, p, state, r.URL.Query().Get("repository"), t, cursor, r.URL.Query().Get("linked_only") == "true", limit+1, taskID, strings.TrimSpace(r.URL.Query().Get("task_query")))
	if e != nil {
		writeError(w, http.StatusInternalServerError, "failed to list Git pull requests")
		return
	}
	type result struct {
		item      GitPullRequestItem
		id        pgtype.UUID
		updated   pgtype.Timestamptz
		source    string
		mirrorIDs []pgtype.UUID
	}
	var results []result
	for rows.Next() {
		var x result
		var ext, branch, author, avatar, rollup, snapshotHead pgtype.Text
		var merged, closed, created, snapshotFetched pgtype.Timestamptz
		var projects []pgtype.UUID
		var head string
		pr := &x.item.GitHubPullRequestResponse
		e = rows.Scan(&x.id, &x.source, &pr.Provider, &x.item.InstanceURL, &ext, &pr.RepoOwner, &pr.RepoName, &pr.Number, &pr.Title, &pr.State, &pr.HtmlURL, &branch, &author, &avatar, &merged, &closed, &created, &x.updated, &pr.Additions, &pr.Deletions, &pr.ChangedFiles, &projects, &x.mirrorIDs, &rollup, &snapshotHead, &snapshotFetched, &head, &pr.ChecksTotal, &pr.ChecksPassed, &pr.ChecksFailed, &pr.ChecksPending)
		if e != nil {
			rows.Close()
			writeError(w, http.StatusInternalServerError, "failed to read Git pull requests")
			return
		}
		pr.ID = uuidToString(x.id)
		pr.WorkspaceID = uuidToString(ws)
		pr.Branch = textToPtr(branch)
		pr.AuthorLogin = textToPtr(author)
		pr.AuthorAvatarURL = textToPtr(avatar)
		pr.MergedAt = timestampToPtr(merged)
		pr.ClosedAt = timestampToPtr(closed)
		pr.PRCreatedAt = timestampToString(created)
		pr.PRUpdatedAt = timestampToString(x.updated)
		x.item.ExternalRepoID = textToPtr(ext)
		x.item.ProjectIDs = []string{}
		for _, id := range projects {
			x.item.ProjectIDs = append(x.item.ProjectIDs, uuidToString(id))
		}
		x.item.Tasks = []GitTaskReference{}
		if x.source == "github" {
			available := head != "" && snapshotHead.Valid && snapshotHead.String == head && snapshotFetched.Valid
			pr.SnapshotAvailable = &available
			pr.ChecksRunning = pr.ChecksPending
			if available {
				pr.ChecksRollup = textToPtr(rollup)
				switch strings.ToLower(rollup.String) {
				case "success":
					pr.ChecksConclusion = strPtrOrNil("passed")
				case "failure", "error":
					pr.ChecksConclusion = strPtrOrNil("failed")
				case "pending", "expected":
					pr.ChecksConclusion = strPtrOrNil("pending")
				}
			}
		} else {
			pr.ChecksConclusion = aggregateChecksConclusion(pr.ChecksFailed, pr.ChecksPassed, pr.ChecksPending, pr.ChecksTotal)
			pr.ChecksRunning = pr.ChecksPending
		}
		results = append(results, x)
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		writeError(w, http.StatusInternalServerError, "failed to list Git pull requests")
		return
	}
	var next *string
	if len(results) > limit {
		results = results[:limit]
		last := results[len(results)-1]
		v := base64.RawURLEncoding.EncodeToString([]byte(last.updated.Time.Format(time.RFC3339Nano) + "|" + last.item.ID))
		next = &v
	}
	idsVCS, idsGH := []pgtype.UUID{}, []pgtype.UUID{}
	for _, x := range results {
		if x.source == "github" {
			idsGH = append(idsGH, x.mirrorIDs...)
		} else {
			idsVCS = append(idsVCS, x.mirrorIDs...)
		}
	}
	links, e := h.gitTaskReferences(r.Context(), ws, idsVCS, idsGH)
	if e != nil {
		writeError(w, http.StatusInternalServerError, "failed to load associated tasks")
		return
	}
	items := []GitPullRequestItem{}
	for _, x := range results {
		seen := map[string]int{}
		for _, mirrorID := range x.mirrorIDs {
			for _, task := range links[x.source+":"+uuidToString(mirrorID)] {
				if index, ok := seen[task.ID]; ok {
					old := &x.item.Tasks[index]
					old.ReferenceOnly = old.ReferenceOnly && task.ReferenceOnly
					old.CloseIntent = old.CloseIntent || task.CloseIntent
				} else {
					seen[task.ID] = len(x.item.Tasks)
					x.item.Tasks = append(x.item.Tasks, task)
				}
			}
		}
		items = append(items, x.item)
	}
	writeJSON(w, http.StatusOK, map[string]any{"pull_requests": items, "next_cursor": next})
}
func (h *Handler) gitTaskReferences(ctx context.Context, ws pgtype.UUID, vcsIDs, ghIDs []pgtype.UUID) (map[string][]GitTaskReference, error) {
	out := map[string][]GitTaskReference{}
	if len(vcsIDs)+len(ghIDs) == 0 {
		return out, nil
	}
	rows, e := h.DB.Query(ctx, `SELECT 'vcs',l.pull_request_id,t.id,t.project_id,t.title,t.number,l.reference_only,l.close_intent FROM task_vcs_pull_request l JOIN task t ON t.id=l.task_id AND t.workspace_id=$1 WHERE l.pull_request_id=ANY($2::uuid[])
 UNION ALL SELECT 'github',l.pull_request_id,t.id,t.project_id,t.title,t.number,l.reference_only,l.close_intent FROM task_pull_request l JOIN task t ON t.id=l.task_id AND t.workspace_id=$1 WHERE l.pull_request_id=ANY($3::uuid[])`, ws, vcsIDs, ghIDs)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	for rows.Next() {
		var kind string
		var pr, id, project pgtype.UUID
		var x GitTaskReference
		if e = rows.Scan(&kind, &pr, &id, &project, &x.Title, &x.Number, &x.ReferenceOnly, &x.CloseIntent); e != nil {
			return nil, e
		}
		x.ID = uuidToString(id)
		x.ProjectID = uuidToPtr(project)
		key := kind + ":" + uuidToString(pr)
		out[key] = append(out[key], x)
	}
	return out, rows.Err()
}
func (h *Handler) taskGitRepositoryEligible(ctx context.Context, task db.Task, kind string, connection pgtype.UUID, instance, path, externalID string) bool {
	if !task.ProjectID.Valid {
		return true
	}
	project, err := h.Queries.GetProjectInWorkspace(ctx, db.GetProjectInWorkspaceParams{ID: task.ProjectID, WorkspaceID: task.WorkspaceID})
	if err != nil || project.RepoMode == "none" {
		return false
	}
	resources, e := h.Queries.ListProjectResources(ctx, task.ProjectID)
	if e != nil {
		return false
	}
	hasRepo := false
	for _, r := range resources {
		if r.WorkspaceID != task.WorkspaceID || r.ResourceType != "github_repo" {
			continue
		}
		hasRepo = true
		var ref projectgit.Ref
		if json.Unmarshal(r.ResourceRef, &ref) != nil {
			continue
		}
		if in := ref.Integration; in != nil {
			if in.Kind != kind || in.ConnectionID != uuidToString(connection) {
				continue
			}
			if in.ExternalRepoID != "" && externalID != "" {
				if in.ExternalRepoID == externalID {
					return true
				}
				continue
			}
			if in.RepositoryPath == path || (kind == "github_app" && strings.EqualFold(in.RepositoryPath, path)) {
				return true
			}
		} else if loc, err := vcs.ParseCloneURL(ref.URL); err == nil {
			if kind == "github_app" && loc.Host == "github.com" && strings.EqualFold(loc.Path, path) {
				return true
			}
			if rp, ok := vcs.MatchInstance(loc, instance); ok && rp == path {
				return true
			}
		}
	}
	return !hasRepo && project.RepoMode == "inherit"
}

// A failed projection lookup deliberately returns nil, so realtime clients
// fall back to workspace-scoped invalidation rather than missing an event.
func (h *Handler) gitProjectsForRepository(ctx context.Context, ws pgtype.UUID, kind string, connection pgtype.UUID, path, id string) []string {
	rows, e := h.DB.Query(ctx, `SELECT DISTINCT b.project_id FROM project_repo_binding b JOIN project_resource r ON r.id=b.resource_id AND r.workspace_id=b.workspace_id AND r.project_id=b.project_id
 WHERE b.workspace_id=$1 AND b.connection_kind=$2 AND b.connection_id=$3 AND
 ((b.external_repo_id IS NOT NULL AND $5<>'' AND b.external_repo_id=$5) OR ((b.external_repo_id IS NULL OR $5='') AND (b.repository_path=$4 OR ($2='github_app' AND lower(b.repository_path)=lower($4)))))`, ws, kind, connection, path, id)
	if e != nil {
		return nil
	}
	defer rows.Close()
	out := []string{}
	for rows.Next() {
		var p pgtype.UUID
		if rows.Scan(&p) != nil {
			return nil
		}
		out = append(out, uuidToString(p))
	}
	if rows.Err() != nil {
		return nil
	}
	return out
}

func (h *Handler) SetProjectGitRepoMode(w http.ResponseWriter, r *http.Request) {
	project, ok := h.loadProjectForResource(w, r, chi.URLParam(r, "id"))
	if !ok {
		return
	}
	if !h.requireProjectResourceManager(w, r, project) {
		return
	}
	var body struct {
		Mode string `json:"repo_mode"`
	}
	if json.NewDecoder(r.Body).Decode(&body) != nil {
		writeError(w, http.StatusBadRequest, "invalid request")
		return
	}
	switch body.Mode {
	case "inherit", "project_only", "none":
	default:
		writeError(w, http.StatusBadRequest, "invalid repository mode")
		return
	}
	tx, e := h.TxStarter.Begin(r.Context())
	if e != nil {
		writeError(w, http.StatusInternalServerError, "failed to save repository mode")
		return
	}
	defer tx.Rollback(r.Context())
	if e = projectgit.LockProject(r.Context(), tx, project.WorkspaceID, project.ID); e != nil {
		writeError(w, http.StatusConflict, "project changed")
		return
	}
	if _, e = tx.Exec(r.Context(), `UPDATE project SET repo_mode=$3,updated_at=now() WHERE id=$1 AND workspace_id=$2`, project.ID, project.WorkspaceID, body.Mode); e != nil {
		writeError(w, http.StatusInternalServerError, "failed to save repository mode")
		return
	}
	if e = tx.Commit(r.Context()); e != nil {
		writeError(w, http.StatusInternalServerError, "failed to save repository mode")
		return
	}
	h.publish("project_resource:updated", uuidToString(project.WorkspaceID), "system", "", map[string]any{"project_id": uuidToString(project.ID), "repo_mode": body.Mode})
	writeJSON(w, http.StatusOK, map[string]string{"repo_mode": body.Mode})
}
