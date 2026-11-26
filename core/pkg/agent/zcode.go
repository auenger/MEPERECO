package agent

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"
)

// ZCode's official headless CLI emits the protocol session event envelope
// followed by a tagged result. The user-owned ~/.zcode remains the source of
// account, model, native agents, skills, commands, plugins and workflows.
type zcodeBackend struct{ cfg Config }

var zcodeBlockedArgs = map[string]blockedArgMode{
	"-p": blockedWithValue, "--prompt": blockedWithValue, "--cwd": blockedWithValue,
	"--output-format": blockedWithValue, "--mode": blockedWithValue,
	"--resume": blockedWithValue, "--continue": blockedStandalone, "-c": blockedStandalone,
	"--json": blockedStandalone, "--no-color": blockedStandalone,
	"--target": blockedWithValue, "--target-replace": blockedStandalone,
	"--workflow-mode": blockedWithValue, "--surface": blockedWithValue, "--model": blockedWithValue,
	"--settings": blockedWithValue, "--permission-mode": blockedWithValue,
	"--allowed-tools": blockedWithValue, "--print": blockedStandalone,
	"--max-turns": blockedWithValue,
}

func buildZCodeArgs(prompt string, opts ExecOptions, logger *slog.Logger) []string {
	args := []string{"--prompt", prompt, "--output-format", "stream-json", "--no-color",
		"--mode", "guarded", "--workflow-mode", "onDemand", "--surface", "terminal"}
	if opts.Cwd != "" {
		args = append(args, "--cwd", opts.Cwd)
	}
	if opts.ResumeSessionID != "" {
		args = append(args, "--resume", opts.ResumeSessionID)
	}
	args = append(args, filterCustomArgs(opts.ExtraArgs, zcodeBlockedArgs, logger)...)
	args = append(args, filterCustomArgs(opts.CustomArgs, zcodeBlockedArgs, logger)...)
	return args
}

type zcodeStreamEvent struct {
	Type       string       `json:"type"`
	SessionID  string       `json:"sessionId"`
	Response   string       `json:"response"`
	Payload    zcodePayload `json:"payload"`
	Usage      *zcodeUsage  `json:"usage"`
	Projection struct {
		Status string `json:"status"`
	} `json:"projection"`
}

type zcodePayload struct {
	Kind       string          `json:"kind"`
	Delta      string          `json:"delta"`
	Role       string          `json:"role"`
	Content    json.RawMessage `json:"content"`
	ModelID    string          `json:"modelId"`
	ProviderID string          `json:"providerId"`
	ToolCallID string          `json:"toolCallId"`
	ToolName   string          `json:"toolName"`
	Input      map[string]any  `json:"input"`
	Result     json.RawMessage `json:"result"`
	Error      json.RawMessage `json:"error"`
}

type zcodeUsage struct {
	InputTokens      int64 `json:"inputTokens"`
	OutputTokens     int64 `json:"outputTokens"`
	CacheReadTokens  int64 `json:"cacheReadTokens"`
	CacheWriteTokens int64 `json:"cacheWriteTokens"`
}

type zcodeStreamState struct {
	terminal     streamTerminalState
	sessionID    string
	modelID      string
	lastFailure  string
	streamedText bool
	toolSeen     map[string]bool
	usage        map[string]TokenUsage
}

func zcodeJSONText(raw json.RawMessage) string {
	if len(raw) == 0 || string(raw) == "null" {
		return ""
	}
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return s
	}
	return string(raw)
}

func (s *zcodeStreamState) handle(event zcodeStreamEvent, emit func(Message)) {
	if event.SessionID != "" && s.sessionID == "" {
		s.sessionID = event.SessionID
		emit(Message{Type: MessageStatus, Status: "running", SessionID: s.sessionID})
	}
	p := event.Payload
	if p.ModelID != "" {
		s.modelID = p.ModelID
	}
	switch event.Type {
	case "model.streaming":
		switch p.Kind {
		case "text_delta":
			if p.Delta != "" {
				s.streamedText = true
				emit(Message{Type: MessageText, Content: p.Delta})
			}
		case "reasoning_delta":
			if p.Delta != "" {
				emit(Message{Type: MessageThinking, Content: p.Delta})
			}
		}
	case "message.upserted":
		if p.Role == "assistant" {
			var content string
			if json.Unmarshal(p.Content, &content) == nil && content != "" {
				s.terminal.lastAssistantText = content
			}
		}
	case "tool.updated":
		if p.ToolCallID == "" {
			return
		}
		switch p.Kind {
		case "scheduled", "started":
			if s.toolSeen == nil {
				s.toolSeen = map[string]bool{}
			}
			if !s.toolSeen[p.ToolCallID] {
				s.toolSeen[p.ToolCallID] = true
				emit(Message{Type: MessageToolUse, Tool: p.ToolName, CallID: p.ToolCallID, Input: p.Input})
			}
		case "result", "error":
			output := zcodeJSONText(p.Result)
			if output == "" {
				output = zcodeJSONText(p.Error)
			}
			emit(Message{Type: MessageToolResult, Tool: p.ToolName, CallID: p.ToolCallID, Output: output})
		}
	case "turn.failed":
		// Upstream can terminate without a result row, but the provider error
		// is in payload.error.message (e.g. insufficient plan quota).
		var failure struct {
			Message string `json:"message"`
		}
		if json.Unmarshal(p.Error, &failure) == nil && failure.Message != "" {
			s.lastFailure = failure.Message
		} else {
			var message string
			if json.Unmarshal(p.Error, &message) == nil {
				s.lastFailure = message
			}
		}
	case "permission.requested":
		emit(Message{Type: MessageStatus, Status: "Permission requested by ZCode"})
	case "result":
		s.terminal.sawResult = true
		s.terminal.finalResultText = event.Response
		switch strings.ToLower(event.Projection.Status) {
		case "error", "failed", "aborted", "cancelled":
			s.terminal.resultIsError = true
		}
		// The final summary is the turn's aggregate; never count per-event
		// usage again or invent a model/token split from totalTokenCount.
		if u := event.Usage; u != nil && s.modelID != "" && (u.InputTokens > 0 || u.OutputTokens > 0 || u.CacheReadTokens > 0 || u.CacheWriteTokens > 0) {
			s.usage = map[string]TokenUsage{s.modelID: {InputTokens: u.InputTokens, OutputTokens: u.OutputTokens, CacheReadTokens: u.CacheReadTokens, CacheWriteTokens: u.CacheWriteTokens}}
		}
	}
}

func (b *zcodeBackend) Execute(ctx context.Context, prompt string, opts ExecOptions) (*Session, error) {
	path := b.cfg.ExecutablePath
	if path == "" {
		path = "zcode"
	}
	if _, err := exec.LookPath(path); err != nil {
		return nil, fmt.Errorf("ZCode executable not found: %w", err)
	}
	if hasManagedMcpConfig(opts.McpConfig) {
		return nil, fmt.Errorf("ZCode currently uses its native MCP configuration; configure MCP in ZCode rather than an agent-specific override")
	}
	if strings.TrimSpace(opts.Model) != "" {
		return nil, fmt.Errorf("ZCode model selection is managed by ZCode or Magpie; clear the Agent/Task model override and configure the model on the runtime machine")
	}
	env := zcodeLaunchEnv(path, b.cfg.Env)
	runCtx, cancel := runContext(ctx, opts.Timeout)
	cmd, env, cleanup, err := prepareZCodeCommand(runCtx, b.cfg, path, buildZCodeArgs(prompt, opts, b.cfg.Logger), env)
	if err != nil {
		cancel()
		return nil, err
	}
	hideAgentWindow(cmd)
	configureProcessGroup(cmd)
	cmd.Cancel = func() error { signalProcessGroup(cmd.Process, syscall.SIGKILL); return nil }
	cmd.WaitDelay = 10 * time.Second
	cmd.Dir = opts.Cwd
	// Inherit ZCode's native settings (including any Magpie-managed provider).
	// Never create, rewrite, or copy its model/credential configuration.
	cmd.Env = buildEnv(env)
	logAgentCommand(b.cfg.Logger, "zcode", cmd)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		cancel()
		cleanup()
		return nil, err
	}
	stderr := newStderrTail(newLogWriter(b.cfg.Logger, "[zcode:stderr] "), agentStderrTailBytes)
	cmd.Stderr = stderr
	if err := cmd.Start(); err != nil {
		cancel()
		cleanup()
		return nil, fmt.Errorf("start ZCode: %w", err)
	}
	msgCh := make(chan Message, 256)
	resCh := make(chan Result, 1)
	go func() {
		defer cancel()
		defer cleanup()
		defer close(resCh)
		defer close(msgCh)
		started := time.Now()
		state := zcodeStreamState{}
		emit := func(msg Message) {
			select {
			case msgCh <- msg:
			case <-runCtx.Done():
			}
		}
		done := make(chan struct{})
		go func() {
			select {
			case <-runCtx.Done():
				_ = stdout.Close()
			case <-done:
			}
		}()
		scanner := bufio.NewScanner(stdout)
		scanner.Buffer(make([]byte, 0, 64*1024), 16*1024*1024)
		for scanner.Scan() {
			var event zcodeStreamEvent
			if json.Unmarshal(scanner.Bytes(), &event) == nil && event.Type != "" {
				state.handle(event, emit)
			}
		}
		state.terminal.scanErr = scanner.Err()
		if state.terminal.scanErr != nil {
			_ = stdout.Close()
			signalProcessGroup(cmd.Process, syscall.SIGKILL)
		}
		exitErr := cmd.Wait()
		close(done)
		status, output, errMsg := finalizeStreamResult("ZCode", opts.Timeout, runCtx.Err(), nil, exitErr, state.sessionID, state.terminal, "")
		if status == "failed" && state.lastFailure != "" {
			errMsg = "ZCode: " + state.lastFailure
		}
		if errMsg != "" {
			errMsg = withAgentStderr(errMsg, "zcode", stderr.Tail())
		}
		if status == "completed" && !state.streamedText && output != "" {
			emit(Message{Type: MessageText, Content: output})
		}
		resCh <- Result{Status: status, Output: output, Error: errMsg, SessionID: state.sessionID, DurationMs: time.Since(started).Milliseconds(), Usage: state.usage}
	}()
	return &Session{Messages: msgCh, Result: resCh}, nil
}

// Desktop distributions place the official built-in provider asset beside
// Resources/glm, but some installed CLI wrappers search under glm instead.
// Supply that existing asset when the wrapper is detected, without copying it
// or overriding a user's explicit configuration.
func zcodeLaunchEnv(executable string, extra map[string]string) map[string]string {
	const key = "ZCODE_BUILTIN_PROVIDER_CONFIG_FILE"
	if extra[key] != "" || os.Getenv(key) != "" {
		return extra
	}
	resolved, err := exec.LookPath(executable)
	if err != nil {
		return extra
	}
	dir := filepath.Dir(resolved)
	candidates := []string{
		filepath.Join(dir, "config", "provider", "zcode-builtin.json"),
		filepath.Join(dir, "..", "config", "provider", "zcode-builtin.json"),
	}
	isWrapper := false
	if file, err := os.Open(resolved); err == nil {
		header, _ := io.ReadAll(io.LimitReader(file, 16*1024))
		_ = file.Close()
		lower := strings.ToLower(string(header))
		isWrapper = strings.Contains(lower, "zcode.cjs") && (strings.HasPrefix(lower, "#!") || strings.Contains(lower, "@echo"))
	}
	if isWrapper {
		appPath := extra["ZCODE_APP"]
		if appPath == "" {
			appPath = os.Getenv("ZCODE_APP")
		}
		if appPath != "" {
			candidates = append(candidates,
				filepath.Join(appPath, "Resources", "config", "provider", "zcode-builtin.json"),
				filepath.Join(appPath, "Contents", "Resources", "config", "provider", "zcode-builtin.json"),
				filepath.Join(appPath, "resources", "config", "provider", "zcode-builtin.json"))
		}
		switch runtime.GOOS {
		case "darwin":
			candidates = append(candidates, "/Applications/ZCode.app/Contents/Resources/config/provider/zcode-builtin.json")
			if home, err := os.UserHomeDir(); err == nil {
				candidates = append(candidates, filepath.Join(home, "Applications", "ZCode.app", "Contents", "Resources", "config", "provider", "zcode-builtin.json"))
			}
		case "windows":
			for _, root := range []string{os.Getenv("LOCALAPPDATA"), os.Getenv("ProgramFiles"), os.Getenv("ProgramFiles(x86)")} {
				if root != "" {
					candidates = append(candidates, filepath.Join(root, "Programs", "ZCode", "resources", "config", "provider", "zcode-builtin.json"), filepath.Join(root, "ZCode", "resources", "config", "provider", "zcode-builtin.json"))
				}
			}
		}
	}
	for _, candidate := range candidates {
		if info, err := os.Stat(candidate); err == nil && info.Mode().IsRegular() {
			env := make(map[string]string, len(extra)+1)
			for k, v := range extra {
				env[k] = v
			}
			env[key] = filepath.Clean(candidate)
			return env
		}
	}
	return extra
}
