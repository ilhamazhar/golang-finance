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
	FinancingStatusApplied    FinancingStatus = "APPLIED"  // customer applied; margin & schedule not yet set
	FinancingStatusApproved   FinancingStatus = "APPROVED" // staff/admin set the terms; awaiting the owner's akad signature
	FinancingStatusActive     FinancingStatus = "ACTIVE"   // akad signed, disbursed, installments running
	FinancingStatusSettled    FinancingStatus = "SETTLED"  // all installments paid
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
	MarginAmount int64  `json:"margin_amount" gorm:"not null"`          // keuntungan — set by staff/admin at approval, then fixed
	TotalPrice   int64  `json:"total_price" gorm:"not null"`            // CostPrice + MarginAmount, locked at akad
	DownPayment  int64  `json:"down_payment" gorm:"not null;default:0"` // uang muka, reduces principal
	Tenor        int    `json:"tenor" gorm:"not null"`                  // number of monthly installments
	Currency     string `json:"currency" gorm:"default:'IDR'"`

	Status       FinancingStatus `json:"status" gorm:"not null;default:'APPLIED'"`
	AkadSignedAt *time.Time      `json:"akad_signed_at,omitempty"`
	// FirstDueDate is the requested due date of installment #1, captured at
	// application so it can prefill (and default) the schedule generated at approval.
	FirstDueDate *time.Time `json:"first_due_date,omitempty"`
	// ApprovedBy / ApprovedAt record who underwrote the application (always a
	// staff or admin, since only they may approve) and when. Nil while APPLIED.
	ApprovedBy *uuid.UUID `json:"approved_by,omitempty" gorm:"type:uuid;index"`
	Approver   *User      `json:"-" gorm:"foreignKey:ApprovedBy;constraint:OnDelete:SET NULL"`
	ApprovedAt *time.Time `json:"approved_at,omitempty"`

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

// CreateMurabahahRequest is the customer's application. It deliberately omits
// MarginAmount: the institution's profit is set by staff/admin at approval, not
// by the applicant (a customer setting their own margin would be meaningless).
type CreateMurabahahRequest struct {
	AssetName   string `json:"asset_name" validate:"required,max=255"`
	CostPrice   int64  `json:"cost_price" validate:"required,gt=0"`
	DownPayment int64  `json:"down_payment" validate:"gte=0"`
	Tenor       int    `json:"tenor" validate:"required,gt=0,lte=360"`
	// FirstDueDate is optional; when zero the service defaults it (e.g. one month out).
	FirstDueDate *time.Time `json:"first_due_date,omitempty"`
}

// ApproveFinancingRequest carries the underwritten terms set by staff/admin when
// approving an APPLIED financing. The approver confirms (and may correct) the
// financial figures and sets the margin; the schedule is generated from these.
type ApproveFinancingRequest struct {
	CostPrice    int64 `json:"cost_price" validate:"required,gt=0"`
	MarginAmount int64 `json:"margin_amount" validate:"gte=0"`
	DownPayment  int64 `json:"down_payment" validate:"gte=0"`
	Tenor        int   `json:"tenor" validate:"required,gt=0,lte=360"`
	// FirstDueDate is optional; falls back to the applied-for date, then to one month out.
	FirstDueDate *time.Time `json:"first_due_date,omitempty"`
}

// ApprovedTerms is the persisted result of approval: the locked financial
// figures the repository writes alongside the generated schedule, plus the
// approver attribution (who approved, and when).
type ApprovedTerms struct {
	CostPrice    int64
	MarginAmount int64
	TotalPrice   int64
	DownPayment  int64
	Tenor        int
	FirstDueDate *time.Time
	ApprovedBy   uuid.UUID
	ApprovedAt   time.Time
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
	UserID       uuid.UUID             `json:"user_id"`
	UserName     string                `json:"user_name,omitempty"` // populated when the owner is loaded (e.g. list endpoints)
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
	FirstDueDate *time.Time            `json:"first_due_date,omitempty"`
	ApprovedBy   *uuid.UUID            `json:"approved_by,omitempty"`
	ApproverName string                `json:"approver_name,omitempty"` // populated when the Approver association is loaded (detail endpoint)
	ApprovedAt   *time.Time            `json:"approved_at,omitempty"`
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
		UserID:       f.UserID,
		UserName:     f.User.Name, // empty unless the User association was loaded
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
		FirstDueDate: f.FirstDueDate,
		ApprovedBy:   f.ApprovedBy,
		ApprovedAt:   f.ApprovedAt,
		CreatedAt:    f.CreatedAt,
	}
	if f.Approver != nil {
		resp.ApproverName = f.Approver.Name // empty unless the Approver association was loaded
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
	FindByUser(ctx context.Context, userID uuid.UUID, page, limit int, search, sort, order string) ([]Financing, int64, error)
	FindAll(ctx context.Context, page, limit int, search, sort, order string) ([]Financing, int64, error)
	UpdateStatus(ctx context.Context, id uint, status FinancingStatus, signedAt *time.Time) error
	// Approve locks the underwritten terms onto an APPLIED financing, attaches the
	// generated schedule, and transitions it to APPROVED — atomically.
	Approve(ctx context.Context, id uint, terms ApprovedTerms, schedule []Installment) error

	// Installment access, used by the akad-signing and payment-settlement flows.
	FindInstallment(ctx context.Context, financingID uint, no int) (*Installment, error)
	FindInstallmentByID(ctx context.Context, id uint) (*Installment, error)
	MarkInstallmentPaid(ctx context.Context, installmentID, paymentID uint, paidAt time.Time) error
	CountUnpaidInstallments(ctx context.Context, financingID uint) (int64, error)
}

type FinancingService interface {
	CreateMurabahah(ctx context.Context, userID uuid.UUID, req CreateMurabahahRequest) (*FinancingResponse, error)
	// GetByID returns a financing. When viewAll is true the ownership check is
	// skipped, so privileged roles (admin/staff) may read any user's financing.
	GetByID(ctx context.Context, userID uuid.UUID, id uint, viewAll bool) (*FinancingResponse, error)
	// List returns the caller's financings, or every financing when viewAll is true.
	List(ctx context.Context, userID uuid.UUID, page, limit int, search, sort, order string, viewAll bool) ([]FinancingResponse, int64, error)
	// Approve sets the underwritten terms (margin and confirmed figures) on an
	// APPLIED financing, generates its schedule, records approverID as the
	// approver, and transitions it to APPROVED. This is a back-office action
	// (staff/admin) and is not scoped by ownership.
	Approve(ctx context.Context, approverID uuid.UUID, id uint, req ApproveFinancingRequest) (*FinancingResponse, error)
	// SignAkad transitions an APPROVED financing to ACTIVE, stamping AkadSignedAt.
	SignAkad(ctx context.Context, userID uuid.UUID, id uint) (*FinancingResponse, error)
	// PayInstallment creates a QRIS payment for one installment of an ACTIVE financing.
	PayInstallment(ctx context.Context, userID uuid.UUID, financingID uint, installmentNo int) (*QRISResponse, error)
}
