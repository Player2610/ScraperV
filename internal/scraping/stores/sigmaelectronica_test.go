package stores_test

import (
	"database/sql"
	"os"
	"testing"

	"github.com/protou/protou/internal/scraping"
	"github.com/protou/protou/internal/scraping/stores"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func sigmaRule() scraping.ScrapeRule {
	return scraping.ScrapeRule{
		StoreID:            1,
		CatalogURLPattern:  "https://www.sigmaelectronica.net/categoria/componentes-electronicos/?page={page}",
		ItemSelector:       "ul.products li.product",
		PriceSelector:      "span.woocommerce-Price-amount bdi",
		NameSelector:       "h2.woocommerce-loop-product__title",
		ImageSelector:      sql.NullString{String: "img.wp-post-image", Valid: true},
		StockSelector:      sql.NullString{String: ".out-of-stock", Valid: true},
		PaginationSelector: sql.NullString{String: "a.next.page-numbers", Valid: true},
		DelayMS:            0,
	}
}

func TestSigmaelectronicaParser_ThreeProducts(t *testing.T) {
	f, err := os.Open("../../../testdata/sigmaelectronica/catalog_page.html")
	require.NoError(t, err)
	defer f.Close()

	parser := &stores.SigmaelectronicaParser{}
	listings, err := parser.Parse(f, sigmaRule())

	require.NoError(t, err)
	assert.Len(t, listings, 3, "expected 3 listings from fixture")
}

func TestSigmaelectronicaParser_CorrectName(t *testing.T) {
	f, err := os.Open("../../../testdata/sigmaelectronica/catalog_page.html")
	require.NoError(t, err)
	defer f.Close()

	parser := &stores.SigmaelectronicaParser{}
	listings, err := parser.Parse(f, sigmaRule())

	require.NoError(t, err)
	require.GreaterOrEqual(t, len(listings), 1)

	assert.Equal(t, "Resistencia 10kΩ 1/4W", listings[0].Name)
}

func TestSigmaelectronicaParser_CorrectPrice(t *testing.T) {
	f, err := os.Open("../../../testdata/sigmaelectronica/catalog_page.html")
	require.NoError(t, err)
	defer f.Close()

	parser := &stores.SigmaelectronicaParser{}
	listings, err := parser.Parse(f, sigmaRule())

	require.NoError(t, err)
	require.GreaterOrEqual(t, len(listings), 1)

	price, ok := scraping.ParsePrice(listings[0].PriceRaw)
	assert.True(t, ok, "price should be parseable")
	assert.Equal(t, 200, price)
}

func TestSigmaelectronicaParser_ProductURL(t *testing.T) {
	f, err := os.Open("../../../testdata/sigmaelectronica/catalog_page.html")
	require.NoError(t, err)
	defer f.Close()

	parser := &stores.SigmaelectronicaParser{}
	listings, err := parser.Parse(f, sigmaRule())

	require.NoError(t, err)
	require.GreaterOrEqual(t, len(listings), 1)

	assert.Contains(t, listings[0].ProductURL, "sigmaelectronica.net")
}

func TestSigmaelectronicaParser_OutOfStockProduct(t *testing.T) {
	f, err := os.Open("../../../testdata/sigmaelectronica/catalog_page.html")
	require.NoError(t, err)
	defer f.Close()

	parser := &stores.SigmaelectronicaParser{}
	listings, err := parser.Parse(f, sigmaRule())

	require.NoError(t, err)
	require.Len(t, listings, 3)

	esp32 := listings[2]
	assert.Equal(t, "Módulo ESP32 DevKit", esp32.Name)

	signal := scraping.ParseStockSignal(esp32.PriceRaw, esp32.StockRaw)
	assert.Equal(t, scraping.StockOut, signal, "ESP32 should be out_of_stock")
}
