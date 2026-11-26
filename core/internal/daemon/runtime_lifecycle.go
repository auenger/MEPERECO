package daemon

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/auenger/MEPERECO/core/internal/daemon/execenv"
	"github.com/auenger/MEPERECO/core/pkg/agent"
)

// Runtime preparation data is scoped to one run. The orchestrator owns the
// lifetime of the run and all acquired resources; provider hooks only supply
// their own configuration and cleanup callbacks.
type runtimePreparation struct {
	codexVersion, openclawBin, cursorMcpAuthSource, openclawMode, hermesSourceHome string
	openclawGateway                                                                execenv.OpenclawGatewayPin
	effectiveMcpConfig                                                             json.RawMessage
	hermesSourceMustExist                                                          bool
	hermesEnv                                                                      map[string]string
	codexSandboxArgs                                                               []string
}
type runtimePrepareInput struct {
	d               *Daemon
	run             AgentRun
	entry           AgentEntry
	resolvedVersion string
	fixedArgs       []string
	log             *slog.Logger
}
type runtimeLifecyclePolicy struct {
	prepare        func(runtimePrepareInput, *runtimePreparation) error
	acquire        func(*Daemon, AgentRun) func()
	afterReuse     func(*AgentRun, *execenv.RunContextForEnv, *execenv.Environment, *slog.Logger)
	customArgs     func([]string, *execenv.Environment) []string
	idleWatchdog   func(Config) time.Duration
	defaults       func(Config) []string
	configureShell func(string, []string, map[string]string, map[string]string, *slog.Logger) error
}

var runtimeLifecyclePolicies = map[string]runtimeLifecyclePolicy{
	"claude":    {defaults: func(c Config) []string { return c.ClaudeArgs }},
	"codebuddy": {defaults: func(c Config) []string { return c.CodebuddyArgs }},
	"qwen":      {defaults: func(c Config) []string { return c.QwenArgs }},
	"codex": {prepare: prepareCodexRuntime, acquire: acquireCodexStore, afterReuse: resumeCodexRuntime, defaults: func(c Config) []string { return c.CodexArgs }, configureShell: func(home string, inherited []string, env, custom map[string]string, log *slog.Logger) error {
		return configureCodexRunShellEnvironment("codex", home, inherited, env, custom, log)
	}},
	"cursor": {prepare: func(i runtimePrepareInput, p *runtimePreparation) error {
		if i.run.Agent != nil {
			p.cursorMcpAuthSource = strings.TrimSpace(i.run.Agent.CustomEnv[execenv.CursorMcpAuthSourceEnv])
		}
		return nil
	}},
	"openclaw": {prepare: func(i runtimePrepareInput, p *runtimePreparation) error {
		p.openclawBin = i.entry.Path
		if i.run.Agent != nil {
			p.openclawMode, p.openclawGateway = decodeOpenclawRuntimeConfig(i.run.Agent.RuntimeConfig, i.d.logger)
		}
		return nil
	}},
	"hermes": {prepare: prepareHermesRuntime, customArgs: func(args []string, env *execenv.Environment) []string {
		return hermesLaunchArgs(args, env != nil && env.HermesHome != "")
	}},
	"opencode": {idleWatchdog: func(c Config) time.Duration { return c.OpenCodeIdleWatchdog }},
}

func runtimeLifecycleFor(provider string) runtimeLifecyclePolicy {
	return runtimeLifecyclePolicies[provider]
}
func (p runtimeLifecyclePolicy) prepareRun(provider string, i runtimePrepareInput) (runtimePreparation, error) {
	// Preserve the cached Codex version for all providers, as in the old generic
	// launch config. Only the Codex adapter consumes this field.
	out := runtimePreparation{codexVersion: i.d.agentVersion("codex")}
	if i.run.Agent != nil {
		out.effectiveMcpConfig = i.run.Agent.McpConfig
		if merged, err := mergeRuntimeAndAgentMcpConfig(provider, out.effectiveMcpConfig); err != nil {
			i.log.Warn("mcp_config: runtime merge failed; using agent configuration only", "provider", provider, "error", err)
		} else {
			out.effectiveMcpConfig = merged
		}
	}
	if p.prepare != nil {
		if err := p.prepare(i, &out); err != nil {
			return out, err
		}
	}
	return out, nil
}
func prepareCodexRuntime(i runtimePrepareInput, p *runtimePreparation) error {
	if i.resolvedVersion != "" {
		p.codexVersion = i.resolvedVersion
	}
	var args []string
	if i.run.Agent != nil {
		args = i.run.Agent.CustomArgs
	}
	extra := append(agent.FilterLaunchPrefix("codex", i.fixedArgs, i.d.logger), i.d.cfg.CodexArgs...)
	p.codexSandboxArgs = agent.NormalizeCodexLaunchArgs(extra, args, p.effectiveMcpConfig, i.d.logger)
	return nil
}
func acquireCodexStore(d *Daemon, run AgentRun) func() {
	if store := execenv.CodexSessionStorePath(d.cfg.Profile, run.AgentID, run.TaskID); store != "" {
		d.markActiveCodexStore(store)
		return func() { d.unmarkActiveCodexStore(store) }
	}
	return func() {}
}
func resumeCodexRuntime(run *AgentRun, ctx *execenv.RunContextForEnv, env *execenv.Environment, log *slog.Logger) {
	gateCodexResumeToRolloutPresence(run, ctx, "codex", env.CodexHome, log)
}
func prepareHermesRuntime(i runtimePrepareInput, p *runtimePreparation) error {
	var args []string
	var overrides map[string]string
	if i.run.Agent != nil {
		args = i.run.Agent.CustomArgs
		overrides = i.run.Agent.CustomEnv
	}
	sel := agent.ParseHermesProfileArgs(args)
	res := execenv.ResolveHermesProfile(overrides["HERMES_HOME"], sel.Name, sel.Found, sel.Inline)
	if res.Err != nil {
		return fmt.Errorf("resolve hermes profile: %w", res.Err)
	}
	p.hermesSourceHome, p.hermesSourceMustExist = res.SourceHome, res.MustExist
	p.hermesEnv = sanitizeAgentEnv(overrides)
	if p.hermesEnv == nil {
		p.hermesEnv = map[string]string{}
	}
	p.hermesEnv["HERMES_HOME"] = res.SourceHome
	return nil
}
func (p runtimeLifecyclePolicy) acquireRun(d *Daemon, run AgentRun) func() {
	if p.acquire != nil {
		return p.acquire(d, run)
	}
	return func() {}
}
func (p runtimeLifecyclePolicy) gateResume(run *AgentRun, ctx *execenv.RunContextForEnv, env *execenv.Environment, log *slog.Logger) {
	if p.afterReuse != nil {
		p.afterReuse(run, ctx, env, log)
	}
}
func (p runtimeLifecyclePolicy) launchArgs(args []string, env *execenv.Environment) []string {
	if p.customArgs != nil {
		return p.customArgs(args, env)
	}
	return args
}
func (p runtimeLifecyclePolicy) watchdog(cfg Config) time.Duration {
	if p.idleWatchdog != nil {
		return p.idleWatchdog(cfg)
	}
	return 0
}
func (p runtimeLifecyclePolicy) configureRunShell(home string, inherited []string, env, custom map[string]string, log *slog.Logger) error {
	if p.configureShell == nil {
		return nil
	}
	return p.configureShell(home, inherited, env, custom, log)
}
