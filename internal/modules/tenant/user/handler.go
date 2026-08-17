package user

import (
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"

	"go-api-starter/internal/apperror"
	"go-api-starter/internal/middleware"
	"go-api-starter/internal/pagination"
	"go-api-starter/internal/response"
	"go-api-starter/internal/validator"
)

type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// Create godoc
// @Summary      Buat user tenant
// @Tags         users
// @Security     BearerAuth
// @Accept       json
// @Produce      json
// @Param        body body CreateRequest true "data user baru"
// @Success      201 {object} View
// @Failure      422 {object} map[string]any
// @Router       /users [post]
func (h *Handler) Create(c *fiber.Ctx) error {
	var req CreateRequest
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, middleware.RequestIDFromCtx(c),
			apperror.Validation(map[string][]string{"_": {"badan permintaan tidak valid"}}))
	}
	if verr := validator.Validate(req); verr != nil {
		return response.Error(c, middleware.RequestIDFromCtx(c), verr)
	}

	view, err := h.svc.Create(c.UserContext(), req)
	if err != nil {
		return response.Error(c, middleware.RequestIDFromCtx(c), err)
	}
	return response.Success(c, 201, view)
}

// Get godoc
// @Summary      Ambil satu user tenant berdasarkan ID
// @Tags         users
// @Security     BearerAuth
// @Produce      json
// @Param        id path string true "id user"
// @Success      200 {object} View
// @Failure      404 {object} map[string]any
// @Router       /users/{id} [get]
func (h *Handler) Get(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return response.Error(c, middleware.RequestIDFromCtx(c), apperror.NotFound("user tidak ditemukan"))
	}

	view, err := h.svc.Get(c.UserContext(), id)
	if err != nil {
		return response.Error(c, middleware.RequestIDFromCtx(c), err)
	}
	return response.Success(c, 200, view)
}

// List godoc
// @Summary      Daftar user tenant
// @Tags         users
// @Security     BearerAuth
// @Produce      json
// @Param        page query int false "nomor halaman"
// @Param        limit query int false "jumlah data per halaman, maksimal 100"
// @Param        sort query string false "created_at, name, atau email"
// @Param        order query string false "asc atau desc"
// @Success      200 {object} map[string]any
// @Router       /users [get]
func (h *Handler) List(c *fiber.Ctx) error {
	params, verr := pagination.Parse(c, Sortable, "created_at")
	if verr != nil {
		return response.Error(c, middleware.RequestIDFromCtx(c), verr)
	}

	views, meta, err := h.svc.List(c.UserContext(), params)
	if err != nil {
		return response.Error(c, middleware.RequestIDFromCtx(c), err)
	}
	return response.SuccessList(c, 200, views, meta)
}

// Update godoc
// @Summary      Perbarui user tenant
// @Tags         users
// @Security     BearerAuth
// @Accept       json
// @Produce      json
// @Param        id path string true "id user"
// @Param        body body UpdateRequest true "field yang mau diubah"
// @Success      200 {object} map[string]any
// @Failure      404 {object} map[string]any
// @Router       /users/{id} [patch]
func (h *Handler) Update(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return response.Error(c, middleware.RequestIDFromCtx(c), apperror.NotFound("user tidak ditemukan"))
	}

	var req UpdateRequest
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, middleware.RequestIDFromCtx(c),
			apperror.Validation(map[string][]string{"_": {"badan permintaan tidak valid"}}))
	}

	if err := h.svc.Update(c.UserContext(), id, req); err != nil {
		return response.Error(c, middleware.RequestIDFromCtx(c), err)
	}
	return response.Success(c, 200, fiber.Map{"message": "berhasil diperbarui"})
}

// Delete godoc
// @Summary      Hapus user tenant (soft delete)
// @Tags         users
// @Security     BearerAuth
// @Produce      json
// @Param        id path string true "id user"
// @Success      200 {object} map[string]any
// @Failure      404 {object} map[string]any
// @Router       /users/{id} [delete]
func (h *Handler) Delete(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return response.Error(c, middleware.RequestIDFromCtx(c), apperror.NotFound("user tidak ditemukan"))
	}

	if err := h.svc.Delete(c.UserContext(), id); err != nil {
		return response.Error(c, middleware.RequestIDFromCtx(c), err)
	}
	return response.Success(c, 200, fiber.Map{"message": "berhasil dihapus"})
}
