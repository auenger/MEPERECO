package daemon

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"runtime"
	"strconv"

	"github.com/auenger/MEPERECO/core/internal/branding"
	"github.com/auenger/MEPERECO/core/internal/daemon/execenv"
)

// buildRunLaunchEnv owns the common task credential and process environment
// boundary. Runtime hooks may configure their own shell but cannot replace
// the task identity or manage daemon-wide resources.
func (d *Daemon) buildRunLaunchEnv(agentRun AgentRun, provider, agentName string, slot int, runTempDir string, env *execenv.Environment, lifecycle runtimeLifecyclePolicy, runLog *slog.Logger) (map[string]string, string, error) {
	// Pass task-scoped auth credentials and context so the spawned agent CLI
	// can call the MEPERStudio API and the local daemon (e.g. `silieco repo checkout`).
	// SILIECO_TASK_SLOT is allocated from the daemon-wide concurrency pool, not
	// per-agent. When one daemon hosts multiple agents, slots index shared
	// daemon-level resources such as GPUs.
	// SILIECO_TOKEN is bound to (agent, task) by the server. Never fall back
	// to the daemon's own credential here: doing so lets agent CLI writes land
	// as the runtime owner's member actor and can retrigger the same agent.
	agentToken, err := runScopedAuthToken(agentRun)
	if err != nil {
		runLog.Error("task auth token invalid; refusing to start agent", "error", err)
		return nil, "", err
	}
	agentEnv := map[string]string{
		"SILIECO_TOKEN":        agentToken,
		"SILIECO_SERVER_URL":   d.cfg.ServerBaseURL,
		"SILIECO_DAEMON_PORT":  fmt.Sprintf("%d", d.cfg.HealthPort),
		"SILIECO_WORKSPACE_ID": agentRun.WorkspaceID,
		"SILIECO_AGENT_NAME":   agentName,
		"SILIECO_AGENT_ID":     agentRun.AgentID,
		"SILIECO_RUN_ID":       agentRun.ID,
		"SILIECO_TASK_SLOT":    strconv.Itoa(slot),
		"TMPDIR":               runTempDir,
		"TMP":                  runTempDir,
		"TEMP":                 runTempDir,
	}
	if checkoutMode := repoCheckoutModeFor(provider, runtime.GOOS); checkoutMode != "" {
		agentEnv[repoCheckoutModeEnv] = checkoutMode
	}
	if agentRun.AutopilotRunID != "" {
		agentEnv["SILIECO_AUTOPILOT_RUN_ID"] = agentRun.AutopilotRunID
	}
	if agentRun.AutopilotID != "" {
		agentEnv["SILIECO_AUTOPILOT_ID"] = agentRun.AutopilotID
	}
	// Quick-create marker — when set, the silieco CLI's `task create`
	// command stamps the new task with origin_type=quick_create +
	// origin_id=<run_id> so the completion handler can find it
	// deterministically (see GetTaskByOrigin).
	if agentRun.QuickCreatePrompt != "" {
		agentEnv["SILIECO_QUICK_CREATE_TASK_ID"] = agentRun.ID
		if len(agentRun.QuickCreateAttachmentIDs) > 0 {
			if raw, err := json.Marshal(agentRun.QuickCreateAttachmentIDs); err == nil {
				agentEnv["SILIECO_QUICK_CREATE_ATTACHMENT_IDS"] = string(raw)
			} else {
				runLog.Warn("quick-create attachment ids: marshal failed; skipping env injection", "error", err)
			}
		}
	}
	branding.AddBrandedRunEnv(agentEnv)
	// Provide the branded executable and preserve the installed legacy command.
	selfBin, err := resolveSelfExecutable()
	if err != nil {
		return nil, "", fmt.Errorf("resolve agent CLI: %w", err)
	}
	agentEnv["PATH"], err = agentCLIPath(selfBin, runTempDir)
	if err != nil {
		return nil, "", fmt.Errorf("prepare MEPERStudio agent CLI: %w", err)
	}
	// Point Codex to the per-task CODEX_HOME so it discovers skills natively
	// without polluting the system ~/.codex/skills/.
	if env.CodexHome != "" {
		agentEnv["CODEX_HOME"] = env.CodexHome
	}
	// Redirect HOME/XDG/npm_config_cache to the per-task writable home under the
	// Linux codex workspace-write sandbox, where the real home is read-only. This
	// lets tools that write to `~` (npm, Prisma, …) succeed without per-tool env
	// tweaks. Set before custom_env below so a user override still wins for the
	// non-blocklisted XDG keys; HOME itself stays blocklisted. Empty TaskHome
	// (macOS/Windows, non-sandboxed providers) leaves the real HOME untouched
	// (SILI-4856).
	if env.RunHome != "" {
		for k, v := range execenv.RunHomeEnv(env.RunHome) {
			agentEnv[k] = v
		}
	}
	// (Hermes HERMES_HOME is applied after custom_env below so the per-task
	// overlay can win over a user-set HERMES_HOME; see
	// layerCustomEnvAndHermesHome.)
	// Point Cursor at per-task project state when managed MCP is present.
	// The workdir .cursor/mcp.json carries the managed server list, while
	// CURSOR_DATA_DIR isolates the matching project approvals from the user's
	// persistent ~/.cursor/projects state.
	if env.CursorDataDir != "" {
		agentEnv["CURSOR_DATA_DIR"] = env.CursorDataDir
	}
	// Point OpenClaw at the per-task synthesized config. The config pins
	// agents.defaults.workspace (and any agents.list[].workspace) to the
	// task workdir, so the CLI's native skill scanner picks up the per-task
	// skills written under {workDir}/skills/. Falls back silently when the
	// preparer didn't run (non-openclaw provider, or write failure).
	if env.OpenclawConfigPath != "" {
		agentEnv["OPENCLAW_CONFIG_PATH"] = env.OpenclawConfigPath
	}
	// Grant the wrapper config permission to $include the user's active
	// config across directories. OpenClaw's $include defaults to confining
	// resolution to the wrapper's own directory; without this, the
	// wrapper-out-of-envRoot $include into ~/.openclaw/openclaw.json is
	// rejected and the run boots with no user-registered agents.
	if rootsValue, ok := composeOpenclawIncludeRoots(env.OpenclawIncludeRoot, os.Getenv("OPENCLAW_INCLUDE_ROOTS")); ok {
		agentEnv["OPENCLAW_INCLUDE_ROOTS"] = rootsValue
	}
	// Inject user-configured custom environment variables (e.g. ANTHROPIC_API_KEY,
	// ANTHROPIC_BASE_URL for router/proxy mode, or CLAUDE_CODE_USE_BEDROCK for
	// Bedrock). These are set per-agent via the agent settings UI.
	// Critical internal variables are blocklisted to prevent accidental or
	// malicious override of daemon-set values.
	var agentCustomEnv map[string]string
	if agentRun.Agent != nil {
		agentCustomEnv = agentRun.Agent.CustomEnv
	}
	layerCustomEnvAndHermesHome(agentEnv, agentCustomEnv, env.HermesHome, d.logger)
	if err := lifecycle.configureRunShell(env.CodexHome, os.Environ(), agentEnv, agentCustomEnv, d.logger); err != nil {
		return nil, "", err
	}
	return agentEnv, agentToken, nil
}
