package auth

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
)

type contextKey string

const (
	claimsKey contextKey = "auth_claims"
)

// RequireStudent validates a Bearer JWT and ensures role == "student".
// On failure returns 401 or 403.
func RequireStudent(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		claims, err := extractBearer(r)
		if err != nil {
			respondUnauthorized(w)
			return
		}
		if claims.Role != "student" {
			respondForbidden(w)
			return
		}
		ctx := context.WithValue(r.Context(), claimsKey, claims)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// RequireOperator validates a Bearer JWT and ensures role == "operator".
// On failure returns 401 or 403.
func RequireOperator(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		claims, err := extractBearer(r)
		if err != nil {
			respondUnauthorized(w)
			return
		}
		if claims.Role != "operator" {
			respondForbidden(w)
			return
		}
		ctx := context.WithValue(r.Context(), claimsKey, claims)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// StudentFromContext extracts Claims from a request context set by RequireStudent.
func StudentFromContext(ctx context.Context) (*Claims, bool) {
	c, ok := ctx.Value(claimsKey).(*Claims)
	if !ok || c == nil {
		return nil, false
	}
	if c.Role != "student" {
		return nil, false
	}
	return c, true
}

// OperatorFromContext extracts Claims from a request context set by RequireOperator.
func OperatorFromContext(ctx context.Context) (*Claims, bool) {
	c, ok := ctx.Value(claimsKey).(*Claims)
	if !ok || c == nil {
		return nil, false
	}
	if c.Role != "operator" {
		return nil, false
	}
	return c, true
}

// ─── helpers ─────────────────────────────────────────────────────────────────

func extractBearer(r *http.Request) (*Claims, error) {
	hdr := r.Header.Get("Authorization")
	if hdr == "" {
		return nil, errNoToken
	}
	parts := strings.SplitN(hdr, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return nil, errNoToken
	}
	return ValidateToken(parts[1])
}

var errNoToken = &authError{"missing or invalid authorization header"}

type authError struct{ msg string }

func (e *authError) Error() string { return e.msg }

func respondUnauthorized(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnauthorized)
	json.NewEncoder(w).Encode(map[string]string{ //nolint:errcheck
		"error": "unauthorized",
		"code":  "UNAUTHORIZED",
	})
}

func respondForbidden(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusForbidden)
	json.NewEncoder(w).Encode(map[string]string{ //nolint:errcheck
		"error": "forbidden",
		"code":  "FORBIDDEN",
	})
}
