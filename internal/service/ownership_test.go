package service

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/ilhamazhar/golang-gpt/internal/domain"
)

// --- Fakes ---------------------------------------------------------------

// fakeFinancingRepo implements domain.FinancingRepository. Only the read
// methods exercised by these tests carry behaviour; the rest are no-ops so the
// type still satisfies the interface.
type fakeFinancingRepo struct {
	byID    map[uint]*domain.Financing
	byUser  []domain.Financing // returned by FindByUser
	all     []domain.Financing // returned by FindAll
	lastAll bool               // true if FindAll was the last list call
}

func (f *fakeFinancingRepo) FindByID(_ context.Context, id uint) (*domain.Financing, error) {
	fin, ok := f.byID[id]
	if !ok {
		return nil, domain.ErrNotFound
	}
	return fin, nil
}

func (f *fakeFinancingRepo) FindByUser(_ context.Context, _ uuid.UUID, _, _ int, _, _, _ string) ([]domain.Financing, int64, error) {
	f.lastAll = false
	return f.byUser, int64(len(f.byUser)), nil
}

func (f *fakeFinancingRepo) FindAll(_ context.Context, _, _ int, _, _, _ string) ([]domain.Financing, int64, error) {
	f.lastAll = true
	return f.all, int64(len(f.all)), nil
}

func (f *fakeFinancingRepo) Create(context.Context, *domain.Financing) error { return nil }
func (f *fakeFinancingRepo) UpdateStatus(context.Context, uint, domain.FinancingStatus, *time.Time) error {
	return nil
}
func (f *fakeFinancingRepo) Approve(context.Context, uint, domain.ApprovedTerms, []domain.Installment) error {
	return nil
}
func (f *fakeFinancingRepo) FindInstallment(context.Context, uint, int) (*domain.Installment, error) {
	return nil, domain.ErrNotFound
}
func (f *fakeFinancingRepo) FindInstallmentByID(context.Context, uint) (*domain.Installment, error) {
	return nil, domain.ErrNotFound
}
func (f *fakeFinancingRepo) MarkInstallmentPaid(context.Context, uint, uint, time.Time) error {
	return nil
}
func (f *fakeFinancingRepo) CountUnpaidInstallments(context.Context, uint) (int64, error) {
	return 0, nil
}

// fakePaymentRepo implements domain.PaymentRepository; only FindByOrderRef matters.
type fakePaymentRepo struct {
	byRef map[string]*domain.Payment
}

func (f *fakePaymentRepo) FindByOrderRef(_ context.Context, ref string) (*domain.Payment, error) {
	p, ok := f.byRef[ref]
	if !ok {
		return nil, domain.ErrNotFound
	}
	return p, nil
}

func (f *fakePaymentRepo) Create(context.Context, *domain.Payment) error { return nil }
func (f *fakePaymentRepo) UpdateQRData(context.Context, uint, string, string, *time.Time) error {
	return nil
}
func (f *fakePaymentRepo) UpdateStatus(context.Context, uint, domain.PaymentStatus, *time.Time) error {
	return nil
}

// fakeUserRepo implements domain.UserRepository; only FindByID/Update carry
// behaviour for the role tests.
type fakeUserRepo struct {
	byID map[uuid.UUID]*domain.User
}

func (f *fakeUserRepo) FindByID(_ context.Context, id uuid.UUID) (*domain.User, error) {
	u, ok := f.byID[id]
	if !ok {
		return nil, domain.ErrNotFound
	}
	return u, nil
}
func (f *fakeUserRepo) Update(_ context.Context, u *domain.User) error {
	f.byID[u.ID] = u
	return nil
}
func (f *fakeUserRepo) Create(context.Context, *domain.User) error { return nil }
func (f *fakeUserRepo) FindByEmail(context.Context, string) (*domain.User, error) {
	return nil, domain.ErrNotFound
}
func (f *fakeUserRepo) FindAll(context.Context, int, int, string, string, string) ([]domain.User, int64, error) {
	return nil, 0, nil
}
func (f *fakeUserRepo) Delete(context.Context, uuid.UUID) error { return nil }

// --- Tests ---------------------------------------------------------------

func TestFinancingService_GetByID_OwnershipAndAdminBypass(t *testing.T) {
	owner := uuid.New()
	other := uuid.New()
	repo := &fakeFinancingRepo{
		byID: map[uint]*domain.Financing{
			1: {ID: 1, UserID: owner, User: domain.User{Name: "Budi"}, AssetName: "Motor", Status: domain.FinancingStatusActive},
		},
	}
	svc := NewFinancingService(repo, nil)

	t.Run("owner reads own financing", func(t *testing.T) {
		resp, err := svc.GetByID(context.Background(), owner, 1, false)
		if err != nil {
			t.Fatalf("owner should read own financing, got error: %v", err)
		}
		if resp.ID != 1 {
			t.Errorf("got financing ID %d, want 1", resp.ID)
		}
	})

	t.Run("non-owner is denied (looks like not found)", func(t *testing.T) {
		if _, err := svc.GetByID(context.Background(), other, 1, false); err == nil {
			t.Fatal("non-owner must not read another user's financing")
		}
	})

	t.Run("admin bypasses ownership", func(t *testing.T) {
		resp, err := svc.GetByID(context.Background(), other, 1, true)
		if err != nil {
			t.Fatalf("admin should read any financing, got error: %v", err)
		}
		if resp.ID != 1 {
			t.Errorf("got financing ID %d, want 1", resp.ID)
		}
		// Admin reading another user's financing should see whose it is.
		if resp.UserName != "Budi" {
			t.Errorf("resp.UserName = %q, want \"Budi\"", resp.UserName)
		}
	})
}

func TestFinancingService_List_AdminSeesAll(t *testing.T) {
	owner := uuid.New()
	repo := &fakeFinancingRepo{
		byUser: []domain.Financing{{ID: 1, UserID: owner, User: domain.User{Name: "Budi"}}},
		all: []domain.Financing{
			{ID: 1, UserID: owner, User: domain.User{Name: "Budi"}},
			{ID: 2, UserID: uuid.New(), User: domain.User{Name: "Siti"}},
			{ID: 3, UserID: uuid.New(), User: domain.User{Name: "Andi"}},
		},
	}
	svc := NewFinancingService(repo, nil)

	t.Run("user sees only their own", func(t *testing.T) {
		_, total, err := svc.List(context.Background(), owner, 1, 10, "", "", "", false)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if total != 1 {
			t.Errorf("user total = %d, want 1", total)
		}
		if repo.lastAll {
			t.Error("user list must use FindByUser, not FindAll")
		}
	})

	t.Run("admin sees all", func(t *testing.T) {
		result, total, err := svc.List(context.Background(), owner, 1, 10, "", "", "", true)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if total != 3 {
			t.Errorf("admin total = %d, want 3", total)
		}
		if !repo.lastAll {
			t.Error("admin list must use FindAll")
		}
		// The owner's name must be carried into the response so an admin can
		// see who each financing belongs to.
		if result[1].UserName != "Siti" {
			t.Errorf("result[1].UserName = %q, want \"Siti\"", result[1].UserName)
		}
		if result[0].UserID != owner {
			t.Errorf("result[0].UserID = %v, want %v", result[0].UserID, owner)
		}
	})
}

func TestUserService_UpdateRole(t *testing.T) {
	id := uuid.New()
	repo := &fakeUserRepo{
		byID: map[uuid.UUID]*domain.User{
			id: {ID: id, Name: "Siti", Role: domain.RoleUser},
		},
	}
	svc := NewUserService(repo)

	t.Run("promotes user to staff", func(t *testing.T) {
		resp, err := svc.UpdateRole(context.Background(), id, domain.RoleStaff)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resp.Role != domain.RoleStaff {
			t.Errorf("response role = %q, want staff", resp.Role)
		}
		if repo.byID[id].Role != domain.RoleStaff {
			t.Errorf("persisted role = %q, want staff", repo.byID[id].Role)
		}
	})

	t.Run("unknown user is not found", func(t *testing.T) {
		if _, err := svc.UpdateRole(context.Background(), uuid.New(), domain.RoleAdmin); err == nil {
			t.Fatal("expected error for unknown user")
		}
	})
}

func TestPaymentService_GetStatus_OwnershipAndAdminBypass(t *testing.T) {
	owner := uuid.New()
	other := uuid.New()
	repo := &fakePaymentRepo{
		byRef: map[string]*domain.Payment{
			"ORDER-1": {OrderRef: "ORDER-1", UserID: owner, Status: domain.PaymentStatusPaid, Amount: 5000},
		},
	}
	svc := NewPaymentService(repo, nil, nil)

	t.Run("owner reads own payment", func(t *testing.T) {
		resp, err := svc.GetStatus(context.Background(), owner, "ORDER-1", false)
		if err != nil {
			t.Fatalf("owner should read own payment, got error: %v", err)
		}
		if resp.OrderRef != "ORDER-1" {
			t.Errorf("got order ref %q, want ORDER-1", resp.OrderRef)
		}
	})

	t.Run("non-owner is denied", func(t *testing.T) {
		if _, err := svc.GetStatus(context.Background(), other, "ORDER-1", false); err == nil {
			t.Fatal("non-owner must not read another user's payment")
		}
	})

	t.Run("admin bypasses ownership", func(t *testing.T) {
		resp, err := svc.GetStatus(context.Background(), other, "ORDER-1", true)
		if err != nil {
			t.Fatalf("admin should read any payment, got error: %v", err)
		}
		if resp.OrderRef != "ORDER-1" {
			t.Errorf("got order ref %q, want ORDER-1", resp.OrderRef)
		}
	})
}
