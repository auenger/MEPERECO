package handler

import (
	"net/http"
	"strconv"
	"strings"
	"unicode/utf8"

	db "github.com/auenger/MEPERECO/core/pkg/db/generated"
)

const inviteCandidatePageSize = 30

type inviteCandidate struct {
	ID        string  `json:"id"`
	Name      string  `json:"name"`
	Email     string  `json:"email"`
	AvatarURL *string `json:"avatar_url"`
}

// A leading @ or a bare domain searches the exact email domain, rather than
// matching other domains that just contain the same substring.
func normalizeInviteSearch(raw string) (term string, domainOnly bool, valid bool) {
	term = strings.ToLower(strings.TrimSpace(raw))
	domainOnly = strings.HasPrefix(term, "@") ||
		(strings.Contains(term, ".") && !strings.ContainsAny(term, "@ \t\r\n"))
	if domainOnly {
		term = strings.TrimPrefix(term, "@")
	}
	valid = utf8.RuneCountInString(term) >= 2 && len(term) <= 254
	return
}

// SearchWorkspaceInviteCandidates returns registered users who can be invited
// to the workspace. Only owners and admins may access the directory search.
func (h *Handler) SearchWorkspaceInviteCandidates(w http.ResponseWriter, r *http.Request) {
	requester, ok := h.workspaceMember(w, r, workspaceIDFromURL(r, "id"))
	if !ok {
		return
	}
	if !roleAllowed(requester.Role, "owner", "admin") {
		writeError(w, http.StatusForbidden, "insufficient workspace role")
		return
	}
	term, domainOnly, valid := normalizeInviteSearch(r.URL.Query().Get("q"))
	if !valid {
		writeError(w, http.StatusBadRequest, "search requires at least two characters")
		return
	}
	page := 0
	if raw := r.URL.Query().Get("page"); raw != "" {
		var err error
		page, err = strconv.Atoi(raw)
		if err != nil || page < 0 || page > 2147483647/inviteCandidatePageSize {
			writeError(w, http.StatusBadRequest, "invalid page")
			return
		}
	}
	rows, err := h.Queries.SearchWorkspaceInviteCandidates(r.Context(), db.SearchWorkspaceInviteCandidatesParams{
		WorkspaceID: requester.WorkspaceID,
		DomainOnly:  domainOnly,
		SearchTerm:  term,
		PageOffset:  int32(page * inviteCandidatePageSize),
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to search invite candidates")
		return
	}
	hasMore := len(rows) > inviteCandidatePageSize
	if hasMore {
		rows = rows[:inviteCandidatePageSize]
	}
	items := make([]inviteCandidate, 0, len(rows))
	for _, row := range rows {
		items = append(items, inviteCandidate{
			ID: uuidToString(row.ID), Name: row.Name, Email: row.Email, AvatarURL: textToPtr(row.AvatarUrl),
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items, "has_more": hasMore})
}
