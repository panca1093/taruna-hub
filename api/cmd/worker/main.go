package main

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/tarunaeduka/bimbel-tryout/api/internal/config"
	"github.com/tarunaeduka/bimbel-tryout/api/internal/infra"
	"github.com/tarunaeduka/bimbel-tryout/api/internal/repository"
	"github.com/tarunaeduka/bimbel-tryout/api/internal/service"
)

const reconcileInterval = 10 * time.Minute

func main() {
	if err := run(); err != nil {
		slog.Error("fatal", slog.String("error", err.Error()))
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	logger := newLogger(cfg)
	slog.SetDefault(logger)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pool, err := infra.NewPostgresPool(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()

	// The worker never enqueues, but the service is built the same way in both binaries.
	inserter, err := infra.NewRiverInserter(pool, logger)
	if err != nil {
		return err
	}
	notifications, err := service.NewNotificationService(
		repository.NewNotificationRepository(pool),
		inserter,
		infra.NewSenders(cfg.SMTP, logger),
		cfg.NotificationSealKey,
		logger,
	)
	if err != nil {
		return err
	}

	client, err := infra.NewRiverWorker(pool, notifications, service.RetryDelay, logger)
	if err != nil {
		return err
	}
	// The client runs on a context no signal cancels: a cancelled Start context is a hard stop.
	if err := client.Start(context.Background()); err != nil {
		return err
	}
	logger.Info("worker started", slog.String("env", cfg.Env), slog.String("queue", service.QueueNotifications))

	go func() {
		ticker := time.NewTicker(reconcileInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				reconcile(ctx, notifications, logger)
			}
		}
	}()
	reconcile(ctx, notifications, logger)

	<-ctx.Done()
	logger.Info("shutting down")
	return infra.StopGracefully(ctx, client, cfg.ShutdownGrace)
}

func reconcile(ctx context.Context, notifications *service.NotificationService, logger *slog.Logger) {
	failed, err := notifications.Reconcile(ctx)
	if err != nil && !errors.Is(err, context.Canceled) {
		logger.Error("reconcile failed", slog.String("error", err.Error()))
		return
	}
	if failed > 0 {
		logger.Warn("reconciled orphaned notifications", slog.Int("count", failed))
	}
}

func newLogger(cfg config.Config) *slog.Logger {
	if cfg.IsProduction() {
		return slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	}
	return slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))
}
