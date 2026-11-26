// Idempotent operator-invoked backfill. Never runs during normal server boot.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/auenger/MEPERECO/core/internal/integrations/vcs"
	"github.com/auenger/MEPERECO/core/internal/projectgit"
	db "github.com/auenger/MEPERECO/core/pkg/db/generated"
)

func main() {
	after := flag.String("after", "", "resume after this project UUID (exclusive)")
	batch := flag.Int("batch", 100, "projects per batch")
	flag.Parse()
	if *batch < 1 || *batch > 1000 {
		log.Fatal("batch must be 1..1000")
	}
	var cursor pgtype.UUID
	if *after != "" {
		if err := cursor.Scan(*after); err != nil {
			log.Fatal(err)
		}
	}
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		log.Fatal("DATABASE_URL must be explicitly set")
	}
	ctx := context.Background()
	pool, e := pgxpool.New(ctx, databaseURL)
	if e != nil {
		log.Fatal(e)
	}
	defer pool.Close()
	conn, e := pool.Acquire(ctx)
	if e != nil {
		log.Fatal(e)
	}
	defer conn.Release()
	const lockKey int64 = 7244554146635925502
	var locked bool
	if e = conn.QueryRow(ctx, "SELECT pg_try_advisory_lock($1)", lockKey).Scan(&locked); e != nil || !locked {
		log.Fatal("another Git binding backfill is running or lock unavailable")
	}
	defer conn.Exec(ctx, "SELECT pg_advisory_unlock($1)", lockKey)
	total, repositories, resolved, unresolved, invalid, duplicates, primaryRepairs := 0, 0, 0, 0, 0, 0, 0
	for {
		rows, e := conn.Query(ctx, `SELECT id,workspace_id FROM project WHERE ($1::uuid IS NULL OR id>$1) ORDER BY id LIMIT $2`, cursor, *batch)
		if e != nil {
			log.Fatal(e)
		}
		type target struct{ id, ws pgtype.UUID }
		var list []target
		for rows.Next() {
			var t target
			if e = rows.Scan(&t.id, &t.ws); e != nil {
				rows.Close()
				log.Fatal(e)
			}
			list = append(list, t)
		}
		e = rows.Err()
		rows.Close()
		if e != nil {
			log.Fatal(e)
		}
		if len(list) == 0 {
			break
		}
		for _, t := range list {
			tx, e := conn.Begin(ctx)
			if e != nil {
				log.Fatal(e)
			}
			if e = projectgit.LockProject(ctx, tx, t.ws, t.id); e != nil {
				tx.Rollback(ctx)
				if errors.Is(e, pgx.ErrNoRows) {
					cursor = t.id
					continue
				}
				log.Fatalf("project %v: %v", t.id, e)
			}
			q := db.New(tx)
			resources, e := q.ListProjectResources(ctx, t.id)
			if e != nil {
				tx.Rollback(ctx)
				log.Fatal(e)
			}
			// Inspect all legacy resources first. Retain duplicate or unparseable
			// repositories for operator review; repair only deterministic primary.
			type candidate struct {
				row    db.ProjectResource
				ref    projectgit.Ref
				fields map[string]json.RawMessage
			}
			var candidates []candidate
			primaryIndex := -1
			projectDuplicates := false
			for _, r := range resources {
				if r.ResourceType != "github_repo" || r.WorkspaceID != t.ws {
					continue
				}
				repositories++
				var ref projectgit.Ref
				var fields map[string]json.RawMessage
				if json.Unmarshal(r.ResourceRef, &ref) != nil || json.Unmarshal(r.ResourceRef, &fields) != nil || fields == nil {
					invalid++
					continue
				}
				if _, err := vcs.ParseCloneURL(ref.URL); err != nil {
					invalid++
				}
				for _, prior := range candidates {
					if projectgit.SameRepository(prior.ref, ref) {
						duplicates++
						projectDuplicates = true
						break
					}
				}
				if ref.Primary && primaryIndex < 0 {
					primaryIndex = len(candidates)
				}
				candidates = append(candidates, candidate{row: r, ref: ref, fields: fields})
			}
			if primaryIndex < 0 && len(candidates) > 0 {
				primaryIndex = 0
			}
			for i, c := range candidates {
				r, ref, fields := c.row, c.ref, c.fields
				changed := false
				if !projectDuplicates && ref.Primary != (i == primaryIndex) {
					fields["primary"], _ = json.Marshal(i == primaryIndex)
					changed = true
					primaryRepairs++
				}
				if ref.Integration == nil && !ref.IntegrationDisabled {
					mapping, e := projectgit.ResolveObservedIntegration(ctx, tx, t.ws, ref.URL)
					if e != nil {
						tx.Rollback(ctx)
						log.Fatal(e)
					}
					if mapping != nil {
						fields["integration"], _ = json.Marshal(mapping)
						changed = true
						resolved++
					} else {
						unresolved++
					}
				}
				if !changed {
					continue
				}
				raw, e := json.Marshal(fields)
				if e != nil {
					tx.Rollback(ctx)
					log.Fatal(e)
				}
				if _, e = q.UpdateProjectResource(ctx, db.UpdateProjectResourceParams{ID: r.ID, ResourceRef: raw, Label: r.Label, Position: r.Position}); e != nil {
					tx.Rollback(ctx)
					log.Fatal(e)
				}
			}
			if e = projectgit.SyncProject(ctx, tx, t.ws, t.id); e != nil {
				tx.Rollback(ctx)
				log.Fatal(e)
			}
			if e = tx.Commit(ctx); e != nil {
				log.Fatal(e)
			}
			cursor = t.id
			total++
		}
		fmt.Printf("processed %d projects; resume with -after %s\n", total, fmt.Sprintf("%x-%x-%x-%x-%x", cursor.Bytes[0:4], cursor.Bytes[4:6], cursor.Bytes[6:8], cursor.Bytes[8:10], cursor.Bytes[10:16]))
	}
	fmt.Printf("Git binding backfill completed: %d projects, %d repositories, %d newly resolved, %d unbound/unresolved, %d invalid locators/candidates, %d duplicate candidates retained, %d primary flags repaired\n", total, repositories, resolved, unresolved, invalid, duplicates, primaryRepairs)
}
