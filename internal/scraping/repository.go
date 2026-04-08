package scraping

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// Repository handles all DB queries for the scraping domain.
type Repository struct{}

// LoadActiveStoresWithRules returns all active stores and their scrape rules.
func (r *Repository) LoadActiveStoresWithRules(ctx context.Context, db *sql.DB) ([]StoreWithRule, error) {
	const q = `
		SELECT
			s.id, s.name, s.base_url,
			COALESCE(s.lat, 0), COALESCE(s.lng, 0),
			sr.id, sr.store_id,
			sr.catalog_url_pattern,
			sr.item_selector, sr.price_selector, sr.name_selector,
			sr.image_selector, sr.stock_selector, sr.sku_selector,
			sr.pagination_selector,
			sr.headers_json,
			sr.delay_ms
		FROM stores s
		JOIN scrape_rules sr ON sr.store_id = s.id
		WHERE s.is_active = true
	`

	rows, err := db.QueryContext(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("loading stores with rules: %w", err)
	}
	defer rows.Close()

	var results []StoreWithRule
	for rows.Next() {
		var sw StoreWithRule
		err := rows.Scan(
			&sw.Store.ID, &sw.Store.Name, &sw.Store.BaseURL,
			&sw.Store.Lat, &sw.Store.Lng,
			&sw.Rule.ID, &sw.Rule.StoreID,
			&sw.Rule.CatalogURLPattern,
			&sw.Rule.ItemSelector, &sw.Rule.PriceSelector, &sw.Rule.NameSelector,
			&sw.Rule.ImageSelector, &sw.Rule.StockSelector, &sw.Rule.SKUSelector,
			&sw.Rule.PaginationSelector,
			&sw.Rule.HeadersJSON,
			&sw.Rule.DelayMS,
		)
		if err != nil {
			return nil, fmt.Errorf("scanning store with rule: %w", err)
		}
		results = append(results, sw)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating stores: %w", err)
	}
	return results, nil
}

// CreateJob inserts a new scrape_job with status=running and returns the new job ID.
func (r *Repository) CreateJob(ctx context.Context, db *sql.DB, storeID int64) (int64, error) {
	const q = `
		INSERT INTO scrape_jobs (store_id, started_at, status)
		VALUES ($1, NOW(), 'running')
		RETURNING id
	`
	var id int64
	if err := db.QueryRowContext(ctx, q, storeID).Scan(&id); err != nil {
		return 0, fmt.Errorf("creating scrape job for store %d: %w", storeID, err)
	}
	return id, nil
}

// UpdateJob sets the final state of a scrape_job row.
func (r *Repository) UpdateJob(
	ctx context.Context,
	db *sql.DB,
	jobID int64,
	status string,
	found, updated, new_ int,
	errMsg *string,
) error {
	now := time.Now()
	const q = `
		UPDATE scrape_jobs
		SET finished_at      = $1,
		    status           = $2,
		    listings_found   = $3,
		    listings_updated = $4,
		    listings_new     = $5,
		    error_message    = $6
		WHERE id = $7
	`
	_, err := db.ExecContext(ctx, q, now, status, found, updated, new_, errMsg, jobID)
	if err != nil {
		return fmt.Errorf("updating scrape job %d: %w", jobID, err)
	}
	return nil
}

// GetLastSuccessfulJobCount returns listings_found from the most recent
// successful (status='success') job for the given store.
// Returns 0 if no successful job exists yet.
func (r *Repository) GetLastSuccessfulJobCount(ctx context.Context, db *sql.DB, storeID int64) (int, error) {
	const q = `
		SELECT COALESCE(MAX(listings_found), 0)
		FROM scrape_jobs
		WHERE store_id = $1
		  AND status = 'success'
	`
	var count int
	if err := db.QueryRowContext(ctx, q, storeID).Scan(&count); err != nil {
		return 0, fmt.Errorf("getting last successful job count for store %d: %w", storeID, err)
	}
	return count, nil
}
