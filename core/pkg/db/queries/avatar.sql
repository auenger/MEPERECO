-- name: IsPublishedAvatarURL :one
-- Legacy local avatar URLs may still be cached by installed clients. Only
-- currently published avatar objects qualify for their unauthenticated read.
SELECT EXISTS (
    SELECT 1 FROM workspace WHERE avatar_url = sqlc.arg(avatar_url)::text
    UNION ALL
    SELECT 1 FROM agent WHERE avatar_url = sqlc.arg(avatar_url)::text
    UNION ALL
    SELECT 1 FROM squad WHERE avatar_url = sqlc.arg(avatar_url)::text
    UNION ALL
    SELECT 1 FROM "user" WHERE avatar_url = sqlc.arg(avatar_url)::text
) AS published;
