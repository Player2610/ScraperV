package cart

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/protou/protou/internal/catalog"
)

// ErrUnavailable is returned when trying to add an unavailable listing.
var ErrUnavailable = errors.New("listing unavailable")

// Service implements business logic for the cart domain.
type Service struct {
	repo        *Repository
	catalogRepo *catalog.Repository
}

// NewService creates a new cart Service.
func NewService(repo *Repository, catalogRepo *catalog.Repository) *Service {
	return &Service{repo: repo, catalogRepo: catalogRepo}
}

// AddToCart adds or updates an item in the user's cart.
// qty=0 removes the item. Validates listing availability before adding.
func (s *Service) AddToCart(ctx context.Context, userID int64, listingID int64, qty int) error {
	if qty > 0 {
		listing, err := s.catalogRepo.GetListing(ctx, listingID)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return ErrUnavailable
			}
			return fmt.Errorf("cart: lookup listing: %w", err)
		}
		if !listing.IsActive() {
			return ErrUnavailable
		}
	}

	cart, err := s.repo.GetOrCreateCart(ctx, userID)
	if err != nil {
		return fmt.Errorf("cart: add to cart: %w", err)
	}
	return s.repo.UpsertItem(ctx, cart.ID, listingID, qty)
}

// RemoveItem removes a single item from the user's cart.
// It is idempotent: if the item was not in the cart the call succeeds silently.
func (s *Service) RemoveItem(ctx context.Context, userID int64, listingID int64) error {
	cart, err := s.repo.GetOrCreateCart(ctx, userID)
	if err != nil {
		return fmt.Errorf("cart: remove item: %w", err)
	}
	return s.repo.RemoveItem(ctx, cart.ID, listingID)
}

// GetCart returns the enriched cart for a user.
func (s *Service) GetCart(ctx context.Context, userID int64) (*CartResponse, error) {
	cart, items, err := s.repo.GetCartWithItems(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("cart: get cart: %w", err)
	}

	enriched := make([]CartItemEnriched, 0, len(items))
	for _, item := range items {
		ei := CartItemEnriched{CartItem: item}
		// Use GetListingAny so we can still show the item name even if out_of_stock
		listing, err := s.catalogRepo.GetListingAny(ctx, item.ListingID)
		if err != nil {
			// listing deleted or DB error
			ei.Unavailable = true
		} else {
			ei.Listing = listing
			// Unavailable if inactive (is_active=false) or out_of_stock or price_on_request
			if !listing.IsActive() || listing.StockSignal == catalog.StockPriceOnRequest {
				ei.Unavailable = true
			}
		}
		enriched = append(enriched, ei)
	}

	return &CartResponse{
		Cart:  cart,
		Items: enriched,
	}, nil
}

// MigrateGuestCart merges localStorage guest items into the DB cart.
// Skips items that are no longer available.
func (s *Service) MigrateGuestCart(ctx context.Context, userID int64, items []GuestItem) error {
	cart, err := s.repo.GetOrCreateCart(ctx, userID)
	if err != nil {
		return fmt.Errorf("cart: migrate guest cart: %w", err)
	}

	for _, item := range items {
		if item.Quantity <= 0 {
			continue
		}
		listing, err := s.catalogRepo.GetListing(ctx, item.ListingID)
		if err != nil {
			continue // skip unavailable
		}
		if !listing.IsActive() {
			continue
		}
		// Merge: add to existing quantity (upsert keeps the new value; use item qty directly)
		_ = s.repo.UpsertItem(ctx, cart.ID, item.ListingID, item.Quantity)
	}
	return nil
}

// GetCartForOrder returns the raw cart and items for use during checkout.
func (s *Service) GetCartForOrder(ctx context.Context, userID int64) (*Cart, []CartItem, error) {
	return s.repo.GetCartWithItems(ctx, userID)
}

// ClearCartByUserID finds the cart for a user and clears it within a transaction.
func (s *Service) ClearCartByUserID(ctx context.Context, tx *sql.Tx, userID int64) error {
	cart, err := s.repo.GetOrCreateCart(ctx, userID)
	if err != nil {
		return err
	}
	return s.repo.ClearCartTx(ctx, tx, cart.ID)
}
