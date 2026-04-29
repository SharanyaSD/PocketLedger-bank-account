// Package domain holds the core Account aggregate. All business rules
// and concurrency guards live here; nothing in this package knows about
// HTTP, JSON wire shapes, or storage. The mutex on Account serialises
// every check+update so concurrent goroutines cannot corrupt the
// balance, race the lifecycle flag, or split a transaction append from
// the balance change it records.
package domain

import (
	"crypto/rand"
	"math/big"
	"sync"
	"time"

	"github.com/google/uuid"

	apperrors "bank-account/errors"
)

const (
	// accountNumberDigits is the length of the public-facing account
	// number (distinct from the internal UUID used in URLs).
	accountNumberDigits = 12

	// DailyWithdrawalLimit caps cumulative withdrawals per UTC day per
	// account. Currently 50,000.00 (5,000,000 paise / cents).
	DailyWithdrawalLimit int64 = 5_000_000
)

// Transaction kind values stored in Transaction.Type. Kept as plain
// strings so the audit log is human-readable in the JSON snapshot.
const (
	TxOpen     = "open"
	TxClose    = "close"
	TxDeposit  = "deposit"
	TxWithdraw = "withdraw"
)

// Holder is the account-holder identity captured at creation time.
// Once on an Account, holder details are immutable for the account's
// lifetime.
type Holder struct {
	Name        string `json:"name"`
	Email       string `json:"email"`
	Phone       string `json:"phone"`
	DateOfBirth string `json:"date_of_birth"`
}

// Transaction is one entry in the per-account audit log. Every state
// transition (open / close / deposit / withdraw) appends one inside
// the same critical section as the balance change, so the ledger
// never diverges from the balance.
type Transaction struct {
	ID             string    `json:"id"`
	Type           string    `json:"type"`
	Amount         int64     `json:"amount"`
	BalanceAfter   int64     `json:"balance_after"`
	Timestamp      time.Time `json:"timestamp"`
	IdempotencyKey string    `json:"idempotency_key,omitempty"`
}

// AccountSnapshot is the point-in-time view of an account, returned by
// Account.Snapshot.
type AccountSnapshot struct {
	Balance       int64
	Open          bool
	Holder        Holder
	AccountNumber string
	AccountType   string
	Currency      string
	CreatedAt     time.Time
}

// AccountState is the full serialisable state of an account, used by
// the persistence layer for save/restore. Unlike AccountSnapshot, it
// includes the audit log and the daily-limit tracking fields.
type AccountState struct {
	Balance          int64         `json:"balance"`
	Open             bool          `json:"open"`
	Closed           bool          `json:"closed"`
	Holder           Holder        `json:"holder"`
	AccountNumber    string        `json:"account_number"`
	AccountType      string        `json:"account_type"`
	Currency         string        `json:"currency"`
	CreatedAt        time.Time     `json:"created_at"`
	Transactions     []Transaction `json:"transactions"`
	DailyWithdrawn   int64         `json:"daily_withdrawn"`
	DailyWindowStart time.Time     `json:"daily_window_start"`
}

// Account is a single bank account.
type Account struct {
	mu      sync.Mutex
	balance int64
	open    bool
	// closed is sticky: once true it never flips back.
	closed        bool
	holder        Holder
	accountNumber string
	accountType   string
	currency      string
	createdAt     time.Time

	// transactions is the append-only audit log for this account.
	// Every state change appends here under the same lock.
	transactions []Transaction

	// idempotencyIndex maps Idempotency-Key -> position in
	// transactions. Allows O(1) lookup on retries so a duplicate
	// deposit/withdraw returns the original result instead of being
	// re-applied.
	idempotencyIndex map[string]int

	// dailyWithdrawn tracks cumulative withdrawals within the rolling
	// UTC day starting at dailyWindowStart. Reset when a withdrawal
	// crosses into a new UTC day.
	dailyWithdrawn   int64
	dailyWindowStart time.Time
}

// NewAccount returns an open account bound to holder with a freshly
// generated account number. The caller (service layer) must have
// validated holder and accountType.
func NewAccount(holder Holder, accountType string) *Account {
	now := time.Now().UTC()
	a := &Account{
		open:             true,
		holder:           holder,
		accountNumber:    generateAccountNumber(),
		accountType:      accountType,
		currency:         "INR",
		createdAt:        now,
		dailyWindowStart: dayOf(now),
	}
	a.transactions = append(a.transactions, Transaction{
		ID:           uuid.NewString(),
		Type:         TxOpen,
		Amount:       0,
		BalanceAfter: 0,
		Timestamp:    now,
	})
	return a
}

// RestoreAccount reconstructs an Account from a persisted state.
// Used by the store after loading the snapshot file on startup.
func RestoreAccount(s AccountState) *Account {
	a := &Account{
		balance:          s.Balance,
		open:             s.Open,
		closed:           s.Closed,
		holder:           s.Holder,
		accountNumber:    s.AccountNumber,
		accountType:      s.AccountType,
		currency:         s.Currency,
		createdAt:        s.CreatedAt,
		transactions:     append([]Transaction(nil), s.Transactions...),
		dailyWithdrawn:   s.DailyWithdrawn,
		dailyWindowStart: s.DailyWindowStart,
	}
	if len(a.transactions) > 0 {
		a.idempotencyIndex = make(map[string]int, len(a.transactions))
		for i, t := range a.transactions {
			if t.IdempotencyKey != "" {
				a.idempotencyIndex[t.IdempotencyKey] = i
			}
		}
	}
	return a
}

// Open opens the account if it has never been closed. No-op on an
// already-open account; ErrAlreadyClosed once closed.
func (a *Account) Open() error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.closed {
		return apperrors.ErrAlreadyClosed
	}
	a.open = true
	return nil
}

// Close marks the account closed and appends a close transaction. A
// second Close returns ErrAlreadyClosed.
func (a *Account) Close() error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.closed {
		return apperrors.ErrAlreadyClosed
	}
	a.open = false
	a.closed = true
	a.transactions = append(a.transactions, Transaction{
		ID:           uuid.NewString(),
		Type:         TxClose,
		Amount:       0,
		BalanceAfter: a.balance,
		Timestamp:    time.Now().UTC(),
	})
	return nil
}

// Deposit adds amount (in paise) to the balance and returns the
// resulting Transaction. If idempotencyKey is non-empty and a previous
// transaction used the same key on this account, the original
// Transaction is returned and no balance change occurs.
func (a *Account) Deposit(amount int64, idempotencyKey string) (Transaction, error) {
	if amount <= 0 {
		return Transaction{}, apperrors.ErrInvalidAmount
	}
	a.mu.Lock()
	defer a.mu.Unlock()

	if t, ok := a.lookupIdempotentLocked(idempotencyKey); ok {
		return t, nil
	}
	if !a.open {
		return Transaction{}, apperrors.ErrAccountClosed
	}

	a.balance += amount
	return a.appendTxLocked(TxDeposit, amount, idempotencyKey), nil
}

// Withdraw subtracts amount (in paise) from the balance. Enforces the
// daily withdrawal limit and supports idempotency. The check (open +
// sufficient funds + daily limit) and the update happen inside the
// same Lock so two concurrent withdrawals cannot together overdraft
// or exceed the limit.
func (a *Account) Withdraw(amount int64, idempotencyKey string) (Transaction, error) {
	if amount <= 0 {
		return Transaction{}, apperrors.ErrInvalidAmount
	}
	a.mu.Lock()
	defer a.mu.Unlock()

	if t, ok := a.lookupIdempotentLocked(idempotencyKey); ok {
		return t, nil
	}
	if !a.open {
		return Transaction{}, apperrors.ErrAccountClosed
	}
	if a.balance < amount {
		return Transaction{}, apperrors.ErrInsufficientFunds
	}

	now := time.Now().UTC()
	today := dayOf(now)
	if !a.dailyWindowStart.Equal(today) {
		a.dailyWithdrawn = 0
		a.dailyWindowStart = today
	}
	if a.dailyWithdrawn+amount > DailyWithdrawalLimit {
		return Transaction{}, apperrors.ErrDailyLimitExceeded
	}

	a.balance -= amount
	a.dailyWithdrawn += amount
	return a.appendTxLocked(TxWithdraw, amount, idempotencyKey), nil
}

// Balance returns the current balance in paise. ErrAccountClosed on
// a closed account.
func (a *Account) Balance() (int64, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if !a.open {
		return 0, apperrors.ErrAccountClosed
	}
	return a.balance, nil
}

// Snapshot returns a consistent view of the account: balance, open
// status, holder, and immutable metadata.
func (a *Account) Snapshot() AccountSnapshot {
	a.mu.Lock()
	defer a.mu.Unlock()
	return AccountSnapshot{
		Balance:       a.balance,
		Open:          a.open,
		Holder:        a.holder,
		AccountNumber: a.accountNumber,
		AccountType:   a.accountType,
		Currency:      a.currency,
		CreatedAt:     a.createdAt,
	}
}

// Transactions returns a defensive copy of the audit log so callers
// can't mutate the account's history.
func (a *Account) Transactions() []Transaction {
	a.mu.Lock()
	defer a.mu.Unlock()
	out := make([]Transaction, len(a.transactions))
	copy(out, a.transactions)
	return out
}

// State returns the full serialisable state of the account for
// persistence. Callers should treat the returned slice as immutable;
// the audit log is copied defensively.
func (a *Account) State() AccountState {
	a.mu.Lock()
	defer a.mu.Unlock()
	txns := make([]Transaction, len(a.transactions))
	copy(txns, a.transactions)
	return AccountState{
		Balance:          a.balance,
		Open:             a.open,
		Closed:           a.closed,
		Holder:           a.holder,
		AccountNumber:    a.accountNumber,
		AccountType:      a.accountType,
		Currency:         a.currency,
		CreatedAt:        a.createdAt,
		Transactions:     txns,
		DailyWithdrawn:   a.dailyWithdrawn,
		DailyWindowStart: a.dailyWindowStart,
	}
}

// lookupIdempotentLocked checks whether an Idempotency-Key has already
// been applied to this account. Caller must hold a.mu.
func (a *Account) lookupIdempotentLocked(key string) (Transaction, bool) {
	if key == "" || a.idempotencyIndex == nil {
		return Transaction{}, false
	}
	idx, ok := a.idempotencyIndex[key]
	if !ok {
		return Transaction{}, false
	}
	return a.transactions[idx], true
}

// appendTxLocked records a new transaction and updates the idempotency
// index. Caller must hold a.mu.
func (a *Account) appendTxLocked(kind string, amount int64, idempotencyKey string) Transaction {
	t := Transaction{
		ID:             uuid.NewString(),
		Type:           kind,
		Amount:         amount,
		BalanceAfter:   a.balance,
		Timestamp:      time.Now().UTC(),
		IdempotencyKey: idempotencyKey,
	}
	a.transactions = append(a.transactions, t)
	if idempotencyKey != "" {
		if a.idempotencyIndex == nil {
			a.idempotencyIndex = make(map[string]int)
		}
		a.idempotencyIndex[idempotencyKey] = len(a.transactions) - 1
	}
	return t
}

// dayOf returns the UTC midnight for t - used as the daily-window key.
func dayOf(t time.Time) time.Time {
	y, m, d := t.UTC().Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

// generateAccountNumber returns a 12-digit numeric string from
// crypto/rand. The first digit is forced non-zero so the displayed
// length is always 12.
func generateAccountNumber() string {
	digits := make([]byte, accountNumberDigits)
	for i := range digits {
		n, err := rand.Int(rand.Reader, big.NewInt(10))
		if err != nil {
			digits[i] = '0'
			continue
		}
		digits[i] = '0' + byte(n.Int64())
	}
	if digits[0] == '0' {
		digits[0] = '1'
	}
	return string(digits)
}
