-- Project resources remain authoritative. These rows are a searchable projection,
-- with explicit application-layer cleanup matching the existing relational model.
CREATE TABLE project_repo_binding (
    resource_id UUID PRIMARY KEY,
    workspace_id UUID NOT NULL,
    project_id UUID NOT NULL,
    clone_key TEXT,
    clone_host TEXT,
    clone_path TEXT,
    connection_kind TEXT CHECK (connection_kind IN ('vcs', 'github_app')),
    connection_id UUID,
    repository_path TEXT,
    external_repo_id TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK ((connection_kind IS NULL) = (connection_id IS NULL))
);
CREATE INDEX idx_project_repo_binding_project ON project_repo_binding (workspace_id, project_id);
CREATE INDEX idx_project_repo_binding_path ON project_repo_binding (workspace_id, connection_kind, connection_id, repository_path);
CREATE INDEX idx_project_repo_binding_external ON project_repo_binding (workspace_id, connection_kind, connection_id, external_repo_id);
-- Provider IDs are additive, legacy mirrors keep their existing path identity.
ALTER TABLE vcs_pull_request ADD COLUMN external_repo_id TEXT;
ALTER TABLE github_pull_request ADD COLUMN external_repo_id TEXT;
CREATE INDEX idx_vcs_pr_repository_id ON vcs_pull_request (connection_id, external_repo_id);
CREATE INDEX idx_github_pr_repository_id ON github_pull_request (workspace_id, installation_id, external_repo_id);
-- Do not attribute the legacy connection+SHA statuses to repositories by guess.
CREATE TABLE vcs_repo_commit_status (
    connection_id UUID NOT NULL,
    repository_path TEXT NOT NULL,
    sha TEXT NOT NULL,
    context TEXT NOT NULL,
    state TEXT NOT NULL CHECK (state IN ('passed', 'failed', 'pending')),
    target_url TEXT,
    description TEXT,
    updated_at TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (connection_id, repository_path, sha, context)
);
