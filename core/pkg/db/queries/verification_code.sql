-- name: CreateVerificationCode :one
INSERT INTO verification_code (email, code, expires_at)
VALUES ($1, $2, $3)
RETURNING *;

-- name: GetLatestVerificationCode :one
SELECT * FROM verification_code
WHERE id = (
    SELECT id FROM verification_code WHERE email = $1
    ORDER BY created_at DESC, id DESC LIMIT 1
)
  AND used = FALSE
  AND expires_at > now()
  AND attempts < 5;

-- name: MarkVerificationCodeUsed :execrows
UPDATE verification_code
SET used = TRUE
WHERE id = $1 AND used = FALSE AND expires_at > now() AND attempts < 5
  AND id = (SELECT newest.id FROM verification_code AS newest
            WHERE newest.email = verification_code.email
            ORDER BY newest.created_at DESC, newest.id DESC LIMIT 1);

-- name: IncrementVerificationCodeAttempts :exec
UPDATE verification_code
SET attempts = attempts + 1
WHERE id = $1;

-- name: GetLatestCodeByEmail :one
SELECT * FROM verification_code
WHERE email = $1
ORDER BY created_at DESC, id DESC
LIMIT 1;

-- name: DeleteExpiredVerificationCodes :exec
DELETE FROM verification_code
WHERE expires_at < now() - interval '1 hour';
