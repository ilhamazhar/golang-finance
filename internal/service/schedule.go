package service

import (
	"errors"
	"time"

	"github.com/ilhamazhar/golang-gpt/internal/domain"
)

// Errors returned by GenerateMurabahahSchedule.
var (
	ErrInvalidTenor        = errors.New("tenor must be greater than zero")
	ErrInvalidCostPrice    = errors.New("cost price must be greater than zero")
	ErrNegativeMargin      = errors.New("margin amount must not be negative")
	ErrNegativeDownPayment = errors.New("down payment must not be negative")
	ErrDownPaymentTooHigh  = errors.New("down payment must be less than total price")
)

// ScheduleParams holds the inputs for a Murabahah installment schedule. All
// money values are integer minor units (e.g. rupiah).
type ScheduleParams struct {
	CostPrice    int64     // harga pokok
	MarginAmount int64     // keuntungan (fixed)
	DownPayment  int64     // uang muka, applied against the principal (pokok)
	Tenor        int       // number of monthly installments
	FirstDueDate time.Time // due date of installment #1
}

// GenerateMurabahahSchedule builds the immutable repayment schedule for a
// Murabahah financing.
//
// Allocation rule (flat method, equal installments): the financed amount
// (CostPrice + MarginAmount - DownPayment) is spread evenly across the tenor so
// the monthly Amount is as flat as possible; the margin is spread the same way;
// and the principal portion is derived as Amount - MarginPart. Any rounding
// remainder lands on the final installment. This keeps the monthly figure clean
// (no per-line off-by-one from flooring two portions independently) while
// guaranteeing:
//
//	sum(PrincipalPart) == CostPrice - DownPayment
//	sum(MarginPart)    == MarginAmount
//	sum(Amount)        == (CostPrice + MarginAmount) - DownPayment
//	Amount[i]          == PrincipalPart[i] + MarginPart[i]
//
// The total obligation never depends on time — that is the riba-free property.
// Due dates step one calendar month at a time from FirstDueDate.
func GenerateMurabahahSchedule(p ScheduleParams) ([]domain.Installment, error) {
	switch {
	case p.Tenor <= 0:
		return nil, ErrInvalidTenor
	case p.CostPrice <= 0:
		return nil, ErrInvalidCostPrice
	case p.MarginAmount < 0:
		return nil, ErrNegativeMargin
	case p.DownPayment < 0:
		return nil, ErrNegativeDownPayment
	case p.DownPayment >= p.CostPrice+p.MarginAmount:
		return nil, ErrDownPaymentTooHigh
	}

	financedTotal := p.CostPrice + p.MarginAmount - p.DownPayment
	marginFinanced := p.MarginAmount

	// Allocate the monthly total first so each installment is even, then carve
	// the margin out of it and treat the rest as principal (pokok).
	amountParts := allocateEven(financedTotal, p.Tenor)
	marginParts := allocateEven(marginFinanced, p.Tenor)

	schedule := make([]domain.Installment, p.Tenor)
	for i := 0; i < p.Tenor; i++ {
		schedule[i] = domain.Installment{
			InstallmentNo: i + 1,
			DueDate:       addMonths(p.FirstDueDate, i),
			PrincipalPart: amountParts[i] - marginParts[i],
			MarginPart:    marginParts[i],
			Amount:        amountParts[i],
			Status:        domain.InstallmentStatusUnpaid,
		}
	}
	return schedule, nil
}

// allocateEven splits total into n integer parts whose sum is exactly total.
// Each part is floor(total/n); the last part takes the remainder. n must be > 0.
func allocateEven(total int64, n int) []int64 {
	parts := make([]int64, n)
	base := total / int64(n)
	var assigned int64
	for i := 0; i < n-1; i++ {
		parts[i] = base
		assigned += base
	}
	parts[n-1] = total - assigned
	return parts
}

// addMonths advances t by n calendar months. To avoid month-overflow surprises
// (e.g. Jan 31 + 1 month landing on Mar 3), a day that overflows the target
// month is clamped to that month's last day.
func addMonths(t time.Time, n int) time.Time {
	if n == 0 {
		return t
	}
	year, month, day := t.Date()
	target := time.Date(year, month+time.Month(n), 1, t.Hour(), t.Minute(), t.Second(), t.Nanosecond(), t.Location())
	last := lastDayOfMonth(target.Year(), target.Month())
	if day > last {
		day = last
	}
	return time.Date(target.Year(), target.Month(), day, t.Hour(), t.Minute(), t.Second(), t.Nanosecond(), t.Location())
}

func lastDayOfMonth(year int, month time.Month) int {
	// Day 0 of the next month is the last day of this month.
	return time.Date(year, month+1, 0, 0, 0, 0, 0, time.UTC).Day()
}
