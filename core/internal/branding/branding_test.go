package branding

import (
	"os"
	"path/filepath"
	"testing"
)

func TestStateDirFreshAndLegacy(t *testing.T) {
	home := t.TempDir()
	current := filepath.Join(home, ".meperstudio")
	legacy := filepath.Join(home, ".silieco")
	if got := StateDir(home); got != current {
		t.Fatalf("fresh: %s", got)
	}
	if err := os.Mkdir(legacy, 0700); err != nil {
		t.Fatal(err)
	}
	if got := StateDir(home); got != legacy {
		t.Fatalf("legacy: %s", got)
	}
	if err := os.Mkdir(current, 0700); err != nil {
		t.Fatal(err)
	}
	if got := StateDir(home); got != current {
		t.Fatalf("canonical: %s", got)
	}
}

func TestEnvAliasesAndTaskContext(t *testing.T) {
	t.Setenv("SILIECO_BRANDING_TEST", "old")
	t.Setenv("MEPERSTUDIO_BRANDING_TEST", "new")
	ApplyEnvAliases()
	if got := os.Getenv("SILIECO_BRANDING_TEST"); got != "new" {
		t.Fatalf("alias: %s", got)
	}
	run := map[string]string{"SILIECO_TOKEN": "task-token", "PATH": "path"}
	AddBrandedRunEnv(run)
	if run["MEPERSTUDIO_TOKEN"] != "task-token" || run["MEPERSTUDIO_PATH"] != "" {
		t.Fatal("unexpected run aliases")
	}
	for _, key := range []string{"silieco_token", "MEPERSTUDIO_TOKEN"} {
		if !IsManagedEnvKey(key) {
			t.Fatalf("unprotected key: %s", key)
		}
	}
	if IsManagedEnvKey("PATH") {
		t.Fatal("ordinary env should not be reserved")
	}
}
