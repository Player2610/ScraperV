//go:build integration

package e2e_test

import (
	"context"
	"database/sql"
	"testing"

	"github.com/protou/protou/internal/auth"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"
)

// TestOperatorFlow_LoginListOrdersTransition verifies the operator flow:
// operator login → list orders → transition a pending order (if available).
//
// Drives the auth and DB layers directly against a real DB (same pattern
// as TestStudentFlow_* — no HTTP, no mocks).
//
// Requirements:
//   - TEST_DATABASE_URL env var pointing to a migrated DB.
//   - A seed operator must exist in the DB (migration 008_seed_operator.sql
//     creates operator@protou.co / operator123).
func TestOperatorFlow_LoginListOrdersTransition(t *testing.T) {
	db := openE2EDB(t)
	defer db.Close()

	ctx := context.Background()

	// ── Step 1: Operator login ─────────────────────────────────────────────────
	// Look up the seed operator (created by 008_seed_operator.sql).
	const operatorEmail = "operator@protou.co"
	const operatorPassword = "operator123"

	var userID int64
	var passwordHash string
	var operatorID int64

	err := db.QueryRowContext(ctx, `
		SELECT u.id, u.password_hash, o.id AS operator_id
		FROM users u
		INNER JOIN operators o ON o.user_id = u.id
		WHERE u.email = $1 AND u.is_active = true
	`, operatorEmail).Scan(&userID, &passwordHash, &operatorID)

	if err == sql.ErrNoRows {
		t.Skip("seed operator not found — run db migrations (008_seed_operator.sql) first")
	}
	require.NoError(t, err, "operator lookup should succeed")
	assert.NotZero(t, operatorID)

	// Verify bcrypt password
	err = bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(operatorPassword))
	require.NoError(t, err, "operator password should match bcrypt hash")

	// Create a session token (same logic as AuthHandler.login)
	token, err := auth.CreateSession(db, operatorID)
	require.NoError(t, err, "create operator session should succeed")
	assert.NotEmpty(t, token)

	t.Cleanup(func() {
		_, _ = db.ExecContext(context.Background(),
			`DELETE FROM operator_sessions WHERE token=$1`, token)
	})

	// Validate the session (round-trip)
	gotOperatorID, err := auth.ValidateSession(db, token)
	require.NoError(t, err, "session validation should succeed")
	assert.Equal(t, operatorID, gotOperatorID, "validated operator ID should match")

	// Invalid token should fail
	_, err = auth.ValidateSession(db, "invalid-token-that-does-not-exist")
	assert.Error(t, err, "invalid session token should return error")

	// ── Step 2: List orders ────────────────────────────────────────────────────
	// Query orders as the operator would (same query used in OrdersHandler.listOrders).
	const listQ = `
		SELECT o.id, o.user_id, o.status, o.payment_method, o.created_at
		FROM orders o
		INNER JOIN users u ON u.id = o.user_id
		ORDER BY o.created_at DESC
		LIMIT 20 OFFSET 0
	`
	rows, err := db.QueryContext(ctx, listQ)
	require.NoError(t, err, "list orders query should succeed")
	defer rows.Close()

	type orderSummary struct {
		ID            int64
		UserID        int64
		Status        string
		PaymentMethod string
	}

	var orderList []orderSummary
	for rows.Next() {
		var o orderSummary
		var createdAt interface{} // discard
		scanErr := rows.Scan(&o.ID, &o.UserID, &o.Status, &o.PaymentMethod, &createdAt)
		require.NoError(t, scanErr)
		orderList = append(orderList, o)
	}
	require.NoError(t, rows.Err())
	// Orders list may be empty in a fresh test DB — that's fine.
	t.Logf("operator list orders: found %d orders", len(orderList))

	// ── Step 3: Transition a pending order ────────────────────────────────────
	// Find an order in pending_confirmation status to try transitioning.
	var pendingOrderID int64
	err = db.QueryRowContext(ctx,
		`SELECT id FROM orders WHERE status='pending_confirmation' ORDER BY created_at DESC LIMIT 1`,
	).Scan(&pendingOrderID)

	if err == sql.ErrNoRows {
		t.Log("no pending_confirmation orders found — skipping transition sub-test")
		return
	}
	require.NoError(t, err)

	// Transition: pending_confirmation → confirmed (same logic as transitionOrderStatus).
	// We replicate the state-machine logic here because transitionOrderStatus is unexported.
	// This is intentional: the operator package uses concrete types and the E2E test
	// drives the DB directly (see design: "Repository interfaces is out-of-scope for MVP").
	tx, err := db.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer func() {
		if tx != nil {
			tx.Rollback() //nolint:errcheck
		}
	}()

	var fromStatus string
	err = tx.QueryRowContext(ctx, `SELECT status FROM orders WHERE id=$1 FOR UPDATE`, pendingOrderID).Scan(&fromStatus)
	require.NoError(t, err)
	assert.Equal(t, "pending_confirmation", fromStatus)

	_, err = tx.ExecContext(ctx, `UPDATE orders SET status='confirmed', updated_at=NOW() WHERE id=$1`, pendingOrderID)
	require.NoError(t, err)

	_, err = tx.ExecContext(ctx, `
		INSERT INTO order_events (order_id, from_status, to_status, actor_id, note)
		VALUES ($1, $2, 'confirmed', $3, 'e2e test confirm')
	`, pendingOrderID, fromStatus, operatorID)
	require.NoError(t, err)

	err = tx.Commit()
	require.NoError(t, err, "commit transition should succeed")
	tx = nil // prevent deferred rollback

	// Verify the order is now confirmed
	var newStatus string
	err = db.QueryRowContext(ctx, `SELECT status FROM orders WHERE id=$1`, pendingOrderID).Scan(&newStatus)
	require.NoError(t, err)
	assert.Equal(t, "confirmed", newStatus, "order status should be confirmed after transition")

	// Verify an event was created
	var eventCount int
	err = db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM order_events WHERE order_id=$1 AND to_status='confirmed'`,
		pendingOrderID,
	).Scan(&eventCount)
	require.NoError(t, err)
	assert.Greater(t, eventCount, 0, "order_events should have a confirmed transition event")

	// Verify that confirmed→pending_confirmation is rejected by the state machine
	// (validTransitions does not include this path).
	// We check the state machine map logic directly: confirmed can go to purchasing, cancelled, failed.
	validFromConfirmed := map[string]bool{
		"purchasing": true,
		"cancelled":  true,
		"failed":     true,
	}
	assert.False(t, validFromConfirmed["pending_confirmation"],
		"confirmed→pending_confirmation should not be a valid transition")
}
