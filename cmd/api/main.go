package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/niteshs1/streakit-api/internal/config"
	"github.com/niteshs1/streakit-api/internal/database"
	"github.com/niteshs1/streakit-api/internal/server"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	cfg, err := config.Load()
	if err != nil {
		logger.Error("failed to load configuration", "error", err)
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(
		context.Background(),
		os.Interrupt,
		syscall.SIGTERM,
	)
	defer stop()

	dbClient, err := database.NewPostgresClient(ctx, cfg.Database)
	if err != nil {
		logger.Error("database connection failed", "error", err)
		os.Exit(1)
	}
	defer dbClient.Close()

	logger.Info("database connected")

	if err := dbClient.Schema.Create(ctx); err != nil {
		logger.Error("database schema migration failed", "error", err)
		os.Exit(1)
	}

	logger.Info("database schema synchronized")

	router := server.New(cfg.App)

	httpServer := &http.Server{
		Addr:              server.Address(cfg.App),
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	serverErrors := make(chan error, 1)

	go func() {
		logger.Info(
			"server started",
			"application", cfg.App.Name,
			"environment", cfg.App.Env,
			"address", httpServer.Addr,
		)

		if err := httpServer.ListenAndServe(); err != nil &&
			!errors.Is(err, http.ErrServerClosed) {
			serverErrors <- err
		}
	}()

	select {
	case err := <-serverErrors:
		logger.Error("HTTP server failed", "error", err)
		os.Exit(1)

	case <-ctx.Done():
		logger.Info("shutdown signal received")
	}

	shutdownCtx, cancel := context.WithTimeout(
		context.Background(),
		10*time.Second,
	)
	defer cancel()

	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		logger.Error("graceful shutdown failed", "error", err)
		return
	}

	logger.Info("server stopped gracefully")
}
