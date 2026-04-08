package scraping

import (
	"crypto/sha256"
	"fmt"
)

// GenerateSKU produces a 16-character hex string from (storeID, productURL).
// This is used as the external_sku when a store does not expose an SKU field.
func GenerateSKU(storeID int64, productURL string) string {
	input := fmt.Sprintf("%d:%s", storeID, productURL)
	sum := sha256.Sum256([]byte(input))
	return fmt.Sprintf("%x", sum[:8]) // 8 bytes = 16 hex chars
}
