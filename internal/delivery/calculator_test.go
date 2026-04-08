//go:build !integration

package delivery

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// ─── IsAddressCovered ────────────────────────────────────────────────────────

func TestIsAddressCovered(t *testing.T) {
	tests := []struct {
		name    string
		address string
		covered bool
	}{
		{"chapinero bogota", "Chapinero, Bogotá", true},
		{"bogota lowercase", "bogota", true},
		{"soacha ciudad verde", "Ciudad Verde, Soacha", true},
		{"bogota dc", "Bogotá D.C.", true},
		{"soacha only", "Soacha", true},
		{"bogota with accent in middle", "Calle 72, Bogotá, Colombia", true},
		{"medellin false", "Medellín", false},
		{"cali false", "Cali", false},
		{"barranquilla false", "Barranquilla", false},
		{"bucaramanga false", "Bucaramanga", false},
		{"empty string false", "", false},
		{"cartagena false", "Cartagena de Indias", false},
		{"bogota mixed case", "BOGOTA", true},
		{"soacha mixed case", "SOACHA", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsAddressCovered(tt.address)
			assert.Equal(t, tt.covered, got, "IsAddressCovered(%q)", tt.address)
		})
	}
}

// ─── haversine ───────────────────────────────────────────────────────────────

func TestHaversine_SamePoint_IsZero(t *testing.T) {
	bogotaCenter := LatLng{Lat: 4.6097, Lng: -74.0817}
	dist := haversine(bogotaCenter, bogotaCenter)
	assert.InDelta(t, 0.0, dist, 0.001, "distance from a point to itself should be 0")
}

func TestHaversine_BogotaToSoacha_Approx20km(t *testing.T) {
	bogota := LatLng{Lat: 4.6097, Lng: -74.0817}
	soacha := LatLng{Lat: 4.5794, Lng: -74.2173}
	dist := haversine(bogota, soacha)
	// Soacha is roughly 15–25 km from Bogotá center
	assert.InDelta(t, 18.0, dist, 8.0, "Bogotá to Soacha should be roughly 18 km ± 8 km")
}

func TestHaversine_BogotaToCali_Approx300km(t *testing.T) {
	bogota := LatLng{Lat: 4.6097, Lng: -74.0817}
	cali := LatLng{Lat: 3.4516, Lng: -76.5320}
	dist := haversine(bogota, cali)
	// Haversine great-circle distance Bogotá→Cali is ~300 km
	assert.InDelta(t, 300.0, dist, 30.0, "Bogotá to Cali should be roughly 300 km (great-circle)")
}

// ─── Calculate ───────────────────────────────────────────────────────────────

// testBrackets returns a simple set of fee brackets for tests.
func testBrackets() []FeeBracket {
	maxClose := 5.0
	maxMid := 15.0
	return []FeeBracket{
		{ID: 1, DistanceKmMin: 0, DistanceKmMax: &maxClose, FeeCOP: 3000},
		{ID: 2, DistanceKmMin: 5, DistanceKmMax: &maxMid, FeeCOP: 5000},
		{ID: 3, DistanceKmMin: 15, DistanceKmMax: nil, FeeCOP: 8000},
	}
}

func TestCalculate_SingleStore_CloseDistance_LowFee(t *testing.T) {
	// Store at same location as delivery — distance ~0 km
	store := LatLng{Lat: 4.6097, Lng: -74.0817}
	delivery := LatLng{Lat: 4.6097, Lng: -74.0817}

	result := Calculate([]LatLng{store}, delivery, testBrackets(), 0)

	assert.False(t, result.IsMultiStore)
	assert.Equal(t, 3000, result.TotalFee, "close distance should pick the lowest bracket fee")
}

func TestCalculate_SingleStore_FarDistance_HighFee(t *testing.T) {
	// Store in Bogotá center, delivery far away (simulate 20+ km)
	store := LatLng{Lat: 4.6097, Lng: -74.0817}
	// ~20 km south: approx Soacha
	deliveryFar := LatLng{Lat: 4.4100, Lng: -74.0817}

	result := Calculate([]LatLng{store}, deliveryFar, testBrackets(), 0)

	assert.False(t, result.IsMultiStore)
	assert.Greater(t, result.TotalFee, 3000, "far distance should produce higher fee")
}

func TestCalculate_MultiStore_IsMultiStoreTrue(t *testing.T) {
	storeA := LatLng{Lat: 4.6097, Lng: -74.0817}
	storeB := LatLng{Lat: 4.6500, Lng: -74.0600}
	delivery := LatLng{Lat: 4.6200, Lng: -74.0700}

	result := Calculate([]LatLng{storeA, storeB}, delivery, testBrackets(), 0)

	assert.True(t, result.IsMultiStore, "two stores should set IsMultiStore=true")
}

func TestCalculate_NoStores_ZeroFee(t *testing.T) {
	delivery := LatLng{Lat: 4.6097, Lng: -74.0817}
	result := Calculate([]LatLng{}, delivery, testBrackets(), 0)
	assert.Equal(t, 0, result.TotalFee)
}

func TestCalculate_AlwaysRoundedToNearest500(t *testing.T) {
	// Any Calculate result must be a multiple of 500
	store := LatLng{Lat: 4.6097, Lng: -74.0817}
	delivery := LatLng{Lat: 4.6097, Lng: -74.0817}

	result := Calculate([]LatLng{store}, delivery, testBrackets(), 0)

	assert.Equal(t, 0, result.TotalFee%500, "fee must be a multiple of 500")
}

func TestCalculate_MultiStore_WithDiscount(t *testing.T) {
	storeA := LatLng{Lat: 4.6097, Lng: -74.0817}
	storeB := LatLng{Lat: 4.6200, Lng: -74.0700}
	delivery := LatLng{Lat: 4.6097, Lng: -74.0817}

	resultNoDiscount := Calculate([]LatLng{storeA, storeB}, delivery, testBrackets(), 0)
	resultWithDiscount := Calculate([]LatLng{storeA, storeB}, delivery, testBrackets(), 10)

	// With 10% discount, fee should be less or equal
	assert.LessOrEqual(t, resultWithDiscount.TotalFee, resultNoDiscount.TotalFee,
		"discount should reduce or maintain the fee")
}

func TestCalculate_SingleStore_FeeAlwaysMultiple500(t *testing.T) {
	tests := []struct {
		name     string
		store    LatLng
		delivery LatLng
	}{
		{"close", LatLng{4.6097, -74.0817}, LatLng{4.6100, -74.0820}},
		{"mid range", LatLng{4.6097, -74.0817}, LatLng{4.65, -74.05}},
		{"far", LatLng{4.6097, -74.0817}, LatLng{4.4, -74.08}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Calculate([]LatLng{tt.store}, tt.delivery, testBrackets(), 0)
			assert.Equal(t, 0, result.TotalFee%500, "fee must be multiple of 500")
		})
	}
}
