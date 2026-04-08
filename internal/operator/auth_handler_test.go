//go:build !integration

package operator

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"
)

// ─── OperatorIDFromContext ────────────────────────────────────────────────────

// TestOperatorIDFromContext is a pure function test — no DB required.
func TestOperatorIDFromContext_ValuePresent(t *testing.T) {
	ctx := context.WithValue(context.Background(), operatorIDKey, int64(42))

	id, ok := OperatorIDFromContext(ctx)

	assert.True(t, ok)
	assert.Equal(t, int64(42), id)
}

func TestOperatorIDFromContext_ValueAbsent(t *testing.T) {
	id, ok := OperatorIDFromContext(context.Background())

	assert.False(t, ok)
	assert.Equal(t, int64(0), id)
}

func TestOperatorIDFromContext_WrongType(t *testing.T) {
	// If someone puts a non-int64 value under the key, ok should be false.
	ctx := context.WithValue(context.Background(), operatorIDKey, "not-an-int64")

	id, ok := OperatorIDFromContext(ctx)

	assert.False(t, ok)
	assert.Equal(t, int64(0), id)
}

// ─── login — input validation (no DB reached) ────────────────────────────────

func TestLogin_MissingEmail_Returns400(t *testing.T) {
	h := &AuthHandler{db: nil}

	reqBody := map[string]string{"email": "", "password": "secret"}
	body, err := json.Marshal(reqBody)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/v1/operator/auth/login", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.login(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	var resp map[string]string
	require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
	assert.Equal(t, "MISSING_FIELDS", resp["code"])
}

func TestLogin_MissingPassword_Returns400(t *testing.T) {
	h := &AuthHandler{db: nil}

	reqBody := map[string]string{"email": "op@protou.co", "password": ""}
	body, err := json.Marshal(reqBody)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/v1/operator/auth/login", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.login(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	var resp map[string]string
	require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
	assert.Equal(t, "MISSING_FIELDS", resp["code"])
}

func TestLogin_InvalidJSON_Returns400(t *testing.T) {
	h := &AuthHandler{db: nil}

	req := httptest.NewRequest(http.MethodPost, "/v1/operator/auth/login",
		bytes.NewReader([]byte("{bad json")))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.login(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	var resp map[string]string
	require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
	assert.Equal(t, "INVALID_BODY", resp["code"])
}

// ─── logout — cookie clearing (no DB side-effects verified) ──────────────────

// TestLogout_NoCookie_Returns204 verifies that logout always clears the cookie and
// responds 204 even when no session cookie is present (graceful no-op path).
func TestLogout_NoCookie_Returns204(t *testing.T) {
	h := &AuthHandler{db: nil}

	req := httptest.NewRequest(http.MethodPost, "/v1/operator/auth/logout", nil)
	w := httptest.NewRecorder()

	h.logout(w, req)

	assert.Equal(t, http.StatusNoContent, w.Code)
}

// TestLogout_ClearsCookie verifies the Set-Cookie header is present and the cookie
// attributes are correct: HttpOnly, Secure, SameSite=Strict, MaxAge=-1, empty value.
func TestLogout_ClearsCookie(t *testing.T) {
	h := &AuthHandler{db: nil}

	req := httptest.NewRequest(http.MethodPost, "/v1/operator/auth/logout", nil)
	w := httptest.NewRecorder()

	h.logout(w, req)

	resp := w.Result()
	cookies := resp.Cookies()
	require.NotEmpty(t, cookies, "logout should set a clearing cookie")

	var sessionCookie *http.Cookie
	for _, c := range cookies {
		if c.Name == "session" {
			sessionCookie = c
			break
		}
	}
	require.NotNil(t, sessionCookie, "logout must set a 'session' cookie")

	assert.Equal(t, "", sessionCookie.Value, "clearing cookie value must be empty")
	assert.True(t, sessionCookie.HttpOnly, "cookie must be HttpOnly")
	assert.True(t, sessionCookie.Secure, "cookie must be Secure")
	assert.Equal(t, http.SameSiteStrictMode, sessionCookie.SameSite, "cookie must be SameSite=Strict")
	assert.Equal(t, -1, sessionCookie.MaxAge, "cookie MaxAge must be -1 to expire it")
	assert.Equal(t, "/v1/operator", sessionCookie.Path, "cookie path must be /v1/operator")
}

// ─── RequireOperatorSession middleware — no session cookie ────────────────────

// TestRequireOperatorSession_NoCookie_Returns401 tests the middleware without a DB.
// When no cookie is present, the middleware rejects before any DB call.
func TestRequireOperatorSession_NoCookie_Returns401(t *testing.T) {
	nextCalled := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
		w.WriteHeader(http.StatusOK)
	})

	mw := RequireOperatorSession(nil) // nil DB — safe because we never reach DB call
	handler := mw(next)

	req := httptest.NewRequest(http.MethodGet, "/v1/operator/orders", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
	assert.False(t, nextCalled, "next handler must not be called when cookie is absent")

	var resp map[string]string
	require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
	assert.Equal(t, "UNAUTHORIZED", resp["code"])
}

// TestRequireOperatorSession_EmptyToken_Returns401 verifies that an empty cookie value
// is rejected before any DB call.
func TestRequireOperatorSession_EmptyToken_Returns401(t *testing.T) {
	nextCalled := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
	})

	mw := RequireOperatorSession(nil)
	handler := mw(next)

	req := httptest.NewRequest(http.MethodGet, "/v1/operator/orders", nil)
	req.AddCookie(&http.Cookie{Name: "session", Value: ""})
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
	assert.False(t, nextCalled)
}

// ─── DB-dependent paths (documented) ─────────────────────────────────────────
//
// The following login/logout paths require a real DB and are integration-test only:
//
//   - login happy path: QueryRowContext (users+operators join) → bcrypt.CompareHashAndPassword
//     → auth.CreateSession → SetCookie with actual token
//   - login with invalid credentials: QueryRowContext returns sql.ErrNoRows → 401
//   - logout with valid cookie: ExecContext to DELETE operator_sessions row
//   - RequireOperatorSession with valid token: auth.ValidateSession → DB query

// ─── DB-mocked paths via go-sqlmock ──────────────────────────────────────────

func TestLogin_ValidCredentials_Returns200(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	// Pre-hash a known password — bcrypt cost 4 is fast in tests.
	hash, err := bcrypt.GenerateFromPassword([]byte("correctpassword"), 4)
	require.NoError(t, err)

	// Expect the user+operator lookup query.
	rows := sqlmock.NewRows([]string{"id", "password_hash", "operator_id"}).
		AddRow(int64(10), string(hash), int64(99))
	mock.ExpectQuery(`SELECT u\.id, u\.password_hash`).
		WithArgs("op@protou.co").
		WillReturnRows(rows)

	// Expect the INSERT into operator_sessions from auth.CreateSession.
	mock.ExpectExec(`INSERT INTO operator_sessions`).
		WillReturnResult(sqlmock.NewResult(1, 1))

	h := &AuthHandler{db: db}

	reqBody := map[string]string{"email": "op@protou.co", "password": "correctpassword"}
	body, err := json.Marshal(reqBody)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/v1/operator/auth/login", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.login(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	// Verify Set-Cookie header is present with a non-empty session token.
	resp := w.Result()
	var sessionCookie *http.Cookie
	for _, c := range resp.Cookies() {
		if c.Name == "session" {
			sessionCookie = c
			break
		}
	}
	require.NotNil(t, sessionCookie, "login must set a session cookie")
	assert.NotEmpty(t, sessionCookie.Value, "session cookie value must not be empty")
	assert.True(t, sessionCookie.HttpOnly)
	assert.True(t, sessionCookie.Secure)

	// Verify JSON body contains operator_id.
	var respBody map[string]interface{}
	require.NoError(t, json.NewDecoder(w.Body).Decode(&respBody))
	assert.Equal(t, float64(99), respBody["operator_id"])

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestLogin_WrongPassword_Returns401(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	hash, err := bcrypt.GenerateFromPassword([]byte("correctpassword"), 4)
	require.NoError(t, err)

	rows := sqlmock.NewRows([]string{"id", "password_hash", "operator_id"}).
		AddRow(int64(10), string(hash), int64(99))
	mock.ExpectQuery(`SELECT u\.id, u\.password_hash`).
		WithArgs("op@protou.co").
		WillReturnRows(rows)

	h := &AuthHandler{db: db}

	reqBody := map[string]string{"email": "op@protou.co", "password": "wrongpassword"}
	body, err := json.Marshal(reqBody)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/v1/operator/auth/login", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.login(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)

	var resp map[string]string
	require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
	assert.Equal(t, "INVALID_CREDENTIALS", resp["code"])

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestLogin_UnknownEmail_Returns401(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	mock.ExpectQuery(`SELECT u\.id, u\.password_hash`).
		WithArgs("nobody@protou.co").
		WillReturnError(sql.ErrNoRows)

	h := &AuthHandler{db: db}

	reqBody := map[string]string{"email": "nobody@protou.co", "password": "anypassword"}
	body, err := json.Marshal(reqBody)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/v1/operator/auth/login", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.login(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)

	var resp map[string]string
	require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
	assert.Equal(t, "INVALID_CREDENTIALS", resp["code"])

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestLogin_MissingFields_Returns400(t *testing.T) {
	// Both email and password missing (empty JSON object).
	h := &AuthHandler{db: nil}

	reqBody := map[string]string{}
	body, err := json.Marshal(reqBody)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/v1/operator/auth/login", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.login(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var resp map[string]string
	require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
	assert.Equal(t, "MISSING_FIELDS", resp["code"])
}

func TestLogout_WithValidSession_Returns204AndClearsCookie(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	const token = "abc123validtoken"

	// Expect the DELETE from operator_sessions.
	mock.ExpectExec(`DELETE FROM operator_sessions WHERE token`).
		WithArgs(token).
		WillReturnResult(sqlmock.NewResult(0, 1))

	h := &AuthHandler{db: db}

	req := httptest.NewRequest(http.MethodPost, "/v1/operator/auth/logout", nil)
	req.AddCookie(&http.Cookie{Name: "session", Value: token})
	w := httptest.NewRecorder()

	h.logout(w, req)

	assert.Equal(t, http.StatusNoContent, w.Code)

	// Cookie must be cleared (MaxAge = -1, empty value).
	resp := w.Result()
	var sessionCookie *http.Cookie
	for _, c := range resp.Cookies() {
		if c.Name == "session" {
			sessionCookie = c
			break
		}
	}
	require.NotNil(t, sessionCookie, "logout must set a clearing Set-Cookie header")
	assert.Equal(t, "", sessionCookie.Value)
	assert.Equal(t, -1, sessionCookie.MaxAge)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestLogout_WithoutSession_Returns204(t *testing.T) {
	// logout is idempotent: no cookie → still 204, no DB call expected.
	h := &AuthHandler{db: nil}

	req := httptest.NewRequest(http.MethodPost, "/v1/operator/auth/logout", nil)
	w := httptest.NewRecorder()

	h.logout(w, req)

	assert.Equal(t, http.StatusNoContent, w.Code)
}
