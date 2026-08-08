CREATE TABLE platform.tenants (
    id            UUID PRIMARY KEY,
    code          TEXT NOT NULL,
    name          TEXT NOT NULL,
    schema_name   TEXT NOT NULL,
    status        TEXT NOT NULL DEFAULT 'provisioning'
                  CHECK (status IN ('provisioning', 'active', 'suspended')),
    suspended_at  TIMESTAMPTZ NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at    TIMESTAMPTZ NULL,
    created_by    UUID NULL,
    updated_by    UUID NULL,
    deleted_by    UUID NULL
);

CREATE UNIQUE INDEX tenants_code_key ON platform.tenants (code) WHERE deleted_at IS NULL;
CREATE UNIQUE INDEX tenants_schema_name_key ON platform.tenants (schema_name) WHERE deleted_at IS NULL;

CREATE TRIGGER set_updated_at
    BEFORE UPDATE ON platform.tenants
    FOR EACH ROW EXECUTE FUNCTION platform.trigger_set_updated_at();
