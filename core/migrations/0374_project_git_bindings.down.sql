DROP TABLE IF EXISTS vcs_repo_commit_status;
DROP INDEX IF EXISTS idx_github_pr_repository_id;
DROP INDEX IF EXISTS idx_vcs_pr_repository_id;
ALTER TABLE github_pull_request DROP COLUMN IF EXISTS external_repo_id;
ALTER TABLE vcs_pull_request DROP COLUMN IF EXISTS external_repo_id;
DROP TABLE IF EXISTS project_repo_binding;
