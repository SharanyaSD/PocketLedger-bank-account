package domain_test

import (
	stderrors "errors"
	"sync"
	"sync/atomic"
	"testing"

	"bank-account/domain"
	apperrors "bank-account/errors"
)

var testHolder = domain.Holder{Name: "Test User", Email: "test@example.com"}

func TestNewAccount_OpenAndZeroBalance(t *testing.T) {
	a := domain.NewAccount(testHolder, "savings")
	bal, err := a.Balance()
	if err != nil {
		t.Fatalf("balance: %v", err)
	}
	if bal != 0 {
		t.Fatalf("balance = %d, want 0", bal)
	}
}

func TestDeposit(t *testing.T) {
	a := domain.NewAccount(testHolder, "savings")
	if _, err := a.Deposit(1000, ""); err != nil {
		t.Fatalf("deposit: %v", err)
	}
	bal, _ := a.Balance()
	if bal != 1000 {
		t.Fatalf("balance = %d, want 1000", bal)
	}
}

func TestDeposit_InvalidAmount(t *testing.T) {
	a := domain.NewAccount(testHolder, "savings")
	for _, v := range []int64{0, -1, -1000} {
		if _, err := a.Deposit(v, ""); !stderrors.Is(err, apperrors.ErrInvalidAmount) {
			t.Fatalf("deposit %d: %v, want ErrInvalidAmount", v, err)
		}
	}
}

func TestWithdraw_Insufficient(t *testing.T) {
	a := domain.NewAccount(testHolder, "savings")
	_, _ = a.Deposit(100, "")
	if _, err := a.Withdraw(200, ""); !stderrors.Is(err, apperrors.ErrInsufficientFunds) {
		t.Fatalf("withdraw: %v, want ErrInsufficientFunds", err)
	}
}

func TestClose_ThenOpsRejected(t *testing.T) {
	a := domain.NewAccount(testHolder, "savings")
	if err := a.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	if _, err := a.Deposit(100, ""); !stderrors.Is(err, apperrors.ErrAccountClosed) {
		t.Fatalf("deposit on closed: %v", err)
	}
}

func TestConcurrentDeposits(t *testing.T) {
	a := domain.NewAccount(testHolder, "savings")
	const n = 1000
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = a.Deposit(1, "")
		}()
	}
	wg.Wait()
	bal, _ := a.Balance()
	if bal != int64(n) {
		t.Fatalf("balance = %d, want %d", bal, n)
	}
}

func TestConcurrentWithdrawals_NoOverdraft(t *testing.T) {
	a := domain.NewAccount(testHolder, "savings")
	if _, err := a.Deposit(100, ""); err != nil {
		t.Fatalf("seed: %v", err)
	}
	const attempts = 200
	var success int64
	var wg sync.WaitGroup
	for i := 0; i < attempts; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := a.Withdraw(1, "")
			if err == nil {
				atomic.AddInt64(&success, 1)
			}
		}()
	}
	wg.Wait()
	if success != 100 {
		t.Fatalf("successful withdrawals = %d, want 100", success)
	}
}
