package catalog

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
)

// Handler holds the HTTP handlers for the catalog domain.
type Handler struct {
	svc *Service
}

// NewHandler creates a new catalog Handler.
func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// RegisterRoutes mounts catalog routes onto the provided chi.Router.
// All routes are under /v1.
func (h *Handler) RegisterRoutes(r chi.Router) {
	r.Get("/v1/listings", h.listListings)
	r.Get("/v1/listings/{id}", h.getListing)
	r.Get("/v1/categories", h.listCategories)
	r.Get("/v1/categories/{slug}/listings", h.listingsByCategory)
	r.Get("/v1/stores", h.listStores)
}

// listListings handles GET /v1/listings
// Query params: q, category_id, store_id, page, per_page
func (h *Handler) listListings(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")
	page, perPage := parsePagination(r)

	filters := ListingFilters{}
	if v := r.URL.Query().Get("category_id"); v != "" {
		id, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			respondError(w, http.StatusBadRequest, "invalid category_id", "INVALID_PARAM")
			return
		}
		filters.CategoryID = &id
	}
	if v := r.URL.Query().Get("store_id"); v != "" {
		id, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			respondError(w, http.StatusBadRequest, "invalid store_id", "INVALID_PARAM")
			return
		}
		filters.StoreID = &id
	}

	listings, total, err := h.svc.SearchListings(r.Context(), q, filters, Page{Number: page, PerPage: perPage})
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to search listings", "INTERNAL")
		return
	}
	if listings == nil {
		listings = []Listing{}
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"listings": listings,
		"total":    total,
		"page":     page,
		"per_page": perPage,
	})
}

// getListing handles GET /v1/listings/{id}
func (h *Handler) getListing(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid listing id", "INVALID_PARAM")
		return
	}

	listing, err := h.svc.GetListing(r.Context(), id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			respondError(w, http.StatusNotFound, "listing not found", "NOT_FOUND")
			return
		}
		respondError(w, http.StatusInternalServerError, "failed to get listing", "INTERNAL")
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"listing": listing,
	})
}

// listCategories handles GET /v1/categories
func (h *Handler) listCategories(w http.ResponseWriter, r *http.Request) {
	cats, err := h.svc.ListCategoriesTree(r.Context())
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to list categories", "INTERNAL")
		return
	}
	if cats == nil {
		cats = []Category{}
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"categories": cats,
	})
}

// listingsByCategory handles GET /v1/categories/{slug}/listings
func (h *Handler) listingsByCategory(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")

	cat, err := h.svc.GetCategoryBySlug(r.Context(), slug)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			respondError(w, http.StatusNotFound, "category not found", "NOT_FOUND")
			return
		}
		respondError(w, http.StatusInternalServerError, "failed to get category", "INTERNAL")
		return
	}

	page, perPage := parsePagination(r)
	filters := ListingFilters{CategoryID: &cat.ID}

	listings, total, err := h.svc.SearchListings(r.Context(), "", filters, Page{Number: page, PerPage: perPage})
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to list listings", "INTERNAL")
		return
	}
	if listings == nil {
		listings = []Listing{}
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"category": cat,
		"listings": listings,
		"total":    total,
		"page":     page,
		"per_page": perPage,
	})
}

// listStores handles GET /v1/stores
func (h *Handler) listStores(w http.ResponseWriter, r *http.Request) {
	stores, err := h.svc.ListStores(r.Context())
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to list stores", "INTERNAL")
		return
	}
	if stores == nil {
		stores = []Store{}
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"stores": stores,
	})
}

// ─── helpers ─────────────────────────────────────────────────────────────────

func parsePagination(r *http.Request) (page, perPage int) {
	page = 1
	perPage = 20
	if v := r.URL.Query().Get("page"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			page = n
		}
	}
	if v := r.URL.Query().Get("per_page"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 100 {
			perPage = n
		}
	}
	return
}

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
