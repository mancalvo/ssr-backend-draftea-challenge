package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/mancalvo/ssr-backend-draftea-challenge/internal/app"
	"github.com/mancalvo/ssr-backend-draftea-challenge/internal/config"
	dbseeds "github.com/mancalvo/ssr-backend-draftea-challenge/internal/infrastructure/database"
	"github.com/mancalvo/ssr-backend-draftea-challenge/internal/infrastructure/database/seeds"
	infrahttp "github.com/mancalvo/ssr-backend-draftea-challenge/internal/infrastructure/http"
	"github.com/mancalvo/ssr-backend-draftea-challenge/pkg/logger"
)

func main() {
	// Load configuration
	cfg := config.Load()

	// Initialize database connection
	db, err := dbseeds.NewPostgresConnection(cfg.Database)
	if err != nil {
		logger.Error("failed to connect to database", "error", err)
		os.Exit(1)
	}
	defer db.Close()

	// Run migrations
	if err := dbseeds.MigrateUp(cfg.Database); err != nil {
		logger.Error("failed to run migrations", "error", err)
		os.Exit(1)
	}

	// Run seeds if in development environment
	if err := seeds.Run(context.Background(), db.DB, cfg); err != nil {
		logger.Error("failed to run seeds", "error", err)
		os.Exit(1)
	}

	// Initialize dependencies
	container := app.NewContainer(db)

	// Build router with dependencies
	router := infrahttp.NewRouter(
		container.WalletHandlers,
		container.PaymentHandlers,
		container.EntitlementHandlers,
	)

	// Apply global middleware
	handler := infrahttp.Chain(
		router,
		infrahttp.ContentType,
		infrahttp.Logger,
		infrahttp.Recoverer,
	)

	// Create and start server
	server := infrahttp.NewServer(":"+cfg.Server.Port, handler)

	// Graceful shutdown
	go func() {
		if err := server.Start(); err != nil && err != http.ErrServerClosed {
			logger.Error("server failed", "error", err)
			os.Exit(1)
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		logger.Error("server forced to shutdown", "error", err)
		os.Exit(1)
	}

	logger.Info("server exited gracefully")
}
