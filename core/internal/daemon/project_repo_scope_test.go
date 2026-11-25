package daemon

import "testing"

func TestRunRepositoryScope(t *testing.T) {
	d := &Daemon{workspaces: map[string]*workspaceState{"w": {}}}
	d.setRunRepoScope("w", "run-a", []RepoData{{URL: "https://git.example.com/g/a", Ref: "develop"}})
	d.setRunRepoScope("w", "run-b", []RepoData{})
	if !d.runRepoScopeAllowed("w", "run-a", "https://git.example.com/g/a") {
		t.Fatal("configured repository denied")
	}
	if d.runRepoScopeAllowed("w", "run-a", "https://git.example.com/g/b") {
		t.Fatal("unconfigured repository permitted")
	}
	if d.runRepoScopeAllowed("w", "run-b", "https://git.example.com/g/a") {
		t.Fatal("empty explicit scope must stay empty")
	}
	d.workspaces["w"] = &workspaceState{}
	if d.runRepoScopeAllowed("w", "run-b", "https://git.example.com/g/a") {
		t.Fatal("workspace refresh must not widen a scope")
	}
	if ref := d.runRepoDefaultRef("w", "run-a", "https://git.example.com/g/a"); ref != "develop" {
		t.Fatalf("lost run ref: %s", ref)
	}
}
