package stores

import (
	"io"
	"strings"

	"github.com/PuerkitoBio/goquery"

	"github.com/protou/protou/internal/scraping"
)

// VistronicaParser handles the WooCommerce-based Vistronica catalog.
// Key quirk: many products show "Consultar precio" instead of a numeric price.
// Those are mapped to price_on_request via ParseStockSignal.
type VistronicaParser struct{}

// Parse extracts listings from a Vistronica catalog HTML page.
// Products with "Consultar precio" are upserted with stock_signal=price_on_request.
func (p *VistronicaParser) Parse(r io.Reader, rule scraping.ScrapeRule) ([]scraping.RawListing, error) {
	doc, err := goquery.NewDocumentFromReader(r)
	if err != nil {
		return nil, err
	}

	var listings []scraping.RawListing

	doc.Find(rule.ItemSelector).Each(func(_ int, s *goquery.Selection) {
		raw := scraping.RawListing{}

		raw.Name = strings.TrimSpace(s.Find(rule.NameSelector).First().Text())
		if raw.Name == "" {
			return
		}

		// Price — may be "Consultar precio" for some products
		raw.PriceRaw = strings.TrimSpace(s.Find(rule.PriceSelector).First().Text())
		if raw.PriceRaw == "" {
			// Try the "consultar precio" anchor text as fallback
			consultarText := strings.TrimSpace(s.Find("a.consultar-precio, .price-on-request, .consultar").Text())
			if consultarText != "" {
				raw.PriceRaw = consultarText
			} else {
				raw.PriceRaw = "Consultar precio"
			}
		}

		// Image
		if rule.ImageSelector.Valid && rule.ImageSelector.String != "" {
			imgSel := s.Find(rule.ImageSelector.String).First()
			if src, ok := imgSel.Attr("src"); ok {
				raw.ImageURL = strings.TrimSpace(src)
			} else if src, ok := imgSel.Attr("data-src"); ok {
				raw.ImageURL = strings.TrimSpace(src)
			}
		}

		// Product URL
		if href, ok := s.Find("a.woocommerce-LoopProduct-link").First().Attr("href"); ok {
			raw.ProductURL = strings.TrimSpace(href)
		} else if href, ok := s.Find("a").First().Attr("href"); ok {
			raw.ProductURL = strings.TrimSpace(href)
		}

		// Stock
		stockText := strings.ToLower(s.Find(".out-of-stock, .stock").Text())
		if strings.Contains(stockText, "agotado") || strings.Contains(stockText, "out of stock") {
			raw.StockRaw = "agotado"
		}

		listings = append(listings, raw)
	})

	return listings, nil
}
