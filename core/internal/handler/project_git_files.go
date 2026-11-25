package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

const gitFilePageSize = 30
const gitFileResponseLimit = 8 << 20
const gitFilePatchLimit = 128 << 10

type gitChangedFile struct {
	Path           string `json:"path"`
	PreviousPath   string `json:"previous_path,omitempty"`
	Status         string `json:"status"`
	Additions      *int   `json:"additions,omitempty"`
	Deletions      *int   `json:"deletions,omitempty"`
	Patch          string `json:"patch,omitempty"`
	PatchTruncated bool   `json:"patch_truncated,omitempty"`
}

type gitFilesPage struct {
	Files    []gitChangedFile `json:"files"`
	NextPage *int             `json:"next_page"`
}

func (h *Handler) WorkspaceGitPullRequestFiles(w http.ResponseWriter, r *http.Request) {
	h.gitPullRequestFiles(w, r, false)
}
func (h *Handler) ProjectGitPullRequestFiles(w http.ResponseWriter, r *http.Request) {
	h.gitPullRequestFiles(w, r, true)
}

func (h *Handler) gitPullRequestFiles(w http.ResponseWriter, r *http.Request, projectScoped bool) {
	w.Header().Set("Cache-Control", "private, no-store")
	if r.Header.Get("X-Actor-Source") == "agent_run_token" {
		writeError(w, http.StatusForbidden, "task tokens cannot read platform file contents")
		return
	}
	ws, p, ok := h.gitScope(w, r, projectScoped)
	if !ok {
		return
	}
	id, ok := parseUUIDOrBadRequest(w, chi.URLParam(r, "prId"), "pull request id")
	if !ok {
		return
	}
	page, ok := parseGitHubPageParam(w, r, "page", 1, 1, 100)
	if !ok {
		return
	}
	// Only fetch from an active connection and a repository in the same scoped
	// binding projection as the PR list. Never accept a remote URL from callers.
	var source, provider, instance, owner, repo string
	var external pgtype.Text
	var connID pgtype.UUID
	var number int32
	err := h.DB.QueryRow(r.Context(), gitMatchedSQL+` SELECT source,provider,instance_url,repo_owner,repo_name,external_repo_id,connection_id,pr_number FROM dedup WHERE id=$3`, ws, p, id).Scan(&source, &provider, &instance, &owner, &repo, &external, &connID, &number)
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusNotFound, "pull request not found in Git scope")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to resolve pull request")
		return
	}
	if number <= 0 {
		writeError(w, http.StatusBadRequest, "invalid pull request number")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
	defer cancel()
	// Do not forward credentials to redirect targets, particularly custom PAT headers.
	client := &http.Client{Timeout: 15 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	var out gitFilesPage
	switch {
	case source == "vcs" && provider == "gitlab":
		conn, e := h.Queries.GetVCSConnectionByID(ctx, connID)
		if e != nil || conn.WorkspaceID != ws || conn.Provider != provider || conn.InstanceUrl != instance {
			writeError(w, http.StatusNotFound, "platform connection unavailable")
			return
		}
		if !h.isVCSConfigured() {
			writeError(w, http.StatusServiceUnavailable, "platform credentials unavailable")
			return
		}
		token, e := h.openVCSSecret(conn.AccessTokenEncrypted)
		if e != nil || token == "" {
			writeError(w, http.StatusServiceUnavailable, "platform credentials unavailable")
			return
		}
		project := owner + "/" + repo
		if external.Valid && external.String != "" {
			id, e := strconv.ParseInt(external.String, 10, 64)
			if e != nil || id <= 0 {
				writeError(w, http.StatusBadRequest, "invalid provider repository ID")
				return
			}
			project = strconv.FormatInt(id, 10)
		}
		endpoint := fmt.Sprintf("%s/api/v4/projects/%s/merge_requests/%d/diffs?page=%d&per_page=%d", strings.TrimRight(conn.InstanceUrl, "/"), url.PathEscape(project), number, page, gitFilePageSize)
		out, err = fetchGitLabFiles(ctx, client, endpoint, token, page)
	case source == "github":
		installation, e := h.Queries.GetGitHubInstallationByID(ctx, connID)
		if e != nil || installation.WorkspaceID != ws {
			writeError(w, http.StatusNotFound, "platform connection unavailable")
			return
		}
		out, err = fetchGitHubFiles(ctx, client, installation.InstallationID, owner, repo, external, number, page)
	default:
		writeError(w, http.StatusNotImplemented, "file changes are not supported by this provider; open the original pull request")
		return
	}
	if err != nil {
		writeError(w, http.StatusBadGateway, "could not read file changes from the provider; check connection permissions or open the original pull request")
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func readGitFilesJSON(ctx context.Context, client *http.Client, endpoint, token, tokenHeader string, target any) (http.Header, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	if tokenHeader == "Authorization" {
		setGitHubAPIHeaders(req, token)
	} else {
		req.Header.Set(tokenHeader, token)
		req.Header.Set("Accept", "application/json")
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("provider status %d", resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, gitFileResponseLimit+1))
	if err != nil {
		return nil, err
	}
	if len(data) > gitFileResponseLimit {
		return nil, errors.New("provider response too large")
	}
	return resp.Header, json.Unmarshal(data, target)
}

func limitGitFilePatch(file *gitChangedFile) {
	if len(file.Patch) > gitFilePatchLimit {
		file.Patch = file.Patch[:gitFilePatchLimit]
		file.PatchTruncated = true
	}
}

func fetchGitLabFiles(ctx context.Context, client *http.Client, endpoint, token string, page int) (gitFilesPage, error) {
	var rows []struct {
		OldPath   string `json:"old_path"`
		NewPath   string `json:"new_path"`
		Diff      string `json:"diff"`
		New       bool   `json:"new_file"`
		Deleted   bool   `json:"deleted_file"`
		Renamed   bool   `json:"renamed_file"`
		Collapsed bool   `json:"collapsed"`
		TooLarge  bool   `json:"too_large"`
	}
	headers, err := readGitFilesJSON(ctx, client, endpoint, token, "PRIVATE-TOKEN", &rows)
	if err != nil {
		return gitFilesPage{}, err
	}
	out := gitFilesPage{Files: make([]gitChangedFile, 0, len(rows))}
	for _, row := range rows {
		file := gitChangedFile{Path: row.NewPath, Status: "modified", Patch: row.Diff, PatchTruncated: row.Collapsed || row.TooLarge}
		switch {
		case row.New:
			file.Status = "added"
		case row.Deleted:
			file.Status = "removed"
			file.Path = row.OldPath
		case row.Renamed:
			file.Status = "renamed"
			file.PreviousPath = row.OldPath
		}
		limitGitFilePatch(&file)
		out.Files = append(out.Files, file)
	}
	if n, e := strconv.Atoi(headers.Get("X-Next-Page")); e == nil && n > page && n <= 100 {
		out.NextPage = &n
	} else if headers.Get("X-Next-Page") == "" && len(rows) == gitFilePageSize && page < 100 {
		n := page + 1
		out.NextPage = &n
	}
	return out, nil
}

func fetchGitHubFiles(ctx context.Context, client *http.Client, installationID int64, owner, repo string, external pgtype.Text, number int32, page int) (gitFilesPage, error) {
	jwt, err := signGitHubAppJWT(time.Now())
	if err != nil || jwt == "" {
		return gitFilesPage{}, errors.New("GitHub App credentials unavailable")
	}
	// Restrict the short-lived token to the mirrored repository and read-only PR permission.
	body := map[string]any{"permissions": map[string]string{"pull_requests": "read"}}
	if external.Valid {
		n, e := strconv.ParseInt(external.String, 10, 64)
		if e != nil || n <= 0 {
			return gitFilesPage{}, errors.New("invalid repository id")
		}
		body["repository_ids"] = []int64{n}
	} else {
		body["repositories"] = []string{repo}
	}
	payload, err := json.Marshal(body)
	if err != nil {
		return gitFilesPage{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, fmt.Sprintf("%s/app/installations/%d/access_tokens", strings.TrimRight(githubAPIBase, "/"), installationID), strings.NewReader(string(payload)))
	if err != nil {
		return gitFilesPage{}, err
	}
	setGitHubAPIHeaders(req, jwt)
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return gitFilesPage{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		return gitFilesPage{}, fmt.Errorf("token status %d", resp.StatusCode)
	}
	var minted struct {
		Token string `json:"token"`
	}
	if err = json.NewDecoder(io.LimitReader(resp.Body, githubAPIResponseLimit)).Decode(&minted); err != nil || minted.Token == "" {
		return gitFilesPage{}, errors.New("could not mint repository token")
	}
	defer revokeGitHubInstallationToken(client, minted.Token)
	if owner == "" || repo == "" || owner == "." || owner == ".." || repo == "." || repo == ".." || strings.ContainsAny(owner+repo, "/\\") {
		return gitFilesPage{}, errors.New("invalid repository path")
	}
	endpoint := fmt.Sprintf("%s/repos/%s/%s/pulls/%d/files?page=%d&per_page=%d", strings.TrimRight(githubAPIBase, "/"), url.PathEscape(owner), url.PathEscape(repo), number, page, gitFilePageSize)
	var rows []struct {
		Filename  string `json:"filename"`
		Previous  string `json:"previous_filename"`
		Status    string `json:"status"`
		Patch     string `json:"patch"`
		Additions int    `json:"additions"`
		Deletions int    `json:"deletions"`
	}
	headers, err := readGitFilesJSON(ctx, client, endpoint, minted.Token, "Authorization", &rows)
	if err != nil {
		return gitFilesPage{}, err
	}
	out := gitFilesPage{Files: make([]gitChangedFile, 0, len(rows))}
	for _, row := range rows {
		file := gitChangedFile{Path: row.Filename, PreviousPath: row.Previous, Status: row.Status, Patch: row.Patch, Additions: &row.Additions, Deletions: &row.Deletions}
		limitGitFilePatch(&file)
		out.Files = append(out.Files, file)
	}
	if strings.Contains(headers.Get("Link"), `rel="next"`) && page < 100 {
		n := page + 1
		out.NextPage = &n
	}
	return out, nil
}
