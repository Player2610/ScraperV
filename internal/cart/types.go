package cart

import (
	"time"

	"github.com/protou/protou/internal/catalog"
)

// Cart represents a row from the carts table.
type Cart struct {
	ID        int64     `json:"id"`
	UserID    int64     `json:"user_id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// CartItem represents a row from the cart_items table.
type CartItem struct {
	ID        int64     `json:"id"`
	CartID    int64     `json:"cart_id"`
	ListingID int64     `json:"listing_id"`
	Quantity  int       `json:"quantity"`
	AddedAt   time.Time `json:"added_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// CartItemEnriched is a cart item combined with current listing data.
type CartItemEnriched struct {
	CartItem
	Listing     *catalog.Listing `json:"listing"`
	Unavailable bool             `json:"unavailable"`
}

// CartResponse is the full cart with enriched items.
type CartResponse struct {
	Cart  *Cart              `json:"cart"`
	Items []CartItemEnriched `json:"items"`
}

// GuestItem is used when migrating a guest (localStorage) cart.
type GuestItem struct {
	ListingID int64 `json:"listing_id"`
	Quantity  int   `json:"quantity"`
}
