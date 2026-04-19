package services

import (
	"context"
	"errors"
	"fmt"
	"time"

	"clipshare/internal/models"
	"clipshare/internal/repository"
	"clipshare/pkg/auth"

	"github.com/google/uuid"
)

var (
	ErrAdminExists      = errors.New("admin already set up")
	ErrInvalidSetup     = errors.New("invalid setup token")
	ErrInvalidCreds     = errors.New("invalid credentials")
	ErrInviteInvalid    = errors.New("invite code invalid or already used")
	ErrDeviceNotFound   = errors.New("device not recognized")
	ErrUserHasDevice    = errors.New("user already has a registered device")
)

type AuthService struct {
	userRepo   repository.UserRepository
	inviteRepo repository.InviteCodeRepository
	deviceRepo repository.DeviceTokenRepository
	setupRepo  repository.SetupTokenRepository
	jwtManager *auth.JWTManager
}

func NewAuthService(
	userRepo repository.UserRepository,
	inviteRepo repository.InviteCodeRepository,
	deviceRepo repository.DeviceTokenRepository,
	setupRepo repository.SetupTokenRepository,
	jwtManager *auth.JWTManager,
) *AuthService {
	return &AuthService{
		userRepo:   userRepo,
		inviteRepo: inviteRepo,
		deviceRepo: deviceRepo,
		setupRepo:  setupRepo,
		jwtManager: jwtManager,
	}
}

type SetupStatus struct {
	NeedsSetup bool `json:"needs_setup"`
}

// -------------------- Bootstrap --------------------

func (s *AuthService) Status(ctx context.Context) (*SetupStatus, error) {
	adminExists, err := s.userRepo.AnyAdminExists(ctx)
	if err != nil {
		return nil, err
	}
	return &SetupStatus{NeedsSetup: !adminExists}, nil
}

// EnsureSetupToken creates a one-time setup token iff no admin exists and
// no unused setup token is already persisted. Returns the plaintext token
// (empty string if none was created — either admin exists or an unused
// token is already present and still valid).
func (s *AuthService) EnsureSetupToken(ctx context.Context) (string, error) {
	adminExists, err := s.userRepo.AnyAdminExists(ctx)
	if err != nil {
		return "", err
	}
	if adminExists {
		// No need for a setup token — drop any leftover ones to keep state clean.
		_ = s.setupRepo.DeleteAll(ctx)
		return "", nil
	}
	unused, err := s.setupRepo.AnyUnused(ctx)
	if err != nil {
		return "", err
	}
	if unused {
		// Can't surface the plaintext of an existing token (only hash is stored).
		// Rotate: wipe and mint a fresh one so the admin can re-read it from logs.
		if err := s.setupRepo.DeleteAll(ctx); err != nil {
			return "", err
		}
	}
	token, err := auth.GenerateCode(4, 5)
	if err != nil {
		return "", err
	}
	if err := s.setupRepo.Create(ctx, auth.HashToken(auth.NormalizeCode(token))); err != nil {
		return "", err
	}
	return token, nil
}

type SetupAdminRequest struct {
	SetupToken string `json:"setup_token"`
	Username   string `json:"username"`
	Password   string `json:"password"`
}

type LoginResponse struct {
	AccessToken string       `json:"access_token"`
	ExpiresIn   int          `json:"expires_in"`
	User        *models.User `json:"user"`
}

func (s *AuthService) SetupAdmin(ctx context.Context, req SetupAdminRequest) (*LoginResponse, error) {
	if req.Username == "" || req.Password == "" || req.SetupToken == "" {
		return nil, fmt.Errorf("username, password, and setup_token are required")
	}
	if len(req.Username) > 32 {
		return nil, fmt.Errorf("username must be 32 characters or fewer")
	}
	if len(req.Password) < 8 {
		return nil, fmt.Errorf("password must be at least 8 characters")
	}
	adminExists, err := s.userRepo.AnyAdminExists(ctx)
	if err != nil {
		return nil, err
	}
	if adminExists {
		return nil, ErrAdminExists
	}
	ok, err := s.setupRepo.ConsumeByHash(ctx, auth.HashToken(auth.NormalizeCode(req.SetupToken)))
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, ErrInvalidSetup
	}
	hash, err := auth.HashPassword(req.Password)
	if err != nil {
		return nil, err
	}
	user, err := s.userRepo.CreateAdmin(ctx, req.Username, hash)
	if err != nil {
		return nil, fmt.Errorf("create admin: %w", err)
	}
	// No more setup tokens needed.
	_ = s.setupRepo.DeleteAll(ctx)

	accessToken, err := s.jwtManager.GenerateAccessToken(user.ID, user.Username, user.IsAdmin)
	if err != nil {
		return nil, err
	}
	return &LoginResponse{
		AccessToken: accessToken,
		ExpiresIn:   int(s.jwtManager.AccessTokenExpiry().Seconds()),
		User:        user,
	}, nil
}

// -------------------- Admin login --------------------

type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func (s *AuthService) AdminLogin(ctx context.Context, req LoginRequest) (*LoginResponse, error) {
	if req.Username == "" || req.Password == "" {
		return nil, ErrInvalidCreds
	}
	user, err := s.userRepo.GetByUsername(ctx, req.Username)
	if err != nil {
		return nil, err
	}
	if user == nil || !user.IsAdmin || user.PasswordHash == nil {
		return nil, ErrInvalidCreds
	}
	ok, err := auth.VerifyPassword(req.Password, *user.PasswordHash)
	if err != nil || !ok {
		return nil, ErrInvalidCreds
	}
	_ = s.userRepo.UpdateLastLogin(ctx, user.ID)

	accessToken, err := s.jwtManager.GenerateAccessToken(user.ID, user.Username, user.IsAdmin)
	if err != nil {
		return nil, err
	}
	return &LoginResponse{
		AccessToken: accessToken,
		ExpiresIn:   int(s.jwtManager.AccessTokenExpiry().Seconds()),
		User:        user,
	}, nil
}

// -------------------- Invites --------------------

type CreateInviteRequest struct {
	Note          string `json:"note,omitempty"`
	ExpiresInDays int    `json:"expires_in_days,omitempty"`
}

type CreateInviteResponse struct {
	Code    string    `json:"code"`
	Invite  *InviteView `json:"invite"`
}

type InviteView struct {
	ID         uuid.UUID  `json:"id"`
	Note       *string    `json:"note,omitempty"`
	RedeemedBy *uuid.UUID `json:"redeemed_by,omitempty"`
	RedeemedAt *time.Time `json:"redeemed_at,omitempty"`
	ExpiresAt  *time.Time `json:"expires_at,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
}

func toInviteView(inv *models.InviteCode) *InviteView {
	return &InviteView{
		ID: inv.ID, Note: inv.Note,
		RedeemedBy: inv.RedeemedBy, RedeemedAt: inv.RedeemedAt,
		ExpiresAt: inv.ExpiresAt, CreatedAt: inv.CreatedAt,
	}
}

func (s *AuthService) CreateInvite(ctx context.Context, adminID uuid.UUID, req CreateInviteRequest) (*CreateInviteResponse, error) {
	code, err := auth.GenerateCode(3, 4)
	if err != nil {
		return nil, err
	}
	var note *string
	if req.Note != "" {
		n := req.Note
		note = &n
	}
	var expiresAt *time.Time
	if req.ExpiresInDays > 0 {
		t := time.Now().Add(time.Duration(req.ExpiresInDays) * 24 * time.Hour)
		expiresAt = &t
	}
	inv, err := s.inviteRepo.Create(ctx, auth.HashToken(auth.NormalizeCode(code)), adminID, note, expiresAt)
	if err != nil {
		return nil, err
	}
	return &CreateInviteResponse{Code: code, Invite: toInviteView(inv)}, nil
}

func (s *AuthService) ListInvites(ctx context.Context) ([]*InviteView, error) {
	rows, err := s.inviteRepo.List(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]*InviteView, 0, len(rows))
	for _, r := range rows {
		out = append(out, toInviteView(r))
	}
	return out, nil
}

func (s *AuthService) DeleteInvite(ctx context.Context, id uuid.UUID) error {
	return s.inviteRepo.Delete(ctx, id)
}

// -------------------- Invite redemption --------------------

type RedeemRequest struct {
	Code        string `json:"code"`
	Username    string `json:"username"`
	DeviceLabel string `json:"device_label,omitempty"`
}

type RedeemResponse struct {
	DeviceToken string       `json:"device_token"`
	User        *models.User `json:"user"`
}

func (s *AuthService) RedeemInvite(ctx context.Context, req RedeemRequest) (*RedeemResponse, error) {
	if req.Code == "" || req.Username == "" {
		return nil, ErrInviteInvalid
	}
	if len(req.Username) > 32 {
		return nil, fmt.Errorf("username must be 32 characters or fewer")
	}
	inv, err := s.inviteRepo.GetByHash(ctx, auth.HashToken(auth.NormalizeCode(req.Code)))
	if err != nil {
		return nil, err
	}
	if inv == nil || inv.RedeemedAt != nil {
		return nil, ErrInviteInvalid
	}
	if inv.ExpiresAt != nil && inv.ExpiresAt.Before(time.Now()) {
		return nil, ErrInviteInvalid
	}

	// Find or create the user for this username.
	user, err := s.userRepo.GetByUsername(ctx, req.Username)
	if err != nil {
		return nil, err
	}
	if user == nil {
		user, err = s.userRepo.Create(ctx, req.Username)
		if err != nil {
			return nil, fmt.Errorf("create user: %w", err)
		}
	}

	// One user = one device. If there's already a token, this invite attempt fails.
	existing, err := s.deviceRepo.GetByUserID(ctx, user.ID)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return nil, ErrUserHasDevice
	}

	// Atomically mark the invite redeemed. Prevents double-spending.
	if err := s.inviteRepo.MarkRedeemed(ctx, inv.ID, user.ID); err != nil {
		return nil, ErrInviteInvalid
	}

	token, err := auth.GenerateDeviceToken()
	if err != nil {
		return nil, err
	}
	var label *string
	if req.DeviceLabel != "" {
		l := req.DeviceLabel
		label = &l
	}
	if _, err := s.deviceRepo.Create(ctx, user.ID, auth.HashToken(token), label); err != nil {
		return nil, err
	}

	_ = s.userRepo.UpdateLastLogin(ctx, user.ID)
	return &RedeemResponse{DeviceToken: token, User: user}, nil
}

// ValidateDeviceToken is used by middleware to look up a device token.
// Returns the associated user + device token record, or nil if unknown.
func (s *AuthService) ValidateDeviceToken(ctx context.Context, token string) (*models.User, *models.DeviceToken, error) {
	tok, err := s.deviceRepo.GetByHash(ctx, auth.HashToken(token))
	if err != nil {
		return nil, nil, err
	}
	if tok == nil {
		return nil, nil, nil
	}
	user, err := s.userRepo.GetByID(ctx, tok.UserID)
	if err != nil {
		return nil, nil, err
	}
	if user == nil {
		return nil, nil, nil
	}
	_ = s.deviceRepo.Touch(ctx, tok.ID)
	return user, tok, nil
}

func (s *AuthService) LogoutDevice(ctx context.Context, userID uuid.UUID) error {
	return s.deviceRepo.DeleteByUserID(ctx, userID)
}

func (s *AuthService) GetUser(ctx context.Context, userID uuid.UUID) (*models.User, error) {
	return s.userRepo.GetByID(ctx, userID)
}
