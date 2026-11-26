package daemon

import (
	"github.com/auenger/MEPERECO/core/internal/branding"
	"github.com/auenger/MEPERECO/core/pkg/agent"
	"strings"
	"sync"
	"time"
)

// shellResolveTTL bounds how long one login-shell PATH resolution is reused
// across probeAgentCLIs calls.
//
// This is deliberately much longer than agentDiscoveryInterval so the frequent
// discovery round stays a pure exec.LookPath sweep: resolveAgentsViaLoginShell
// forks the user's login shell and runs their rc files, and there is almost
// always at least one uninstalled provider to miss LookPath on, so a short TTL
// would turn discovery into a shell fork every few minutes for the life of the
// daemon.
//
// The practical effect: a CLI on the daemon's own PATH is discovered within
// agentDiscoveryInterval, while one reachable only through the login shell
// (nvm/fnm shims, a ~/.local/bin that only ~/.zshrc adds) takes up to this long
// — still without a restart, which is the part that was previously impossible.
var shellResolveTTL = 30 * time.Minute

var (
	shellResolveMu    sync.Mutex
	shellResolveCache map[string]string
	shellResolveKey   string
	shellResolvedAt   time.Time
)

// shellResolveEnvKey fingerprints the environment that determines what a login
// shell resolves. A change to any of these invalidates the cache immediately,
// independent of the TTL — the cached answer was for a different environment.
func shellResolveEnvKey() string {
	return strings.Join([]string{
		branding.Getenv("PATH"),
		branding.Getenv("SHELL"),
		branding.Getenv("HOME"),
	}, "\x00")
}

// cachedShellResolvedAgents resolves every standard agent command name through
// the user's login shell, reusing the previous result for shellResolveTTL as
// long as the resolution-relevant environment is unchanged.
//
// resolveAgentsViaLoginShell forks the user's login shell, which runs their rc
// files, so this must stay a cache and not a per-probe call: probeAgentCLIs now
// runs periodically on a live daemon, and there is almost always at least one
// uninstalled provider to miss LookPath on.
func cachedShellResolvedAgents() map[string]string {
	shellResolveMu.Lock()
	defer shellResolveMu.Unlock()
	key := shellResolveEnvKey()
	if shellResolveCache != nil && shellResolveKey == key && time.Since(shellResolvedAt) < shellResolveTTL {
		return shellResolveCache
	}
	resolved := resolveAgentsViaLoginShell(defaultAgentCommandNames)
	if resolved == nil {
		// Distinguish "resolved nothing" from "never resolved" so a failing
		// shell doesn't get re-forked on every probe inside the TTL window.
		resolved = map[string]string{}
	}
	shellResolveCache = resolved
	shellResolveKey = key
	shellResolvedAt = time.Now()
	return shellResolveCache
}

// probeAgentCLIs discovers which built-in agent CLIs are installed on this
// machine and returns one AgentEntry per provider that resolved.
//
// This is pure discovery: no version detection and no minimum-version gate
// (detectBuiltinRuntimes owns those, per registration round). The result is
// therefore the machine's *availability* set, which is exactly what
// /health.agents reports and what `silieco daemon probe-runtimes` prints.
//
// It is called once from LoadConfig at startup and again from the periodic
// workspace sync (refreshAgentAvailability), so a CLI the user installs while
// the daemon is already running gets picked up without a restart (SILI-5439).
// Everything it reads is process-external (PATH, SILIECO_*_PATH, SILIECO_*_MODEL),
// so re-running it is the only way to observe such an install.
//
// A var so tests can stub discovery without installing real CLIs.
var probeAgentCLIs = func() map[string]AgentEntry {
	// Probe available agent CLIs. exec.LookPath is the primary path, but on
	// macOS/Linux a GUI-launched daemon (Electron, Launchpad) does not
	// inherit the user's interactive shell PATH — fnm/nvm/volta multishells,
	// the Anthropic native installer prefix, and per-user npm prefixes all
	// live in dirs that only get added to PATH by ~/.zshrc or ~/.bashrc.
	// shellResolvedAgents asks the user's login shell, lazily on first miss,
	// to resolve every standard agent name to its canonical absolute path,
	// so we can find binaries the bare daemon process can't see. See
	// resolveAgentsViaLoginShell for the details and constraints.
	//
	// Laziness matters: the happy path (every agent on the daemon's PATH or
	// pinned to an explicit SILIECO_*_PATH) must not pay the cost of
	// spawning the user's login shell — that touches their rc files and
	// adds startup latency that scales with whatever they put in there. We
	// only fork a shell when a bare command name actually missed LookPath.
	//
	// The resolution is cached process-wide with a TTL (not per call) because
	// this function now also runs periodically on a live daemon: a per-call
	// sync.Once would fork a login shell on every discovery round, since there
	// is almost always at least one uninstalled provider to miss on. The TTL
	// still lets a CLI installed into a login-shell-only PATH dir (nvm, fnm,
	// ~/.local/bin via ~/.zshrc) be discovered without a restart (SILI-5439).
	getShellResolved := cachedShellResolvedAgents
	probe := func(descriptor agent.RuntimeDescriptor) (AgentEntry, bool) {
		envVar, defaultCmd, modelEnv := descriptor.PathEnv, descriptor.Command, descriptor.ModelEnv
		cmd := envOrDefault(envVar, defaultCmd)
		if path, err := resolveAgentExecutablePath(cmd); err == nil {
			return AgentEntry{
				Path:    path,
				Command: cmd,
				Model:   strings.TrimSpace(branding.Getenv(modelEnv)),
			}, true
		}
		// The shell fallback only rescues bare command names. An operator
		// who pinned SILIECO_*_PATH to an absolute or relative path that
		// doesn't exist should hard-miss, not silently get a different
		// binary.
		if strings.ContainsAny(cmd, "/\\") {
			return AgentEntry{}, false
		}
		if path, ok := getShellResolved()[cmd]; ok {
			return AgentEntry{
				Path:    path,
				Command: cmd,
				Model:   strings.TrimSpace(branding.Getenv(modelEnv)),
			}, true
		}
		if cmd == defaultCmd {
			if fallback := desktopRuntimeProbes[descriptor.DesktopBundleProbe]; fallback != nil {
				return fallback(cmd, modelEnv)
			}
		}
		return AgentEntry{}, false
	}

	agents := map[string]AgentEntry{}
	for _, descriptor := range agent.RuntimeDescriptors() {
		if descriptor.DirectProbeOnly {
			cmd := envOrDefault(descriptor.PathEnv, descriptor.Command)
			if path, err := resolveAgentExecutablePath(cmd); err == nil {
				agents[descriptor.ID] = AgentEntry{Path: path, Command: cmd, Model: strings.TrimSpace(branding.Getenv(descriptor.ModelEnv))}
			}
			continue
		}
		if e, ok := probe(descriptor); ok {
			agents[descriptor.ID] = e
		}
	}

	return agents
}
