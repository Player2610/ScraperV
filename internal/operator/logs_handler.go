package operator

import (
	"database/sql"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
)

// LogsHandler handles operator log endpoints.
type LogsHandler struct {
	db *sql.DB
}

// NewLogsHandler creates a LogsHandler.
func NewLogsHandler(db *sql.DB) *LogsHandler {
	return &LogsHandler{db: db}
}

// RegisterRoutes mounts log endpoints.
func (h *LogsHandler) RegisterRoutes(r chi.Router) {
	r.Get("/v1/operator/notification-logs", h.listNotificationLogs)
	r.Get("/v1/operator/scrape-jobs", h.listScrapeJobs)
}

// GET /v1/operator/notification-logs?order_id=
func (h *LogsHandler) listNotificationLogs(w http.ResponseWriter, r *http.Request) {
	orderIDStr := r.URL.Query().Get("order_id")

	type NotifLog struct {
		ID           int64     `json:"id"`
		OrderID      int64     `json:"order_id"`
		UserID       int64     `json:"user_id"`
		Channel      string    `json:"channel"`
		Event        string    `json:"event"`
		SentAt       time.Time `json:"sent_at"`
		Status       string    `json:"status"`
		ErrorMessage *string   `json:"error_message"`
	}

	var (
		rows *sql.Rows
		err  error
	)

	if orderIDStr != "" {
		orderID, parseErr := strconv.ParseInt(orderIDStr, 10, 64)
		if parseErr != nil {
			respondError(w, http.StatusBadRequest, "invalid order_id", "INVALID_PARAM")
			return
		}
		const q = `
			SELECT id, order_id, user_id, channel, event, sent_at, status, error_message
			FROM notification_logs
			WHERE order_id = $1
			ORDER BY sent_at DESC
			LIMIT 100
		`
		rows, err = h.db.QueryContext(r.Context(), q, orderID)
	} else {
		const q = `
			SELECT id, order_id, user_id, channel, event, sent_at, status, error_message
			FROM notification_logs
			ORDER BY sent_at DESC
			LIMIT 100
		`
		rows, err = h.db.QueryContext(r.Context(), q)
	}
	if err != nil {
		log.Printf("operator logs: notification-logs: %v", err)
		respondError(w, http.StatusInternalServerError, "failed to list notification logs", "INTERNAL")
		return
	}
	defer rows.Close()

	var logs []NotifLog
	for rows.Next() {
		var l NotifLog
		var errMsg sql.NullString
		if err := rows.Scan(&l.ID, &l.OrderID, &l.UserID, &l.Channel, &l.Event, &l.SentAt, &l.Status, &errMsg); err != nil {
			log.Printf("operator logs: scan notif log: %v", err)
			respondError(w, http.StatusInternalServerError, "failed to list notification logs", "INTERNAL")
			return
		}
		if errMsg.Valid {
			l.ErrorMessage = &errMsg.String
		}
		logs = append(logs, l)
	}
	if err := rows.Err(); err != nil {
		respondError(w, http.StatusInternalServerError, "failed to list notification logs", "INTERNAL")
		return
	}
	if logs == nil {
		logs = []NotifLog{}
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{"notification_logs": logs})
}

// GET /v1/operator/scrape-jobs?store_id=&limit=20
func (h *LogsHandler) listScrapeJobs(w http.ResponseWriter, r *http.Request) {
	storeIDStr := r.URL.Query().Get("store_id")
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 || limit > 100 {
		limit = 20
	}

	type ScrapeJob struct {
		ID              int64      `json:"id"`
		StoreID         int64      `json:"store_id"`
		StartedAt       time.Time  `json:"started_at"`
		FinishedAt      *time.Time `json:"finished_at"`
		Status          string     `json:"status"`
		ListingsFound   int        `json:"listings_found"`
		ListingsUpdated int        `json:"listings_updated"`
		ListingsNew     int        `json:"listings_new"`
		ErrorMessage    *string    `json:"error_message"`
	}

	var (
		rows *sql.Rows
		err  error
	)

	if storeIDStr != "" {
		storeID, parseErr := strconv.ParseInt(storeIDStr, 10, 64)
		if parseErr != nil {
			respondError(w, http.StatusBadRequest, "invalid store_id", "INVALID_PARAM")
			return
		}
		const q = `
			SELECT id, store_id, started_at, finished_at, status,
				listings_found, listings_updated, listings_new, error_message
			FROM scrape_jobs
			WHERE store_id = $1
			ORDER BY started_at DESC
			LIMIT $2
		`
		rows, err = h.db.QueryContext(r.Context(), q, storeID, limit)
	} else {
		const q = `
			SELECT id, store_id, started_at, finished_at, status,
				listings_found, listings_updated, listings_new, error_message
			FROM scrape_jobs
			ORDER BY started_at DESC
			LIMIT $1
		`
		rows, err = h.db.QueryContext(r.Context(), q, limit)
	}
	if err != nil {
		log.Printf("operator logs: scrape-jobs: %v", err)
		respondError(w, http.StatusInternalServerError, "failed to list scrape jobs", "INTERNAL")
		return
	}
	defer rows.Close()

	var jobs []ScrapeJob
	for rows.Next() {
		var j ScrapeJob
		var finishedAt sql.NullTime
		var errMsg sql.NullString
		if err := rows.Scan(
			&j.ID, &j.StoreID, &j.StartedAt, &finishedAt, &j.Status,
			&j.ListingsFound, &j.ListingsUpdated, &j.ListingsNew, &errMsg,
		); err != nil {
			log.Printf("operator logs: scan scrape job: %v", err)
			respondError(w, http.StatusInternalServerError, "failed to list scrape jobs", "INTERNAL")
			return
		}
		if finishedAt.Valid {
			j.FinishedAt = &finishedAt.Time
		}
		if errMsg.Valid {
			j.ErrorMessage = &errMsg.String
		}
		jobs = append(jobs, j)
	}
	if err := rows.Err(); err != nil {
		respondError(w, http.StatusInternalServerError, "failed to list scrape jobs", "INTERNAL")
		return
	}
	if jobs == nil {
		jobs = []ScrapeJob{}
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{"scrape_jobs": jobs})
}
