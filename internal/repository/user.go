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

// userSortColumns whitelists the columns clients may sort by, mapping the API
// sort key to the actual DB column. Sorting is interpolated into the SQL (GORM
// can't parameterize identifiers), so this allowlist is what prevents injection
// — never build the ORDER BY from raw input.
var userSortColumns = map[string]string{
	"name":       "name",
	"email":      "email",
	"role":       "role",
	"created_at": "created_at",
	"status":     "email_verified_at",
}

func (r *userRepo) FindAll(ctx context.Context, page, limit int, search, sort, order string) ([]domain.User, int64, error) {
	var users []domain.User
	var total int64

	// Shared base query so the count and the page apply the same search filter.
	base := r.db.WithContext(ctx).Model(&domain.User{})
	if search != "" {
		like := "%" + search + "%"
		base = base.Where("name ILIKE ? OR email ILIKE ?", like, like)
	}

	if err := base.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// Resolve sort against the allowlist; fall back to newest-first.
	column, ok := userSortColumns[sort]
	if !ok {
		column = "created_at"
	}
	if order != "asc" && order != "desc" {
		order = "desc"
	}

	offset := (page - 1) * limit
	err := base.Order(column + " " + order).Offset(offset).Limit(limit).Find(&users).Error
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
