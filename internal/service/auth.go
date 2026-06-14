package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"html"
	"time"

	"github.com/google/uuid"
	"github.com/ilhamazhar/golang-gpt/internal/domain"
	"github.com/ilhamazhar/golang-gpt/pkg/jwt"
	"github.com/ilhamazhar/golang-gpt/pkg/password"
)

type authService struct {
	users         domain.UserRepository
	store         domain.TokenStore
	mailer        domain.Mailer
	access        *jwt.Manager
	refresh       *jwt.Manager
	refreshExpiry time.Duration
	verifyExpiry  time.Duration
	appBaseURL    string
	exposeToken   bool // when true, verification tokens are returned via the API (dev only)
}

func NewAuthService(
	users domain.UserRepository,
	store domain.TokenStore,
	mailer domain.Mailer,
	access, refresh *jwt.Manager,
	refreshExpiry, verifyExpiry time.Duration,
	appBaseURL string,
	exposeToken bool,
) domain.AuthService {
	return &authService{
		users:         users,
		store:         store,
		mailer:        mailer,
		access:        access,
		refresh:       refresh,
		refreshExpiry: refreshExpiry,
		verifyExpiry:  verifyExpiry,
		appBaseURL:    appBaseURL,
		exposeToken:   exposeToken,
	}
}

func (s *authService) Register(ctx context.Context, req domain.RegisterRequest) (domain.RegisterResponse, error) {
	hash, err := password.Hash(req.Password, password.DefaultParams)
	if err != nil {
		return domain.RegisterResponse{}, err
	}

	user := &domain.User{
		Name:         req.Name,
		Email:        req.Email,
		PasswordHash: hash,
	}

	if err := s.users.Create(ctx, user); err != nil {
		return domain.RegisterResponse{}, errors.New("email already registered")
	}

	token, err := s.issueVerification(ctx, user)
	if err != nil {
		return domain.RegisterResponse{}, err
	}

	resp := domain.RegisterResponse{User: domain.ToUserResponse(user)}
	if s.exposeToken {
		resp.VerificationToken = token
	}
	return resp, nil
}

// issueVerification creates a one-time verification token, stores it, and
// "sends" the verification link. With no email provider configured it logs the
// link; swap this for a real mailer in production.
func (s *authService) issueVerification(ctx context.Context, user *domain.User) (string, error) {
	token, err := randomToken()
	if err != nil {
		return "", err
	}
	if err := s.store.SaveVerification(ctx, token, user.ID.String(), s.verifyExpiry); err != nil {
		return "", fmt.Errorf("store verification token: %w", err)
	}

	link := fmt.Sprintf("%s/auth/verify?token=%s", s.appBaseURL, token)
	subject := "Verify your email"
	body := fmt.Sprintf(
		`<p>Hi %s,</p>`+
			`<p>Thanks for registering. Please confirm your email address by clicking the link below:</p>`+
			`<p><a href="%s">Verify my email</a></p>`+
			`<p>Or paste this URL into your browser:<br>%s</p>`+
			`<p>This link expires in %.0f hours. If you didn't create this account, you can ignore this email.</p>`,
		html.EscapeString(user.Name), link, link, s.verifyExpiry.Hours(),
	)

	if err := s.mailer.Send(ctx, user.Email, subject, body); err != nil {
		return "", fmt.Errorf("send verification email: %w", err)
	}
	return token, nil
}

func randomToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate token: %w", err)
	}
	return hex.EncodeToString(b), nil
}

func (s *authService) Login(ctx context.Context, req domain.LoginRequest) (*domain.TokenResponse, error) {
	user, err := s.users.FindByEmail(ctx, req.Email)
	if err != nil {
		return nil, errors.New("invalid credentials")
	}

	match, err := password.Verify(req.Password, user.PasswordHash)
	if err != nil || !match {
		return nil, errors.New("invalid credentials")
	}

	if user.EmailVerifiedAt == nil {
		return nil, domain.ErrEmailNotVerified
	}

	accessToken, err := s.access.Generate(user.ID, user.Email)
	if err != nil {
		return nil, err
	}

	refreshToken, err := s.refresh.Generate(user.ID, user.Email)
	if err != nil {
		return nil, err
	}

	if err := s.store.Save(ctx, refreshToken, user.ID.String(), s.refreshExpiry); err != nil {
		return nil, fmt.Errorf("store refresh token: %w", err)
	}

	return &domain.TokenResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		TokenType:    "Bearer",
		ExpiresIn:    int64(s.access.Expiry().Seconds()),
		User:         domain.ToUserResponse(user),
	}, nil
}

func (s *authService) VerifyEmail(ctx context.Context, token string) error {
	if token == "" {
		return errors.New("verification token is required")
	}

	userID, err := s.store.GetVerification(ctx, token)
	if err != nil {
		return errors.New("invalid or expired verification token")
	}

	id, err := uuid.Parse(userID)
	if err != nil {
		return errors.New("invalid verification token")
	}

	user, err := s.users.FindByID(ctx, id)
	if err != nil {
		return errors.New("user not found")
	}

	if user.EmailVerifiedAt == nil {
		now := time.Now()
		user.EmailVerifiedAt = &now
		if err := s.users.Update(ctx, user); err != nil {
			return fmt.Errorf("mark email verified: %w", err)
		}
	}

	// One-time use: drop the token whether or not it had already been consumed.
	if err := s.store.RevokeVerification(ctx, token); err != nil {
		return fmt.Errorf("revoke verification token: %w", err)
	}
	return nil
}

func (s *authService) ResendVerification(ctx context.Context, email string) (string, error) {
	user, err := s.users.FindByEmail(ctx, email)
	if err != nil {
		// Don't reveal whether the email exists.
		return "", nil
	}
	if user.EmailVerifiedAt != nil {
		return "", errors.New("email already verified")
	}

	token, err := s.issueVerification(ctx, user)
	if err != nil {
		return "", err
	}
	if s.exposeToken {
		return token, nil
	}
	return "", nil
}

func (s *authService) RefreshToken(ctx context.Context, refreshToken string) (*domain.TokenResponse, error) {
	claims, err := s.refresh.Verify(refreshToken)
	if err != nil {
		return nil, errors.New("invalid refresh token")
	}

	if claims.TokenType != "refresh" {
		return nil, errors.New("invalid token type")
	}

	if _, err := s.store.Exists(ctx, refreshToken); err != nil {
		return nil, errors.New("refresh token not found or revoked")
	}

	user, err := s.users.FindByID(ctx, claims.UserID)
	if err != nil {
		return nil, errors.New("user not found")
	}

	// Revoke old token before issuing new ones (rotation)
	if err := s.store.Revoke(ctx, refreshToken); err != nil {
		return nil, fmt.Errorf("revoke old token: %w", err)
	}

	newAccess, err := s.access.Generate(user.ID, user.Email)
	if err != nil {
		return nil, err
	}

	newRefresh, err := s.refresh.Generate(user.ID, user.Email)
	if err != nil {
		return nil, err
	}

	if err := s.store.Save(ctx, newRefresh, user.ID.String(), s.refreshExpiry); err != nil {
		return nil, fmt.Errorf("store new refresh token: %w", err)
	}

	return &domain.TokenResponse{
		AccessToken:  newAccess,
		RefreshToken: newRefresh,
		TokenType:    "Bearer",
		ExpiresIn:    int64(s.access.Expiry().Seconds()),
		User:         domain.ToUserResponse(user),
	}, nil
}

func (s *authService) Logout(ctx context.Context, refreshToken string) error {
	claims, err := s.refresh.Verify(refreshToken)
	if err != nil {
		return errors.New("invalid refresh token")
	}
	if claims.TokenType != "refresh" {
		return errors.New("invalid token type")
	}
	return s.store.Revoke(ctx, refreshToken)
}

func (s *authService) GetProfile(ctx context.Context, id uuid.UUID) (domain.UserResponse, error) {
	user, err := s.users.FindByID(ctx, id)
	if err != nil {
		return domain.UserResponse{}, errors.New("user not found")
	}
	return domain.ToUserResponse(user), nil
}

func (s *authService) ChangePassword(ctx context.Context, id uuid.UUID, req domain.ChangePasswordRequest) error {
	user, err := s.users.FindByID(ctx, id)
	if err != nil {
		return errors.New("user not found")
	}

	match, err := password.Verify(req.CurrentPassword, user.PasswordHash)
	if err != nil || !match {
		return errors.New("current password is incorrect")
	}

	hash, err := password.Hash(req.NewPassword, password.DefaultParams)
	if err != nil {
		return fmt.Errorf("failed to hash password: %w", err)
	}

	user.PasswordHash = hash
	return s.users.Update(ctx, user)
}
