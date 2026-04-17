package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"clipshare/internal/config"
	"clipshare/internal/handlers"
	"clipshare/internal/middleware"
	"clipshare/internal/repository"
	"clipshare/internal/services"
	"clipshare/internal/storage"
	"clipshare/pkg/auth"
	"clipshare/pkg/email"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/compress"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
)

func seedDevUser(ctx context.Context, db *sqlx.DB) {
	const devUserID = "00000000-0000-0000-0000-000000000001"

	var exists bool
	err := db.GetContext(ctx, &exists, "SELECT EXISTS(SELECT 1 FROM users WHERE id = $1)", devUserID)
	if err != nil {
		log.Printf("Warning: could not check dev user: %v", err)
		return
	}

	if !exists {
		_, err = db.ExecContext(ctx, `
			INSERT INTO users (id, email, is_verified, is_admin, storage_quota_bytes)
			VALUES ($1, 'dev@localhost', true, true, 5368709120)
			ON CONFLICT (id) DO NOTHING
		`, devUserID)
		if err != nil {
			log.Printf("Warning: could not seed dev user: %v", err)
			return
		}
		log.Println("Seeded dev user (dev@localhost)")
	}
}

func main() {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Initialize database
	db, err := sqlx.Connect("postgres", cfg.DatabaseURL())
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	// Initialize RustFS client
	rustfsClient, err := storage.NewRustFSClient(
		cfg.RustFS.Endpoint,
		cfg.RustFS.AccessKey,
		cfg.RustFS.SecretKey,
		cfg.RustFS.UseSSL,
		cfg.RustFS.PublicEndpoint,
		cfg.RustFS.Buckets,
	)
	if err != nil {
		log.Fatalf("Failed to create RustFS client: %v", err)
	}

	// Initialize repositories
	userRepo := repository.NewUserRepository(db)
	magicTokenRepo := repository.NewMagicTokenRepository(db)
	refreshTokenRepo := repository.NewRefreshTokenRepository(db)
	clipRepo := repository.NewClipRepository(db)
	shareRepo := repository.NewShareRepository(db)

	// Initialize JWT manager
	jwtManager := auth.NewJWTManager(
		cfg.Auth.JWTSecret,
		cfg.Auth.AccessTokenExpiry,
		cfg.Auth.RefreshTokenExpiry,
	)

	// Initialize email service
	emailService := email.NewService(email.Config{
		Host:     cfg.Email.Host,
		Port:     cfg.Email.Port,
		Username: cfg.Email.Username,
		Password: cfg.Email.Password,
		From:     cfg.Email.From,
		FromName: cfg.Email.FromName,
		UseTLS:   cfg.Email.UseTLS,
	})

	// Initialize services
	authService := services.NewAuthService(
		userRepo,
		magicTokenRepo,
		refreshTokenRepo,
		jwtManager,
		emailService,
	)

	clipService := services.NewClipService(
		clipRepo,
		shareRepo,
		userRepo,
		rustfsClient,
		cfg.Server.PublicURL,
	)

	// Initialize handlers
	authHandler := handlers.NewAuthHandler(authService, jwtManager)
	clipHandler := handlers.NewClipHandler(clipService, jwtManager)

	// Create Fiber app
	app := fiber.New(fiber.Config{
		ErrorHandler: middleware.ErrorHandler,
		ReadTimeout:  5 * time.Minute,
		WriteTimeout: 0,       // no write timeout — video streaming can take arbitrarily long
		BodyLimit:    2 << 30, // 2GB
	})

	// Global middleware
	app.Use(recover.New())
	app.Use(logger.New())
	app.Use(compress.New(compress.Config{
		Next: func(c *fiber.Ctx) bool {
			ct := string(c.Response().Header.ContentType())
			return strings.HasPrefix(ct, "video/") || strings.HasPrefix(ct, "application/octet-stream")
		},
	}))
	app.Use(middleware.CORSConfig())

	// Auth middleware
	app.Use(middleware.AuthMiddleware(jwtManager))

	// API routes
	api := app.Group("/api/v1")
	authHandler.RegisterRoutes(api)
	clipHandler.RegisterRoutes(api)

	// Seed dev user in development mode
	if cfg.Server.Environment == "development" {
		seedDevUser(context.Background(), db)
	}

	// Health check
	app.Get("/health", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"status": "healthy",
			"time":   time.Now().UTC(),
		})
	})

	// Setup graceful shutdown
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)

	go func() {
		_ = <-c
		log.Println("Shutting down server...")
		_ = app.Shutdown()
	}()

	// Start server
	addr := cfg.Server.Host + ":" + strconv.Itoa(cfg.Server.Port)
	log.Printf("Server starting on %s", addr)
	if err := app.Listen(":" + strconv.Itoa(cfg.Server.Port)); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
