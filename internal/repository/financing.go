package repository

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/ilhamazhar/golang-gpt/internal/domain"
	"gorm.io/gorm"
)

type financingRepo struct {
	db *gorm.DB
}

func NewFinancingRepository(db *gorm.DB) domain.FinancingRepository {
	return &financingRepo{db: db}
}

// Create inserts the financing together with its installment schedule. GORM
// persists the has-many association in a single transaction, so the schedule is
// never written without its parent.
func (r *financingRepo) Create(ctx context.Context, f *domain.Financing) error {
	return r.db.WithContext(ctx).Create(f).Error
}

func (r *financingRepo) FindByID(ctx context.Context, id uint) (*domain.Financing, error) {
	var f domain.Financing
	err := r.db.WithContext(ctx).
		Preload("Installments", func(db *gorm.DB) *gorm.DB {
			return db.Order("installment_no ASC")
		}).
		First(&f, id).Error
	return &f, err
}

func (r *financingRepo) FindByUser(ctx context.Context, userID uuid.UUID, page, limit int) ([]domain.Financing, int64, error) {
	var list []domain.Financing
	var total int64

	offset := (page - 1) * limit
	if err := r.db.WithContext(ctx).Model(&domain.Financing{}).Where("user_id = ?", userID).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	// Listing is intentionally light: installments are loaded only on FindByID.
	err := r.db.WithContext(ctx).
		Where("user_id = ?", userID).
		Order("created_at DESC").
		Offset(offset).Limit(limit).
		Find(&list).Error
	return list, total, err
}

func (r *financingRepo) UpdateStatus(ctx context.Context, id uint, status domain.FinancingStatus, signedAt *time.Time) error {
	updates := map[string]any{"status": status}
	if signedAt != nil {
		updates["akad_signed_at"] = *signedAt
	}
	result := r.db.WithContext(ctx).Model(&domain.Financing{}).Where("id = ?", id).Updates(updates)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (r *financingRepo) FindInstallment(ctx context.Context, financingID uint, no int) (*domain.Installment, error) {
	var inst domain.Installment
	err := r.db.WithContext(ctx).
		Where("financing_id = ? AND installment_no = ?", financingID, no).
		First(&inst).Error
	return &inst, err
}

func (r *financingRepo) FindInstallmentByID(ctx context.Context, id uint) (*domain.Installment, error) {
	var inst domain.Installment
	err := r.db.WithContext(ctx).First(&inst, id).Error
	return &inst, err
}

func (r *financingRepo) MarkInstallmentPaid(ctx context.Context, installmentID, paymentID uint, paidAt time.Time) error {
	updates := map[string]any{
		"status":     domain.InstallmentStatusPaid,
		"paid_at":    paidAt,
		"payment_id": paymentID,
	}
	return r.db.WithContext(ctx).Model(&domain.Installment{}).Where("id = ?", installmentID).Updates(updates).Error
}

func (r *financingRepo) CountUnpaidInstallments(ctx context.Context, financingID uint) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&domain.Installment{}).
		Where("financing_id = ? AND status <> ?", financingID, domain.InstallmentStatusPaid).
		Count(&count).Error
	return count, err
}
