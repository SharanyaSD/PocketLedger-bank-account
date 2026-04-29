// Package errors holds the sentinel error values returned by the domain
// and service layers. The HTTP handler maps each one to a specific status
// code and a stable string code in the response envelope.
//
// The package is named "errors" to mirror the directory; importers should
// alias it (e.g. `apperrors "bank-account/errors"`) to avoid collision with
// the standard library "errors" package.
package errors

import stderrors "errors"

var (
	// ErrAccountNotFound is returned when a lookup by ID misses.
	ErrAccountNotFound = stderrors.New("account_not_found")

	// ErrAccountClosed is returned when an operation targets a closed account.
	ErrAccountClosed = stderrors.New("account_closed")

	// ErrAlreadyClosed is returned when Close (or Open) is called on an
	// account that is already in the closed state. Once closed, an account
	// stays closed.
	ErrAlreadyClosed = stderrors.New("already_closed")

	// ErrInsufficientFunds is returned when a withdrawal exceeds the balance.
	ErrInsufficientFunds = stderrors.New("insufficient_funds")

	// ErrInvalidAmount is returned for non-positive or non-finite amounts.
	ErrInvalidAmount = stderrors.New("invalid_amount")

	// ErrInvalidHolder is returned when the account-holder details on
	// create are missing, malformed, or fail validation: empty name,
	// non-parseable email, malformed phone, missing/invalid date of
	// birth, or underage holder.
	ErrInvalidHolder = stderrors.New("invalid_holder")

	// ErrInvalidAccountType is returned when the requested account type
	// is not one of the supported values (savings, current).
	ErrInvalidAccountType = stderrors.New("invalid_account_type")

	// ErrDailyLimitExceeded is returned when a withdrawal would push
	// today's cumulative withdrawn total past the per-account daily
	// limit. The window is rolling-by-UTC-day.
	ErrDailyLimitExceeded = stderrors.New("daily_limit_exceeded")
)
