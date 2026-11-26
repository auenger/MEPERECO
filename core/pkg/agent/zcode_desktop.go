package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

func zcodeEnvValue(env map[string]string, key string) string {
	if value, ok := env[key]; ok {
		return strings.TrimSpace(value)
	}
	return strings.TrimSpace(os.Getenv(key))
}

// Some official desktop versions ship the CLI kernel separately from the
// native credential module in app.asar. Run that unchanged kernel with the
// official Electron Node host, resolving its own credential module through
// the application's ASAR file system. On macOS use the shipped background
// helper (LSUIElement) rather than the main GUI application, which registers
// transient Dock icons for headless runs. The OS credential store still owns
// all access decisions; no passwords or credential files are read by us.
func zcodeOfficialDesktopHost(path string, env map[string]string) (host, kernel, asarModule string, ok bool) {
	config := zcodeEnvValue(env, "ZCODE_BUILTIN_PROVIDER_CONFIG_FILE")
	if config == "" {
		return
	}
	resources := filepath.Dir(filepath.Dir(filepath.Dir(config)))
	kernel = filepath.Join(resources, "glm", "zcode.cjs")
	resolved, err := exec.LookPath(path)
	if err != nil {
		return "", "", "", false
	}
	ownKernel := filepath.Clean(resolved) == filepath.Clean(kernel)
	wrapper := false
	if file, err := os.Open(resolved); err == nil {
		head, _ := io.ReadAll(io.LimitReader(file, 16*1024))
		_ = file.Close()
		text := strings.ToLower(string(head))
		wrapper = strings.Contains(text, "zcode.cjs") && (strings.HasPrefix(text, "#!") || strings.Contains(text, "@echo"))
	}
	if !ownKernel && !wrapper {
		return "", "", "", false
	}
	switch runtime.GOOS {
	case "darwin":
		contents := filepath.Dir(resources)
		host = filepath.Join(contents, "Frameworks", "ZCode Helper.app", "Contents", "MacOS", "ZCode Helper")
		if info, err := os.Stat(host); err != nil || !info.Mode().IsRegular() {
			// Older desktop packages may not ship the generic helper.
			host = filepath.Join(contents, "MacOS", "ZCode")
		}
	case "windows":
		host = filepath.Join(filepath.Dir(resources), "ZCode.exe")
	default:
		return "", "", "", false
	}
	for _, file := range []string{host, kernel, filepath.Join(resources, "app.asar")} {
		if info, err := os.Stat(file); err != nil || !info.Mode().IsRegular() {
			return "", "", "", false
		}
	}
	return host, kernel, filepath.Join(resources, "app.asar", "node_modules", "@napi-rs", "keyring", "index.js"), true
}

func prepareZCodeCommand(ctx context.Context, cfg Config, path string, args []string, env map[string]string) (*exec.Cmd, map[string]string, func(), error) {
	host, kernel, keyring, ok := zcodeOfficialDesktopHost(path, env)
	if !ok {
		return cfg.commandAt(path).exec(ctx, args...), env, func() {}, nil
	}
	dir, err := os.MkdirTemp("", "meper-zcode-launch-")
	if err != nil {
		return nil, nil, nil, err
	}
	cleanup := func() { _ = os.RemoveAll(dir) }
	literal, err := json.Marshal(keyring)
	if err != nil {
		cleanup()
		return nil, nil, nil, err
	}
	script := fmt.Sprintf(`const {registerHooks}=require("node:module");
const {pathToFileURL}=require("node:url");
const officialKeyring=%s;
registerHooks({resolve(specifier,context,nextResolve){
 if(specifier==="@napi-rs/keyring") return {url:pathToFileURL(officialKeyring).href,format:"commonjs",shortCircuit:true};
 return nextResolve(specifier,context);
}});`, literal)
	hook := filepath.Join(dir, "official-keyring.cjs")
	if err := os.WriteFile(hook, []byte(script), 0600); err != nil {
		cleanup()
		return nil, nil, nil, err
	}
	out := make(map[string]string, len(env)+1)
	for k, v := range env {
		out[k] = v
	}
	out["ELECTRON_RUN_AS_NODE"] = "1"
	full := append([]string{"--require", hook, kernel}, cfg.commandAt(path).Argv(args...)...)
	return exec.CommandContext(ctx, host, full...), out, cleanup, nil
}
