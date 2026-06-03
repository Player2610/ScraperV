package orders

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/protou/protou/internal/auth"
)

// validPaymentMethods is the set of accepted payment_method values.
var validPaymentMethods = map[PaymentMethod]bool{
	PaymentNequi:      true,
	PaymentDaviplata:  true,
	PaymentEfectivo:   true,
	PaymentLlavesBrev: true,
}

// Handler holds HTTP handlers for the orders domain.
type Handler struct {
	svc *Service
}

// NewHandler creates a new orders Handler.
func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// RegisterRoutes mounts order routes onto r.
func (h *Handler) RegisterRoutes(r chi.Router) {
	r.Group(func(r chi.Router) {
		r.Use(auth.RequireStudent)
		r.Post("/v1/checkout/delivery-fee", h.deliveryFee)
		r.Post("/v1/orders", h.createOrder)
		r.Get("/v1/orders", h.listOrders)
		r.Get("/v1/orders/{id}", h.getOrder)
	})
}

// POST /v1/checkout/delivery-fee
func (h *Handler) deliveryFee(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.StudentFromContext(r.Context())
	if !ok {
		respondError(w, http.StatusUnauthorized, "unauthorized", "UNAUTHORIZED")
		return
	}

	var req DeliveryFeeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body", "INVALID_BODY")
		return
	}

	result, err := h.svc.CalculateDeliveryFee(r.Context(), claims.Sub, req.AddressID)
	if err != nil {
		if errors.Is(err, ErrOutsideZone) {
			respondError(w, http.StatusUnprocessableEntity, err.Error(), "OUTSIDE_ZONE")
			return
		}
		respondError(w, http.StatusInternalServerError, "failed to calculate fee", "INTERNAL")
		return
	}

	respondJSON(w, http.StatusOK, result)
}

// POST /v1/orders
func (h *Handler) createOrder(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.StudentFromContext(r.Context())
	if !ok {
		respondError(w, http.StatusUnauthorized, "unauthorized", "UNAUTHORIZED")
		return
	}

	var req CreateOrderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body", "INVALID_BODY")
		return
	}
	if req.PaymentMethod == "" {
		respondError(w, http.StatusBadRequest, "payment_method is required", "MISSING_FIELDS")
		return
	}
	if !validPaymentMethods[req.PaymentMethod] {
		respondJSON(w, http.StatusBadRequest, map[string]string{
			"error": "INVALID_PAYMENT_METHOD",
		})
		return
	}

	order, items, err := h.svc.CreateOrder(r.Context(), claims.Sub, req)
	if err != nil {
		switch {
		case errors.Is(err, ErrOutsideZone):
			respondError(w, http.StatusUnprocessableEntity, err.Error(), "OUTSIDE_ZONE")
		case errors.Is(err, ErrEmptyCart):
			respondError(w, http.StatusUnprocessableEntity, err.Error(), "EMPTY_CART")
		case errors.Is(err, ErrUnavailableItems):
			respondError(w, http.StatusUnprocessableEntity, err.Error(), "UNAVAILABLE_ITEMS")
		default:
			respondError(w, http.StatusInternalServerError, "failed to create order", "INTERNAL")
		}
		return
	}

	respondJSON(w, http.StatusCreated, map[string]interface{}{
		"order": order,
		"items": items,
	})
}

// GET /v1/orders
func (h *Handler) listOrders(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.StudentFromContext(r.Context())
	if !ok {
		respondError(w, http.StatusUnauthorized, "unauthorized", "UNAUTHORIZED")
		return
	}

	orders, err := h.svc.ListOrders(r.Context(), claims.Sub)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to list orders", "INTERNAL")
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"orders": orders,
	})
}

// GET /v1/orders/{id}
func (h *Handler) getOrder(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.StudentFromContext(r.Context())
	if !ok {
		respondError(w, http.StatusUnauthorized, "unauthorized", "UNAUTHORIZED")
		return
	}

	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid order id", "INVALID_PARAM")
		return
	}

	order, items, err := h.svc.GetOrder(r.Context(), id, claims.Sub)
	if err != nil {
		if errors.Is(err, ErrNotFound) || errors.Is(err, sql.ErrNoRows) {
			respondError(w, http.StatusNotFound, "order not found", "NOT_FOUND")
			return
		}
		if errors.Is(err, ErrForbidden) {
			respondError(w, http.StatusForbidden, "forbidden", "FORBIDDEN")
			return
		}
		respondError(w, http.StatusInternalServerError, "failed to get order", "INTERNAL")
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"order": order,
		"items": items,
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
