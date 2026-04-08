package scraping

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

// UpsertListing upserts a raw listing into the listings table.
// It returns (isNew, isUpdated, error).
//
// Logic:
//  1. Parse price and stock from raw strings.
//  2. Derive external_sku from raw.SKU or hash(storeID, productURL).
//  3. INSERT ON CONFLICT (store_id, external_sku) DO UPDATE when any tracked field changed.
//  4. If price_cop or stock_signal changed, insert a price_history row.
func UpsertListing(
	ctx context.Context,
	db *sql.DB,
	storeID int64,
	raw RawListing,
	categoryID *int64,
) (isNew bool, isUpdated bool, err error) {
	// 1. Parse price
	var priceCOP *int
	if p, ok := ParsePrice(raw.PriceRaw); ok {
		priceCOP = &p
	}

	// 2. Parse stock signal
	signal := ParseStockSignal(raw.PriceRaw, raw.StockRaw)
	if priceCOP == nil {
		signal = StockPriceOnRequest
	}

	// 3. Derive SKU
	sku := strings.TrimSpace(raw.SKU)
	if sku == "" {
		sku = GenerateSKU(storeID, raw.ProductURL)
	}

	// 4. Two-step approach: SELECT then INSERT or UPDATE.
	// Check existing row first.
	type existingRow struct {
		id          int64
		priceCOP    *int
		stockSignal string
	}

	const selectQ = `
		SELECT id, price_cop, stock_signal
		FROM listings
		WHERE store_id = $1 AND external_sku = $2
	`
	var existing existingRow
	var sqlPrice sql.NullInt64
	scanErr := db.QueryRowContext(ctx, selectQ, storeID, sku).Scan(
		&existing.id, &sqlPrice, &existing.stockSignal,
	)

	if scanErr != nil && scanErr != sql.ErrNoRows {
		return false, false, fmt.Errorf("checking existing listing: %w", scanErr)
	}

	rowExists := scanErr == nil

	if !rowExists {
		// INSERT new listing
		const insertQ = `
			INSERT INTO listings (
				store_id, external_sku, name, price_cop,
				image_url, product_url, stock_signal, category_id,
				last_scraped_at, is_active, created_at, updated_at
			) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, NOW(), true, NOW(), NOW())
			RETURNING id
		`
		var newID int64
		if err := db.QueryRowContext(ctx, insertQ,
			storeID, sku, raw.Name, priceCOP,
			nullString(raw.ImageURL), raw.ProductURL, string(signal), categoryID,
		).Scan(&newID); err != nil {
			return false, false, fmt.Errorf("inserting listing: %w", err)
		}

		// Insert initial price_history
		if insertErr := insertPriceHistory(ctx, db, newID, priceCOP, signal); insertErr != nil {
			return true, false, fmt.Errorf("inserting initial price history: %w", insertErr)
		}
		return true, false, nil
	}

	// Row exists — check if anything changed
	existingPrice := ptrInt64(sqlPrice)
	priceChanged := !intPtrsEqual(existingPrice, priceCOP)
	stockChanged := existing.stockSignal != string(signal)

	if !priceChanged && !stockChanged {
		// No change to price/stock — still update name/image/url but don't track history
		const touchQ = `
			UPDATE listings
			SET name = $1, image_url = $2, product_url = $3, last_scraped_at = NOW()
			WHERE id = $4
		`
		if _, err := db.ExecContext(ctx, touchQ,
			raw.Name, nullString(raw.ImageURL), raw.ProductURL, existing.id,
		); err != nil {
			return false, false, fmt.Errorf("touching listing %d: %w", existing.id, err)
		}
		return false, false, nil
	}

	// Price or stock changed — update and record history
	const updateQ = `
		UPDATE listings
		SET name = $1, price_cop = $2, image_url = $3, product_url = $4,
		    stock_signal = $5, last_scraped_at = NOW(), updated_at = NOW()
		WHERE id = $6
	`
	if _, err := db.ExecContext(ctx, updateQ,
		raw.Name, priceCOP, nullString(raw.ImageURL), raw.ProductURL,
		string(signal), existing.id,
	); err != nil {
		return false, false, fmt.Errorf("updating listing %d: %w", existing.id, err)
	}

	if insertErr := insertPriceHistory(ctx, db, existing.id, priceCOP, signal); insertErr != nil {
		return false, true, fmt.Errorf("inserting price history for listing %d: %w", existing.id, insertErr)
	}

	return false, true, nil
}

// insertPriceHistory inserts one price_history row for the given listing.
func insertPriceHistory(ctx context.Context, db *sql.DB, listingID int64, priceCOP *int, signal StockSignal) error {
	const q = `
		INSERT INTO price_history (listing_id, price_cop, stock_signal, scraped_at)
		VALUES ($1, $2, $3, NOW())
	`
	if _, err := db.ExecContext(ctx, q, listingID, priceCOP, string(signal)); err != nil {
		return fmt.Errorf("inserting price_history for listing %d: %w", listingID, err)
	}
	return nil
}

// nullString returns a *string or nil for use in SQL nullable string parameters.
func nullString(s string) interface{} {
	if s == "" {
		return nil
	}
	return s
}

// ptrInt64 converts a sql.NullInt64 to *int.
func ptrInt64(n sql.NullInt64) *int {
	if !n.Valid {
		return nil
	}
	v := int(n.Int64)
	return &v
}

// intPtrsEqual compares two *int values for equality (nil-safe).
func intPtrsEqual(a, b *int) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return *a == *b
}
