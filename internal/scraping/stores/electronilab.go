package stores

import (
	"io"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/protou/protou/internal/scraping"
)

// ElectronilabParser handles the WooCommerce-based Electronilab catalog.
// Electronilab has a clean HTML structure — standard WooCommerce selectors apply.
type ElectronilabParser struct{}

// Parse extracts listings from an Electronilab catalog HTML page.
func (p *ElectronilabParser) Parse(r io.Reader, rule scraping.ScrapeRule) ([]scraping.RawListing, error) {
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

		raw.PriceRaw = strings.TrimSpace(s.Find(rule.PriceSelector).First().Text())

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
