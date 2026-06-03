package users

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"regexp"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/httprate"

	"github.com/protou/protou/internal/auth"
)

// emailRe is a simplified RFC5322 email pattern sufficient for registration.
var emailRe = regexp.MustCompile(`^[^@\s]+@[^@\s]+\.[^@\s]+$`)

// Handler holds HTTP handlers for the users domain.
type Handler struct {
	svc *Service
}

// NewHandler creates a new users Handler.
func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// RegisterRoutes mounts user/auth routes onto r.
func (h *Handler) RegisterRoutes(r chi.Router) {
	// Public — rate limited per IP
	rateLimiter := httprate.Limit(10, time.Minute,
		httprate.WithKeyByIP(),
		httprate.WithLimitHandler(func(w http.ResponseWriter, r *http.Request) {
			respondError(w, http.StatusTooManyRequests, "too many requests, try again later", "RATE_LIMITED")
		}),
	)
	r.With(rateLimiter).Post("/v1/auth/register", h.register)
	r.With(rateLimiter).Post("/v1/auth/login", h.login)

	// Protected (student)
	r.Group(func(r chi.Router) {
		r.Use(auth.RequireStudent)
		r.Get("/v1/users/me", h.getMe)
		r.Delete("/v1/users/me", h.deleteMe)
		r.Get("/v1/users/me/addresses", h.listAddresses)
		r.Post("/v1/users/me/addresses", h.addAddress)
		r.Delete("/v1/users/me/addresses/{id}", h.deleteAddress)
	})
}

// POST /v1/auth/register
func (h *Handler) register(w http.ResponseWriter, r *http.Request) {
	var req RegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body", "INVALID_BODY")
		return
	}
	if req.Email == "" || req.Password == "" || req.FullName == "" {
		respondError(w, http.StatusBadRequest, "email, full_name, and password are required", "MISSING_FIELDS")
		return
	}
	if !emailRe.MatchString(req.Email) {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "INVALID_EMAIL"})
		return
	}
	if len(req.Password) < 8 {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "PASSWORD_TOO_SHORT"})
		return
	}

	user, token, err := h.svc.Register(r.Context(), req)
	if err != nil {
		if errors.Is(err, ErrDuplicateEmail) {
			respondError(w, http.StatusConflict, "email already registered", "DUPLICATE_EMAIL")
			return
		}
		respondError(w, http.StatusInternalServerError, "registration failed", "INTERNAL")
		return
	}

	respondJSON(w, http.StatusCreated, map[string]interface{}{
		"user":  user,
		"token": token,
	})
}

// POST /v1/auth/login
func (h *Handler) login(w http.ResponseWriter, r *http.Request) {
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

	user, token, err := h.svc.Login(r.Context(), req.Email, req.Password)
	if err != nil {
		if errors.Is(err, ErrInvalidCredentials) {
			respondError(w, http.StatusUnauthorized, "credenciales inválidas", "INVALID_CREDENTIALS")
			return
		}
		respondError(w, http.StatusInternalServerError, "login failed", "INTERNAL")
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"user":  user,
		"token": token,
	})
}

// GET /v1/users/me
func (h *Handler) getMe(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.StudentFromContext(r.Context())
	if !ok {
		respondError(w, http.StatusUnauthorized, "unauthorized", "UNAUTHORIZED")
		return
	}

	user, err := h.svc.GetUserByID(r.Context(), claims.Sub)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			respondError(w, http.StatusNotFound, "user not found", "NOT_FOUND")
			return
		}
		respondError(w, http.StatusInternalServerError, "failed to get user", "INTERNAL")
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"user": user,
	})
}

// GET /v1/users/me/addresses
func (h *Handler) listAddresses(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.StudentFromContext(r.Context())
	if !ok {
		respondError(w, http.StatusUnauthorized, "unauthorized", "UNAUTHORIZED")
		return
	}

	addrs, err := h.svc.ListAddresses(r.Context(), claims.Sub)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to list addresses", "INTERNAL")
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"addresses": addrs,
	})
}

// POST /v1/users/me/addresses
func (h *Handler) addAddress(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.StudentFromContext(r.Context())
	if !ok {
		respondError(w, http.StatusUnauthorized, "unauthorized", "UNAUTHORIZED")
		return
	}

	var input AddressInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body", "INVALID_BODY")
		return
	}
	if input.FullAddress == "" {
		respondError(w, http.StatusBadRequest, "full_address is required", "MISSING_FIELDS")
		return
	}

	addr, err := h.svc.AddAddress(r.Context(), claims.Sub, input)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to add address", "INTERNAL")
		return
	}

	respondJSON(w, http.StatusCreated, map[string]interface{}{
		"address": addr,
	})
}

// DELETE /v1/users/me/addresses/{id}
func (h *Handler) deleteAddress(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.StudentFromContext(r.Context())
	if !ok {
		respondError(w, http.StatusUnauthorized, "unauthorized", "UNAUTHORIZED")
		return
	}

	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid address id", "INVALID_PARAM")
		return
	}

	if err := h.svc.DeleteAddress(r.Context(), id, claims.Sub); err != nil {
		if errors.Is(err, ErrNotFound) || errors.Is(err, sql.ErrNoRows) {
			respondError(w, http.StatusNotFound, "address not found", "NOT_FOUND")
			return
		}
		respondError(w, http.StatusInternalServerError, "failed to delete address", "INTERNAL")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// DELETE /v1/users/me — anonymize user data (HABEAS DATA right to erasure)
func (h *Handler) deleteMe(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.StudentFromContext(r.Context())
	if !ok {
		respondError(w, http.StatusUnauthorized, "unauthorized", "UNAUTHORIZED")
		return
	}

	if err := h.svc.DeleteAccount(r.Context(), claims.Sub); err != nil {
		respondError(w, http.StatusInternalServerError, "failed to process deletion request", "INTERNAL_ERROR")
		return
	}

	respondJSON(w, http.StatusAccepted, map[string]string{
		"message": "Tu cuenta será eliminada en las próximas 24 horas",
	})
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
