package handlers

import (
	"github.com/gofiber/fiber/v2"

	"clipshare/internal/middleware"
	"clipshare/internal/services"
)

type InstanceHandler struct {
	svc *services.InstanceService
}

func NewInstanceHandler(svc *services.InstanceService) *InstanceHandler {
	return &InstanceHandler{svc: svc}
}

func (h *InstanceHandler) RegisterRoutes(r fiber.Router) {
	g := r.Group("/instance")
	// Any authenticated user can see the server's storage state so their UI
	// can display "X GB of Y GB used" and disable uploads when full.
	g.Get("/storage", middleware.RequireAuth, h.GetStorage)
	g.Put("/storage", middleware.RequireAdmin, h.SetStorage)
}

func (h *InstanceHandler) GetStorage(c *fiber.Ctx) error {
	status, err := h.svc.GetStorageStatus(c.Context())
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(status)
}

type setStorageRequest struct {
	LimitBytes int64 `json:"limit_bytes"`
}

func (h *InstanceHandler) SetStorage(c *fiber.Ctx) error {
	var req setStorageRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid request body"})
	}
	if req.LimitBytes < 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "limit_bytes must be >= 0"})
	}
	if err := h.svc.SetStorageLimit(c.Context(), req.LimitBytes); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	status, err := h.svc.GetStorageStatus(c.Context())
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(status)
}
