package stores_test

import (
	"database/sql"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/protou/protou/internal/scraping"
	"github.com/protou/protou/internal/scraping/stores"
)

func electronilabRule() scraping.ScrapeRule {
	return scraping.ScrapeRule{
		StoreID:            2,
		CatalogURLPattern:  "https://electronilab.co/tienda/page/{page}/",
		ItemSelector:       "ul.products li.product",
		PriceSelector:      "span.woocommerce-Price-amount bdi",
		NameSelector:       "h2.woocommerce-loop-product__title",
		ImageSelector:      sql.NullString{String: "img.wp-post-image", Valid: true},
		StockSelector:      sql.NullString{String: ".out-of-stock", Valid: true},
		PaginationSelector: sql.NullString{String: "a.next.page-numbers", Valid: true},
		DelayMS:            0,
	}
}

func TestElectronilabParser_ThreeProducts(t *testing.T) {
	f, err := os.Open("../../../testdata/electronilab/catalog_page.html")
	require.NoError(t, err)
	defer f.Close()

	parser := &stores.ElectronilabParser{}
	listings, err := parser.Parse(f, electronilabRule())

	require.NoError(t, err)
	assert.Len(t, listings, 3, "expected 3 listings from fixture")
}

func TestElectronilabParser_CorrectNames(t *testing.T) {
	f, err := os.Open("../../../testdata/electronilab/catalog_page.html")
	require.NoError(t, err)
	defer f.Close()

	parser := &stores.ElectronilabParser{}
	listings, err := parser.Parse(f, electronilabRule())

	require.NoError(t, err)
	require.Len(t, listings, 3)

	assert.Equal(t, "Sensor de Temperatura y Humedad DHT22", listings[0].Name)
	assert.Equal(t, "Arduino Uno R3", listings[1].Name)
	assert.Equal(t, "Módulo Relay 5V 1 Canal", listings[2].Name)
}

func TestElectronilabParser_PricesParseCorrectly(t *testing.T) {
	f, err := os.Open("../../../testdata/electronilab/catalog_page.html")
	require.NoError(t, err)
	defer f.Close()

	parser := &stores.ElectronilabParser{}
	listings, err := parser.Parse(f, electronilabRule())

	require.NoError(t, err)
	require.Len(t, listings, 3)

	cases := []struct {
		idx      int
		expected int
	}{
		{0, 12500},
		{1, 45000},
		{2, 8900},
	}

	for _, tc := range cases {
		price, ok := scraping.ParsePrice(listings[tc.idx].PriceRaw)
		assert.True(t, ok, "price for listing %d should be parseable", tc.idx)
		assert.Equal(t, tc.expected, price, "price mismatch for listing %d", tc.idx)
	}
}

func TestElectronilabParser_ProductURLs(t *testing.T) {
	f, err := os.Open("../../../testdata/electronilab/catalog_page.html")
	require.NoError(t, err)
	defer f.Close()

	parser := &stores.ElectronilabParser{}
	listings, err := parser.Parse(f, electronilabRule())

	require.NoError(t, err)
	for i, l := range listings {
		assert.NotEmpty(t, l.ProductURL, "listing %d should have a ProductURL", i)
		assert.Contains(t, l.ProductURL, "electronilab.co", "listing %d URL should reference electronilab.co", i)
	}
}
