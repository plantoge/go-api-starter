# CLAUDE.md

Rules for working in this repository. Breaking any of these has a real
consequence — usually data leaking across tenants — not just a style nit.

## Multi-tenancy (see `docs/multi-tenancy.md` for the full explanation)

- **Every tenant data query goes through `database.WithTenant(ctx, fn)`.**
  No repository holds a bare `*sqlx.DB` for tenant tables. The only
  exceptions are CLI/DDL paths that need one transaction spanning schema
  creation itself — something `WithTenant` can't express, since the schema
  doesn't exist yet when the transaction starts (`platform/tenant`
  provisioning/purge, `permission.Sync`, `cli cleanup`). Every one of these
  must call `database.ValidSchemaName` before interpolating a schema name
  into SQL, the same defense-in-depth `WithTenant` itself uses.
- **`*fiber.Ctx` never leaves the handler layer.** Services and
  repositories take `context.Context` only. Fiber is built on `fasthttp`,
  which reuses request objects across requests — anything that escapes the
  handler with a live `*fiber.Ctx` reference can end up reading a
  different request's (possibly a different tenant's) data.
- **Every `sort` query parameter is validated against an explicit
  whitelist** (`pagination.Parse` + a module's `Sortable` map) before it's
  interpolated into `ORDER BY`. Never pass a raw query value into SQL,
  even indirectly.

## Module boundaries

- **A module never calls another module's repository directly.**
  Cross-module interaction goes through the other module's *service*.
- New tenant-scoped modules copy the shape of
  `internal/modules/tenant/user/` — see `docs/adding-a-module.md`.

## Errors and responses

- Services return `*apperror.Error`, never a bare `error` wrapping a
  driver error straight to the client.
- Handlers never call `c.Status(...)` themselves — `response.Success` /
  `response.Error` own that.

## Database

- IDs are generated in Go (`uuid.New()`), never as a DB-side default —
  see `docs/database-conventions.md` for why.
- New tables follow the audit-column convention in
  `docs/database-conventions.md`, unless they're a join table or a
  log/token table (the two documented exceptions).

## Testing

- Integration tests use `testsupport.OpenTestDB` / `OpenTestPlatformDB`,
  backed by a local `app_test` PostgreSQL database — never Docker, never
  testcontainers.
- Tests that create a schema must drop it in `t.Cleanup`.
