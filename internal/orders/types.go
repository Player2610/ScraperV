package orders

import (
	"encoding/json"
	"time"
)

// OrderStatus mirrors the DB enum order_status_enum.
type OrderStatus string

const (
	StatusPendingConfirmation OrderStatus = "pending_confirmation"
	StatusConfirmed           OrderStatus = "confirmed"
	StatusPurchasing          OrderStatus = "purchasing"
	StatusInDelivery          OrderStatus = "in_delivery"
	StatusDelivered           OrderStatus = "delivered"
	StatusCancelled           OrderStatus = "cancelled"
	StatusFailed              OrderStatus = "failed"
)

// PaymentMethod mirrors the DB enum payment_method_enum.
type PaymentMethod string

const (
	PaymentNequi      PaymentMethod = "nequi"
	PaymentDaviplata  PaymentMethod = "daviplata"
	PaymentEfectivo   PaymentMethod = "efectivo"
	PaymentLlavesBrev PaymentMethod = "llaves_breve"
)

// AddressSnapshot is stored as JSONB in orders.delivery_address_snapshot.
type AddressSnapshot struct {
	FullAddress string   `json:"full_address"`
	Label       *string  `json:"label,omitempty"`
	Reference   *string  `json:"reference,omitempty"`
	Lat         *float64 `json:"lat,omitempty"`
	Lng         *float64 `json:"lng,omitempty"`
}

// Order mirrors the orders table.
type Order struct {
	ID                      int64           `json:"id"`
	UserID                  int64           `json:"user_id"`
	Status                  OrderStatus     `json:"status"`
	DeliveryAddressSnapshot AddressSnapshot `json:"delivery_address"`
	SubtotalCOP             int             `json:"subtotal_cop"`
	DeliveryFeeCOP          int             `json:"delivery_fee_cop"`
	TotalCOP                int             `json:"total_cop"`
	PaymentMethod           PaymentMethod   `json:"payment_method"`
	Notes                   *string         `json:"notes"`
	CreatedAt               time.Time       `json:"created_at"`
	UpdatedAt               time.Time       `json:"updated_at"`
}

// OrderItem mirrors the order_items table.
type OrderItem struct {
	ID                    int64     `json:"id"`
	OrderID               int64     `json:"order_id"`
	ListingID             *int64    `json:"listing_id"`
	ListingNameSnapshot   string    `json:"listing_name_snapshot"`
	ListingStoreSnapshot  string    `json:"listing_store_snapshot"`
	PriceSnapshotCOP      int       `json:"price_snapshot_cop"`
	Quantity              int       `json:"quantity"`
	IsCancelled           bool      `json:"is_cancelled"`
	CreatedAt             time.Time `json:"created_at"`
}

// NewOrder holds data needed to insert an order.
type NewOrder struct {
	UserID                  int64
	DeliveryAddressSnapshot AddressSnapshot
	SubtotalCOP             int
	DeliveryFeeCOP          int
	TotalCOP                int
	PaymentMethod           PaymentMethod
	Notes                   *string
}

// NewOrderItem holds data needed to insert an order_item.
type NewOrderItem struct {
	ListingID             int64
	ListingNameSnapshot   string
	ListingStoreSnapshot  string
	PriceSnapshotCOP      int
	Quantity              int
}

// CreateOrderRequest is the HTTP request body for POST /v1/orders.
type CreateOrderRequest struct {
	AddressID     int64         `json:"address_id"`
	PaymentMethod PaymentMethod `json:"payment_method"`
	Notes         *string       `json:"notes"`
}

// DeliveryFeeRequest is the HTTP request body for POST /v1/checkout/delivery-fee.
type DeliveryFeeRequest struct {
	AddressID int64 `json:"address_id"`
}

// MarshalJSON for AddressSnapshot (to ensure it serializes correctly).
func (a AddressSnapshot) MarshalJSON() ([]byte, error) {
	type Alias AddressSnapshot
	return json.Marshal(Alias(a))
}
