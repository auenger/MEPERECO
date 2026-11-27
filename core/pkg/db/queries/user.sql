-- name: GetUser :one
SELECT * FROM "user"
WHERE id = $1;

-- name: GetUserByEmail :one
SELECT * FROM "user"
WHERE email = $1;

-- name: SearchWorkspaceInviteCandidates :many
SELECT u.id, u.name, u.email, u.avatar_url
FROM "user" u
WHERE (
    CASE WHEN sqlc.arg(domain_only)::boolean THEN
        lower(split_part(u.email, '@', 2)) = sqlc.arg(search_term)::text
    ELSE
        strpos(lower(u.name), sqlc.arg(search_term)::text) > 0
        OR strpos(lower(u.email), sqlc.arg(search_term)::text) > 0
    END
)
AND NOT EXISTS (
    SELECT 1 FROM member m
    WHERE m.workspace_id = sqlc.arg(workspace_id) AND m.user_id = u.id
)
AND NOT EXISTS (
    SELECT 1 FROM workspace_invitation wi
    WHERE wi.workspace_id = sqlc.arg(workspace_id)
      AND wi.status = 'pending' AND wi.expires_at > now()
      AND (wi.invitee_user_id = u.id OR lower(wi.invitee_email) = lower(u.email))
)
ORDER BY (lower(u.email) = sqlc.arg(search_term)::text) DESC, lower(u.name), lower(u.email), u.id
LIMIT 31 OFFSET sqlc.arg(page_offset)::integer;

-- name: GetUsersByIDs :many
-- Batch lookup from the GLOBAL user table (not gated on membership, so departed
-- members still render). Used to enrich attribution initiator / originator refs on
-- run responses without an N+1 (SILI-4302 §9). Returns only the display fields.
SELECT id, name, email, avatar_url FROM "user"
WHERE id = ANY(@ids::uuid[]);

-- name: CreateUser :one
INSERT INTO "user" (name, email, avatar_url)
VALUES ($1, $2, $3)
RETURNING *;

-- name: UpdateUser :one
-- Patches the user-controlled profile fields. Each parameter follows
-- COALESCE-on-NULL semantics so the handler can omit any field it
-- doesn't intend to write.
--
-- `timezone` (Viewing-tz preference) participates in
-- the same shape but uses sqlc.narg + a sentinel-string convention:
-- the handler passes the empty string "" to mean "clear back to NULL"
-- (browser-detected fallback), an IANA name like "Asia/Shanghai" to
-- pin a value, and `sqlc.narg('timezone') IS NULL` (no value at all)
-- to leave the existing column untouched. Folding it into UpdateUser
-- rather than carrying a dedicated UpdateUserTimezone keeps the
-- profile-patch shape uniform between Preferences fields.
UPDATE "user" SET
    name = COALESCE($2, name),
    avatar_url = COALESCE($3, avatar_url),
    language = COALESCE($4, language),
    profile_description = COALESCE(sqlc.narg('profile_description'), profile_description),
    timezone = CASE
        WHEN sqlc.narg('timezone')::text IS NULL THEN timezone
        WHEN sqlc.narg('timezone')::text = ''    THEN NULL
        ELSE sqlc.narg('timezone')::text
    END,
    updated_at = now()
WHERE id = $1
RETURNING *;

-- name: MarkUserOnboarded :one
UPDATE "user" SET
    onboarded_at = COALESCE(onboarded_at, now()),
    updated_at = now()
WHERE id = $1
RETURNING *;

-- name: PatchUserOnboarding :one
-- Partial update of the user's onboarding decision fields. Currently only the
-- questionnaire JSONB is patchable — the v2 attempt at persisting Step 3
-- runtime choice on the user row was reverted; that state now lives in a
-- frontend Zustand transient store.
UPDATE "user" SET
    onboarding_questionnaire = COALESCE(sqlc.narg('questionnaire'), onboarding_questionnaire),
    updated_at = now()
WHERE id = sqlc.arg('id')
RETURNING *;

-- name: JoinCloudWaitlist :one
-- Records interest in cloud runtimes. Does NOT mark onboarding
-- complete — the user still has to pick a real path (CLI / Skip)
-- in Step 3. Repeating the call overwrites email + reason.
UPDATE "user" SET
    cloud_waitlist_email = $2,
    cloud_waitlist_reason = $3,
    updated_at = now()
WHERE id = $1
RETURNING *;

-- name: SetStarterContentState :one
-- Atomically transition starter_content_state. The handler is
-- responsible for checking the current value first (to decide between
-- "transition NULL -> imported and run the seeding" vs "already
-- decided, short-circuit"). Using COALESCE here would swallow the
-- transition, so this is a straight assignment.
UPDATE "user" SET
    starter_content_state = $2,
    updated_at = now()
WHERE id = $1
RETURNING *;
