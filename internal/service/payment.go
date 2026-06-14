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
	repo   domain.PaymentRepository
	xendit *xenclient.Client
}

func NewPaymentService(repo domain.PaymentRepository, xendit *xenclient.Client) domain.PaymentService {
	return &paymentService{
		repo:   repo,
		xendit: xendit,
	}
}

func (s *paymentService) CreateQRIS(ctx context.Context, userID uuid.UUID, req domain.CreateQRISRequest) (*domain.QRISResponse, error) {
	orderRef := fmt.Sprintf("ORDER-%s-%d", userID, time.Now().UnixMilli())

	payment := &domain.Payment{
		UserID:      userID,
		OrderRef:    orderRef,
		Amount:      req.Amount,
		Currency:    "IDR",
		Status:      domain.PaymentStatusPending,
		Description: req.Description,
	}
	if err := s.repo.Create(ctx, payment); err != nil {
		return nil, fmt.Errorf("failed to create payment record: %w", err)
	}

	qr, err := s.xendit.CreateQRIS(ctx, orderRef, req.Amount, req.Description)
	if err != nil {
		_ = s.repo.UpdateStatus(ctx, payment.ID, domain.PaymentStatusFailed, nil)
		return nil, fmt.Errorf("failed to create QRIS: %w", err)
	}

	if err := s.repo.UpdateQRData(ctx, payment.ID, qr.ID, qr.QRString, &qr.ExpiresAt); err != nil {
		return nil, fmt.Errorf("failed to update payment with QR Data info: %w", err)
	}

	return &domain.QRISResponse{
		OrderRef:    orderRef,
		QRString:    qr.QRString,
		Amount:      req.Amount,
		Currency:    "IDR",
		Status:      domain.PaymentStatusPending,
		ExpiresAt:   &qr.ExpiresAt,
		Description: req.Description,
	}, nil
}

func (s *paymentService) GetStatus(ctx context.Context, orderRef string) (*domain.PaymentStatusResponse, error) {
	payment, err := s.repo.FindByOrderRef(ctx, orderRef)
	if err != nil {
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
		return s.repo.UpdateStatus(ctx, payment.ID, domain.PaymentStatusPaid, &now)

	case "payment.failed":
		return s.repo.UpdateStatus(ctx, payment.ID, domain.PaymentStatusFailed, nil)

	case "payment.expired":
		return s.repo.UpdateStatus(ctx, payment.ID, domain.PaymentStatusExpired, nil)
	}

	return nil

}
