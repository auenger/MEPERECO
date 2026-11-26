package daemon

import (
	"fmt"
	"log/slog"

	"github.com/auenger/MEPERECO/core/internal/daemon/execenv"
	"github.com/auenger/MEPERECO/core/pkg/agent"
	"github.com/auenger/MEPERECO/core/pkg/runfailure"
)

// normalizeRunResult maps provider results to the shared task lifecycle while preserving continuity evidence.
func (d *Daemon) normalizeRunResult(provider string, result agent.Result, env *execenv.Environment, usageEntries []AgentRunUsageEntry, runLog *slog.Logger) (RunResult, error) {
	switch result.Status {
	case "completed":
		if result.Output == "" {
			// The agent completed successfully but produced no text output.
			// This is valid — the agent may have done all its work via tool
			// calls (e.g. posting comments via CLI, pushing code). Treat as
			// a normal completion so the task is not incorrectly marked as
			// blocked.
			return RunResult{
				Status:    "completed",
				Comment:   "",
				SessionID: result.SessionID,
				WorkDir:   env.WorkDir,
				EnvRoot:   env.RootDir,
				Usage:     usageEntries,
			}, nil
		}
		// Detect "poisoned" terminal output: the agent didn't reach a real
		// conclusion but emitted a known fallback marker (iteration limit,
		// fallback meta message). Route through the blocked path with a
		// specific failure_reason so the server can exclude this session
		// from the (agent_id, task_id) resume lookup — otherwise a manual
		// rerun would inherit the same poisoned session and reproduce the
		// same bad output.
		if reason, ok := classifyPoisonedOutput(result.Output); ok {
			runLog.Warn("agent finished with poisoned fallback output, classifying as blocked",
				"failure_reason", reason,
			)
			return RunResult{
				Status:        "blocked",
				Comment:       result.Output,
				SessionID:     result.SessionID,
				WorkDir:       env.WorkDir,
				EnvRoot:       env.RootDir,
				Usage:         usageEntries,
				FailureReason: reason,
			}, nil
		}
		return RunResult{
			Status:    "completed",
			Comment:   result.Output,
			SessionID: result.SessionID,
			WorkDir:   env.WorkDir,
			EnvRoot:   env.RootDir,
			Usage:     usageEntries,
		}, nil
	case "timeout":
		// Surface session_id/work_dir so the chat resume pointer is kept
		// in sync even when the agent times out after building a session.
		// We mark as "blocked" (not a hard error return) so handleTask
		// goes through the FailTask path that forwards session info.
		comment := result.Error
		if comment == "" {
			comment = fmt.Sprintf("%s timed out after %s", provider, d.cfg.AgentTimeout)
		}
		failureReason := "timeout"
		if reason, ok := classifyResumeUnsafeTimeout(provider, comment); ok {
			runLog.Warn("agent timed out with resume-unsafe session, classifying as blocked",
				"failure_reason", reason,
			)
			failureReason = reason
		}
		return RunResult{
			Status:        "blocked",
			Comment:       comment,
			SessionID:     result.SessionID,
			WorkDir:       env.WorkDir,
			EnvRoot:       env.RootDir,
			FailureReason: failureReason,
			Usage:         usageEntries,
		}, nil
	case "idle_watchdog":
		// The idle watchdog force-stopped the run because the backend
		// went silent (e.g. claude blocked on a tool call against a
		// frozen child process). Route through the blocked path with a
		// dedicated failure_reason so the run leaves "running" state and
		// operators can tell idle-stop apart from a real timeout.
		comment := result.Error
		if comment == "" {
			comment = idleWatchdogReason(d.cfg.AgentIdleWatchdog)
		}
		return RunResult{
			Status:        "blocked",
			Comment:       comment,
			SessionID:     result.SessionID,
			WorkDir:       env.WorkDir,
			EnvRoot:       env.RootDir,
			FailureReason: "idle_watchdog",
			Usage:         usageEntries,
		}, nil
	case "cancelled":
		// Server cancelled the task (e.g. task reassignment, user cancel).
		// handleTask's cancelledByPoll branch already discards this result,
		// so this case is mainly defensive — and preserves the "cancelled"
		// status string for the "agent finished" log line so operators can
		// distinguish "task cancelled by server" from a real timeout.
		return RunResult{
			Status:    "cancelled",
			Comment:   "task cancelled by server",
			SessionID: result.SessionID,
			WorkDir:   env.WorkDir,
			EnvRoot:   env.RootDir,
			Usage:     usageEntries,
		}, nil
	default:
		errMsg := result.Error
		if errMsg == "" {
			errMsg = fmt.Sprintf("%s execution %s", provider, result.Status)
		}
		// Forward SessionID/WorkDir on the blocked path: backends commonly
		// emit a real session_id before failing (rate-limit, tool error,
		// model reject, …). Without this the chat_session resume pointer
		// would either be left stale or overwritten with NULL on the
		// server, causing the next chat turn to lose context.
		//
		// Classify upstream API 400 invalid_request_error failures with a
		// dedicated failure_reason so GetLastTaskSession excludes the
		// task from the (agent_id, task_id) resume lookup. Without this
		// classifier a corrupt image or oversized payload baked into the
		// conversation permanently blocks the task: every follow-up
		// task resumes the same poisoned session and hits the same 400.
		failureReason, _ := classifyPoisonedError(errMsg)
		if failureReason != "" {
			runLog.Warn("agent failed with poisoned API error, classifying as blocked",
				"failure_reason", failureReason,
			)
		} else {
			// SILI-2946: classifyPoisonedError only matches the
			// session-poisoning Anthropic 400 shape. Everything else
			// falls through to taskfailure.Classify, which maps the
			// raw error string to one of the 14 agent_error.*
			// sub-reasons (provider auth, capacity, context overflow,
			// runner crash, …) or to ReasonAgentUnknown. This keeps
			// the failure_reason column in the canonical refined
			// taxonomy at write time instead of waiting on the
			// SILI-1949 offline backfill to re-classify after the
			// fact.
			failureReason = runfailure.Classify(errMsg).String()
		}
		return RunResult{
			Status:        "blocked",
			Comment:       errMsg,
			SessionID:     result.SessionID,
			WorkDir:       env.WorkDir,
			EnvRoot:       env.RootDir,
			Usage:         usageEntries,
			FailureReason: failureReason,
		}, nil
	}
}
