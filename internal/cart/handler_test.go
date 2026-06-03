//go:build !integration

package cart_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/protou/protou/internal/auth"
	"github.com/protou/protou/internal/cart"
)

// ─── helpers ─────────────────────────────────────────────────────────────────

// newCartRouter builds a chi router with cart routes.  The service is backed by
// nil repositories — this is intentional: all tests below exercise code paths
// that return before any repository call (auth check, param parse, validation),
// OR inject a JWT so that the middleware passes and we reach a layer that panics
// if the repo is nil.  For the nil-repo cases we only verify the early-return
// HTTP status code.
//
// For tests that need to reach service logic we use a real DB — those live in
// the integration suite.
func newCartRouter(t *testing.T) (*chi.Mux, string) {
	t.Helper()

	secret := "cart-handler-test-secret"
	if os.Getenv("JWT_SECRET") == "" {
		t.Setenv("JWT_SECRET", secret)
	}

	// Issue a valid student token so we can exercise paths past auth.
	token, err := auth.IssueToken(42, "student")
	require.NoError(t, err)

	// We create a handler with a nil service for tests that never reach it.
	h := cart.NewHandler(nil)
	r := chi.NewRouter()
	h.RegisterRoutes(r)
	return r, token
}

// bearerHeader returns an Authorization header value for a JWT.
func bearerHeader(token string) string { return "Bearer " + token }

// ─── upsertItem — input validation ───────────────────────────────────────────

// TestUpsertItem_NoAuth_Returns401 verifies that a request without a Bearer
// token is rejected by the RequireStudent middleware.
func TestUpsertItem_NoAuth_Returns401(t *testing.T) {
	r, _ := newCartRouter(t)

	req := httptest.NewRequest(http.MethodPut, "/v1/cart/items/1",
		bytes.NewBufferString(`{"quantity":1}`))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	r.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

// TestUpsertItem_InvalidListingID_Returns400 verifies that a non-integer
// listing_id in the URL is rejected with INVALID_PARAM.
func TestUpsertItem_InvalidListingID_Returns400(t *testing.T) {
	r, token := newCartRouter(t)

	req := httptest.NewRequest(http.MethodPut, "/v1/cart/items/not-a-number",
		bytes.NewBufferString(`{"quantity":1}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", bearerHeader(token))
	rr := httptest.NewRecorder()

	r.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
	assert.Contains(t, rr.Body.String(), "INVALID_PARAM")
}

// TestUpsertItem_InvalidBody_Returns400 verifies that malformed JSON is
// rejected with INVALID_BODY.
func TestUpsertItem_InvalidBody_Returns400(t *testing.T) {
	r, token := newCartRouter(t)

	req := httptest.NewRequest(http.MethodPut, "/v1/cart/items/5",
		bytes.NewBufferString(`{bad json`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", bearerHeader(token))
	rr := httptest.NewRecorder()

	r.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
	assert.Contains(t, rr.Body.String(), "INVALID_BODY")
}

// TestUpsertItem_QuantityValidation verifies the 1–99 quantity range guard.
//
// The handler returns 400 {"error":"INVALID_QUANTITY"} for qty < 1 or qty > 99.
// This validation runs AFTER JWT auth and body decode but BEFORE any DB call,
// so we need a valid token and a parseable body.
func TestUpsertItem_QuantityValidation(t *testing.T) {
	cases := []struct {
		name     string
		quantity int
		wantCode int
		wantBody string
	}{
		{"qty zero", 0, http.StatusBadRequest, "INVALID_QUANTITY"},
		{"qty negative", -1, http.StatusBadRequest, "INVALID_QUANTITY"},
		{"qty 100 (over max)", 100, http.StatusBadRequest, "INVALID_QUANTITY"},
		{"qty 99 (max valid — reaches service)", 99, -1, ""}, // passes validation
		{"qty 1 (min valid — reaches service)", 1, -1, ""},   // passes validation
		{"qty 50 (mid range — reaches service)", 50, -1, ""}, // passes validation
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// Each sub-test needs its own router (JWT_SECRET stays set after first call).
			r, token := newCartRouter(t)

			body, err := json.Marshal(map[string]int{"quantity": tc.quantity})
			require.NoError(t, err)

			req := httptest.NewRequest(http.MethodPut, "/v1/cart/items/10",
				bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Authorization", bearerHeader(token))
			rr := httptest.NewRecorder()

			// Wrap in a recover in case the nil service panics (only for valid-qty cases).
			func() {
				defer func() { recover() }() //nolint:errcheck
				r.ServeHTTP(rr, req)
			}()

			if tc.wantCode == -1 {
				// Validation passed — code is NOT 400 for INVALID_QUANTITY.
				// It may be 500 (nil service panics were recovered) or another code.
				assert.NotEqual(t, http.StatusBadRequest, rr.Code,
					"valid quantity must not be rejected by validation (got %d)", rr.Code)
				// Additionally: the body must not contain INVALID_QUANTITY.
				assert.NotContains(t, rr.Body.String(), "INVALID_QUANTITY")
			} else {
				assert.Equal(t, tc.wantCode, rr.Code)
				assert.Contains(t, rr.Body.String(), tc.wantBody)
			}
		})
	}
}

// ─── removeItem — new endpoint DELETE /v1/cart/items/{listing_id} ────────────

// TestRemoveItem_NoAuth_Returns401 verifies the RequireStudent guard on the new
// DELETE /v1/cart/items/{listing_id} endpoint.
func TestRemoveItem_NoAuth_Returns401(t *testing.T) {
	r, _ := newCartRouter(t)

	req := httptest.NewRequest(http.MethodDelete, "/v1/cart/items/1", nil)
	rr := httptest.NewRecorder()

	r.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

// TestRemoveItem_InvalidListingID_Returns400 verifies that a non-integer
// listing_id is rejected.
func TestRemoveItem_InvalidListingID_Returns400(t *testing.T) {
	r, token := newCartRouter(t)

	req := httptest.NewRequest(http.MethodDelete, "/v1/cart/items/abc", nil)
	req.Header.Set("Authorization", bearerHeader(token))
	rr := httptest.NewRecorder()

	r.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
	assert.Contains(t, rr.Body.String(), "INVALID_PARAM")
}

// TestRemoveItem_RouteExists verifies that DELETE /v1/cart/items/{listing_id}
// is actually registered (chi returns 405 Method Not Allowed for existing routes
// with the wrong method, and 404 for non-existent routes).
//
// We confirm this route exists by checking the response is NOT 404.
func TestRemoveItem_RouteExists(t *testing.T) {
	r, token := newCartRouter(t)

	// Use a valid numeric listing_id with auth; auth passes so chi routes the
	// request.  The nil service will panic inside RemoveItem, which we recover.
	req := httptest.NewRequest(http.MethodDelete, "/v1/cart/items/99", nil)
	req.Header.Set("Authorization", bearerHeader(token))
	rr := httptest.NewRecorder()

	func() {
		defer func() { recover() }() //nolint:errcheck
		r.ServeHTTP(rr, req)
	}()

	// 404 means the route is not registered — that would be a regression.
	assert.NotEqual(t, http.StatusNotFound, rr.Code,
		"DELETE /v1/cart/items/{listing_id} must be a registered route")
	// 405 (method not allowed) would mean chi knows the path but not DELETE — also wrong.
	assert.NotEqual(t, http.StatusMethodNotAllowed, rr.Code,
		"DELETE method must be allowed on /v1/cart/items/{listing_id}")
}

// TestClearCart_NoAuth_Returns401 verifies that DELETE /v1/cart requires auth.
func TestClearCart_NoAuth_Returns401(t *testing.T) {
	r, _ := newCartRouter(t)

	req := httptest.NewRequest(http.MethodDelete, "/v1/cart", nil)
	rr := httptest.NewRecorder()

	r.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

// TestMigrateGuestCart_NoAuth_Returns401 verifies that POST /v1/cart/migrate
// requires auth.
func TestMigrateGuestCart_NoAuth_Returns401(t *testing.T) {
	r, _ := newCartRouter(t)

	req := httptest.NewRequest(http.MethodPost, "/v1/cart/migrate",
		bytes.NewBufferString(`{"items":[]}`))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	r.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

// TestGetCart_NoAuth_Returns401 verifies that GET /v1/cart requires auth.
func TestGetCart_NoAuth_Returns401(t *testing.T) {
	r, _ := newCartRouter(t)

	req := httptest.NewRequest(http.MethodGet, "/v1/cart", nil)
	rr := httptest.NewRecorder()

	r.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

// ─── route registration completeness ─────────────────────────────────────────

// TestCartRoutes_AllRegistered verifies that all five cart endpoints are
// registered by checking chi returns 401 (auth guard fires) rather than 404
// (route not found) for authenticated requests to each route.
func TestCartRoutes_AllRegistered(t *testing.T) {
	routes := []struct {
		method string
		path   string
	}{
		{http.MethodGet, "/v1/cart"},
		{http.MethodPut, "/v1/cart/items/1"},
		{http.MethodDelete, "/v1/cart/items/1"},
		{http.MethodDelete, "/v1/cart"},
		{http.MethodPost, "/v1/cart/migrate"},
	}

	// Without a token all routes return 401 (not 404) — confirming they exist.
	for _, rt := range routes {
		t.Run(fmt.Sprintf("%s %s", rt.method, rt.path), func(t *testing.T) {
			r, _ := newCartRouter(t)

			var bodyReader *bytes.Reader
			if rt.method == http.MethodPut || rt.method == http.MethodPost {
				bodyReader = bytes.NewReader([]byte(`{}`))
			} else {
				bodyReader = bytes.NewReader(nil)
			}

			req := httptest.NewRequest(rt.method, rt.path, bodyReader)
			if rt.method == http.MethodPut || rt.method == http.MethodPost {
				req.Header.Set("Content-Type", "application/json")
			}
			rr := httptest.NewRecorder()
			r.ServeHTTP(rr, req)

			assert.NotEqual(t, http.StatusNotFound, rr.Code,
				"%s %s must be registered (got 404)", rt.method, rt.path)
			assert.NotEqual(t, http.StatusMethodNotAllowed, rr.Code,
				"%s %s method must be allowed", rt.method, rt.path)
		})
	}
}
