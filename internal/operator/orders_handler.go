package operator

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
)

// OrderEvent represents a row in order_events.
type OrderEvent struct {
	ID         int64     `json:"id"`
	OrderID    int64     `json:"order_id"`
	FromStatus *string   `json:"from_status"`
	ToStatus   string    `json:"to_status"`
	ActorID    *int64    `json:"actor_id"`
	Note       *string   `json:"note"`
	CreatedAt  time.Time `json:"created_at"`
}

// OrdersHandler handles operator order endpoints.
type OrdersHandler struct {
	db *sql.DB
	n  *notifier
}

// NewOrdersHandler creates an OrdersHandler.
func NewOrdersHandler(db *sql.DB, n *notifier) *OrdersHandler {
	return &OrdersHandler{db: db, n: n}
}

// RegisterRoutes mounts operator order routes (all protected).
func (h *OrdersHandler) RegisterRoutes(r chi.Router) {
	r.Get("/v1/operator/orders", h.listOrders)
	r.Get("/v1/operator/orders/{id}", h.getOrder)
	r.Post("/v1/operator/orders/{id}/confirm", h.confirmOrder)
	r.Post("/v1/operator/orders/{id}/transition", h.transitionOrder)
	r.Post("/v1/operator/orders/{id}/items/{item_id}/cancel", h.cancelItem)
	r.Put("/v1/operator/orders/{id}/delivery-fee", h.overrideDeliveryFee)
	r.Post("/v1/operator/orders/{id}/assign-courier", h.assignCourier)
	r.Post("/v1/operator/orders/{id}/payment", h.recordPayment)
}

// GET /v1/operator/orders?status=&page=&per_page=
func (h *OrdersHandler) listOrders(w http.ResponseWriter, r *http.Request) {
	status := r.URL.Query().Get("status")
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	perPage, _ := strconv.Atoi(r.URL.Query().Get("per_page"))
	if page <= 0 {
		page = 1
	}
	if perPage <= 0 || perPage > 100 {
		perPage = 20
	}
	offset := (page - 1) * perPage

	var (
		rows *sql.Rows
		err  error
	)

	if status != "" {
		const q = `
			SELECT o.id, o.user_id, o.status, o.delivery_address_snapshot,
				o.subtotal_cop, o.delivery_fee_cop, o.total_cop, o.payment_method, o.notes,
				o.created_at, o.updated_at,
				u.name AS student_name, u.email AS student_email
			FROM orders o
			INNER JOIN users u ON u.id = o.user_id
			WHERE o.status = $1
			ORDER BY o.created_at DESC
			LIMIT $2 OFFSET $3
		`
		rows, err = h.db.QueryContext(r.Context(), q, status, perPage, offset)
	} else {
		const q = `
			SELECT o.id, o.user_id, o.status, o.delivery_address_snapshot,
				o.subtotal_cop, o.delivery_fee_cop, o.total_cop, o.payment_method, o.notes,
				o.created_at, o.updated_at,
				u.name AS student_name, u.email AS student_email
			FROM orders o
			INNER JOIN users u ON u.id = o.user_id
			ORDER BY o.created_at DESC
			LIMIT $1 OFFSET $2
		`
		rows, err = h.db.QueryContext(r.Context(), q, perPage, offset)
	}
	if err != nil {
		slog.Error("operator orders: list", "error", err)
		respondError(w, http.StatusInternalServerError, "failed to list orders", "INTERNAL")
		return
	}
	defer rows.Close()

	type OrderRow struct {
		ID             int64     `json:"id"`
		UserID         int64     `json:"user_id"`
		Status         string    `json:"status"`
		SubtotalCOP    int       `json:"subtotal_cop"`
		DeliveryFeeCOP int       `json:"delivery_fee_cop"`
		TotalCOP       int       `json:"total_cop"`
		PaymentMethod  string    `json:"payment_method"`
		Notes          *string   `json:"notes"`
		CreatedAt      time.Time `json:"created_at"`
		UpdatedAt      time.Time `json:"updated_at"`
		StudentName    string    `json:"student_name"`
		StudentEmail   string    `json:"student_email"`
	}

	var orders []OrderRow
	for rows.Next() {
		var o OrderRow
		var notes sql.NullString
		var addrJSON []byte
		if err := rows.Scan(
			&o.ID, &o.UserID, &o.Status, &addrJSON,
			&o.SubtotalCOP, &o.DeliveryFeeCOP, &o.TotalCOP, &o.PaymentMethod, &notes,
			&o.CreatedAt, &o.UpdatedAt, &o.StudentName, &o.StudentEmail,
		); err != nil {
			slog.Error("operator orders: scan", "error", err)
			respondError(w, http.StatusInternalServerError, "failed to list orders", "INTERNAL")
			return
		}
		if notes.Valid {
			o.Notes = &notes.String
		}
		orders = append(orders, o)
	}
	if err := rows.Err(); err != nil {
		slog.Error("operator orders: rows", "error", err)
		respondError(w, http.StatusInternalServerError, "failed to list orders", "INTERNAL")
		return
	}

	if orders == nil {
		orders = []OrderRow{}
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"orders":   orders,
		"page":     page,
		"per_page": perPage,
	})
}

// GET /v1/operator/orders/{id}
func (h *OrdersHandler) getOrder(w http.ResponseWriter, r *http.Request) {
	id, err := parseIDParam(r, "id")
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid order id", "INVALID_PARAM")
		return
	}

	order, err := getOrderByID(r.Context(), h.db, id)
	if err != nil {
		if err == sql.ErrNoRows {
			respondError(w, http.StatusNotFound, "order not found", "NOT_FOUND")
			return
		}
		slog.Error("operator orders: get", "error", err)
		respondError(w, http.StatusInternalServerError, "failed to get order", "INTERNAL")
		return
	}

	items, err := listOrderItems(r.Context(), h.db, id)
	if err != nil {
		slog.Error("operator orders: items", "error", err)
		respondError(w, http.StatusInternalServerError, "failed to get order items", "INTERNAL")
		return
	}

	events, err := listOrderEvents(r.Context(), h.db, id)
	if err != nil {
		slog.Error("operator orders: events", "error", err)
		respondError(w, http.StatusInternalServerError, "failed to get order events", "INTERNAL")
		return
	}

	delivery, err := getOrderDelivery(r.Context(), h.db, id)
	if err != nil && err != sql.ErrNoRows {
		slog.Error("operator orders: delivery", "error", err)
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"order":    order,
		"items":    items,
		"events":   events,
		"delivery": delivery,
	})
}

// POST /v1/operator/orders/{id}/confirm
func (h *OrdersHandler) confirmOrder(w http.ResponseWriter, r *http.Request) {
	id, err := parseIDParam(r, "id")
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid order id", "INVALID_PARAM")
		return
	}

	operatorID, _ := OperatorIDFromContext(r.Context())

	if err := transitionOrderStatus(r.Context(), h.db, id, "confirmed", operatorID, "order confirmed by operator", h.n); err != nil {
		handleTransitionError(w, err)
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"status": "confirmed"})
}

// POST /v1/operator/orders/{id}/transition
func (h *OrdersHandler) transitionOrder(w http.ResponseWriter, r *http.Request) {
	id, err := parseIDParam(r, "id")
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid order id", "INVALID_PARAM")
		return
	}

	var req struct {
		To   string  `json:"to"`
		Note *string `json:"note"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body", "INVALID_BODY")
		return
	}
	if req.To == "" {
		respondError(w, http.StatusBadRequest, "to is required", "MISSING_FIELDS")
		return
	}

	operatorID, _ := OperatorIDFromContext(r.Context())
	note := "status transition"
	if req.Note != nil && *req.Note != "" {
		note = *req.Note
	}

	if err := transitionOrderStatus(r.Context(), h.db, id, req.To, operatorID, note, h.n); err != nil {
		handleTransitionError(w, err)
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"status": req.To})
}

// POST /v1/operator/orders/{id}/items/{item_id}/cancel
func (h *OrdersHandler) cancelItem(w http.ResponseWriter, r *http.Request) {
	orderID, err := parseIDParam(r, "id")
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid order id", "INVALID_PARAM")
		return
	}
	itemID, err := parseIDParam(r, "item_id")
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid item id", "INVALID_PARAM")
		return
	}

	var req struct {
		Reason string `json:"reason"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body", "INVALID_BODY")
		return
	}

	operatorID, _ := OperatorIDFromContext(r.Context())

	tx, err := h.db.BeginTx(r.Context(), nil)
	if err != nil {
		slog.Error("operator: cancel item: begin tx", "error", err)
		respondError(w, http.StatusInternalServerError, "failed to cancel item", "INTERNAL")
		return
	}
	defer func() {
		if err != nil {
			tx.Rollback() //nolint:errcheck
		}
	}()

	// Cancel the item
	const cancelQ = `
		UPDATE order_items SET is_cancelled = true
		WHERE id = $1 AND order_id = $2 AND is_cancelled = false
	`
	res, err := tx.ExecContext(r.Context(), cancelQ, itemID, orderID)
	if err != nil {
		slog.Error("operator: cancel item: update", "error", err)
		respondError(w, http.StatusInternalServerError, "failed to cancel item", "INTERNAL")
		return
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		respondError(w, http.StatusNotFound, "item not found or already cancelled", "NOT_FOUND")
		return
	}

	// Recalculate subtotal from non-cancelled items
	const subtotalQ = `
		SELECT COALESCE(SUM(price_snapshot_cop * quantity), 0)
		FROM order_items
		WHERE order_id = $1 AND is_cancelled = false
	`
	var newSubtotal int
	if err = tx.QueryRowContext(r.Context(), subtotalQ, orderID).Scan(&newSubtotal); err != nil {
		slog.Error("operator: cancel item: subtotal", "error", err)
		respondError(w, http.StatusInternalServerError, "failed to recalculate subtotal", "INTERNAL")
		return
	}

	// Get delivery fee
	const feeQ = `SELECT delivery_fee_cop FROM orders WHERE id = $1`
	var deliveryFee int
	if err = tx.QueryRowContext(r.Context(), feeQ, orderID).Scan(&deliveryFee); err != nil {
		slog.Error("operator: cancel item: get fee", "error", err)
		respondError(w, http.StatusInternalServerError, "failed to get delivery fee", "INTERNAL")
		return
	}

	newTotal := newSubtotal + deliveryFee

	// Update order totals
	const updateQ = `
		UPDATE orders SET subtotal_cop = $1, total_cop = $2, updated_at = NOW()
		WHERE id = $3
	`
	if _, err = tx.ExecContext(r.Context(), updateQ, newSubtotal, newTotal, orderID); err != nil {
		slog.Error("operator: cancel item: update order", "error", err)
		respondError(w, http.StatusInternalServerError, "failed to update order totals", "INTERNAL")
		return
	}

	// Check if all items cancelled
	const allCancelledQ = `
		SELECT COUNT(*) FROM order_items WHERE order_id = $1 AND is_cancelled = false
	`
	var activeCount int
	if err = tx.QueryRowContext(r.Context(), allCancelledQ, orderID).Scan(&activeCount); err != nil {
		slog.Error("operator: cancel item: count active", "error", err)
		respondError(w, http.StatusInternalServerError, "failed to check remaining items", "INTERNAL")
		return
	}

	reason := req.Reason
	if reason == "" {
		reason = "item cancelled by operator"
	}

	// Create event for item cancellation
	const eventQ = `
		INSERT INTO order_events (order_id, from_status, to_status, actor_id, note)
		SELECT id, status, status, $2, $3 FROM orders WHERE id = $1
	`
	if _, err = tx.ExecContext(r.Context(), eventQ, orderID, operatorID, fmt.Sprintf("item %d cancelled: %s", itemID, reason)); err != nil {
		slog.Error("operator: cancel item: event", "error", err)
		respondError(w, http.StatusInternalServerError, "failed to create event", "INTERNAL")
		return
	}

	if activeCount == 0 {
		// All items cancelled → transition order to cancelled
		const statusQ = `
			UPDATE orders SET status = 'cancelled', updated_at = NOW() WHERE id = $1
			RETURNING status
		`
		if _, err = tx.ExecContext(r.Context(), statusQ, orderID); err != nil {
			slog.Error("operator: cancel item: cancel order", "error", err)
			respondError(w, http.StatusInternalServerError, "failed to cancel order", "INTERNAL")
			return
		}
		const cancelEventQ = `
			INSERT INTO order_events (order_id, from_status, to_status, actor_id, note)
			VALUES ($1, $2, 'cancelled', $3, 'all items cancelled')
		`
		// Get current status for from_status
		var curStatus string
		_ = h.db.QueryRowContext(r.Context(), `SELECT status FROM orders WHERE id = $1`, orderID).Scan(&curStatus)
		if _, err = tx.ExecContext(r.Context(), cancelEventQ, orderID, curStatus, operatorID); err != nil {
			slog.Error("operator: cancel item: cancel event", "error", err)
		}
	}

	if err = tx.Commit(); err != nil {
		slog.Error("operator: cancel item: commit", "error", err)
		respondError(w, http.StatusInternalServerError, "failed to commit", "INTERNAL")
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"new_subtotal_cop": newSubtotal,
		"new_total_cop":    newTotal,
		"all_cancelled":    activeCount == 0,
	})
}

// PUT /v1/operator/orders/{id}/delivery-fee
func (h *OrdersHandler) overrideDeliveryFee(w http.ResponseWriter, r *http.Request) {
	id, err := parseIDParam(r, "id")
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid order id", "INVALID_PARAM")
		return
	}

	var req struct {
		FeeCOP int `json:"fee_cop"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body", "INVALID_BODY")
		return
	}
	if req.FeeCOP < 0 {
		respondError(w, http.StatusBadRequest, "fee_cop must be >= 0", "INVALID_FIELDS")
		return
	}

	operatorID, _ := OperatorIDFromContext(r.Context())

	// Get current delivery_fee_cop and subtotal
	const getQ = `SELECT delivery_fee_cop, subtotal_cop FROM orders WHERE id = $1`
	var oldFee, subtotal int
	if err := h.db.QueryRowContext(r.Context(), getQ, id).Scan(&oldFee, &subtotal); err != nil {
		if err == sql.ErrNoRows {
			respondError(w, http.StatusNotFound, "order not found", "NOT_FOUND")
			return
		}
		respondError(w, http.StatusInternalServerError, "failed to get order", "INTERNAL")
		return
	}

	newTotal := subtotal + req.FeeCOP

	tx, txErr := h.db.BeginTx(r.Context(), nil)
	if txErr != nil {
		respondError(w, http.StatusInternalServerError, "failed to update fee", "INTERNAL")
		return
	}
	defer func() { tx.Rollback() }() //nolint:errcheck

	const updateQ = `
		UPDATE orders SET delivery_fee_cop = $1, total_cop = $2, updated_at = NOW()
		WHERE id = $3
	`
	if _, err := tx.ExecContext(r.Context(), updateQ, req.FeeCOP, newTotal, id); err != nil {
		slog.Error("operator: override fee: update", "error", err)
		respondError(w, http.StatusInternalServerError, "failed to update fee", "INTERNAL")
		return
	}

	note := fmt.Sprintf("delivery fee changed from %d to %d by operator", oldFee, req.FeeCOP)
	const eventQ = `
		INSERT INTO order_events (order_id, from_status, to_status, actor_id, note)
		SELECT id, status, status, $2, $3 FROM orders WHERE id = $1
	`
	if _, err := tx.ExecContext(r.Context(), eventQ, id, operatorID, note); err != nil {
		slog.Error("operator: override fee: event", "error", err)
	}

	if err := tx.Commit(); err != nil {
		respondError(w, http.StatusInternalServerError, "failed to commit", "INTERNAL")
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"delivery_fee_cop": req.FeeCOP,
		"total_cop":        newTotal,
	})
}

// POST /v1/operator/orders/{id}/assign-courier
func (h *OrdersHandler) assignCourier(w http.ResponseWriter, r *http.Request) {
	id, err := parseIDParam(r, "id")
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid order id", "INVALID_PARAM")
		return
	}

	var req struct {
		CourierID int64 `json:"courier_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body", "INVALID_BODY")
		return
	}
	if req.CourierID == 0 {
		respondError(w, http.StatusBadRequest, "courier_id is required", "MISSING_FIELDS")
		return
	}

	// Check courier is active
	const courierQ = `SELECT is_active FROM couriers WHERE id = $1`
	var isActive bool
	if err := h.db.QueryRowContext(r.Context(), courierQ, req.CourierID).Scan(&isActive); err != nil {
		if err == sql.ErrNoRows {
			respondError(w, http.StatusUnprocessableEntity, "courier not found", "NOT_FOUND")
			return
		}
		respondError(w, http.StatusInternalServerError, "failed to check courier", "INTERNAL")
		return
	}
	if !isActive {
		respondError(w, http.StatusUnprocessableEntity, "courier is not active", "COURIER_INACTIVE")
		return
	}

	// Check order status is purchasing or in_delivery
	const statusQ = `SELECT status, delivery_fee_cop FROM orders WHERE id = $1`
	var orderStatus string
	var deliveryFee int
	if err := h.db.QueryRowContext(r.Context(), statusQ, id).Scan(&orderStatus, &deliveryFee); err != nil {
		if err == sql.ErrNoRows {
			respondError(w, http.StatusNotFound, "order not found", "NOT_FOUND")
			return
		}
		respondError(w, http.StatusInternalServerError, "failed to get order", "INTERNAL")
		return
	}
	if orderStatus != "purchasing" && orderStatus != "in_delivery" {
		respondError(w, http.StatusUnprocessableEntity, "order must be in purchasing or in_delivery status", "INVALID_STATUS")
		return
	}

	// Upsert deliveries row
	const upsertQ = `
		INSERT INTO deliveries (order_id, courier_id, assigned_at, delivery_fee_cop)
		VALUES ($1, $2, NOW(), $3)
		ON CONFLICT (order_id) DO UPDATE
		SET courier_id = EXCLUDED.courier_id, assigned_at = EXCLUDED.assigned_at
	`
	if _, err := h.db.ExecContext(r.Context(), upsertQ, id, req.CourierID, deliveryFee); err != nil {
		slog.Error("operator: assign courier: upsert", "error", err)
		respondError(w, http.StatusInternalServerError, "failed to assign courier", "INTERNAL")
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"order_id":   id,
		"courier_id": req.CourierID,
	})
}

// POST /v1/operator/orders/{id}/payment
func (h *OrdersHandler) recordPayment(w http.ResponseWriter, r *http.Request) {
	id, err := parseIDParam(r, "id")
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid order id", "INVALID_PARAM")
		return
	}

	var req struct {
		Method    string `json:"method"`
		AmountCOP int    `json:"amount_cop"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body", "INVALID_BODY")
		return
	}
	if req.Method == "" || req.AmountCOP <= 0 {
		respondError(w, http.StatusBadRequest, "method and amount_cop are required", "MISSING_FIELDS")
		return
	}

	operatorID, _ := OperatorIDFromContext(r.Context())

	// Check order status is delivered
	const statusQ = `SELECT status FROM orders WHERE id = $1`
	var orderStatus string
	if err := h.db.QueryRowContext(r.Context(), statusQ, id).Scan(&orderStatus); err != nil {
		if err == sql.ErrNoRows {
			respondError(w, http.StatusNotFound, "order not found", "NOT_FOUND")
			return
		}
		respondError(w, http.StatusInternalServerError, "failed to get order", "INTERNAL")
		return
	}
	if orderStatus != "delivered" {
		respondError(w, http.StatusUnprocessableEntity, "order must be in delivered status", "INVALID_STATUS")
		return
	}

	// Insert payment record (UNIQUE on order_id → 409 on duplicate)
	const insertQ = `
		INSERT INTO payment_records (order_id, method, amount_cop, received_at, received_by_operator_id)
		VALUES ($1, $2, $3, NOW(), $4)
		RETURNING id
	`
	var paymentID int64
	if err := h.db.QueryRowContext(r.Context(), insertQ, id, req.Method, req.AmountCOP, operatorID).Scan(&paymentID); err != nil {
		// Check for duplicate (unique violation on order_id)
		if isDuplicateError(err) {
			respondError(w, http.StatusConflict, "payment already recorded for this order", "DUPLICATE_PAYMENT")
			return
		}
		slog.Error("operator: record payment: insert", "error", err)
		respondError(w, http.StatusInternalServerError, "failed to record payment", "INTERNAL")
		return
	}

	respondJSON(w, http.StatusCreated, map[string]interface{}{
		"payment_id": paymentID,
		"order_id":   id,
		"method":     req.Method,
		"amount_cop": req.AmountCOP,
	})
}

// ─── shared helpers ────────────────────────────────────────────────────────────

type orderRow struct {
	ID             int64           `json:"id"`
	UserID         int64           `json:"user_id"`
	Status         string          `json:"status"`
	SubtotalCOP    int             `json:"subtotal_cop"`
	DeliveryFeeCOP int             `json:"delivery_fee_cop"`
	TotalCOP       int             `json:"total_cop"`
	PaymentMethod  string          `json:"payment_method"`
	Notes          *string         `json:"notes"`
	CreatedAt      time.Time       `json:"created_at"`
	UpdatedAt      time.Time       `json:"updated_at"`
	StudentName    string          `json:"student_name"`
	StudentEmail   string          `json:"student_email"`
	AddrJSON       json.RawMessage `json:"delivery_address"`
}

func getOrderByID(ctx context.Context, db *sql.DB, id int64) (*orderRow, error) {
	const q = `
		SELECT o.id, o.user_id, o.status, o.delivery_address_snapshot,
			o.subtotal_cop, o.delivery_fee_cop, o.total_cop, o.payment_method, o.notes,
			o.created_at, o.updated_at,
			u.name, u.email
		FROM orders o
		INNER JOIN users u ON u.id = o.user_id
		WHERE o.id = $1
	`
	var o orderRow
	var notes sql.NullString
	var addrBytes []byte
	err := db.QueryRowContext(ctx, q, id).Scan(
		&o.ID, &o.UserID, &o.Status, &addrBytes,
		&o.SubtotalCOP, &o.DeliveryFeeCOP, &o.TotalCOP, &o.PaymentMethod, &notes,
		&o.CreatedAt, &o.UpdatedAt, &o.StudentName, &o.StudentEmail,
	)
	if err != nil {
		return nil, err
	}
	if notes.Valid {
		o.Notes = &notes.String
	}
	o.AddrJSON = json.RawMessage(addrBytes)
	return &o, nil
}

type itemRow struct {
	ID                   int64     `json:"id"`
	OrderID              int64     `json:"order_id"`
	ListingID            *int64    `json:"listing_id"`
	ListingNameSnapshot  string    `json:"listing_name_snapshot"`
	ListingStoreSnapshot string    `json:"listing_store_snapshot"`
	PriceSnapshotCOP     int       `json:"price_snapshot_cop"`
	Quantity             int       `json:"quantity"`
	IsCancelled          bool      `json:"is_cancelled"`
	CreatedAt            time.Time `json:"created_at"`
}

func listOrderItems(ctx context.Context, db *sql.DB, orderID int64) ([]itemRow, error) {
	const q = `
		SELECT id, order_id, listing_id, listing_name_snapshot, listing_store_snapshot,
			price_snapshot_cop, quantity, is_cancelled, created_at
		FROM order_items
		WHERE order_id = $1
		ORDER BY id ASC
	`
	rows, err := db.QueryContext(ctx, q, orderID)
	if err != nil {
		return nil, fmt.Errorf("list order items: %w", err)
	}
	defer rows.Close()

	var items []itemRow
	for rows.Next() {
		var item itemRow
		var lid sql.NullInt64
		if err := rows.Scan(
			&item.ID, &item.OrderID, &lid, &item.ListingNameSnapshot, &item.ListingStoreSnapshot,
			&item.PriceSnapshotCOP, &item.Quantity, &item.IsCancelled, &item.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan item: %w", err)
		}
		if lid.Valid {
			item.ListingID = &lid.Int64
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("items rows: %w", err)
	}
	if items == nil {
		items = []itemRow{}
	}
	return items, nil
}

func listOrderEvents(ctx context.Context, db *sql.DB, orderID int64) ([]OrderEvent, error) {
	const q = `
		SELECT id, order_id, from_status, to_status, actor_id, note, created_at
		FROM order_events
		WHERE order_id = $1
		ORDER BY created_at ASC
	`
	rows, err := db.QueryContext(ctx, q, orderID)
	if err != nil {
		return nil, fmt.Errorf("list order events: %w", err)
	}
	defer rows.Close()

	var events []OrderEvent
	for rows.Next() {
		var e OrderEvent
		var fromStatus sql.NullString
		var actorID sql.NullInt64
		var note sql.NullString
		if err := rows.Scan(&e.ID, &e.OrderID, &fromStatus, &e.ToStatus, &actorID, &note, &e.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan event: %w", err)
		}
		if fromStatus.Valid {
			e.FromStatus = &fromStatus.String
		}
		if actorID.Valid {
			e.ActorID = &actorID.Int64
		}
		if note.Valid {
			e.Note = &note.String
		}
		events = append(events, e)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("events rows: %w", err)
	}
	if events == nil {
		events = []OrderEvent{}
	}
	return events, nil
}

type deliveryRow struct {
	ID             int64      `json:"id"`
	OrderID        int64      `json:"order_id"`
	CourierID      *int64     `json:"courier_id"`
	CourierName    *string    `json:"courier_name"`
	CourierPhone   *string    `json:"courier_phone"`
	AssignedAt     *time.Time `json:"assigned_at"`
	PickedUpAt     *time.Time `json:"picked_up_at"`
	DeliveredAt    *time.Time `json:"delivered_at"`
	DeliveryFeeCOP int        `json:"delivery_fee_cop"`
	Notes          *string    `json:"notes"`
}

func getOrderDelivery(ctx context.Context, db *sql.DB, orderID int64) (*deliveryRow, error) {
	const q = `
		SELECT d.id, d.order_id, d.courier_id, c.name, c.phone,
			d.assigned_at, d.picked_up_at, d.delivered_at,
			d.delivery_fee_cop, d.notes
		FROM deliveries d
		LEFT JOIN couriers c ON c.id = d.courier_id
		WHERE d.order_id = $1
	`
	var d deliveryRow
	var courierID sql.NullInt64
	var courierName, courierPhone, notes sql.NullString
	var assignedAt, pickedUpAt, deliveredAt sql.NullTime
	err := db.QueryRowContext(ctx, q, orderID).Scan(
		&d.ID, &d.OrderID, &courierID, &courierName, &courierPhone,
		&assignedAt, &pickedUpAt, &deliveredAt, &d.DeliveryFeeCOP, &notes,
	)
	if err != nil {
		return nil, err
	}
	if courierID.Valid {
		d.CourierID = &courierID.Int64
	}
	if courierName.Valid {
		d.CourierName = &courierName.String
	}
	if courierPhone.Valid {
		d.CourierPhone = &courierPhone.String
	}
	if assignedAt.Valid {
		d.AssignedAt = &assignedAt.Time
	}
	if pickedUpAt.Valid {
		d.PickedUpAt = &pickedUpAt.Time
	}
	if deliveredAt.Valid {
		d.DeliveredAt = &deliveredAt.Time
	}
	if notes.Valid {
		d.Notes = &notes.String
	}
	return &d, nil
}

func parseIDParam(r *http.Request, param string) (int64, error) {
	s := chi.URLParam(r, param)
	return strconv.ParseInt(s, 10, 64)
}

func isDuplicateError(err error) bool {
	if err == nil {
		return false
	}
	return contains(err.Error(), "duplicate key") || contains(err.Error(), "unique constraint")
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsStr(s, substr))
}

func containsStr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
