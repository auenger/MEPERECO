package daemon

import (
	"path/filepath"
	"testing"
)

func TestZCodeNativeSkillRoot(t *testing.T) {
	base := t.TempDir()
	t.Setenv("ZCODE_DATA_BASE_DIR", base)
	roots, ok, err := localSkillRootsForProvider("zcode")
	if err != nil || !ok || len(roots) == 0 || roots[0].path != filepath.Join(base, ".zcode", "skills") {
		t.Fatalf("roots=%+v ok=%v err=%v", roots, ok, err)
	}
}
