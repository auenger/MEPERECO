package daemon

import (
	"encoding/json"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// Chat claims use the same project-resource directory resolver as task claims.
func TestProjectChatWorkingDirectory(t *testing.T) {
	dir := t.TempDir()
	ref, err := json.Marshal(localDirectoryRef{LocalPath: dir, DaemonID: "local-test", ExecutionMode: "in_place"})
	if err != nil {
		t.Fatal(err)
	}
	run := AgentRun{ChatSessionID: "group-chat", ProjectID: "bound-project", ProjectResources: []ProjectResourceData{{ResourceType: "local_directory", ResourceRef: ref}}}
	assignment, err := localDirectoryAssignmentForRun(run, "local-test")
	if err != nil || assignment == nil {
		t.Fatalf("missing project directory: %v", err)
	}
	cmd := exec.Command("pwd")
	cmd.Dir = assignment.AbsPath
	out, err := cmd.Output()
	if err != nil {
		t.Fatal(err)
	}
	actual, err := filepath.EvalSymlinks(strings.TrimSpace(string(out)))
	if err != nil {
		t.Fatal(err)
	}
	want, err := filepath.EvalSymlinks(dir)
	if err != nil {
		t.Fatal(err)
	}
	if actual != want {
		t.Fatalf("group chat pwd = %q, want %q", actual, want)
	}
	unrelated, err := localDirectoryAssignmentForRun(run, "other-daemon")
	if err != nil || unrelated != nil {
		t.Fatalf("directory belonging to another runtime must not be selected: %v %v", unrelated, err)
	}
}
