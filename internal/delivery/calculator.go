package delivery

import (
	"fmt"
	"math"
	"strings"
)

// IsAddressCovered reports whether the full address is within the delivery zone
// (Bogotá or Soacha).
func IsAddressCovered(fullAddress string) bool {
	lower := strings.ToLower(fullAddress)
	return strings.Contains(lower, "bogotá") ||
		strings.Contains(lower, "bogota") ||
		strings.Contains(lower, "soacha")
}

// haversine returns the great-circle distance in km between two coordinates.
func haversine(a, b LatLng) float64 {
	const earthRadiusKm = 6371.0
	lat1 := toRad(a.Lat)
	lat2 := toRad(b.Lat)
	dLat := toRad(b.Lat - a.Lat)
	dLng := toRad(b.Lng - a.Lng)

	h := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Cos(lat1)*math.Cos(lat2)*math.Sin(dLng/2)*math.Sin(dLng/2)
	c := 2 * math.Atan2(math.Sqrt(h), math.Sqrt(1-h))
	return earthRadiusKm * c
}

func toRad(deg float64) float64 { return deg * math.Pi / 180 }

// roundToNearest500 rounds v up to the nearest 500 COP.
func roundToNearest500(v float64) int {
	return int(math.Round(v/500) * 500)
}

// centroid computes the geographic mean of a set of points.
func centroid(points []LatLng) LatLng {
	if len(points) == 0 {
		return LatLng{}
	}
	var sumLat, sumLng float64
	for _, p := range points {
		sumLat += p.Lat
		sumLng += p.Lng
	}
	n := float64(len(points))
	return LatLng{Lat: sumLat / n, Lng: sumLng / n}
}

// feeForDistance looks up the bracket fee for a given distance.
// Returns 0 if no bracket matches.
func feeForDistance(km float64, brackets []FeeBracket) int {
	for _, b := range brackets {
		if km < b.DistanceKmMin {
			continue
		}
		if b.DistanceKmMax == nil || km <= *b.DistanceKmMax {
			return b.FeeCOP
		}
	}
	// If distance exceeds all brackets, use the last bracket's fee
	if len(brackets) > 0 {
		return brackets[len(brackets)-1].FeeCOP
	}
	return 0
}

// Calculate computes the delivery fee.
// Single store: bracket lookup by haversine distance.
// Multi-store: centroid of stores + 10% surcharge - discountPct.
func Calculate(stores []LatLng, delivery LatLng, brackets []FeeBracket, discountPct int) FeeResult {
	if len(stores) == 0 {
		return FeeResult{TotalFee: 0, Breakdown: "no stores"}
	}

	if len(stores) == 1 {
		dist := haversine(stores[0], delivery)
		fee := feeForDistance(dist, brackets)
		rounded := roundToNearest500(float64(fee))
		return FeeResult{
			TotalFee:     rounded,
			IsMultiStore: false,
			Breakdown:    fmt.Sprintf("1 tienda, %.1f km → %d COP", dist, rounded),
		}
	}

	// Multi-store: use centroid of store locations
	center := centroid(stores)
	dist := haversine(center, delivery)
	baseFee := feeForDistance(dist, brackets)

	// Apply 10% surcharge for multiple pickups
	withSurcharge := float64(baseFee) * 1.10

	// Apply operator discount
	discount := float64(discountPct) / 100.0
	finalFee := withSurcharge * (1 - discount)
	rounded := roundToNearest500(finalFee)

	return FeeResult{
		TotalFee:     rounded,
		IsMultiStore: true,
		Breakdown: fmt.Sprintf(
			"%d tiendas, centroide %.1f km, base %d COP +10%% -%.0f%% → %d COP",
			len(stores), dist, baseFee, float64(discountPct), rounded,
		),
	}
}
