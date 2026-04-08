package users

import "time"

// User represents a row from the users table.
type User struct {
	ID                  int64      `json:"id"`
	Email               string     `json:"email"`
	Phone               *string    `json:"phone"`
	Name                string     `json:"name"`
	PasswordHash        string     `json:"-"`
	IsActive            bool       `json:"is_active"`
	CreatedAt           time.Time  `json:"created_at"`
	UpdatedAt           time.Time  `json:"updated_at"`
	HabeasDataConsentAt *time.Time `json:"habeas_data_consent_at,omitempty"`
	DeletedAt           *time.Time `json:"-"`
}

// Address represents a row from the addresses table.
type Address struct {
	ID          int64     `json:"id"`
	UserID      int64     `json:"user_id"`
	Label       *string   `json:"label"`
	FullAddress string    `json:"full_address"`
	Reference   *string   `json:"reference"`
	Lat         *float64  `json:"lat"`
	Lng         *float64  `json:"lng"`
	CreatedAt   time.Time `json:"created_at"`
}

// AddressInput holds the fields needed to create an address.
type AddressInput struct {
	Label       *string  `json:"label"`
	FullAddress string   `json:"full_address"`
	Reference   *string  `json:"reference"`
	Lat         *float64 `json:"lat"`
	Lng         *float64 `json:"lng"`
}

// RegisterRequest holds the payload for POST /v1/auth/register.
type RegisterRequest struct {
	Email             string `json:"email"`
	Phone             string `json:"phone"`
	FullName          string `json:"full_name"`
	Password          string `json:"password"`
	HabeasDataConsent bool   `json:"habeas_data_consent"`
}
