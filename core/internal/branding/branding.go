// Package branding centralizes the public MEPERStudio configuration names.
package branding

import (
	"os"
	"path/filepath"
	"strings"
)

// ApplyEnvAliases lets existing readers continue using the legacy keys while
// operators configure MEPERSTUDIO_* names. Explicit legacy values remain valid.
// A new key wins when both are present. Call at process entry, before reading config.
func ApplyEnvAliases() {
	for _, entry := range os.Environ() {
		key, value, ok := strings.Cut(entry, "=")
		if !ok || !strings.HasPrefix(key, "MEPERSTUDIO_") {
			continue
		}
		_ = os.Setenv("SILIECO_"+strings.TrimPrefix(key, "MEPERSTUDIO_"), value)
	}
}

// StateDir uses the existing legacy directory if the user has one, preserving
// authentication, profiles and running daemon identity. Fresh installs use the
// branded directory; no directory is moved while an old daemon may be running.
func StateDir(home string) string {
	current := filepath.Join(home, ".meperstudio")
	if _, err := os.Stat(current); err == nil {
		return current
	}
	legacy := filepath.Join(home, ".silieco")
	if _, err := os.Stat(legacy); err == nil {
		return legacy
	}
	return current
}

// IsManagedEnvKey covers both namespaces; both must be filtered from inherited
// agent environments and rejected as custom per-agent overrides.
func IsManagedEnvKey(key string) bool {
	key = strings.ToUpper(key)
	return strings.HasPrefix(key, "MEPERSTUDIO_") || strings.HasPrefix(key, "SILIECO_")
}

// AddBrandedRunEnv mirrors only daemon-generated task context, never the
// inherited daemon environment or user custom variables.
func AddBrandedRunEnv(values map[string]string) {
	for key, value := range values {
		if strings.HasPrefix(key, "SILIECO_") {
			values["MEPERSTUDIO_"+strings.TrimPrefix(key, "SILIECO_")] = value
		}
	}
}
