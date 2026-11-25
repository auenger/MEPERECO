package daemon

import "strings"

type runRepoScope struct {
	workspaceID string
	refs        map[string]string
}

// An explicit scope may be empty. Keep it independently from the refreshed
// workspace state so re-registration cannot widen a live run's scope.
func (d *Daemon) setRunRepoScope(workspaceID, runID string, repos []RepoData) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.runRepoScopes == nil {
		d.runRepoScopes = make(map[string]runRepoScope)
	}
	allowed := make(map[string]string)
	for _, repo := range repos {
		if u := strings.TrimSpace(repo.URL); u != "" {
			allowed[u] = strings.TrimSpace(repo.Ref)
		}
	}
	d.runRepoScopes[runID] = runRepoScope{workspaceID: workspaceID, refs: allowed}
}
func (d *Daemon) runRepoScopeAllowed(workspaceID, runID, repoURL string) bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	scope, scoped := d.runRepoScopes[runID]
	if !scoped {
		return true
	} // legacy server claim (workspace compatibility)
	if scope.workspaceID != workspaceID {
		return false
	}
	_, allowed := scope.refs[strings.TrimSpace(repoURL)]
	return allowed
}
