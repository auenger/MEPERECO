-- Existing projects and older clients retain the historical fallback behavior.
ALTER TABLE project ADD COLUMN repo_mode TEXT NOT NULL DEFAULT 'inherit'
    CHECK (repo_mode IN ('inherit', 'project_only', 'none'));
