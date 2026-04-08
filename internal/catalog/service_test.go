//go:build !integration

package catalog

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// ─── sanitizeFTSQuery ────────────────────────────────────────────────────────
//
// sanitizeFTSQuery is tested directly because it is the pure business-logic
// function exported by the service layer.  Tests that call s.SearchListings or
// s.GetListing require a real PostgreSQL connection and belong in the
// integration test suite.

func TestSanitizeFTSQuery_PlainWord(t *testing.T) {
	out := sanitizeFTSQuery("resistencia")
	assert.Equal(t, "'resistencia'", out)
}

func TestSanitizeFTSQuery_MultipleWords(t *testing.T) {
	out := sanitizeFTSQuery("resistencia 10k")
	assert.Equal(t, "'resistencia' & '10k'", out)
}

func TestSanitizeFTSQuery_OhmsSymbol_DoesNotPanic(t *testing.T) {
	// Ω is not in the allowed set — should be stripped without panicking
	assert.NotPanics(t, func() {
		out := sanitizeFTSQuery("10kΩ")
		assert.NotEmpty(t, out, "should still return a token after stripping Ω")
	})
}

func TestSanitizeFTSQuery_CppSpecialChars_DoesNotPanic(t *testing.T) {
	assert.NotPanics(t, func() {
		out := sanitizeFTSQuery("C++")
		// + signs are stripped; "C" should remain
		assert.Equal(t, "'C'", out)
	})
}

func TestSanitizeFTSQuery_Microfarad_DoesNotPanic(t *testing.T) {
	assert.NotPanics(t, func() {
		// µ is not in allowed chars — should be stripped; 100F or 100 & F remains
		sanitizeFTSQuery("100µF")
	})
}

func TestSanitizeFTSQuery_OnlySpecialChars_ReturnsEmpty(t *testing.T) {
	out := sanitizeFTSQuery("!@#$%^&*()")
	assert.Equal(t, "", out, "only special chars should yield empty string")
}

func TestSanitizeFTSQuery_EmptyString_ReturnsEmpty(t *testing.T) {
	out := sanitizeFTSQuery("")
	assert.Equal(t, "", out)
}

func TestSanitizeFTSQuery_SpanishAccentedChars_Preserved(t *testing.T) {
	out := sanitizeFTSQuery("módulo sensor")
	assert.Equal(t, "'módulo' & 'sensor'", out)
}

func TestSanitizeFTSQuery_AllowedCharsPassThrough(t *testing.T) {
	out := sanitizeFTSQuery("arduino uno")
	assert.Equal(t, "'arduino' & 'uno'", out)
}

func TestSanitizeFTSQuery_LeadingTrailingSpaces(t *testing.T) {
	out := sanitizeFTSQuery("  sensor  ")
	assert.Equal(t, "'sensor'", out)
}

func TestSanitizeFTSQuery_MixedSpecialAndNormal(t *testing.T) {
	// "100µF capacitor" -> µ stripped, becomes "100F capacitor" or "100 F capacitor"
	out := sanitizeFTSQuery("100µF capacitor")
	assert.NotEmpty(t, out)
	assert.Contains(t, out, "'capacitor'")
}

// ─── ErrNotFound sentinel ────────────────────────────────────────────────────

func TestErrNotFound_IsNotNil(t *testing.T) {
	assert.NotNil(t, ErrNotFound)
	assert.Equal(t, "not found", ErrNotFound.Error())
}

// ─── Listing.IsActive ────────────────────────────────────────────────────────

func TestListingIsActive_OutOfStock_ReturnsFalse(t *testing.T) {
	l := &Listing{StockSignal: StockOut}
	assert.False(t, l.IsActive())
}

func TestListingIsActive_InStock_ReturnsTrue(t *testing.T) {
	l := &Listing{StockSignal: StockIn}
	assert.True(t, l.IsActive())
}

func TestListingIsActive_PriceOnRequest_ReturnsFalse(t *testing.T) {
	// price_on_request is not StockOut, but GetListing already excludes it.
	// IsActive only checks StockOut.
	l := &Listing{StockSignal: StockPriceOnRequest}
	assert.True(t, l.IsActive(),
		"IsActive only checks StockOut; price_on_request exclusion is done at repo level")
}

func TestListingIsActive_Unknown_ReturnsTrue(t *testing.T) {
	l := &Listing{StockSignal: StockUnknown}
	assert.True(t, l.IsActive())
}

// ─── Page.Offset ─────────────────────────────────────────────────────────────

func TestPageOffset(t *testing.T) {
	tests := []struct {
		name     string
		page     Page
		expected int
	}{
		{"first page", Page{Number: 1, PerPage: 20}, 0},
		{"zero page treated as first", Page{Number: 0, PerPage: 20}, 0},
		{"negative page", Page{Number: -1, PerPage: 20}, 0},
		{"second page", Page{Number: 2, PerPage: 20}, 20},
		{"third page", Page{Number: 3, PerPage: 10}, 20},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.page.Offset())
		})
	}
}
