CREATE TABLE platform.login_attempts (
    id           UUID PRIMARY KEY,
    scope        TEXT NOT NULL CHECK (scope IN ('platform', 'tenant')),
    tenant_id    UUID NULL,
    email        TEXT NOT NULL,
    success      BOOLEAN NOT NULL,
    attempted_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX login_attempts_lookup_idx
    ON platform.login_attempts (scope, tenant_id, email, attempted_at DESC);
