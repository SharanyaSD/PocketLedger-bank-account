package store

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"sync"

	"bank-account/domain"
)

const snapshotVersion = 1

type Snapshot struct {
	Version  int                            `json:"version"`
	Accounts map[string]domain.AccountState `json:"accounts"`
}

type Persister struct {
	path string
	mu   sync.Mutex
}

func NewPersister(path string) *Persister {
	return &Persister{path: path}
}

func (p *Persister) Path() string { return p.path }

func (p *Persister) Save(s Snapshot) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	dir := filepath.Dir(p.path)
	if dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	tmp := p.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, p.path)
}

func (p *Persister) Load() (Snapshot, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	data, err := os.ReadFile(p.path)
	if errors.Is(err, fs.ErrNotExist) {
		return Snapshot{Version: snapshotVersion, Accounts: map[string]domain.AccountState{}}, nil
	}
	if err != nil {
		return Snapshot{}, err
	}
	var s Snapshot
	if err := json.Unmarshal(data, &s); err != nil {
		return Snapshot{}, err
	}
	if s.Accounts == nil {
		s.Accounts = map[string]domain.AccountState{}
	}
	return s, nil
}
