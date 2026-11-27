package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/auenger/MEPERECO/core/internal/middleware"
	db "github.com/auenger/MEPERECO/core/pkg/db/generated"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

func TestFixedProjectGroupChatPostgres(t *testing.T) {
	dsn := os.Getenv("MEPERSTUDIO_PROJECT_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("isolated project access PostgreSQL database required")
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
	row := func(sql string, args ...any) pgtype.UUID {
		t.Helper()
		var id pgtype.UUID
		if err := tx.QueryRow(ctx, sql, args...).Scan(&id); err != nil {
			t.Fatal(err)
		}
		return id
	}
	exec("CREATE SCHEMA project_chat_test")
	exec("SET LOCAL search_path TO project_chat_test")
	exec("CREATE TABLE project(id uuid PRIMARY KEY DEFAULT gen_random_uuid(), workspace_id uuid)")
	exec("CREATE TABLE member(id uuid PRIMARY KEY DEFAULT gen_random_uuid(), workspace_id uuid,user_id uuid,role text)")
	exec("CREATE TABLE project_member(project_id uuid, member_id uuid, PRIMARY KEY(project_id,member_id))")
	exec(`CREATE TABLE chat_session(
        id uuid PRIMARY KEY DEFAULT gen_random_uuid(),workspace_id uuid,agent_id uuid,creator_id uuid,
        title text DEFAULT 'Project group',session_id text,work_dir text,status text DEFAULT 'active',
        created_at timestamptz DEFAULT now(),updated_at timestamptz DEFAULT now(),unread_since timestamptz,
        runtime_id uuid,last_read_at timestamptz DEFAULT now(),is_agent_intro boolean DEFAULT false,
        pinned_at timestamptz,project_id uuid,kind text DEFAULT 'group')`)
	exec("CREATE TABLE chat_session_participant(chat_session_id uuid,participant_type text,participant_id uuid)")
	ws := row("SELECT gen_random_uuid()")
	creator := row("SELECT gen_random_uuid()")
	employee := row("SELECT gen_random_uuid()")
	project := row("INSERT INTO project(workspace_id) VALUES($1) RETURNING id", ws)
	other := row("INSERT INTO project(workspace_id) VALUES($1) RETURNING id", ws)
	exec("INSERT INTO member(workspace_id,user_id,role) VALUES($1,$2,'owner'),($1,$3,'member')", ws, creator, employee)
	session := row("INSERT INTO chat_session(workspace_id,creator_id,project_id) VALUES($1,$2,$3) RETURNING id", ws, creator, project)
	exec("INSERT INTO chat_session_participant VALUES($1,'member',$2),($1,'member',$3)", session, creator, employee)
	q := db.New(tx)
	h := &Handler{Queries: q, TxStarter: tx}
	request := func(user pgtype.UUID, role, method, body string) *http.Request {
		r := httptest.NewRequest(method, "/", strings.NewReader(body))
		r.Header.Set("X-User-ID", uuidToString(user))
		rc := chi.NewRouteContext()
		rc.URLParams.Add("sessionId", uuidToString(session))
		c := context.WithValue(r.Context(), chi.RouteCtxKey, rc)
		return r.WithContext(middleware.SetMemberContext(c, uuidToString(ws), db.Member{WorkspaceID: ws, UserID: user, Role: role}))
	}
	checkRead := func(user pgtype.UUID, role string, want bool) {
		t.Helper()
		w := httptest.NewRecorder()
		_, got := h.loadChatSessionForUser(w, request(user, role, http.MethodGet, ""), uuidToString(user), uuidToString(ws), uuidToString(session))
		if got != want {
			t.Fatalf("group read got %v want %v; %s", got, want, w.Body.String())
		}
	}
	checkRead(creator, "owner", true)
	checkRead(employee, "member", false)
	if err := q.GrantProjectMember(ctx, ws, project, employee); err != nil {
		t.Fatal(err)
	}
	checkRead(employee, "member", true)
	for _, body := range []string{`{"project_id":null}`, `{"project_id":"` + uuidToString(other) + `"}`} {
		w := httptest.NewRecorder()
		h.UpdateChatSession(w, request(creator, "owner", http.MethodPatch, body))
		if w.Code != http.StatusConflict {
			t.Fatalf("fixed project update returned %d: %s", w.Code, w.Body.String())
		}
	}
	if err := q.RevokeProjectMember(ctx, ws, project, employee); err != nil {
		t.Fatal(err)
	}
	checkRead(employee, "member", false)
}
