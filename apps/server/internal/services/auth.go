package services

import (
	"context"
	"fmt"
	"time"

	"clipshare/internal/models"
	"clipshare/internal/repository"
	"clipshare/pkg/auth"
	"clipshare/pkg/email"

	"github.com/google/uuid"
)

type AuthService struct {
	userRepo      repository.UserRepository
	magicRepo     repository.MagicTokenRepository
	refreshRepo   repository.RefreshTokenRepository
	jwtManager    *auth.JWTManager
	eemailService *email.Service
}

func NewAuthService(
	userRepo repository.UserRepository,
	magicRepo repository.MagicTokenRepository,
	refreshRepo repository.RefreshTokenRepository,
	jwtManager *auth.JWTManager,
	emailService *email.Service,
) *AuthService {
	return &AuthService{
		userRepo:      userRepo,
		magicRepo:     magicRepo,
		refreshRepo:   refreshRepo,
		jwtManager:    jwtManager,
		eemailService: emailService,
	}
}

type MagicLinkRequest struct {
	Email string `json:"email" validate:"required,email"`
}

type MagicLinkResponse struct {
	Success  bool   `json:"success"`
	Message  string `json:"message"`
	DevToken string `json:"dev_token,omitempty"` // For development only
}

type VerifyRequest struct {
	Token  string `json:"token" validate:"required"`
	AppURL string `json:"app_url" validate:"required"`
}

type VerifyResponse struct {
	AccessToken  string       `json:"access_token"`
	RefreshToken string       `json:"refresh_token"`
	ExpiresIn    int          `json:"expires_in"`
	User         *models.User `json:"user"`
}

type RefreshRequest struct {
	RefreshToken string `json:"refresh_token" validate:"required"`
}

type RefreshResponse struct {
	AccessToken string `json:"access_token"`
	ExpiresIn   int    `json:"expires_in"`
}

func (s *AuthService) RequestMagicLink(ctx context.Context, email, appURL string) (*MagicLinkResponse, error) {
	// Find or create user
	user, err := s.userRepo.GetByEmail(ctx, email)
	if err != nil {
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	if user == nil {
		// Create new user
		user, err = s.userRepo.Create(ctx, email)
		if err != nil {
			return nil, fmt.Errorf("failed to create user: %w", err)
		}
	}

	// Generate magic token
	token, tokenHash, err := auth.GenerateMagicToken()
	if err != nil {
		return nil, fmt.Errorf("failed to generate token: %w", err)
	}

	// Store token hash
	expiresAt := time.Now().Add(15 * time.Minute)
	_, err = s.magicRepo.Create(ctx, user.ID, tokenHash, expiresAt)
	if err != nil {
		return nil, fmt.Errorf("failed to store token: %w", err)
	}

	// Send email (if configured)
	if s.eemailService != nil && s.eemailService.IsConfigured() {
		if err := s.eemailService.SendMagicLink(email, token, appURL); err != nil {
			// In development, return token directly
			return &MagicLinkResponse{
				Success:  true,
				Message:  "Magic link generated (email not sent - use dev_token)",
				DevToken: token,
			}, nil
		}
	}

	return &MagicLinkResponse{
		Success:  true,
		Message:  "Magic link sent to your email",
		DevToken: token, // Always return in development
	}, nil
}

func (s *AuthService) VerifyMagicLink(ctx context.Context, token, appURL string) (*VerifyResponse, error) {
	// Hash the token and look it up
	tokenHash := auth.HashToken(token)

	magicToken, err := s.magicRepo.GetByHash(ctx, tokenHash)
	if err != nil {
		return nil, fmt.Errorf("failed to get token: %w", err)
	}

	if magicToken == nil {
		return nil, fmt.Errorf("invalid or expired token")
	}

	// Mark token as used
	if err := s.magicRepo.MarkUsed(ctx, magicToken.ID); err != nil {
		return nil, fmt.Errorf("failed to mark token used: %w", err)
	}

	// Get user
	user, err := s.userRepo.GetByID(ctx, magicToken.UserID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	if user == nil {
		return nil, fmt.Errorf("user not found")
	}

	// Update last login
	if err := s.userRepo.UpdateLastLogin(ctx, user.ID); err != nil {
		// Log but don't fail
		_ = err
	}

	// Verify email if not already
	if !user.IsVerified {
		if err := s.userRepo.VerifyEmail(ctx, user.ID); err != nil {
			_ = err
		}
	}

	// Generate tokens
	accessToken, err := s.jwtManager.GenerateAccessToken(user.ID, user.Email, user.IsAdmin)
	if err != nil {
		return nil, fmt.Errorf("failed to generate access token: %w", err)
	}

	refreshToken, refreshHash, err := s.jwtManager.GenerateRefreshToken(user.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to generate refresh token: %w", err)
	}

	// Store refresh token
	refreshExpiresAt := time.Now().Add(7 * 24 * time.Hour)
	_, err = s.refreshRepo.Create(ctx, user.ID, refreshHash, refreshExpiresAt)
	if err != nil {
		return nil, fmt.Errorf("failed to store refresh token: %w", err)
	}

	return &VerifyResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresIn:    int(s.jwtManager.AccessTokenExpiry().Seconds()),
		User:         user,
	}, nil
}

func (s *AuthService) RefreshToken(ctx context.Context, refreshToken string) (*RefreshResponse, error) {
	// Hash and lookup
	tokenHash := auth.HashToken(refreshToken)

	storedToken, err := s.refreshRepo.GetByHash(ctx, tokenHash)
	if err != nil {
		return nil, fmt.Errorf("failed to get refresh token: %w", err)
	}

	if storedToken == nil {
		return nil, fmt.Errorf("invalid refresh token")
	}

	// Get user
	user, err := s.userRepo.GetByID(ctx, storedToken.UserID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	if user == nil {
		return nil, fmt.Errorf("user not found")
	}

	// Generate new access token
	accessToken, err := s.jwtManager.GenerateAccessToken(user.ID, user.Email, user.IsAdmin)
	if err != nil {
		return nil, fmt.Errorf("failed to generate access token: %w", err)
	}

	return &RefreshResponse{
		AccessToken: accessToken,
		ExpiresIn:   int(s.jwtManager.AccessTokenExpiry().Seconds()),
	}, nil
}

func (s *AuthService) Logout(ctx context.Context, userID uuid.UUID, refreshToken string) error {
	if refreshToken != "" {
		tokenHash := auth.HashToken(refreshToken)
		storedToken, err := s.refreshRepo.GetByHash(ctx, tokenHash)
		if err == nil && storedToken != nil {
			_ = s.refreshRepo.Revoke(ctx, storedToken.ID)
		}
	}
	return nil
}

func (s *AuthService) LogoutAll(ctx context.Context, userID uuid.UUID) error {
	return s.refreshRepo.RevokeAllForUser(ctx, userID)
}

func (s *AuthService) GetUser(ctx context.Context, userID uuid.UUID) (*models.User, error) {
	return s.userRepo.GetByID(ctx, userID)
}
