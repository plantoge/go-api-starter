package pagination

import (
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v2"
	"go-api-starter/internal/apperror"
	"go-api-starter/internal/response"
)

type Params struct {
	Page  int
	Limit int
	Sort  string
	Order string
}

// Parse reads page/limit/sort/order query params. sort must be a key in
// sortable (the caller's whitelist of columns that are safe to interpolate
// into ORDER BY) — anything else is rejected. This is the only place a raw
// query value is allowed near a sort column name; every repository must go
// through it rather than reading c.Query("sort") itself.
func Parse(c *fiber.Ctx, sortable map[string]string, defaultSort string) (Params, *apperror.Error) {
	page, _ := strconv.Atoi(c.Query("page", "1"))
	if page < 1 {
		page = 1
	}

	limit, _ := strconv.Atoi(c.Query("limit", "20"))
	if limit < 1 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}

	sort := c.Query("sort", defaultSort)
	if _, ok := sortable[sort]; !ok {
		return Params{}, apperror.Validation(map[string][]string{
			"sort": {"kolom sortir tidak dikenal"},
		})
	}

	order := strings.ToLower(c.Query("order", "desc"))
	if order != "asc" && order != "desc" {
		return Params{}, apperror.Validation(map[string][]string{
			"order": {"harus 'asc' atau 'desc'"},
		})
	}

	return Params{Page: page, Limit: limit, Sort: sort, Order: order}, nil
}

func (p Params) Offset() int {
	return (p.Page - 1) * p.Limit
}

// OrderByClause returns "<column> ASC|DESC" using ONLY the whitelisted SQL
// column name from sortable[p.Sort] — never the raw p.Sort value itself —
// so it is always safe to append directly to a query string.
//
// Parse already rejects an unknown sort/order before a Params ever reaches
// here, but a Params built by hand (bypassing Parse — e.g. a CLI command
// constructing one directly) has no such guarantee. A missing key used to
// silently produce a broken "ORDER BY  DESC" — a SQL syntax error, not an
// injection risk, but an opaque one. col falls back to the ordinal
// position "1" (always valid regardless of which columns exist); order
// falls back to DESC.
func (p Params) OrderByClause(sortable map[string]string) string {
	col, ok := sortable[p.Sort]
	if !ok {
		col = "1"
	}
	order := strings.ToUpper(p.Order)
	if order != "ASC" && order != "DESC" {
		order = "DESC"
	}
	return col + " " + order
}

func BuildMeta(page, limit, total int) response.Meta {
	totalPages := total / limit
	if total%limit != 0 {
		totalPages++
	}
	return response.Meta{Page: page, Limit: limit, Total: total, TotalPages: totalPages}
}
