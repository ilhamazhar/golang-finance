package domain

import (
	"context"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type TokenStore interface {
	Save(ctx context.Context, token, userID string, ttl time.Duration) error
	Exists(ctx context.Context, token string) (string, error)
	Revoke(ctx context.Context, token string) error
}

// --- GORM Model ---

type User struct {
	ID           uuid.UUID      `json:"id" gorm:"type:uuid;default:gen_random_uuid();primaryKey"`
	Name         string         `json:"name" gorm:"not null"`
	Email        string         `json:"email" gorm:"not null"`
	PasswordHash string         `json:"-" gorm:"not null"`
	CreatedAt    time.Time      `json:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at"`
	DeletedAt    gorm.DeletedAt `json:"-" gorm:"index"`
}

// --- Request DTOs ---

type RegisterRequest struct {
	Name            string `json:"name" validate:"required,max=255"`
	Email           string `json:"email" validate:"required,email"`
	Password        string `json:"password" validate:"required,min=6"`
	PasswordConfirm string `json:"password_confirm" validate:"required,eqfield=Password"`
}

type LoginRequest struct {
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required"`
}

type RefreshRequest struct {
	RefreshToken string `json:"refresh_token" validate:"required"`
}

type LogoutRequest struct {
	RefreshToken string `json:"refresh_token" validate:"required"`
}

type UpdateUserRequest struct {
	Name  string `json:"name" validate:"omitempty,max=255"`
	Email string `json:"email" validate:"omitempty,email"`
}

type ChangePasswordRequest struct {
	CurrentPassword string `json:"current_password" validate:"required"`
	NewPassword     string `json:"new_password" validate:"required,min=6"`
	ConfirmPassword string `json:"confirm_password" validate:"required,eqfield=NewPassword"`
}

// --- Response DTOs (never expose PasswordHash) ---

type UserResponse struct {
	ID        uuid.UUID `json:"id"`
	Name      string    `json:"name"`
	Email     string    `json:"email"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type TokenResponse struct {
	AccessToken  string       `json:"access_token"`
	RefreshToken string       `json:"refresh_token"`
	TokenType    string       `json:"token_type"`
	ExpiresIn    int64        `json:"expires_in"` // seconds until access token expires
	User         UserResponse `json:"user"`
}

func ToUserResponse(u *User) UserResponse {
	return UserResponse{
		ID:        u.ID,
		Name:      u.Name,
		Email:     u.Email,
		CreatedAt: u.CreatedAt,
		UpdatedAt: u.UpdatedAt,
	}
}

// --- Repository & Service interfaces

type UserRepository interface {
	Create(ctx context.Context, user *User) error
	FindByEmail(ctx context.Context, email string) (*User, error)
	FindByID(ctx context.Context, id uuid.UUID) (*User, error)
	FindAll(ctx context.Context, page, limit int) ([]User, int64, error)
	Update(ctx context.Context, user *User) error
	Delete(ctx context.Context, id uuid.UUID) error
}

type UserService interface {
	FindAll(ctx context.Context, page, limit int) ([]UserResponse, int64, error)
	FindByID(ctx context.Context, id uuid.UUID) (UserResponse, error)
	Update(ctx context.Context, id uuid.UUID, req UpdateUserRequest) (UserResponse, error)
	Delete(ctx context.Context, id uuid.UUID) error
}

type AuthService interface {
	Register(ctx context.Context, req RegisterRequest) (UserResponse, error)
	Login(ctx context.Context, req LoginRequest) (*TokenResponse, error)
	RefreshToken(ctx context.Context, refreshToken string) (*TokenResponse, error)
	Logout(ctx context.Context, refreshToken string) error
	GetProfile(ctx context.Context, id uuid.UUID) (UserResponse, error)
	ChangePassword(ctx context.Context, id uuid.UUID, req ChangePasswordRequest) error
}
