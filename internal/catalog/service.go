package catalog

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"regexp"
	"strings"
)

// ftsSpecialChars is the set of characters NOT allowed in Spanish FTS tokens.
// We keep: ASCII letters, digits, and accented Spanish letters.
var ftsSpecialChars = regexp.MustCompile(`[^a-zA-Z0-9áéíóúüñÁÉÍÓÚÜÑ ]+`)

// Service wraps the catalog Repository with business-logic concerns:
// hiding price_on_request items, sanitizing FTS queries, and deriving
// out-of-stock warning flags.
type Service struct {
	repo *Repository
}

// NewService creates a new catalog Service.
func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

// SearchListings delegates to the repository after sanitizing the FTS query.
// price_on_request listings are already excluded at the DB level.
func (s *Service) SearchListings(
	ctx context.Context,
	query string,
	filters ListingFilters,
	page Page,
) ([]Listing, int, error) {
	sanitized := sanitizeFTSQuery(query)
	return s.repo.SearchListings(ctx, sanitized, filters, page)
}

// GetListing returns a single listing by ID.
// Returns (nil, ErrNotFound) when the listing does not exist, is inactive,
// or is price_on_request.
func (s *Service) GetListing(ctx context.Context, id int64) (*Listing, error) {
	listing, err := s.repo.GetListing(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return listing, nil
}

// GetCategoryBySlug returns a category by its URL slug, or ErrNotFound.
func (s *Service) GetCategoryBySlug(ctx context.Context, slug string) (*Category, error) {
	cat, err := s.repo.GetCategoryBySlug(ctx, slug)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return cat, nil
}

// ListCategoriesTree returns all categories assembled into a parent-child tree.
func (s *Service) ListCategoriesTree(ctx context.Context) ([]Category, error) {
	return s.repo.ListCategoriesTree(ctx)
}

// ListStores returns all active stores.
func (s *Service) ListStores(ctx context.Context) ([]Store, error) {
	return s.repo.ListStores(ctx)
}

// sanitizeFTSQuery strips characters outside [a-zA-Z0-9áéíóúüñÁÉÍÓÚÜÑ ],
// splits the result into words, wraps each word in single quotes for
// to_tsquery('spanish', ...), and joins them with the & operator.
// Returns an empty string when the input yields no valid tokens.
func sanitizeFTSQuery(raw string) string {
	clean := ftsSpecialChars.ReplaceAllString(raw, " ")
	words := strings.Fields(clean)
	if len(words) == 0 {
		return ""
	}
	tokens := make([]string, 0, len(words))
	for _, w := range words {
		tokens = append(tokens, fmt.Sprintf("'%s'", w))
	}
	return strings.Join(tokens, " & ")
}

// ErrNotFound is returned when a catalog entity cannot be found.
var ErrNotFound = errors.New("not found")
