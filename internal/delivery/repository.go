package delivery

import (
	"context"
	"database/sql"
	"fmt"
)

// Repository provides data-access for delivery configuration and couriers.
type Repository struct {
	db *sql.DB
}

// NewRepository creates a new delivery Repository.
func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

// LoadBrackets returns all delivery fee brackets ordered by distance_km_min.
func (r *Repository) LoadBrackets(ctx context.Context) ([]FeeBracket, error) {
	const q = `
		SELECT id, distance_km_min, distance_km_max, fee_cop
		FROM delivery_fee_brackets
		ORDER BY distance_km_min ASC
	`
	rows, err := r.db.QueryContext(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("delivery: load brackets: %w", err)
	}
	defer rows.Close()

	var brackets []FeeBracket
	for rows.Next() {
		var b FeeBracket
		var maxKm sql.NullFloat64
		if err := rows.Scan(&b.ID, &b.DistanceKmMin, &maxKm, &b.FeeCOP); err != nil {
			return nil, fmt.Errorf("delivery: scan bracket: %w", err)
		}
		if maxKm.Valid {
			b.DistanceKmMax = &maxKm.Float64
		}
		brackets = append(brackets, b)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("delivery: brackets rows: %w", err)
	}
	return brackets, nil
}

// LoadConfig returns the singleton delivery configuration row.
func (r *Repository) LoadConfig(ctx context.Context) (*DeliveryConfig, error) {
	const q = `
		SELECT multi_store_discount_pct, updated_at
		FROM delivery_config
		WHERE id = 1
	`
	var cfg DeliveryConfig
	err := r.db.QueryRowContext(ctx, q).Scan(&cfg.MultiStoreDiscountPct, &cfg.UpdatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			// Return sensible defaults if not configured
			return &DeliveryConfig{MultiStoreDiscountPct: 30}, nil
		}
		return nil, fmt.Errorf("delivery: load config: %w", err)
	}
	return &cfg, nil
}

// ListCouriers returns all couriers, active ones first.
func (r *Repository) ListCouriers(ctx context.Context) ([]Courier, error) {
	const q = `
		SELECT id, name, phone, is_active, created_at
		FROM couriers
		ORDER BY is_active DESC, name ASC
	`
	rows, err := r.db.QueryContext(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("delivery: list couriers: %w", err)
	}
	defer rows.Close()

	var couriers []Courier
	for rows.Next() {
		var c Courier
		if err := rows.Scan(&c.ID, &c.Name, &c.Phone, &c.IsActive, &c.CreatedAt); err != nil {
			return nil, fmt.Errorf("delivery: scan courier: %w", err)
		}
		couriers = append(couriers, c)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("delivery: couriers rows: %w", err)
	}
	return couriers, nil
}

// GetCourierByID returns a courier by primary key.
func (r *Repository) GetCourierByID(ctx context.Context, id int64) (*Courier, error) {
	const q = `
		SELECT id, name, phone, is_active, created_at
		FROM couriers
		WHERE id = $1
	`
	var c Courier
	err := r.db.QueryRowContext(ctx, q, id).Scan(
		&c.ID, &c.Name, &c.Phone, &c.IsActive, &c.CreatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("delivery: courier %d: %w", id, sql.ErrNoRows)
		}
		return nil, fmt.Errorf("delivery: get courier: %w", err)
	}
	return &c, nil
}
