package branding

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestStateDirFreshAndLegacy(t *testing.T) {
	raw, err := os.ReadFile("testdata/state-dir.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases []struct {
		Directories []string
		Expected    string
	}
	if err := json.Unmarshal(raw, &cases); err != nil {
		t.Fatal(err)
	}
	for _, tc := range cases {
		home := t.TempDir()
		for _, dir := range tc.Directories {
			if err := os.Mkdir(filepath.Join(home, dir), 0700); err != nil {
				t.Fatal(err)
			}
		}
		if got := StateDir(home); got != filepath.Join(home, tc.Expected) {
			t.Fatalf("%v: %s", tc, got)
		}
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

func TestCanonicalLookup(t *testing.T) {
	t.Setenv("SILIECO_BRANDING_LOOKUP", "old")
	t.Setenv("MEPERSTUDIO_BRANDING_LOOKUP", "new")
	if Getenv("SILIECO_BRANDING_LOOKUP") != "new" || Getenv("MEPERSTUDIO_BRANDING_LOOKUP") != "new" {
		t.Fatal("canonical precedence lost")
	}
	t.Setenv("MEPERSTUDIO_BRANDING_LOOKUP", "")
	if value, ok := LookupEnv("SILIECO_BRANDING_LOOKUP"); !ok || value != "" {
		t.Fatal("explicit empty value must win")
	}
}
