package cart

import (
	"context"
	"database/sql"
	"fmt"
)

// Repository provides data-access operations for the cart domain.
type Repository struct {
	db *sql.DB
}

// NewRepository creates a new cart Repository.
func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

// GetOrCreateCart returns the existing cart for a user or inserts a new one.
func (r *Repository) GetOrCreateCart(ctx context.Context, userID int64) (*Cart, error) {
	const upsert = `
		INSERT INTO carts (user_id)
		VALUES ($1)
		ON CONFLICT (user_id) DO UPDATE SET updated_at = NOW()
		RETURNING id, user_id, created_at, updated_at
	`
	var c Cart
	err := r.db.QueryRowContext(ctx, upsert, userID).Scan(
		&c.ID, &c.UserID, &c.CreatedAt, &c.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("cart: get or create cart: %w", err)
	}
	return &c, nil
}

// UpsertItem adds or updates a cart item. qty=0 removes the item.
func (r *Repository) UpsertItem(ctx context.Context, cartID int64, listingID int64, qty int) error {
	if qty == 0 {
		const del = `DELETE FROM cart_items WHERE cart_id = $1 AND listing_id = $2`
		_, err := r.db.ExecContext(ctx, del, cartID, listingID)
		if err != nil {
			return fmt.Errorf("cart: remove item: %w", err)
		}
		return nil
	}

	const upsert = `
		INSERT INTO cart_items (cart_id, listing_id, quantity)
		VALUES ($1, $2, $3)
		ON CONFLICT (cart_id, listing_id) DO UPDATE
			SET quantity = $3, updated_at = NOW()
	`
	_, err := r.db.ExecContext(ctx, upsert, cartID, listingID, qty)
	if err != nil {
		return fmt.Errorf("cart: upsert item: %w", err)
	}
	// Also bump cart.updated_at
	_, _ = r.db.ExecContext(ctx, `UPDATE carts SET updated_at = NOW() WHERE id = $1`, cartID)
	return nil
}

// GetCartWithItems returns the cart and all its items for a user.
func (r *Repository) GetCartWithItems(ctx context.Context, userID int64) (*Cart, []CartItem, error) {
	cart, err := r.GetOrCreateCart(ctx, userID)
	if err != nil {
		return nil, nil, err
	}

	const q = `
		SELECT id, cart_id, listing_id, quantity, added_at, updated_at
		FROM cart_items
		WHERE cart_id = $1
		ORDER BY added_at ASC
	`
	rows, err := r.db.QueryContext(ctx, q, cart.ID)
	if err != nil {
		return nil, nil, fmt.Errorf("cart: get items: %w", err)
	}
	defer rows.Close()

	var items []CartItem
	for rows.Next() {
		var item CartItem
		if err := rows.Scan(
			&item.ID, &item.CartID, &item.ListingID, &item.Quantity, &item.AddedAt, &item.UpdatedAt,
		); err != nil {
			return nil, nil, fmt.Errorf("cart: scan item: %w", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, nil, fmt.Errorf("cart: items rows: %w", err)
	}
	return cart, items, nil
}

// RemoveItem deletes a single item from a cart by listing ID.
// It is idempotent: if the item does not exist the call succeeds silently.
func (r *Repository) RemoveItem(ctx context.Context, cartID int64, listingID int64) error {
	const q = `DELETE FROM cart_items WHERE cart_id = $1 AND listing_id = $2`
	_, err := r.db.ExecContext(ctx, q, cartID, listingID)
	if err != nil {
		return fmt.Errorf("cart: remove item: %w", err)
	}
	return nil
}

// ClearCart removes all items from a cart.
func (r *Repository) ClearCart(ctx context.Context, cartID int64) error {
	const q = `DELETE FROM cart_items WHERE cart_id = $1`
	_, err := r.db.ExecContext(ctx, q, cartID)
	if err != nil {
		return fmt.Errorf("cart: clear cart: %w", err)
	}
	return nil
}

// ClearCartTx removes all items within a transaction.
func (r *Repository) ClearCartTx(ctx context.Context, tx *sql.Tx, cartID int64) error {
	const q = `DELETE FROM cart_items WHERE cart_id = $1`
	_, err := tx.ExecContext(ctx, q, cartID)
	if err != nil {
		return fmt.Errorf("cart: clear cart tx: %w", err)
	}
	return nil
}
