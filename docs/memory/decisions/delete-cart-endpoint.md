# Decision: Added DELETE /v1/cart/items/{listing_id}

> Engram #117 | 2026-04-08 | topic: `cart/remove-item-endpoint`

**What**: Added `DELETE /v1/cart/items/{listing_id}` endpoint to remove a single cart item without affecting the rest of the cart.

**Why**: The upsert endpoint (`PUT /v1/cart/items/{listing_id}`) previously accepted `quantity=0` to delete items, but a validation change (quantity must be 1–99) blocked that path. A dedicated DELETE endpoint was needed.

**Where**:
- `internal/cart/repository.go` — added `RemoveItem(ctx, cartID, listingID int64) error`
- `internal/cart/service.go` — added `RemoveItem(ctx, userID, listingID int64) error`
- `internal/cart/handler.go` — added `removeItem` handler + registered `r.Delete("/v1/cart/items/{listing_id}", h.removeItem)` before the existing `DELETE /v1/cart`
- `internal/cart/service_test.go` — added two unit tests

**Learned**:
- The endpoint is **idempotent**: if the item was not in the cart, DELETE affects 0 rows but returns 204 No Content.
- No listing availability check in RemoveItem — unlike AddToCart, removal is always permitted regardless of stock status.
- Route ordering in chi: `DELETE /v1/cart/items/{listing_id}` must be registered before `DELETE /v1/cart`.
