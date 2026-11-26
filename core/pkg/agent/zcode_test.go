package agent

import (
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"
)

func TestZCodeArgs(t *testing.T) {
	args := buildZCodeArgs("hello", ExecOptions{Cwd: "/example", ResumeSessionID: "sess_1", CustomArgs: []string{"--mode", "yolo", "--output-format", "text", "--locale", "zh-CN"}}, slog.Default())
	want := []string{"--prompt", "hello", "--output-format", "stream-json", "--no-color", "--mode", "guarded", "--workflow-mode", "onDemand", "--surface", "terminal", "--cwd", "/example", "--resume", "sess_1", "--locale", "zh-CN"}
	if !slices.Equal(args, want) {
		t.Fatalf("got %q; want %q", args, want)
	}
	if ModelSelectionSupported("zcode") {
		t.Fatal("ZCode models must be managed by the runtime")
	}
	if backend, err := New("zcode", Config{}); err != nil || backend == nil {
		t.Fatal(err)
	}
}

func TestZCodeStream(t *testing.T) {
	events := []string{
		`{"type":"turn.started","sessionId":"sess_1","payload":{}}`,
		`{"type":"session.updated","sessionId":"sess_1","payload":{"modelId":"glm-test"}}`,
		`{"type":"model.streaming","sessionId":"sess_1","payload":{"kind":"text_delta","delta":"hello"}}`,
		`{"type":"tool.updated","sessionId":"sess_1","payload":{"kind":"scheduled","toolCallId":"t1","toolName":"Read","input":{"path":"demo.txt"}}}`,
		`{"type":"tool.updated","sessionId":"sess_1","payload":{"kind":"started","toolCallId":"t1","toolName":"Read"}}`,
		`{"type":"tool.updated","sessionId":"sess_1","payload":{"kind":"result","toolCallId":"t1","toolName":"Read","result":"ok"}}`,
		`{"type":"result","sessionId":"sess_1","response":"hello","usage":{"inputTokens":10,"outputTokens":2},"projection":{"status":"idle"}}`,
	}
	var state zcodeStreamState
	var msgs []Message
	for _, line := range events {
		var event zcodeStreamEvent
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			t.Fatal(err)
		}
		state.handle(event, func(m Message) { msgs = append(msgs, m) })
	}
	if !state.terminal.sawResult || state.sessionID != "sess_1" || state.terminal.finalResultText != "hello" || state.usage["glm-test"].InputTokens != 10 {
		t.Fatalf("bad state: %+v", state)
	}
	if len(msgs) != 4 || msgs[0].Type != MessageStatus || msgs[1].Type != MessageText || msgs[2].Type != MessageToolUse || msgs[3].Type != MessageToolResult {
		t.Fatalf("bad messages: %+v", msgs)
	}
}

func TestZCodeLaunchEnv(t *testing.T) {
	t.Setenv("ZCODE_BUILTIN_PROVIDER_CONFIG_FILE", "")
	root := t.TempDir()
	exe := filepath.Join(root, "zcode")
	if runtime.GOOS == "windows" {
		exe += ".exe"
	}
	if err := os.WriteFile(exe, []byte("#!/bin/sh\n"), 0700); err != nil {
		t.Fatal(err)
	}
	config := filepath.Join(root, "config", "provider", "zcode-builtin.json")
	if err := os.MkdirAll(filepath.Dir(config), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(config, []byte("{}"), 0600); err != nil {
		t.Fatal(err)
	}
	env := zcodeLaunchEnv(exe, map[string]string{"KEEP": "yes"})
	if env["ZCODE_BUILTIN_PROVIDER_CONFIG_FILE"] != config || env["KEEP"] != "yes" {
		t.Fatalf("unexpected env mapping")
	}
	if zcodeLaunchEnv(exe, map[string]string{"ZCODE_BUILTIN_PROVIDER_CONFIG_FILE": "explicit"})["ZCODE_BUILTIN_PROVIDER_CONFIG_FILE"] != "explicit" {
		t.Fatal("explicit config overwritten")
	}
}

func TestZCodeMacOSBackgroundHost(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("macOS desktop layout")
	}
	root := t.TempDir()
	contents := filepath.Join(root, "ZCode.app", "Contents")
	resources := filepath.Join(contents, "Resources")
	config := filepath.Join(resources, "config", "provider", "zcode-builtin.json")
	kernel := filepath.Join(resources, "glm", "zcode.cjs")
	helper := filepath.Join(contents, "Frameworks", "ZCode Helper.app", "Contents", "MacOS", "ZCode Helper")
	main := filepath.Join(contents, "MacOS", "ZCode")
	wrapper := filepath.Join(root, "zcode")
	for _, path := range []string{config, kernel, helper, main, filepath.Join(resources, "app.asar")} {
		if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("fixture"), 0700); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(wrapper, []byte("#!/bin/sh\nexec node zcode.cjs \"$@\"\n"), 0700); err != nil {
		t.Fatal(err)
	}
	env := map[string]string{"ZCODE_BUILTIN_PROVIDER_CONFIG_FILE": config}
	host, selectedKernel, _, ok := zcodeOfficialDesktopHost(wrapper, env)
	if !ok || host != helper || selectedKernel != kernel {
		t.Fatalf("expected background helper %q, got %q (%v)", helper, host, ok)
	}
	cmd, preparedEnv, cleanup, err := prepareZCodeCommand(context.Background(), Config{}, wrapper, []string{"--version"}, env)
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	if cmd.Path != helper || preparedEnv["ELECTRON_RUN_AS_NODE"] != "1" {
		t.Fatalf("unexpected launcher %q", cmd.Path)
	}
}

func TestZCodeBackendFixture(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell fixture")
	}
	for _, tc := range []struct {
		name, stream string
		status       string
	}{
		{"terminal", `{"type":"result","sessionId":"sess_1","response":"ok","projection":{"status":"idle"}}`, "completed"},
		{"missing-terminal", `{"type":"turn.started","sessionId":"sess_1"}`, "failed"},
		{"provider-error", `{"type":"turn.failed","sessionId":"sess_1","payload":{"error":{"code":"1113","message":"[1113] insufficient resources"}}}`, "failed"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			exe := filepath.Join(t.TempDir(), "zcode")
			if err := os.WriteFile(exe, []byte("#!/bin/sh\nprintf '%s\\n' '"+tc.stream+"'\n"), 0700); err != nil {
				t.Fatal(err)
			}
			b, err := New("zcode", Config{ExecutablePath: exe})
			if err != nil {
				t.Fatal(err)
			}
			sess, err := b.Execute(context.Background(), "hello", ExecOptions{Timeout: 5 * time.Second})
			if err != nil {
				t.Fatal(err)
			}
			for range sess.Messages {
			}
			result := <-sess.Result
			if result.Status != tc.status || result.SessionID != "sess_1" || (tc.status == "completed" && result.Output != "ok") {
				t.Fatalf("%+v", result)
			}
			if tc.name == "missing-terminal" && !strings.Contains(result.Error, "terminal result") {
				t.Fatalf("unexpected error %q", result.Error)
			}
			if tc.name == "provider-error" && !strings.Contains(result.Error, "1113") {
				t.Fatalf("provider error lost: %q", result.Error)
			}
		})
	}
}

func TestZCodeNativeModelOwnership(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell fixture")
	}
	root := t.TempDir()
	personal := filepath.Join(root, "provider.json")
	exe := filepath.Join(root, "zcode")
	original := []byte(`{"config":{"defaultModelSelection":{"providerId":"magpie","modelId":"glm-5.3-flash"}}}`)
	if err := os.WriteFile(personal, original, 0600); err != nil {
		t.Fatal(err)
	}
	// The native CLI sees the original config path, not a generated per-run
	// copy. Usage is based on its actual model, not an Agent override.
	stream := `{"type":"session.updated","sessionId":"sess_1","payload":{"providerId":"magpie","modelId":"glm-5.3-flash"}}
{"type":"result","sessionId":"sess_1","response":"ok","usage":{"inputTokens":10,"outputTokens":2},"projection":{"status":"idle"}}`
	script := "#!/bin/sh\n[ \"$ZCODE_PERSONAL_PROVIDER_CONFIG_FILE\" = \"$EXPECTED_CONFIG_FILE\" ] || exit 2\nprintf '%s\\n' '" + stream + "'\n"
	if err := os.WriteFile(exe, []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	b, err := New("zcode", Config{ExecutablePath: exe, Env: map[string]string{
		"ZCODE_PERSONAL_PROVIDER_CONFIG_FILE": personal, "EXPECTED_CONFIG_FILE": personal,
		"ZCODE_BUILTIN_PROVIDER_CONFIG_FILE": filepath.Join(root, "missing.json"),
	}})
	if err != nil {
		t.Fatal(err)
	}
	models, err := ListModels(context.Background(), "zcode", exe)
	if err != nil || len(models) != 0 || ModelSelectionSupported("zcode") {
		t.Fatalf("ZCode advertises a per-agent model catalog: %+v, %v", models, err)
	}
	if !ModelKnownIncompatibleWithProvider("zcode", "old-model") {
		t.Fatal("switching to ZCode should clear the previous model")
	}
	if _, err := b.Execute(context.Background(), "test", ExecOptions{Model: "old-model"}); err == nil || !strings.Contains(err.Error(), "managed by ZCode or Magpie") {
		t.Fatalf("explicit override should fail before launching the CLI: %v", err)
	}
	sess, err := b.Execute(context.Background(), "test", ExecOptions{Timeout: 5 * time.Second})
	if err != nil {
		t.Fatal(err)
	}
	for range sess.Messages {
	}
	r := <-sess.Result
	if r.Status != "completed" || r.Output != "ok" || r.Usage["glm-5.3-flash"].InputTokens != 10 {
		t.Fatalf("native result: %+v", r)
	}
	if current, err := os.ReadFile(personal); err != nil || string(current) != string(original) {
		t.Fatal("native model configuration modified")
	}
}

// Opt-in only: uses the current user's native ZCode login. Never runs during
// routine tests or CI, and the prompt requests no tool calls.
func TestZCodeInstalledSmoke(t *testing.T) {
	if os.Getenv("MEPERSTUDIO_ZCODE_SMOKE") != "1" {
		t.Skip("explicit native runtime smoke test")
	}
	model := os.Getenv("MEPERSTUDIO_ZCODE_SMOKE_MODEL")
	work := t.TempDir()
	b, err := New("zcode", Config{})
	if err != nil {
		t.Fatal(err)
	}
	s, err := b.Execute(context.Background(), "Reply ZCODE_RUNTIME_OK only. Do not use tools.", ExecOptions{Cwd: work, Timeout: 65 * time.Second})
	if err != nil {
		t.Fatal(err)
	}
	for range s.Messages {
	}
	r := <-s.Result
	if r.Status != "completed" {
		t.Fatalf("ZCode native smoke: %s: %s", r.Status, r.Error)
	}
	if !strings.Contains(r.Output, "ZCODE_RUNTIME_OK") || r.SessionID == "" {
		t.Fatal("native result missing expected response or session")
	}
	if model != "" {
		if _, ok := r.Usage[model]; !ok {
			t.Fatalf("native CLI did not report expected model %q in usage", model)
		}
	}
	t.Logf("native terminal result verified, session present, usage models: %d", len(r.Usage))
}
