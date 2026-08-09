# Multi-Tenancy Rules

These two rules are what keep tenant A's data from ever appearing in
tenant B's response. Breaking either one is a data breach, not a bug.

## Rule 1: all tenant data access goes through `database.WithTenant`

```go
// Right
func (r *Repository) FindByID(ctx context.Context, id uuid.UUID) (User, error) {
	var u User
	err := r.db.WithTenant(ctx, func(tx *sqlx.Tx) error {
		return tx.Get(&u, `SELECT * FROM users WHERE id = $1 AND deleted_at IS NULL`, id)
	})
	return u, err
}
```
```go
// Wrong — bypasses tenant scoping entirely
func (r *Repository) FindByID(ctx context.Context, id uuid.UUID) (User, error) {
	var u User
	err := r.pool.Get(&u, `SELECT * FROM users WHERE id = $1`, id) // which tenant?!
	return u, err
}
```
`WithTenant` runs your query inside a transaction with `SET LOCAL
search_path` pinned to the calling tenant's schema — resolved from
`context.Context`, never from a request parameter. `SET LOCAL` is
automatically cleared when the transaction ends (commit or rollback), so a
connection can never carry one tenant's schema into the next caller that
borrows it from the pool.

## Rule 2: `*fiber.Ctx` stops at the handler

Fiber is built on `fasthttp`, which **reuses** its context objects between
requests for performance. If a `*fiber.Ctx` (or anything derived from it)
escapes into a goroutine or gets stored anywhere, it can be silently
overwritten by a completely different request — including a different
tenant's — before it's read back.

```go
// Right — handler converts everything it needs before calling the service
func (h *Handler) Get(c *fiber.Ctx) error {
	id, _ := uuid.Parse(c.Params("id"))
	view, err := h.svc.Get(c.UserContext(), id) // context.Context only, from here down
	if err != nil {
		return response.Error(c, middleware.RequestIDFromCtx(c), err)
	}
	return response.Success(c, 200, view)
}
```
```go
// Wrong — never do this
func (h *Handler) Get(c *fiber.Ctx) error {
	go func() {
		log.Println(c.Params("id")) // c may belong to a different request by now
	}()
	return nil
}
```
Services and repositories only ever see `context.Context`. This isn't just
a safety rule — it also means business logic can be unit tested without
starting an HTTP server.

## What ends up in `context.Context`

Set by middleware, read via `database.TenantFromContext` /
`database.ActorFromContext`:
- `database.TenantInfo` — tenant ID + schema name, set by `RequireTenant`
- `database.Actor` — user ID + scope, set by `RequirePlatform` /
  `RequireTenant`

The one exception is login: there's no token yet to prove tenant identity,
so `modules/tenant/auth.Service.Login` constructs `TenantInfo` itself from
the resolved `tenant_code`, before calling `WithTenant`. Every other
tenant-scoped code path receives it already set by middleware.
