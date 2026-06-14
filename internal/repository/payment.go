package repository

import (
	"context"
	"time"

	"github.com/ilhamazhar/golang-gpt/internal/domain"
	"gorm.io/gorm"
)

type paymentRepo struct {
	db *gorm.DB
}

func NewPaymentRepository(db *gorm.DB) domain.PaymentRepository {
	return &paymentRepo{db: db}
}

func (r *paymentRepo) Create(ctx context.Context, payment *domain.Payment) error {
	return r.db.WithContext(ctx).Create(payment).Error
}

func (r *paymentRepo) FindByOrderRef(ctx context.Context, orderRef string) (*domain.Payment, error) {
	var payment domain.Payment
	err := r.db.WithContext(ctx).Where("order_ref = ?", orderRef).First(&payment).Error
	return &payment, err
}

func (r *paymentRepo) UpdateQRData(ctx context.Context, id uint, xenditID, qrString string, expiresAt *time.Time) error {
	updates := map[string]any{
		"xendit_id":  xenditID,
		"qr_string":  qrString,
		"expires_at": expiresAt,
	}
	return r.db.WithContext(ctx).Model(&domain.Payment{}).Where("id = ?", id).Updates(updates).Error
}

func (r *paymentRepo) UpdateStatus(ctx context.Context, id uint, status domain.PaymentStatus, paidAt *time.Time) error {
	updates := map[string]any{"status": status}
	if paidAt != nil {
		updates["paid_at"] = *paidAt
	}
	return r.db.WithContext(ctx).Model(&domain.Payment{}).Where("id = ?", id).Updates(updates).Error
}
