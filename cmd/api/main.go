package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"agnos-backend-assignment/internal/auth"
	"agnos-backend-assignment/internal/client"
	"agnos-backend-assignment/internal/config"
	"agnos-backend-assignment/internal/database"
	"agnos-backend-assignment/internal/handler"
	"agnos-backend-assignment/internal/repository"
	"agnos-backend-assignment/internal/service"
)

func main() {
	if err := run(); err != nil {
		slog.Error("application stopped", "error", err)
		os.Exit(1)
	}
}
func run() error {
	command := "serve"
	if len(os.Args) > 1 {
		command = os.Args[1]
	}
	if command != "serve" && command != "migrate" && command != "seed" {
		return fmt.Errorf("usage: api [serve|migrate|seed]")
	}
	var cfg config.Config
	if command == "serve" {
		var err error
		cfg, err = config.Load()
		if err != nil {
			return err
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if os.Getenv("DATABASE_URL") == "" {
		return fmt.Errorf("DATABASE_URL is required")
	}
	pool, err := database.Open(ctx, os.Getenv("DATABASE_URL"))
	if err != nil {
		return fmt.Errorf("database connection failed")
	}
	defer pool.Close()
	switch command {
	case "migrate":
		return database.Migrate(ctx, pool)
	case "seed":
		a := os.Getenv("HOSPITAL_A_API_URL")
		if a == "" {
			a = "https://hospital-a.api.co.th"
		}
		return database.Seed(ctx, pool, a, os.Getenv("HOSPITAL_B_API_URL"))
	}
	svc := service.New(&repository.Postgres{Pool: pool}, client.New(cfg.HISTimeout), auth.New(cfg.JWTSecret, cfg.TokenTTL))
	server := &http.Server{Addr: cfg.Address, Handler: handler.Router(svc, cfg.ProvisioningKey, pool.Ping), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 10 * time.Second, WriteTimeout: 25 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 16 << 10}
	signalCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	done := make(chan error, 1)
	go func() { slog.Info("HTTP server starting", "address", cfg.Address); done <- server.ListenAndServe() }()
	select {
	case err = <-done:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	case <-signalCtx.Done():
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer shutdownCancel()
		return server.Shutdown(shutdownCtx)
	}
}
