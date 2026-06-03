// Package operator implements the operator panel HTTP handlers.
package operator

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/httprate"
	"golang.org/x/crypto/bcrypt"

	"github.com/protou/protou/internal/auth"
)

// ctxKey is a private context key type for this package.
type ctxKey string

const operatorIDKey ctxKey = "operator_id"

// OperatorIDFromContext returns the operator_id set by RequireOperatorSession.
func OperatorIDFromContext(ctx context.Context) (int64, bool) {
	id, ok := ctx.Value(operatorIDKey).(int64)
	return id, ok
}

// AuthHandler handles operator authentication.
type AuthHandler struct {
	db *sql.DB
}

// NewAuthHandler creates an AuthHandler.
func NewAuthHandler(db *sql.DB) *AuthHandler {
	return &AuthHandler{db: db}
}

// RegisterRoutes mounts operator auth routes.
func (h *AuthHandler) RegisterRoutes(r chi.Router) {
	// Login is rate-limited per IP (stricter than student auth: 5/min).
	loginLimiter := httprate.Limit(5, time.Minute,
		httprate.WithKeyByIP(),
		httprate.WithLimitHandler(func(w http.ResponseWriter, r *http.Request) {
			respondError(w, http.StatusTooManyRequests, "too many requests, try again later", "RATE_LIMITED")
		}),
	)
	r.With(loginLimiter).Post("/v1/operator/auth/login", h.login)
	r.Post("/v1/operator/auth/logout", h.logout)
}

// POST /v1/operator/auth/login
func (h *AuthHandler) login(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body", "INVALID_BODY")
		return
	}
	if req.Email == "" || req.Password == "" {
		respondError(w, http.StatusBadRequest, "email and password are required", "MISSING_FIELDS")
		return
	}

	// Look up user by email, join operators table to get operator_id
	const userQ = `
		SELECT u.id, u.password_hash, o.id AS operator_id
		FROM users u
		INNER JOIN operators o ON o.user_id = u.id
		WHERE u.email = $1 AND u.is_active = true
	`
	var userID int64
	var passwordHash string
	var operatorID int64
	err := h.db.QueryRowContext(r.Context(), userQ, req.Email).Scan(&userID, &passwordHash, &operatorID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			respondError(w, http.StatusUnauthorized, "credenciales inválidas", "INVALID_CREDENTIALS")
			return
		}
		slog.Error("operator auth: lookup user", "error", err)
		respondError(w, http.StatusInternalServerError, "login failed", "INTERNAL")
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(req.Password)); err != nil {
		respondError(w, http.StatusUnauthorized, "credenciales inválidas", "INVALID_CREDENTIALS")
		return
	}

	token, err := auth.CreateSession(h.db, operatorID)
	if err != nil {
		slog.Error("operator auth: create session", "error", err)
		respondError(w, http.StatusInternalServerError, "login failed", "INTERNAL")
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "session",
		Value:    token,
		Path:     "/v1/operator",
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteStrictMode,
		Expires:  time.Now().Add(8 * time.Hour),
	})

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"operator_id": operatorID,
	})
}

// POST /v1/operator/auth/logout
func (h *AuthHandler) logout(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie("session")
	if err == nil && cookie.Value != "" {
		const delQ = `DELETE FROM operator_sessions WHERE token = $1`
		if _, dbErr := h.db.ExecContext(r.Context(), delQ, cookie.Value); dbErr != nil {
			slog.Error("operator auth: delete session", "error", dbErr)
		}
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "session",
		Value:    "",
		Path:     "/v1/operator",
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   -1,
	})

	w.WriteHeader(http.StatusNoContent)
}

// RequireOperatorSession is middleware that validates operator session cookies.
// It puts the operator_id into context.
func RequireOperatorSession(db *sql.DB) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			cookie, err := r.Cookie("session")
			if err != nil || cookie.Value == "" {
				respondError(w, http.StatusUnauthorized, "unauthorized", "UNAUTHORIZED")
				return
			}

			operatorID, err := auth.ValidateSession(db, cookie.Value)
			if err != nil {
				respondError(w, http.StatusUnauthorized, "session expired or invalid", "UNAUTHORIZED")
				return
			}

			ctx := context.WithValue(r.Context(), operatorIDKey, operatorID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// ─── helpers ─────────────────────────────────────────────────────────────────

func respondJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v) //nolint:errcheck
}

func respondError(w http.ResponseWriter, status int, msg, code string) {
	respondJSON(w, status, map[string]string{
		"error": msg,
		"code":  code,
	})
}
