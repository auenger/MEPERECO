-- Password registration and recovery use separate, purpose-bound email
-- challenges. The legacy login OTP table cannot authorize a password change.
CREATE TABLE auth_password_challenge (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    email text NOT NULL,
    purpose text NOT NULL CHECK (purpose IN ('register', 'reset')),
    code_hash text,
    expires_at timestamptz NOT NULL,
    attempts integer NOT NULL DEFAULT 0,
    created_at timestamptz NOT NULL DEFAULT now(),
    token_hash text UNIQUE,
    token_expires_at timestamptz,
    UNIQUE (email, purpose)
);
CREATE INDEX auth_password_challenge_expiry_idx ON auth_password_challenge (expires_at);

-- Database-backed limit keeps the password routes protected even on a
-- single-node self-hosted installation without Redis.
CREATE TABLE auth_password_rate_limit (
    bucket text PRIMARY KEY,
    attempts integer NOT NULL,
    expires_at timestamptz NOT NULL
);
CREATE INDEX auth_password_rate_limit_expiry_idx ON auth_password_rate_limit (expires_at);
