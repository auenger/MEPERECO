package agent

import (
	"context"
	"strings"
	"testing"
)

func TestRuntimeRegistry(t *testing.T) {
	seen := map[string]bool{}
	for _, d := range RuntimeDescriptors() {
		if seen[d.ID] || d.Command == "" || d.ContextFile == "" || d.LocalSkillPath == "" || d.ProjectSkillPath == "" || d.LaunchHeader == "" {
			t.Fatalf("incomplete descriptor: %s", d.ID)
		}
		seen[d.ID] = true
		backend, err := New(d.ID, Config{})
		if err != nil || backend == nil {
			t.Fatalf("%s: %v", d.ID, err)
		}
		if !IsSupportedType(d.ID) || LaunchHeader(d.ID) != d.LaunchHeader || ModelSelectionSupported(d.ID) != d.Capabilities.ModelSelection {
			t.Fatalf("registry mismatch: %s", d.ID)
		}
	}
	if len(RuntimeCatalog()) != len(seen) {
		t.Fatal("catalog mismatch")
	}
	if _, err := New("unknown", Config{}); err == nil || !strings.Contains(err.Error(), "zcode") {
		t.Fatal(err)
	}
	if IsSupportedType("unknown") {
		t.Fatal("unknown runtime was registered")
	}
}

// A synthetic adapter registers all generic consumers through one descriptor.
// It does not execute an external program or mutate the production registry.
func TestRegisterAdditionalRuntime(t *testing.T) {
	d := RuntimeDescriptor{ID: "fixture", Command: "fixture-cli", DisplayName: "Fixture", LaunchHeader: "fixture protocol", ContextFile: "AGENTS.md", LocalSkillPath: ".fixture/skills", ProjectSkillPath: ".fixture/skills", Capabilities: RuntimeCapabilities{ModelSelection: true}, factory: func(cfg Config) Backend { return &claudeBackend{cfg: cfg} }, listModels: func(context.Context, string, string) ([]Model, error) { return []Model{{ID: "fixture-model"}}, nil }, blockedArgs: map[string]blockedArgMode{"--protocol": blockedWithValue}}
	descriptors := append(RuntimeDescriptors(), d)
	indexed := indexRuntimeDescriptors(descriptors)
	actual := indexed[d.ID]
	b := actual.newBackend(Config{LaunchPrefix: []string{"wrapper", "--protocol", "test"}}).(*claudeBackend)
	if len(b.cfg.LaunchPrefix) != 1 || b.cfg.LaunchPrefix[0] != "wrapper" {
		t.Fatal(b.cfg.LaunchPrefix)
	}
	models, err := actual.listModels(context.Background(), d.ID, d.Command)
	if err != nil || len(models) != 1 || models[0].ID != "fixture-model" || !actual.Capabilities.ModelSelection {
		t.Fatal(models, err)
	}
	if IsSupportedType(d.ID) {
		t.Fatal("test registry leaked into production registry")
	}
}
