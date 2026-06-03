package scraping

import (
	"database/sql"
	"time"
)

// StockSignal represents the availability state of a listing.
type StockSignal string

const (
	StockIn             StockSignal = "in_stock"
	StockOut            StockSignal = "out_of_stock"
	StockUnknown        StockSignal = "unknown"
	StockPriceOnRequest StockSignal = "price_on_request"
)

// Store holds the DB store record used by the scraper.
type Store struct {
	ID      int64
	Name    string
	BaseURL string
	Lat     float64
	Lng     float64
}

// ScrapeRule holds the per-store scraping configuration loaded from DB.
type ScrapeRule struct {
	ID                 int64
	StoreID            int64
	CatalogURLPattern  string
	ItemSelector       string
	PriceSelector      string
	NameSelector       string
	ImageSelector      sql.NullString
	StockSelector      sql.NullString
	SKUSelector        sql.NullString
	PaginationSelector sql.NullString
	HeadersJSON        []byte
	DelayMS            int
}

// StoreWithRule bundles a Store with its associated ScrapeRule.
type StoreWithRule struct {
	Store Store
	Rule  ScrapeRule
}

// ScrapeJob represents one execution record for scraping a single store.
type ScrapeJob struct {
	ID              int64
	StoreID         int64
	StartedAt       time.Time
	FinishedAt      *time.Time
	Status          string
	ListingsFound   int
	ListingsUpdated int
	ListingsNew     int
	ErrorMessage    *string
}

// RawListing is the raw parsed data for a single product before DB upsert.
type RawListing struct {
	Name       string
	PriceRaw   string
	SKU        string
	ImageURL   string
	ProductURL string
	StockRaw   string
}
