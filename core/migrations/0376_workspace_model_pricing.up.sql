CREATE TABLE workspace_model_price (
    workspace_id uuid NOT NULL REFERENCES workspace(id) ON DELETE CASCADE,
    provider text NOT NULL,
    model text NOT NULL,
    mode text NOT NULL CHECK (mode IN ('custom','alias','free')),
    reference_key text NOT NULL DEFAULT '',
    input_per_m double precision NOT NULL DEFAULT 0 CHECK (input_per_m BETWEEN 0 AND 1000000),
    output_per_m double precision NOT NULL DEFAULT 0 CHECK (output_per_m BETWEEN 0 AND 1000000),
    cache_read_per_m double precision NOT NULL DEFAULT 0 CHECK (cache_read_per_m BETWEEN 0 AND 1000000),
    cache_write_per_m double precision NOT NULL DEFAULT 0 CHECK (cache_write_per_m BETWEEN 0 AND 1000000),
    updated_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (workspace_id, provider, model)
);
