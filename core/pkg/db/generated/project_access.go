package db

import (
	"context"
	"github.com/jackc/pgx/v5/pgtype"
)

// ListAccessibleProjectIDs returns an explicit set even for administrators.
// Joining membership and project prevents cross-workspace grants from granting access.
func (q *Queries) ListAccessibleProjectIDs(ctx context.Context, workspaceID, userID pgtype.UUID) ([]pgtype.UUID, error) {
	rows, err := q.db.Query(ctx, `SELECT p.id FROM project p
        JOIN member m ON m.workspace_id = p.workspace_id
        WHERE p.workspace_id = $1 AND m.user_id = $2
        AND (m.role IN ('owner','admin') OR EXISTS (
            SELECT 1 FROM project_member pm WHERE pm.project_id = p.id AND pm.member_id = m.id
        ))`, workspaceID, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	ids := make([]pgtype.UUID, 0)
	for rows.Next() {
		var id pgtype.UUID
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

func (q *Queries) CanAccessProject(ctx context.Context, workspaceID, userID, projectID pgtype.UUID) (bool, error) {
	if !projectID.Valid {
		return true, nil
	}
	var allowed bool
	err := q.db.QueryRow(ctx, `SELECT EXISTS (
        SELECT 1 FROM project p JOIN member m ON m.workspace_id = p.workspace_id
        WHERE p.id = $3 AND p.workspace_id = $1 AND m.user_id = $2
        AND (m.role IN ('owner','admin') OR EXISTS (
            SELECT 1 FROM project_member pm WHERE pm.project_id = p.id AND pm.member_id = m.id
        )))`, workspaceID, userID, projectID).Scan(&allowed)
	return allowed, err
}

func (q *Queries) ListProjectMemberUserIDs(ctx context.Context, workspaceID, projectID pgtype.UUID) ([]pgtype.UUID, error) {
	rows, err := q.db.Query(ctx, `SELECT m.user_id FROM member m JOIN project_member pm ON pm.member_id=m.id
        JOIN project p ON p.id=pm.project_id AND p.workspace_id=m.workspace_id
        WHERE p.workspace_id=$1 AND p.id=$2 ORDER BY m.user_id`, workspaceID, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	ids := make([]pgtype.UUID, 0)
	for rows.Next() {
		var id pgtype.UUID
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

func (q *Queries) GrantProjectMember(ctx context.Context, workspaceID, projectID, userID pgtype.UUID) error {
	_, err := q.db.Exec(ctx, `INSERT INTO project_member (project_id,member_id)
        SELECT p.id,m.id FROM project p JOIN member m ON m.workspace_id=p.workspace_id
        WHERE p.workspace_id=$1 AND p.id=$2 AND m.user_id=$3 ON CONFLICT DO NOTHING`, workspaceID, projectID, userID)
	return err
}

func (q *Queries) RevokeProjectMember(ctx context.Context, workspaceID, projectID, userID pgtype.UUID) error {
	_, err := q.db.Exec(ctx, `DELETE FROM project_member pm USING member m, project p
        WHERE pm.member_id=m.id AND pm.project_id=p.id AND p.workspace_id=m.workspace_id
          AND p.workspace_id=$1 AND p.id=$2 AND m.user_id=$3`, workspaceID, projectID, userID)
	return err
}

// Attachment access follows its original task/chat and every project-backed
// reference. This deliberately prevents a reference in a shared task from
// exposing a file whose original project is restricted.
func (q *Queries) CanAccessAttachment(ctx context.Context, workspaceID, userID, attachmentID pgtype.UUID) (bool, error) {
	var allowed bool
	err := q.db.QueryRow(ctx, `SELECT project_attachment_accessible($1,$2,$3)`, workspaceID, userID, attachmentID).Scan(&allowed)
	return allowed, err
}

// ListProjectAudience returns the current recipients of a project-scoped event
// in one query (including workspace administrators). A NULL project denotes
// a workspace-level resource.
func (q *Queries) ListProjectAudience(ctx context.Context, workspaceID, projectID pgtype.UUID) ([]pgtype.UUID, error) {
	rows, err := q.db.Query(ctx, `SELECT m.user_id FROM member m WHERE m.workspace_id=$1 AND
        ($2::uuid IS NULL OR EXISTS (SELECT 1 FROM project p WHERE p.id=$2 AND p.workspace_id=$1 AND
        (m.role IN ('owner','admin') OR EXISTS(SELECT 1 FROM project_member pm WHERE pm.project_id=p.id AND pm.member_id=m.id))))`, workspaceID, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]pgtype.UUID, 0)
	for rows.Next() {
		var id pgtype.UUID
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}
