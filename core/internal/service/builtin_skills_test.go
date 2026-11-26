package service

import (
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
)

func TestDefaultBuiltinSkillsAndCatalog(t *testing.T) {
	svc := &AgentRunService{}
	defaults := svc.DefaultBuiltinSkills()
	if len(defaults) != 3 {
		t.Fatalf("default skills = %d, want 3", len(defaults))
	}
	for _, skill := range defaults {
		switch skill.Name {
		case "silieco-mentioning", "silieco-projects-and-resources", "silieco-working-on-tasks":
		default:
			t.Fatalf("unexpected default skill %q", skill.Name)
		}
	}
	catalog := svc.BuiltinSkillCatalog()
	if len(catalog) != len(svc.BuiltinSkills()) || len(catalog) <= len(defaults) {
		t.Fatalf("incomplete optional catalog: %d entries", len(catalog))
	}
	skill, ok := svc.BuiltinSkillByID("builtin:silieco-new-feature")
	if !ok || skill.Content == "" || skill.Description == "" {
		t.Fatal("optional feature skill cannot be loaded on demand")
	}
	if _, ok := svc.BuiltinSkillByID("builtin:../unknown"); ok {
		t.Fatal("unknown builtin resolved")
	}
	resolved, missing := svc.ResolveRequiredSkills(t.Context(), pgtype.UUID{}, []string{"builtin:silieco-new-feature"})
	if len(resolved) != 1 || len(missing) != 0 {
		t.Fatalf("explicit SOP builtin not resolved: %v", missing)
	}
}

func TestMergeRunSkillsPrefersFrozenStage(t *testing.T) {
	base := []AgentSkillData{{Name: "base", Content: "base"}, {ID: "workspace-id", Name: "shared", Content: "live"}}
	required := []AgentSkillData{{ID: "workspace-id", Name: "original-name", Content: "snapshot"}, {Name: "stage-only", Content: "stage"}}
	merged := MergeRunSkills(base, required)
	if len(merged) != 3 || merged[1].Content != "snapshot" || merged[1].Name != "original-name" || merged[2].Name != "stage-only" {
		t.Fatalf("unexpected run skills: %#v", merged)
	}
	bundles, refs := BuildAgentSkillBundles(merged)
	if len(bundles) != len(refs) || bundles[1].Hash != refs[1].Hash || bundles[1].Content != "snapshot" {
		t.Fatal("slim refs do not match the frozen stage bundle")
	}
	if base[1].Content != "live" {
		t.Fatal("merge mutated the agent's live configuration")
	}
}
