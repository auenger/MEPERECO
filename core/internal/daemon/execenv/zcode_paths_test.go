package execenv

import (
	"path/filepath"
	"testing"
)

func TestZCodeProjectContextPaths(t *testing.T) {
	root := t.TempDir()
	if got := skillsDirPath(root, "zcode"); got != filepath.Join(root, ".zcode", "skills") {
		t.Fatal(got)
	}
	if got := runtimeConfigPath(root, "zcode"); got != filepath.Join(root, "AGENTS.md") {
		t.Fatal(got)
	}
}
