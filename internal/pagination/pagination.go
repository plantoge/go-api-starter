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
func (p Params) OrderByClause(sortable map[string]string) string {
	col := sortable[p.Sort]
	return col + " " + strings.ToUpper(p.Order)
}

func BuildMeta(page, limit, total int) response.Meta {
	totalPages := total / limit
	if total%limit != 0 {
		totalPages++
	}
	return response.Meta{Page: page, Limit: limit, Total: total, TotalPages: totalPages}
}
