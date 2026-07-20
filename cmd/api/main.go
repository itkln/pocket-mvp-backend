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

	"pocket-mvp-backend/internal/buildinfo"
	"pocket-mvp-backend/internal/config"
	"pocket-mvp-backend/internal/database"
	"pocket-mvp-backend/internal/httpapi"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		slog.Error("load configuration", "error", err)
		os.Exit(1)
	}

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: cfg.LogLevel,
	}))
	slog.SetDefault(logger)

	startupCtx, cancelStartup := context.WithTimeout(context.Background(), cfg.DatabaseConnectTimeout)
	defer cancelStartup()

	db, err := database.Open(startupCtx, cfg.DatabaseURL, cfg.DatabaseMaxConnections)
	if err != nil {
		logger.Error("connect to database", "error", err)
		os.Exit(1)
	}
	defer db.Close()

	handler := httpapi.New(httpapi.Dependencies{
		Database:       db,
		Logger:         logger,
		AllowedOrigins: cfg.AllowedOrigins,
		Build:          buildinfo.Current(),
	})
	server := &http.Server{
		Addr:              cfg.HTTPAddress,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	serverErrors := make(chan error, 1)
	go func() {
		logger.Info("api started",
			"address", cfg.HTTPAddress,
			"environment", cfg.Environment,
			"version", buildinfo.Version,
		)
		serverErrors <- server.ListenAndServe()
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)

	select {
	case sig := <-stop:
		logger.Info("shutdown requested", "signal", sig.String())
	case err := <-serverErrors:
		if !errors.Is(err, http.ErrServerClosed) {
			logger.Error("api stopped unexpectedly", "error", err)
			os.Exit(1)
		}
	}

	shutdownCtx, cancelShutdown := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancelShutdown()
	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.Error("graceful shutdown failed", "error", err)
		_ = server.Close()
	}
	logger.Info("api stopped")
}
