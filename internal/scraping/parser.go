package scraping

import (
	"fmt"
	"io"
	"strings"
	"unicode"

	"github.com/PuerkitoBio/goquery"
)

// StoreParser parses an HTML page into a slice of RawListing.
type StoreParser interface {
	Parse(r io.Reader, rule ScrapeRule) ([]RawListing, error)
}

// DefaultParser uses goquery CSS selectors from the ScrapeRule to extract listings.
type DefaultParser struct{}

// Parse reads an HTML page from r and returns raw listings extracted using the
// CSS selectors defined in rule. Returns an empty slice (not an error) when no
// items match the item_selector.
func (p *DefaultParser) Parse(r io.Reader, rule ScrapeRule) ([]RawListing, error) {
	doc, err := goquery.NewDocumentFromReader(r)
	if err != nil {
		return nil, fmt.Errorf("parsing HTML: %w", err)
	}

	var listings []RawListing

	doc.Find(rule.ItemSelector).Each(func(_ int, s *goquery.Selection) {
		raw := RawListing{}

		// Name
		if rule.NameSelector != "" {
			raw.Name = strings.TrimSpace(s.Find(rule.NameSelector).First().Text())
		}

		// Price
		if rule.PriceSelector != "" {
			raw.PriceRaw = strings.TrimSpace(s.Find(rule.PriceSelector).First().Text())
		}

		// Image URL
		if rule.ImageSelector.Valid && rule.ImageSelector.String != "" {
			imgSel := s.Find(rule.ImageSelector.String).First()
			if src, exists := imgSel.Attr("src"); exists {
				raw.ImageURL = strings.TrimSpace(src)
			} else if dataSrc, exists := imgSel.Attr("data-src"); exists {
				raw.ImageURL = strings.TrimSpace(dataSrc)
			}
		}

		// Product URL — look for the first anchor wrapping the item or the name anchor
		if href, exists := s.Find("a").First().Attr("href"); exists {
			raw.ProductURL = strings.TrimSpace(href)
		}

		// SKU (optional)
		if rule.SKUSelector.Valid && rule.SKUSelector.String != "" {
			raw.SKU = strings.TrimSpace(s.Find(rule.SKUSelector.String).First().Text())
		}

		// Stock signal raw text
		if rule.StockSelector.Valid && rule.StockSelector.String != "" {
			raw.StockRaw = strings.TrimSpace(s.Find(rule.StockSelector.String).First().Text())
			if raw.StockRaw == "" {
				// Check if selector element exists at all — its presence signals out of stock
				if s.Find(rule.StockSelector.String).Length() > 0 {
					raw.StockRaw = "out_of_stock"
				}
			}
		}

		// Skip items with no name (guard against malformed HTML)
		if raw.Name == "" {
			return
		}

		listings = append(listings, raw)
	})

	return listings, nil
}

// ParsePrice converts a raw price string (e.g. "$ 1.200", "COP 5.000", "1200")
// to an integer number of COP.
// Returns (price, true) on success, (0, false) when the price is unparseable
// or zero — which should be treated as price_on_request.
func ParsePrice(raw string) (int, bool) {
	// Strip currency markers and whitespace
	s := raw
	s = strings.ReplaceAll(s, "COP", "")
	s = strings.ReplaceAll(s, "$", "")
	s = strings.ReplaceAll(s, ".", "")
	s = strings.ReplaceAll(s, ",", "")
	s = strings.Map(func(r rune) rune {
		if unicode.IsSpace(r) {
			return -1
		}
		return r
	}, s)

	// If there's a range (e.g. "1000–2000"), take the first value
	if idx := strings.Index(s, "–"); idx != -1 {
		s = s[:idx]
	}
	if idx := strings.Index(s, "-"); idx != -1 && idx > 0 {
		s = s[:idx]
	}

	// Keep only digits
	var digits strings.Builder
	for _, r := range s {
		if unicode.IsDigit(r) {
			digits.WriteRune(r)
		}
	}

	if digits.Len() == 0 {
		return 0, false
	}

	var price int
	for _, r := range digits.String() {
		price = price*10 + int(r-'0')
	}

	if price == 0 {
		return 0, false
	}

	return price, true
}

// ParseStockSignal derives a StockSignal from the raw stock text scraped from
// the product page.
func ParseStockSignal(priceRaw, stockRaw string) StockSignal {
	lower := strings.ToLower(stockRaw + " " + priceRaw)

	if strings.Contains(lower, "agotado") ||
		strings.Contains(lower, "sin stock") ||
		strings.Contains(lower, "out of stock") ||
		strings.Contains(lower, "out_of_stock") ||
		strings.Contains(lower, "no disponible") {
		return StockOut
	}

	if strings.Contains(lower, "consultar precio") ||
		strings.Contains(lower, "consulte precio") ||
		strings.Contains(lower, "price on request") ||
		strings.Contains(lower, "precio a consultar") {
		return StockPriceOnRequest
	}

	// If price is missing/zero treat as price_on_request
	if _, ok := ParsePrice(priceRaw); !ok {
		return StockPriceOnRequest
	}

	if strings.Contains(lower, "disponible") ||
		strings.Contains(lower, "en stock") ||
		strings.Contains(lower, "in_stock") ||
		strings.Contains(lower, "in stock") {
		return StockIn
	}

	// Default: in_stock if a numeric price exists
	return StockIn
}
