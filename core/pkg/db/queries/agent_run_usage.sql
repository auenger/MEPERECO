-- name: UpsertAgentRunUsage :exec
-- Bumps `updated_at` on INSERT and on conflict so the hourly-rollup worker
-- detects the row as dirty and re-aggregates its bucket.
-- Without the conflict-side bump, a correction to historical token counts
-- would never propagate to the rollup.
-- cost_usd_ticks is the provider's own price for this usage (1e-10 USD), NULL
-- when it reports none. It is overwritten like the token counters so a
-- corrected report replaces the previous figure rather than accumulating.
INSERT INTO agent_run_usage (run_id, provider, model, input_tokens, output_tokens, cache_read_tokens, cache_write_tokens, cost_usd_ticks, updated_at) SELECT
    $1 AS run_id,
    $2 AS provider,
    $3 AS model,
    $4 AS input_tokens,
    $5 AS output_tokens,
    $6 AS cache_read_tokens,
    $7 AS cache_write_tokens,
    sqlc.narg('cost_usd_ticks') AS cost_usd_ticks,
    now() AS updated_at
WHERE (($1)::uuid IS NULL OR EXISTS (SELECT 1 FROM agent_run AS ref_run_id WHERE ref_run_id.id = ($1) FOR KEY SHARE))

ON CONFLICT (run_id, provider, model)
DO UPDATE SET
    input_tokens = EXCLUDED.input_tokens,
    output_tokens = EXCLUDED.output_tokens,
    cache_read_tokens = EXCLUDED.cache_read_tokens,
    cache_write_tokens = EXCLUDED.cache_write_tokens,
    cost_usd_ticks = EXCLUDED.cost_usd_ticks,
    updated_at = now();

-- name: GetAgentRunUsage :many
SELECT * FROM agent_run_usage
WHERE run_id = $1
ORDER BY model;

-- name: GetTaskUsageSummary :one
SELECT
    COALESCE(SUM(tu.input_tokens), 0)::bigint AS total_input_tokens,
    COALESCE(SUM(tu.output_tokens), 0)::bigint AS total_output_tokens,
    COALESCE(SUM(tu.cache_read_tokens), 0)::bigint AS total_cache_read_tokens,
    COALESCE(SUM(tu.cache_write_tokens), 0)::bigint AS total_cache_write_tokens,
    COALESCE(SUM(tu.cost_usd_ticks), 0)::bigint AS total_cost_usd_ticks,
    COALESCE(SUM(tu.input_tokens)       FILTER (WHERE tu.cost_usd_ticks IS NULL), 0)::bigint AS uncosted_input_tokens,
    COALESCE(SUM(tu.output_tokens)      FILTER (WHERE tu.cost_usd_ticks IS NULL), 0)::bigint AS uncosted_output_tokens,
    COALESCE(SUM(tu.cache_read_tokens)  FILTER (WHERE tu.cost_usd_ticks IS NULL), 0)::bigint AS uncosted_cache_read_tokens,
    COALESCE(SUM(tu.cache_write_tokens) FILTER (WHERE tu.cost_usd_ticks IS NULL), 0)::bigint AS uncosted_cache_write_tokens,
    COUNT(DISTINCT tu.run_id)::int AS run_count
FROM agent_run_usage tu
JOIN agent_run atq ON atq.id = tu.run_id
WHERE atq.task_id = $1;

-- name: ListDashboardUsageDaily :many
-- Authoritative, indexed raw usage over the requested workspace and date range.
-- Chat runs have a NULL task_id and must be included. Read-time aggregation
-- avoids stale hourly-rollup results and supports exact run-kind filtering.
SELECT
    DATE(tu.created_at AT TIME ZONE sqlc.arg('tz')::text) AS date,
    LOWER(tu.provider) AS provider,
    tu.model,
    SUM(tu.input_tokens)::bigint AS input_tokens,
    SUM(tu.output_tokens)::bigint AS output_tokens,
    SUM(tu.cache_read_tokens)::bigint AS cache_read_tokens,
    SUM(tu.cache_write_tokens)::bigint AS cache_write_tokens,
    COALESCE(SUM(tu.cost_usd_ticks), 0)::bigint AS cost_usd_ticks,
    COALESCE(SUM(tu.input_tokens) FILTER (WHERE tu.cost_usd_ticks IS NULL), 0)::bigint AS uncosted_input_tokens,
    COALESCE(SUM(tu.output_tokens) FILTER (WHERE tu.cost_usd_ticks IS NULL), 0)::bigint AS uncosted_output_tokens,
    COALESCE(SUM(tu.cache_read_tokens) FILTER (WHERE tu.cost_usd_ticks IS NULL), 0)::bigint AS uncosted_cache_read_tokens,
    COALESCE(SUM(tu.cache_write_tokens) FILTER (WHERE tu.cost_usd_ticks IS NULL), 0)::bigint AS uncosted_cache_write_tokens,
    COUNT(DISTINCT tu.run_id)::int AS run_count
FROM agent_run_usage tu
JOIN agent_run atq ON atq.id = tu.run_id
JOIN agent a ON a.id = atq.agent_id
LEFT JOIN task i ON i.id = atq.task_id AND i.workspace_id = a.workspace_id
LEFT JOIN chat_session cs ON cs.id = atq.chat_session_id AND cs.workspace_id = a.workspace_id
WHERE a.workspace_id = $1
  AND tu.created_at >= sqlc.arg('since')::timestamptz
  AND (sqlc.narg('project_id')::uuid IS NULL OR (CASE WHEN atq.chat_session_id IS NOT NULL THEN cs.project_id ELSE i.project_id END) = sqlc.narg('project_id'))
  AND (sqlc.arg('run_kind')::text = '' OR agent_run_usage_kind(atq.chat_session_id, atq.task_id) = sqlc.arg('run_kind'))
GROUP BY DATE(tu.created_at AT TIME ZONE sqlc.arg('tz')::text), LOWER(tu.provider), tu.model
ORDER BY date DESC, provider, tu.model;

-- name: ListDashboardUsageByAgent :many
-- Same source and scope as daily usage, retaining the model for workspace pricing.
SELECT
    atq.agent_id,
    LOWER(tu.provider) AS provider,
    tu.model,
    SUM(tu.input_tokens)::bigint AS input_tokens,
    SUM(tu.output_tokens)::bigint AS output_tokens,
    SUM(tu.cache_read_tokens)::bigint AS cache_read_tokens,
    SUM(tu.cache_write_tokens)::bigint AS cache_write_tokens,
    COALESCE(SUM(tu.cost_usd_ticks), 0)::bigint AS cost_usd_ticks,
    COALESCE(SUM(tu.input_tokens) FILTER (WHERE tu.cost_usd_ticks IS NULL), 0)::bigint AS uncosted_input_tokens,
    COALESCE(SUM(tu.output_tokens) FILTER (WHERE tu.cost_usd_ticks IS NULL), 0)::bigint AS uncosted_output_tokens,
    COALESCE(SUM(tu.cache_read_tokens) FILTER (WHERE tu.cost_usd_ticks IS NULL), 0)::bigint AS uncosted_cache_read_tokens,
    COALESCE(SUM(tu.cache_write_tokens) FILTER (WHERE tu.cost_usd_ticks IS NULL), 0)::bigint AS uncosted_cache_write_tokens,
    COUNT(DISTINCT tu.run_id)::int AS run_count
FROM agent_run_usage tu
JOIN agent_run atq ON atq.id = tu.run_id
JOIN agent a ON a.id = atq.agent_id
LEFT JOIN task i ON i.id = atq.task_id AND i.workspace_id = a.workspace_id
LEFT JOIN chat_session cs ON cs.id = atq.chat_session_id AND cs.workspace_id = a.workspace_id
WHERE a.workspace_id = $1
  AND tu.created_at >= sqlc.arg('since')::timestamptz
  AND (sqlc.narg('project_id')::uuid IS NULL OR (CASE WHEN atq.chat_session_id IS NOT NULL THEN cs.project_id ELSE i.project_id END) = sqlc.narg('project_id'))
  AND (sqlc.arg('run_kind')::text = '' OR agent_run_usage_kind(atq.chat_session_id, atq.task_id) = sqlc.arg('run_kind'))
GROUP BY atq.agent_id, LOWER(tu.provider), tu.model
ORDER BY atq.agent_id, provider, tu.model;

-- name: ListDashboardRunTimeDaily :many
-- Daily per-date run time + run counts for the workspace, optionally
-- scoped to a single project. Powers the workspace dashboard's "Time"
-- and "Runs" metrics on the same toggle as Tokens / Cost. Bucketed by
-- completed_at (terminal time) sliced into calendar days under the
-- caller-supplied @tz — same Viewing-tz treatment as ListDashboardUsageDaily
-- so the Time / Runs tabs cut their day boundary identically to the
-- Cost / Tokens tabs (a viewer east of UTC would otherwise see the four
-- tabs disagree on a "1d" window). Only terminal runs (completed or
-- failed) with both started_at and completed_at populated contribute.
--
-- @since is already the viewer's local start-of-day-(N) (parseSinceParamInTZ)
-- — passed straight through, NOT re-truncated; see ListDashboardUsageDaily.
SELECT
    DATE(atq.completed_at AT TIME ZONE sqlc.arg('tz')::text) AS date,
    COALESCE(
        SUM(EXTRACT(EPOCH FROM (atq.completed_at - atq.started_at)))::bigint,
        0
    )::bigint AS total_seconds,
    COUNT(*)::int AS run_count,
    COUNT(*) FILTER (WHERE atq.status = 'failed')::int AS failed_count,
    COUNT(*) FILTER (WHERE agent_run_usage_kind(atq.chat_session_id, atq.task_id) = 'chat')::int AS chat_run_count,
    COUNT(*) FILTER (WHERE agent_run_usage_kind(atq.chat_session_id, atq.task_id) = 'task')::int AS task_run_count,
    COUNT(*) FILTER (WHERE agent_run_usage_kind(atq.chat_session_id, atq.task_id) = 'other')::int AS other_run_count
FROM agent_run atq
JOIN agent a ON a.id = atq.agent_id
LEFT JOIN task i ON i.id = atq.task_id AND i.workspace_id = a.workspace_id
LEFT JOIN chat_session cs ON cs.id = atq.chat_session_id AND cs.workspace_id = a.workspace_id
WHERE a.workspace_id = $1
  AND atq.status IN ('completed', 'failed')
  AND atq.started_at IS NOT NULL
  AND atq.completed_at IS NOT NULL
  AND atq.completed_at >= sqlc.arg('since')::timestamptz
  AND (sqlc.narg('project_id')::uuid IS NULL OR (CASE WHEN atq.chat_session_id IS NOT NULL THEN cs.project_id ELSE i.project_id END) = sqlc.narg('project_id'))
  AND (sqlc.arg('run_kind')::text = '' OR agent_run_usage_kind(atq.chat_session_id, atq.task_id) = sqlc.arg('run_kind'))
GROUP BY DATE(atq.completed_at AT TIME ZONE sqlc.arg('tz')::text)
ORDER BY DATE(atq.completed_at AT TIME ZONE sqlc.arg('tz')::text) DESC;

-- name: ListDashboardAgentRunTime :many
-- Per-agent total run run time and run count for the workspace, optionally
-- scoped to a single project. Counts only terminal runs (completed or failed)
-- with both started_at and completed_at populated — queued/running runs have
-- no finite duration. Anchored on completed_at so the window matches the
-- token cost window (which is anchored on tu.created_at, ~= completion time).
--
-- No date bucketing, so no @tz — but @since is the viewer's local
-- start-of-day-(N) so the "last N days" window lines up with the per-agent
-- cost card; passed straight through without re-truncation.
SELECT
    atq.agent_id,
    COALESCE(
        SUM(EXTRACT(EPOCH FROM (atq.completed_at - atq.started_at)))::bigint,
        0
    )::bigint AS total_seconds,
    COUNT(*)::int AS run_count,
    COUNT(*) FILTER (WHERE atq.status = 'failed')::int AS failed_count,
    COUNT(*) FILTER (WHERE agent_run_usage_kind(atq.chat_session_id, atq.task_id) = 'chat')::int AS chat_run_count,
    COUNT(*) FILTER (WHERE agent_run_usage_kind(atq.chat_session_id, atq.task_id) = 'task')::int AS task_run_count,
    COUNT(*) FILTER (WHERE agent_run_usage_kind(atq.chat_session_id, atq.task_id) = 'other')::int AS other_run_count
FROM agent_run atq
JOIN agent a ON a.id = atq.agent_id
LEFT JOIN task i ON i.id = atq.task_id AND i.workspace_id = a.workspace_id
LEFT JOIN chat_session cs ON cs.id = atq.chat_session_id AND cs.workspace_id = a.workspace_id
WHERE a.workspace_id = $1
  AND atq.status IN ('completed', 'failed')
  AND atq.started_at IS NOT NULL
  AND atq.completed_at IS NOT NULL
  AND atq.completed_at >= @since::timestamptz
  AND (sqlc.narg('project_id')::uuid IS NULL OR (CASE WHEN atq.chat_session_id IS NOT NULL THEN cs.project_id ELSE i.project_id END) = sqlc.narg('project_id'))
  AND (sqlc.arg('run_kind')::text = '' OR agent_run_usage_kind(atq.chat_session_id, atq.task_id) = sqlc.arg('run_kind'))
GROUP BY atq.agent_id
ORDER BY total_seconds DESC;

-- name: ListDashboardFailuresDaily :many
-- Daily per-(date, failure_reason) terminal-run counts for the workspace,
-- optionally scoped to a single project. Powers the workspace dashboard's
-- "Errors" trend and the errors-by-class breakdown.
--
-- Shape note: this returns EVERY terminal run, not just the failures. The
-- `failure_reason = ''` row of each date carries that date's succeeded
-- count, which is the denominator the client needs for an error rate. A
-- failed row whose failure_reason column is NULL or empty (pre-SILI-1949
-- rows, or a failure path that forgot to classify) collapses into the
-- 'unclassified' bucket so it stays countable instead of masquerading as a
-- success. Cardinality is bounded by days x (21 reasons + 2), so the whole
-- window fits in one small payload.
--
-- Unlike ListDashboardRunTimeDaily this does NOT require started_at — a run
-- that expired in the queue (failure_reason='queued_expired') never started
-- but is unambiguously a failure, and dropping it would under-report exactly
-- the outage the Errors chart exists to surface. Every failure path sets
-- completed_at, so bucketing on it covers all of them.
--
-- @since is already the viewer's local start-of-day-(N) (parseSinceParamInTZ)
-- — passed straight through, NOT re-truncated; see ListDashboardUsageDaily.
SELECT
    DATE(atq.completed_at AT TIME ZONE sqlc.arg('tz')::text) AS date,
    CASE
        WHEN atq.status = 'failed'
            THEN COALESCE(NULLIF(atq.failure_reason, ''), 'unclassified')
        ELSE ''
    END AS failure_reason,
    COUNT(*)::int AS run_count
FROM agent_run atq
JOIN agent a ON a.id = atq.agent_id
LEFT JOIN task i ON i.id = atq.task_id AND i.workspace_id = a.workspace_id
LEFT JOIN chat_session cs ON cs.id = atq.chat_session_id AND cs.workspace_id = a.workspace_id
WHERE a.workspace_id = $1
  AND atq.status IN ('completed', 'failed')
  AND atq.completed_at IS NOT NULL
  AND atq.completed_at >= sqlc.arg('since')::timestamptz
  AND (sqlc.narg('project_id')::uuid IS NULL OR (CASE WHEN atq.chat_session_id IS NOT NULL THEN cs.project_id ELSE i.project_id END) = sqlc.narg('project_id'))
  AND (sqlc.arg('run_kind')::text = '' OR agent_run_usage_kind(atq.chat_session_id, atq.task_id) = sqlc.arg('run_kind'))
GROUP BY 1, 2
ORDER BY 1 DESC, 2;

-- name: ListDashboardFailuresByAgent :many
-- Per-(agent, failure_reason) terminal-run counts — the "top offenders"
-- half of the dashboard's errors breakdown. Same `failure_reason = ''`
-- succeeded-bucket convention as ListDashboardFailuresDaily, so the client
-- can rank agents by failure rate rather than raw count.
--
-- No date bucketing, so no @tz — @since is the viewer's local
-- start-of-day-(N) so the window lines up with the per-agent run-time card.
SELECT
    atq.agent_id,
    CASE
        WHEN atq.status = 'failed'
            THEN COALESCE(NULLIF(atq.failure_reason, ''), 'unclassified')
        ELSE ''
    END AS failure_reason,
    COUNT(*)::int AS run_count
FROM agent_run atq
JOIN agent a ON a.id = atq.agent_id
LEFT JOIN task i ON i.id = atq.task_id AND i.workspace_id = a.workspace_id
LEFT JOIN chat_session cs ON cs.id = atq.chat_session_id AND cs.workspace_id = a.workspace_id
WHERE a.workspace_id = $1
  AND atq.status IN ('completed', 'failed')
  AND atq.completed_at IS NOT NULL
  AND atq.completed_at >= @since::timestamptz
  AND (sqlc.narg('project_id')::uuid IS NULL OR (CASE WHEN atq.chat_session_id IS NOT NULL THEN cs.project_id ELSE i.project_id END) = sqlc.narg('project_id'))
  AND (sqlc.arg('run_kind')::text = '' OR agent_run_usage_kind(atq.chat_session_id, atq.task_id) = sqlc.arg('run_kind'))
GROUP BY atq.agent_id, 2
ORDER BY atq.agent_id, 2;
