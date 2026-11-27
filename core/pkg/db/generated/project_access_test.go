package db

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

// The test uses a transaction-local schema and an isolated database only.
func TestProjectAccessPostgres(t *testing.T) {
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
	exec("CREATE SCHEMA project_access_test")
	exec("SET LOCAL search_path TO project_access_test")
	exec("CREATE TABLE project(id uuid PRIMARY KEY DEFAULT gen_random_uuid(), workspace_id uuid NOT NULL)")
	exec("CREATE TABLE member(id uuid PRIMARY KEY DEFAULT gen_random_uuid(), workspace_id uuid NOT NULL,user_id uuid NOT NULL,role text NOT NULL)")
	exec("CREATE TABLE task(id uuid PRIMARY KEY DEFAULT gen_random_uuid(), workspace_id uuid, project_id uuid)")
	exec("CREATE TABLE chat_session(id uuid PRIMARY KEY DEFAULT gen_random_uuid(), workspace_id uuid, project_id uuid)")
	exec("CREATE TABLE attachment(id uuid PRIMARY KEY DEFAULT gen_random_uuid(), workspace_id uuid, task_id uuid, chat_session_id uuid)")
	exec("CREATE TABLE attachment_reference(attachment_id uuid, workspace_id uuid, target_type text,target_id uuid)")
	ws := row("SELECT gen_random_uuid()")
	other := row("SELECT gen_random_uuid()")
	admin := row("SELECT gen_random_uuid()")
	existing := row("SELECT gen_random_uuid()")
	employee := row("SELECT gen_random_uuid()")
	exec("INSERT INTO member(workspace_id,user_id,role) VALUES($1,$2,'owner'),($1,$3,'member'),($4,$5,'member')", ws, admin, existing, other, employee)
	p1 := row("INSERT INTO project(workspace_id) VALUES($1) RETURNING id", ws)
	migration, err := os.ReadFile(filepath.Join("..", "..", "..", "migrations", "0385_project_members.up.sql"))
	if err != nil {
		t.Fatal(err)
	}
	exec(string(migration))
	exec("INSERT INTO member(workspace_id,user_id,role) VALUES($1,$2,'member')", ws, employee)
	p2 := row("INSERT INTO project(workspace_id) VALUES($1) RETURNING id", ws)
	q := New(tx)
	assert := func(user, project pgtype.UUID, want bool) {
		t.Helper()
		got, err := q.CanAccessProject(ctx, ws, user, project)
		if err != nil || got != want {
			t.Fatalf("access got %v want %v (err %v)", got, want, err)
		}
	}
	assert(admin, p1, true)
	assert(admin, p2, true)
	assert(existing, p1, true)
	assert(existing, p2, false)
	assert(employee, p1, false)
	if err := q.GrantProjectMember(ctx, ws, p1, employee); err != nil {
		t.Fatal(err)
	}
	assert(employee, p1, true)
	assert(employee, p2, false)
	taskID := row("INSERT INTO task(workspace_id,project_id) VALUES($1,$2) RETURNING id", ws, p2)
	attID := row("INSERT INTO attachment(workspace_id,task_id) VALUES($1,$2) RETURNING id", ws, taskID)
	allowed, err := q.CanAccessAttachment(ctx, ws, employee, attID)
	if err != nil || allowed {
		t.Fatalf("unauthorized project attachment visible: %v %v", allowed, err)
	}
	allowed, err = q.CanAccessAttachment(ctx, ws, admin, attID)
	if err != nil || !allowed {
		t.Fatalf("admin project attachment denied: %v %v", allowed, err)
	}
	ids, err := q.ListAccessibleProjectIDs(ctx, ws, employee)
	if err != nil || len(ids) != 1 || ids[0] != p1 {
		t.Fatalf("unexpected allowed project IDs %v: %v", ids, err)
	}
	if err := q.GrantProjectMember(ctx, other, p2, employee); err != nil {
		t.Fatal(err)
	}
	assert(employee, p2, false)
	if err := q.RevokeProjectMember(ctx, ws, p1, employee); err != nil {
		t.Fatal(err)
	}
	assert(employee, p1, false)
	if err := q.GrantProjectMember(ctx, ws, p2, employee); err != nil {
		t.Fatal(err)
	}
	exec("DELETE FROM member WHERE workspace_id=$1 AND user_id=$2", ws, employee)
	exec("INSERT INTO member(workspace_id,user_id,role) VALUES($1,$2,'member')", ws, employee)
	assert(employee, p2, false)
	// Cross-workspace member rows cannot confer access, even if a malformed grant exists.
	exec("INSERT INTO project_member SELECT $1,id FROM member WHERE workspace_id=$2 AND user_id=$3", p2, other, employee)
	assert(employee, p2, false)
}
