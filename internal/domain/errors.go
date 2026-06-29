package domain

import "errors"

var (
	ErrNotFound         = errors.New("not found")
	ErrEmailNotVerified = errors.New("email not verified")

	// Financing state errors.
	ErrFinancingNotApplied  = errors.New("financing is not awaiting approval")
	ErrFinancingNotApproved = errors.New("financing is not approved (cannot sign the akad yet)")
	ErrFinancingNotActive   = errors.New("financing is not active (sign the akad first)")
	ErrInstallmentPaid      = errors.New("installment already paid")
)
