package scraping

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGenerateSKU_Length(t *testing.T) {
	sku := GenerateSKU(1, "https://example.com/product/123")
	assert.Len(t, sku, 16, "GenerateSKU should return exactly 16 hex characters")
}

func TestGenerateSKU_Deterministic(t *testing.T) {
	a := GenerateSKU(1, "https://example.com/product/123")
	b := GenerateSKU(1, "https://example.com/product/123")
	assert.Equal(t, a, b, "GenerateSKU should be deterministic")
}

func TestGenerateSKU_DifferentInputs(t *testing.T) {
	a := GenerateSKU(1, "https://example.com/product/123")
	b := GenerateSKU(2, "https://example.com/product/123")
	c := GenerateSKU(1, "https://example.com/product/456")

	assert.NotEqual(t, a, b, "different storeID should produce different SKU")
	assert.NotEqual(t, a, c, "different productURL should produce different SKU")
}

func TestGenerateSKU_HexCharacters(t *testing.T) {
	sku := GenerateSKU(99, "https://tienda.com/producto/test")
	for _, r := range sku {
		assert.True(t,
			(r >= '0' && r <= '9') || (r >= 'a' && r <= 'f'),
			"SKU should contain only lowercase hex characters, got %q", string(r),
		)
	}
}
