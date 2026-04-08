package platform

import (
	"context"
	"database/sql"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
)

// loggerKey is used as context key for the per-request slog.Logger.
type loggerKey struct{}

const healthDBTimeout = 3 * time.Second

// NewServer creates and configures a chi router with standard middleware and a
// health check endpoint. CORS is configured to allow requests from the
// Cloudflare Pages origin. db is used by the health check endpoint to verify
// database connectivity.
func NewServer(cfg Config, db *sql.DB) *chi.Mux {
	r := chi.NewRouter()

	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(maxBodySize(1 << 20)) // 1 MB body size limit

	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{cfg.CORSOrigin},
		AllowedMethods:   []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-Request-ID"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	r.Use(securityHeaders)
	r.Use(withRequestID)

	r.Get("/health", healthHandler(db))

	return r
}

// securityHeaders adds standard security-related HTTP response headers.
func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Content-Security-Policy-Report-Only", "default-src 'self'")

		w.Header().Set("Strict-Transport-Security", "max-age=63072000; includeSubDomains")

		next.ServeHTTP(w, r)
	})
}

// LoggerFromContext returns the per-request slog.Logger stored by withRequestID,
// or the default logger if none is found.
func LoggerFromContext(ctx context.Context) *slog.Logger {
	if l, ok := ctx.Value(loggerKey{}).(*slog.Logger); ok && l != nil {
		return l
	}
	return slog.Default()
}

// withRequestID extracts the chi request ID and attaches a slog.Logger with
// the request_id field to the request context.
func withRequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reqID := middleware.GetReqID(r.Context())
		logger := slog.Default().With("request_id", reqID)
		ctx := context.WithValue(r.Context(), loggerKey{}, logger)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// maxBodySize limits the request body to n bytes.
func maxBodySize(n int64) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			r.Body = http.MaxBytesReader(w, r.Body, n)
			next.ServeHTTP(w, r)
		})
	}
}

// healthHandler returns an http.HandlerFunc that checks database connectivity.
// It responds 200 {"status":"ok"} when the database is reachable, or
// 503 {"status":"error","message":"db unavailable"} when not.
func healthHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), healthDBTimeout)
		defer cancel()

		w.Header().Set("Content-Type", "application/json")

		if err := db.PingContext(ctx); err != nil {
			slog.Default().Error("health check db ping failed", "error", err)
			w.WriteHeader(http.StatusServiceUnavailable)
			json.NewEncoder(w).Encode(map[string]string{ //nolint:errcheck
				"status": "degraded",
				"db":     "error",
				"error":  err.Error(),
			})
			return
		}

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"status": "ok", "db": "ok"}) //nolint:errcheck
	}
}
