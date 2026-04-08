package orders

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"

	"github.com/protou/protou/internal/cart"
	"github.com/protou/protou/internal/catalog"
	"github.com/protou/protou/internal/delivery"
	"github.com/protou/protou/internal/notifications"
	"github.com/protou/protou/internal/users"
)

// Service implements business logic for orders.
type Service struct {
	repo         *Repository
	cartSvc      *cart.Service
	catalogRepo  *catalog.Repository
	deliveryRepo *delivery.Repository
	userRepo     *users.Repository
	notifSvc     *notifications.Service
}

// NewService creates a new orders Service.
func NewService(
	repo *Repository,
	cartSvc *cart.Service,
	catalogRepo *catalog.Repository,
	deliveryRepo *delivery.Repository,
	userRepo *users.Repository,
	notifSvc *notifications.Service,
) *Service {
	return &Service{
		repo:         repo,
		cartSvc:      cartSvc,
		catalogRepo:  catalogRepo,
		deliveryRepo: deliveryRepo,
		userRepo:     userRepo,
		notifSvc:     notifSvc,
	}
}

// CalculateDeliveryFee returns a fee preview for a given address.
func (s *Service) CalculateDeliveryFee(ctx context.Context, userID int64, addressID int64) (*delivery.FeeResult, error) {
	addr, err := s.userRepo.GetAddress(ctx, addressID, userID)
	if err != nil {
		return nil, fmt.Errorf("orders: get address: %w", err)
	}

	if !delivery.IsAddressCovered(addr.FullAddress) {
		return nil, ErrOutsideZone
	}

	_, cartItems, err := s.cartSvc.GetCartForOrder(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("orders: get cart for fee: %w", err)
	}

	brackets, err := s.deliveryRepo.LoadBrackets(ctx)
	if err != nil {
		return nil, fmt.Errorf("orders: load brackets: %w", err)
	}

	cfg, err := s.deliveryRepo.LoadConfig(ctx)
	if err != nil {
		return nil, fmt.Errorf("orders: load delivery config: %w", err)
	}

	storeLatLngs, err := s.getStoreLatLngs(ctx, cartItems)
	if err != nil {
		return nil, err
	}

	var deliveryLatLng delivery.LatLng
	if addr.Lat != nil && addr.Lng != nil {
		deliveryLatLng = delivery.LatLng{Lat: *addr.Lat, Lng: *addr.Lng}
	}

	result := delivery.Calculate(storeLatLngs, deliveryLatLng, brackets, cfg.MultiStoreDiscountPct)
	return &result, nil
}

// CreateOrder creates a new order transactionally.
func (s *Service) CreateOrder(ctx context.Context, userID int64, req CreateOrderRequest) (*Order, []OrderItem, error) {
	// 1. Validate address and coverage
	addr, err := s.userRepo.GetAddress(ctx, req.AddressID, userID)
	if err != nil {
		return nil, nil, fmt.Errorf("orders: get address: %w", err)
	}
	if !delivery.IsAddressCovered(addr.FullAddress) {
		return nil, nil, ErrOutsideZone
	}

	// 2. Get cart items
	_, cartItems, err := s.cartSvc.GetCartForOrder(ctx, userID)
	if err != nil {
		return nil, nil, fmt.Errorf("orders: get cart: %w", err)
	}
	if len(cartItems) == 0 {
		return nil, nil, ErrEmptyCart
	}

	// 3. Validate all items are available and build order items with snapshots
	var newItems []NewOrderItem
	subtotal := 0
	for _, ci := range cartItems {
		listing, err := s.catalogRepo.GetListingAny(ctx, ci.ListingID)
		if err != nil {
			return nil, nil, ErrUnavailableItems
		}
		if !listing.IsActive() || listing.StockSignal == catalog.StockPriceOnRequest {
			return nil, nil, ErrUnavailableItems
		}
		if listing.PriceCOP == nil {
			return nil, nil, ErrUnavailableItems
		}
		itemTotal := *listing.PriceCOP * ci.Quantity
		subtotal += itemTotal
		newItems = append(newItems, NewOrderItem{
			ListingID:            ci.ListingID,
			ListingNameSnapshot:  listing.Name,
			ListingStoreSnapshot: listing.Store.Name,
			PriceSnapshotCOP:     *listing.PriceCOP,
			Quantity:             ci.Quantity,
		})
	}

	// 4. Load delivery config and calculate fee
	brackets, err := s.deliveryRepo.LoadBrackets(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("orders: load brackets: %w", err)
	}
	cfg, err := s.deliveryRepo.LoadConfig(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("orders: load config: %w", err)
	}
	storeLatLngs, err := s.getStoreLatLngs(ctx, cartItems)
	if err != nil {
		return nil, nil, err
	}

	var deliveryLatLng delivery.LatLng
	if addr.Lat != nil && addr.Lng != nil {
		deliveryLatLng = delivery.LatLng{Lat: *addr.Lat, Lng: *addr.Lng}
	}
	feeResult := delivery.Calculate(storeLatLngs, deliveryLatLng, brackets, cfg.MultiStoreDiscountPct)

	// 5. Build address snapshot
	addrSnapshot := AddressSnapshot{
		FullAddress: addr.FullAddress,
		Label:       addr.Label,
		Reference:   addr.Reference,
		Lat:         addr.Lat,
		Lng:         addr.Lng,
	}

	// 6. Execute transaction
	tx, err := s.repo.DB().BeginTx(ctx, nil)
	if err != nil {
		return nil, nil, fmt.Errorf("orders: begin tx: %w", err)
	}
	defer func() {
		if err != nil {
			tx.Rollback() //nolint:errcheck
		}
	}()

	newOrder := NewOrder{
		UserID:                  userID,
		DeliveryAddressSnapshot: addrSnapshot,
		SubtotalCOP:             subtotal,
		DeliveryFeeCOP:          feeResult.TotalFee,
		TotalCOP:                subtotal + feeResult.TotalFee,
		PaymentMethod:           req.PaymentMethod,
		Notes:                   req.Notes,
	}

	order, err := s.repo.CreateOrderTx(ctx, tx, newOrder)
	if err != nil {
		return nil, nil, fmt.Errorf("orders: create order: %w", err)
	}

	if err := s.repo.CreateOrderItemsTx(ctx, tx, order.ID, newItems); err != nil {
		return nil, nil, err
	}

	if err := s.cartSvc.ClearCartByUserID(ctx, tx, userID); err != nil {
		return nil, nil, fmt.Errorf("orders: clear cart in tx: %w", err)
	}

	if err := s.repo.CreateOrderEventTx(ctx, tx, order.ID, string(StatusPendingConfirmation), "order created"); err != nil {
		return nil, nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, nil, fmt.Errorf("orders: commit: %w", err)
	}

	// 7. Async notification (don't block, don't fail)
	orderCopy := *order
	go func() {
		user, userErr := s.userRepo.GetUserByID(context.Background(), userID)
		if userErr != nil {
			slog.Error("orders: notification: get user", "user_id", userID, "error", userErr)
			return
		}
		notifOrder := notifications.Order{
			ID:             orderCopy.ID,
			SubtotalCOP:    orderCopy.SubtotalCOP,
			DeliveryFeeCOP: orderCopy.DeliveryFeeCOP,
			TotalCOP:       orderCopy.TotalCOP,
			PaymentMethod:  string(orderCopy.PaymentMethod),
			DeliveryAddressSnapshot: notifications.AddressSnapshot{
				FullAddress: orderCopy.DeliveryAddressSnapshot.FullAddress,
				Label:       orderCopy.DeliveryAddressSnapshot.Label,
				Reference:   orderCopy.DeliveryAddressSnapshot.Reference,
			},
		}
		var notifItems []notifications.OrderItem
		for _, ni := range newItems {
			notifItems = append(notifItems, notifications.OrderItem{
				OrderID:              orderCopy.ID,
				ListingNameSnapshot:  ni.ListingNameSnapshot,
				ListingStoreSnapshot: ni.ListingStoreSnapshot,
				PriceSnapshotCOP:     ni.PriceSnapshotCOP,
				Quantity:             ni.Quantity,
			})
		}
		if notifErr := s.notifSvc.SendOrderCreated(context.Background(), user.Email, user.Name, notifOrder, notifItems); notifErr != nil {
			slog.Error("orders: notification: send email", "error", notifErr)
		}
	}()

	items := buildOrderItemsList(order.ID, newItems)
	return order, items, nil
}

// GetOrder returns an order by ID, verifying ownership.
func (s *Service) GetOrder(ctx context.Context, id, userID int64) (*Order, []OrderItem, error) {
	order, items, err := s.repo.GetOrder(ctx, id, userID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil, ErrNotFound
		}
		return nil, nil, err
	}
	if items == nil {
		items = []OrderItem{}
	}
	return order, items, nil
}

// ListOrders returns all orders for a user.
func (s *Service) ListOrders(ctx context.Context, userID int64) ([]Order, error) {
	orders, err := s.repo.ListOrders(ctx, userID)
	if err != nil {
		return nil, err
	}
	if orders == nil {
		orders = []Order{}
	}
	return orders, nil
}

// ─── helpers ─────────────────────────────────────────────────────────────────

// getStoreLatLngs loads unique store coordinates for cart items.
// Stores without lat/lng are skipped (treated as at origin 0,0 — acceptable for MVP).
func (s *Service) getStoreLatLngs(ctx context.Context, cartItems []cart.CartItem) ([]delivery.LatLng, error) {
	seenStores := map[int64]bool{}
	var latLngs []delivery.LatLng

	for _, ci := range cartItems {
		// Get listing to find store_id
		listing, err := s.catalogRepo.GetListingAny(ctx, ci.ListingID)
		if err != nil {
			continue
		}
		storeID := listing.Store.ID
		if seenStores[storeID] {
			continue
		}
		seenStores[storeID] = true

		lat, lng, err := s.repo.GetStoreLatLng(ctx, storeID)
		if err != nil || lat == nil || lng == nil {
			// Store has no coordinates — use centroid of 0,0 (fallback)
			continue
		}
		latLngs = append(latLngs, delivery.LatLng{Lat: *lat, Lng: *lng})
	}

	// If no coordinates available, return a single "origin" point
	if len(latLngs) == 0 {
		latLngs = append(latLngs, delivery.LatLng{Lat: 4.6097, Lng: -74.0817}) // Bogotá center
	}
	return latLngs, nil
}

func buildOrderItemsList(orderID int64, newItems []NewOrderItem) []OrderItem {
	items := make([]OrderItem, len(newItems))
	for i, ni := range newItems {
		id := ni.ListingID
		items[i] = OrderItem{
			OrderID:              orderID,
			ListingID:            &id,
			ListingNameSnapshot:  ni.ListingNameSnapshot,
			ListingStoreSnapshot: ni.ListingStoreSnapshot,
			PriceSnapshotCOP:     ni.PriceSnapshotCOP,
			Quantity:             ni.Quantity,
		}
	}
	return items
}

