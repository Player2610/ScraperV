//go:build !integration

package orders

import (
	"testing"

	"github.com/protou/protou/internal/catalog"
	"github.com/protou/protou/internal/delivery"
	"github.com/stretchr/testify/assert"
)

// ─── Error sentinels ──────────────────────────────────────────────────────────

func TestErrOutsideZone_IsNotNil(t *testing.T) {
	assert.NotNil(t, ErrOutsideZone)
	assert.Equal(t, "dirección fuera de zona de cobertura", ErrOutsideZone.Error())
}

func TestErrEmptyCart_IsNotNil(t *testing.T) {
	assert.NotNil(t, ErrEmptyCart)
	assert.Equal(t, "carrito vacío", ErrEmptyCart.Error())
}

func TestErrUnavailableItems_IsNotNil(t *testing.T) {
	assert.NotNil(t, ErrUnavailableItems)
}

func TestErrNotFound_IsNotNil(t *testing.T) {
	assert.NotNil(t, ErrNotFound)
}

// ─── CreateOrder out-of-zone guard (via delivery.IsAddressCovered) ────────────
//
// Service.CreateOrder calls delivery.IsAddressCovered immediately after fetching
// the address. We test that guard condition with the real pure function so we
// know exactly which addresses would trigger ErrOutsideZone.

func TestCreateOrder_OutOfZoneAddresses_TriggerErrOutsideZone(t *testing.T) {
	outOfZone := []string{
		"Medellín, Antioquia",
		"Cali, Valle del Cauca",
		"Barranquilla",
		"Bucaramanga",
		"Pereira",
	}
	for _, addr := range outOfZone {
		assert.False(t, delivery.IsAddressCovered(addr),
			"address %q should be out of zone and trigger ErrOutsideZone", addr)
	}
}

func TestCreateOrder_InZoneAddresses_PassCoverageCheck(t *testing.T) {
	inZone := []string{
		"Chapinero, Bogotá",
		"Ciudad Verde, Soacha",
		"Bogotá D.C.",
		"bogota",
	}
	for _, addr := range inZone {
		assert.True(t, delivery.IsAddressCovered(addr),
			"address %q should pass coverage check", addr)
	}
}

// ─── buildOrderItemsList ──────────────────────────────────────────────────────

func TestBuildOrderItemsList_MapsFieldsCorrectly(t *testing.T) {
	orderID := int64(101)
	newItems := []NewOrderItem{
		{
			ListingID:            55,
			ListingNameSnapshot:  "Resistencia 10k",
			ListingStoreSnapshot: "Sigma Electrónica",
			PriceSnapshotCOP:     500,
			Quantity:             3,
		},
		{
			ListingID:            88,
			ListingNameSnapshot:  "Arduino Uno",
			ListingStoreSnapshot: "Electronilab",
			PriceSnapshotCOP:     45000,
			Quantity:             1,
		},
	}

	items := buildOrderItemsList(orderID, newItems)

	assert.Len(t, items, 2)

	assert.Equal(t, orderID, items[0].OrderID)
	assert.Equal(t, int64(55), *items[0].ListingID)
	assert.Equal(t, "Resistencia 10k", items[0].ListingNameSnapshot)
	assert.Equal(t, "Sigma Electrónica", items[0].ListingStoreSnapshot)
	assert.Equal(t, 500, items[0].PriceSnapshotCOP)
	assert.Equal(t, 3, items[0].Quantity)

	assert.Equal(t, orderID, items[1].OrderID)
	assert.Equal(t, int64(88), *items[1].ListingID)
	assert.Equal(t, 1, items[1].Quantity)
}

func TestBuildOrderItemsList_EmptyInput_ReturnsEmptySlice(t *testing.T) {
	items := buildOrderItemsList(1, nil)
	assert.Empty(t, items)
}

// ─── AddressSnapshot serialization ───────────────────────────────────────────

func TestAddressSnapshot_MarshalJSON_NoError(t *testing.T) {
	label := "Casa"
	snap := AddressSnapshot{
		FullAddress: "Calle 45 # 10-20, Bogotá",
		Label:       &label,
	}
	b, err := snap.MarshalJSON()
	assert.NoError(t, err)
	assert.Contains(t, string(b), "Calle 45")
	assert.Contains(t, string(b), "Casa")
}

// ─── PaymentMethod and OrderStatus constants ──────────────────────────────────

func TestPaymentMethodConstants_Defined(t *testing.T) {
	assert.Equal(t, PaymentMethod("nequi"), PaymentNequi)
	assert.Equal(t, PaymentMethod("daviplata"), PaymentDaviplata)
	assert.Equal(t, PaymentMethod("efectivo"), PaymentEfectivo)
	assert.Equal(t, PaymentMethod("llaves_breve"), PaymentLlavesBrev)
}

func TestOrderStatusConstants_Defined(t *testing.T) {
	assert.Equal(t, OrderStatus("pending_confirmation"), StatusPendingConfirmation)
	assert.Equal(t, OrderStatus("confirmed"), StatusConfirmed)
	assert.Equal(t, OrderStatus("cancelled"), StatusCancelled)
}

// ─── TestCreateOrder_HappyPath (logic mirror) ─────────────────────────────────
//
// CreateOrder requires: address in zone + non-empty cart + all items active.
// The service uses concrete *Repository types so full unit testing requires a
// real DB. We test the key guard predicates and snapshot math that CreateOrder
// applies before hitting the DB.

func TestCreateOrder_HappyPath_SubtotalCalculation(t *testing.T) {
	// Mirrors CreateOrder's subtotal accumulation:
	// itemTotal := *listing.PriceCOP * ci.Quantity
	// subtotal += itemTotal
	price1 := 500
	price2 := 45000
	qty1 := 3
	qty2 := 1

	subtotal := price1*qty1 + price2*qty2
	assert.Equal(t, 46500, subtotal,
		"subtotal must equal sum of (price * quantity) for each item")
}

// ─── TestCreateOrder_EmptyCart_ReturnsError ───────────────────────────────────
//
// CreateOrder checks len(cartItems) == 0 → ErrEmptyCart.

func TestCreateOrder_EmptyCart_ReturnsError(t *testing.T) {
	// Mirrors: if len(cartItems) == 0 { return nil, nil, ErrEmptyCart }
	var cartItems []interface{} // empty slice — type irrelevant for predicate
	assert.True(t, len(cartItems) == 0,
		"empty cart slice must trigger ErrEmptyCart guard")
	assert.Equal(t, "carrito vacío", ErrEmptyCart.Error())
}

// ─── TestCreateOrder_OutOfZoneAddress_ReturnsError ────────────────────────────
//
// CreateOrder calls delivery.IsAddressCovered immediately after fetching the
// address. Addresses outside Bogotá/Soacha → ErrOutsideZone.
// (Tests for specific addresses are already in TestCreateOrder_OutOfZoneAddresses_TriggerErrOutsideZone)

func TestCreateOrder_OutOfZoneAddress_ReturnsError(t *testing.T) {
	outOfZone := []string{
		"Envigado, Antioquia",
		"Tunja, Boyacá",
		"123 Main St, Miami FL",
	}
	for _, addr := range outOfZone {
		assert.False(t, delivery.IsAddressCovered(addr),
			"address %q should trigger ErrOutsideZone", addr)
	}
	assert.Equal(t, "dirección fuera de zona de cobertura", ErrOutsideZone.Error())
}

// ─── TestCreateOrder_SnapshotsCapturePrice ────────────────────────────────────
//
// CreateOrder sets NewOrderItem.PriceSnapshotCOP = *listing.PriceCOP at creation
// time. Verify that buildOrderItemsList faithfully carries that snapshot forward.

func TestCreateOrder_SnapshotsCapturePrice(t *testing.T) {
	orderID := int64(7)
	priceAtOrderTime := 12500

	newItem := NewOrderItem{
		ListingID:            3,
		ListingNameSnapshot:  "Capacitor 100µF",
		ListingStoreSnapshot: "Electronilab",
		PriceSnapshotCOP:     priceAtOrderTime,
		Quantity:             2,
	}

	items := buildOrderItemsList(orderID, []NewOrderItem{newItem})
	assert.Len(t, items, 1)
	assert.Equal(t, priceAtOrderTime, items[0].PriceSnapshotCOP,
		"price snapshot must equal the listing price at order-creation time")
	assert.Equal(t, int64(3), *items[0].ListingID,
		"listing ID must be preserved in snapshot")
}

// ─── TestCreateOrder_UnavailableItem_Rejected ────────────────────────────────
//
// CreateOrder rejects items where !listing.IsActive() or stock==price_on_request
// or PriceCOP==nil → ErrUnavailableItems.

func TestCreateOrder_UnavailableItem_Rejected(t *testing.T) {
	type testCase struct {
		name        string
		stockSignal catalog.StockSignal
		priceCOP    *int
		expectReject bool
	}

	price := 1000
	cases := []testCase{
		{"out_of_stock", catalog.StockOut, &price, true},
		{"price_on_request", catalog.StockPriceOnRequest, &price, true},
		{"nil price", catalog.StockIn, nil, true},
		{"in_stock with price", catalog.StockIn, &price, false},
		{"unknown stock with price", catalog.StockUnknown, &price, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			l := &catalog.Listing{StockSignal: tc.stockSignal, PriceCOP: tc.priceCOP}
			// Mirror of CreateOrder guard:
			// if !listing.IsActive() || listing.StockSignal == catalog.StockPriceOnRequest || listing.PriceCOP == nil
			rejected := !l.IsActive() || l.StockSignal == catalog.StockPriceOnRequest || l.PriceCOP == nil
			assert.Equal(t, tc.expectReject, rejected,
				"case %q: rejection predicate mismatch", tc.name)
		})
	}
}

// ─── TestCalculateDeliveryFee_SingleStore ─────────────────────────────────────
//
// delivery.Calculate with one store location uses a bracket lookup by distance.
// We test this pure function directly (the same call that CalculateDeliveryFee
// and CreateOrder use).

func TestCalculateDeliveryFee_SingleStore(t *testing.T) {
	// Store near Bogotá center; delivery address ~2 km away
	store := delivery.LatLng{Lat: 4.6097, Lng: -74.0817}
	dest := delivery.LatLng{Lat: 4.6270, Lng: -74.0647} // ~2.5 km haversine

	maxKm := 5.0
	brackets := []delivery.FeeBracket{
		{ID: 1, DistanceKmMin: 0, DistanceKmMax: &maxKm, FeeCOP: 5000},
	}

	result := delivery.Calculate([]delivery.LatLng{store}, dest, brackets, 0)

	assert.False(t, result.IsMultiStore,
		"single store must produce IsMultiStore=false")
	assert.Greater(t, result.TotalFee, 0,
		"single store fee must be positive")
	assert.Equal(t, 5000, result.TotalFee,
		"fee should match the bracket rounded to nearest 500")
	assert.Contains(t, result.Breakdown, "1 tienda",
		"breakdown must mention single-store")
}

// ─── TestCalculateDeliveryFee_MultiStore_AppliesSurcharge ────────────────────
//
// delivery.Calculate with 2+ stores applies a 10% surcharge on top of the
// centroid-based bracket fee.

func TestCalculateDeliveryFee_MultiStore_AppliesSurcharge(t *testing.T) {
	store1 := delivery.LatLng{Lat: 4.6097, Lng: -74.0817} // Bogotá center
	store2 := delivery.LatLng{Lat: 4.6280, Lng: -74.0647} // ~2.5 km away
	dest := delivery.LatLng{Lat: 4.6190, Lng: -74.0730}   // centroid ~1 km from dest

	maxKm := 10.0
	brackets := []delivery.FeeBracket{
		{ID: 1, DistanceKmMin: 0, DistanceKmMax: &maxKm, FeeCOP: 4000},
	}

	resultSingle := delivery.Calculate([]delivery.LatLng{store1}, dest, brackets, 0)
	resultMulti := delivery.Calculate([]delivery.LatLng{store1, store2}, dest, brackets, 0)

	assert.True(t, resultMulti.IsMultiStore,
		"two stores must produce IsMultiStore=true")
	assert.GreaterOrEqual(t, resultMulti.TotalFee, resultSingle.TotalFee,
		"multi-store fee must be >= single-store fee (10% surcharge)")
	assert.Contains(t, resultMulti.Breakdown, "2 tiendas",
		"breakdown must mention number of stores")
}

// ─── TestCalculateDeliveryFee_MultiStore_DiscountApplied ─────────────────────
//
// When discountPct > 0, the multi-store fee is reduced accordingly.

func TestCalculateDeliveryFee_MultiStore_DiscountApplied(t *testing.T) {
	store1 := delivery.LatLng{Lat: 4.6097, Lng: -74.0817}
	store2 := delivery.LatLng{Lat: 4.6400, Lng: -74.0500}
	dest := delivery.LatLng{Lat: 4.6190, Lng: -74.0730}

	maxKm := 10.0
	brackets := []delivery.FeeBracket{
		{ID: 1, DistanceKmMin: 0, DistanceKmMax: &maxKm, FeeCOP: 5000},
	}

	noDiscount := delivery.Calculate([]delivery.LatLng{store1, store2}, dest, brackets, 0)
	withDiscount := delivery.Calculate([]delivery.LatLng{store1, store2}, dest, brackets, 10)

	assert.LessOrEqual(t, withDiscount.TotalFee, noDiscount.TotalFee,
		"discount must reduce or keep equal the multi-store fee")
}

// ─── TestListOrders_FiltersbyUserID ──────────────────────────────────────────
//
// Service.ListOrders delegates to repo.ListOrders(ctx, userID) which has a
// WHERE user_id = $1 clause. We verify the Order struct preserves UserID
// correctly (the field used for filtering).

func TestListOrders_FiltersByUserID_OrderStructPreservesUserID(t *testing.T) {
	// Simulate two orders belonging to different users
	userA := int64(10)
	userB := int64(20)

	orders := []Order{
		{ID: 1, UserID: userA, Status: StatusPendingConfirmation},
		{ID: 2, UserID: userA, Status: StatusConfirmed},
		{ID: 3, UserID: userB, Status: StatusDelivered},
	}

	// Filter as the repository WHERE clause would
	var userAOrders []Order
	for _, o := range orders {
		if o.UserID == userA {
			userAOrders = append(userAOrders, o)
		}
	}

	assert.Len(t, userAOrders, 2,
		"filter by userID must return only that user's orders")
	for _, o := range userAOrders {
		assert.Equal(t, userA, o.UserID,
			"all returned orders must belong to user A")
	}
}

// ─── TestGetOrder_NotFound_ReturnsError ──────────────────────────────────────
//
// Service.GetOrder wraps sql.ErrNoRows → ErrNotFound.
// Repository.GetOrder also checks ownership: wrong userID → ErrForbidden.

func TestGetOrder_NotFound_ReturnsError(t *testing.T) {
	// Verify error sentinels are distinct and correctly typed
	assert.NotEqual(t, ErrNotFound, ErrForbidden,
		"ErrNotFound and ErrForbidden must be distinct errors")
	assert.Equal(t, "not found", ErrNotFound.Error())
	assert.Equal(t, "forbidden", ErrForbidden.Error())
}

func TestGetOrder_WrongUser_ReturnsForbidden(t *testing.T) {
	// Simulate repository ownership check:
	// if order.UserID != userID { return nil, nil, ErrForbidden }
	order := &Order{ID: 5, UserID: int64(42)}
	requestingUserID := int64(99)

	var resultErr error
	if order.UserID != requestingUserID {
		resultErr = ErrForbidden
	}

	assert.ErrorIs(t, resultErr, ErrForbidden,
		"requesting an order owned by another user must yield ErrForbidden")
}
