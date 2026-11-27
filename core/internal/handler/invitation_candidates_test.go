package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"testing"

	"github.com/auenger/MEPERECO/core/internal/middleware"
	db "github.com/auenger/MEPERECO/core/pkg/db/generated"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

func TestNormalizeInviteSearch(t *testing.T) {
	for _, tc := range []struct {
		raw, term     string
		domain, valid bool
	}{
		{"  @Example.COM ", "example.com", true, true},
		{"Example.COM", "example.com", true, true},
		{"Alice@Example.COM", "alice@example.com", false, true},
		{" Alice ", "alice", false, true},
		{"张三", "张三", false, true},
		{"@", "", true, false},
		{"a", "a", false, false},
		{"  ", "", false, false},
	} {
		term, domain, valid := normalizeInviteSearch(tc.raw)
		if term != tc.term || domain != tc.domain || valid != tc.valid {
			t.Errorf("%q: got (%q, %v, %v)", tc.raw, term, domain, valid)
		}
	}
}

func TestInviteCandidateSearchRejectsInvalidRequests(t *testing.T) {
	h := &Handler{}
	for _, tc := range []struct {
		role, query string
		want        int
	}{
		{"member", "q=example.com", http.StatusForbidden},
		{"admin", "q=@", http.StatusBadRequest},
		{"owner", "q=example.com&page=-1", http.StatusBadRequest},
		{"owner", "q=example.com&page=oops", http.StatusBadRequest},
	} {
		r := httptest.NewRequest(http.MethodGet, "/?"+tc.query, nil)
		r = r.WithContext(middleware.SetMemberContext(r.Context(), "workspace", db.Member{Role: tc.role}))
		w := httptest.NewRecorder()
		h.SearchWorkspaceInviteCandidates(w, r)
		if w.Code != tc.want {
			t.Errorf("%s %s: got %d, want %d", tc.role, tc.query, w.Code, tc.want)
		}
	}
}

// This test uses only temporary tables in a transaction. Point it at an
// isolated PostgreSQL instance via MEPERSTUDIO_INVITE_TEST_DATABASE_URL.
func TestInviteCandidateSearchPostgres(t *testing.T) {
	dsn := os.Getenv("MEPERSTUDIO_INVITE_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("isolated invitation test database required")
	}
	ctx := context.Background()
	conn, err := pgx.Connect(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(ctx)
	tx, err := conn.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	exec := func(sql string, args ...any) {
		t.Helper()
		if _, err := tx.Exec(ctx, sql, args...); err != nil {
			t.Fatal(err)
		}
	}
	exec(`CREATE TEMP TABLE "user" (id uuid DEFAULT gen_random_uuid(), name text, email text, avatar_url text)`)
	exec(`CREATE TEMP TABLE member (workspace_id uuid, user_id uuid)`)
	exec(`CREATE TEMP TABLE workspace_invitation (workspace_id uuid, invitee_user_id uuid, invitee_email text, status text, expires_at timestamptz)`)
	var workspaceID pgtype.UUID
	if err := tx.QueryRow(ctx, `SELECT gen_random_uuid()`).Scan(&workspaceID); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 35; i++ {
		exec(`INSERT INTO "user" (name,email) VALUES ($1,$2)`, fmt.Sprintf("Person %02d", i), fmt.Sprintf("person%02d@example.com", i))
	}
	exec(`INSERT INTO "user" (name,email) VALUES ('Other domain','person@notexample.com'), ('Subdomain','sub@sub.example.com')`)
	exec(`INSERT INTO member SELECT $1,id FROM "user" WHERE email = 'person00@example.com'`, workspaceID)
	exec(`INSERT INTO workspace_invitation SELECT $1,id,email,'pending',now()+interval '1 day' FROM "user" WHERE email = 'person01@example.com'`, workspaceID)
	exec(`INSERT INTO workspace_invitation SELECT $1,NULL,'PERSON02@example.com','pending',now()+interval '1 day'`, workspaceID)
	exec(`INSERT INTO workspace_invitation SELECT $1,id,email,'pending',now()-interval '1 day' FROM "user" WHERE email = 'person03@example.com'`, workspaceID)
	h := &Handler{Queries: db.New(tx)}
	search := func(q string, page int) ([]inviteCandidate, bool) {
		t.Helper()
		r := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/?q=%s&page=%d", url.QueryEscape(q), page), nil)
		r = r.WithContext(middleware.SetMemberContext(r.Context(), uuidToString(workspaceID), db.Member{Role: "admin", WorkspaceID: workspaceID}))
		w := httptest.NewRecorder()
		h.SearchWorkspaceInviteCandidates(w, r)
		if w.Code != http.StatusOK {
			t.Fatalf("search %q: status %d: %s", q, w.Code, w.Body.String())
		}
		var response struct {
			Items   []inviteCandidate `json:"items"`
			HasMore bool              `json:"has_more"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
			t.Fatal(err)
		}
		return response.Items, response.HasMore
	}
	first, more := search("@EXAMPLE.COM", 0)
	if len(first) != 30 || !more || first[0].Email != "person03@example.com" {
		t.Fatalf("unexpected first page: %v, %v", first, more)
	}
	second, more := search("example.com", 1)
	if len(second) != 2 || more || second[0].Email != "person33@example.com" {
		t.Fatalf("unexpected second page: %v, %v", second, more)
	}
	items, _ := search("Person 03", 0)
	if len(items) != 1 || items[0].Email != "person03@example.com" {
		t.Fatalf("name search: %v", items)
	}
	items, _ = search("person04@EXAMPLE.COM", 0)
	if len(items) != 1 || items[0].Email != "person04@example.com" {
		t.Fatalf("email search: %v", items)
	}
	items, _ = search("person01@example.com", 0)
	if len(items) != 0 {
		t.Fatalf("pending invitation returned: %v", items)
	}
	items, _ = search("%%", 0)
	if len(items) != 0 {
		t.Fatalf("wildcard treated as pattern: %v", items)
	}
}
