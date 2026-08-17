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

// Parse baca query param page/limit/sort/order. Nilai sort wajib ada
// sebagai kunci di sortable — whitelist kolom milik pemanggil yang memang
// aman ditempel ke ORDER BY — selain itu ditolak. Cuma di sinilah nilai
// query mentah boleh berdekatan dengan nama kolom sortir; semua repository
// wajib lewat fungsi ini, jangan baca c.Query("sort") sendiri.
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

// OrderByClause ngasih balik "<kolom> ASC|DESC" dengan HANYA memakai nama
// kolom SQL dari whitelist sortable[p.Sort] — nggak pernah nilai mentah
// p.Sort — jadi hasilnya selalu aman ditempel langsung ke string query.
//
// Parse sebenarnya sudah nolak sort/order yang nggak dikenal sebelum
// Params sampai ke sini. Tapi Params yang dirakit manual (lewat jalur yang
// nggak pakai Parse — misalnya perintah CLI yang bikin sendiri) nggak
// dapat jaminan itu. Dulu kunci yang nggak ketemu diam-diam menghasilkan
// "ORDER BY  DESC" yang rusak — bukan celah injection, tapi error sintaks
// SQL yang bikin bingung. Sekarang col jatuh ke posisi ordinal "1" (selalu
// sah, apa pun kolom yang ada), dan order jatuh ke DESC.
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
