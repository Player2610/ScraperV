package delivery

import "time"

// LatLng holds a geographic coordinate pair.
type LatLng struct {
	Lat float64
	Lng float64
}

// FeeBracket mirrors a row from the delivery_fee_brackets table.
type FeeBracket struct {
	ID             int64    `json:"id"`
	DistanceKmMin  float64  `json:"distance_km_min"`
	DistanceKmMax  *float64 `json:"distance_km_max"`
	FeeCOP         int      `json:"fee_cop"`
}

// DeliveryConfig mirrors the single row in delivery_config.
type DeliveryConfig struct {
	MultiStoreDiscountPct int       `json:"multi_store_discount_pct"`
	UpdatedAt             time.Time `json:"updated_at"`
}

// Courier mirrors a row from the couriers table.
type Courier struct {
	ID        int64     `json:"id"`
	Name      string    `json:"name"`
	Phone     string    `json:"phone"`
	IsActive  bool      `json:"is_active"`
	CreatedAt time.Time `json:"created_at"`
}

// FeeResult is the output of the delivery fee calculator.
type FeeResult struct {
	TotalFee     int    `json:"total_fee"`
	IsMultiStore bool   `json:"is_multi_store"`
	Breakdown    string `json:"breakdown"`
}
