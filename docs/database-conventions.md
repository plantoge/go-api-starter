# Database Conventions

## Every table (with two exceptions below)

```sql
created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
deleted_at  TIMESTAMPTZ NULL
created_by  UUID NULL
updated_by  UUID NULL
deleted_by  UUID NULL
```

- `updated_at` is maintained by a per-schema trigger
  (`trigger_set_updated_at`) — attach it to every new table:
  ```sql
  CREATE TRIGGER set_updated_at
      BEFORE UPDATE ON your_table
      FOR EACH ROW EXECUTE FUNCTION trigger_set_updated_at();
  ```
- `*_by` columns are filled from `database.ActorFromContext(ctx)` in the
  repository layer, never left for the database to guess.
- CLI-driven platform-tenant lifecycle operations (`cmd/cli/commands/tenant.go`)
  have no logged-in user to attribute actions to, so they run with a fixed
  sentinel actor (`database.Actor{UserID: uuid.Nil, Scope: "cli"}`) — if you
  see `00000000-0000-0000-0000-000000000000` in a `*_by` column, it means
  the action was taken via the CLI, not that the audit trail is broken.
- IDs are `UUID PRIMARY KEY` with **no database default** — generate with
  `uuid.New()` in Go before inserting. (A `SET LOCAL search_path`
  transaction only searches the tenant's own schema, not `public`, so a
  DB-side default calling an extension function there would fail
  unpredictably. Generating in Go sidesteps the problem entirely.)

## Soft delete

Every query filters `WHERE deleted_at IS NULL` unless it's specifically
looking for deleted rows (like `tenant purge`'s 30-day check). Any column
with a uniqueness requirement needs a **partial** unique index so a
deleted row's value can be reused:
```sql
CREATE UNIQUE INDEX users_email_key ON users (email) WHERE deleted_at IS NULL;
```

## Exceptions

**Join tables** (`role_permissions`, `user_roles`): only `created_at` +
`created_by`. Access is revoked with a hard `DELETE`, not a soft delete —
stacking `deleted_at IS NULL` filters onto every permission check for no
benefit isn't worth it, and grant/revoke history belongs in logs, not in
the join table.

**Log/token tables** (`refresh_tokens`, `login_attempts`): only
`created_at`. They're append-only — nothing ever updates or soft-deletes a
row in them. A revoked refresh token is represented by its own
`revoked_at` column, not the standard `deleted_at`.

## Naming

- Tables: plural, `snake_case` (`users`, `role_permissions`).
- Columns: `snake_case`, no type prefixes (`email`, not `str_email`).
- Foreign keys: `<singular_table>_id` (`user_id`, `role_id`).
