package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

type Config struct {
	Server   ServerConfig
	Database DatabaseConfig
	RustFS   RustFSConfig
	Auth     AuthConfig
}

type ServerConfig struct {
	Host        string
	Port        int
	Environment string
	PublicURL   string
	FrontendURL string
}

type DatabaseConfig struct {
	Host     string
	Port     int
	User     string
	Password string
	Database string
	SSLMode  string
}

type RustFSConfig struct {
	Endpoint       string
	PublicEndpoint string
	AccessKey      string
	SecretKey      string
	UseSSL         bool
	Buckets        BucketConfig
}

type BucketConfig struct {
	Clips      string
	Thumbnails string
	Processed  string
}

type AuthConfig struct {
	JWTSecret          string
	AccessTokenExpiry  time.Duration
	RefreshTokenExpiry time.Duration
}

func Load() (*Config, error) {
	cfg := &Config{
		Server: ServerConfig{
			Host:        getEnv("SERVER_HOST", "0.0.0.0"),
			Port:        getEnvAsInt("SERVER_PORT", 8080),
			Environment: getEnv("ENV", "development"),
			PublicURL:   getEnv("SERVER_PUBLIC_URL", "http://localhost:8080"),
			FrontendURL: getEnv("FRONTEND_URL", "http://localhost:3000"),
		},
		Database: DatabaseConfig{
			Host:     getEnv("DB_HOST", "localhost"),
			Port:     getEnvAsInt("DB_PORT", 5432),
			User:     getEnv("DB_USER", "clipshare"),
			Password: getEnv("DB_PASSWORD", "clipshare"),
			Database: getEnv("DB_NAME", "clipshare"),
			SSLMode:  getEnv("DB_SSLMODE", "disable"),
		},
		RustFS: RustFSConfig{
			Endpoint:       getEnv("RUSTFS_ENDPOINT", "localhost:9000"),
			PublicEndpoint: getEnv("RUSTFS_PUBLIC_ENDPOINT", ""),
			AccessKey:      getEnv("RUSTFS_ACCESS_KEY", "clipshare"),
			SecretKey:      getEnv("RUSTFS_SECRET_KEY", "clipshare123"),
			UseSSL:         getEnvAsBool("RUSTFS_USE_SSL", false),
			Buckets: BucketConfig{
				Clips:      getEnv("RUSTFS_BUCKET_CLIPS", "clips-raw"),
				Thumbnails: getEnv("RUSTFS_BUCKET_THUMBNAILS", "thumbnails"),
				Processed:  getEnv("RUSTFS_BUCKET_PROCESSED", "clips-processed"),
			},
		},
		Auth: AuthConfig{
			JWTSecret:          getEnv("JWT_SECRET", "your-super-secret-key-change-this-in-production"),
			AccessTokenExpiry:  getEnvAsDuration("ACCESS_TOKEN_EXPIRY", 24*time.Hour),
			RefreshTokenExpiry: getEnvAsDuration("REFRESH_TOKEN_EXPIRY", 7*24*time.Hour),
		},
	}

	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return cfg, nil
}

func (c *Config) Validate() error {
	if c.Auth.JWTSecret == "your-super-secret-key-change-this-in-production" && c.Server.Environment == "production" {
		return fmt.Errorf("JWT_SECRET must be set in production")
	}
	return nil
}

func (c *Config) DatabaseURL() string {
	return fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=%s",
		c.Database.User,
		c.Database.Password,
		c.Database.Host,
		c.Database.Port,
		c.Database.Database,
		c.Database.SSLMode,
	)
}

func getEnv(key, defaultValue string) string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	return value
}

func getEnvAsInt(key string, defaultValue int) int {
	valueStr := os.Getenv(key)
	if valueStr == "" {
		return defaultValue
	}
	value, err := strconv.Atoi(valueStr)
	if err != nil {
		return defaultValue
	}
	return value
}

func getEnvAsBool(key string, defaultValue bool) bool {
	valueStr := os.Getenv(key)
	if valueStr == "" {
		return defaultValue
	}
	value, err := strconv.ParseBool(valueStr)
	if err != nil {
		return defaultValue
	}
	return value
}

func getEnvAsDuration(key string, defaultValue time.Duration) time.Duration {
	valueStr := os.Getenv(key)
	if valueStr == "" {
		return defaultValue
	}
	value, err := time.ParseDuration(valueStr)
	if err != nil {
		return defaultValue
	}
	return value
}
