package service

import (
	"testing"
	"time"

	"github.com/ilhamazhar/golang-gpt/internal/domain"
)

func sum(insts []domain.Installment, pick func(domain.Installment) int64) int64 {
	var total int64
	for _, i := range insts {
		total += pick(i)
	}
	return total
}

func TestGenerateMurabahahSchedule_TotalsAreExact(t *testing.T) {
	// 10,000,000 cost + 2,000,000 margin, 1,000,000 DP, 12 months.
	p := ScheduleParams{
		CostPrice:    10_000_000,
		MarginAmount: 2_000_000,
		DownPayment:  1_000_000,
		Tenor:        12,
		FirstDueDate: time.Date(2026, time.July, 15, 0, 0, 0, 0, time.UTC),
	}

	sched, err := GenerateMurabahahSchedule(p)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(sched) != 12 {
		t.Fatalf("expected 12 installments, got %d", len(sched))
	}

	wantPrincipal := p.CostPrice - p.DownPayment // 9,000,000
	if got := sum(sched, func(i domain.Installment) int64 { return i.PrincipalPart }); got != wantPrincipal {
		t.Errorf("principal sum = %d, want %d", got, wantPrincipal)
	}
	if got := sum(sched, func(i domain.Installment) int64 { return i.MarginPart }); got != p.MarginAmount {
		t.Errorf("margin sum = %d, want %d", got, p.MarginAmount)
	}
	wantTotal := p.CostPrice + p.MarginAmount - p.DownPayment // 11,000,000
	if got := sum(sched, func(i domain.Installment) int64 { return i.Amount }); got != wantTotal {
		t.Errorf("amount sum = %d, want %d", got, wantTotal)
	}
}

func TestGenerateMurabahahSchedule_RemainderOnLastInstallment(t *testing.T) {
	// 1,000,000 financed over 3 months -> 333,333 x2 + 333,334 last.
	p := ScheduleParams{
		CostPrice:    1_000_000,
		MarginAmount: 0,
		Tenor:        3,
		FirstDueDate: time.Date(2026, time.January, 31, 0, 0, 0, 0, time.UTC),
	}
	sched, err := GenerateMurabahahSchedule(p)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if sched[0].PrincipalPart != 333_333 || sched[1].PrincipalPart != 333_333 {
		t.Errorf("first two parts = %d, %d; want 333333 each", sched[0].PrincipalPart, sched[1].PrincipalPart)
	}
	if sched[2].PrincipalPart != 333_334 {
		t.Errorf("last part = %d; want 333334 (absorbs remainder)", sched[2].PrincipalPart)
	}
	if got := sum(sched, func(i domain.Installment) int64 { return i.Amount }); got != 1_000_000 {
		t.Errorf("amount sum = %d, want 1000000", got)
	}
}

func TestGenerateMurabahahSchedule_EvenMonthlyAmount(t *testing.T) {
	// cost 20M + margin 5M - DP 1M = 24M financed; 24M / 12 = 2,000,000 flat.
	// Every monthly Amount must be exactly 2,000,000 (no per-line off-by-one).
	p := ScheduleParams{
		CostPrice:    20_000_000,
		MarginAmount: 5_000_000,
		DownPayment:  1_000_000,
		Tenor:        12,
		FirstDueDate: time.Date(2026, time.July, 15, 0, 0, 0, 0, time.UTC),
	}
	sched, err := GenerateMurabahahSchedule(p)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for _, inst := range sched {
		if inst.Amount != 2_000_000 {
			t.Errorf("installment %d amount = %d, want 2000000", inst.InstallmentNo, inst.Amount)
		}
		if inst.PrincipalPart+inst.MarginPart != inst.Amount {
			t.Errorf("installment %d parts %d+%d != amount %d",
				inst.InstallmentNo, inst.PrincipalPart, inst.MarginPart, inst.Amount)
		}
	}
	if got := sum(sched, func(i domain.Installment) int64 { return i.PrincipalPart }); got != 19_000_000 {
		t.Errorf("principal sum = %d, want 19000000", got)
	}
	if got := sum(sched, func(i domain.Installment) int64 { return i.MarginPart }); got != 5_000_000 {
		t.Errorf("margin sum = %d, want 5000000", got)
	}
}

func TestGenerateMurabahahSchedule_DueDatesStepMonthlyWithClamp(t *testing.T) {
	// Starting Jan 31 must clamp Feb to 28 (2026 is not a leap year), not roll into March.
	p := ScheduleParams{
		CostPrice:    300,
		Tenor:        3,
		FirstDueDate: time.Date(2026, time.January, 31, 0, 0, 0, 0, time.UTC),
	}
	sched, err := GenerateMurabahahSchedule(p)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := []time.Time{
		time.Date(2026, time.January, 31, 0, 0, 0, 0, time.UTC),
		time.Date(2026, time.February, 28, 0, 0, 0, 0, time.UTC),
		time.Date(2026, time.March, 31, 0, 0, 0, 0, time.UTC),
	}
	for i, w := range want {
		if !sched[i].DueDate.Equal(w) {
			t.Errorf("installment %d due = %s, want %s", i+1, sched[i].DueDate, w)
		}
	}
}

func TestGenerateMurabahahSchedule_InstallmentNumbersAndStatus(t *testing.T) {
	p := ScheduleParams{CostPrice: 1200, Tenor: 6, FirstDueDate: time.Date(2026, time.June, 1, 0, 0, 0, 0, time.UTC)}
	sched, _ := GenerateMurabahahSchedule(p)
	for i, inst := range sched {
		if inst.InstallmentNo != i+1 {
			t.Errorf("installment[%d].InstallmentNo = %d, want %d", i, inst.InstallmentNo, i+1)
		}
		if inst.Status != domain.InstallmentStatusUnpaid {
			t.Errorf("installment[%d].Status = %s, want UNPAID", i, inst.Status)
		}
	}
}

func TestGenerateMurabahahSchedule_Validation(t *testing.T) {
	base := ScheduleParams{CostPrice: 1000, MarginAmount: 100, Tenor: 6, FirstDueDate: time.Now()}

	cases := []struct {
		name   string
		mutate func(ScheduleParams) ScheduleParams
		want   error
	}{
		{"zero tenor", func(p ScheduleParams) ScheduleParams { p.Tenor = 0; return p }, ErrInvalidTenor},
		{"negative tenor", func(p ScheduleParams) ScheduleParams { p.Tenor = -1; return p }, ErrInvalidTenor},
		{"zero cost", func(p ScheduleParams) ScheduleParams { p.CostPrice = 0; return p }, ErrInvalidCostPrice},
		{"negative margin", func(p ScheduleParams) ScheduleParams { p.MarginAmount = -1; return p }, ErrNegativeMargin},
		{"negative dp", func(p ScheduleParams) ScheduleParams { p.DownPayment = -1; return p }, ErrNegativeDownPayment},
		{"dp too high", func(p ScheduleParams) ScheduleParams { p.DownPayment = 5000; return p }, ErrDownPaymentTooHigh},
		// DP between cost (1000) and total (1100) would yield negative principal; reject it.
		{"dp exceeds cost", func(p ScheduleParams) ScheduleParams { p.DownPayment = 1050; return p }, ErrDownPaymentTooHigh},
		{"dp equals cost", func(p ScheduleParams) ScheduleParams { p.DownPayment = 1000; return p }, ErrDownPaymentTooHigh},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := GenerateMurabahahSchedule(tc.mutate(base))
			if err != tc.want {
				t.Errorf("got error %v, want %v", err, tc.want)
			}
		})
	}
}
