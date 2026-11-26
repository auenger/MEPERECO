package execenv

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRuntimeSkillBriefRefresh(t *testing.T) {
	for _, provider := range []string{"claude", "codex"} {
		t.Run(provider, func(t *testing.T) {
			workDir := t.TempDir()
			path := runtimeConfigPath(workDir, provider)
			userText := "# User instructions\nKeep this.\n"
			if err := os.WriteFile(path, []byte(userText), 0o644); err != nil {
				t.Fatal(err)
			}
			ctx := RunContextForEnv{AgentName: "Test", AgentSkills: []SkillContextForEnv{{Name: "deleted-thermal-skill", Content: "old"}}}
			if _, err := InjectRuntimeConfig(workDir, provider, ctx); err != nil {
				t.Fatal(err)
			}
			ctx.AgentSkills = []SkillContextForEnv{{Name: "current-skill", Content: "new"}}
			if _, err := InjectRuntimeConfig(workDir, provider, ctx); err != nil {
				t.Fatal(err)
			}
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			text := string(data)
			if strings.Contains(text, "deleted-thermal-skill") || !strings.Contains(text, "current-skill") || !strings.Contains(text, "MEPERStudio Agent Runtime") || !strings.Contains(text, "builtin:catalog") || !strings.HasPrefix(text, userText) || strings.Count(text, runtimeMarkerBegin) != 1 {
				t.Fatalf("incorrectly refreshed %s brief", provider)
			}
		})
	}
}

func TestClaudeManagedSkillRefreshPreservesUserSkills(t *testing.T) {
	root := t.TempDir()
	workDir := filepath.Join(root, "workdir")
	userSkill := filepath.Join(skillsDirPath(workDir, "claude"), "user-owned", "SKILL.md")
	if err := os.MkdirAll(filepath.Dir(userSkill), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(userSkill, []byte("user skill"), 0o644); err != nil {
		t.Fatal(err)
	}
	manifest := &sidecarManifest{}
	ctx := RunContextForEnv{AgentName: "Test", AgentSkills: []SkillContextForEnv{{Name: "deleted-thermal-skill", Content: "old"}}}
	if err := writeContextFiles(workDir, "claude", ctx, manifest); err != nil {
		t.Fatal(err)
	}
	if err := writeSidecarManifest(root, manifest); err != nil {
		t.Fatal(err)
	}
	oldDir := filepath.Join(skillsDirPath(workDir, "claude"), "deleted-thermal-skill")
	if _, err := os.Stat(filepath.Join(oldDir, "SKILL.md")); err != nil {
		t.Fatal(err)
	}
	if err := removeReusedManagedSkillDirs(root, skillsDirPath(workDir, "claude")); err != nil {
		t.Fatal(err)
	}
	if err := CleanupSidecars(root); err != nil {
		t.Fatal(err)
	}
	ctx.AgentSkills = []SkillContextForEnv{{Name: "current-skill", Content: "new"}}
	if err := writeContextFiles(workDir, "claude", ctx, &sidecarManifest{}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(oldDir); !os.IsNotExist(err) {
		t.Fatalf("deleted skill directory survived refresh: %v", err)
	}
	if _, err := os.Stat(filepath.Join(skillsDirPath(workDir, "claude"), "current-skill", "SKILL.md")); err != nil {
		t.Fatal(err)
	}
	if data, err := os.ReadFile(userSkill); err != nil || string(data) != "user skill" {
		t.Fatalf("user skill was modified: %v", err)
	}
}
