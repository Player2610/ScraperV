package scraping

import (
	"strings"

	"github.com/PuerkitoBio/goquery"
)

// goQueryFromString is a helper to parse an HTML string into a goquery Document.
func goQueryFromString(html string) (*goquery.Document, error) {
	return goquery.NewDocumentFromReader(strings.NewReader(html))
}
