package handlers

import (
	"errors"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"

	"clipshare/internal/middleware"
	"clipshare/internal/services"
)

type AuthHandler struct {
	authService *services.AuthService
}

func NewAuthHandler(authService *services.AuthService) *AuthHandler {
	return &AuthHandler{authService: authService}
}

func (h *AuthHandler) RegisterRoutes(r fiber.Router) {
	g := r.Group("/auth")
	g.Get("/status", h.Status)
	g.Post("/setup", h.SetupAdmin)
	g.Post("/login", h.AdminLogin)
	g.Post("/redeem", h.RedeemInvite)
	g.Get("/me", h.GetCurrentUser)
	g.Delete("/logout", h.Logout)

	// Admin-only invite management.
	g.Post("/invites", middleware.RequireAdmin, h.CreateInvite)
	g.Get("/invites", middleware.RequireAdmin, h.ListInvites)
	g.Delete("/invites/:id", middleware.RequireAdmin, h.DeleteInvite)
}

func (h *AuthHandler) Status(c *fiber.Ctx) error {
	status, err := h.authService.Status(c.Context())
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(status)
}

func (h *AuthHandler) SetupAdmin(c *fiber.Ctx) error {
	var req services.SetupAdminRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid request body"})
	}
	resp, err := h.authService.SetupAdmin(c.Context(), req)
	if err != nil {
		switch {
		case errors.Is(err, services.ErrAdminExists):
			return c.Status(fiber.StatusConflict).JSON(fiber.Map{"error": err.Error()})
		case errors.Is(err, services.ErrInvalidSetup):
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": err.Error()})
		default:
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
		}
	}
	return c.JSON(resp)
}

func (h *AuthHandler) AdminLogin(c *fiber.Ctx) error {
	var req services.LoginRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid request body"})
	}
	resp, err := h.authService.AdminLogin(c.Context(), req)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Invalid username or password"})
	}
	return c.JSON(resp)
}

func (h *AuthHandler) RedeemInvite(c *fiber.Ctx) error {
	var req services.RedeemRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid request body"})
	}
	resp, err := h.authService.RedeemInvite(c.Context(), req)
	if err != nil {
		switch {
		case errors.Is(err, services.ErrUserHasDevice):
			return c.Status(fiber.StatusConflict).JSON(fiber.Map{
				"error": "this account already has a registered device; ask the admin for a new invite on a different username",
			})
		default:
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Invalid or expired invite code"})
		}
	}
	return c.JSON(resp)
}

func (h *AuthHandler) CreateInvite(c *fiber.Ctx) error {
	userID, ok := c.Locals("user_id").(uuid.UUID)
	if !ok {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Not authenticated"})
	}
	var req services.CreateInviteRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid request body"})
	}
	resp, err := h.authService.CreateInvite(c.Context(), userID, req)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(resp)
}

func (h *AuthHandler) ListInvites(c *fiber.Ctx) error {
	rows, err := h.authService.ListInvites(c.Context())
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"invites": rows})
}

func (h *AuthHandler) DeleteInvite(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid invite id"})
	}
	if err := h.authService.DeleteInvite(c.Context(), id); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"success": true})
}

func (h *AuthHandler) Logout(c *fiber.Ctx) error {
	userID, ok := c.Locals("user_id").(uuid.UUID)
	if !ok {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Not authenticated"})
	}
	if err := h.authService.LogoutDevice(c.Context(), userID); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"success": true})
}

func (h *AuthHandler) GetCurrentUser(c *fiber.Ctx) error {
	userID, ok := c.Locals("user_id").(uuid.UUID)
	if !ok {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Not authenticated"})
	}
	user, err := h.authService.GetUser(c.Context(), userID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(user)
}
