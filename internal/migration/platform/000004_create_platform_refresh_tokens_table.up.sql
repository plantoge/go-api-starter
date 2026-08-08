CREATE TABLE platform.refresh_tokens (
    id          UUID PRIMARY KEY,
    user_id     UUID NOT NULL REFERENCES platform.users (id),
    token_hash  TEXT NOT NULL,
    expires_at  TIMESTAMPTZ NOT NULL,
    revoked_at  TIMESTAMPTZ NULL,
    user_agent  TEXT NULL,
    ip          TEXT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX platform_refresh_tokens_hash_key ON platform.refresh_tokens (token_hash);
CREATE INDEX platform_refresh_tokens_user_id_idx ON platform.refresh_tokens (user_id);
