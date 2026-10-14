package infra

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/riverqueue/river"

	"github.com/tarunaeduka/bimbel-tryout/api/internal/model"
	"github.com/tarunaeduka/bimbel-tryout/api/internal/service"
)

// gateSender holds only the first Send, so a test can interrupt one attempt and observe the retry.
type gateSender struct {
	// inner is the real transport; nil makes the gate the whole transport.
	inner service.Sender

	started chan struct{}
	release chan struct{}

	startOnce sync.Once

	mu     sync.Mutex
	ctxErr error
}

func (g *gateSender) Configured() bool { return true }

func (g *gateSender) Send(ctx context.Context, msg service.Message) error {
	var first bool
	g.startOnce.Do(func() {
		first = true
		close(g.started)
	})
	if !first {
		return g.forward(ctx, msg)
	}
	select {
	case <-g.release:
		g.record(ctx.Err())
		return g.forward(ctx, msg)
	case <-ctx.Done():
		g.record(ctx.Err())
		return ctx.Err()
	}
}

func (g *gateSender) forward(ctx context.Context, msg service.Message) error {
	if g.inner == nil {
		return nil
	}
	return g.inner.Send(ctx, msg)
}

func (g *gateSender) record(err error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.ctxErr = err
}

func (g *gateSender) cancelled() error {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.ctxErr
}

// stopSignal wraps the real client so the test can observe that a graceful stop was requested.
type stopSignal struct {
	*river.Client[pgx.Tx]

	once     sync.Once
	stopping chan struct{}
}

func (s *stopSignal) Stop(ctx context.Context) error {
	s.once.Do(func() { close(s.stopping) })
	return s.Client.Stop(ctx)
}

// A graceful stop must let an in-flight send finish, so the send fails if its context is cancelled.
func TestShutdownLetsAnInFlightSendFinish(t *testing.T) {
	gate := &gateSender{started: make(chan struct{}), release: make(chan struct{})}
	h := newHarness(t, harnessOpts{emailSender: gate})

	client := h.workerClient(t, fastRetry)
	if err := client.Start(context.Background()); err != nil {
		t.Fatalf("start worker: %v", err)
	}
	t.Cleanup(func() {
		stopCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = client.StopAndCancel(stopCtx)
	})

	id := h.enqueue(h.newNotification(eventKey("shutdown"), nil))

	select {
	case <-gate.started:
	case <-time.After(30 * time.Second):
		t.Fatal("the worker never started the send")
	}

	sig := &stopSignal{Client: client, stopping: make(chan struct{})}
	stopDone := make(chan error, 1)
	go func() { stopDone <- StopGracefully(context.Background(), sig, 30*time.Second) }()

	// Release the send only once Stop has been requested, so this cannot pass vacuously.
	select {
	case <-sig.stopping:
	case <-time.After(30 * time.Second):
		t.Fatal("the shutdown path never called Stop")
	}
	close(gate.release)

	if err := <-stopDone; err != nil {
		t.Fatalf("shutdown: %v", err)
	}
	if err := gate.cancelled(); err != nil {
		t.Errorf("the in-flight send was cancelled: %v", err)
	}
	if row := h.row(id); row.Status != model.StatusSent {
		t.Errorf("status = %s, want sent: a graceful stop must let the send finish", row.Status)
	}
}

// fakeStopper answers each stop with a fixed error and counts which one was asked for.
type fakeStopper struct {
	stopErr error

	mu          sync.Mutex
	graceCalls  int
	cancelCalls int
}

func (f *fakeStopper) Stop(context.Context) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.graceCalls++
	return f.stopErr
}

func (f *fakeStopper) StopAndCancel(context.Context) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.cancelCalls++
	return nil
}

func TestStopGracefullyEscalatesOnlyWhenTheWindowIsSpent(t *testing.T) {
	timed := &fakeStopper{}
	if err := StopGracefully(context.Background(), timed, time.Second); err != nil {
		t.Fatalf("stop that finishes in time: %v", err)
	}
	if timed.graceCalls != 1 || timed.cancelCalls != 0 {
		t.Errorf("calls = stop %d cancel %d, want 1 and 0: a timely stop never escalates", timed.graceCalls, timed.cancelCalls)
	}

	spent := &fakeStopper{stopErr: context.DeadlineExceeded}
	if err := StopGracefully(context.Background(), spent, time.Millisecond); err != nil {
		t.Fatalf("stop whose window ran out: %v", err)
	}
	if spent.graceCalls != 1 || spent.cancelCalls != 1 {
		t.Errorf("calls = stop %d cancel %d, want 1 and 1", spent.graceCalls, spent.cancelCalls)
	}

	broken := &fakeStopper{stopErr: errors.New("stop failed")}
	if err := StopGracefully(context.Background(), broken, time.Second); err == nil {
		t.Error("a failing graceful stop was reported as a clean shutdown")
	}
	if broken.cancelCalls != 0 {
		t.Errorf("cancel calls = %d, want 0: only a spent window escalates", broken.cancelCalls)
	}
}
