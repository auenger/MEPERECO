CREATE TABLE workspace_pricing_currency (
    workspace_id uuid PRIMARY KEY REFERENCES workspace(id) ON DELETE CASCADE,
    currency text NOT NULL CHECK (currency IN ('USD','CNY')),
    usd_to_cny double precision NOT NULL DEFAULT 0 CHECK (usd_to_cny >= 0 AND usd_to_cny <= 1000),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CHECK (currency <> 'CNY' OR usd_to_cny > 0)
);
