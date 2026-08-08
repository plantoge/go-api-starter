# Adding a Module

Every tenant-scoped module follows the shape of
`internal/modules/tenant/user/` — copy that folder as your starting point.

## File shape

```
modules/tenant/<name>/
  model.go       # DB row struct(s), `db:"..."` tags
  dto.go         # request/response structs, `json:"..."` + `validate:"..."` tags
  repository.go  # SQL via sqlx, always through database.WithTenant
  service.go     # business logic, context.Context only, no HTTP types
  handler.go     # Fiber only, converts to/from DTOs, never touches SQL
```

## Steps

1. **Add a migration.** Create
   `internal/migration/tenant/0000N_create_<name>_table.up.sql` (and
   `.down.sql`), following `database-conventions.md` for audit columns and
   soft delete. Bump the version number.
2. **Write `model.go`** — one struct per table, `db:"..."` tags matching
   column names exactly.
3. **Write `repository.go`** — every method wraps its query in
   `db.WithTenant(ctx, func(tx *sqlx.Tx) error {...})`. Declare a
   `Sortable map[string]string` whitelist for anything the list endpoint
   can sort by — never accept a raw `sort` query value.
4. **Write `dto.go`** — request structs with `validate:"..."` tags, a
   `View` struct for responses, and a `toView` mapper.
5. **Write `service.go`** — depends only on the repository and
   `context.Context`. Password hashing, external calls, or cross-module
   calls (via another module's *service*, never its repository) go here.
6. **Write `handler.go`** — parse the body, call `validator.Validate`,
   call the service, translate the result with `response.Success` /
   `response.Error`. Never write `c.Status(...)` directly.
7. **Add permission constants** in `internal/permission/constants.go` for
   the new resource (`<name>.view`, `<name>.create`, ...), following the
   existing `user.*` ones.
8. **Wire routes** in `internal/server/router.go`, inside the `tenantAPI`
   group, each behind
   `middleware.RequirePermission(permission.YourConstant, deps.PermissionCache)`.
9. **Add the handler to `server.Dependencies`** and construct it in
   `cmd/api/main.go`.
10. **Run `cli permission sync --all`** after deploying, so existing
    tenants get the new permission rows.

## Sub-grouping modules

Keep `modules/tenant/` flat until it holds 7–8 modules and navigation gets
hard. Then group by domain, e.g. `modules/tenant/sales/order/`,
`modules/tenant/inventory/product/` — the module itself doesn't change
shape, only its path. **Modules must not call another module's repository
directly** — cross-module interaction always goes through the other
module's *service*.
