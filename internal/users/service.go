package users

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/protou/protou/internal/auth"
	"golang.org/x/crypto/bcrypt"
)

// ErrDuplicateEmail is returned when a registration email already exists.
var ErrDuplicateEmail = errors.New("email already registered")

// ErrInvalidCredentials is returned when login credentials are wrong.
var ErrInvalidCredentials = errors.New("credenciales inválidas")

// ErrNotFound is returned when a user or address is not found.
var ErrNotFound = errors.New("not found")

// Service implements business logic for users and auth.
type Service struct {
	repo *Repository
}

// NewService creates a new Service.
func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

// Register creates a new student account and returns the user + JWT.
func (s *Service) Register(ctx context.Context, req RegisterRequest) (*User, string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), 12)
	if err != nil {
		return nil, "", fmt.Errorf("users: hash password: %w", err)
	}

	var consentAt *time.Time
	if req.HabeasDataConsent {
		now := time.Now().UTC()
		consentAt = &now
	}

	user, err := s.repo.CreateUser(ctx, req.Email, req.Phone, req.FullName, string(hash), consentAt)
	if err != nil {
		// lib/pq unique violation error code 23505
		if isDuplicateKeyErr(err) {
			return nil, "", ErrDuplicateEmail
		}
		return nil, "", fmt.Errorf("users: register: %w", err)
	}

	token, err := auth.IssueToken(user.ID, "student")
	if err != nil {
		return nil, "", fmt.Errorf("users: issue token: %w", err)
	}
	return user, token, nil
}

// DeleteAccount anonymizes PII for the given user (HABEAS DATA right to erasure).
func (s *Service) DeleteAccount(ctx context.Context, userID int64) error {
	if err := s.repo.AnonymizeUser(ctx, userID); err != nil {
		return fmt.Errorf("users: delete account: %w", err)
	}
	return nil
}

// Login verifies credentials and returns the user + JWT.
func (s *Service) Login(ctx context.Context, email, password string) (*User, string, error) {
	user, err := s.repo.GetUserByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, "", ErrInvalidCredentials
		}
		return nil, "", fmt.Errorf("users: login lookup: %w", err)
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return nil, "", ErrInvalidCredentials
	}

	token, err := auth.IssueToken(user.ID, "student")
	if err != nil {
		return nil, "", fmt.Errorf("users: issue token: %w", err)
	}
	return user, token, nil
}

// AddAddress creates an address for the given user (geocoding skipped for now).
func (s *Service) AddAddress(ctx context.Context, userID int64, input AddressInput) (*Address, error) {
	addr, err := s.repo.CreateAddress(ctx, userID, input)
	if err != nil {
		return nil, fmt.Errorf("users: add address: %w", err)
	}
	return addr, nil
}

// GetUserByID returns a user by primary key.
func (s *Service) GetUserByID(ctx context.Context, id int64) (*User, error) {
	user, err := s.repo.GetUserByID(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("users: get by id: %w", err)
	}
	return user, nil
}

// ListAddresses returns all addresses for a user.
func (s *Service) ListAddresses(ctx context.Context, userID int64) ([]Address, error) {
	addrs, err := s.repo.ListAddresses(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("users: list addresses: %w", err)
	}
	if addrs == nil {
		addrs = []Address{}
	}
	return addrs, nil
}

// DeleteAddress removes an address from a user.
func (s *Service) DeleteAddress(ctx context.Context, id, userID int64) error {
	err := s.repo.DeleteAddress(ctx, id, userID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		}
		return fmt.Errorf("users: delete address: %w", err)
	}
	return nil
}

// isDuplicateKeyErr checks for PostgreSQL unique_violation (23505).
func isDuplicateKeyErr(err error) bool {
	if err == nil {
		return false
	}
	return contains(err.Error(), "23505") || contains(err.Error(), "duplicate key")
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(sub) == 0 ||
		func() bool {
			for i := 0; i <= len(s)-len(sub); i++ {
				if s[i:i+len(sub)] == sub {
					return true
				}
			}
			return false
		}())
}
