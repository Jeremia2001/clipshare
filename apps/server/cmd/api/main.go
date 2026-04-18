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
			INSERT INTO users (id, username, is_admin, storage_quota_bytes)
			VALUES ($1, 'dev', true, 5368709120)
			ON CONFLICT (id) DO NOTHING
		`, devUserID)
		if err != nil {
			log.Printf("Warning: could not seed dev user: %v", err)
			return
		}
		log.Println("Seeded dev user (dev)")
	}
}

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	db, err := sqlx.Connect("postgres", cfg.DatabaseURL())
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

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

	userRepo := repository.NewUserRepository(db)
	inviteRepo := repository.NewInviteCodeRepository(db)
	deviceRepo := repository.NewDeviceTokenRepository(db)
	setupRepo := repository.NewSetupTokenRepository(db)
	instanceRepo := repository.NewInstanceRepository(db)
	clipRepo := repository.NewClipRepository(db)
	shareRepo := repository.NewShareRepository(db)
	commentRepo := repository.NewCommentRepository(db)

	jwtManager := auth.NewJWTManager(
		cfg.Auth.JWTSecret,
		cfg.Auth.AccessTokenExpiry,
		cfg.Auth.RefreshTokenExpiry,
	)

	authService := services.NewAuthService(userRepo, inviteRepo, deviceRepo, setupRepo, jwtManager)
	instanceService := services.NewInstanceService(instanceRepo)

	clipService := services.NewClipService(
		clipRepo,
		shareRepo,
		userRepo,
		commentRepo,
		rustfsClient,
		cfg.Server.PublicURL,
	)

	authHandler := handlers.NewAuthHandler(authService)
	instanceHandler := handlers.NewInstanceHandler(instanceService)
	clipHandler := handlers.NewClipHandler(clipService, instanceService, jwtManager)

	app := fiber.New(fiber.Config{
		ErrorHandler: middleware.ErrorHandler,
		ReadTimeout:  5 * time.Minute,
		WriteTimeout: 0,       // no write timeout — video streaming can take arbitrarily long
		BodyLimit:    2 << 30, // 2GB
	})

	app.Use(recover.New())
	app.Use(logger.New())
	app.Use(compress.New(compress.Config{
		Next: func(c *fiber.Ctx) bool {
			ct := string(c.Response().Header.ContentType())
			return strings.HasPrefix(ct, "video/") || strings.HasPrefix(ct, "application/octet-stream")
		},
	}))
	app.Use(middleware.CORSConfig())

	// Auth middleware — populates user info for both JWT (admin) and device-token (regular user) credentials.
	app.Use(middleware.AuthMiddleware(jwtManager, authService))

	api := app.Group("/api/v1")
	authHandler.RegisterRoutes(api)
	instanceHandler.RegisterRoutes(api)
	clipHandler.RegisterRoutes(api)

	if cfg.Server.Environment == "development" {
		seedDevUser(context.Background(), db)
	} else {
		// Production: mint a one-time admin setup token on first launch.
		// Printed to stdout so the operator can read it from the container logs.
		if token, err := authService.EnsureSetupToken(context.Background()); err != nil {
			log.Printf("Warning: could not prepare setup token: %v", err)
		} else if token != "" {
			log.Println("========================================================")
			log.Println(" ClipShare admin setup required.")
			log.Println(" Use this one-time setup token to create the admin account:")
			log.Printf("   %s", token)
			log.Println(" This token is single-use and will not be shown again.")
			log.Println("========================================================")
		}
	}

	app.Get("/health", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"status": "healthy",
			"time":   time.Now().UTC(),
		})
	})

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)

	go func() {
		_ = <-c
		log.Println("Shutting down server...")
		_ = app.Shutdown()
	}()

	addr := cfg.Server.Host + ":" + strconv.Itoa(cfg.Server.Port)
	log.Printf("Server starting on %s", addr)
	if err := app.Listen(":" + strconv.Itoa(cfg.Server.Port)); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
