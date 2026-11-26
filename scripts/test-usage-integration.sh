#!/usr/bin/env bash
# Disposable PostgreSQL; never reads the application's production database URL.
set -euo pipefail
repo_root=$(cd "$(dirname "$0")/.." && pwd)
container=$(docker run --rm -d -e POSTGRES_PASSWORD=usage-test -e POSTGRES_DB=usage_test -p 127.0.0.1::5432 pgvector/pgvector:pg17)
trap 'docker stop "$container" >/dev/null 2>&1 || true' EXIT
ready=false
for _ in $(seq 1 60); do
  if docker exec "$container" pg_isready -h 127.0.0.1 -U postgres -d usage_test >/dev/null 2>&1; then ready=true; break; fi
  sleep 1
done
if [[ "$ready" != true ]]; then docker logs "$container" >&2; exit 1; fi
port=$(docker port "$container" 5432/tcp | awk -F: 'NR==1{print $NF}')
export MEPERSTUDIO_USAGE_TEST_DATABASE_URL="postgres://postgres:usage-test@127.0.0.1:$port/usage_test?sslmode=disable"
cd "$repo_root/core"
DATABASE_URL="$MEPERSTUDIO_USAGE_TEST_DATABASE_URL" go run ./cmd/migrate up
go test -race -count=1 ./internal/handler -run 'TestUsageLedgerPostgres|TestDashboardRunKind|TestWorkspaceModelPricing' -v
