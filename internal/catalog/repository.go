package catalog

import (
	"context"
	"database/sql"
	"fmt"
)

// Repository provides data-access operations for the catalog domain.
type Repository struct {
	db *sql.DB
}

// NewRepository creates a new Repository backed by the provided *sql.DB.
func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

// SearchListings performs a full-text search using the listings.search_vector column.
// Listings with price_on_request or is_active=false are excluded.
// Returns the matching page, the total match count, and any error.
func (r *Repository) SearchListings(
	ctx context.Context,
	query string,
	filters ListingFilters,
	page Page,
) ([]Listing, int, error) {
	// Build base WHERE clause
	args := []interface{}{}
	argN := 1

	where := "l.is_active = true AND l.stock_signal != 'price_on_request'"

	if query != "" {
		where += fmt.Sprintf(" AND l.search_vector @@ to_tsquery('spanish', $%d)", argN)
		args = append(args, query)
		argN++
	}

	if filters.CategoryID != nil {
		where += fmt.Sprintf(" AND l.category_id = $%d", argN)
		args = append(args, *filters.CategoryID)
		argN++
	}

	if filters.StoreID != nil {
		where += fmt.Sprintf(" AND l.store_id = $%d", argN)
		args = append(args, *filters.StoreID)
		argN++
	}

	// Count query
	countSQL := fmt.Sprintf(`
		SELECT COUNT(*)
		FROM listings l
		WHERE %s
	`, where)

	var total int
	if err := r.db.QueryRowContext(ctx, countSQL, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("catalog: count listings: %w", err)
	}

	// Data query
	limitArg := argN
	offsetArg := argN + 1
	args = append(args, page.PerPage, page.Offset())

	dataSQL := fmt.Sprintf(`
		SELECT
			l.id,
			l.name,
			l.description,
			l.price_cop,
			l.image_url,
			l.product_url,
			l.stock_signal,
			l.last_scraped_at,
			s.id   AS store_id,
			s.name AS store_name,
			s.base_url AS store_base_url,
			c.id   AS cat_id,
			c.name AS cat_name,
			c.slug AS cat_slug,
			c.parent_id AS cat_parent_id
		FROM listings l
		JOIN stores s ON s.id = l.store_id
		LEFT JOIN categories c ON c.id = l.category_id
		WHERE %s
		ORDER BY l.last_scraped_at DESC NULLS LAST, l.id DESC
		LIMIT $%d OFFSET $%d
	`, where, limitArg, offsetArg)

	rows, err := r.db.QueryContext(ctx, dataSQL, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("catalog: search listings: %w", err)
	}
	defer rows.Close()

	listings, err := scanListings(rows)
	if err != nil {
		return nil, 0, err
	}
	return listings, total, nil
}

// GetListing returns a single listing by its ID.
// Returns an error wrapping sql.ErrNoRows if the listing is not found,
// inactive, or is price_on_request.
func (r *Repository) GetListing(ctx context.Context, id int64) (*Listing, error) {
	const q = `
		SELECT
			l.id,
			l.name,
			l.description,
			l.price_cop,
			l.image_url,
			l.product_url,
			l.stock_signal,
			l.last_scraped_at,
			s.id   AS store_id,
			s.name AS store_name,
			s.base_url AS store_base_url,
			c.id   AS cat_id,
			c.name AS cat_name,
			c.slug AS cat_slug,
			c.parent_id AS cat_parent_id
		FROM listings l
		JOIN stores s ON s.id = l.store_id
		LEFT JOIN categories c ON c.id = l.category_id
		WHERE l.id = $1
		  AND l.is_active = true
		  AND l.stock_signal != 'price_on_request'
	`
	rows, err := r.db.QueryContext(ctx, q, id)
	if err != nil {
		return nil, fmt.Errorf("catalog: get listing: %w", err)
	}
	defer rows.Close()

	listings, err := scanListings(rows)
	if err != nil {
		return nil, err
	}
	if len(listings) == 0 {
		return nil, fmt.Errorf("catalog: listing %d: %w", id, sql.ErrNoRows)
	}
	return &listings[0], nil
}

// GetListingAny returns a listing by ID regardless of active or stock status.
// Used internally by the cart to enrich items with current data.
func (r *Repository) GetListingAny(ctx context.Context, id int64) (*Listing, error) {
	const q = `
		SELECT
			l.id,
			l.name,
			l.description,
			l.price_cop,
			l.image_url,
			l.product_url,
			l.stock_signal,
			l.last_scraped_at,
			s.id   AS store_id,
			s.name AS store_name,
			s.base_url AS store_base_url,
			c.id   AS cat_id,
			c.name AS cat_name,
			c.slug AS cat_slug,
			c.parent_id AS cat_parent_id
		FROM listings l
		JOIN stores s ON s.id = l.store_id
		LEFT JOIN categories c ON c.id = l.category_id
		WHERE l.id = $1
	`
	rows, err := r.db.QueryContext(ctx, q, id)
	if err != nil {
		return nil, fmt.Errorf("catalog: get listing any: %w", err)
	}
	defer rows.Close()

	listings, err := scanListings(rows)
	if err != nil {
		return nil, err
	}
	if len(listings) == 0 {
		return nil, fmt.Errorf("catalog: listing %d: %w", id, sql.ErrNoRows)
	}
	return &listings[0], nil
}

// GetCategoryBySlug fetches a category by its URL slug.
func (r *Repository) GetCategoryBySlug(ctx context.Context, slug string) (*Category, error) {
	const q = `
		SELECT id, name, slug, parent_id, icon_url
		FROM categories
		WHERE slug = $1
	`
	row := r.db.QueryRowContext(ctx, q, slug)
	var cat Category
	if err := row.Scan(&cat.ID, &cat.Name, &cat.Slug, &cat.ParentID, &cat.IconURL); err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("catalog: category %q: %w", slug, sql.ErrNoRows)
		}
		return nil, fmt.Errorf("catalog: get category by slug: %w", err)
	}
	return &cat, nil
}

// ListCategoriesTree returns all categories assembled into a parent-child tree.
// Top-level categories (parent_id IS NULL) are the roots; their children are
// attached in the Children slice.
func (r *Repository) ListCategoriesTree(ctx context.Context) ([]Category, error) {
	const q = `
		SELECT id, name, slug, parent_id, icon_url
		FROM categories
		ORDER BY parent_id NULLS FIRST, name
	`
	rows, err := r.db.QueryContext(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("catalog: list categories: %w", err)
	}
	defer rows.Close()

	byID := map[int64]*Category{}
	var roots []*Category

	for rows.Next() {
		var cat Category
		if err := rows.Scan(&cat.ID, &cat.Name, &cat.Slug, &cat.ParentID, &cat.IconURL); err != nil {
			return nil, fmt.Errorf("catalog: scan category: %w", err)
		}
		c := &Category{
			ID:       cat.ID,
			Name:     cat.Name,
			Slug:     cat.Slug,
			ParentID: cat.ParentID,
			IconURL:  cat.IconURL,
		}
		byID[c.ID] = c
		if c.ParentID == nil {
			roots = append(roots, c)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("catalog: list categories rows: %w", err)
	}

	// Attach children to parents
	for _, c := range byID {
		if c.ParentID != nil {
			if parent, ok := byID[*c.ParentID]; ok {
				parent.Children = append(parent.Children, c)
			}
		}
	}

	out := make([]Category, 0, len(roots))
	for _, r := range roots {
		out = append(out, *r)
	}
	return out, nil
}

// ListStores returns all active stores.
func (r *Repository) ListStores(ctx context.Context) ([]Store, error) {
	const q = `
		SELECT id, name, base_url
		FROM stores
		WHERE is_active = true
		ORDER BY name
	`
	rows, err := r.db.QueryContext(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("catalog: list stores: %w", err)
	}
	defer rows.Close()

	var stores []Store
	for rows.Next() {
		var s Store
		if err := rows.Scan(&s.ID, &s.Name, &s.BaseURL); err != nil {
			return nil, fmt.Errorf("catalog: scan store: %w", err)
		}
		stores = append(stores, s)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("catalog: list stores rows: %w", err)
	}
	return stores, nil
}

// scanListings reads a *sql.Rows result set into a []Listing slice.
func scanListings(rows *sql.Rows) ([]Listing, error) {
	var listings []Listing
	for rows.Next() {
		var l Listing
		var catID sql.NullInt64
		var catName, catSlug sql.NullString
		var catParentID sql.NullInt64

		err := rows.Scan(
			&l.ID,
			&l.Name,
			&l.Description,
			&l.PriceCOP,
			&l.ImageURL,
			&l.ProductURL,
			&l.StockSignal,
			&l.LastScrapedAt,
			&l.Store.ID,
			&l.Store.Name,
			&l.Store.BaseURL,
			&catID,
			&catName,
			&catSlug,
			&catParentID,
		)
		if err != nil {
			return nil, fmt.Errorf("catalog: scan listing: %w", err)
		}
		if catID.Valid {
			cat := &Category{
				ID:   catID.Int64,
				Name: catName.String,
				Slug: catSlug.String,
			}
			if catParentID.Valid {
				cat.ParentID = &catParentID.Int64
			}
			l.Category = cat
		}
		l.OutOfStock = l.StockSignal == StockOut
		listings = append(listings, l)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("catalog: scan listings rows: %w", err)
	}
	return listings, nil
}
