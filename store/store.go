// Package store is the in-memory account registry. It maps UUID
// strings to *domain.Account and protects the map (not the accounts)
// with a sync.RWMutex: many goroutines can resolve an ID concurrently,
// only create/delete take the write lock.
//
// Note the lock split: the RWMutex here guards the *map*. Each Account
// has its own sync.Mutex guarding *balance state*. They are independent.
package store

import (
	"sync"
	"time"

	"github.com/google/uuid"

	"bank-account/domain"
	apperrors "bank-account/errors"
)

// AccountSummary is a snapshot of an account suitable for List output.
type AccountSummary struct {
	ID            string        `json:"id"`
	AccountNumber string        `json:"account_number"`
	AccountType   string        `json:"account_type"`
	Currency      string        `json:"currency"`
	Balance       int64         `json:"balance"`
	Open          bool          `json:"open"`
	Holder        domain.Holder `json:"holder"`
	CreatedAt     time.Time     `json:"created_at"`
}

// Store is a goroutine-safe registry of accounts. With an attached
// Persister, mutating callers (typically the service layer) call
// Snapshot to flush state to disk after each successful change; on
// startup the store loads any existing snapshot.
type Store struct {
	mu        sync.RWMutex
	accounts  map[string]*domain.Account
	persister *Persister
}

// NewStore returns an empty in-memory Store with no persistence.
func NewStore() *Store {
	return &Store{accounts: make(map[string]*domain.Account)}
}

// WithPersister attaches p to the store and immediately loads any
// existing snapshot from disk. Subsequent calls to Snapshot will
// write through p. Returns the same store for fluent construction.
func (s *Store) WithPersister(p *Persister) *Store {
	s.persister = p
	snap, err := p.Load()
	if err != nil {
		panic("store: failed to load snapshot from " + p.Path() + ": " + err.Error())
	}
	s.mu.Lock()
	for id, st := range snap.Accounts {
		s.accounts[id] = domain.RestoreAccount(st)
	}
	s.mu.Unlock()
	return s
}

func (s *Store) Snapshot() error {
	if s.persister == nil {
		return nil
	}
	s.mu.RLock()
	accts := make(map[string]domain.AccountState, len(s.accounts))
	for id, acc := range s.accounts {
		accts[id] = acc.State()
	}
	s.mu.RUnlock()
	return s.persister.Save(Snapshot{
		Version:  snapshotVersion,
		Accounts: accts,
	})
}

func (s *Store) Create(holder domain.Holder, accountType string) (string, *domain.Account, error) {
	id := uuid.NewString()
	acc := domain.NewAccount(holder, accountType)
	s.mu.Lock()
	s.accounts[id] = acc
	s.mu.Unlock()
	return id, acc, nil
}

func (s *Store) Get(id string) (*domain.Account, error) {
	s.mu.RLock()
	acc, ok := s.accounts[id]
	s.mu.RUnlock()
	if !ok {
		return nil, apperrors.ErrAccountNotFound
	}
	return acc, nil
}

func (s *Store) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.accounts[id]; !ok {
		return apperrors.ErrAccountNotFound
	}
	delete(s.accounts, id)
	return nil
}

func (s *Store) List() []AccountSummary {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]AccountSummary, 0, len(s.accounts))
	for id, acc := range s.accounts {
		snap := acc.Snapshot()
		out = append(out, AccountSummary{
			ID:            id,
			AccountNumber: snap.AccountNumber,
			AccountType:   snap.AccountType,
			Currency:      snap.Currency,
			Balance:       snap.Balance,
			Open:          snap.Open,
			Holder:        snap.Holder,
			CreatedAt:     snap.CreatedAt,
		})
	}
	return out
}
