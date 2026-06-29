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

// CreateMurabahah records a customer's financing application as APPLIED. The
// margin and the installment schedule are NOT set here — they are determined by
// staff/admin at approval, since the institution (not the applicant) sets its
// profit. TotalPrice provisionally equals CostPrice until a margin is approved.
func (s *financingService) CreateMurabahah(ctx context.Context, userID uuid.UUID, req domain.CreateMurabahahRequest) (*domain.FinancingResponse, error) {
	// A down payment can never exceed the cost price (it is applied against the
	// principal). Validate up front; the full schedule is checked at approval.
	if req.DownPayment > req.CostPrice {
		return nil, errors.New("down payment cannot exceed the cost price")
	}

	financing := &domain.Financing{
		UserID:       userID,
		AkadType:     domain.AkadMurabahah,
		AssetName:    req.AssetName,
		CostPrice:    req.CostPrice,
		MarginAmount: 0,
		TotalPrice:   req.CostPrice,
		DownPayment:  req.DownPayment,
		Tenor:        req.Tenor,
		Currency:     "IDR",
		Status:       domain.FinancingStatusApplied,
		FirstDueDate: req.FirstDueDate,
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

// Approve sets the underwritten terms on an APPLIED financing, generates its
// immutable schedule, stamps the approver (approverID) and time, and transitions
// it to APPROVED. It is a back-office action (staff/admin) performed on any
// user's application, so it is not ownership-scoped.
func (s *financingService) Approve(ctx context.Context, approverID uuid.UUID, id uint, req domain.ApproveFinancingRequest) (*domain.FinancingResponse, error) {
	f, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, domain.ErrNotFound
	}
	if f.Status != domain.FinancingStatusApplied {
		return nil, domain.ErrFinancingNotApplied
	}

	// First due date: the approver's choice wins, else the date applied for, else
	// default to one month out.
	firstDue := time.Now().AddDate(0, 1, 0)
	switch {
	case req.FirstDueDate != nil:
		firstDue = *req.FirstDueDate
	case f.FirstDueDate != nil:
		firstDue = *f.FirstDueDate
	}

	totalPrice := req.CostPrice + req.MarginAmount
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

	terms := domain.ApprovedTerms{
		CostPrice:    req.CostPrice,
		MarginAmount: req.MarginAmount,
		TotalPrice:   totalPrice,
		DownPayment:  req.DownPayment,
		Tenor:        req.Tenor,
		FirstDueDate: &firstDue,
		ApprovedBy:   approverID,
		ApprovedAt:   time.Now(),
	}
	if err := s.repo.Approve(ctx, id, terms, schedule); err != nil {
		return nil, fmt.Errorf("failed to approve financing: %w", err)
	}

	f, err = s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to reload financing: %w", err)
	}
	resp := domain.ToFinancingResponse(f)
	return &resp, nil
}

// SignAkad signs an APPROVED financing, transitioning it to ACTIVE so installments
// can be paid. Signing is the point at which the obligation becomes binding, and
// only the owner may sign their own akad.
func (s *financingService) SignAkad(ctx context.Context, userID uuid.UUID, id uint) (*domain.FinancingResponse, error) {
	f, err := s.repo.FindByID(ctx, id)
	if err != nil || f.UserID != userID {
		return nil, domain.ErrNotFound
	}
	if f.Status != domain.FinancingStatusApproved {
		return nil, domain.ErrFinancingNotApproved
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
