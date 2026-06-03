//go:build integration

package platform_test

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	_ "github.com/lib/pq"
	"github.com/protou/protou/internal/cart"
	"github.com/protou/protou/internal/catalog"
	"github.com/protou/protou/internal/delivery"
	"github.com/protou/protou/internal/notifications"
	"github.com/protou/protou/internal/operator"
	"github.com/protou/protou/internal/orders"
	"github.com/protou/protou/internal/platform"
	"github.com/protou/protou/internal/users"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// buildIntegrationServer creates a real test HTTP server backed by a real DB.
// It wires every domain handler the same way main.go does.
func buildIntegrationServer(t *testing.T, db *sql.DB) *httptest.Server {
	t.Helper()

	cfg := platform.Config{
		CORSOrigin: "http://localhost",
		JWTSecret:  os.Getenv("JWT_SECRET"),
	}
	if cfg.JWTSecret == "" {
		cfg.JWTSecret = "integration-test-jwt-secret"
		t.Setenv("JWT_SECRET", cfg.JWTSecret)
	}

	router := platform.NewServer(cfg, db)

	catalogRepo := catalog.NewRepository(db)
	catalogSvc := catalog.NewService(catalogRepo)
	catalogHandler := catalog.NewHandler(catalogSvc)
	catalogHandler.RegisterRoutes(router)

	userRepo := users.NewRepository(db)
	userSvc := users.NewService(userRepo)
	userHandler := users.NewHandler(userSvc)
	userHandler.RegisterRoutes(router)

	cartRepo := cart.NewRepository(db)
	cartSvc := cart.NewService(cartRepo, catalogRepo)
	cartHandler := cart.NewHandler(cartSvc)
	cartHandler.RegisterRoutes(router)

	deliveryRepo := delivery.NewRepository(db)
	notifSvc := notifications.NewService(db)

	orderRepo := orders.NewRepository(db)
	orderSvc := orders.NewService(orderRepo, cartSvc, catalogRepo, deliveryRepo, userRepo, notifSvc)
	orderHandler := orders.NewHandler(orderSvc)
	orderHandler.RegisterRoutes(router)

	operatorHandler := operator.NewHandler(db, notifSvc)
	operatorHandler.RegisterRoutes(router)

	srv := httptest.NewServer(router)
	t.Cleanup(srv.Close)
	return srv
}

// openTestDB opens a connection to the DB specified by DATABASE_URL.
func openTestDB(t *testing.T) *sql.DB {
	t.Helper()
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("no DATABASE_URL — skipping integration test")
	}
	db, err := sql.Open("postgres", dsn)
	require.NoError(t, err, "open test DB")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	require.NoError(t, db.PingContext(ctx), "ping test DB")
	t.Cleanup(func() { db.Close() })
	return db
}

// postJSON is a convenience helper that sends a JSON POST and decodes the
// response body into dest (if non-nil).
func postJSON(t *testing.T, client *http.Client, url string, payload interface{}, dest interface{}, authToken string) *http.Response {
	t.Helper()
	body, err := json.Marshal(payload)
	require.NoError(t, err)
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	if authToken != "" {
		req.Header.Set("Authorization", "Bearer "+authToken)
	}
	resp, err := client.Do(req)
	require.NoError(t, err)
	if dest != nil {
		defer resp.Body.Close()
		raw, _ := io.ReadAll(resp.Body)
		if err := json.Unmarshal(raw, dest); err != nil {
			t.Logf("response body (could not decode): %s", raw)
		}
	}
	return resp
}

// getJSON is a convenience helper for authenticated GET requests.
func getJSON(t *testing.T, client *http.Client, url, authToken string, dest interface{}) *http.Response {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, url, nil)
	require.NoError(t, err)
	if authToken != "" {
		req.Header.Set("Authorization", "Bearer "+authToken)
	}
	resp, err := client.Do(req)
	require.NoError(t, err)
	if dest != nil {
		defer resp.Body.Close()
		raw, _ := io.ReadAll(resp.Body)
		if err := json.Unmarshal(raw, dest); err != nil {
			t.Logf("response body (could not decode): %s", raw)
		}
	}
	return resp
}

// TestStudentE2E exercises the full happy-path student flow:
//   register → login → add-to-cart → checkout (POST /v1/orders)
//
// The test skips when DATABASE_URL is not set.  After checkout it queries the
// DB directly to verify the order row was created.
func TestStudentE2E(t *testing.T) {
	db := openTestDB(t)
	srv := buildIntegrationServer(t, db)
	client := srv.Client()

	unique := fmt.Sprintf("student_e2e_%d", time.Now().UnixNano())
	email := unique + "@example.com"
	password := "securepassword123"
	fullName := "E2E Test Student"

	// ── 1. Register ────────────────────────────────────────────────────────────
	var registerResp struct {
		Token string `json:"token"`
		User  struct {
			ID int64 `json:"id"`
		} `json:"user"`
	}
	resp := postJSON(t, client, srv.URL+"/v1/auth/register", map[string]interface{}{
		"email":               email,
		"full_name":           fullName,
		"password":            password,
		"habeas_data_consent": true,
	}, &registerResp, "")
	assert.Equal(t, http.StatusCreated, resp.StatusCode, "register should return 201")
	require.NotEmpty(t, registerResp.Token, "register must return a JWT token")
	token := registerResp.Token
	userID := registerResp.User.ID

	// ── 2. Login ───────────────────────────────────────────────────────────────
	var loginResp struct {
		Token string `json:"token"`
	}
	resp = postJSON(t, client, srv.URL+"/v1/auth/login", map[string]string{
		"email":    email,
		"password": password,
	}, &loginResp, "")
	assert.Equal(t, http.StatusOK, resp.StatusCode, "login should return 200")
	require.NotEmpty(t, loginResp.Token, "login must return a JWT token")
	token = loginResp.Token // use the login token for subsequent requests

	// ── 3. Add address (required for checkout) ─────────────────────────────────
	var addrResp struct {
		Address struct {
			ID int64 `json:"id"`
		} `json:"address"`
	}
	resp = postJSON(t, client, srv.URL+"/v1/users/me/addresses", map[string]string{
		"full_address": "Calle 100 #15-20, Bogotá",
	}, &addrResp, token)
	require.Equal(t, http.StatusCreated, resp.StatusCode, "add address should return 201")
	addressID := addrResp.Address.ID
	require.NotZero(t, addressID, "address ID must be non-zero")

	// ── 4. Add item to cart (requires a real listing in DB) ────────────────────
	// Find an active listing to add to the cart.
	var listingID int64
	err := db.QueryRowContext(context.Background(),
		`SELECT id FROM listings WHERE is_active = true AND price_cop IS NOT NULL LIMIT 1`,
	).Scan(&listingID)
	if err != nil {
		t.Skip("no active listing in DB — skipping cart/checkout steps")
	}

	addItemURL := fmt.Sprintf("%s/v1/cart/items/%d", srv.URL, listingID)
	req, err := http.NewRequest(http.MethodPut, addItemURL, bytes.NewBufferString(`{"quantity":1}`))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err = client.Do(req)
	require.NoError(t, err)
	resp.Body.Close()
	assert.Equal(t, http.StatusNoContent, resp.StatusCode, "add to cart should return 204")

	// ── 5. Checkout ────────────────────────────────────────────────────────────
	var orderResp struct {
		Order struct {
			ID int64 `json:"id"`
		} `json:"order"`
	}
	resp = postJSON(t, client, srv.URL+"/v1/orders", map[string]interface{}{
		"address_id":     addressID,
		"payment_method": "efectivo",
	}, &orderResp, token)
	assert.Equal(t, http.StatusCreated, resp.StatusCode, "checkout should return 201")
	orderID := orderResp.Order.ID
	require.NotZero(t, orderID, "order ID must be non-zero after checkout")

	// ── 6. Verify order exists in DB ───────────────────────────────────────────
	var dbStatus string
	err = db.QueryRowContext(context.Background(),
		`SELECT status FROM orders WHERE id = $1 AND user_id = $2`,
		orderID, userID,
	).Scan(&dbStatus)
	require.NoError(t, err, "order must exist in DB after checkout")
	assert.Equal(t, "pending_confirmation", dbStatus,
		"new orders must start as pending_confirmation")

	// ── cleanup: delete test data so the test is repeatable ───────────────────
	t.Cleanup(func() {
		db.ExecContext(context.Background(), `DELETE FROM order_items WHERE order_id = $1`, orderID)                                      //nolint:errcheck
		db.ExecContext(context.Background(), `DELETE FROM order_events WHERE order_id = $1`, orderID)                                     //nolint:errcheck
		db.ExecContext(context.Background(), `DELETE FROM orders WHERE id = $1`, orderID)                                                 //nolint:errcheck
		db.ExecContext(context.Background(), `DELETE FROM cart_items WHERE cart_id IN (SELECT id FROM carts WHERE user_id = $1)`, userID) //nolint:errcheck
		db.ExecContext(context.Background(), `DELETE FROM carts WHERE user_id = $1`, userID)                                              //nolint:errcheck
		db.ExecContext(context.Background(), `DELETE FROM addresses WHERE user_id = $1`, userID)                                          //nolint:errcheck
		db.ExecContext(context.Background(), `DELETE FROM users WHERE id = $1`, userID)                                                   //nolint:errcheck
	})
}

// TestCheckoutOutOfZone verifies that a checkout attempt with an address
// outside the delivery zone returns 422 and creates no order.
func TestCheckoutOutOfZone(t *testing.T) {
	db := openTestDB(t)
	srv := buildIntegrationServer(t, db)
	client := srv.Client()

	unique := fmt.Sprintf("ooz_%d", time.Now().UnixNano())
	email := unique + "@example.com"

	// Register
	var registerResp struct {
		Token string `json:"token"`
		User  struct {
			ID int64 `json:"id"`
		} `json:"user"`
	}
	resp := postJSON(t, client, srv.URL+"/v1/auth/register", map[string]interface{}{
		"email":               email,
		"full_name":           "OOZ Test",
		"password":            "securepassword123",
		"habeas_data_consent": true,
	}, &registerResp, "")
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	token := registerResp.Token
	userID := registerResp.User.ID

	// Add an out-of-zone address
	var addrResp struct {
		Address struct {
			ID int64 `json:"id"`
		} `json:"address"`
	}
	resp = postJSON(t, client, srv.URL+"/v1/users/me/addresses", map[string]string{
		"full_address": "Av. Paulista 1578, São Paulo, Brasil",
	}, &addrResp, token)
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	addressID := addrResp.Address.ID

	// Count orders before
	var ordersBefore int
	db.QueryRowContext(context.Background(), //nolint:errcheck
		`SELECT COUNT(*) FROM orders WHERE user_id = $1`, userID,
	).Scan(&ordersBefore)

	// Attempt checkout — should be rejected 422
	var checkoutResp struct {
		Error string `json:"error"`
		Code  string `json:"code"`
	}
	resp = postJSON(t, client, srv.URL+"/v1/orders", map[string]interface{}{
		"address_id":     addressID,
		"payment_method": "efectivo",
	}, &checkoutResp, token)

	assert.Equal(t, http.StatusUnprocessableEntity, resp.StatusCode,
		"out-of-zone checkout must return 422")
	assert.Equal(t, "OUTSIDE_ZONE", checkoutResp.Code,
		"response code must be OUTSIDE_ZONE")

	// Verify no order was created
	var ordersAfter int
	db.QueryRowContext(context.Background(), //nolint:errcheck
		`SELECT COUNT(*) FROM orders WHERE user_id = $1`, userID,
	).Scan(&ordersAfter)
	assert.Equal(t, ordersBefore, ordersAfter,
		"out-of-zone checkout must not create an order row")

	// Cleanup
	t.Cleanup(func() {
		db.ExecContext(context.Background(), `DELETE FROM addresses WHERE user_id = $1`, userID) //nolint:errcheck
		db.ExecContext(context.Background(), `DELETE FROM users WHERE id = $1`, userID)          //nolint:errcheck
	})

}
