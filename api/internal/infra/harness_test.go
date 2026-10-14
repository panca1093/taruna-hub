package infra

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"

	"github.com/tarunaeduka/bimbel-tryout/api/internal/config"
	"github.com/tarunaeduka/bimbel-tryout/api/internal/model"
	"github.com/tarunaeduka/bimbel-tryout/api/internal/repository"
	"github.com/tarunaeduka/bimbel-tryout/api/internal/service"
)

// fastRetry keeps integration retries in the millisecond range; production uses service.RetryDelay.
func fastRetry(int) time.Duration { return 25 * time.Millisecond }

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelDebug}))
}

type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

type harness struct {
	t        *testing.T
	pool     *pgxpool.Pool
	repo     *repository.NotificationRepository
	svc      *service.NotificationService
	inserter *RiverInserter
	jobs     service.JobInserter
	senders  map[string]service.Sender
	smtp     *fakeSMTP
	smtpCfg  config.SMTPConfig
	logger   *slog.Logger
	tenant   uuid.UUID
	// logOutput exposes whatever the harness logger captured; tests that assert on redaction set it.
	logOutput func() string

	mu sync.Mutex
	// owned is every notification id this harness created, so cleanup deletes exactly those rows.
	owned []uuid.UUID
}

type harnessOpts struct {
	// noSMTP leaves email unconfigured, which is how the unavailable-channel case is built.
	noSMTP     bool
	smtpMutate func(*config.SMTPConfig)
	sealKey    []byte
	clock      func() time.Time
	logger     *slog.Logger
	// wrapJobs wraps the service's job inserter; a rollback test records what the service inserted.
	wrapJobs func(service.JobInserter) service.JobInserter
	// reconcileLimit lowers the service's maintenance bound so a test can assert its own rows.
	reconcileLimit int
	// emailSender replaces the SMTP transport, so a test can hold a send open and watch its context.
	emailSender service.Sender
	// wrapEmail wraps whatever email transport the harness built, so a test can gate the real send.
	wrapEmail func(service.Sender) service.Sender
}

func newHarness(t *testing.T, opts harnessOpts) *harness {
	t.Helper()
	pool := testPool(t)
	ctx := context.Background()

	logger := opts.logger
	if logger == nil {
		logger = testLogger()
	}

	tenants := repository.NewTenantRepository(pool)
	tenant, err := tenants.Create(ctx, "p02-"+uuid.NewString()[:8], "P02 harness")
	if err != nil {
		t.Fatalf("create tenant: %v", err)
	}

	h := &harness{t: t, pool: pool, repo: repository.NewNotificationRepository(pool), smtp: newFakeSMTP(t), logger: logger, tenant: tenant.ID, logOutput: func() string { return "" }}
	t.Cleanup(h.cleanup)

	inserter, err := NewRiverInserter(pool, logger)
	if err != nil {
		t.Fatalf("river inserter: %v", err)
	}
	h.inserter = inserter
	h.jobs = inserter
	if opts.wrapJobs != nil {
		h.jobs = opts.wrapJobs(inserter)
	}

	if !opts.noSMTP {
		host, port := h.smtp.hostPort(t)
		h.smtpCfg = config.SMTPConfig{Host: host, Port: port, From: "no-reply@tarunaeduka.id", TLS: config.SMTPTLSNone}
		if opts.smtpMutate != nil {
			opts.smtpMutate(&h.smtpCfg)
		}
	}

	var svcOpts []service.Option
	if opts.clock != nil {
		svcOpts = append(svcOpts, service.WithClock(opts.clock))
	}
	if opts.reconcileLimit > 0 {
		svcOpts = append(svcOpts, service.WithReconcileLimit(opts.reconcileLimit))
	}
	senders := map[string]service.Sender{
		string(model.ChannelEmail):    NewSMTPSender(h.smtpCfg, logger),
		string(model.ChannelWhatsApp): UnavailableSender{},
	}
	if opts.emailSender != nil {
		senders[string(model.ChannelEmail)] = opts.emailSender
	}
	if opts.wrapEmail != nil {
		senders[string(model.ChannelEmail)] = opts.wrapEmail(senders[string(model.ChannelEmail)])
	}
	h.senders = senders
	svc, err := service.NewNotificationService(h.repo, h.jobs, senders, opts.sealKey, logger, svcOpts...)
	if err != nil {
		t.Fatalf("notification service: %v", err)
	}
	h.svc = svc
	return h
}

// serviceWithKey rebuilds the service with another seal key over the same transports.
func (h *harness) serviceWithKey(key []byte) *service.NotificationService {
	h.t.Helper()
	svc, err := service.NewNotificationService(h.repo, h.jobs, h.senders, key, h.logger)
	if err != nil {
		h.t.Fatalf("notification service with another key: %v", err)
	}
	return svc
}

func (h *harness) cleanup() {
	ctx := context.Background()
	h.mu.Lock()
	owned := append([]uuid.UUID(nil), h.owned...)
	h.mu.Unlock()
	if len(owned) > 0 {
		ids := make([]string, 0, len(owned))
		for _, id := range owned {
			ids = append(ids, id.String())
		}
		// Only the ids this harness recorded, so a parallel package's rows are never touched.
		_, _ = h.pool.Exec(ctx, `DELETE FROM river.river_job WHERE args->>'notification_id' = ANY($1)`, ids)
		_, _ = h.pool.Exec(ctx, `DELETE FROM notification_logs WHERE id::text = ANY($1)`, ids)
	}
	_, _ = h.pool.Exec(ctx, `DELETE FROM tenants WHERE id = $1`, h.tenant)
}

// record keeps the ids this harness created so cleanup can delete exactly rows it owns.
func (h *harness) record(ids ...uuid.UUID) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.owned = append(h.owned, ids...)
}

func (h *harness) notificationRowExists(id uuid.UUID) bool {
	return h.count(`SELECT count(*) FROM notification_logs WHERE id = $1`, id) > 0
}

func (h *harness) jobExistsFor(id uuid.UUID) bool {
	return h.count(`SELECT count(*) FROM river.river_job WHERE args->>'notification_id' = $1`, id.String()) > 0
}

func (h *harness) tenantScope() *uuid.UUID { return &h.tenant }

// scope is the harness tenant; a test wanting the platform scope passes service.Scope{} itself.
func (h *harness) scope() service.Scope { return service.Scope{TenantID: h.tenantScope()} }

func (h *harness) newNotification(eventKey string, mutate func(*service.NewNotification)) service.NewNotification {
	n := service.NewNotification{
		Scope:     h.scope(),
		EventKey:  eventKey,
		Channel:   string(model.ChannelEmail),
		Recipient: "student@example.com",
		Template:  "smtp-test",
	}
	if mutate != nil {
		mutate(&n)
	}
	return n
}

// enqueue commits one notification through the caller-facing path.
func (h *harness) enqueue(n service.NewNotification) uuid.UUID {
	h.t.Helper()
	ctx := context.Background()
	tx, err := h.pool.Begin(ctx)
	if err != nil {
		h.t.Fatalf("begin: %v", err)
	}
	if err := h.svc.Enqueue(ctx, tx, n); err != nil {
		_ = tx.Rollback(ctx)
		h.t.Fatalf("enqueue: %v", err)
	}
	if err := tx.Commit(ctx); err != nil {
		h.t.Fatalf("commit: %v", err)
	}
	id := h.lastRowID(n.EventKey)
	h.record(id)
	return id
}

// enqueueTx runs Enqueue inside a caller-supplied transaction so rollback can be asserted.
func (h *harness) enqueueTx(ctx context.Context, tx pgx.Tx, n service.NewNotification) error {
	return h.svc.Enqueue(ctx, tx, n)
}

func (h *harness) lastRowID(eventKey string) uuid.UUID {
	h.t.Helper()
	var id uuid.UUID
	if err := h.pool.QueryRow(context.Background(),
		`SELECT id FROM notification_logs WHERE event_key = $1 ORDER BY created_at DESC LIMIT 1`, eventKey).Scan(&id); err != nil {
		h.t.Fatalf("find row for %s: %v", eventKey, err)
	}
	return id
}

func (h *harness) row(id uuid.UUID) model.Notification {
	h.t.Helper()
	n, err := h.repo.FindForDelivery(context.Background(), id)
	if err != nil {
		h.t.Fatalf("read row %s: %v", id, err)
	}
	return n
}

func (h *harness) sql(query string, args ...any) {
	h.t.Helper()
	if _, err := h.pool.Exec(context.Background(), query, args...); err != nil {
		h.t.Fatalf("exec %s: %v", query, err)
	}
}

func (h *harness) count(query string, args ...any) int {
	h.t.Helper()
	var n int
	if err := h.pool.QueryRow(context.Background(), query, args...).Scan(&n); err != nil {
		h.t.Fatalf("count %s: %v", query, err)
	}
	return n
}

// jobState reads the River side of one notification.
func (h *harness) jobState(id uuid.UUID) (state string, attempt, maxAttempts int, errors int) {
	h.t.Helper()
	err := h.pool.QueryRow(context.Background(),
		`SELECT state::text, attempt, max_attempts, coalesce(array_length(errors, 1), 0)
		   FROM river.river_job WHERE kind = 'notification' AND args->>'notification_id' = $1`,
		id.String()).Scan(&state, &attempt, &maxAttempts, &errors)
	if err != nil {
		h.t.Fatalf("read job for %s: %v", id, err)
	}
	return state, attempt, maxAttempts, errors
}

func (h *harness) jobErrors(id uuid.UUID) string {
	h.t.Helper()
	var out *string
	if err := h.pool.QueryRow(context.Background(),
		`SELECT errors::text FROM river.river_job WHERE kind = 'notification' AND args->>'notification_id' = $1`,
		id.String()).Scan(&out); err != nil {
		h.t.Fatalf("read job errors: %v", err)
	}
	if out == nil {
		return ""
	}
	return *out
}

func (h *harness) workerClient(t *testing.T, retry func(int) time.Duration, opts ...RiverWorkerOption) *river.Client[pgx.Tx] {
	t.Helper()
	client, err := NewRiverWorker(h.pool, h.svc, retry, h.logger, opts...)
	if err != nil {
		t.Fatalf("river worker client: %v", err)
	}
	return client
}

// startWorker starts a worker client and stops it when the test ends.
func (h *harness) startWorker(t *testing.T, retry func(int) time.Duration, opts ...RiverWorkerOption) *river.Client[pgx.Tx] {
	t.Helper()
	client := h.workerClient(t, retry, opts...)
	ctx, cancel := context.WithCancel(context.Background())
	if err := client.Start(ctx); err != nil {
		cancel()
		t.Fatalf("start worker: %v", err)
	}
	t.Cleanup(func() {
		stopCtx, stopCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer stopCancel()
		_ = client.StopAndCancel(stopCtx)
		cancel()
	})
	return client
}

// waitFor polls a bounded condition; it is a barrier, not a fixed sleep.
func waitFor(t *testing.T, what string, cond func() bool) {
	t.Helper()
	waitForWithin(t, 10*time.Second, what, cond)
}

// waitForWithin is waitFor with a bigger budget for River's unconfigurable five-second retry tick.
func waitForWithin(t *testing.T, budget time.Duration, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(budget)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("timed out after %s waiting for %s", budget, what)
}

func waitForMessage(t *testing.T, s *fakeSMTP, want int) {
	t.Helper()
	waitFor(t, "fake smtp messages", func() bool { return s.messageCount() >= want })
}
