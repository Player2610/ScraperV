// Package operator provides the operator panel HTTP handlers and middleware.
package operator

import (
	"database/sql"

	"github.com/go-chi/chi/v5"

	"github.com/protou/protou/internal/notifications"
)

// Handler is the root operator panel handler that registers all sub-routes.
type Handler struct {
	db              *sql.DB
	authHandler     *AuthHandler
	ordersHandler   *OrdersHandler
	couriersHandler *CouriersHandler
	deliveryHandler *DeliveryConfigHandler
	logsHandler     *LogsHandler
}

// NewHandler creates the operator Handler with all dependencies wired.
func NewHandler(db *sql.DB, notifSvc *notifications.Service) *Handler {
	n := &notifier{svc: notifSvc, db: db}
	return &Handler{
		db:              db,
		authHandler:     NewAuthHandler(db),
		ordersHandler:   NewOrdersHandler(db, n),
		couriersHandler: NewCouriersHandler(db),
		deliveryHandler: NewDeliveryConfigHandler(db),
		logsHandler:     NewLogsHandler(db),
	}
}

// RegisterRoutes mounts all operator routes onto r.
// Login/logout are public; everything else is protected by RequireOperatorSession.
func (h *Handler) RegisterRoutes(r chi.Router) {
	// Public auth routes
	h.authHandler.RegisterRoutes(r)

	// Protected operator routes
	r.Group(func(r chi.Router) {
		r.Use(RequireOperatorSession(h.db))
		h.ordersHandler.RegisterRoutes(r)
		h.couriersHandler.RegisterRoutes(r)
		h.deliveryHandler.RegisterRoutes(r)
		h.logsHandler.RegisterRoutes(r)
	})
}
