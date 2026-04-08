package operator

import (
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
)

// CouriersHandler handles operator courier endpoints.
type CouriersHandler struct {
	db *sql.DB
}

// NewCouriersHandler creates a CouriersHandler.
func NewCouriersHandler(db *sql.DB) *CouriersHandler {
	return &CouriersHandler{db: db}
}

// RegisterRoutes mounts courier management routes.
func (h *CouriersHandler) RegisterRoutes(r chi.Router) {
	r.Get("/v1/operator/couriers", h.listCouriers)
	r.Post("/v1/operator/couriers", h.createCourier)
	r.Put("/v1/operator/couriers/{id}", h.updateCourier)
}

// GET /v1/operator/couriers?all=true
func (h *CouriersHandler) listCouriers(w http.ResponseWriter, r *http.Request) {
	showAll := r.URL.Query().Get("all") == "true"

	var (
		rows *sql.Rows
		err  error
	)

	if showAll {
		const q = `SELECT id, name, phone, is_active, created_at FROM couriers ORDER BY is_active DESC, name ASC`
		rows, err = h.db.QueryContext(r.Context(), q)
	} else {
		const q = `SELECT id, name, phone, is_active, created_at FROM couriers WHERE is_active = true ORDER BY name ASC`
		rows, err = h.db.QueryContext(r.Context(), q)
	}
	if err != nil {
		log.Printf("operator couriers: list: %v", err)
		respondError(w, http.StatusInternalServerError, "failed to list couriers", "INTERNAL")
		return
	}
	defer rows.Close()

	type Courier struct {
		ID        int64     `json:"id"`
		Name      string    `json:"name"`
		Phone     string    `json:"phone"`
		IsActive  bool      `json:"is_active"`
		CreatedAt time.Time `json:"created_at"`
	}

	var couriers []Courier
	for rows.Next() {
		var c Courier
		if err := rows.Scan(&c.ID, &c.Name, &c.Phone, &c.IsActive, &c.CreatedAt); err != nil {
			log.Printf("operator couriers: scan: %v", err)
			respondError(w, http.StatusInternalServerError, "failed to list couriers", "INTERNAL")
			return
		}
		couriers = append(couriers, c)
	}
	if err := rows.Err(); err != nil {
		respondError(w, http.StatusInternalServerError, "failed to list couriers", "INTERNAL")
		return
	}
	if couriers == nil {
		couriers = []Courier{}
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{"couriers": couriers})
}

// POST /v1/operator/couriers
func (h *CouriersHandler) createCourier(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name  string `json:"name"`
		Phone string `json:"phone"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body", "INVALID_BODY")
		return
	}
	if req.Name == "" || req.Phone == "" {
		respondError(w, http.StatusBadRequest, "name and phone are required", "MISSING_FIELDS")
		return
	}

	const q = `
		INSERT INTO couriers (name, phone) VALUES ($1, $2)
		RETURNING id, name, phone, is_active, created_at
	`
	var c struct {
		ID        int64     `json:"id"`
		Name      string    `json:"name"`
		Phone     string    `json:"phone"`
		IsActive  bool      `json:"is_active"`
		CreatedAt time.Time `json:"created_at"`
	}
	if err := h.db.QueryRowContext(r.Context(), q, req.Name, req.Phone).Scan(
		&c.ID, &c.Name, &c.Phone, &c.IsActive, &c.CreatedAt,
	); err != nil {
		log.Printf("operator couriers: create: %v", err)
		respondError(w, http.StatusInternalServerError, "failed to create courier", "INTERNAL")
		return
	}

	respondJSON(w, http.StatusCreated, map[string]interface{}{"courier": c})
}

// PUT /v1/operator/couriers/{id}
func (h *CouriersHandler) updateCourier(w http.ResponseWriter, r *http.Request) {
	id, err := parseIDParam(r, "id")
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid courier id", "INVALID_PARAM")
		return
	}

	var req struct {
		Name     string `json:"name"`
		Phone    string `json:"phone"`
		IsActive *bool  `json:"is_active"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body", "INVALID_BODY")
		return
	}

	const q = `
		UPDATE couriers
		SET
			name      = COALESCE(NULLIF($1, ''), name),
			phone     = COALESCE(NULLIF($2, ''), phone),
			is_active = COALESCE($3, is_active)
		WHERE id = $4
		RETURNING id, name, phone, is_active, created_at
	`
	var c struct {
		ID        int64     `json:"id"`
		Name      string    `json:"name"`
		Phone     string    `json:"phone"`
		IsActive  bool      `json:"is_active"`
		CreatedAt time.Time `json:"created_at"`
	}
	if err := h.db.QueryRowContext(r.Context(), q, req.Name, req.Phone, req.IsActive, id).Scan(
		&c.ID, &c.Name, &c.Phone, &c.IsActive, &c.CreatedAt,
	); err != nil {
		if err == sql.ErrNoRows {
			respondError(w, http.StatusNotFound, "courier not found", "NOT_FOUND")
			return
		}
		log.Printf("operator couriers: update: %v", err)
		respondError(w, http.StatusInternalServerError, "failed to update courier", "INTERNAL")
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{"courier": c})
}
