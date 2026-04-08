package operator

import (
	"context"
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
)

// DeliveryConfigHandler handles operator delivery config endpoints.
type DeliveryConfigHandler struct {
	db *sql.DB
}

// NewDeliveryConfigHandler creates a DeliveryConfigHandler.
func NewDeliveryConfigHandler(db *sql.DB) *DeliveryConfigHandler {
	return &DeliveryConfigHandler{db: db}
}

// RegisterRoutes mounts delivery config routes.
func (h *DeliveryConfigHandler) RegisterRoutes(r chi.Router) {
	r.Get("/v1/operator/delivery-config", h.getConfig)
	r.Put("/v1/operator/delivery-config", h.updateConfig)
}

type bracket struct {
	ID            *int64   `json:"id,omitempty"`
	DistanceKmMin float64  `json:"distance_km_min"`
	DistanceKmMax *float64 `json:"distance_km_max"`
	FeeCOP        int      `json:"fee_cop"`
}

type deliveryConfigResponse struct {
	Brackets          []bracket `json:"brackets"`
	MultiStoreDiscPct int       `json:"multi_store_discount_pct"`
	UpdatedAt         time.Time `json:"updated_at"`
}

// GET /v1/operator/delivery-config
func (h *DeliveryConfigHandler) getConfig(w http.ResponseWriter, r *http.Request) {
	brackets, err := fetchBrackets(r.Context(), h.db)
	if err != nil {
		log.Printf("operator delivery-config: load brackets: %v", err)
		respondError(w, http.StatusInternalServerError, "failed to load config", "INTERNAL")
		return
	}

	var cfg deliveryConfigResponse
	cfg.Brackets = brackets

	const cfgQ = `SELECT multi_store_discount_pct, updated_at FROM delivery_config WHERE id = 1`
	if err := h.db.QueryRowContext(r.Context(), cfgQ).Scan(&cfg.MultiStoreDiscPct, &cfg.UpdatedAt); err != nil {
		if err == sql.ErrNoRows {
			cfg.MultiStoreDiscPct = 30
		} else {
			log.Printf("operator delivery-config: load config: %v", err)
			respondError(w, http.StatusInternalServerError, "failed to load config", "INTERNAL")
			return
		}
	}

	respondJSON(w, http.StatusOK, cfg)
}

// PUT /v1/operator/delivery-config
func (h *DeliveryConfigHandler) updateConfig(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Brackets          []bracket `json:"brackets"`
		MultiStoreDiscPct int       `json:"multi_store_discount_pct"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body", "INVALID_BODY")
		return
	}

	// Validate
	for _, b := range req.Brackets {
		if b.FeeCOP <= 0 {
			respondError(w, http.StatusBadRequest, "bracket fee_cop must be > 0", "INVALID_FIELDS")
			return
		}
	}

	tx, err := h.db.BeginTx(r.Context(), nil)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to update config", "INTERNAL")
		return
	}
	defer func() { tx.Rollback() }() //nolint:errcheck

	// Delete all brackets atomically
	if _, err := tx.ExecContext(r.Context(), `DELETE FROM delivery_fee_brackets`); err != nil {
		log.Printf("operator delivery-config: delete brackets: %v", err)
		respondError(w, http.StatusInternalServerError, "failed to update config", "INTERNAL")
		return
	}

	// Insert new brackets
	for _, b := range req.Brackets {
		const insertQ = `
			INSERT INTO delivery_fee_brackets (distance_km_min, distance_km_max, fee_cop)
			VALUES ($1, $2, $3)
		`
		if _, err := tx.ExecContext(r.Context(), insertQ, b.DistanceKmMin, b.DistanceKmMax, b.FeeCOP); err != nil {
			log.Printf("operator delivery-config: insert bracket: %v", err)
			respondError(w, http.StatusInternalServerError, "failed to update config", "INTERNAL")
			return
		}
	}

	// Upsert delivery_config
	const upsertQ = `
		INSERT INTO delivery_config (id, multi_store_discount_pct, updated_at)
		VALUES (1, $1, NOW())
		ON CONFLICT (id) DO UPDATE
		SET multi_store_discount_pct = EXCLUDED.multi_store_discount_pct,
		    updated_at = NOW()
	`
	if _, err := tx.ExecContext(r.Context(), upsertQ, req.MultiStoreDiscPct); err != nil {
		log.Printf("operator delivery-config: upsert config: %v", err)
		respondError(w, http.StatusInternalServerError, "failed to update config", "INTERNAL")
		return
	}

	if err := tx.Commit(); err != nil {
		log.Printf("operator delivery-config: commit: %v", err)
		respondError(w, http.StatusInternalServerError, "failed to update config", "INTERNAL")
		return
	}

	h.getConfig(w, r)
}

// fetchBrackets loads delivery fee brackets from DB.
func fetchBrackets(ctx context.Context, db *sql.DB) ([]bracket, error) {
	const q = `
		SELECT id, distance_km_min, distance_km_max, fee_cop
		FROM delivery_fee_brackets
		ORDER BY distance_km_min ASC
	`
	rows, err := db.QueryContext(ctx, q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var brackets []bracket
	for rows.Next() {
		var b bracket
		var maxKm sql.NullFloat64
		var id int64
		if err := rows.Scan(&id, &b.DistanceKmMin, &maxKm, &b.FeeCOP); err != nil {
			return nil, err
		}
		b.ID = &id
		if maxKm.Valid {
			b.DistanceKmMax = &maxKm.Float64
		}
		brackets = append(brackets, b)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if brackets == nil {
		brackets = []bracket{}
	}
	return brackets, nil
}
