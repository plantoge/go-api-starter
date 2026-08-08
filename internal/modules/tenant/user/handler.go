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
