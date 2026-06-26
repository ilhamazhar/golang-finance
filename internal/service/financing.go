package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/ilhamazhar/golang-gpt/internal/domain"
)

type financingService struct {
	repo     domain.FinancingRepository
	payments domain.PaymentService
}

func NewFinancingService(repo domain.FinancingRepository, payments domain.PaymentService) domain.FinancingService {
	return &financingService{repo: repo, payments: payments}
}

// CreateMurabahah locks the total price (cost + margin) at creation, generates
// the immutable installment schedule, and persists both atomically as a DRAFT.
// The akad is not yet signed — that is a separate step before disbursement.
func (s *financingService) CreateMurabahah(ctx context.Context, userID uuid.UUID, req domain.CreateMurabahahRequest) (*domain.FinancingResponse, error) {
	totalPrice := req.CostPrice + req.MarginAmount

	// Default the first due date to one month out when the caller omits it.
	firstDue := time.Now().AddDate(0, 1, 0)
	if req.FirstDueDate != nil {
		firstDue = *req.FirstDueDate
	}

	schedule, err := GenerateMurabahahSchedule(ScheduleParams{
		CostPrice:    req.CostPrice,
		MarginAmount: req.MarginAmount,
		DownPayment:  req.DownPayment,
		Tenor:        req.Tenor,
		FirstDueDate: firstDue,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to generate schedule: %w", err)
	}

	financing := &domain.Financing{
		UserID:       userID,
		AkadType:     domain.AkadMurabahah,
		AssetName:    req.AssetName,
		CostPrice:    req.CostPrice,
		MarginAmount: req.MarginAmount,
		TotalPrice:   totalPrice,
		DownPayment:  req.DownPayment,
		Tenor:        req.Tenor,
		Currency:     "IDR",
		Status:       domain.FinancingStatusDraft,
		Installments: schedule,
	}
	if err := s.repo.Create(ctx, financing); err != nil {
		return nil, fmt.Errorf("failed to create financing: %w", err)
	}

	// Reload so the response carries the owner's name (FindByID preloads User),
	// consistent with the get/list/sign responses.
	created, err := s.repo.FindByID(ctx, financing.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to reload financing: %w", err)
	}
	resp := domain.ToFinancingResponse(created)
	return &resp, nil
}

func (s *financingService) GetByID(ctx context.Context, userID uuid.UUID, id uint, viewAll bool) (*domain.FinancingResponse, error) {
	f, err := s.repo.FindByID(ctx, id)
	// Treat "belongs to another user" the same as "not found" so existence
	// of other users' financings is not revealed. Privileged roles (admin/staff)
	// skip the ownership check and may read any financing.
	if err != nil || (!viewAll && f.UserID != userID) {
		return nil, errors.New("financing not found")
	}
	resp := domain.ToFinancingResponse(f)
	return &resp, nil
}

func (s *financingService) List(ctx context.Context, userID uuid.UUID, page, limit int, search, sort, order string, viewAll bool) ([]domain.FinancingResponse, int64, error) {
	// Privileged roles (admin/staff) list every financing; regular users see only their own.
	var (
		list  []domain.Financing
		total int64
		err   error
	)
	if viewAll {
		list, total, err = s.repo.FindAll(ctx, page, limit, search, sort, order)
	} else {
		list, total, err = s.repo.FindByUser(ctx, userID, page, limit, search, sort, order)
	}
	if err != nil {
		return nil, 0, fmt.Errorf("failed to fetch financings: %w", err)
	}
	result := make([]domain.FinancingResponse, len(list))
	for i := range list {
		result[i] = domain.ToFinancingResponse(&list[i])
	}
	return result, total, nil
}

// SignAkad signs a DRAFT financing, transitioning it to ACTIVE so installments
// can be paid. Signing is the point at which the obligation becomes binding.
func (s *financingService) SignAkad(ctx context.Context, userID uuid.UUID, id uint) (*domain.FinancingResponse, error) {
	f, err := s.repo.FindByID(ctx, id)
	if err != nil || f.UserID != userID {
		return nil, domain.ErrNotFound
	}
	if f.Status != domain.FinancingStatusDraft {
		return nil, domain.ErrFinancingNotDraft
	}

	now := time.Now()
	if err := s.repo.UpdateStatus(ctx, id, domain.FinancingStatusActive, &now); err != nil {
		return nil, fmt.Errorf("failed to sign akad: %w", err)
	}

	f, err = s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to reload financing: %w", err)
	}
	resp := domain.ToFinancingResponse(f)
	return &resp, nil
}

// PayInstallment creates a QRIS payment for a single installment of an ACTIVE
// financing. The installment is settled later, when Xendit calls the webhook.
func (s *financingService) PayInstallment(ctx context.Context, userID uuid.UUID, financingID uint, installmentNo int) (*domain.QRISResponse, error) {
	f, err := s.repo.FindByID(ctx, financingID)
	if err != nil || f.UserID != userID {
		return nil, domain.ErrNotFound
	}
	if f.Status != domain.FinancingStatusActive {
		return nil, domain.ErrFinancingNotActive
	}

	inst, err := s.repo.FindInstallment(ctx, financingID, installmentNo)
	if err != nil {
		return nil, domain.ErrNotFound
	}
	if inst.Status == domain.InstallmentStatusPaid {
		return nil, domain.ErrInstallmentPaid
	}

	desc := fmt.Sprintf("Angsuran ke-%d - %s", installmentNo, f.AssetName)
	return s.payments.CreateForInstallment(ctx, userID, inst.ID, inst.Amount, desc)
}
