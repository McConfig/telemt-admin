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

	"github.com/McConfig/telemt-admin/internal/cli"
	"github.com/McConfig/telemt-admin/internal/config"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "healthcheck" {
		if err := healthcheck(); err != nil {
			slog.Error("healthcheck", "err", err)
			os.Exit(1)
		}
		return
	}
	if len(os.Args) > 1 && os.Args[1] != "serve" {
		cfg, err := config.Load(os.Getenv)
		if err != nil {
			slog.Error("конфигурация", "err", err)
			os.Exit(1)
		}
		if err := cli.Run(context.Background(), os.Args[1:], cfg, os.Stdin, os.Stdout); err != nil {
			slog.Error("команда", "err", err)
			os.Exit(1)
		}
		return
	}
	if err := run(); err != nil {
		slog.Error("остановка", "err", err)
		os.Exit(1)
	}
}

func healthcheck() error {
	url := os.Getenv("TGADMIN_HEALTH_URL")
	if url == "" {
		url = "http://127.0.0.1:8090/readyz"
	}
	client := &http.Client{Timeout: 3 * time.Second}
	// #nosec G704 -- адрес задаётся оператором через TGADMIN_HEALTH_URL.
	resp, err := client.Get(url)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("readyz: %s", resp.Status)
	}
	return nil
}

func run() error {
	cfg, err := config.Load(os.Getenv)
	if err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	handler, closeFn, err := newHandler(ctx, cfg)
	if err != nil {
		return err
	}
	defer func() { _ = closeFn() }()
	srv := &http.Server{
		Addr:              cfg.Listen,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       120 * time.Second,
	}
	go func() {
		<-ctx.Done()
		shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdown)
	}()
	slog.Info("сервер запущен", "addr", cfg.Listen)
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}
