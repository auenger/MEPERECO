package daemon

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// Agent runs are isolated. A hard link keeps the same executable/signature;
// copying is the cross-volume fallback (notably on Windows). Do not mutate
// the installed CLI or rely on shell aliases, which fail in noninteractive
// shells and some provider sandboxes.
func agentCLIPath(selfBin, runTempDir string) (string, error) {
	dir := filepath.Join(runTempDir, "bin")
	if err := os.MkdirAll(dir, 0700); err != nil {
		return "", err
	}
	name := "meperstudio"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	target := filepath.Join(dir, name)
	if err := linkOrCopyCLI(selfBin, target); err != nil {
		return "", err
	}
	// Old persisted agent instructions may still invoke the legacy command.
	legacy := filepath.Join(dir, strings.Replace(name, "meperstudio", "silieco", 1))
	if err := linkOrCopyCLI(target, legacy); err != nil {
		return "", err
	}

	return dir + string(os.PathListSeparator) + filepath.Dir(selfBin) + string(os.PathListSeparator) + os.Getenv("PATH"), nil
}

func linkOrCopyCLI(source, target string) error {
	if err := os.Link(source, target); err == nil {
		return nil
	}
	src, err := os.Open(source)
	if err != nil {
		return err
	}
	defer src.Close()
	dst, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0700)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(dst, src)
	closeErr := dst.Close()
	if copyErr != nil || closeErr != nil {
		_ = os.Remove(target)
		return fmt.Errorf("copy compatible CLI: %v / %v", copyErr, closeErr)
	}
	return nil
}
