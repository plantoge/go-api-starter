package pagination

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
)

var userSortable = map[string]string{
	"created_at": "created_at",
	"name":       "name",
}

func parseWith(t *testing.T, target string) (Params, *fiber.Ctx) {
	t.Helper()
	app := fiber.New()
	var got Params
	var gotErr error
	app.Get("/x", func(c *fiber.Ctx) error {
		p, appErr := Parse(c, userSortable, "created_at")
		got = p
		if appErr != nil {
			gotErr = appErr
		}
		return nil
	})
	req := httptest.NewRequest(http.MethodGet, target, nil)
	_, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	_ = gotErr
	return got, nil
}

func TestParse_Defaults(t *testing.T) {
	p, _ := parseWith(t, "/x")
	if p.Page != 1 {
		t.Errorf("Page = %d, want 1", p.Page)
	}
	if p.Limit != 20 {
		t.Errorf("Limit = %d, want 20", p.Limit)
	}
	if p.Sort != "created_at" {
		t.Errorf("Sort = %q, want created_at", p.Sort)
	}
	if p.Order != "desc" {
		t.Errorf("Order = %q, want desc", p.Order)
	}
}

func TestParse_LimitClampedAt100(t *testing.T) {
	p, _ := parseWith(t, "/x?limit=500")
	if p.Limit != 100 {
		t.Errorf("Limit = %d, want clamped to 100", p.Limit)
	}
}

func TestParse_RejectsSortNotInWhitelist(t *testing.T) {
	app := fiber.New()
	var appErr *fiber.Error
	_ = appErr
	var gotIsNil bool
	app.Get("/x", func(c *fiber.Ctx) error {
		_, err := Parse(c, userSortable, "created_at")
		gotIsNil = err == nil
		return nil
	})
	req := httptest.NewRequest(http.MethodGet, "/x?sort=password_hash", nil)
	if _, err := app.Test(req); err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	if gotIsNil {
		t.Error("Parse() accepted a sort column outside the whitelist")
	}
}

func TestOffset(t *testing.T) {
	p := Params{Page: 3, Limit: 20}
	if got := p.Offset(); got != 40 {
		t.Errorf("Offset() = %d, want 40", got)
	}
}

func TestOrderByClause_UsesWhitelistedColumnOnly(t *testing.T) {
	p := Params{Sort: "name", Order: "asc"}
	if got := p.OrderByClause(userSortable); got != "name ASC" {
		t.Errorf("OrderByClause() = %q, want %q", got, "name ASC")
	}
}
