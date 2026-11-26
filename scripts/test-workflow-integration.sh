#!/usr/bin/env bash
# Start a disposable PostgreSQL instance; never use the application's DATABASE_URL.
set -euo pipefail
repo_root=$(cd "$(dirname "$0")/.." && pwd)
container=$(docker run --rm -d -e POSTGRES_PASSWORD=workflow-test -e POSTGRES_DB=workflow_test -p 127.0.0.1::5432 pgvector/pgvector:pg17)
trap 'docker stop "$container" >/dev/null 2>&1 || true' EXIT
ready=false
for _ in $(seq 1 60); do
  if docker exec "$container" pg_isready -h 127.0.0.1 -U postgres -d workflow_test >/dev/null 2>&1; then ready=true; break; fi
  sleep 1
done
if [[ "$ready" != true ]]; then docker logs "$container" >&2; exit 1; fi
port=$(docker port "$container" 5432/tcp | awk -F: 'NR==1{print $NF}')
export MEPERSTUDIO_TEST_DATABASE_URL="postgres://postgres:workflow-test@127.0.0.1:$port/workflow_test?sslmode=disable"
cd "$repo_root/core"
DATABASE_URL="$MEPERSTUDIO_TEST_DATABASE_URL" go run ./cmd/migrate up
# Rehearse the additive upgrade from 0380 on representative legacy records.
# The only database affected is the disposable container created above.
for migration in 0383_workflow_scheduled_readiness 0382_runtime_registry_protocol 0381_workflow_completion_policy; do
  docker exec -i "$container" psql -U postgres -d workflow_test -v ON_ERROR_STOP=1 < "migrations/${migration}.down.sql"
  docker exec "$container" psql -U postgres -d workflow_test -v ON_ERROR_STOP=1 -c "DELETE FROM schema_migrations WHERE version='${migration}'"
done
docker exec -i "$container" psql -U postgres -d workflow_test -v ON_ERROR_STOP=1 <<'SQL'
DO $$
DECLARE ws uuid; actor uuid; sop uuid; ver uuid; run uuid;
BEGIN
  INSERT INTO workspace(name,slug,task_prefix) VALUES('Migration fixture','migration-fixture','MIG') RETURNING id INTO ws;
  INSERT INTO "user"(name,email) VALUES('Migration fixture','migration-fixture@example.test') RETURNING id INTO actor;
  INSERT INTO member(workspace_id,user_id,role) VALUES(ws,actor,'owner');
  INSERT INTO workflow(workspace_id,name,created_by) VALUES(ws,'Legacy SOP',actor) RETURNING id INTO sop;
  INSERT INTO workflow_version(workspace_id,workflow_id,version,created_by) VALUES(ws,sop,1,actor) RETURNING id INTO ver;
  INSERT INTO workflow_stage(workspace_id,workflow_version_id,stable_key,name,position) VALUES(ws,ver,'legacy','Legacy template stage',0);
  INSERT INTO workflow_instance(workspace_id,workflow_id,workflow_version_id,title,status,created_by) VALUES(ws,sop,ver,'Legacy run','active',actor) RETURNING id INTO run;
  INSERT INTO workflow_instance_stage(workspace_id,workflow_instance_id,stable_key,name,position) VALUES(ws,run,'legacy','Legacy run stage',0);
  INSERT INTO runtime_profile(workspace_id,display_name,protocol_family,command_name,created_by) VALUES(ws,'Legacy profile','zcode','zcode',actor);
END $$;
SQL
DATABASE_URL="$MEPERSTUDIO_TEST_DATABASE_URL" go run ./cmd/migrate up
docker exec -i "$container" psql -U postgres -d workflow_test -v ON_ERROR_STOP=1 <<'SQL'
DO $$
BEGIN
  IF NOT EXISTS (SELECT 1 FROM workflow_stage WHERE name='Legacy template stage' AND completion_rule='{"type":"all_runs_terminal"}'::jsonb) OR
     NOT EXISTS (SELECT 1 FROM workflow_instance_stage WHERE name='Legacy run stage' AND completion_rule='{"type":"all_runs_terminal"}'::jsonb) OR
     NOT EXISTS (SELECT 1 FROM runtime_profile WHERE display_name='Legacy profile' AND protocol_family='zcode') THEN
    RAISE EXCEPTION 'legacy data rewritten or missing after upgrade';
  END IF;
END $$;
SQL
go test -race -count=1 ./internal/service -run 'TestWorkflow.*Postgres' -v
