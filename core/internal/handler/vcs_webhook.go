package handler

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/silieco-ai/silieco/core/internal/integrations/vcs"
	db "github.com/silieco-ai/silieco/core/pkg/db/generated"
	"github.com/silieco-ai/silieco/core/pkg/protocol"
)

// ── Response mappers ────────────────────────────────────────────────────────

// vcsPullRequestToResponse maps a stored VCS PR onto the shared PR response
// shape for single-PR webhook broadcasts (no aggregated check counts; the
// frontend re-queries the task's PR list for fresh counts).
func vcsPullRequestToResponse(p db.VcsPullRequest) GitHubPullRequestResponse {
	return GitHubPullRequestResponse{
		ID:               uuidToString(p.ID),
		Provider:         p.Provider,
		WorkspaceID:      uuidToString(p.WorkspaceID),
		RepoOwner:        p.RepoOwner,
		RepoName:         p.RepoName,
		Number:           p.PrNumber,
		Title:            p.Title,
		State:            p.State,
		HtmlURL:          p.HtmlUrl,
		Branch:           textToPtr(p.Branch),
		AuthorLogin:      textToPtr(p.AuthorLogin),
		AuthorAvatarURL:  textToPtr(p.AuthorAvatarUrl),
		MergedAt:         timestampToPtr(p.MergedAt),
		ClosedAt:         timestampToPtr(p.ClosedAt),
		PRCreatedAt:      timestampToString(p.PrCreatedAt),
		PRUpdatedAt:      timestampToString(p.PrUpdatedAt),
		MergeableState:   nil,
		ChecksConclusion: nil,
		Additions:        p.Additions,
		Deletions:        p.Deletions,
		ChangedFiles:     p.ChangedFiles,
	}
}

// vcsPullRequestRowToResponse maps an task's PR-list row, which carries the
// aggregated commit-status counts, onto the shared response shape.
func vcsPullRequestRowToResponse(p db.ListVCSPullRequestsByTaskRow) GitHubPullRequestResponse {
	return GitHubPullRequestResponse{
		ID:               uuidToString(p.ID),
		Provider:         p.Provider,
		WorkspaceID:      uuidToString(p.WorkspaceID),
		RepoOwner:        p.RepoOwner,
		RepoName:         p.RepoName,
		Number:           p.PrNumber,
		Title:            p.Title,
		State:            p.State,
		HtmlURL:          p.HtmlUrl,
		Branch:           textToPtr(p.Branch),
		AuthorLogin:      textToPtr(p.AuthorLogin),
		AuthorAvatarURL:  textToPtr(p.AuthorAvatarUrl),
		MergedAt:         timestampToPtr(p.MergedAt),
		ClosedAt:         timestampToPtr(p.ClosedAt),
		PRCreatedAt:      timestampToString(p.PrCreatedAt),
		PRUpdatedAt:      timestampToString(p.PrUpdatedAt),
		MergeableState:   nil,
		ChecksConclusion: aggregateChecksConclusion(p.ChecksFailed, p.ChecksPassed, p.ChecksPending, p.ChecksTotal),
		ChecksTotal:      p.ChecksTotal,
		ChecksPassed:     p.ChecksPassed,
		ChecksFailed:     p.ChecksFailed,
		ChecksPending:    p.ChecksPending,
		ChecksRunning:    p.ChecksPending,
		FailedCheckNames: []string{},
		Additions:        p.Additions,
		Deletions:        p.Deletions,
		ChangedFiles:     p.ChangedFiles,
	}
}

// ── Webhook ─────────────────────────────────────────────────────────────────

// HandleVCSWebhook (POST /api/webhooks/vcs/{connectionId}) authenticates and
// mirrors webhooks from any token-based Git provider. The connection id in the path
// selects the workspace, the provider, and the decryption secret; the provider
// adapter handles the provider-specific signature scheme, event header, and
// payload shape, returning normalized events to the shared mirror logic below.
func (h *Handler) HandleVCSWebhook(w http.ResponseWriter, r *http.Request) {
	// Where the integration is off (the managed cloud) the endpoint behaves as
	// if it does not exist — a bare 404 that reveals nothing about config, the
	// same response a genuinely unknown connection id gets below.
	if !h.isVCSAvailable() {
		writeError(w, http.StatusNotFound, "unknown connection")
		return
	}
	if !h.isVCSConfigured() {
		writeError(w, http.StatusServiceUnavailable, "vcs webhooks not configured")
		return
	}
	connUUID, ok := parseUUIDOrBadRequest(w, chi.URLParam(r, "connectionId"), "connection id")
	if !ok {
		return
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, 10<<20)) // 10 MiB cap
	if err != nil {
		writeError(w, http.StatusBadRequest, "read body failed")
		return
	}

	conn, err := h.Queries.GetVCSConnectionByID(r.Context(), connUUID)
	if err != nil {
		if !errors.Is(err, pgx.ErrNoRows) {
			slog.Warn("vcs: lookup connection failed", "err", err)
		}
		writeError(w, http.StatusNotFound, "unknown connection")
		return
	}
	provider, ok := vcs.For(conn.Provider)
	if !ok {
		slog.Error("vcs: connection has unknown provider", "provider", conn.Provider)
		writeError(w, http.StatusInternalServerError, "unknown provider")
		return
	}

	secret, err := h.openVCSSecret(conn.WebhookSecretEncrypted)
	if err != nil {
		slog.Error("vcs: decrypt webhook secret failed", "err", err)
		writeError(w, http.StatusInternalServerError, "secret error")
		return
	}
	if !provider.VerifySignature(secret, r.Header, body) {
		writeError(w, http.StatusUnauthorized, "invalid signature")
		return
	}

	switch provider.EventKind(r.Header) {
	case vcs.EventPullRequest:
		if pr, err := provider.ParsePullRequest(body); err != nil {
			slog.Warn("vcs: bad pull_request payload", "provider", conn.Provider, "err", err)
		} else {
			if err := h.mirrorVCSPullRequest(r.Context(), conn, pr); err != nil {
				writeError(w, http.StatusServiceUnavailable, "PR mirror unavailable; retry delivery")
				return
			}
		}
	case vcs.EventCIStatus:
		if st, err := provider.ParseCIStatus(body); err != nil {
			slog.Warn("vcs: bad status payload", "provider", conn.Provider, "err", err)
		} else {
			if err := h.mirrorVCSCIStatus(r.Context(), conn, st); err != nil {
				writeError(w, http.StatusServiceUnavailable, "CI mirror unavailable; retry delivery")
				return
			}
		}
	default:
		// Acknowledge unmodelled events so the provider doesn't flag the hook.
	}
	w.WriteHeader(http.StatusAccepted)
}

func (h *Handler) mirrorVCSPullRequest(ctx context.Context, conn db.VcsConnection, ev vcs.PullRequestEvent) error {
	if ev.RepoOwner == "" || ev.RepoName == "" || ev.Number == 0 {
		slog.Warn("vcs: pull_request missing repo identity", "provider", conn.Provider)
		return nil
	}

	pr, err := h.Queries.UpsertVCSPullRequest(ctx, db.UpsertVCSPullRequestParams{
		WorkspaceID:     conn.WorkspaceID,
		ConnectionID:    conn.ID,
		Provider:        conn.Provider,
		ExternalRepoID:  ptrToText(strPtrOrNil(ev.RepoID)),
		RepoOwner:       ev.RepoOwner,
		RepoName:        ev.RepoName,
		PrNumber:        ev.Number,
		Title:           ev.Title,
		State:           ev.State,
		HtmlUrl:         ev.HTMLURL,
		Branch:          ptrToText(strPtrOrNil(ev.Branch)),
		AuthorLogin:     ptrToText(strPtrOrNil(ev.AuthorLogin)),
		AuthorAvatarUrl: ptrToText(strPtrOrNil(ev.AuthorAvatarURL)),
		MergedAt:        parseGHTime(ev.MergedAt),
		ClosedAt:        parseGHTime(ev.ClosedAt),
		PrCreatedAt:     parseGHTimeRequired(ev.CreatedAt),
		PrUpdatedAt:     parseGHTimeRequired(ev.UpdatedAt),
		Additions:       ev.Additions,
		Deletions:       ev.Deletions,
		ChangedFiles:    ev.ChangedFiles,
		HeadSha:         ev.HeadSHA,
	})
	if err != nil {
		slog.Warn("vcs: upsert pr failed", "err", err)
		return err
	}

	// Out-of-order guard for the link metadata. UpsertVCSPullRequest keeps the
	// newer persisted row on a stale redelivery, so `pr` may reflect a newer
	// event than this `ev`. Everything the link write derives below —
	// close_intent, reference_only, preserveCloseIntent — comes from `ev`, so
	// rewriting the link from a stale event would corrupt what the newer event
	// already set (e.g. a redelivered older "opened" event flipping a merged
	// PR's link back to reference_only, blocking auto-advance). If the persisted
	// row is strictly newer than this event, the newer event already linked and
	// published — stop here. (An event with no usable timestamp falls back to
	// now(), which is never strictly after the stored value, so it proceeds.)
	evUpdatedAt := parseGHTimeRequired(ev.UpdatedAt)
	if pr.PrUpdatedAt.Valid && evUpdatedAt.Valid && pr.PrUpdatedAt.Time.After(evUpdatedAt.Time) {
		return nil
	}

	workspaceID := uuidToString(conn.WorkspaceID)
	resp := vcsPullRequestToResponse(pr)

	// Auto-link to tasks by identifiers in title/body/branch. Connecting a
	// a provider is the opt-in, so there is no separate per-workspace flag. The
	// task-side machinery is shared with GitHub.
	linkedTaskIDs := make([]string, 0)
	idents := extractIdentifiers(ev.Title, ev.Body, ev.Branch)
	closingIdents := map[string]struct{}{}
	for _, c := range extractClosingIdentifiers(ev.Title, ev.Body) {
		closingIdents[c] = struct{}{}
	}
	// qualifyingIdents genuinely tie this PR to an task: a title prefix, a
	// branch-name reference, or a body closing keyword. An identifier matched
	// ONLY by a bare body mention is reference_only — it links (so the PR shows
	// in history) but is hidden from the task PR list and excluded from the
	// close aggregate, so a drive-by "Related SILI-1" neither looks like a
	// working PR nor blocks a genuine Closes sibling from advancing the task.
	// Mirrors the GitHub path (SILI-3739); branch is deliberately excluded from
	// the closing-keyword scan there and here.
	qualifyingIdents := map[string]struct{}{}
	for _, id := range extractIdentifiers(ev.Title, ev.Branch) {
		qualifyingIdents[id] = struct{}{}
	}
	for c := range closingIdents {
		qualifyingIdents[c] = struct{}{}
	}
	// Freeze close_intent once the terminal merge/close event has arrived.
	preserveCloseIntent := !ev.Terminal() && (ev.State == "merged" || ev.State == "closed")
	prefix := h.getTaskPrefix(ctx, conn.WorkspaceID)
	reevalTasks := make([]db.Task, 0, len(idents))
	var linkErr error
	for _, id := range idents {
		task, ok := h.lookupTaskByIdentifier(ctx, conn.WorkspaceID, prefix, id)
		if !ok {
			continue
		}
		_, declared := closingIdents[id]
		eligible := h.taskGitRepositoryEligible(ctx, task, "vcs", conn.ID, conn.InstanceUrl, ev.RepoOwner+"/"+ev.RepoName, ev.RepoID)
		closeIntent := declared && !preserveCloseIntent && eligible
		_, qualifies := qualifyingIdents[id]
		referenceOnly := !qualifies || !eligible
		if err := h.Queries.LinkTaskToVCSPullRequest(ctx, db.LinkTaskToVCSPullRequestParams{
			TaskID:              task.ID,
			PullRequestID:       pr.ID,
			CloseIntent:         closeIntent,
			ReferenceOnly:       referenceOnly,
			PreserveCloseIntent: preserveCloseIntent,
			LinkedByType:        strToText("system"),
			LinkedByID:          pgtype.UUID{},
		}); err != nil {
			slog.Warn("vcs: link failed", "err", err)
			linkErr = err
			continue
		}
		linkedTaskIDs = append(linkedTaskIDs, uuidToString(task.ID))
		if eligible {
			reevalTasks = append(reevalTasks, task)
		}
	}

	if ev.State == "merged" || ev.State == "closed" {
		for _, task := range reevalTasks {
			if task.Status == "done" || task.Status == "cancelled" {
				continue
			}
			counts, err := h.Queries.GetTaskCombinedPullRequestCloseAggregate(ctx, task.ID)
			if err != nil {
				slog.Warn("vcs: count linked pr states failed", "err", err, "task_id", uuidToString(task.ID))
				continue
			}
			if counts.OpenCount == 0 && counts.MergedWithCloseIntentCount > 0 {
				h.advanceTaskToDone(ctx, task, workspaceID)
			}
		}
	}

	h.publish(protocol.EventPullRequestUpdated, workspaceID, "system", "", map[string]any{
		"pull_request":    resp,
		"linked_task_ids": linkedTaskIDs,
		"project_ids":     h.gitProjectsForRepository(ctx, conn.WorkspaceID, "vcs", conn.ID, ev.RepoOwner+"/"+ev.RepoName, ev.RepoID),
	})
	return linkErr
}

func (h *Handler) mirrorVCSCIStatus(ctx context.Context, conn db.VcsConnection, ev vcs.CIStatusEvent) error {
	if ev.SHA == "" || ev.State == "" {
		return nil
	}
	// Repository-qualified and legacy task checks commit together. Lock the
	// workspace/connection in the same order as connection deletion.
	tx, err := h.TxStarter.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var id pgtype.UUID
	if err = tx.QueryRow(ctx, `SELECT id FROM workspace WHERE id=$1 FOR KEY SHARE`, conn.WorkspaceID).Scan(&id); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		return err
	}
	if err = tx.QueryRow(ctx, `SELECT id FROM vcs_connection WHERE id=$1 AND workspace_id=$2 FOR KEY SHARE`, conn.ID, conn.WorkspaceID).Scan(&id); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		return err
	}
	qtx := h.Queries.WithTx(tx)
	stamp := parseGHTimeRequired(ev.UpdatedAt)
	if err = qtx.UpsertVCSCommitStatus(ctx, db.UpsertVCSCommitStatusParams{
		ConnectionID: conn.ID, Sha: ev.SHA, Context: ev.Context, State: ev.State,
		TargetUrl: ptrToText(strPtrOrNil(ev.TargetURL)), Description: ptrToText(strPtrOrNil(ev.Description)), UpdatedAt: stamp,
	}); err != nil {
		slog.Warn("vcs: upsert commit status failed", "err", err)
		return err
	}
	path, pathErr := vcs.NormalizeRepositoryPath(ev.RepoPath)
	if pathErr == nil {
		_, err = tx.Exec(ctx, `INSERT INTO vcs_repo_commit_status(connection_id,repository_path,sha,context,state,target_url,description,updated_at)
		VALUES($1,$2,$3,$4,$5,$6,$7,$8)
		ON CONFLICT(connection_id,repository_path,sha,context) DO UPDATE SET state=EXCLUDED.state,target_url=EXCLUDED.target_url,description=EXCLUDED.description,updated_at=EXCLUDED.updated_at
		WHERE EXCLUDED.updated_at>=vcs_repo_commit_status.updated_at`, conn.ID, path, ev.SHA, ev.Context, ev.State, ptrToText(strPtrOrNil(ev.TargetURL)), ptrToText(strPtrOrNil(ev.Description)), stamp)
		if err != nil {
			slog.Warn("vcs: project CI mirror failed", "err", err)
			return err
		}
	}
	taskIDs, err := qtx.ListTaskIDsForVCSPRHead(ctx, db.ListTaskIDsForVCSPRHeadParams{ConnectionID: conn.ID, HeadSha: ev.SHA})
	if err != nil {
		return err
	}
	if err = tx.Commit(ctx); err != nil {
		return err
	}
	workspaceID := uuidToString(conn.WorkspaceID)
	if pathErr == nil {
		h.publish(protocol.EventPullRequestUpdated, workspaceID, "system", "", map[string]any{"project_ids": h.gitProjectsForRepository(ctx, conn.WorkspaceID, "vcs", conn.ID, path, ev.RepoID)})
	}
	for _, taskID := range taskIDs {
		h.publish(protocol.EventPullRequestUpdated, workspaceID, "system", "", map[string]any{"task_id": uuidToString(taskID)})
	}
	return nil
}
