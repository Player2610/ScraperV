package scraping

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParsePrice(t *testing.T) {
	cases := []struct {
		input    string
		expected int
		ok       bool
	}{
		{"$ 200", 200, true},
		{"$\u00a0200", 200, true}, // non-breaking space
		{"$ 1.200", 1200, true},
		{"$ 12.500", 12500, true},
		{"$ 35.000", 35000, true},
		{"COP 45.000", 45000, true},
		{"45000", 45000, true},
		{"Consultar precio", 0, false},
		{"", 0, false},
		{"0", 0, false},
		{"$ 1.000 + IVA", 1000, true},
		{"$ 1.000 – $ 2.000", 1000, true}, // range → take first value
	}

	for _, tc := range cases {

		t.Run(tc.input, func(t *testing.T) {
			got, ok := ParsePrice(tc.input)
			assert.Equal(t, tc.ok, ok, "ok mismatch for %q", tc.input)
			if tc.ok {
				assert.Equal(t, tc.expected, got, "price mismatch for %q", tc.input)
			}
		})
	}
}

func TestParseStockSignal(t *testing.T) {
	cases := []struct {
		priceRaw string
		stockRaw string
		expected StockSignal
	}{
		{"$ 5.000", "", StockIn},
		{"$ 5.000", "Agotado", StockOut},
		{"$ 5.000", "out of stock", StockOut},
		{"$ 5.000", "Sin stock", StockOut},
		{"Consultar precio", "", StockPriceOnRequest},
		{"consultar precio", "", StockPriceOnRequest},
		{"", "", StockPriceOnRequest},
		{"$ 0", "", StockPriceOnRequest},
		{"$ 1.000", "Disponible", StockIn},
		{"$ 1.000", "En stock", StockIn},
	}

	for _, tc := range cases {

		t.Run(tc.priceRaw+"/"+tc.stockRaw, func(t *testing.T) {
			got := ParseStockSignal(tc.priceRaw, tc.stockRaw)
			assert.Equal(t, tc.expected, got)
		})
	}
}
