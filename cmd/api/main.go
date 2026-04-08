package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/protou/protou/internal/auth"
	"github.com/protou/protou/internal/cart"
	"github.com/protou/protou/internal/catalog"
	"github.com/protou/protou/internal/delivery"
	"github.com/protou/protou/internal/notifications"
	"github.com/protou/protou/internal/operator"
	"github.com/protou/protou/internal/orders"
	"github.com/protou/protou/internal/platform"
	"github.com/protou/protou/internal/users"
)

const (
	serverReadTimeout  = 15 * time.Second
	serverWriteTimeout = 30 * time.Second
	serverIdleTimeout  = 60 * time.Second
	shutdownTimeout    = 10 * time.Second
)

func main() {
	// Initialize structured JSON logging.
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})))

	cfg := platform.LoadConfig()

	db, err := platform.NewDB(cfg)
	if err != nil {
		slog.Error("failed to connect to database", "error", err)
		os.Exit(1)
	}
	defer db.Close()

	// Start operator session cleanup goroutine.
	auth.StartSessionCleanup(db)

	router := platform.NewServer(cfg, db)

	// Catalog
	catalogRepo := catalog.NewRepository(db)
	catalogSvc := catalog.NewService(catalogRepo)
	catalogHandler := catalog.NewHandler(catalogSvc)
	catalogHandler.RegisterRoutes(router)

	// Users + Auth
	userRepo := users.NewRepository(db)
	userSvc := users.NewService(userRepo)
	userHandler := users.NewHandler(userSvc)
	userHandler.RegisterRoutes(router)

	// Cart
	cartRepo := cart.NewRepository(db)
	cartSvc := cart.NewService(cartRepo, catalogRepo)
	cartHandler := cart.NewHandler(cartSvc)
	cartHandler.RegisterRoutes(router)

	// Delivery
	deliveryRepo := delivery.NewRepository(db)

	// Notifications
	notifSvc := notifications.NewService(db, cfg.ResendAPIKey, cfg.NotificationsFrom)

	// Orders
	orderRepo := orders.NewRepository(db)
	orderSvc := orders.NewService(orderRepo, cartSvc, catalogRepo, deliveryRepo, userRepo, notifSvc)
	orderHandler := orders.NewHandler(orderSvc)
	orderHandler.RegisterRoutes(router)

	// Operator panel
	operatorHandler := operator.NewHandler(db, notifSvc)
	operatorHandler.RegisterRoutes(router)

	addr := fmt.Sprintf(":%s", cfg.Port)

	srv := &http.Server{
		Addr:         addr,
		Handler:      router,
		ReadTimeout:  serverReadTimeout,
		WriteTimeout: serverWriteTimeout,
		IdleTimeout:  serverIdleTimeout,
	}

	// Graceful shutdown on SIGINT / SIGTERM.
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-quit
		slog.Info("shutdown signal received, draining connections")
		ctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer cancel()
		if err := srv.Shutdown(ctx); err != nil {
			slog.Error("graceful shutdown failed", "error", err)
		}
	}()

	slog.Info("server starting", "addr", addr)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		slog.Error("server failed", "error", err)
		os.Exit(1)
	}
	slog.Info("server stopped")
}
