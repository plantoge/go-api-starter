package auth

import (
	"github.com/gofiber/fiber/v2"

	"go-api-starter/internal/apperror"
	"go-api-starter/internal/middleware"
	"go-api-starter/internal/response"
	"go-api-starter/internal/validator"
)

type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

func badBody(c *fiber.Ctx) error {
	return response.Error(c, middleware.RequestIDFromCtx(c),
		apperror.Validation(map[string][]string{"_": {"badan permintaan tidak valid"}}))
}

// Login godoc
// @Summary      Login staf tenant
// @Tags         tenant-auth
// @Accept       json
// @Produce      json
// @Param        body body LoginRequest true "tenant_code, email, dan password"
// @Success      200 {object} LoginResponse
// @Failure      401 {object} map[string]any
// @Router       /auth/login [post]
func (h *Handler) Login(c *fiber.Ctx) error {
	var req LoginRequest
	if err := c.BodyParser(&req); err != nil {
		return badBody(c)
	}
	if verr := validator.Validate(req); verr != nil {
		return response.Error(c, middleware.RequestIDFromCtx(c), verr)
	}
	res, err := h.svc.Login(c.UserContext(), req)
	if err != nil {
		return response.Error(c, middleware.RequestIDFromCtx(c), err)
	}
	return response.Success(c, 200, res)
}

// Refresh godoc
// @Summary      Perbarui access token tenant
// @Tags         tenant-auth
// @Accept       json
// @Produce      json
// @Param        body body RefreshRequest true "tenant_code dan refresh token"
// @Success      200 {object} LoginResponse
// @Failure      401 {object} map[string]any
// @Router       /auth/refresh [post]
func (h *Handler) Refresh(c *fiber.Ctx) error {
	var req RefreshRequest
	if err := c.BodyParser(&req); err != nil {
		return badBody(c)
	}
	if verr := validator.Validate(req); verr != nil {
		return response.Error(c, middleware.RequestIDFromCtx(c), verr)
	}
	res, err := h.svc.Refresh(c.UserContext(), req)
	if err != nil {
		return response.Error(c, middleware.RequestIDFromCtx(c), err)
	}
	return response.Success(c, 200, res)
}

// Logout godoc
// @Summary      Cabut refresh token tenant
// @Tags         tenant-auth
// @Accept       json
// @Produce      json
// @Param        body body LogoutRequest true "tenant_code dan refresh token yang mau dicabut"
// @Success      200 {object} map[string]any
// @Router       /auth/logout [post]
func (h *Handler) Logout(c *fiber.Ctx) error {
	var req LogoutRequest
	if err := c.BodyParser(&req); err != nil {
		return badBody(c)
	}
	if verr := validator.Validate(req); verr != nil {
		return response.Error(c, middleware.RequestIDFromCtx(c), verr)
	}
	if err := h.svc.Logout(c.UserContext(), req); err != nil {
		return response.Error(c, middleware.RequestIDFromCtx(c), err)
	}
	return response.Success(c, 200, fiber.Map{"message": "berhasil logout"})
}
