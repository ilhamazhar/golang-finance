package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/ilhamazhar/golang-gpt/internal/domain"
	"gorm.io/gorm"
)

type userRepo struct {
	db *gorm.DB
}

func NewUserRepository(db *gorm.DB) domain.UserRepository {
	return &userRepo{db: db}
}

func (r *userRepo) Create(ctx context.Context, user *domain.User) error {
	return r.db.WithContext(ctx).Create(user).Error
}

func (r *userRepo) FindAll(ctx context.Context, page, limit int) ([]domain.User, int64, error) {
	var users []domain.User
	var total int64

	offset := (page - 1) * limit
	if err := r.db.WithContext(ctx).Model(&domain.User{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	err := r.db.WithContext(ctx).Order("created_at DESC").Offset(offset).Limit(limit).Find(&users).Error
	return users, total, err
}

func (r *userRepo) FindByEmail(ctx context.Context, email string) (*domain.User, error) {
	var user domain.User
	err := r.db.WithContext(ctx).Where("email = ?", email).First(&user).Error
	return &user, err
}

func (r *userRepo) FindByID(ctx context.Context, id uuid.UUID) (*domain.User, error) {
	var user domain.User
	err := r.db.WithContext(ctx).First(&user, id).Error
	return &user, err
}

func (r *userRepo) Update(ctx context.Context, user *domain.User) error {
	return r.db.WithContext(ctx).Save(user).Error
}

func (r *userRepo) Delete(ctx context.Context, id uuid.UUID) error {
	// Soft delete: GORM sets deleted_at instead of removing the row,
	// so payment records keep a valid user reference.
	result := r.db.WithContext(ctx).Delete(&domain.User{}, id)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return domain.ErrNotFound
	}
	return nil
}
