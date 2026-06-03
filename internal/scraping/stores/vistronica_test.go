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

func vistronicaRule() scraping.ScrapeRule {
	return scraping.ScrapeRule{
		StoreID:            3,
		CatalogURLPattern:  "https://www.vistronica.com.co/catalogo/page/{page}/",
		ItemSelector:       "ul.products li.product",
		PriceSelector:      "span.woocommerce-Price-amount bdi",
		NameSelector:       "h2.woocommerce-loop-product__title",
		ImageSelector:      sql.NullString{String: "img.wp-post-image", Valid: true},
		StockSelector:      sql.NullString{String: ".out-of-stock", Valid: true},
		PaginationSelector: sql.NullString{String: "a.next.page-numbers", Valid: true},
		DelayMS:            0,
	}
}

func TestVistronicaParser_TwoProducts(t *testing.T) {
	f, err := os.Open("../../../testdata/vistronica/catalog_page.html")
	require.NoError(t, err)
	defer f.Close()

	parser := &stores.VistronicaParser{}
	listings, err := parser.Parse(f, vistronicaRule())

	require.NoError(t, err)
	assert.Len(t, listings, 2, "expected 2 listings from fixture")
}

func TestVistronicaParser_NormalProduct(t *testing.T) {
	f, err := os.Open("../../../testdata/vistronica/catalog_page.html")
	require.NoError(t, err)
	defer f.Close()

	parser := &stores.VistronicaParser{}
	listings, err := parser.Parse(f, vistronicaRule())

	require.NoError(t, err)
	require.GreaterOrEqual(t, len(listings), 1)

	transistor := listings[0]
	assert.Equal(t, "Transistor NPN 2N2222", transistor.Name)

	price, ok := scraping.ParsePrice(transistor.PriceRaw)
	assert.True(t, ok, "transistor price should be parseable")
	assert.Equal(t, 500, price)

	signal := scraping.ParseStockSignal(transistor.PriceRaw, transistor.StockRaw)
	assert.Equal(t, scraping.StockIn, signal)
}

func TestVistronicaParser_ConsultarPrecioProduct(t *testing.T) {
	f, err := os.Open("../../../testdata/vistronica/catalog_page.html")
	require.NoError(t, err)
	defer f.Close()

	parser := &stores.VistronicaParser{}
	listings, err := parser.Parse(f, vistronicaRule())

	require.NoError(t, err)
	require.Len(t, listings, 2)

	stepper := listings[1]
	assert.Equal(t, "Kit Motor Paso a Paso 28BYJ-48", stepper.Name)

	signal := scraping.ParseStockSignal(stepper.PriceRaw, stepper.StockRaw)
	assert.Equal(t, scraping.StockPriceOnRequest, signal,
		"product with 'Consultar precio' should map to price_on_request; priceRaw=%q stockRaw=%q",
		stepper.PriceRaw, stepper.StockRaw)
}

func TestVistronicaParser_ProductURLs(t *testing.T) {
	f, err := os.Open("../../../testdata/vistronica/catalog_page.html")
	require.NoError(t, err)
	defer f.Close()

	parser := &stores.VistronicaParser{}
	listings, err := parser.Parse(f, vistronicaRule())

	require.NoError(t, err)
	for i, l := range listings {
		assert.NotEmpty(t, l.ProductURL, "listing %d should have a ProductURL", i)
		assert.Contains(t, l.ProductURL, "vistronica.com.co")
	}
}
