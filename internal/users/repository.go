package users

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// Repository provides data-access operations for the users domain.
type Repository struct {
	db *sql.DB
}

// NewRepository creates a new Repository.
func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

// CreateUser inserts a new user and returns the created row.
// consentAt should be non-nil when the user has given HABEAS DATA consent.
func (r *Repository) CreateUser(ctx context.Context, email, phone, name, passwordHash string, consentAt *time.Time) (*User, error) {
	const q = `
		INSERT INTO users (email, phone, name, password_hash, habeas_data_consent_at)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, email, phone, name, password_hash, is_active, created_at, updated_at,
		          habeas_data_consent_at, deleted_at
	`
	var phoneVal interface{}
	if phone != "" {
		phoneVal = phone
	}
	return scanUser(r.db.QueryRowContext(ctx, q, email, phoneVal, name, passwordHash, consentAt))
}

// GetUserByEmail fetches a user by email address.
func (r *Repository) GetUserByEmail(ctx context.Context, email string) (*User, error) {
	const q = `
		SELECT id, email, phone, name, password_hash, is_active, created_at, updated_at,
		       habeas_data_consent_at, deleted_at
		FROM users
		WHERE email = $1
	`
	return scanUser(r.db.QueryRowContext(ctx, q, email))
}

// GetUserByID fetches a user by primary key.
func (r *Repository) GetUserByID(ctx context.Context, id int64) (*User, error) {
	const q = `
		SELECT id, email, phone, name, password_hash, is_active, created_at, updated_at,
		       habeas_data_consent_at, deleted_at
		FROM users
		WHERE id = $1
	`
	return scanUser(r.db.QueryRowContext(ctx, q, id))
}

// CreateAddress inserts a new address for the given user.
func (r *Repository) CreateAddress(ctx context.Context, userID int64, addr AddressInput) (*Address, error) {
	const q = `
		INSERT INTO addresses (user_id, label, full_address, reference, lat, lng)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, user_id, label, full_address, reference, lat, lng, created_at
	`
	return scanAddress(r.db.QueryRowContext(ctx, q,
		userID, addr.Label, addr.FullAddress, addr.Reference, addr.Lat, addr.Lng))
}

// ListAddresses returns all addresses belonging to a user.
func (r *Repository) ListAddresses(ctx context.Context, userID int64) ([]Address, error) {
	const q = `
		SELECT id, user_id, label, full_address, reference, lat, lng, created_at
		FROM addresses
		WHERE user_id = $1
		ORDER BY created_at DESC
	`
	rows, err := r.db.QueryContext(ctx, q, userID)
	if err != nil {
		return nil, fmt.Errorf("users: list addresses: %w", err)
	}
	defer rows.Close()

	var addresses []Address
	for rows.Next() {
		addr, err := scanAddressRow(rows)
		if err != nil {
			return nil, err
		}
		addresses = append(addresses, *addr)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("users: list addresses rows: %w", err)
	}
	return addresses, nil
}

// GetAddress fetches a single address, verifying it belongs to userID.
func (r *Repository) GetAddress(ctx context.Context, id, userID int64) (*Address, error) {
	const q = `
		SELECT id, user_id, label, full_address, reference, lat, lng, created_at
		FROM addresses
		WHERE id = $1 AND user_id = $2
	`
	return scanAddress(r.db.QueryRowContext(ctx, q, id, userID))
}

// DeleteAddress removes an address by id, verifying ownership.
func (r *Repository) DeleteAddress(ctx context.Context, id, userID int64) error {
	const q = `DELETE FROM addresses WHERE id = $1 AND user_id = $2`
	res, err := r.db.ExecContext(ctx, q, id, userID)
	if err != nil {
		return fmt.Errorf("users: delete address: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("users: address %d: %w", id, sql.ErrNoRows)
	}
	return nil
}

// ─── scan helpers ─────────────────────────────────────────────────────────────

func scanUser(row *sql.Row) (*User, error) {
	var u User
	var phone sql.NullString
	var consentAt, deletedAt sql.NullTime
	err := row.Scan(
		&u.ID, &u.Email, &phone, &u.Name, &u.PasswordHash,
		&u.IsActive, &u.CreatedAt, &u.UpdatedAt,
		&consentAt, &deletedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("users: %w", sql.ErrNoRows)
		}
		return nil, fmt.Errorf("users: scan user: %w", err)
	}
	if phone.Valid {
		u.Phone = &phone.String
	}
	if consentAt.Valid {
		t := consentAt.Time
		u.HabeasDataConsentAt = &t
	}
	if deletedAt.Valid {
		t := deletedAt.Time
		u.DeletedAt = &t
	}
	return &u, nil
}

// AnonymizeUser replaces PII fields with anonymized values and sets deleted_at.
func (r *Repository) AnonymizeUser(ctx context.Context, userID int64) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE users
		SET email        = 'deleted_' || id || '@protou.invalid',
		    name         = 'Usuario eliminado',
		    phone        = NULL,
		    password_hash = '',
		    is_active    = false,
		    deleted_at   = NOW()
		WHERE id = $1
	`, userID)
	if err != nil {
		return fmt.Errorf("users: anonymize user %d: %w", userID, err)
	}
	return nil
}


func scanAddress(row *sql.Row) (*Address, error) {
	var a Address
	var label, reference sql.NullString
	var lat, lng sql.NullFloat64
	err := row.Scan(
		&a.ID, &a.UserID, &label, &a.FullAddress, &reference, &lat, &lng, &a.CreatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("users: address: %w", sql.ErrNoRows)
		}
		return nil, fmt.Errorf("users: scan address: %w", err)
	}
	if label.Valid {
		a.Label = &label.String
	}
	if reference.Valid {
		a.Reference = &reference.String
	}
	if lat.Valid {
		a.Lat = &lat.Float64
	}
	if lng.Valid {
		a.Lng = &lng.Float64
	}
	return &a, nil
}

func scanAddressRow(row *sql.Rows) (*Address, error) {
	var a Address
	var label, reference sql.NullString
	var lat, lng sql.NullFloat64
	err := row.Scan(
		&a.ID, &a.UserID, &label, &a.FullAddress, &reference, &lat, &lng, &a.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("users: scan address row: %w", err)
	}
	if label.Valid {
		a.Label = &label.String
	}
	if reference.Valid {
		a.Reference = &reference.String
	}
	if lat.Valid {
		a.Lat = &lat.Float64
	}
	if lng.Valid {
		a.Lng = &lng.Float64
	}
	return &a, nil
}
