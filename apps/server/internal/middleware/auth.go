package middleware

import (
	"context"
	"os"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"

	"clipshare/internal/models"
	"clipshare/pkg/auth"
)

// DeviceValidator is satisfied by AuthService. It resolves a plaintext
// device token to the owning user. Passed in via a small interface so the
// middleware package doesn't depend on services.
type DeviceValidator interface {
	ValidateDeviceToken(ctx context.Context, token string) (*models.User, *models.DeviceToken, error)
}

// AuthMiddleware populates c.Locals with user info when a valid credential is
// presented. Tries JWT (admin sessions) first, then device token. Never
// rejects — RequireAuth is what enforces presence.
func AuthMiddleware(jwtManager *auth.JWTManager, devices DeviceValidator) fiber.Handler {
	return func(c *fiber.Ctx) error {
		tokenStr := extractBearer(c)
		if tokenStr == "" {
			return c.Next()
		}

		// Try JWT first (short token, signed). If ValidateToken succeeds, this is an admin session.
		if claims, err := jwtManager.ValidateToken(tokenStr); err == nil {
			c.Locals("user_id", claims.UserID)
			c.Locals("username", claims.Username)
			c.Locals("is_admin", claims.IsAdmin)
			return c.Next()
		}

		// Fall back to device token lookup.
		if devices != nil {
			user, _, err := devices.ValidateDeviceToken(c.Context(), tokenStr)
			if err == nil && user != nil {
				c.Locals("user_id", user.ID)
				c.Locals("username", user.Username)
				c.Locals("is_admin", user.IsAdmin)
			}
		}
		return c.Next()
	}
}

func extractBearer(c *fiber.Ctx) string {
	if h := c.Get("Authorization"); h != "" {
		parts := strings.SplitN(h, " ", 2)
		if len(parts) == 2 && strings.ToLower(parts[0]) == "bearer" {
			return parts[1]
		}
	}
	return c.Query("token")
}

// devUserID matches the UUID seeded by cmd/api/main.go:seedDevUser.
var devUserID = uuid.MustParse("00000000-0000-0000-0000-000000000001")

// forceDevUser overwrites whatever AuthMiddleware parsed from an incoming
// token. Dev mode means "every request is the dev user" — honoring a stale
// token from a previous real login would pin requests to a now-deleted user
// and cause FK violations on insert.
func forceDevUser(c *fiber.Ctx) {
	c.Locals("user_id", devUserID)
	c.Locals("username", "dev")
	c.Locals("is_admin", true)
}

func RequireAuth(c *fiber.Ctx) error {
	if isDev() {
		forceDevUser(c)
		return c.Next()
	}

	if c.Locals("user_id") == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "Authentication required",
		})
	}
	return c.Next()
}

func RequireAdmin(c *fiber.Ctx) error {
	if isDev() {
		forceDevUser(c)
		return c.Next()
	}
	if c.Locals("user_id") == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "Authentication required",
		})
	}
	isAdmin, ok := c.Locals("is_admin").(bool)
	if !ok || !isAdmin {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"error": "Admin access required",
		})
	}
	return c.Next()
}

func isDev() bool {
	return os.Getenv("ENV") == "development"
}

func CORSConfig() fiber.Handler {
	return func(c *fiber.Ctx) error {
		c.Set("Access-Control-Allow-Origin", "*")
		c.Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-App-URL")
		c.Set("Access-Control-Allow-Credentials", "true")

		if c.Method() == "OPTIONS" {
			return c.SendStatus(fiber.StatusNoContent)
		}

		return c.Next()
	}
}

func ErrorHandler(c *fiber.Ctx, err error) error {
	code := fiber.StatusInternalServerError
	message := "Internal server error"

	if e, ok := err.(*fiber.Error); ok {
		code = e.Code
		message = e.Message
	}

	return c.Status(code).JSON(fiber.Map{
		"error": message,
	})
}
