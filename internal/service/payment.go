package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/ilhamazhar/golang-gpt/internal/domain"
	xenclient "github.com/ilhamazhar/golang-gpt/pkg/xendit"
)

type paymentService struct {
	repo    domain.PaymentRepository
	xendit  *xenclient.Client
	finRepo domain.FinancingRepository
}

func NewPaymentService(repo domain.PaymentRepository, xendit *xenclient.Client, finRepo domain.FinancingRepository) domain.PaymentService {
	return &paymentService{
		repo:    repo,
		xendit:  xendit,
		finRepo: finRepo,
	}
}

func (s *paymentService) CreateQRIS(ctx context.Context, userID uuid.UUID, req domain.CreateQRISRequest) (*domain.QRISResponse, error) {
	payment := &domain.Payment{
		UserID:      userID,
		OrderRef:    fmt.Sprintf("ORDER-%s-%d", userID, time.Now().UnixMilli()),
		Amount:      req.Amount,
		Currency:    "IDR",
		Status:      domain.PaymentStatusPending,
		Description: req.Description,
	}
	return s.createQRIS(ctx, payment)
}

func (s *paymentService) CreateForInstallment(ctx context.Context, userID uuid.UUID, installmentID uint, amount int64, description string) (*domain.QRISResponse, error) {
	payment := &domain.Payment{
		UserID:        userID,
		InstallmentID: &installmentID,
		OrderRef:      fmt.Sprintf("INST-%d-%d", installmentID, time.Now().UnixMilli()),
		Amount:        amount,
		Currency:      "IDR",
		Status:        domain.PaymentStatusPending,
		Description:   description,
	}
	return s.createQRIS(ctx, payment)
}

// createQRIS persists the payment, requests a QRIS from Xendit, and stores the
// returned QR data. Shared by standalone and installment payments.
func (s *paymentService) createQRIS(ctx context.Context, payment *domain.Payment) (*domain.QRISResponse, error) {
	if err := s.repo.Create(ctx, payment); err != nil {
		return nil, fmt.Errorf("failed to create payment record: %w", err)
	}

	qr, err := s.xendit.CreateQRIS(ctx, payment.OrderRef, payment.Amount, payment.Description)
	if err != nil {
		_ = s.repo.UpdateStatus(ctx, payment.ID, domain.PaymentStatusFailed, nil)
		return nil, fmt.Errorf("failed to create QRIS: %w", err)
	}

	if err := s.repo.UpdateQRData(ctx, payment.ID, qr.ID, qr.QRString, &qr.ExpiresAt); err != nil {
		return nil, fmt.Errorf("failed to update payment with QR Data info: %w", err)
	}

	return &domain.QRISResponse{
		OrderRef:    payment.OrderRef,
		QRString:    qr.QRString,
		Amount:      payment.Amount,
		Currency:    "IDR",
		Status:      domain.PaymentStatusPending,
		ExpiresAt:   &qr.ExpiresAt,
		Description: payment.Description,
	}, nil
}

func (s *paymentService) GetStatus(ctx context.Context, userID uuid.UUID, orderRef string, viewAll bool) (*domain.PaymentStatusResponse, error) {
	payment, err := s.repo.FindByOrderRef(ctx, orderRef)
	// Treat "belongs to another user" the same as "not found" so existence of
	// other users' payments is not revealed. Privileged roles (admin/staff)
	// skip the ownership check and may read any payment.
	if err != nil || (!viewAll && payment.UserID != userID) {
		return nil, errors.New("failed to find payment")
	}

	return &domain.PaymentStatusResponse{
		OrderRef:    payment.OrderRef,
		Status:      payment.Status,
		Amount:      payment.Amount,
		PaidAt:      payment.PaidAt,
		ExpiresAt:   payment.ExpiresAt,
		Description: payment.Description,
	}, nil
}

func (s *paymentService) HandleWebhook(ctx context.Context, callbackToken string, body []byte) error {
	if !s.xendit.VerifyCallbackToken(callbackToken) {
		return errors.New("invalid callback token")
	}

	var event struct {
		Event string `json:"event"`
		Data  struct {
			ID          string `json:"id"`
			ReferenceID string `json:"reference_id"`
			Status      string `json:"status"`
		} `json:"data"`
	}

	if err := json.Unmarshal(body, &event); err != nil {
		return fmt.Errorf("failed to parse webhook body: %w", err)
	}

	orderRef := event.Data.ReferenceID
	if orderRef == "" {
		return errors.New("missing order reference in webhook")
	}

	payment, err := s.repo.FindByOrderRef(ctx, orderRef)
	if err != nil {
		return fmt.Errorf("payment not found for ref %s", orderRef)
	}

	switch event.Event {
	case "payment.succeeded":
		now := time.Now()
		if err := s.repo.UpdateStatus(ctx, payment.ID, domain.PaymentStatusPaid, &now); err != nil {
			return err
		}
		// If this payment settles a financing installment, mark it paid and
		// settle the whole financing once nothing remains unpaid.
		if payment.InstallmentID != nil {
			return s.settleInstallment(ctx, *payment.InstallmentID, payment.ID, now)
		}
		return nil

	case "payment.failed":
		return s.repo.UpdateStatus(ctx, payment.ID, domain.PaymentStatusFailed, nil)

	case "payment.expired":
		return s.repo.UpdateStatus(ctx, payment.ID, domain.PaymentStatusExpired, nil)
	}

	return nil

}

func (s *paymentService) settleInstallment(ctx context.Context, installmentID, paymentID uint, paidAt time.Time) error {
	if err := s.finRepo.MarkInstallmentPaid(ctx, installmentID, paymentID, paidAt); err != nil {
		return fmt.Errorf("failed to mark installment paid: %w", err)
	}

	inst, err := s.finRepo.FindInstallmentByID(ctx, installmentID)
	if err != nil {
		return fmt.Errorf("failed to load installment: %w", err)
	}

	remaining, err := s.finRepo.CountUnpaidInstallments(ctx, inst.FinancingID)
	if err != nil {
		return fmt.Errorf("failed to count unpaid installments: %w", err)
	}
	if remaining == 0 {
		return s.finRepo.UpdateStatus(ctx, inst.FinancingID, domain.FinancingStatusSettled, nil)
	}
	return nil
}
