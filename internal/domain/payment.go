package domain

import (
	"context"
	"time"

	"github.com/google/uuid"
)

type PaymentStatus string

const (
	PaymentStatusPending PaymentStatus = "PENDING"
	PaymentStatusPaid    PaymentStatus = "PAID"
	PaymentStatusExpired PaymentStatus = "EXPIRED"
	PaymentStatusFailed  PaymentStatus = "FAILED"
)

type Payment struct {
	ID     uint      `json:"id" gorm:"primaryKey" `
	UserID uuid.UUID `json:"user_id" gorm:"not null;index"`
	User   User      `json:"-" gorm:"foreignKey:UserID;constraint:OnDelete:CASCADE"`
	// InstallmentID links this payment to a financing installment. Nil for
	// standalone QRIS payments; set when paying a Murabahah installment so the
	// webhook can settle the right installment.
	InstallmentID *uint         `json:"installment_id,omitempty" gorm:"index"`
	OrderRef      string        `json:"order_ref" gorm:"not null;uniqueIndex"`
	XenditID      string        `json:"xendit_id,omitempty" gorm:"index"`
	QRString      string        `json:"qr_string,omitempty" gorm:"type:text"`
	Amount        int64         `json:"amount" gorm:"not null"`
	Currency      string        `json:"currency" gorm:"default:'IDR'"`
	Status        PaymentStatus `json:"status" gorm:"default:'PENDING'"`
	ExpiresAt     *time.Time    `json:"expires_at"`
	PaidAt        *time.Time    `json:"paid_at"`
	Description   string        `json:"description"`
	CreatedAt     time.Time     `json:"created_at"`
	UpdatedAt     time.Time     `json:"updated_at"`
}

// --- Request DTOs ---

type CreateQRISRequest struct {
	Amount      int64  `json:"amount" validate:"required,gt=0"`
	Description string `json:"description" validate:"required,max=255"`
}

type QRISResponse struct {
	OrderRef    string        `json:"order_ref"`
	QRString    string        `json:"qr_string"`
	Amount      int64         `json:"amount"`
	Currency    string        `json:"currency"`
	Status      PaymentStatus `json:"status"`
	ExpiresAt   *time.Time    `json:"expires_at"`
	Description string        `json:"description"`
}

type PaymentStatusResponse struct {
	OrderRef    string        `json:"order_ref"`
	Amount      int64         `json:"amount"`
	Status      PaymentStatus `json:"status"`
	PaidAt      *time.Time    `json:"paid_at,omitempty"`
	ExpiresAt   *time.Time    `json:"expires_at,omitempty"`
	Description string        `json:"description,omitempty"`
}

type PaymentRepository interface {
	Create(ctx context.Context, p *Payment) error
	FindByOrderRef(ctx context.Context, orderRef string) (*Payment, error)
	UpdateQRData(ctx context.Context, id uint, xenditID, qrString string, expiresAt *time.Time) error
	UpdateStatus(ctx context.Context, id uint, status PaymentStatus, paidAt *time.Time) error
}

type PaymentService interface {
	CreateQRIS(ctx context.Context, userID uuid.UUID, req CreateQRISRequest) (*QRISResponse, error)
	// CreateForInstallment creates a QRIS payment bound to a financing installment.
	CreateForInstallment(ctx context.Context, userID uuid.UUID, installmentID uint, amount int64, description string) (*QRISResponse, error)
	GetStatus(ctx context.Context, userID uuid.UUID, orderRef string) (*PaymentStatusResponse, error)
	HandleWebhook(ctx context.Context, callbackToken string, body []byte) error
}
