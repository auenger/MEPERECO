package daemon

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/auenger/MEPERECO/core/internal/branding"
)

// Optional desktop-distribution fallbacks are selected by runtime metadata,
// not hard-coded in the general PATH discovery loop.
var desktopRuntimeProbes = map[string]func(string, string) (AgentEntry, bool){
	"zcode-app": probeZCodeDesktop,
	"codex-app": probeCodexDesktop,
}

func probeZCodeDesktop(cmd, modelEnv string) (AgentEntry, bool) {
	if runtime.GOOS != "darwin" {
		return AgentEntry{}, false
	}
	// Official desktop installation ships an executable Node bundle.
	// Only offer it when node is also usable in the daemon environment;
	// the normal version probe will still reject a broken install.
	if _, err := exec.LookPath("node"); err == nil {
		paths := []string{"/Applications/ZCode.app/Contents/Resources/glm/zcode.cjs"}
		if home, err := os.UserHomeDir(); err == nil {
			paths = append(paths, filepath.Join(home, "Applications", "ZCode.app", "Contents", "Resources", "glm", "zcode.cjs"))
		}
		for _, path := range paths {
			if resolved, err := resolveAgentExecutablePath(path); err == nil {
				return AgentEntry{Path: resolved, Command: path}, true
			}
		}
	}
	return AgentEntry{}, false
}
func probeCodexDesktop(cmd, modelEnv string) (AgentEntry, bool) {
	// Codex Desktop bundles its CLI inside the macOS app instead of
	// installing it onto PATH.
	for _, p := range codexDesktopAppBundlePaths() {
		if _, err := os.Stat(p); err == nil {
			return AgentEntry{
				Path:    p,
				Command: cmd,
				Model:   strings.TrimSpace(branding.Getenv(modelEnv)),
			}, true
		}
	}
	return AgentEntry{}, false
}
