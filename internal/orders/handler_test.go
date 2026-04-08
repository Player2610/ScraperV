//go:build !integration

package orders_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/protou/protou/internal/auth"
	"github.com/protou/protou/internal/orders"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ─── helpers ─────────────────────────────────────────────────────────────────

// newOrderRouter builds a chi router with order routes backed by a nil service.
// Tests below only exercise code paths that return before any service call
// (auth guard, body decode, payment_method validation).
func newOrderRouter(t *testing.T) (*chi.Mux, string) {
	t.Helper()

	if os.Getenv("JWT_SECRET") == "" {
		t.Setenv("JWT_SECRET", "order-handler-test-secret")
	}

	token, err := auth.IssueToken(42, "student")
	require.NoError(t, err)

	h := orders.NewHandler(nil)
	r := chi.NewRouter()
	h.RegisterRoutes(r)
	return r, token
}

func bearerHeader(token string) string { return "Bearer " + token }

// ─── createOrder — input validation ──────────────────────────────────────────

// TestCreateOrder_NoAuth_Returns401 verifies that POST /v1/orders requires auth.
func TestCreateOrder_NoAuth_Returns401(t *testing.T) {
	r, _ := newOrderRouter(t)

	req := httptest.NewRequest(http.MethodPost, "/v1/orders",
		bytes.NewBufferString(`{"address_id":1,"payment_method":"efectivo"}`))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	r.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

// TestCreateOrder_InvalidBody_Returns400 verifies that malformed JSON is
// rejected with INVALID_BODY before any service call.
func TestCreateOrder_InvalidBody_Returns400(t *testing.T) {
	r, token := newOrderRouter(t)

	req := httptest.NewRequest(http.MethodPost, "/v1/orders",
		bytes.NewBufferString(`{bad json`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", bearerHeader(token))
	rr := httptest.NewRecorder()

	r.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
	assert.Contains(t, rr.Body.String(), "INVALID_BODY")
}

// TestCreateOrder_MissingPaymentMethod_Returns400 verifies that an empty
// payment_method field is rejected with MISSING_FIELDS.
func TestCreateOrder_MissingPaymentMethod_Returns400(t *testing.T) {
	r, token := newOrderRouter(t)

	body, err := json.Marshal(map[string]interface{}{
		"address_id":     1,
		"payment_method": "",
	})
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/v1/orders", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", bearerHeader(token))
	rr := httptest.NewRecorder()

	r.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
	assert.Contains(t, rr.Body.String(), "MISSING_FIELDS")
}

// TestCreateOrder_InvalidPaymentMethod_Returns400 verifies that an unrecognised
// payment_method value is rejected with INVALID_PAYMENT_METHOD.
//
// This is the validation added in the hardening phase.
func TestCreateOrder_InvalidPaymentMethod_Returns400(t *testing.T) {
	invalidMethods := []string{
		"credit_card",
		"paypal",
		"transferencia",
		"NEQUI", // wrong case
		"cash",
	}

	for _, method := range invalidMethods {
		t.Run(method, func(t *testing.T) {
			r, token := newOrderRouter(t)

			body, err := json.Marshal(map[string]interface{}{
				"address_id":     1,
				"payment_method": method,
			})
			require.NoError(t, err)

			req := httptest.NewRequest(http.MethodPost, "/v1/orders", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Authorization", bearerHeader(token))
			rr := httptest.NewRecorder()

			r.ServeHTTP(rr, req)

			assert.Equal(t, http.StatusBadRequest, rr.Code,
				"payment method %q must be rejected with 400", method)
			assert.Contains(t, rr.Body.String(), "INVALID_PAYMENT_METHOD",
				"response body must contain INVALID_PAYMENT_METHOD for method %q", method)
		})
	}
}

// TestCreateOrder_ValidPaymentMethods_PassValidation verifies that all four
// accepted payment methods pass the validation layer.
//
// The service call will reach the nil service and panic; we recover to confirm
// only that the response code is NOT 400 with INVALID_PAYMENT_METHOD.
func TestCreateOrder_ValidPaymentMethods_PassValidation(t *testing.T) {
	validMethods := []string{"nequi", "daviplata", "efectivo", "llaves_breve"}

	for _, method := range validMethods {
		t.Run(method, func(t *testing.T) {
			r, token := newOrderRouter(t)

			body, err := json.Marshal(map[string]interface{}{
				"address_id":     1,
				"payment_method": method,
			})
			require.NoError(t, err)

			req := httptest.NewRequest(http.MethodPost, "/v1/orders", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Authorization", bearerHeader(token))
			rr := httptest.NewRecorder()

			func() {
				defer func() { recover() }() //nolint:errcheck
				r.ServeHTTP(rr, req)
			}()

			// Must NOT be a 400 INVALID_PAYMENT_METHOD rejection.
			assert.NotContains(t, rr.Body.String(), "INVALID_PAYMENT_METHOD",
				"valid payment method %q must pass validation", method)
		})
	}
}

// ─── deliveryFee — auth guard ─────────────────────────────────────────────────

// TestDeliveryFee_NoAuth_Returns401 verifies POST /v1/checkout/delivery-fee
// requires auth.
func TestDeliveryFee_NoAuth_Returns401(t *testing.T) {
	r, _ := newOrderRouter(t)

	req := httptest.NewRequest(http.MethodPost, "/v1/checkout/delivery-fee",
		bytes.NewBufferString(`{"address_id":1}`))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	r.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

// TestDeliveryFee_InvalidBody_Returns400 verifies malformed JSON is rejected.
func TestDeliveryFee_InvalidBody_Returns400(t *testing.T) {
	r, token := newOrderRouter(t)

	req := httptest.NewRequest(http.MethodPost, "/v1/checkout/delivery-fee",
		bytes.NewBufferString(`{bad json`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", bearerHeader(token))
	rr := httptest.NewRecorder()

	r.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
	assert.Contains(t, rr.Body.String(), "INVALID_BODY")
}

// ─── getOrder — input validation ─────────────────────────────────────────────

// TestGetOrder_NoAuth_Returns401 verifies GET /v1/orders/{id} requires auth.
func TestGetOrder_NoAuth_Returns401(t *testing.T) {
	r, _ := newOrderRouter(t)

	req := httptest.NewRequest(http.MethodGet, "/v1/orders/1", nil)
	rr := httptest.NewRecorder()

	r.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

// TestGetOrder_InvalidID_Returns400 verifies a non-integer order id is rejected.
func TestGetOrder_InvalidID_Returns400(t *testing.T) {
	r, token := newOrderRouter(t)

	req := httptest.NewRequest(http.MethodGet, "/v1/orders/not-a-number", nil)
	req.Header.Set("Authorization", bearerHeader(token))
	rr := httptest.NewRecorder()

	r.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
	assert.Contains(t, rr.Body.String(), "INVALID_PARAM")
}

// ─── route registration completeness ─────────────────────────────────────────

// TestOrderRoutes_AllRegistered verifies that all four order endpoints are
// registered.  Without a token all routes return 401 (not 404).
func TestOrderRoutes_AllRegistered(t *testing.T) {
	routes := []struct {
		method string
		path   string
	}{
		{http.MethodPost, "/v1/checkout/delivery-fee"},
		{http.MethodPost, "/v1/orders"},
		{http.MethodGet, "/v1/orders"},
		{http.MethodGet, "/v1/orders/1"},
	}

	for _, rt := range routes {
		t.Run(fmt.Sprintf("%s %s", rt.method, rt.path), func(t *testing.T) {
			r, _ := newOrderRouter(t)

			var body *bytes.Reader
			if rt.method == http.MethodPost {
				body = bytes.NewReader([]byte(`{}`))
			} else {
				body = bytes.NewReader(nil)
			}

			req := httptest.NewRequest(rt.method, rt.path, body)
			if rt.method == http.MethodPost {
				req.Header.Set("Content-Type", "application/json")
			}
			rr := httptest.NewRecorder()
			r.ServeHTTP(rr, req)

			assert.NotEqual(t, http.StatusNotFound, rr.Code,
				"%s %s must be registered (got 404)", rt.method, rt.path)
		})
	}
}
