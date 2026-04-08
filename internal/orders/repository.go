package orders

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
)

// Repository provides data-access for the orders domain.
type Repository struct {
	db *sql.DB
}

// NewRepository creates a new orders Repository.
func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

// DB returns the underlying *sql.DB for transaction creation.
func (r *Repository) DB() *sql.DB { return r.db }

// CreateOrderTx inserts a new order row within the given transaction.
func (r *Repository) CreateOrderTx(ctx context.Context, tx *sql.Tx, order NewOrder) (*Order, error) {
	addrJSON, err := json.Marshal(order.DeliveryAddressSnapshot)
	if err != nil {
		return nil, fmt.Errorf("orders: marshal address snapshot: %w", err)
	}

	const q = `
		INSERT INTO orders (
			user_id, delivery_address_snapshot, subtotal_cop,
			delivery_fee_cop, total_cop, payment_method, notes
		) VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, user_id, status, delivery_address_snapshot,
			subtotal_cop, delivery_fee_cop, total_cop, payment_method, notes,
			created_at, updated_at
	`
	row := tx.QueryRowContext(ctx, q,
		order.UserID,
		addrJSON,
		order.SubtotalCOP,
		order.DeliveryFeeCOP,
		order.TotalCOP,
		order.PaymentMethod,
		order.Notes,
	)
	return scanOrder(row)
}

// CreateOrderItemsTx inserts multiple order_items rows within a transaction.
func (r *Repository) CreateOrderItemsTx(ctx context.Context, tx *sql.Tx, orderID int64, items []NewOrderItem) error {
	const q = `
		INSERT INTO order_items (
			order_id, listing_id, listing_name_snapshot,
			listing_store_snapshot, price_snapshot_cop, quantity
		) VALUES ($1, $2, $3, $4, $5, $6)
	`
	for _, item := range items {
		_, err := tx.ExecContext(ctx, q,
			orderID, item.ListingID, item.ListingNameSnapshot,
			item.ListingStoreSnapshot, item.PriceSnapshotCOP, item.Quantity,
		)
		if err != nil {
			return fmt.Errorf("orders: insert order item: %w", err)
		}
	}
	return nil
}

// CreateOrderEventTx inserts an order_event within a transaction.
// to_status is the new status (pending_confirmation for order creation).
func (r *Repository) CreateOrderEventTx(ctx context.Context, tx *sql.Tx, orderID int64, toStatus string, note string) error {
	const q = `
		INSERT INTO order_events (order_id, to_status, note)
		VALUES ($1, $2, $3)
	`
	_, err := tx.ExecContext(ctx, q, orderID, toStatus, note)
	if err != nil {
		return fmt.Errorf("orders: create order event: %w", err)
	}
	return nil
}

// GetOrder returns an order with its items, verifying ownership by userID.
// Returns an error wrapping sql.ErrNoRows if not found, or a 403 error if wrong user.
func (r *Repository) GetOrder(ctx context.Context, id, userID int64) (*Order, []OrderItem, error) {
	const q = `
		SELECT id, user_id, status, delivery_address_snapshot,
			subtotal_cop, delivery_fee_cop, total_cop, payment_method, notes,
			created_at, updated_at
		FROM orders
		WHERE id = $1
	`
	row := r.db.QueryRowContext(ctx, q, id)
	order, err := scanOrder(row)
	if err != nil {
		return nil, nil, err
	}
	if order.UserID != userID {
		return nil, nil, ErrForbidden
	}

	items, err := r.listOrderItems(ctx, id)
	if err != nil {
		return nil, nil, err
	}
	return order, items, nil
}

// ListOrders returns all orders for a user, newest first.
func (r *Repository) ListOrders(ctx context.Context, userID int64) ([]Order, error) {
	const q = `
		SELECT id, user_id, status, delivery_address_snapshot,
			subtotal_cop, delivery_fee_cop, total_cop, payment_method, notes,
			created_at, updated_at
		FROM orders
		WHERE user_id = $1
		ORDER BY created_at DESC
	`
	rows, err := r.db.QueryContext(ctx, q, userID)
	if err != nil {
		return nil, fmt.Errorf("orders: list orders: %w", err)
	}
	defer rows.Close()

	var orders []Order
	for rows.Next() {
		var o Order
		var addrJSON []byte
		var notes sql.NullString
		err := rows.Scan(
			&o.ID, &o.UserID, &o.Status, &addrJSON,
			&o.SubtotalCOP, &o.DeliveryFeeCOP, &o.TotalCOP, &o.PaymentMethod, &notes,
			&o.CreatedAt, &o.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("orders: scan order: %w", err)
		}
		if err := json.Unmarshal(addrJSON, &o.DeliveryAddressSnapshot); err != nil {
			return nil, fmt.Errorf("orders: unmarshal address snapshot: %w", err)
		}
		if notes.Valid {
			o.Notes = &notes.String
		}
		orders = append(orders, o)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("orders: list orders rows: %w", err)
	}
	return orders, nil
}

func (r *Repository) listOrderItems(ctx context.Context, orderID int64) ([]OrderItem, error) {
	const q = `
		SELECT id, order_id, listing_id, listing_name_snapshot, listing_store_snapshot,
			price_snapshot_cop, quantity, is_cancelled, created_at
		FROM order_items
		WHERE order_id = $1
		ORDER BY id ASC
	`
	rows, err := r.db.QueryContext(ctx, q, orderID)
	if err != nil {
		return nil, fmt.Errorf("orders: list items: %w", err)
	}
	defer rows.Close()

	var items []OrderItem
	for rows.Next() {
		var item OrderItem
		var listingID sql.NullInt64
		err := rows.Scan(
			&item.ID, &item.OrderID, &listingID, &item.ListingNameSnapshot, &item.ListingStoreSnapshot,
			&item.PriceSnapshotCOP, &item.Quantity, &item.IsCancelled, &item.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("orders: scan item: %w", err)
		}
		if listingID.Valid {
			item.ListingID = &listingID.Int64
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("orders: items rows: %w", err)
	}
	return items, nil
}

// GetStoreLatlng returns the lat/lng for a store, used for delivery fee calculation.
func (r *Repository) GetStoreLatLng(ctx context.Context, storeID int64) (lat, lng *float64, err error) {
	const q = `SELECT lat, lng FROM stores WHERE id = $1`
	var latN, lngN sql.NullFloat64
	if err := r.db.QueryRowContext(ctx, q, storeID).Scan(&latN, &lngN); err != nil {
		return nil, nil, fmt.Errorf("orders: get store lat/lng: %w", err)
	}
	if latN.Valid {
		lat = &latN.Float64
	}
	if lngN.Valid {
		lng = &lngN.Float64
	}
	return lat, lng, nil
}

// ─── scan helpers ─────────────────────────────────────────────────────────────

func scanOrder(row *sql.Row) (*Order, error) {
	var o Order
	var addrJSON []byte
	var notes sql.NullString
	err := row.Scan(
		&o.ID, &o.UserID, &o.Status, &addrJSON,
		&o.SubtotalCOP, &o.DeliveryFeeCOP, &o.TotalCOP, &o.PaymentMethod, &notes,
		&o.CreatedAt, &o.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("orders: %w", sql.ErrNoRows)
		}
		return nil, fmt.Errorf("orders: scan order: %w", err)
	}
	if err := json.Unmarshal(addrJSON, &o.DeliveryAddressSnapshot); err != nil {
		return nil, fmt.Errorf("orders: unmarshal address snapshot: %w", err)
	}
	if notes.Valid {
		o.Notes = &notes.String
	}
	return &o, nil
}
