package domain

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// AkadType is the Islamic contract (akad) underlying a financing. Murabahah
// (cost-plus sale) is the first supported type; others are reserved for later.
type AkadType string

const (
	AkadMurabahah AkadType = "MURABAHAH"
)

// FinancingStatus tracks the lifecycle of a financing contract.
type FinancingStatus string

const (
	FinancingStatusDraft      FinancingStatus = "DRAFT"   // created, akad not yet signed
	FinancingStatusActive     FinancingStatus = "ACTIVE"  // akad signed, disbursed, installments running
	FinancingStatusSettled    FinancingStatus = "SETTLED" // all installments paid
	FinancingStatusWrittenOff FinancingStatus = "WRITTEN_OFF"
)

// InstallmentStatus tracks a single installment (angsuran).
type InstallmentStatus string

const (
	InstallmentStatusUnpaid InstallmentStatus = "UNPAID"
	InstallmentStatusPaid   InstallmentStatus = "PAID"
	InstallmentStatusLate   InstallmentStatus = "LATE"
)

// Financing is the akad header for a Murabahah cost-plus sale.
//
// Syariah invariant: TotalPrice = CostPrice + MarginAmount, and MarginAmount is
// fixed once AkadSignedAt is set. It is NEVER recalculated as a function of time
// (that would be riba). All money fields are integer minor units (e.g. rupiah).
type Financing struct {
	ID     uint      `json:"id" gorm:"primaryKey"`
	UserID uuid.UUID `json:"user_id" gorm:"not null;index"`
	User   User      `json:"-" gorm:"foreignKey:UserID;constraint:OnDelete:CASCADE"`

	AkadType  AkadType `json:"akad_type" gorm:"not null;default:'MURABAHAH'"`
	AssetName string   `json:"asset_name" gorm:"not null"` // object of sale (Murabahah requires a real asset)

	CostPrice    int64  `json:"cost_price" gorm:"not null"`             // harga pokok — what the financier paid
	MarginAmount int64  `json:"margin_amount" gorm:"not null"`          // keuntungan — fixed profit
	TotalPrice   int64  `json:"total_price" gorm:"not null"`            // CostPrice + MarginAmount, locked at akad
	DownPayment  int64  `json:"down_payment" gorm:"not null;default:0"` // uang muka, reduces principal
	Tenor        int    `json:"tenor" gorm:"not null"`                  // number of monthly installments
	Currency     string `json:"currency" gorm:"default:'IDR'"`

	Status       FinancingStatus `json:"status" gorm:"not null;default:'DRAFT'"`
	AkadSignedAt *time.Time      `json:"akad_signed_at,omitempty"`

	Installments []Installment `json:"installments,omitempty" gorm:"foreignKey:FinancingID;constraint:OnDelete:CASCADE"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Installment is one row of the repayment schedule (jadwal angsuran). The full
// schedule is generated upfront and its total is immutable.
type Installment struct {
	ID            uint              `json:"id" gorm:"primaryKey"`
	FinancingID   uint              `json:"financing_id" gorm:"not null;index"`
	InstallmentNo int               `json:"installment_no" gorm:"not null"` // 1-based
	DueDate       time.Time         `json:"due_date" gorm:"not null"`
	PrincipalPart int64             `json:"principal_part" gorm:"not null"` // pokok portion
	MarginPart    int64             `json:"margin_part" gorm:"not null"`    // margin portion
	Amount        int64             `json:"amount" gorm:"not null"`         // PrincipalPart + MarginPart
	Status        InstallmentStatus `json:"status" gorm:"not null;default:'UNPAID'"`
	PaidAt        *time.Time        `json:"paid_at,omitempty"`
	PaymentID     *uint             `json:"payment_id,omitempty"` // links to the Payment/Xendit row that settled it

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// --- Request DTOs ---

type CreateMurabahahRequest struct {
	AssetName    string `json:"asset_name" validate:"required,max=255"`
	CostPrice    int64  `json:"cost_price" validate:"required,gt=0"`
	MarginAmount int64  `json:"margin_amount" validate:"gte=0"`
	DownPayment  int64  `json:"down_payment" validate:"gte=0"`
	Tenor        int    `json:"tenor" validate:"required,gt=0,lte=360"`
	// FirstDueDate is optional; when zero the service defaults it (e.g. one month out).
	FirstDueDate *time.Time `json:"first_due_date,omitempty"`
}

// --- Response DTOs ---

type InstallmentResponse struct {
	InstallmentNo int               `json:"installment_no"`
	DueDate       time.Time         `json:"due_date"`
	PrincipalPart int64             `json:"principal_part"`
	MarginPart    int64             `json:"margin_part"`
	Amount        int64             `json:"amount"`
	Status        InstallmentStatus `json:"status"`
	PaidAt        *time.Time        `json:"paid_at,omitempty"`
}

type FinancingResponse struct {
	ID           uint                  `json:"id"`
	AkadType     AkadType              `json:"akad_type"`
	AssetName    string                `json:"asset_name"`
	CostPrice    int64                 `json:"cost_price"`
	MarginAmount int64                 `json:"margin_amount"`
	TotalPrice   int64                 `json:"total_price"`
	DownPayment  int64                 `json:"down_payment"`
	Tenor        int                   `json:"tenor"`
	Currency     string                `json:"currency"`
	Status       FinancingStatus       `json:"status"`
	AkadSignedAt *time.Time            `json:"akad_signed_at,omitempty"`
	Installments []InstallmentResponse `json:"installments,omitempty"`
	CreatedAt    time.Time             `json:"created_at"`
}

func ToInstallmentResponse(i Installment) InstallmentResponse {
	return InstallmentResponse{
		InstallmentNo: i.InstallmentNo,
		DueDate:       i.DueDate,
		PrincipalPart: i.PrincipalPart,
		MarginPart:    i.MarginPart,
		Amount:        i.Amount,
		Status:        i.Status,
		PaidAt:        i.PaidAt,
	}
}

func ToFinancingResponse(f *Financing) FinancingResponse {
	resp := FinancingResponse{
		ID:           f.ID,
		AkadType:     f.AkadType,
		AssetName:    f.AssetName,
		CostPrice:    f.CostPrice,
		MarginAmount: f.MarginAmount,
		TotalPrice:   f.TotalPrice,
		DownPayment:  f.DownPayment,
		Tenor:        f.Tenor,
		Currency:     f.Currency,
		Status:       f.Status,
		AkadSignedAt: f.AkadSignedAt,
		CreatedAt:    f.CreatedAt,
	}
	for _, inst := range f.Installments {
		resp.Installments = append(resp.Installments, ToInstallmentResponse(inst))
	}
	return resp
}

// --- Repository & Service interfaces ---

type FinancingRepository interface {
	Create(ctx context.Context, f *Financing) error
	FindByID(ctx context.Context, id uint) (*Financing, error)
	FindByUser(ctx context.Context, userID uuid.UUID, page, limit int) ([]Financing, int64, error)
	UpdateStatus(ctx context.Context, id uint, status FinancingStatus, signedAt *time.Time) error

	// Installment access, used by the akad-signing and payment-settlement flows.
	FindInstallment(ctx context.Context, financingID uint, no int) (*Installment, error)
	FindInstallmentByID(ctx context.Context, id uint) (*Installment, error)
	MarkInstallmentPaid(ctx context.Context, installmentID, paymentID uint, paidAt time.Time) error
	CountUnpaidInstallments(ctx context.Context, financingID uint) (int64, error)
}

type FinancingService interface {
	CreateMurabahah(ctx context.Context, userID uuid.UUID, req CreateMurabahahRequest) (*FinancingResponse, error)
	GetByID(ctx context.Context, userID uuid.UUID, id uint) (*FinancingResponse, error)
	ListByUser(ctx context.Context, userID uuid.UUID, page, limit int) ([]FinancingResponse, int64, error)
	// SignAkad transitions a DRAFT financing to ACTIVE, stamping AkadSignedAt.
	SignAkad(ctx context.Context, userID uuid.UUID, id uint) (*FinancingResponse, error)
	// PayInstallment creates a QRIS payment for one installment of an ACTIVE financing.
	PayInstallment(ctx context.Context, userID uuid.UUID, financingID uint, installmentNo int) (*QRISResponse, error)
}
