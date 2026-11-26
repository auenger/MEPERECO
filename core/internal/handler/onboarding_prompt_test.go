package handler

import (
	"strings"
	"testing"
)

func TestNormalizeOnboardingAgentRequest(t *testing.T) {
	for _, tc := range []struct{ name, template, instructions, wantName, wantInstructions string }{
		{"Sili Agent", onboardingAgentTemplate, "You are Sili Agent, the built-in AI assistant for this MEPERStudio workspace.\nRun `silieco --help`; use `silieco <command> --help`. Keep builtin:silieco-working-on-tasks and ~/.silieco/config.", "MEPER Agent", "You are MEPER Agent, the built-in AI assistant for this MEPERStudio workspace.\nRun `meperstudio --help`; use `meperstudio <command> --help`. Keep builtin:silieco-working-on-tasks and ~/.silieco/config."},
		{"MEPER Agent", onboardingAgentTemplate, "你是 MEPER Agent，这个 MEPERStudio workspace 内置的 AI 助手。\n你的工具箱是 `silieco` CLI。先跑 `silieco --help`。", "MEPER Agent", "你是 MEPER Agent，这个 MEPERStudio workspace 内置的 AI 助手。\n你的工具箱是 `meperstudio` CLI。先跑 `meperstudio --help`。"},
		{"MEPER Agent", onboardingAgentTemplate, "Custom prompt: use silieco for legacy project.", "MEPER Agent", "Custom prompt: use silieco for legacy project."},
		{"Custom agent", "", "You are MEPER Agent, the built-in AI assistant for this MEPERStudio workspace. Use `silieco --help`.", "Custom agent", "You are MEPER Agent, the built-in AI assistant for this MEPERStudio workspace. Use `silieco --help`."},
	} {
		req := CreateAgentRequest{Name: tc.name, Template: tc.template, Instructions: tc.instructions}
		normalizeOnboardingAgentRequest(&req)
		if req.Name != tc.wantName || req.Instructions != tc.wantInstructions {
			t.Fatalf("unexpected normalized request: %q %q", req.Name, req.Instructions)
		}
	}
	if strings.Contains(onboardingAssistantInstructions, "silieco") || !strings.Contains(onboardingAssistantInstructions, "`meperstudio --help`") || strings.Contains(onboardingAssistantInstructions, "authenticated as the workspace owner") {
		t.Fatal("legacy shim prompt is stale")
	}
}
