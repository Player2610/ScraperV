package operator

import (
	"context"
	"database/sql"
	"log"

	"github.com/protou/protou/internal/notifications"
)

// notifier sends status-change notification emails.
type notifier struct {
	svc *notifications.Service
	db  *sql.DB
}

// sendStatusNotification looks up the student email and sends an appropriate email.
func (n *notifier) sendStatusNotification(ctx context.Context, db *sql.DB, orderID int64, toStatus string) {
	// Look up user email and name from the order
	const q = `
		SELECT u.email, u.name
		FROM orders o
		INNER JOIN users u ON u.id = o.user_id
		WHERE o.id = $1
	`
	var email, name string
	if err := db.QueryRowContext(ctx, q, orderID).Scan(&email, &name); err != nil {
		log.Printf("operator notify: lookup user for order %d: %v", orderID, err)
		return
	}

	var err error
	switch toStatus {
	case "confirmed":
		err = n.svc.SendOrderConfirmed(ctx, email, name, orderID)
	case "in_delivery":
		err = n.svc.SendOrderInDelivery(ctx, email, name, orderID)
	case "delivered":
		err = n.svc.SendOrderDelivered(ctx, email, name, orderID)
	case "cancelled":
		err = n.svc.SendOrderCancelled(ctx, email, name, orderID)
	default:
		// No email for other transitions
		return
	}
	if err != nil {
		log.Printf("operator notify: send %s email for order %d: %v", toStatus, orderID, err)
	}
}
