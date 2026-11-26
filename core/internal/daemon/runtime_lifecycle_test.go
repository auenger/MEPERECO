package daemon

import (
	"log/slog"
	"testing"
	"time"

	"github.com/auenger/MEPERECO/core/internal/daemon/execenv"
	"github.com/auenger/MEPERECO/core/pkg/agent"
)

func TestRuntimeLifecyclePolicies(t *testing.T) {
	cfg := Config{OpenCodeIdleWatchdog: 3 * time.Second, CodexArgs: []string{"--sandbox", "workspace-write"}}
	if runtimeLifecycleFor("opencode").watchdog(cfg) != cfg.OpenCodeIdleWatchdog {
		t.Fatal("OpenCode watchdog lost")
	}
	if runtimeLifecycleFor("claude").watchdog(cfg) != 0 {
		t.Fatal("unrelated watchdog")
	}
	if args := runtimeLifecycleFor("codex").defaults(cfg); len(args) != 2 {
		t.Fatal(args)
	}
	p := runtimeLifecycleFor("new-runtime")
	if p.watchdog(cfg) != 0 || p.launchArgs([]string{"test"}, nil)[0] != "test" {
		t.Fatal("generic adapter required special case")
	}
	p.acquireRun(nil, AgentRun{})()
	if err := p.configureRunShell("", nil, nil, nil, slog.Default()); err != nil {
		t.Fatal(err)
	}
}

func TestNormalizeRunResult(t *testing.T) {
	d := &Daemon{}
	env := &execenv.Environment{WorkDir: "/work", RootDir: "/run"}
	for _, tc := range []struct{ input, want string }{{"completed", "completed"}, {"timeout", "blocked"}, {"idle_watchdog", "blocked"}, {"failed", "blocked"}, {"cancelled", "cancelled"}} {
		t.Run(tc.input, func(t *testing.T) {
			result, err := d.normalizeRunResult("generic", agent.Result{Status: tc.input, SessionID: "session"}, env, nil, slog.Default())
			if err != nil || result.Status != tc.want || result.SessionID != "session" || result.WorkDir != env.WorkDir || result.EnvRoot != env.RootDir {
				t.Fatalf("%+v %v", result, err)
			}
			if tc.input == "idle_watchdog" && result.FailureReason != "idle_watchdog" {
				t.Fatal(result)
			}
		})
	}
}
