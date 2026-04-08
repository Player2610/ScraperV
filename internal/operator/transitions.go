package operator

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"net/http"
)

// ErrInvalidTransition is returned when a status transition is not allowed.
var ErrInvalidTransition = errors.New("invalid status transition")

// ErrOrderNotFound is returned when the order does not exist.
var ErrOrderNotFound = errors.New("order not found")

// validTransitions defines the allowed state machine transitions.
var validTransitions = map[string][]string{
	"pending_confirmation": {"confirmed", "cancelled", "failed"},
	"confirmed":            {"purchasing", "cancelled", "failed"},
	"purchasing":           {"in_delivery", "cancelled", "failed"},
	"in_delivery":          {"delivered", "failed"},
	"delivered":            {},
	"cancelled":            {},
	"failed":               {},
}

// isTransitionAllowed returns true if the transition from→to is valid.
func isTransitionAllowed(from, to string) bool {
	allowed, ok := validTransitions[from]
	if !ok {
		return false
	}
	for _, s := range allowed {
		if s == to {
			return true
		}
	}
	return false
}

// transitionOrderStatus performs a validated status transition in a transaction.
// If n is non-nil, sends a notification email async (fire-and-forget).
func transitionOrderStatus(
	ctx context.Context,
	db *sql.DB,
	orderID int64,
	toStatus string,
	actorID int64,
	note string,
	n *notifier,
) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() {
		if err != nil {
			tx.Rollback() //nolint:errcheck
		}
	}()

	// Get current status with a row lock
	const getQ = `SELECT status FROM orders WHERE id = $1 FOR UPDATE`
	var fromStatus string
	if err = tx.QueryRowContext(ctx, getQ, orderID).Scan(&fromStatus); err != nil {
		if err == sql.ErrNoRows {
			return ErrOrderNotFound
		}
		return fmt.Errorf("get order status: %w", err)
	}

	if !isTransitionAllowed(fromStatus, toStatus) {
		err = fmt.Errorf("%w: %s → %s", ErrInvalidTransition, fromStatus, toStatus)
		return err
	}

	// Update status
	const updateQ = `UPDATE orders SET status = $1, updated_at = NOW() WHERE id = $2`
	if _, err = tx.ExecContext(ctx, updateQ, toStatus, orderID); err != nil {
		return fmt.Errorf("update status: %w", err)
	}

	// Create event
	const eventQ = `
		INSERT INTO order_events (order_id, from_status, to_status, actor_id, note)
		VALUES ($1, $2, $3, $4, $5)
	`
	if _, err = tx.ExecContext(ctx, eventQ, orderID, fromStatus, toStatus, actorID, note); err != nil {
		return fmt.Errorf("create event: %w", err)
	}

	if err = tx.Commit(); err != nil {
		return fmt.Errorf("commit: %w", err)
	}

	// Send notification async (fire-and-forget)
	if n != nil {
		go n.sendStatusNotification(context.Background(), db, orderID, toStatus)
	}

	return nil
}

// handleTransitionError maps transition errors to HTTP responses.
func handleTransitionError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrOrderNotFound):
		respondError(w, http.StatusNotFound, "order not found", "NOT_FOUND")
	case errors.Is(err, ErrInvalidTransition):
		respondError(w, http.StatusUnprocessableEntity, err.Error(), "INVALID_TRANSITION")
	default:
		log.Printf("operator: transition: %v", err)
		respondError(w, http.StatusInternalServerError, "transition failed", "INTERNAL")
	}
}
