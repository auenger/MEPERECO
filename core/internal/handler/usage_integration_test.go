package handler

import (
	"context"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	db "github.com/auenger/MEPERECO/core/pkg/db/generated"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestDashboardRunKind(t *testing.T) {
	for _, input := range []string{"", "all", "chat", "task", "other", "invalid"} {
		r := httptest.NewRequest("GET", "/?run_kind="+input, nil)
		w := httptest.NewRecorder()
		result, ok := parseDashboardRunKind(w, r)
		if input == "invalid" {
			if ok || w.Code != 400 {
				t.Fatal(input, w.Code)
			}
			continue
		}
		want := input
		if input == "all" {
			want = ""
		}
		if !ok || result != want {
			t.Fatal(input, result, ok)
		}
	}
}

// Requires an isolated migrated PostgreSQL database. Do not fall back to the
// service's DATABASE_URL. All fixtures and rollup state changes are rolled back.
func TestUsageLedgerPostgres(t *testing.T) {
	url := os.Getenv("MEPERSTUDIO_USAGE_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("isolated usage database required")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	row := func(sql string, args ...any) pgtype.UUID {
		t.Helper()
		var id pgtype.UUID
		if e := tx.QueryRow(ctx, sql, args...).Scan(&id); e != nil {
			t.Fatal(e)
		}
		return id
	}
	exec := func(sql string, args ...any) {
		t.Helper()
		if _, e := tx.Exec(ctx, sql, args...); e != nil {
			t.Fatal(e)
		}
	}
	ws := row(`INSERT INTO workspace(name,slug,task_prefix) VALUES('Usage test','usage-test-'||gen_random_uuid()::text,'UT') RETURNING id`)
	actor := row(`INSERT INTO "user"(name,email) VALUES('Usage owner',gen_random_uuid()::text||'@example.test') RETURNING id`)
	rt := row(`INSERT INTO agent_runtime(workspace_id,name,runtime_mode,provider,owner_id) VALUES($1,'Fixture','local','claude',$2) RETURNING id`, ws, actor)
	agent := row(`INSERT INTO agent(workspace_id,name,runtime_mode,owner_id,runtime_id) VALUES($1,'Fixture','local',$2,$3) RETURNING id`, ws, actor, rt)
	project := row(`INSERT INTO project(workspace_id,title) VALUES($1,'Project') RETURNING id`, ws)
	task := row(`INSERT INTO task(workspace_id,title,status,creator_type,creator_id,project_id,number) VALUES($1,'Task','done','member',$2,$3,1) RETURNING id`, ws, actor, project)
	chat := row(`INSERT INTO chat_session(workspace_id,agent_id,creator_id,project_id,title) VALUES($1,$2,$3,$4,'Direct chat') RETURNING id`, ws, agent, actor, project)
	group := row(`INSERT INTO chat_session(workspace_id,creator_id,kind,title) VALUES($1,$2,'group','Group chat') RETURNING id`, ws, actor)
	// Use four different runs with one model, so aggregating by model cannot
	// obscure whether all four sources and their individual counts were retained.
	for _, f := range []struct {
		task, chat pgtype.UUID
		status     string
		tokens     int64
	}{{task, pgtype.UUID{}, "completed", 100}, {pgtype.UUID{}, chat, "failed", 200}, {pgtype.UUID{}, group, "completed", 300}, {pgtype.UUID{}, pgtype.UUID{}, "completed", 400}} {
		run := row(`INSERT INTO agent_run(agent_id,runtime_id,task_id,chat_session_id,status,started_at,completed_at) VALUES($1,$2,$3,$4,$5,now()-interval '1 hour 1 minute',now()-interval '1 hour') RETURNING id`, agent, rt, f.task, f.chat, f.status)
		exec(`INSERT INTO agent_run_usage(run_id,provider,model,input_tokens,output_tokens,cache_read_tokens,created_at,updated_at) VALUES($1,'claude','fixture-model',$2,10,20,now()-interval '1 hour',now()-interval '1 hour')`, run, f.tokens)
	}
	q := db.New(tx)
	since := pgtype.Timestamptz{Time: time.Now().Add(-24 * time.Hour), Valid: true}
	// Simulate the original production failure: the epoch watermark and an empty
	// derived rollup, even though each run already has raw token usage.
	exec(`UPDATE agent_run_usage_hourly_rollup_state SET watermark_at='1970-01-01 00:00:00+00' WHERE id=1`)
	for _, f := range []struct {
		kind                    string
		project                 pgtype.UUID
		tokens                  int64
		runs, chat, task, other int32
	}{{"", pgtype.UUID{}, 1000, 4, 2, 1, 1}, {"chat", pgtype.UUID{}, 500, 2, 2, 0, 0}, {"task", pgtype.UUID{}, 100, 1, 0, 1, 0}, {"other", pgtype.UUID{}, 400, 1, 0, 0, 1}, {"", project, 300, 2, 1, 1, 0}, {"chat", project, 200, 1, 1, 0, 0}} {
		daily, e := q.ListDashboardUsageDaily(ctx, db.ListDashboardUsageDailyParams{WorkspaceID: ws, Since: since, Tz: "UTC", ProjectID: f.project, RunKind: f.kind})
		if e != nil || len(daily) != 1 || daily[0].InputTokens != f.tokens || daily[0].RunCount != f.runs {
			t.Fatalf("daily %s: %+v %v", f.kind, daily, e)
		}
		byAgent, e := q.ListDashboardUsageByAgent(ctx, db.ListDashboardUsageByAgentParams{WorkspaceID: ws, Since: since, ProjectID: f.project, RunKind: f.kind})
		if e != nil || len(byAgent) != 1 || byAgent[0].InputTokens != f.tokens || byAgent[0].RunCount != f.runs {
			t.Fatalf("agents %s: %+v %v", f.kind, byAgent, e)
		}
		times, e := q.ListDashboardRunTimeDaily(ctx, db.ListDashboardRunTimeDailyParams{WorkspaceID: ws, Since: since, Tz: "UTC", ProjectID: f.project, RunKind: f.kind})
		if e != nil || len(times) != 1 || times[0].RunCount != f.runs || times[0].ChatRunCount != f.chat || times[0].TaskRunCount != f.task || times[0].OtherRunCount != f.other {
			t.Fatalf("time %s: %+v %v", f.kind, times, e)
		}
		byTime, e := q.ListDashboardAgentRunTime(ctx, db.ListDashboardAgentRunTimeParams{WorkspaceID: ws, Since: since, ProjectID: f.project, RunKind: f.kind})
		if e != nil || len(byTime) != 1 || byTime[0].RunCount != f.runs {
			t.Fatalf("agent time %s: %+v %v", f.kind, byTime, e)
		}
		failures, e := q.ListDashboardFailuresDaily(ctx, db.ListDashboardFailuresDailyParams{WorkspaceID: ws, Since: since, Tz: "UTC", ProjectID: f.project, RunKind: f.kind})
		if e != nil {
			t.Fatal(e)
		}
		var n int32
		for _, r := range failures {
			n += r.RunCount
		}
		if n != f.runs {
			t.Fatal(f.kind, n)
		}
	}
	runtime, e := q.ListRuntimeUsage(ctx, db.ListRuntimeUsageParams{RuntimeID: rt, Since: since, Tz: "UTC"})
	if e != nil || len(runtime) != 1 || runtime[0].InputTokens != 1000 {
		t.Fatal(runtime, e)
	}
	var count int64
	if e := tx.QueryRow(ctx, `SELECT rollup_agent_run_usage_hourly()`).Scan(&count); e != nil || count == 0 {
		t.Fatal("rollup did not catch up", count, e)
	}
	var sum int64
	if e := tx.QueryRow(ctx, `SELECT COALESCE(sum(input_tokens),0)::bigint FROM agent_run_usage_hourly WHERE workspace_id=$1`, ws).Scan(&sum); e != nil || sum != 1000 {
		t.Fatal("rollup mismatch", sum, e)
	}
	var watermark time.Time
	if e := tx.QueryRow(ctx, `SELECT watermark_at FROM agent_run_usage_hourly_rollup_state WHERE id=1`).Scan(&watermark); e != nil || watermark.Before(time.Now().Add(-2*time.Hour)) {
		t.Fatal("watermark stuck", watermark, e)
	}
	// Replaying a tick must not double-count tokens.
	if e := tx.QueryRow(ctx, `SELECT rollup_agent_run_usage_hourly()`).Scan(&count); e != nil {
		t.Fatal(e)
	}
	if e := tx.QueryRow(ctx, `SELECT COALESCE(sum(input_tokens),0)::bigint FROM agent_run_usage_hourly WHERE workspace_id=$1`, ws).Scan(&sum); e != nil || sum != 1000 {
		t.Fatal("replay changed usage", sum, e)
	}
}
