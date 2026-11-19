package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/niteshs1/streakit-api/internal/config"
	"github.com/niteshs1/streakit-api/internal/database"
	"github.com/niteshs1/streakit-api/internal/handler"
	"github.com/niteshs1/streakit-api/internal/routes"
	"github.com/niteshs1/streakit-api/internal/server"
)

func main() {
	// All real work lives in run() so deferred cleanup (DB pool close,
	// signal unregistration) always executes; os.Exit skips defers.
	if err := run(); err != nil {
		slog.Error("application exited with error", "error", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load configuration: %w", err)
	}

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: cfg.App.LogLevel,
	})).With("app", cfg.App.Name, "env", cfg.App.Env)
	slog.SetDefault(logger)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	db, err := database.NewPostgres(ctx, cfg.Database)
	if err != nil {
		return fmt.Errorf("database connection: %w", err)
	}
	defer func() {
		if err := db.Close(); err != nil {
			logger.Error("closing database", "error", err)
		} else {
			logger.Info("database connection closed")
		}
	}()
	logger.Info("database connected")

	if cfg.Database.AutoMigrate {
		if err := db.Ent.Schema.Create(ctx); err != nil {
			return fmt.Errorf("database schema migration: %w", err)
		}
		logger.Info("database schema synchronized")
	} else {
		logger.Info("auto-migration disabled; skipping schema sync")
	}

	router, err := server.NewRouter(cfg.App, logger, routes.Handlers{
		Health: handler.NewHealthHandler(db),
	})
	if err != nil {
		return err
	}

	httpServer := server.NewHTTPServer(cfg.App, router)

	// Bind synchronously so "port already in use" fails fast and the
	// "server started" log is only emitted once we are actually listening.
	listener, err := net.Listen("tcp", httpServer.Addr)
	if err != nil {
		return fmt.Errorf("listen on %s: %w", httpServer.Addr, err)
	}

	serverErrors := make(chan error, 1)
	go func() {
		logger.Info("server started", "address", listener.Addr().String())
		if err := httpServer.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErrors <- err
		}
		close(serverErrors)
	}()

	select {
	case err := <-serverErrors:
		return fmt.Errorf("http server: %w", err)
	case <-ctx.Done():
		logger.Info("shutdown signal received")
	}

	// Restore default signal behaviour: a second Ctrl+C / SIGTERM now
	// terminates immediately instead of being swallowed.
	stop()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.App.ShutdownTimeout)
	defer cancel()

	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		_ = httpServer.Close()
		return fmt.Errorf("graceful shutdown: %w", err)
	}

	logger.Info("server stopped gracefully")
	return nil
}
