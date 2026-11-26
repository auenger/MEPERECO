package daemon

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/auenger/MEPERECO/core/internal/cli"
)

func TestBrandedAgentCLIPath(t *testing.T) {
	temp := t.TempDir()
	source := filepath.Join(temp, "legacy")
	if err := os.WriteFile(source, []byte("CLI fixture"), 0700); err != nil {
		t.Fatal(err)
	}
	path, err := agentCLIPath(source, filepath.Join(temp, "run"))
	if err != nil {
		t.Fatal(err)
	}
	name := "meperstudio"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	target := filepath.Join(strings.Split(path, string(os.PathListSeparator))[0], name)
	body, err := os.ReadFile(target)
	if err != nil || string(body) != "CLI fixture" {
		t.Fatalf("alias did not resolve: %s: %v", target, err)
	}
}
func TestBrandBuiltinCLIExamples(t *testing.T) {
	old := "`silieco repo checkout <url>` and `silieco --help`; builtin:silieco-runtimes-and-repos; ~/.silieco/config"
	want := "`meperstudio repo checkout <url>` and `meperstudio --help`; builtin:silieco-runtimes-and-repos; ~/.silieco/config"
	if got := cli.BrandCLIText(old); got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}
