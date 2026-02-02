package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/mancalvo/ssr-backend-draftea-challenge/internal/config"
	"github.com/mancalvo/ssr-backend-draftea-challenge/internal/infrastructure/database"
	httpserver "github.com/mancalvo/ssr-backend-draftea-challenge/internal/infrastructure/http"
	"github.com/mancalvo/ssr-backend-draftea-challenge/internal/routes"
	"github.com/mancalvo/ssr-backend-draftea-challenge/pkg/logger"
)

func main() {
	// Load configuration
	cfg := config.Load()

	// Initialize database connection
	db, err := database.NewPostgresConnection(cfg.Database)
	if err != nil {
		logger.Error("failed to connect to database", "error", err)
		os.Exit(1)
	}
	defer db.Close()

	// Build router with dependencies
	router := routes.NewRouter(db)

	// Apply global middleware
	handler := httpserver.Chain(
		router,
		httpserver.ContentType,
		httpserver.Logger,
		httpserver.Recoverer,
	)

	// Create and start server
	server := httpserver.NewServer(":"+cfg.Server.Port, handler)

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
