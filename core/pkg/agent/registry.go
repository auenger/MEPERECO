package agent

import (
	"context"
	"fmt"
	"log/slog"
	"path/filepath"
	"strings"
)

// RuntimeCapabilities are stable adapter capabilities, not dynamic availability.
type RuntimeCapabilities struct {
	ModelSelection              bool `json:"model_selection"`
	InlineSystemPrompt          bool `json:"inline_system_prompt"`
	ResumeRejectionUndetectable bool `json:"resume_rejection_undetectable"`
	ProjectSkills               bool `json:"project_skills"`
	LocalSkills                 bool `json:"local_skills"`
	NativeSkillDiscovery        bool `json:"native_skill_discovery"`
	DisableLocalSkills          bool `json:"disable_local_skills"`
}

// RuntimeDescriptor is the registration boundary for built-in adapters.
// Factory and protocol-specific implementation remain private to this package.
type RuntimeDescriptor struct {
	ID, DisplayName, Command, PathEnv, ModelEnv, LaunchHeader                              string
	ContextFile, ProjectSkillPath, LocalSkillPath, LocalSkillHomeEnv, LocalSkillHomeSuffix string
	DirectProbeOnly, PluginSkills                                                          bool
	DesktopBundleProbe                                                                     string
	HomeSkillMaterialization                                                               bool
	SkipProjectSkillDirectory                                                              bool
	Capabilities                                                                           RuntimeCapabilities
	factory                                                                                func(Config) Backend
	listModels                                                                             func(context.Context, string, string) ([]Model, error)
	blockedArgs                                                                            map[string]blockedArgMode
}

var runtimeDescriptors = []RuntimeDescriptor{
	{
		ID: "claude", DisplayName: "Claude",
		Command: "claude", PathEnv: "SILIECO_CLAUDE_PATH", ModelEnv: "SILIECO_CLAUDE_MODEL",
		LaunchHeader:     "claude (stream-json)",
		ContextFile:      "CLAUDE.md",
		ProjectSkillPath: ".claude/skills",
		LocalSkillPath:   ".claude/skills",
		PluginSkills:     true,
		Capabilities:     RuntimeCapabilities{ModelSelection: true, ProjectSkills: true, LocalSkills: true, NativeSkillDiscovery: true, DisableLocalSkills: true},
		factory:          func(cfg Config) Backend { return &claudeBackend{cfg: cfg} },
		blockedArgs:      claudeBlockedArgs,
		listModels: func(ctx context.Context, providerType, executablePath string) ([]Model, error) {
			models := claudeStaticModels()
			annotateClaudeThinking(ctx, models, executablePath)
			return models, nil
		}},
	{
		ID: "codebuddy", DisplayName: "Codebuddy",
		Command: "codebuddy", PathEnv: "SILIECO_CODEBUDDY_PATH", ModelEnv: "SILIECO_CODEBUDDY_MODEL",
		LaunchHeader:     "codebuddy (stream-json)",
		ContextFile:      "CODEBUDDY.md",
		ProjectSkillPath: ".codebuddy/skills",
		LocalSkillPath:   ".codebuddy/skills",
		Capabilities:     RuntimeCapabilities{ModelSelection: true, ProjectSkills: true, LocalSkills: true, NativeSkillDiscovery: true},
		factory:          func(cfg Config) Backend { return &codebuddyBackend{cfg: cfg} },
		blockedArgs:      codebuddyBlockedArgs,
		listModels: func(ctx context.Context, providerType, executablePath string) ([]Model, error) {
			return cachedDiscovery(providerType, func() ([]Model, error) {
				models, err := discoverCodebuddyModels(ctx, executablePath)
				if err != nil {
					return nil, err
				}
				annotateCodebuddyThinking(ctx, models, executablePath)
				return models, nil
			})
		}},
	{
		ID: "codex", DisplayName: "Codex",
		Command: "codex", PathEnv: "SILIECO_CODEX_PATH", ModelEnv: "SILIECO_CODEX_MODEL",
		LaunchHeader:      "codex app-server",
		ContextFile:       "AGENTS.md",
		ProjectSkillPath:  ".agent_context/skills",
		LocalSkillPath:    ".codex/skills",
		LocalSkillHomeEnv: "CODEX_HOME", LocalSkillHomeSuffix: "skills",
		DesktopBundleProbe:       "codex-app",
		HomeSkillMaterialization: true,
		Capabilities:             RuntimeCapabilities{ModelSelection: true, ProjectSkills: true, LocalSkills: true, NativeSkillDiscovery: true, DisableLocalSkills: true},
		factory:                  func(cfg Config) Backend { return &codexBackend{cfg: cfg} },
		blockedArgs:              codexBlockedArgs,
		listModels: func(ctx context.Context, providerType, executablePath string) ([]Model, error) {
			return cachedDiscovery(discoveryCacheKey(providerType, executablePath), func() ([]Model, error) {
				return discoverCodexModels(ctx, executablePath), nil
			})
		}},
	{
		ID: "copilot", DisplayName: "Copilot",
		Command: "copilot", PathEnv: "SILIECO_COPILOT_PATH", ModelEnv: "SILIECO_COPILOT_MODEL",
		LaunchHeader:     "copilot (json)",
		ContextFile:      "AGENTS.md",
		ProjectSkillPath: ".github/skills",
		LocalSkillPath:   ".copilot/skills",
		Capabilities:     RuntimeCapabilities{ModelSelection: true, ResumeRejectionUndetectable: true, ProjectSkills: true, LocalSkills: true, NativeSkillDiscovery: true},
		factory:          func(cfg Config) Backend { return &copilotBackend{cfg: cfg} },
		blockedArgs:      copilotBlockedArgs,
		listModels: func(ctx context.Context, providerType, executablePath string) ([]Model, error) {
			return cachedDiscovery(providerType, func() ([]Model, error) {
				return discoverCopilotModels(ctx, executablePath)
			})
		}},
	{
		ID: "opencode", DisplayName: "Opencode",
		Command: "opencode", PathEnv: "SILIECO_OPENCODE_PATH", ModelEnv: "SILIECO_OPENCODE_MODEL",
		LaunchHeader:     "opencode run (json)",
		ContextFile:      "AGENTS.md",
		ProjectSkillPath: ".opencode/skills",
		LocalSkillPath:   ".config/opencode/skills",
		Capabilities:     RuntimeCapabilities{ModelSelection: true, ResumeRejectionUndetectable: true, ProjectSkills: true, LocalSkills: true, NativeSkillDiscovery: true},
		factory:          func(cfg Config) Backend { return &opencodeBackend{cfg: cfg} },
		blockedArgs:      opencodeBlockedArgs,
		listModels: func(ctx context.Context, providerType, executablePath string) ([]Model, error) {
			return cachedDiscovery(discoveryCacheKey(providerType, executablePath), func() ([]Model, error) {
				return discoverOpenCodeModels(ctx, executablePath)
			})
		}},
	{
		ID: "deveco", DisplayName: "Deveco",
		Command: "deveco", PathEnv: "SILIECO_DEVECO_PATH", ModelEnv: "SILIECO_DEVECO_MODEL",
		LaunchHeader:     "deveco run (json)",
		ContextFile:      "AGENTS.md",
		ProjectSkillPath: ".deveco/skills",
		LocalSkillPath:   ".config/deveco/skills",
		Capabilities:     RuntimeCapabilities{ModelSelection: true, ResumeRejectionUndetectable: true, ProjectSkills: true, LocalSkills: true, NativeSkillDiscovery: true},
		factory:          func(cfg Config) Backend { return &devecoBackend{cfg: cfg} },
		blockedArgs:      devecoBlockedArgs,
		listModels: func(ctx context.Context, providerType, executablePath string) ([]Model, error) {
			return cachedDiscovery(discoveryCacheKey(providerType, executablePath), func() ([]Model, error) {
				return discoverDevecoModels(ctx, executablePath)
			})
		}},
	{
		ID: "openclaw", DisplayName: "Openclaw",
		Command: "openclaw", PathEnv: "SILIECO_OPENCLAW_PATH", ModelEnv: "SILIECO_OPENCLAW_MODEL",
		LaunchHeader:     "openclaw agent (json)",
		ContextFile:      "AGENTS.md",
		ProjectSkillPath: "skills",
		LocalSkillPath:   ".openclaw/skills",
		Capabilities:     RuntimeCapabilities{ModelSelection: true, InlineSystemPrompt: true, ProjectSkills: true, LocalSkills: true, NativeSkillDiscovery: true},
		factory:          func(cfg Config) Backend { return &openclawBackend{cfg: cfg} },
		blockedArgs:      openclawBlockedArgs,
		listModels: func(ctx context.Context, providerType, executablePath string) ([]Model, error) {
			return cachedDiscovery(providerType, func() ([]Model, error) {
				return discoverOpenclawAgents(ctx, executablePath)
			})
		}},
	{
		ID: "hermes", DisplayName: "Hermes",
		Command: "hermes", PathEnv: "SILIECO_HERMES_PATH", ModelEnv: "SILIECO_HERMES_MODEL",
		LaunchHeader:             "hermes acp",
		ContextFile:              "AGENTS.md",
		ProjectSkillPath:         ".agent_context/skills",
		LocalSkillPath:           ".hermes/skills",
		HomeSkillMaterialization: true, SkipProjectSkillDirectory: true,
		Capabilities: RuntimeCapabilities{ModelSelection: true, ProjectSkills: true, LocalSkills: true, NativeSkillDiscovery: true},
		factory:      func(cfg Config) Backend { return &hermesBackend{cfg: cfg} },
		blockedArgs:  hermesBlockedArgs,
		listModels: func(ctx context.Context, providerType, executablePath string) ([]Model, error) {
			return cachedDiscovery(providerType, func() ([]Model, error) {
				return discoverHermesModels(ctx, executablePath)
			})
		}},
	{
		ID: "pi", DisplayName: "Pi",
		Command: "pi", PathEnv: "SILIECO_PI_PATH", ModelEnv: "SILIECO_PI_MODEL",
		LaunchHeader:     "pi (json mode)",
		ContextFile:      "AGENTS.md",
		ProjectSkillPath: ".pi/skills",
		LocalSkillPath:   ".pi/agent/skills",
		Capabilities:     RuntimeCapabilities{ModelSelection: true, ProjectSkills: true, LocalSkills: true, NativeSkillDiscovery: true},
		factory:          func(cfg Config) Backend { return &piBackend{cfg: cfg} },
		blockedArgs:      piBlockedArgs,
		listModels: func(ctx context.Context, providerType, executablePath string) ([]Model, error) {
			return cachedDiscovery(providerType, func() ([]Model, error) {
				return discoverPiModels(ctx, executablePath)
			})
		}},
	{
		ID: "cursor", DisplayName: "Cursor",
		Command: "cursor-agent", PathEnv: "SILIECO_CURSOR_PATH", ModelEnv: "SILIECO_CURSOR_MODEL",
		LaunchHeader:     "cursor-agent (stream-json)",
		ContextFile:      "AGENTS.md",
		ProjectSkillPath: ".cursor/skills",
		LocalSkillPath:   ".cursor/skills",
		Capabilities:     RuntimeCapabilities{ModelSelection: true, ResumeRejectionUndetectable: true, ProjectSkills: true, LocalSkills: true, NativeSkillDiscovery: true},
		factory:          func(cfg Config) Backend { return &cursorBackend{cfg: cfg} },
		blockedArgs:      cursorBlockedArgs,
		listModels: func(ctx context.Context, providerType, executablePath string) ([]Model, error) {
			return cachedDiscovery(providerType, func() ([]Model, error) {
				return discoverCursorModels(ctx, executablePath)
			})
		}},
	{
		ID: "kimi", DisplayName: "Kimi",
		Command: "kimi", PathEnv: "SILIECO_KIMI_PATH", ModelEnv: "SILIECO_KIMI_MODEL",
		LaunchHeader:     "kimi acp",
		ContextFile:      "AGENTS.md",
		ProjectSkillPath: ".kimi/skills",
		LocalSkillPath:   ".kimi/skills",
		Capabilities:     RuntimeCapabilities{ModelSelection: true, InlineSystemPrompt: true, ProjectSkills: true, LocalSkills: true, NativeSkillDiscovery: true},
		factory:          func(cfg Config) Backend { return &kimiBackend{cfg: cfg} },
		blockedArgs:      kimiBlockedArgs,
		listModels: func(ctx context.Context, providerType, executablePath string) ([]Model, error) {
			return cachedDiscovery(providerType, func() ([]Model, error) {
				return discoverKimiModels(ctx, executablePath)
			})
		}},
	{
		ID: "kiro", DisplayName: "Kiro",
		Command: "kiro-cli", PathEnv: "SILIECO_KIRO_PATH", ModelEnv: "SILIECO_KIRO_MODEL",
		LaunchHeader:     "kiro-cli acp",
		ContextFile:      "AGENTS.md",
		ProjectSkillPath: ".kiro/skills",
		LocalSkillPath:   ".kiro/skills",
		Capabilities:     RuntimeCapabilities{ModelSelection: true, ProjectSkills: true, LocalSkills: true, NativeSkillDiscovery: true},
		factory:          func(cfg Config) Backend { return &kiroBackend{cfg: cfg} },
		blockedArgs:      kiroBlockedArgs,
		listModels: func(ctx context.Context, providerType, executablePath string) ([]Model, error) {
			return cachedDiscovery(providerType, func() ([]Model, error) {
				return discoverKiroModels(ctx, executablePath)
			})
		}},
	{
		ID: "antigravity", DisplayName: "Antigravity",
		Command: "agy", PathEnv: "SILIECO_ANTIGRAVITY_PATH", ModelEnv: "SILIECO_ANTIGRAVITY_MODEL",
		LaunchHeader:     "agy -p (non-interactive)",
		ContextFile:      "AGENTS.md",
		ProjectSkillPath: ".agents/skills",
		LocalSkillPath:   ".gemini/antigravity-cli/skills",
		Capabilities:     RuntimeCapabilities{ModelSelection: true, ResumeRejectionUndetectable: true, ProjectSkills: true, LocalSkills: true, NativeSkillDiscovery: true},
		factory:          func(cfg Config) Backend { return &antigravityBackend{cfg: cfg} },
		blockedArgs:      antigravityBlockedArgs,
		listModels: func(ctx context.Context, providerType, executablePath string) ([]Model, error) {
			// agy 1.0.6 added a `--model` flag plus an `agy models` catalog
			// command (SILI-3125). Enumerate it on demand like the other
			// dynamic-discovery backends.
			return cachedDiscovery(providerType, func() ([]Model, error) {
				return discoverAntigravityModels(ctx, executablePath)
			})
		}},
	{
		ID: "qoder", DisplayName: "Qoder",
		Command: "qodercli", PathEnv: "SILIECO_QODER_PATH", ModelEnv: "SILIECO_QODER_MODEL",
		LaunchHeader:     "qodercli --acp",
		ContextFile:      "AGENTS.md",
		ProjectSkillPath: ".qoder/skills",
		LocalSkillPath:   ".qoder/skills",
		DirectProbeOnly:  true,
		Capabilities:     RuntimeCapabilities{ModelSelection: true, ProjectSkills: true, LocalSkills: true, NativeSkillDiscovery: true},
		factory:          func(cfg Config) Backend { return &qoderBackend{cfg: cfg} },
		blockedArgs:      qoderBlockedArgs,
		listModels: func(ctx context.Context, providerType, executablePath string) ([]Model, error) {
			return cachedDiscovery(providerType, func() ([]Model, error) {
				return discoverQoderModels(ctx, executablePath)
			})
		}},
	{
		ID: "traecli", DisplayName: "Trae",
		Command: "traecli", PathEnv: "SILIECO_TRAECLI_PATH", ModelEnv: "SILIECO_TRAECLI_MODEL",
		LaunchHeader:     "traecli acp serve",
		ContextFile:      "AGENTS.md",
		ProjectSkillPath: ".traecli/skills",
		LocalSkillPath:   ".traecli/skills",
		Capabilities:     RuntimeCapabilities{ModelSelection: true, InlineSystemPrompt: true, ProjectSkills: true, LocalSkills: true},
		factory:          func(cfg Config) Backend { return &traecliBackend{cfg: cfg} },
		blockedArgs:      traecliBlockedArgs,
		listModels: func(ctx context.Context, providerType, executablePath string) ([]Model, error) {
			// Official TRAE CLI is ACP-native: it returns its model catalog from
			// session/new. Enumerate it on demand like the other ACP backends
			// (requires a logged-in traecli; falls back to manual entry on error).
			return cachedDiscovery(providerType, func() ([]Model, error) {
				return discoverTraecliModels(ctx, executablePath)
			})
		}},
	{
		ID: "grok", DisplayName: "Grok",
		Command: "grok", PathEnv: "SILIECO_GROK_PATH", ModelEnv: "SILIECO_GROK_MODEL",
		LaunchHeader:      "grok agent stdio",
		ContextFile:       "AGENTS.md",
		ProjectSkillPath:  ".grok/skills",
		LocalSkillPath:    ".grok/skills",
		LocalSkillHomeEnv: "GROK_HOME", LocalSkillHomeSuffix: "skills",
		Capabilities: RuntimeCapabilities{ModelSelection: true, ProjectSkills: true, LocalSkills: true},
		factory:      func(cfg Config) Backend { return &grokBackend{cfg: cfg} },
		blockedArgs:  grokBlockedArgs,
		listModels: func(ctx context.Context, providerType, executablePath string) ([]Model, error) {
			// xAI Grok Build is ACP-native (`grok agent stdio`); model catalog
			// comes from session/new. Falls back to a small static list so the
			// UI picker stays usable offline / unauthenticated.
			return cachedDiscovery(providerType, func() ([]Model, error) {
				return discoverGrokModels(ctx, executablePath)
			})
		}},
	{
		ID: "qwen", DisplayName: "Qwen Code",
		Command: "qwen", PathEnv: "SILIECO_QWEN_PATH", ModelEnv: "SILIECO_QWEN_MODEL",
		LaunchHeader:      "qwen -p (stream-json)",
		ContextFile:       "QWEN.md",
		ProjectSkillPath:  ".qwen/skills",
		LocalSkillPath:    ".qwen/skills",
		LocalSkillHomeEnv: "QWEN_HOME", LocalSkillHomeSuffix: "skills",
		Capabilities: RuntimeCapabilities{ModelSelection: true, ProjectSkills: true, LocalSkills: true, NativeSkillDiscovery: true},
		factory:      func(cfg Config) Backend { return &qwenBackend{cfg: cfg} },
		blockedArgs:  qwenBlockedArgs,
		listModels: func(ctx context.Context, providerType, executablePath string) ([]Model, error) {
			// Qwen Code has no account-independent headless model catalog. An
			// empty list keeps the runtime default and manual model entry available
			// without advertising a Token-Plan-specific model to other accounts.
			return []Model{}, nil
		}},
	{
		ID: "zcode", DisplayName: "ZCode",
		Command: "zcode", PathEnv: "SILIECO_ZCODE_PATH",
		LaunchHeader:      "zcode --prompt (stream-json)",
		ContextFile:       "AGENTS.md",
		ProjectSkillPath:  ".zcode/skills",
		LocalSkillPath:    ".zcode/skills",
		LocalSkillHomeEnv: "ZCODE_DATA_BASE_DIR", LocalSkillHomeSuffix: ".zcode/skills",
		DesktopBundleProbe: "zcode-app",
		Capabilities:       RuntimeCapabilities{ModelSelection: false, ProjectSkills: true, LocalSkills: true, NativeSkillDiscovery: true},
		factory:            func(cfg Config) Backend { return &zcodeBackend{cfg: cfg} },
		blockedArgs:        zcodeBlockedArgs,
		listModels: func(ctx context.Context, providerType, executablePath string) ([]Model, error) {
			// ZCode/Magpie own the model configuration; no per-agent catalog.
			return []Model{}, nil
		}},
}
var runtimeByID = indexRuntimeDescriptors(runtimeDescriptors)

func indexRuntimeDescriptors(descriptors []RuntimeDescriptor) map[string]RuntimeDescriptor {
	m := make(map[string]RuntimeDescriptor, len(descriptors))
	for _, d := range descriptors {
		if d.ID == "" || d.factory == nil || d.listModels == nil {
			panic("invalid runtime descriptor")
		}
		if _, ok := m[d.ID]; ok {
			panic("duplicate runtime descriptor: " + d.ID)
		}
		m[d.ID] = d
	}
	return m
}

func RuntimeDescriptors() []RuntimeDescriptor {
	return append([]RuntimeDescriptor(nil), runtimeDescriptors...)
}
func RuntimeDescriptorFor(id string) (RuntimeDescriptor, bool) {
	d, ok := runtimeByID[id]
	return d, ok
}
func RuntimeCapabilitiesFor(id string) *RuntimeCapabilities {
	d, ok := RuntimeDescriptorFor(id)
	if !ok {
		return nil
	}
	c := d.Capabilities
	return &c
}
func RuntimeIDs() []string {
	ids := make([]string, 0, len(runtimeDescriptors))
	for _, d := range runtimeDescriptors {
		ids = append(ids, d.ID)
	}
	return ids
}
func RuntimeCommands() []string {
	var commands []string
	for _, d := range runtimeDescriptors {
		if !d.DirectProbeOnly {
			commands = append(commands, d.Command)
		}
	}
	return commands
}
func RuntimeFactory(id string, cfg Config) (Backend, error) {
	d, ok := RuntimeDescriptorFor(id)
	if !ok {
		return nil, fmt.Errorf("unknown agent type: %q (supported: %s)", id, strings.Join(RuntimeIDs(), ", "))
	}
	return d.newBackend(cfg), nil
}

func (d RuntimeDescriptor) newBackend(cfg Config) Backend {
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}
	cfg.LaunchPrefix = filterLaunchPrefix(d.blockedArgs, cfg.LaunchPrefix, cfg.Logger)
	return d.factory(cfg)
}
func (d RuntimeDescriptor) LocalSkillsDirectory(home string, getenv func(string) string) string {
	if d.LocalSkillHomeEnv != "" {
		if base := strings.TrimSpace(getenv(d.LocalSkillHomeEnv)); base != "" {
			return filepath.Join(base, filepath.FromSlash(d.LocalSkillHomeSuffix))
		}
	}
	return filepath.Join(home, filepath.FromSlash(d.LocalSkillPath))
}

// RuntimeCatalogEntry is the public, transport-safe registry view. It exposes
// no paths, credentials or implementation hooks.
type RuntimeCatalogEntry struct {
	ID           string              `json:"id"`
	DisplayName  string              `json:"display_name"`
	Capabilities RuntimeCapabilities `json:"capabilities"`
}

func RuntimeCatalog() []RuntimeCatalogEntry {
	entries := make([]RuntimeCatalogEntry, 0, len(runtimeDescriptors))
	for _, d := range runtimeDescriptors {
		entries = append(entries, RuntimeCatalogEntry{ID: d.ID, DisplayName: d.DisplayName, Capabilities: d.Capabilities})
	}
	return entries
}
