// Package catalog manages product listings, categories, tags, and stores.
// It provides search and browsing functionality for the public catalog.
package catalog

import "time"

// StockSignal mirrors the DB enum stock_signal_enum.
type StockSignal string

const (
	StockIn             StockSignal = "in_stock"
	StockOut            StockSignal = "out_of_stock"
	StockUnknown        StockSignal = "unknown"
	StockPriceOnRequest StockSignal = "price_on_request"
)

// Store is a slim representation of a store row used in catalog responses.
type Store struct {
	ID      int64  `json:"id"`
	Name    string `json:"name"`
	BaseURL string `json:"base_url"`
}

// Category represents a product category with optional parent for tree building.
type Category struct {
	ID       int64       `json:"id"`
	Name     string      `json:"name"`
	Slug     string      `json:"slug"`
	ParentID *int64      `json:"parent_id"`
	IconURL  *string     `json:"icon_url"`
	Children []*Category `json:"children,omitempty"`
}

// Listing is a single catalog listing as returned from the repository.
type Listing struct {
	ID            int64       `json:"id"`
	Store         Store       `json:"store"`
	Name          string      `json:"name"`
	Description   *string     `json:"description"`
	PriceCOP      *int        `json:"price_cop"`
	ImageURL      *string     `json:"image_url"`
	ProductURL    string      `json:"product_url"`
	StockSignal   StockSignal `json:"stock_signal"`
	Category      *Category   `json:"category"`
	LastScrapedAt *time.Time  `json:"last_scraped_at"`
	OutOfStock    bool        `json:"out_of_stock"`
}

// IsActive returns true if the listing can be added to a cart.
// GetListing already filters out is_active=false and price_on_request,
// so this only needs to check the stock signal.
func (l *Listing) IsActive() bool {
	return l.StockSignal != StockOut
}

// ListingFilters holds optional filter parameters for catalog searches.
type ListingFilters struct {
	CategoryID *int64
	StoreID    *int64
}

// Page holds pagination parameters.
type Page struct {
	Number  int
	PerPage int
}

// Offset calculates the SQL offset for pagination.
func (p Page) Offset() int {
	if p.Number <= 1 {
		return 0
	}
	return (p.Number - 1) * p.PerPage
}
