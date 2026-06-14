package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/ilhamazhar/golang-gpt/internal/domain"
	"github.com/ilhamazhar/golang-gpt/pkg/jwt"
	"github.com/ilhamazhar/golang-gpt/pkg/password"
)

type authService struct {
	users         domain.UserRepository
	store         domain.TokenStore
	access        *jwt.Manager
	refresh       *jwt.Manager
	refreshExpiry time.Duration
}

func NewAuthService(
	users domain.UserRepository,
	store domain.TokenStore,
	access, refresh *jwt.Manager,
	refreshExpiry time.Duration,
) domain.AuthService {
	return &authService{
		users:         users,
		store:         store,
		access:        access,
		refresh:       refresh,
		refreshExpiry: refreshExpiry,
	}
}

func (s *authService) Register(ctx context.Context, req domain.RegisterRequest) (domain.UserResponse, error) {
	hash, err := password.Hash(req.Password, password.DefaultParams)
	if err != nil {
		return domain.UserResponse{}, err
	}

	user := &domain.User{
		Name:         req.Name,
		Email:        req.Email,
		PasswordHash: hash,
	}

	if err := s.users.Create(ctx, user); err != nil {
		return domain.UserResponse{}, errors.New("email already registered")
	}

	return domain.ToUserResponse(user), nil
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
