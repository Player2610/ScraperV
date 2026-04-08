// Package stores provides store-specific scraper implementations.
// Each file handles quirks for a specific electronics store.
package stores

import (
	"io"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/protou/protou/internal/scraping"
)

// SigmaelectronicaParser handles the WooCommerce-based Sigmaelectrónica catalog.
// Selectors target the standard WooCommerce product grid layout.
type SigmaelectronicaParser struct{}

// Parse extracts listings from a Sigmaelectrónica catalog HTML page.
// It applies store-specific handling:
//   - Strips IVA markers from price text
//   - Detects "Agotado" badges as out_of_stock
func (p *SigmaelectronicaParser) Parse(r io.Reader, rule scraping.ScrapeRule) ([]scraping.RawListing, error) {
	doc, err := goquery.NewDocumentFromReader(r)
	if err != nil {
		return nil, err
	}

	var listings []scraping.RawListing

	doc.Find(rule.ItemSelector).Each(func(_ int, s *goquery.Selection) {
		raw := scraping.RawListing{}

		// Name
		raw.Name = strings.TrimSpace(s.Find(rule.NameSelector).First().Text())
		if raw.Name == "" {
			return
		}

		// Price — handle "$ 1.200" or "$ 1.200 + IVA"
		priceText := strings.TrimSpace(s.Find(rule.PriceSelector).First().Text())
		// Strip trailing " + IVA" annotations
		if idx := strings.Index(priceText, "+"); idx != -1 {
			priceText = priceText[:idx]
		}
		raw.PriceRaw = strings.TrimSpace(priceText)

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

		// Stock — check for "Agotado" badge or disabled add-to-cart
		agotadoText := strings.ToLower(s.Find(".out-of-stock, .button.disabled, .stock").Text())
		if strings.Contains(agotadoText, "agotado") || strings.Contains(agotadoText, "out of stock") {
			raw.StockRaw = "agotado"
		}

		listings = append(listings, raw)
	})

	return listings, nil
}
