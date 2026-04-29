// Package dto holds the wire-shape Data Transfer Objects shared
// between the handler (HTTP layer) and the service (application layer).
//
// Why a dedicated package?
//   - The domain layer must remain free of HTTP/JSON concerns so the
//     business rules can be tested without a server.
//   - The service and handler both need a shared vocabulary for inputs
//     and outputs; centralising the structs keeps that vocabulary in
//     one place and makes the API contract easy to scan.
//
// Conventions observed by every type below:
//   - Request structs carry json tags so the HTTP handler can bind a
//     request body directly with c.ShouldBindJSON.
//   - Fields sourced from the URL path (AccountID) use `json:"-"` so
//     a malicious client cannot override them via the body.
//   - Response structs reuse the domain value types where appropriate
//     (domain.Holder) — the dependency points one way: dto -> domain.
package dto

import (
	"time"

	"bank-account/domain"
)

// HolderInput is the unvalidated holder block from a create request.
// Every field is required at the wire layer; the service trims and
// validates them via mail.ParseAddress (email), digit-counting (phone),
// and time.Parse + age check (date_of_birth, must be 18+).
type HolderInput struct {
	Name        string `json:"name"`
	Email       string `json:"email"`
	Phone       string `json:"phone"`
	DateOfBirth string `json:"date_of_birth"`
}

// CreateAccountRequest is the input to Service.CreateAccount. Bound
// from the body of POST /api/v1/accounts.
//
// AccountType accepts "savings" or "current". An empty string defaults
// to "savings" so older clients keep working.
type CreateAccountRequest struct {
	Holder      HolderInput `json:"holder"`
	AccountType string      `json:"account_type"`
}

// GetAccountRequest is the input to Service.GetAccount. AccountID
// comes from the URL path.
type GetAccountRequest struct {
	AccountID string `json:"-"`
}

// CloseAccountRequest is the input to Service.CloseAccount.
type CloseAccountRequest struct {
	AccountID string `json:"-"`
}

// GetBalanceRequest is the input to Service.GetBalance.
type GetBalanceRequest struct {
	AccountID string `json:"-"`
}

// DepositRequest is the input to Service.Deposit. AccountID is set
// from the URL path; Amount is bound from the body and is in decimal
// currency units (e.g. 10.50 = ten rupees fifty paise);
// IdempotencyKey is set from the Idempotency-Key request header.
//
// When IdempotencyKey is non-empty and matches a key already applied
// to this account, the original result is returned and no balance
// change occurs.
type DepositRequest struct {
	AccountID      string  `json:"-"`
	Amount         float64 `json:"amount"`
	IdempotencyKey string  `json:"-"`
}

// WithdrawRequest is the input to Service.Withdraw. Same conventions
// as DepositRequest.
type WithdrawRequest struct {
	AccountID      string  `json:"-"`
	Amount         float64 `json:"amount"`
	IdempotencyKey string  `json:"-"`
}

// ListTransactionsRequest is the input to Service.ListTransactions.
type ListTransactionsRequest struct {
	AccountID string `json:"-"`
}

// Transaction is the wire shape of one audit-log entry. Mirrors
// domain.Transaction with extra display strings for the amounts.
type Transaction struct {
	ID                  string    `json:"id"`
	Type                string    `json:"type"`
	Amount              int64     `json:"amount"`
	AmountDisplay       string    `json:"amount_display"`
	BalanceAfter        int64     `json:"balance_after"`
	BalanceAfterDisplay string    `json:"balance_after_display"`
	Timestamp           time.Time `json:"timestamp"`
	IdempotencyKey      string    `json:"idempotency_key,omitempty"`
}

// ListTransactionsResponse is the response shape for
// GET /api/v1/accounts/:id/transactions.
type ListTransactionsResponse struct {
	AccountID    string        `json:"account_id"`
	Transactions []Transaction `json:"transactions"`
}

// AccountInfo is the response shape for every endpoint that returns
// lifecycle info about a single account: Create, Get, Close.
//
// AccountID is the internal UUID used in URLs.
// AccountNumber is the 12-digit public-facing identifier.
type AccountInfo struct {
	AccountID     string        `json:"account_id"`
	AccountNumber string        `json:"account_number"`
	AccountType   string        `json:"account_type"`
	Currency      string        `json:"currency"`
	Balance       int64         `json:"balance"`
	Open          bool          `json:"open"`
	Status        string        `json:"status"`
	Holder        domain.Holder `json:"holder"`
	CreatedAt     time.Time     `json:"created_at"`
}

// BalanceInfo is the response shape for balance-mutating and balance-
// reading endpoints. Returned by GetBalance, Deposit, and Withdraw.
type BalanceInfo struct {
	AccountID      string `json:"account_id"`
	Balance        int64  `json:"balance"`
	BalanceDisplay string `json:"balance_display"`
}
