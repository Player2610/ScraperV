//go:build integration

package operator_test

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
	"github.com/protou/protou/internal/notifications"
	"github.com/protou/protou/internal/operator"
	"github.com/protou/protou/internal/platform"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"
)

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

// buildOperatorServer creates a real test HTTP server with the operator handler.
func buildOperatorServer(t *testing.T, db *sql.DB) *httptest.Server {
	t.Helper()

	cfg := platform.Config{
		CORSOrigin: "http://localhost",
	}
	if os.Getenv("JWT_SECRET") == "" {
		t.Setenv("JWT_SECRET", "integration-test-jwt-secret")
	}

	router := platform.NewServer(cfg, db)
	notifSvc := notifications.NewService(db)
	h := operator.NewHandler(db, notifSvc)
	h.RegisterRoutes(router)

	srv := httptest.NewServer(router)
	t.Cleanup(srv.Close)
	return srv
}

// doJSON sends a JSON request and optionally decodes the response body.
func doJSON(t *testing.T, client *http.Client, method, url string, payload interface{}, dest interface{}, cookies []*http.Cookie) *http.Response {
	t.Helper()
	var body io.Reader
	if payload != nil {
		b, err := json.Marshal(payload)
		require.NoError(t, err)
		body = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, url, body)
	require.NoError(t, err)
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	for _, c := range cookies {
		req.AddCookie(c)
	}
	resp, err := client.Do(req)
	require.NoError(t, err)
	if dest != nil {
		raw, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err := json.Unmarshal(raw, dest); err != nil {
			t.Logf("response body (could not decode): %s", raw)
		}
	}
	return resp
}

// createTestOperator inserts a user + operator row for testing and returns the
// credentials.  The caller is responsible for cleanup.
func createTestOperator(t *testing.T, db *sql.DB) (email, password string, operatorID int64, userID int64) {
	t.Helper()
	password = "operatorpass123"
	email = fmt.Sprintf("op_e2e_%d@protou.test", time.Now().UnixNano())

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.MinCost)
	require.NoError(t, err)

	err = db.QueryRowContext(context.Background(), `
		INSERT INTO users (email, name, password_hash, is_active)
		VALUES ($1, 'E2E Operator', $2, true)
		RETURNING id
	`, email, string(hash)).Scan(&userID)
	require.NoError(t, err, "insert test operator user")

	err = db.QueryRowContext(context.Background(), `
		INSERT INTO operators (user_id) VALUES ($1) RETURNING id
	`, userID).Scan(&operatorID)
	require.NoError(t, err, "insert test operator row")

	return email, password, operatorID, userID
}

// createTestOrder inserts a minimal pending_confirmation order for testing.
func createTestOrder(t *testing.T, db *sql.DB, studentID int64) int64 {
	t.Helper()
	addrJSON := `{"full_address":"Calle 100 #15-20, Bogotá"}`
	var orderID int64
	err := db.QueryRowContext(context.Background(), `
		INSERT INTO orders (
			user_id, status, delivery_address_snapshot,
			subtotal_cop, delivery_fee_cop, total_cop, payment_method
		) VALUES (
			$1, 'pending_confirmation', $2::jsonb,
			50000, 5000, 55000, 'efectivo'
		) RETURNING id
	`, studentID, addrJSON).Scan(&orderID)
	require.NoError(t, err, "insert test order")
	return orderID
}

// createTestStudent inserts a minimal student user for test order ownership.
func createTestStudent(t *testing.T, db *sql.DB) int64 {
	t.Helper()
	email := fmt.Sprintf("student_e2e_%d@protou.test", time.Now().UnixNano())
	var id int64
	err := db.QueryRowContext(context.Background(), `
		INSERT INTO users (email, name, password_hash, is_active)
		VALUES ($1, 'E2E Student', '', true)
		RETURNING id
	`, email).Scan(&id)
	require.NoError(t, err, "insert test student")
	return id
}

// TestOperatorE2E exercises the full operator happy-path:
//
//   operator login → GET /v1/operator/orders → confirm order → mark payment
//
// The test skips when DATABASE_URL is not set.
func TestOperatorE2E(t *testing.T) {
	db := openTestDB(t)
	srv := buildOperatorServer(t, db)
	client := srv.Client()

	// Seed test data
	opEmail, opPassword, _, opUserID := createTestOperator(t, db)
	studentID := createTestStudent(t, db)
	orderID := createTestOrder(t, db, studentID)

	t.Cleanup(func() {
		db.ExecContext(context.Background(), `DELETE FROM payment_records WHERE order_id = $1`, orderID)                                                   //nolint:errcheck
		db.ExecContext(context.Background(), `DELETE FROM order_events WHERE order_id = $1`, orderID)                                                      //nolint:errcheck
		db.ExecContext(context.Background(), `DELETE FROM orders WHERE id = $1`, orderID)                                                                  //nolint:errcheck
		db.ExecContext(context.Background(), `DELETE FROM users WHERE id = $1`, studentID)                                                                 //nolint:errcheck
		db.ExecContext(context.Background(), `DELETE FROM operators WHERE user_id = $1`, opUserID)                                                         //nolint:errcheck
		db.ExecContext(context.Background(), `DELETE FROM operator_sessions WHERE operator_id IN (SELECT id FROM operators WHERE user_id = $1)`, opUserID) //nolint:errcheck
		db.ExecContext(context.Background(), `DELETE FROM users WHERE id = $1`, opUserID)                                                                  //nolint:errcheck
	})

	// ── 1. Operator login ──────────────────────────────────────────────────────
	var loginResp struct {
		OperatorID int64 `json:"operator_id"`
	}
	resp := doJSON(t, client, http.MethodPost, srv.URL+"/v1/operator/auth/login",
		map[string]string{"email": opEmail, "password": opPassword},
		&loginResp, nil)
	assert.Equal(t, http.StatusOK, resp.StatusCode, "operator login should return 200")
	require.NotZero(t, loginResp.OperatorID, "login must return an operator_id")

	// Extract session cookie
	var sessionCookie *http.Cookie
	for _, c := range resp.Cookies() {
		if c.Name == "session" {
			sessionCookie = c
			break
		}
	}
	require.NotNil(t, sessionCookie, "login must set a session cookie")

	cookies := []*http.Cookie{sessionCookie}

	// ── 2. GET /v1/operator/orders ─────────────────────────────────────────────
	var listResp struct {
		Orders []struct {
			ID     int64  `json:"id"`
			Status string `json:"status"`
		} `json:"orders"`
	}
	resp = doJSON(t, client, http.MethodGet, srv.URL+"/v1/operator/orders",
		nil, &listResp, cookies)
	assert.Equal(t, http.StatusOK, resp.StatusCode, "list orders should return 200")

	// ── 3. Confirm the test order ──────────────────────────────────────────────
	var confirmResp struct {
		Status string `json:"status"`
	}
	confirmURL := fmt.Sprintf("%s/v1/operator/orders/%d/confirm", srv.URL, orderID)
	resp = doJSON(t, client, http.MethodPost, confirmURL, map[string]string{},
		&confirmResp, cookies)
	assert.Equal(t, http.StatusOK, resp.StatusCode, "confirm order should return 200")
	assert.Equal(t, "confirmed", confirmResp.Status, "confirm response must carry status=confirmed")

	// ── 4. Transition order through to delivered so we can record payment ──────
	// purchasing
	transitionURL := fmt.Sprintf("%s/v1/operator/orders/%d/transition", srv.URL, orderID)
	doJSON(t, client, http.MethodPost, transitionURL,
		map[string]string{"to": "purchasing"}, nil, cookies)
	// in_delivery
	doJSON(t, client, http.MethodPost, transitionURL,
		map[string]string{"to": "in_delivery"}, nil, cookies)
	// delivered
	doJSON(t, client, http.MethodPost, transitionURL,
		map[string]string{"to": "delivered"}, nil, cookies)

	// ── 5. Record payment ──────────────────────────────────────────────────────
	paymentURL := fmt.Sprintf("%s/v1/operator/orders/%d/payment", srv.URL, orderID)
	resp = doJSON(t, client, http.MethodPost, paymentURL,
		map[string]interface{}{"method": "efectivo", "amount_cop": 55000},
		nil, cookies)
	assert.Equal(t, http.StatusCreated, resp.StatusCode, "record payment should return 201")

	// ── 6. Verify order has status "delivered" (payment is a separate record) ──
	var dbStatus string
	err := db.QueryRowContext(context.Background(),
		`SELECT status FROM orders WHERE id = $1`, orderID,
	).Scan(&dbStatus)
	require.NoError(t, err, "order must still exist in DB")
	assert.Equal(t, "delivered", dbStatus,
		"order status must be delivered after payment recording")

	// ── 7. Verify payment record was created ───────────────────────────────────
	var paymentCount int
	db.QueryRowContext(context.Background(), //nolint:errcheck
		`SELECT COUNT(*) FROM payment_records WHERE order_id = $1`, orderID,
	).Scan(&paymentCount)
	assert.Equal(t, 1, paymentCount, "one payment record must exist after recording payment")
}
