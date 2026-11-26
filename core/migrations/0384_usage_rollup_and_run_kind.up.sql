-- Preserve raw usage; repair the catch-up scheduler without rewriting historic data.
CREATE FUNCTION public.agent_run_usage_kind(chat_id uuid, task_id uuid) RETURNS text
LANGUAGE sql IMMUTABLE PARALLEL SAFE AS $$
    SELECT CASE WHEN chat_id IS NOT NULL THEN 'chat' WHEN task_id IS NOT NULL THEN 'task' ELSE 'other' END;
$$;

CREATE OR REPLACE FUNCTION public.rollup_agent_run_usage_hourly() RETURNS bigint
    LANGUAGE plpgsql
    AS $$
DECLARE
    v_lock_ok BOOLEAN;
    v_from    TIMESTAMPTZ;
    v_to      TIMESTAMPTZ;
    v_rows    BIGINT := 0;
    v_target  TIMESTAMPTZ;
    v_next    TIMESTAMPTZ;
BEGIN
    SELECT pg_try_advisory_lock(4246) INTO v_lock_ok;
    IF NOT v_lock_ok THEN
        RETURN 0;
    END IF;

    BEGIN
        INSERT INTO agent_run_usage_hourly_rollup_state(id) VALUES(1) ON CONFLICT(id) DO NOTHING;
        UPDATE agent_run_usage_hourly_rollup_state
           SET last_run_started_at = now(),
               last_error          = NULL
         WHERE id = 1
        RETURNING watermark_at INTO v_from;

        -- Fresh databases start at the epoch. Skip intervals without any
        -- pending usage rather than taking decades of 1-day ticks to arrive
        -- at the first real usage row. Preserve the bounded 1-day processing
        -- window once actual pending history begins. Dirty keys are handled
        -- by the existing window function, including historical corrections.
        v_target := now() - INTERVAL '5 minutes';
        IF v_from + INTERVAL '1 day' < v_target THEN
            SELECT LEAST(
                COALESCE((SELECT min(updated_at) FROM agent_run_usage WHERE updated_at >= v_from AND updated_at < v_target), v_target),
                COALESCE((SELECT min(created_at) FROM agent_run_usage WHERE updated_at IS NULL AND created_at >= v_from AND created_at < v_target), v_target)
            ) INTO v_next;
            v_from := LEAST(v_next, v_target - INTERVAL '1 microsecond');
        END IF;
        v_to := LEAST(v_target, v_from + INTERVAL '1 day');

        IF v_from < v_to THEN
            v_rows := rollup_agent_run_usage_hourly_window(v_from, v_to);

            UPDATE agent_run_usage_hourly_rollup_state
               SET watermark_at         = v_to,
                   last_run_finished_at = now(),
                   last_run_rows        = v_rows
             WHERE id = 1;
        ELSE
            UPDATE agent_run_usage_hourly_rollup_state
               SET last_run_finished_at = now(),
                   last_run_rows        = 0
             WHERE id = 1;
        END IF;

        PERFORM pg_advisory_unlock(4246);
    EXCEPTION WHEN OTHERS THEN
        UPDATE agent_run_usage_hourly_rollup_state
           SET last_error           = SQLERRM,
               last_run_finished_at = now()
         WHERE id = 1;
        PERFORM pg_advisory_unlock(4246);
        RAISE;
    END;

    -- TTL prune. Runs AFTER the advisory lock is released: on a large
    -- stale backlog the prune can be slow, and holding lock 4246 through
    -- it would serialise every concurrent cron tick. It is a plain
    -- bounded DELETE — idempotent and safe to run unlocked.
    PERFORM prune_agent_run_usage_hourly_dirty();
    RETURN v_rows;
END;
$$;
