package service

import (
	"fmt"
	"log"
	"math"
	"net/mail"
	"strings"
	"time"

	"bank-account/domain"
	"bank-account/dto"
	apperrors "bank-account/errors"
	"bank-account/store"
)

const (
	maxHolderNameLen = 200
	minPhoneDigits   = 10
	maxPhoneDigits   = 15
	minHolderAge     = 18
	dobLayout        = "2006-01-02"
)

var supportedAccountTypes = map[string]string{
	"savings": "savings",
	"current": "current",
}

type Service struct {
	store *store.Store
}

func NewService(s *store.Store) *Service {
	return &Service{store: s}
}

func (s *Service) CreateAccount(req dto.CreateAccountRequest) (dto.AccountInfo, error) {
	holder, err := validateHolder(req.Holder)
	if err != nil {
		return dto.AccountInfo{}, err
	}
	accountType, err := validateAccountType(req.AccountType)
	if err != nil {
		return dto.AccountInfo{}, err
	}
	id, acc, err := s.store.Create(holder, accountType)
	if err != nil {
		return dto.AccountInfo{}, err
	}
	s.persistAfterMutation()
	return accountInfoFrom(id, acc), nil
}

func (s *Service) GetAccount(req dto.GetAccountRequest) (dto.AccountInfo, error) {
	acc, err := s.store.Get(req.AccountID)
	if err != nil {
		return dto.AccountInfo{}, err
	}
	return accountInfoFrom(req.AccountID, acc), nil
}

func (s *Service) persistAfterMutation() {
	if err := s.store.Snapshot(); err != nil {
		log.Printf("persistence: snapshot failed: %v", err)
	}
}

func accountInfoFrom(id string, acc *domain.Account) dto.AccountInfo {
	snap := acc.Snapshot()
	return dto.AccountInfo{
		AccountID:     id,
		AccountNumber: snap.AccountNumber,
		AccountType:   snap.AccountType,
		Currency:      snap.Currency,
		Balance:       snap.Balance,
		Open:          snap.Open,
		Status:        statusOf(snap.Open),
		Holder:        snap.Holder,
		CreatedAt:     snap.CreatedAt,
	}
}

func validateHolder(in dto.HolderInput) (domain.Holder, error) {
	name := strings.TrimSpace(in.Name)
	email := strings.TrimSpace(in.Email)

	if name == "" || len(name) > maxHolderNameLen {
		return domain.Holder{}, apperrors.ErrInvalidHolder
	}
	addr, err := mail.ParseAddress(email)
	if err != nil || addr.Address != email {
		return domain.Holder{}, apperrors.ErrInvalidHolder
	}
	phone, err := normalisePhone(in.Phone)
	if err != nil {
		return domain.Holder{}, err
	}
	dob, err := normaliseDOB(in.DateOfBirth)
	if err != nil {
		return domain.Holder{}, err
	}
	return domain.Holder{
		Name:        name,
		Email:       email,
		Phone:       phone,
		DateOfBirth: dob,
	}, nil
}

func normalisePhone(in string) (string, error) {
	s := strings.TrimSpace(in)
	if s == "" {
		return "", apperrors.ErrInvalidHolder
	}
	digits := 0
	for _, r := range s {
		switch {
		case r >= '0' && r <= '9':
			digits++
		case r == ' ', r == '-', r == '+', r == '(', r == ')':
		default:
			return "", apperrors.ErrInvalidHolder
		}
	}
	if digits < minPhoneDigits || digits > maxPhoneDigits {
		return "", apperrors.ErrInvalidHolder
	}
	return s, nil
}

func normaliseDOB(in string) (string, error) {
	s := strings.TrimSpace(in)
	if s == "" {
		return "", apperrors.ErrInvalidHolder
	}
	t, err := time.Parse(dobLayout, s)
	if err != nil {
		return "", apperrors.ErrInvalidHolder
	}
	now := time.Now().UTC()
	if !t.Before(now) {
		return "", apperrors.ErrInvalidHolder
	}
	if yearsBetween(t, now) < minHolderAge {
		return "", apperrors.ErrInvalidHolder
	}
	return t.Format(dobLayout), nil
}

func yearsBetween(birth, now time.Time) int {
	years := now.Year() - birth.Year()
	if now.YearDay() < birth.YearDay() {
		years--
	}
	return years
}

func validateAccountType(in string) (string, error) {
	t := strings.ToLower(strings.TrimSpace(in))
	if t == "" {
		return "savings", nil
	}
	canonical, ok := supportedAccountTypes[t]
	if !ok {
		return "", apperrors.ErrInvalidAccountType
	}
	return canonical, nil
}

func toCents(amount float64) (int64, error) {
	if math.IsNaN(amount) || math.IsInf(amount, 0) {
		return 0, apperrors.ErrInvalidAmount
	}
	if amount <= 0 {
		return 0, apperrors.ErrInvalidAmount
	}
	cents := int64(math.Round(amount * 100))
	if cents <= 0 {
		return 0, apperrors.ErrInvalidAmount
	}
	return cents, nil
}

func formatBalance(cents int64) string {
	sign := ""
	if cents < 0 {
		sign = "-"
		cents = -cents
	}
	return fmt.Sprintf("%s%d.%02d", sign, cents/100, cents%100)
}

func statusOf(open bool) string {
	if open {
		return "open"
	}
	return "closed"
}
