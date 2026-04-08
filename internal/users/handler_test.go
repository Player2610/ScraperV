//go:build !integration

package users_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/go-chi/chi/v5"
	"github.com/protou/protou/internal/users"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newTestRouter builds a chi router with only the register endpoint, backed by
// a sqlmock DB.  Validation runs before any DB call, so the mock DB is never
// queried for invalid inputs; for valid inputs it returns an error (simulating
// a downstream failure) so we get a non-400 response.
func newTestRouter(t *testing.T) (*chi.Mux, sqlmock.Sqlmock) {
	t.Helper()

	// JWT_SECRET must be set for auth.IssueToken (called only after validation).
	if os.Getenv("JWT_SECRET") == "" {
		t.Setenv("JWT_SECRET", "test-secret-for-unit-tests")
	}

	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })

	repo := users.NewRepository(db)
	svc := users.NewService(repo)
	h := users.NewHandler(svc)

	r := chi.NewRouter()
	h.RegisterRoutes(r)
	return r, mock
}

// TestRegisterValidation exercises the input-validation layer of POST
// /v1/auth/register without requiring a real database.
func TestRegisterValidation(t *testing.T) {
	cases := []struct {
		name           string
		body           map[string]string
		wantStatus     int
		wantBodyContains string
	}{
		{
			name:             "invalid email",
			body:             map[string]string{"email": "bad", "full_name": "Test User", "password": "longpassword"},
			wantStatus:       http.StatusBadRequest,
			wantBodyContains: "INVALID_EMAIL",
		},
		{
			name:             "password too short",
			body:             map[string]string{"email": "user@example.com", "full_name": "Test User", "password": "short"},
			wantStatus:       http.StatusBadRequest,
			wantBodyContains: "PASSWORD_TOO_SHORT",
		},
		{
			name: "valid input passes validation",
			body: map[string]string{
				"email":     "user@example.com",
				"full_name": "Test User",
				"password":  "validpass123",
			},
			// Validation passes; the mock DB will cause a downstream error.
			// We only assert the status is NOT 400 (validation did not reject it).
			wantStatus:       -1, // sentinel: assert != 400
			wantBodyContains: "",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r, mock := newTestRouter(t)

			// For the valid-input case, the handler will try to INSERT a user.
			// We let the mock return an error so the test does not need a real DB.
			if tc.wantStatus == -1 {
				mock.ExpectQuery("INSERT INTO users").
					WillReturnError(assert.AnError)
			}

			payload, err := json.Marshal(tc.body)
			require.NoError(t, err)

			req := httptest.NewRequest(http.MethodPost, "/v1/auth/register",
				bytes.NewReader(payload))
			req.Header.Set("Content-Type", "application/json")
			rr := httptest.NewRecorder()

			r.ServeHTTP(rr, req)

			if tc.wantStatus == -1 {
				assert.NotEqual(t, http.StatusBadRequest, rr.Code,
					"valid payload must not be rejected by validation (got %d)", rr.Code)
			} else {
				assert.Equal(t, tc.wantStatus, rr.Code)
				assert.Contains(t, rr.Body.String(), tc.wantBodyContains)
			}
		})
	}
}
