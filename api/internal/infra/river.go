package infra

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"

	"github.com/tarunaeduka/bimbel-tryout/api/internal/service"
)

// riverSchema is the non-default schema golang-migrate owns; River's own migrator is never used.
const riverSchema = "river"

const (
	notificationMaxWorkers = 10
	snoozeDuration         = 30 * time.Second
	// hardStopGrace bounds the escalation once the graceful shutdown window is spent.
	hardStopGrace = 5 * time.Second
)

// RiverStopper is the part of *river.Client the graceful shutdown path uses.
type RiverStopper interface {
	Stop(ctx context.Context) error
	StopAndCancel(ctx context.Context) error
}

// StopGracefully stops within grace, hard-stopping only when spent: a shutdown signal must not cancel the window.
func StopGracefully(ctx context.Context, client RiverStopper, grace time.Duration) error {
	base := context.WithoutCancel(ctx)
	gracefulCtx, cancel := context.WithTimeout(base, grace)
	defer cancel()
	if err := client.Stop(gracefulCtx); !errors.Is(err, context.DeadlineExceeded) {
		return err
	}
	hardCtx, hardCancel := context.WithTimeout(base, hardStopGrace)
	defer hardCancel()
	return client.StopAndCancel(hardCtx)
}

// RiverInserter is the API's write side: it only ever inserts into a caller's transaction.
type RiverInserter struct {
	client *river.Client[pgx.Tx]
}

// NewRiverInserter builds an insert-only client: no queues, no workers, and it is never started.
func NewRiverInserter(pool *pgxpool.Pool, logger *slog.Logger) (*RiverInserter, error) {
	client, err := river.NewClient[pgx.Tx](riverpgxv5.New(pool), &river.Config{
		Schema: riverSchema,
		Logger: logger,
	})
	if err != nil {
		return nil, err
	}
	return &RiverInserter{client: client}, nil
}

func (i *RiverInserter) InsertNotificationTx(ctx context.Context, tx pgx.Tx, args service.NotificationJobArgs) error {
	_, err := i.client.InsertTx(ctx, tx, args, &river.InsertOpts{
		Queue:       service.QueueNotifications,
		MaxAttempts: service.NotificationMaxAttempts,
	})
	return err
}

// notificationWorker maps delivery outcomes onto River's vocabulary; the service never sees River.
type notificationWorker struct {
	river.WorkerDefaults[service.NotificationJobArgs]

	deliver    *service.NotificationService
	retryDelay func(attempt int) time.Duration
}

func (w *notificationWorker) Work(ctx context.Context, job *river.Job[service.NotificationJobArgs]) error {
	err := w.deliver.Deliver(ctx, job.Args.NotificationID, service.Attempt{N: job.Attempt, Max: job.MaxAttempts})
	switch {
	case err == nil:
		return nil
	case errors.Is(err, service.ErrBusy):
		// A snooze decrements Attempt back, so waiting for another owner costs no attempt budget.
		return river.JobSnooze(snoozeDuration)
	case service.IsPermanent(err):
		return river.JobCancel(err)
	default:
		return err
	}
}

// NextRetry is injected rather than hard-coded so integration tests can compress the schedule.
func (w *notificationWorker) NextRetry(job *river.Job[service.NotificationJobArgs]) time.Time {
	return time.Now().Add(w.retryDelay(job.Attempt))
}

type riverWorkerConfig struct {
	maxAttempts int
}

// RiverWorkerOption tunes the worker client; production passes none of them.
type RiverWorkerOption func(*riverWorkerConfig)

// WithClientMaxAttempts changes the client default, so a test can show the per-job budget wins.
func WithClientMaxAttempts(n int) RiverWorkerOption {
	return func(c *riverWorkerConfig) { c.maxAttempts = n }
}

// NewRiverWorker builds the starting client: one queue, one worker, the app's logger.
func NewRiverWorker(pool *pgxpool.Pool, svc *service.NotificationService, retryDelay func(attempt int) time.Duration, logger *slog.Logger, opts ...RiverWorkerOption) (*river.Client[pgx.Tx], error) {
	cfg := riverWorkerConfig{}
	for _, opt := range opts {
		opt(&cfg)
	}

	workers := river.NewWorkers()
	river.AddWorker(workers, &notificationWorker{deliver: svc, retryDelay: retryDelay})

	return river.NewClient[pgx.Tx](riverpgxv5.New(pool), &river.Config{
		Schema:      riverSchema,
		Logger:      logger,
		MaxAttempts: cfg.maxAttempts,
		Queues: map[string]river.QueueConfig{
			service.QueueNotifications: {MaxWorkers: notificationMaxWorkers},
		},
		Workers: workers,
	})
}
