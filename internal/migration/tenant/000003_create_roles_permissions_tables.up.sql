CREATE TABLE roles (
    id          UUID PRIMARY KEY,
    name        TEXT NOT NULL,
    is_system   BOOLEAN NOT NULL DEFAULT false,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at  TIMESTAMPTZ NULL,
    created_by  UUID NULL,
    updated_by  UUID NULL,
    deleted_by  UUID NULL
);

CREATE UNIQUE INDEX roles_name_key ON roles (name) WHERE deleted_at IS NULL;

CREATE TRIGGER set_updated_at
    BEFORE UPDATE ON roles
    FOR EACH ROW EXECUTE FUNCTION trigger_set_updated_at();

CREATE TABLE permissions (
    id          UUID PRIMARY KEY,
    code        TEXT NOT NULL,
    "group"     TEXT NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at  TIMESTAMPTZ NULL,
    created_by  UUID NULL,
    updated_by  UUID NULL,
    deleted_by  UUID NULL
);

CREATE UNIQUE INDEX permissions_code_key ON permissions (code) WHERE deleted_at IS NULL;

CREATE TRIGGER set_updated_at
    BEFORE UPDATE ON permissions
    FOR EACH ROW EXECUTE FUNCTION trigger_set_updated_at();

CREATE TABLE role_permissions (
    role_id        UUID NOT NULL REFERENCES roles (id),
    permission_id  UUID NOT NULL REFERENCES permissions (id),
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_by     UUID NULL,
    PRIMARY KEY (role_id, permission_id)
);

CREATE TABLE user_roles (
    user_id     UUID NOT NULL REFERENCES users (id),
    role_id     UUID NOT NULL REFERENCES roles (id),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_by  UUID NULL,
    PRIMARY KEY (user_id, role_id)
);
