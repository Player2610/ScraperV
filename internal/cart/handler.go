package cart

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/protou/protou/internal/auth"
)

// Handler holds HTTP handlers for the cart domain.
type Handler struct {
	svc *Service
}

// NewHandler creates a new cart Handler.
func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// RegisterRoutes mounts cart routes onto r (all require student auth).
func (h *Handler) RegisterRoutes(r chi.Router) {
	r.Group(func(r chi.Router) {
		r.Use(auth.RequireStudent)
		r.Get("/v1/cart", h.getCart)
		r.Put("/v1/cart/items/{listing_id}", h.upsertItem)
		r.Delete("/v1/cart/items/{listing_id}", h.removeItem)
		r.Delete("/v1/cart", h.clearCart)
		r.Post("/v1/cart/migrate", h.migrateGuestCart)
	})
}

// GET /v1/cart
func (h *Handler) getCart(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.StudentFromContext(r.Context())
	if !ok {
		respondError(w, http.StatusUnauthorized, "unauthorized", "UNAUTHORIZED")
		return
	}

	resp, err := h.svc.GetCart(r.Context(), claims.Sub)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to get cart", "INTERNAL")
		return
	}

	respondJSON(w, http.StatusOK, resp)
}

// PUT /v1/cart/items/{listing_id}
func (h *Handler) upsertItem(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)

	claims, ok := auth.StudentFromContext(r.Context())
	if !ok {
		respondError(w, http.StatusUnauthorized, "unauthorized", "UNAUTHORIZED")
		return
	}

	listingIDStr := chi.URLParam(r, "listing_id")
	listingID, err := strconv.ParseInt(listingIDStr, 10, 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid listing_id", "INVALID_PARAM")
		return
	}

	var body struct {
		Quantity int `json:"quantity"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body", "INVALID_BODY")
		return
	}

	// quantity must be between 1 and 99 (inclusive). Use DELETE /v1/cart to clear.
	if body.Quantity < 1 || body.Quantity > 99 {
		respondJSON(w, http.StatusBadRequest, map[string]string{
			"error": "INVALID_QUANTITY",
		})
		return
	}

	if err := h.svc.AddToCart(r.Context(), claims.Sub, listingID, body.Quantity); err != nil {
		if errors.Is(err, ErrUnavailable) {
			respondError(w, http.StatusUnprocessableEntity, "listing not available", "LISTING_UNAVAILABLE")
			return
		}
		respondError(w, http.StatusInternalServerError, "failed to update cart", "INTERNAL")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// DELETE /v1/cart/items/{listing_id}
// Removes a single item from the authenticated user's cart.
// Idempotent: returns 204 No Content even if the item was not in the cart.
func (h *Handler) removeItem(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.StudentFromContext(r.Context())
	if !ok {
		respondError(w, http.StatusUnauthorized, "unauthorized", "UNAUTHORIZED")
		return
	}

	listingIDStr := chi.URLParam(r, "listing_id")
	listingID, err := strconv.ParseInt(listingIDStr, 10, 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid listing_id", "INVALID_PARAM")
		return
	}

	if err := h.svc.RemoveItem(r.Context(), claims.Sub, listingID); err != nil {
		respondError(w, http.StatusInternalServerError, "failed to remove item", "INTERNAL")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// DELETE /v1/cart
func (h *Handler) clearCart(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.StudentFromContext(r.Context())
	if !ok {
		respondError(w, http.StatusUnauthorized, "unauthorized", "UNAUTHORIZED")
		return
	}

	cart, _, err := h.svc.repo.GetCartWithItems(r.Context(), claims.Sub)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to clear cart", "INTERNAL")
		return
	}
	if err := h.svc.repo.ClearCart(r.Context(), cart.ID); err != nil {
		respondError(w, http.StatusInternalServerError, "failed to clear cart", "INTERNAL")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// POST /v1/cart/migrate
func (h *Handler) migrateGuestCart(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.StudentFromContext(r.Context())
	if !ok {
		respondError(w, http.StatusUnauthorized, "unauthorized", "UNAUTHORIZED")
		return
	}

	var body struct {
		Items []GuestItem `json:"items"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body", "INVALID_BODY")
		return
	}

	if err := h.svc.MigrateGuestCart(r.Context(), claims.Sub, body.Items); err != nil {
		respondError(w, http.StatusInternalServerError, "failed to migrate cart", "INTERNAL")
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"status": "ok"})
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
