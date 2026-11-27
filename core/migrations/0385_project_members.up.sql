-- Workspace membership remains the prerequisite. Project grants belong to a
-- membership (not just a user), so removal/re-invitation cannot restore access.
CREATE TABLE project_member (
    project_id uuid NOT NULL REFERENCES project(id) ON DELETE CASCADE,
    member_id uuid NOT NULL REFERENCES member(id) ON DELETE CASCADE,
    PRIMARY KEY (project_id, member_id)
);
CREATE INDEX project_member_member_idx ON project_member(member_id);
-- Preserve access to existing projects for existing employees on upgrade.
INSERT INTO project_member (project_id, member_id)
SELECT p.id, m.id FROM project p JOIN member m ON m.workspace_id = p.workspace_id
WHERE m.role NOT IN ('owner','admin')
ON CONFLICT DO NOTHING;

CREATE FUNCTION project_attachment_accessible(uuid, uuid, uuid) RETURNS boolean
LANGUAGE sql STABLE AS $$
WITH accessible AS (
        SELECT p.id FROM project p JOIN member m ON m.workspace_id=p.workspace_id
        WHERE p.workspace_id=$1 AND m.user_id=$2
          AND (m.role IN ('owner','admin') OR EXISTS(SELECT 1 FROM project_member pm WHERE pm.project_id=p.id AND pm.member_id=m.id))
    ), projects AS (
        SELECT t.project_id FROM attachment a JOIN task t ON t.id=a.task_id AND t.workspace_id=a.workspace_id WHERE a.id=$3 AND a.workspace_id=$1
        UNION
        SELECT c.project_id FROM attachment a JOIN chat_session c ON c.id=a.chat_session_id AND c.workspace_id=a.workspace_id WHERE a.id=$3 AND a.workspace_id=$1
        UNION
        SELECT CASE WHEN ar.target_type='project' THEN ar.target_id ELSE t.project_id END FROM attachment_reference ar LEFT JOIN task t ON ar.target_type='task' AND ar.target_id=t.id AND t.workspace_id=ar.workspace_id
        WHERE ar.attachment_id=$3 AND ar.workspace_id=$1
    )
    SELECT NOT EXISTS(SELECT 1 FROM projects WHERE project_id IS NOT NULL AND project_id NOT IN (SELECT id FROM accessible));
$$;
