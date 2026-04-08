//go:build !integration

package platform

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSecurityHeaders verifies that the securityHeaders middleware sets the
// expected security-related HTTP response headers.
func TestSecurityHeaders_SetsRequiredHeaders(t *testing.T) {
	handler := securityHeaders(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	assert.Equal(t, "DENY", rr.Header().Get("X-Frame-Options"),
		"X-Frame-Options must be DENY")
	assert.Equal(t, "nosniff", rr.Header().Get("X-Content-Type-Options"),
		"X-Content-Type-Options must be nosniff")
	assert.NotEmpty(t, rr.Header().Get("Content-Security-Policy-Report-Only"),
		"Content-Security-Policy-Report-Only must be set")
}

// TestSecurityHeaders_HSTSAlwaysSet verifies that HSTS is always set by the
// middleware regardless of transport.  The middleware sets it unconditionally
// because Cloud Run sits behind a TLS-terminating load balancer and requests
// always arrive over HTTPS at the infrastructure level.
func TestSecurityHeaders_HSTSAlwaysSet(t *testing.T) {
	handler := securityHeaders(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	// No TLS, no forwarded-proto header — plain HTTP at test level
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	hsts := rr.Header().Get("Strict-Transport-Security")
	assert.NotEmpty(t, hsts, "HSTS must always be set (middleware is unconditional)")
	assert.Contains(t, hsts, "max-age=", "HSTS value must include max-age")
}

// TestSecurityHeaders_HSTSSetWhenForwardedHTTPS verifies that HSTS IS sent when
// the request carries X-Forwarded-Proto: https (Cloud Run behind a load balancer).
func TestSecurityHeaders_HSTSSetWhenForwardedHTTPS(t *testing.T) {
	handler := securityHeaders(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	req.Header.Set("X-Forwarded-Proto", "https")

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	hsts := rr.Header().Get("Strict-Transport-Security")
	assert.NotEmpty(t, hsts, "HSTS must be set when X-Forwarded-Proto is https")
	assert.Contains(t, hsts, "max-age=", "HSTS value must include max-age")
	assert.Contains(t, hsts, "includeSubDomains", "HSTS value must include includeSubDomains")
}

// TestSecurityHeaders_DoesNotInterfereWithDownstream verifies that the middleware
// passes the request through and the downstream handler's status code is preserved.
func TestSecurityHeaders_DoesNotInterfereWithDownstream(t *testing.T) {
	handler := securityHeaders(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTeapot) // arbitrary non-200 status
	}))

	req := httptest.NewRequest(http.MethodGet, "/anything", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusTeapot, rr.Code,
		"security headers middleware must not alter downstream response status")
}

// TestSecurityHeaders verifies that all four required security headers are set
// by the securityHeaders middleware.
func TestSecurityHeaders(t *testing.T) {
	handler := securityHeaders(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	req.Header.Set("X-Forwarded-Proto", "https") // trigger HSTS
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	assert.Equal(t, "DENY", rr.Header().Get("X-Frame-Options"),
		"X-Frame-Options must be DENY")

	assert.Equal(t, "nosniff", rr.Header().Get("X-Content-Type-Options"),
		"X-Content-Type-Options must be nosniff")

	hsts := rr.Header().Get("Strict-Transport-Security")
	assert.NotEmpty(t, hsts, "Strict-Transport-Security must be set")
	assert.Contains(t, hsts, "max-age=63072000",
		"Strict-Transport-Security must contain max-age=63072000")

	csp := rr.Header().Get("Content-Security-Policy-Report-Only")
	assert.NotEmpty(t, csp, "Content-Security-Policy-Report-Only must be set")
	assert.Contains(t, csp, "default-src 'self'",
		"Content-Security-Policy-Report-Only must contain default-src 'self'")
}

// ─── healthHandler ────────────────────────────────────────────────────────────

// TestHealthHandler_DBOk_Returns200 verifies that the health endpoint returns
// 200 {"status":"ok","db":"ok"} when the database is reachable.
func TestHealthHandler_DBOk_Returns200(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.MonitorPingsOption(true))
	require.NoError(t, err)
	defer db.Close()

	// Expect one ping — no error means DB is healthy.
	mock.ExpectPing()

	handler := healthHandler(db)
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code, "health with reachable DB must return 200")
	assert.Equal(t, "application/json", rr.Header().Get("Content-Type"))

	var body map[string]string
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&body))
	assert.Equal(t, "ok", body["status"], "status must be ok when DB is reachable")
	assert.Equal(t, "ok", body["db"], "db must be ok when DB is reachable")

	assert.NoError(t, mock.ExpectationsWereMet())
}

// TestHealthHandler_DBDown_Returns503 verifies that the health endpoint returns
// 503 {"status":"degraded","db":"error",...} when the database is unreachable.
func TestHealthHandler_DBDown_Returns503(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.MonitorPingsOption(true))
	require.NoError(t, err)
	defer db.Close()

	// Simulate a DB failure on ping.
	mock.ExpectPing().WillReturnError(assert.AnError)

	handler := healthHandler(db)
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusServiceUnavailable, rr.Code,
		"health with unreachable DB must return 503")
	assert.Equal(t, "application/json", rr.Header().Get("Content-Type"))

	var body map[string]string
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&body))
	assert.Equal(t, "degraded", body["status"],
		"status must be degraded when DB ping fails")
	assert.Equal(t, "error", body["db"],
		"db field must be error when DB ping fails")
	assert.NotEmpty(t, body["error"],
		"error field must be present when DB ping fails")

	assert.NoError(t, mock.ExpectationsWereMet())
}
