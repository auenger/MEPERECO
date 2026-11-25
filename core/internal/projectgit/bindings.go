// Package projectgit maintains the searchable project repository projection.
package projectgit

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/silieco-ai/silieco/core/internal/integrations/vcs"
	db "github.com/silieco-ai/silieco/core/pkg/db/generated"
)

type Integration struct {
	Kind           string `json:"kind"`
	ConnectionID   string `json:"connection_id"`
	RepositoryPath string `json:"repository_path"`
	ExternalRepoID string `json:"external_repo_id,omitempty"`
}
type Ref struct {
	URL                 string       `json:"url"`
	DefaultBranchHint   string       `json:"default_branch_hint,omitempty"`
	Ref                 string       `json:"ref,omitempty"`
	Primary             bool         `json:"primary,omitempty"`
	Integration         *Integration `json:"integration,omitempty"`
	IntegrationDisabled bool         `json:"integration_disabled,omitempty"`
}

// ValidateIntegration verifies workspace ownership and, when present, the
// external ID against a mirrored repository. Path-based bindings may wait for
// their first event; they are not represented as verified external IDs.
func ValidateIntegration(ctx context.Context, tx db.DBTX, ws pgtype.UUID, in *Integration) error {
	if in == nil {
		return nil
	}
	var id pgtype.UUID
	if err := id.Scan(in.ConnectionID); err != nil || !id.Valid {
		return errors.New("invalid Git connection ID")
	}
	path, err := vcs.NormalizeRepositoryPath(in.RepositoryPath)
	if err != nil {
		return err
	}
	in.RepositoryPath = path
	var found bool
	switch in.Kind {
	case "vcs":
		err = tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM vcs_connection WHERE id=$1 AND workspace_id=$2)`, id, ws).Scan(&found)
	case "github_app":
		err = tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM github_installation WHERE id=$1 AND workspace_id=$2)`, id, ws).Scan(&found)
	default:
		return errors.New("unsupported Git integration kind")
	}
	if err != nil {
		return err
	}
	if !found {
		return errors.New("Git connection does not belong to this workspace")
	}
	// Only retain a stable identity when a trusted provider mirror confirms it.
	pathParts := strings.Split(path, "/")
	owner, name := strings.Join(pathParts[:len(pathParts)-1], "/"), pathParts[len(pathParts)-1]
	if in.ExternalRepoID == "" {
		var identities int64
		var observed string
		if in.Kind == "vcs" {
			err = tx.QueryRow(ctx, `SELECT count(DISTINCT external_repo_id),coalesce(max(external_repo_id),'') FROM vcs_pull_request WHERE workspace_id=$1 AND connection_id=$2 AND repo_owner=$3 AND repo_name=$4`, ws, id, owner, name).Scan(&identities, &observed)
		} else {
			err = tx.QueryRow(ctx, `SELECT count(DISTINCT p.external_repo_id),coalesce(max(p.external_repo_id),'') FROM github_pull_request p JOIN github_installation i ON p.installation_id=i.installation_id AND p.workspace_id=i.workspace_id WHERE i.id=$2 AND p.workspace_id=$1 AND lower(p.repo_owner)=lower($3) AND lower(p.repo_name)=lower($4)`, ws, id, owner, name).Scan(&identities, &observed)
		}
		if err != nil {
			return err
		}
		if identities > 1 {
			return errors.New("ambiguous repository identity; confirm the provider repository ID")
		}
		if identities == 1 {
			in.ExternalRepoID = observed
		}
		return nil
	}

	if in.Kind == "vcs" {
		err = tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM vcs_pull_request WHERE workspace_id=$1 AND connection_id=$2 AND repo_owner=$3 AND repo_name=$4 AND external_repo_id=$5)`, ws, id, owner, name, in.ExternalRepoID).Scan(&found)
	} else {
		err = tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM github_pull_request p JOIN github_installation i ON p.installation_id=i.installation_id AND p.workspace_id=i.workspace_id WHERE i.id=$2 AND p.workspace_id=$1 AND lower(p.repo_owner)=lower($3) AND lower(p.repo_name)=lower($4) AND p.external_repo_id=$5)`, ws, id, owner, name, in.ExternalRepoID).Scan(&found)
	}
	if err != nil {
		return err
	}
	if !found {
		return errors.New("repository ID has not been verified by this connection")
	}
	return nil
}

func LockProject(ctx context.Context, tx pgx.Tx, ws, project pgtype.UUID) error {
	var found pgtype.UUID
	// Match aggregate deletion lock order: workspace then project.
	if err := tx.QueryRow(ctx, `SELECT id FROM workspace WHERE id=$1 FOR KEY SHARE`, ws).Scan(&found); err != nil {
		return err
	}
	return tx.QueryRow(ctx, `SELECT id FROM project WHERE id=$1 AND workspace_id=$2 FOR UPDATE`, project, ws).Scan(&found)
}

// SameRepository never equates an SSH endpoint to an HTTP API endpoint solely
// from hostname. Explicit platform mappings take precedence.
func SameRepository(a, b Ref) bool {
	if a.URL == b.URL {
		return true
	}
	if a.Integration != nil && b.Integration != nil && a.Integration.Kind == b.Integration.Kind && a.Integration.ConnectionID == b.Integration.ConnectionID {
		if a.Integration.ExternalRepoID != "" && b.Integration.ExternalRepoID != "" {
			return a.Integration.ExternalRepoID == b.Integration.ExternalRepoID
		}
		if a.Integration.Kind == "github_app" {
			return strings.EqualFold(a.Integration.RepositoryPath, b.Integration.RepositoryPath)
		}
		return a.Integration.RepositoryPath == b.Integration.RepositoryPath
	}
	x, ex := vcs.ParseCloneURL(a.URL)
	y, ey := vcs.ParseCloneURL(b.URL)
	return ex == nil && ey == nil && x.Key() == y.Key()
}

// SyncProject rebuilds just one aggregate in the caller's transaction after
// its resource changes. The project row is locked by the caller, including
// backfill. Keep disconnected mappings and malformed legacy clone locators.
func SyncProject(ctx context.Context, tx pgx.Tx, ws, project pgtype.UUID) error {
	rows, err := db.New(tx).ListProjectResources(ctx, project)
	if err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `DELETE FROM project_repo_binding b WHERE b.workspace_id=$1 AND b.project_id=$2 AND NOT EXISTS (SELECT 1 FROM project_resource r WHERE r.id=b.resource_id AND r.project_id=b.project_id AND r.workspace_id=b.workspace_id AND r.resource_type='github_repo')`, ws, project); err != nil {
		return err
	}
	for _, row := range rows {
		if row.ResourceType != "github_repo" || row.WorkspaceID != ws {
			continue
		}
		var ref Ref
		var key, host, path, kind, repoPath, external any
		var connection pgtype.UUID
		if json.Unmarshal(row.ResourceRef, &ref) == nil {
			if loc, e := vcs.ParseCloneURL(ref.URL); e == nil {
				key, host, path = loc.Key(), loc.Host, loc.Path
			}
			if in := ref.Integration; in != nil && (in.Kind == "vcs" || in.Kind == "github_app") {
				if e := connection.Scan(in.ConnectionID); e == nil && connection.Valid {
					kind, repoPath = in.Kind, in.RepositoryPath
					if in.ExternalRepoID != "" {
						external = in.ExternalRepoID
					}
				} else {
					connection = pgtype.UUID{}
				}
			}
		}
		_, err = tx.Exec(ctx, `INSERT INTO project_repo_binding(resource_id,workspace_id,project_id,clone_key,clone_host,clone_path,connection_kind,connection_id,repository_path,external_repo_id)
			VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
			ON CONFLICT(resource_id) DO UPDATE SET workspace_id=EXCLUDED.workspace_id,project_id=EXCLUDED.project_id,clone_key=EXCLUDED.clone_key,clone_host=EXCLUDED.clone_host,clone_path=EXCLUDED.clone_path,connection_kind=EXCLUDED.connection_kind,connection_id=EXCLUDED.connection_id,repository_path=EXCLUDED.repository_path,external_repo_id=EXCLUDED.external_repo_id,updated_at=now()`, row.ID, ws, project, key, host, path, kind, connection, repoPath, external)
		if err != nil {
			return fmt.Errorf("sync repository binding: %w", err)
		}
	}
	return nil
}

// ResolveObservedIntegration maps a locator only when a unique connection
// has already mirrored that exact repository. It never contacts a remote or
// guesses a self-hosted SSH API endpoint.
func ResolveObservedIntegration(ctx context.Context, tx db.DBTX, ws pgtype.UUID, rawURL string) (*Integration, error) {
	loc, err := vcs.ParseCloneURL(rawURL)
	if err != nil {
		return nil, nil
	}
	q := db.New(tx)
	conns, err := q.ListVCSConnectionsByWorkspace(ctx, ws)
	if err != nil {
		return nil, err
	}
	var candidates []Integration
	for _, c := range conns {
		path, ok := vcs.MatchInstance(loc, c.InstanceUrl)
		if !ok {
			continue
		}
		var found bool
		var identity string
		err = tx.QueryRow(ctx, `SELECT count(*)>0 AND count(DISTINCT external_repo_id)<=1, CASE WHEN count(DISTINCT external_repo_id)=1 THEN coalesce(max(external_repo_id),'') ELSE '' END FROM vcs_pull_request WHERE workspace_id=$1 AND connection_id=$2 AND repo_owner||'/'||repo_name=$3`, ws, c.ID, path).Scan(&found, &identity)
		if err != nil {
			return nil, err
		}
		if found {
			candidates = append(candidates, Integration{Kind: "vcs", ConnectionID: uuidString(c.ID), RepositoryPath: path, ExternalRepoID: identity})
		}
	}
	if loc.Host == "github.com" && (loc.Scheme == "https" || loc.Scheme == "ssh") {
		insts, err := q.ListGitHubInstallationsByWorkspace(ctx, ws)
		if err != nil {
			return nil, err
		}
		for _, inst := range insts {
			var found bool
			var identity string
			err = tx.QueryRow(ctx, `SELECT count(*)>0 AND count(DISTINCT external_repo_id)<=1, CASE WHEN count(DISTINCT external_repo_id)=1 THEN coalesce(max(external_repo_id),'') ELSE '' END FROM github_pull_request WHERE workspace_id=$1 AND installation_id=$2 AND lower(repo_owner||'/'||repo_name)=lower($3)`, ws, inst.InstallationID, loc.Path).Scan(&found, &identity)
			if err != nil {
				return nil, err
			}
			if found {
				candidates = append(candidates, Integration{Kind: "github_app", ConnectionID: uuidString(inst.ID), RepositoryPath: loc.Path, ExternalRepoID: identity})
			}
		}
	}
	if len(candidates) != 1 {
		return nil, nil
	}
	return &candidates[0], nil
}

func uuidString(id pgtype.UUID) string {
	return fmt.Sprintf("%x-%x-%x-%x-%x", id.Bytes[0:4], id.Bytes[4:6], id.Bytes[6:8], id.Bytes[8:10], id.Bytes[10:16])
}

// PrepareRef is called only for accepted Git resource writes. Old clients that
// omit integration keep the existing mapping unless the clone URL changes.
func PrepareRef(ctx context.Context, tx db.DBTX, ws pgtype.UUID, next, previous json.RawMessage, integrationProvided bool, autoResolve bool) (json.RawMessage, error) {
	var incoming Ref
	if err := json.Unmarshal(next, &incoming); err != nil {
		return nil, err
	}
	var old Ref
	_ = json.Unmarshal(previous, &old)
	if integrationProvided {
		incoming.IntegrationDisabled = incoming.Integration == nil
	} else if incoming.URL == old.URL {
		incoming.IntegrationDisabled = old.IntegrationDisabled
	} else {
		incoming.IntegrationDisabled = false
	}
	if !integrationProvided && incoming.URL == old.URL && old.Integration != nil {
		incoming.Integration = old.Integration
	} else if integrationProvided && incoming.Integration != nil {
		// Unchanged disconnected mappings can be edited by legacy clients.
		if old.Integration == nil || *old.Integration != *incoming.Integration {
			if err := ValidateIntegration(ctx, tx, ws, incoming.Integration); err != nil {
				return nil, err
			}
		}
	} else if !integrationProvided && autoResolve && !incoming.IntegrationDisabled {
		resolved, err := ResolveObservedIntegration(ctx, tx, ws, incoming.URL)
		if err != nil {
			return nil, err
		}
		incoming.Integration = resolved
	}
	return json.Marshal(incoming)
}
