package domain

import "errors"

var (
	ErrNotFound         = errors.New("not found")
	ErrEmailNotVerified = errors.New("email not verified")
)
