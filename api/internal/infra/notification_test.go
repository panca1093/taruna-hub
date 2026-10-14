package infra

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"log/slog"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"

	"github.com/tarunaeduka/bimbel-tryout/api/internal/config"
	"github.com/tarunaeduka/bimbel-tryout/api/internal/model"
	"github.com/tarunaeduka/bimbel-tryout/api/internal/service"
)

func notificationMax() int { return service.NotificationMaxAttempts }

func eventKey(prefix string) string { return prefix + "-" + uuid.NewString()[:8] }

// outOf returns the pointer form the repository takes for a recorded verdict.
func outOf(o model.Outcome) *model.Outcome { return &o }

// recordingInserter records every id the service hands to the job insert; fail makes it fail.
type recordingInserter struct {
	inner service.JobInserter
	fail  error

	mu  sync.Mutex
	ids []uuid.UUID
}

func (r *recordingInserter) wrap(inner service.JobInserter) service.JobInserter {
	r.inner = inner
	return r
}

func (r *recordingInserter) InsertNotificationTx(ctx context.Context, tx pgx.Tx, args service.NotificationJobArgs) error {
	r.mu.Lock()
	r.ids = append(r.ids, args.NotificationID)
	r.mu.Unlock()
	if r.fail != nil {
		return r.fail
	}
	return r.inner.InsertNotificationTx(ctx, tx, args)
}

func (r *recordingInserter) recorded() []uuid.UUID {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]uuid.UUID(nil), r.ids...)
}

func TestEnqueueRollbackLeavesNoStateNoJob(t *testing.T) {
	rec := &recordingInserter{}
	h := newHarness(t, harnessOpts{wrapJobs: rec.wrap})
	ctx := context.Background()

	tx, err := h.pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	if err := h.enqueueTx(ctx, tx, h.newNotification(eventKey("rollback"), nil)); err != nil {
		t.Fatalf("enqueue: %v", err)
	}
	if err := tx.Rollback(ctx); err != nil {
		t.Fatalf("rollback: %v", err)
	}

	ids := rec.recorded()
	if len(ids) == 0 {
		t.Fatal("the service never inserted a job, so the rollback assertion would be vacuous")
	}
	for _, id := range ids {
		if h.notificationRowExists(id) {
			t.Errorf("notification %s survived a rolled-back enqueue", id)
		}
		if h.jobExistsFor(id) {
			t.Errorf("river job for %s survived a rolled-back enqueue", id)
		}
	}
}

func TestCallerFlowRollsBackBusinessStateOnEnqueueError(t *testing.T) {
	rec := &recordingInserter{fail: errors.New("simulated job insert failure")}
	h := newHarness(t, harnessOpts{wrapJobs: rec.wrap})
	ctx := context.Background()
	code := "RB" + uuid.NewString()[:6]

	tx, err := h.pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	// A business row the caller would have committed together with the notification.
	if _, err := tx.Exec(ctx, `INSERT INTO programs (code, name) VALUES ($1, $2)`, code, "rollback probe"); err != nil {
		t.Fatalf("insert business row: %v", err)
	}
	if err := h.enqueueTx(ctx, tx, h.newNotification(eventKey("enqueue-fail"), nil)); err == nil {
		t.Fatal("Enqueue succeeded although the delivery job could not be inserted")
	}
	// Returning the error does not abort the transaction, so the caller must roll back explicitly.
	if err := tx.Rollback(ctx); err != nil {
		t.Fatalf("rollback: %v", err)
	}

	if got := h.count(`SELECT count(*) FROM programs WHERE code = $1`, code); got != 0 {
		t.Errorf("business rows = %d, want 0 after the caller rolled back", got)
	}
	ids := rec.recorded()
	if len(ids) == 0 {
		t.Fatal("the service never reached the job insert, so the rollback assertion would be vacuous")
	}
	for _, id := range ids {
		if h.notificationRowExists(id) {
			t.Errorf("notification %s survived the caller's rollback", id)
		}
		if h.jobExistsFor(id) {
			t.Errorf("river job for %s survived the caller's rollback", id)
		}
	}
}

func TestEnqueueCommitThenWorkerStartDelivers(t *testing.T) {
	h := newHarness(t, harnessOpts{})
	id := h.enqueue(h.newNotification(eventKey("commit"), nil))

	if got := h.smtp.connectionCount(); got != 0 {
		t.Fatalf("smtp connections = %d before any worker existed, want 0", got)
	}
	state, _, _, _ := h.jobState(id)
	if state != "available" {
		t.Fatalf("job state = %s, want available after commit", state)
	}

	h.startWorker(t, fastRetry)
	waitFor(t, "one delivered message", func() bool { return h.smtp.messageCount() == 1 })
	waitFor(t, "status sent", func() bool { return h.row(id).Status == model.StatusSent })

	if got := h.smtp.messageCount(); got != 1 {
		t.Errorf("messages = %d, want exactly 1", got)
	}
	row := h.row(id)
	if row.LastOutcome == nil || *row.LastOutcome != model.OutcomeOK || row.FinishedAt == nil {
		t.Errorf("row = outcome %v finished %v; want ok and a finish time", row.LastOutcome, row.FinishedAt)
	}
}

// An interrupted in-flight send must cost nothing but an attempt: the next client delivers once.
func TestWorkerStoppedBeforeSMTPThenRestartedDeliversOnce(t *testing.T) {
	gate := &gateSender{started: make(chan struct{}), release: make(chan struct{})}
	h := newHarness(t, harnessOpts{wrapEmail: func(inner service.Sender) service.Sender {
		gate.inner = inner
		return gate
	}})
	id := h.enqueue(h.newNotification(eventKey("interrupted"), nil))

	first := h.startWorker(t, fastRetry)
	select {
	case <-gate.started:
	case <-time.After(30 * time.Second):
		t.Fatal("the worker never started the send")
	}

	// Hard stop while Send is blocked: the attempt dies mid-conversation, before the provider saw it.
	stopCtx, cancelStop := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancelStop()
	if err := first.StopAndCancel(stopCtx); err != nil {
		t.Fatalf("stop first client: %v", err)
	}
	<-first.Stopped()

	if got := h.smtp.messageCount(); got != 0 {
		t.Fatalf("messages = %d, want 0: the send was interrupted before it reached the provider", got)
	}

	waitFor(t, "the interrupted attempt to settle", func() bool {
		row := h.row(id)
		return row.Status != model.StatusQueued || row.ClaimToken == nil
	})
	row := h.row(id)
	if row.Status != model.StatusQueued {
		t.Errorf("status = %s, want queued: an interrupted send is not a terminal outcome", row.Status)
	}
	if row.ClaimToken != nil || row.ClaimedUntil != nil {
		t.Errorf("claim survived the interruption: token %v until %v", row.ClaimToken, row.ClaimedUntil)
	}
	if row.Attempts != 1 {
		t.Errorf("attempts = %d, want 1: the interrupted attempt still spent its budget", row.Attempts)
	}
	if row.LastOutcome == nil || *row.LastOutcome != model.OutcomeTransient {
		t.Errorf("last_outcome = %v, want transient after a cancelled send", row.LastOutcome)
	}
	if state, _, _, _ := h.jobState(id); state != "available" && state != "retryable" {
		t.Errorf("job state = %s, want available or retryable: the job must survive a hard stop", state)
	}

	h.startWorker(t, fastRetry)
	waitFor(t, "one delivered message after the restart", func() bool { return h.smtp.messageCount() == 1 })
	waitFor(t, "status sent", func() bool { return h.row(id).Status == model.StatusSent })

	if got := h.smtp.messageCount(); got != 1 {
		t.Errorf("messages = %d, want exactly 1", got)
	}
	if row := h.row(id); row.LastOutcome == nil || *row.LastOutcome != model.OutcomeOK {
		t.Errorf("last_outcome = %v, want ok", row.LastOutcome)
	}
}

// Delivery is at-least-once: an accepted message with no outcome write is sent again, never lost.
func TestCrashAfterSMTPAcceptanceBeforeStatusWriteMayDuplicate(t *testing.T) {
	h := newHarness(t, harnessOpts{})
	ctx := context.Background()
	id := h.enqueue(h.newNotification(eventKey("crash"), nil))

	accepted := 0
	h.smtp.setOnMessage(func(string) (int, string, bool) {
		accepted++
		if accepted == 1 {
			// A crash between acceptance and the outcome write: the lease is gone, so nothing matches.
			h.sql(`UPDATE notification_logs SET claimed_until = now() - interval '1 second' WHERE id = $1`, id)
		}
		return 250, "ok", false
	})

	err := h.svc.Deliver(ctx, id, service.Attempt{N: 1, Max: notificationMax()})
	if !errors.Is(err, service.ErrBusy) {
		t.Fatalf("first attempt err = %v, want ErrBusy: the claim was lost before the status write", err)
	}
	if got := h.smtp.messageCount(); got != 1 {
		t.Fatalf("messages = %d, want 1 after the lost status write", got)
	}
	if row := h.row(id); row.Status != model.StatusQueued {
		t.Fatalf("status = %s, want queued: no outcome was recorded", row.Status)
	}

	if err := h.svc.Deliver(ctx, id, service.Attempt{N: 2, Max: notificationMax()}); err != nil {
		t.Fatalf("second attempt: %v", err)
	}
	if got := h.smtp.messageCount(); got != 2 {
		t.Fatalf("messages = %d, want 2: the retry must deliver a duplicate copy", got)
	}
	if row := h.row(id); row.Status != model.StatusSent {
		t.Fatalf("status = %s, want sent", row.Status)
	}
}

func TestDuplicateEventChannelSuppressed(t *testing.T) {
	h := newHarness(t, harnessOpts{})
	event := eventKey("dup")

	first := h.enqueue(h.newNotification(event, nil))
	second := h.enqueue(h.newNotification(event, nil))

	if second != first {
		t.Errorf("second enqueue produced row %s, want the existing identity %s", second, first)
	}
	if got := h.count(`SELECT count(*) FROM notification_logs WHERE event_key = $1`, event); got != 1 {
		t.Errorf("rows for the identity = %d, want 1", got)
	}
	if got := h.count(`SELECT count(*) FROM river.river_job WHERE kind = 'notification' AND args->>'notification_id' = $1`, first.String()); got != 1 {
		t.Errorf("jobs for the identity = %d, want 1", got)
	}
}

func TestConcurrentDuplicateEnqueue(t *testing.T) {
	h := newHarness(t, harnessOpts{})
	event := eventKey("race-dup")
	const goroutines = 8

	var wg sync.WaitGroup
	errs := make([]error, goroutines)
	start := make(chan struct{})
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			tx, err := h.pool.Begin(context.Background())
			if err != nil {
				errs[i] = err
				return
			}
			if err := h.enqueueTx(context.Background(), tx, h.newNotification(event, nil)); err != nil {
				_ = tx.Rollback(context.Background())
				errs[i] = err
				return
			}
			errs[i] = tx.Commit(context.Background())
		}(i)
	}
	close(start)
	wg.Wait()

	for i, err := range errs {
		if err != nil {
			t.Errorf("goroutine %d: %v", i, err)
		}
	}
	h.record(h.lastRowID(event))
	if got := h.count(`SELECT count(*) FROM notification_logs WHERE event_key = $1`, event); got != 1 {
		t.Errorf("rows for the identity = %d, want 1", got)
	}
	if got := h.count(`SELECT count(*) FROM river.river_job WHERE kind = 'notification'
		AND args->>'notification_id' IN (SELECT id::text FROM notification_logs WHERE event_key = $1)`, event); got != 1 {
		t.Errorf("jobs for the identity = %d, want 1", got)
	}
}

func TestConcurrentDeliverSendsOnce(t *testing.T) {
	h := newHarness(t, harnessOpts{})
	id := h.enqueue(h.newNotification(eventKey("concurrent"), nil))

	reached := make(chan struct{})
	gate := make(chan struct{})
	var once sync.Once
	h.smtp.setOnMessage(func(string) (int, string, bool) {
		once.Do(func() { close(reached) })
		<-gate
		return 250, "ok", false
	})

	winner := make(chan error, 1)
	go func() {
		winner <- h.svc.Deliver(context.Background(), id, service.Attempt{N: 1, Max: notificationMax()})
	}()
	<-reached

	loserErr := h.svc.Deliver(context.Background(), id, service.Attempt{N: 1, Max: notificationMax()})
	if !errors.Is(loserErr, service.ErrBusy) {
		t.Errorf("second Deliver err = %v, want ErrBusy while the first holds the lease", loserErr)
	}
	close(gate)
	if err := <-winner; err != nil {
		t.Fatalf("first Deliver: %v", err)
	}

	if got := h.smtp.messageCount(); got != 1 {
		t.Errorf("messages = %d, want exactly 1", got)
	}
	if row := h.row(id); row.Status != model.StatusSent {
		t.Errorf("status = %s, want sent", row.Status)
	}
}

func TestStaleOutcomeWriteIsIgnored(t *testing.T) {
	h := newHarness(t, harnessOpts{})
	ctx := context.Background()
	id := h.enqueue(h.newNotification(eventKey("stale"), nil))

	token := uuid.New()
	if _, err := h.repo.Claim(ctx, h.tenantScope(), id, token, time.Minute); err != nil {
		t.Fatalf("claim: %v", err)
	}

	// A live foreign claim is a snooze, not a delivery.
	if err := h.svc.Deliver(ctx, id, service.Attempt{N: 1, Max: notificationMax()}); !errors.Is(err, service.ErrBusy) {
		t.Errorf("Deliver while another claim is live: err = %v, want ErrBusy", err)
	}

	reason := "stale"
	wrong := uuid.New()
	if ok, err := h.repo.Finish(ctx, h.tenantScope(), id, wrong, model.StatusSent, 1, outOf(model.OutcomeOK), &reason); err != nil || ok {
		t.Errorf("Finish with a wrong token: ok = %v, err = %v; want false, nil", ok, err)
	}

	h.sql(`UPDATE notification_logs SET claimed_until = now() - interval '1 second' WHERE id = $1`, id)
	if ok, err := h.repo.Finish(ctx, h.tenantScope(), id, token, model.StatusSent, 1, outOf(model.OutcomeOK), &reason); err != nil || ok {
		t.Errorf("Finish with the right token after the lease expired: ok = %v, err = %v; want false, nil", ok, err)
	}

	row := h.row(id)
	if row.Status != model.StatusQueued || row.FinishedAt != nil || row.Attempts != 0 || row.LastOutcome != nil {
		t.Errorf("row = status %s finished %v attempts %d outcome %v; want an untouched queued row",
			row.Status, row.FinishedAt, row.Attempts, row.LastOutcome)
	}
	if got := h.smtp.messageCount(); got != 0 {
		t.Errorf("messages = %d, want 0", got)
	}
}

// A release that loses the lease to an already-terminal row is settled, not a retryable failure.
func TestReleaseLosingOwnershipToATerminalRowIsNotAnError(t *testing.T) {
	h := newHarness(t, harnessOpts{})
	ctx := context.Background()
	id := h.enqueue(h.newNotification(eventKey("lost-release"), nil))

	h.smtp.setOnMessage(func(string) (int, string, bool) {
		// Another owner retires the row while this attempt is mid-conversation.
		h.sql(`UPDATE notification_logs
		          SET status = 'failed', last_outcome = 'permanent', last_error = 'other owner',
		              finished_at = now(), claim_token = NULL, claimed_until = NULL
		        WHERE id = $1`, id)
		return 451, "try again later", false
	})

	if err := h.svc.Deliver(ctx, id, service.Attempt{N: 1, Max: notificationMax()}); err != nil {
		t.Fatalf("Deliver after a terminal owner took the lease: err = %v, want nil", err)
	}
	row := h.row(id)
	if row.Status != model.StatusFailed || row.LastError == nil || *row.LastError != "other owner" {
		t.Errorf("row = status %s error %v; want the other owner's terminal state untouched", row.Status, row.LastError)
	}
}

func TestCompetingTerminalTransitionsOnlyOwnerWins(t *testing.T) {
	t.Run("expiry", func(t *testing.T) {
		h := newHarness(t, harnessOpts{})
		id := h.enqueue(h.newNotification(eventKey("race-expiry"), func(n *service.NewNotification) {
			at := time.Now().Add(time.Hour)
			n.ExpiresAt = &at
		}))
		h.sql(`UPDATE notification_logs SET expires_at = now() - interval '1 second' WHERE id = $1`, id)

		var wg sync.WaitGroup
		errs := make([]error, 2)
		start := make(chan struct{})
		for i := range errs {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				<-start
				errs[i] = h.svc.Deliver(context.Background(), id, service.Attempt{N: 1, Max: notificationMax()})
			}(i)
		}
		close(start)
		wg.Wait()

		for i, err := range errs {
			if err != nil && !errors.Is(err, service.ErrBusy) {
				t.Errorf("caller %d: err = %v, want nil or ErrBusy", i, err)
			}
		}
		row := h.row(id)
		if row.Status != model.StatusExpired {
			t.Errorf("status = %s, want expired", row.Status)
		}
		if row.ClaimToken != nil || row.ClaimedUntil != nil || row.FinishedAt == nil {
			t.Errorf("terminal row still carries claim state: token %v until %v", row.ClaimToken, row.ClaimedUntil)
		}
		if got := h.smtp.messageCount(); got != 0 {
			t.Errorf("messages = %d, want 0: an expired row is never sent", got)
		}
	})

	t.Run("sent", func(t *testing.T) {
		h := newHarness(t, harnessOpts{})
		id := h.enqueue(h.newNotification(eventKey("race-sent"), nil))

		reached := make(chan struct{})
		gate := make(chan struct{})
		var once sync.Once
		h.smtp.setOnMessage(func(string) (int, string, bool) {
			once.Do(func() { close(reached) })
			<-gate
			return 250, "ok", false
		})

		results := make([]error, 2)
		var wg sync.WaitGroup
		wg.Add(1)
		go func() {
			defer wg.Done()
			results[0] = h.svc.Deliver(context.Background(), id, service.Attempt{N: 1, Max: notificationMax()})
		}()
		<-reached
		results[1] = h.svc.Deliver(context.Background(), id, service.Attempt{N: 1, Max: notificationMax()})
		close(gate)
		wg.Wait()

		terminal := 0
		for i, err := range results {
			switch {
			case err == nil:
				terminal++
			case errors.Is(err, service.ErrBusy):
			default:
				t.Errorf("caller %d: err = %v, want nil or ErrBusy", i, err)
			}
		}
		if terminal != 1 {
			t.Errorf("callers that completed = %d, want exactly 1", terminal)
		}
		if row := h.row(id); row.Status != model.StatusSent {
			t.Errorf("status = %s, want sent", row.Status)
		}
		if got := h.smtp.messageCount(); got != 1 {
			t.Errorf("messages = %d, want exactly 1", got)
		}
	})

	t.Run("unavailable", func(t *testing.T) {
		h := newHarness(t, harnessOpts{})
		id := h.enqueue(h.newNotification(eventKey("race-unavailable"), func(n *service.NewNotification) {
			n.Channel = string(model.ChannelWhatsApp)
			n.Recipient = "081234567890"
		}))

		var wg sync.WaitGroup
		errs := make([]error, 2)
		start := make(chan struct{})
		for i := range errs {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				<-start
				errs[i] = h.svc.Deliver(context.Background(), id, service.Attempt{N: 1, Max: notificationMax()})
			}(i)
		}
		close(start)
		wg.Wait()

		for i, err := range errs {
			if err != nil && !errors.Is(err, service.ErrBusy) {
				t.Errorf("caller %d: err = %v, want nil or ErrBusy", i, err)
			}
		}
		row := h.row(id)
		if row.Status != model.StatusUnavailable {
			t.Errorf("status = %s, want unavailable", row.Status)
		}
		if row.ClaimToken != nil || row.ClaimedUntil != nil {
			t.Errorf("terminal row still carries claim state: token %v until %v", row.ClaimToken, row.ClaimedUntil)
		}
		if row.FinishedAt == nil {
			t.Error("finished_at is NULL, want the terminal write to stamp it")
		}
		if row.LastOutcome != nil {
			t.Errorf("last_outcome = %v, want none: an unavailable channel is not a success", *row.LastOutcome)
		}
		if got := h.smtp.connectionCount(); got != 0 {
			t.Errorf("smtp connections = %d, want 0: an unavailable channel never touches the email transport", got)
		}
	})
}

func TestTerminalStatesAreImmutableAndDeliverIsNoop(t *testing.T) {
	h := newHarness(t, harnessOpts{})
	ctx := context.Background()

	for _, status := range []model.NotificationStatus{model.StatusSent, model.StatusFailed, model.StatusExpired, model.StatusUnavailable} {
		t.Run(string(status), func(t *testing.T) {
			id := h.enqueue(h.newNotification(eventKey("terminal-"+string(status)), nil))
			h.sql(`UPDATE notification_logs
			          SET status = $2, finished_at = now(), sealed_params = NULL,
			              claim_token = NULL, claimed_until = NULL
			        WHERE id = $1`, id, string(status))
			before := h.row(id)

			if err := h.svc.Deliver(ctx, id, service.Attempt{N: 1, Max: notificationMax()}); err != nil {
				t.Fatalf("Deliver on a terminal row: %v, want nil", err)
			}
			after := h.row(id)
			if after.Status != before.Status || after.Attempts != before.Attempts || !after.UpdatedAt.Equal(before.UpdatedAt) {
				t.Errorf("terminal row changed: before %+v after %+v", before, after)
			}
			if got := h.smtp.messageCount(); got != 0 {
				t.Errorf("messages = %d, want 0", got)
			}
		})
	}
}

func TestProviderFailureIsVisibleAndRecoverable(t *testing.T) {
	t.Run("transient then success", func(t *testing.T) {
		h := newHarness(t, harnessOpts{})
		id := h.enqueue(h.newNotification(eventKey("recoverable"), nil))

		var mu sync.Mutex
		calls := 0
		second := make(chan struct{})
		gate := make(chan struct{})
		h.smtp.setOnMessage(func(string) (int, string, bool) {
			mu.Lock()
			calls++
			n := calls
			mu.Unlock()
			switch n {
			case 1:
				return 451, "try again later", false
			case 2:
				close(second)
				<-gate
				return 451, "try again later", false
			default:
				return 250, "ok", false
			}
		})

		h.startWorker(t, fastRetry)
		<-second
		// The intermediate outcome is visible on the row before recovery.
		row := h.row(id)
		if row.LastOutcome == nil || *row.LastOutcome != model.OutcomeTransient {
			t.Fatalf("last_outcome = %v, want transient while the first retry is in flight", row.LastOutcome)
		}
		if row.Status != model.StatusQueued || row.FinishedAt != nil {
			t.Fatalf("row = status %s finished %v; want queued and unfinished", row.Status, row.FinishedAt)
		}
		close(gate)

		waitForWithin(t, 60*time.Second, "status sent", func() bool { return h.row(id).Status == model.StatusSent })
		row = h.row(id)
		if row.Attempts != 3 {
			t.Errorf("attempts = %d, want 3", row.Attempts)
		}
		if row.LastOutcome == nil || *row.LastOutcome != model.OutcomeOK {
			t.Errorf("last_outcome = %v, want ok", row.LastOutcome)
		}
		if got := h.smtp.messageCount(); got != 3 {
			t.Errorf("provider calls = %d, want 3", got)
		}
	})

	t.Run("550 is permanent and cancels the river job", func(t *testing.T) {
		h := newHarness(t, harnessOpts{})
		id := h.enqueue(h.newNotification(eventKey("perm550"), nil))
		h.smtp.setOnMessage(func(string) (int, string, bool) { return 550, "no such user", false })

		h.startWorker(t, fastRetry)
		waitFor(t, "status failed", func() bool { return h.row(id).Status == model.StatusFailed })
		waitFor(t, "job cancelled", func() bool {
			state, _, _, _ := h.jobState(id)
			return state == "cancelled"
		})
		row := h.row(id)
		if row.LastOutcome == nil || *row.LastOutcome != model.OutcomePermanent {
			t.Errorf("last_outcome = %v, want permanent", row.LastOutcome)
		}
		if row.LastError == nil || !strings.Contains(*row.LastError, "550") {
			t.Errorf("last_error = %v, want the fixed sanitized rejection text", row.LastError)
		}
		if got := h.smtp.messageCount(); got != 1 {
			t.Errorf("provider calls = %d, want 1: a 5xx is not retried", got)
		}
		if got := h.count(`SELECT count(*) FROM river.river_job WHERE kind='notification' AND args->>'notification_id' = $1 AND state = 'cancelled'`, id.String()); got != 1 {
			t.Errorf("cancelled river jobs = %d, want 1", got)
		}
	})

	t.Run("535 is a permanent authentication failure", func(t *testing.T) {
		h := newHarness(t, harnessOpts{smtpMutate: func(c *config.SMTPConfig) {
			c.Username = "relay"
			c.Password = "s3cret-relay-password"
		}})
		h.smtp.authEnabled = true
		h.smtp.authReplyCode = 535

		id := h.enqueue(h.newNotification(eventKey("perm535"), nil))
		if err := h.svc.Deliver(context.Background(), id, service.Attempt{N: 1, Max: notificationMax()}); !service.IsPermanent(err) {
			t.Fatalf("err = %v, want permanent", err)
		}
		row := h.row(id)
		if row.Status != model.StatusFailed || row.SealedParams != nil {
			t.Errorf("row = status %s sealed %v; want failed and erased", row.Status, row.SealedParams)
		}
		if row.LastError == nil || *row.LastError != "smtp: authentication failed" {
			t.Errorf("last_error = %v, want the fixed authentication text", row.LastError)
		}
		if got := h.smtp.messageCount(); got != 0 {
			t.Errorf("messages = %d, want 0", got)
		}
	})
}

func TestSixthRetryableFailureIsTerminalAndErased(t *testing.T) {
	h := newHarness(t, harnessOpts{sealKey: testSealKey(8)})
	at := time.Now().Add(time.Hour)
	id := h.enqueue(h.newNotification(eventKey("exhaust"), func(n *service.NewNotification) {
		n.SecretParams = map[string]string{"otp": "482913"}
		n.ExpiresAt = &at
	}))

	var calls atomic.Int64
	h.smtp.setOnMessage(func(string) (int, string, bool) {
		calls.Add(1)
		return 451, "try again later", false
	})

	// A different client default proves the per-job budget of six wins; read it before the worker runs.
	if state, _, maxAttempts, _ := h.jobState(id); maxAttempts != notificationMax() || state != "available" {
		t.Fatalf("job = state %s max_attempts %d; want available with max_attempts %d", state, maxAttempts, notificationMax())
	}
	h.startWorker(t, fastRetry, WithClientMaxAttempts(3))

	waitForWithin(t, 90*time.Second, "six provider calls", func() bool { return calls.Load() >= 6 })
	waitForWithin(t, 30*time.Second, "status failed", func() bool { return h.row(id).Status == model.StatusFailed })
	// River finalises the job after the worker returns, so the discard is a separate observable step.
	waitForWithin(t, 30*time.Second, "the exhausted job to be discarded", func() bool {
		state, _, _, _ := h.jobState(id)
		return state == "discarded"
	})

	row := h.row(id)
	if row.Attempts != notificationMax() {
		t.Errorf("attempts = %d, want %d", row.Attempts, notificationMax())
	}
	if row.SealedParams != nil {
		t.Errorf("sealed_params = %d bytes, want NULL after the terminal failure", len(row.SealedParams))
	}
	if row.LastOutcome == nil || *row.LastOutcome != model.OutcomeTransient {
		t.Errorf("last_outcome = %v, want transient", row.LastOutcome)
	}

	state, attempt, maxAttempts, errCount := h.jobState(id)
	if state != "discarded" {
		t.Errorf("job state = %s, want discarded after the sixth retryable failure", state)
	}
	if attempt != notificationMax() || maxAttempts != notificationMax() {
		t.Errorf("job attempt %d of %d, want %d of %d", attempt, maxAttempts, notificationMax(), notificationMax())
	}
	if errCount != notificationMax() {
		t.Errorf("recorded job errors = %d, want %d", errCount, notificationMax())
	}
	if got := calls.Load(); got != int64(notificationMax()) {
		t.Errorf("provider calls = %d, want exactly %d", got, notificationMax())
	}
}

// A snooze at the last attempt releases the row without spending budget.
func TestSnoozeOnTheLastAttemptDoesNotConsumeBudget(t *testing.T) {
	h := newHarness(t, harnessOpts{})
	at := time.Now().Add(time.Hour)
	id := h.enqueue(h.newNotification(eventKey("snooze"), func(n *service.NewNotification) { n.ExpiresAt = &at }))
	h.sql(`UPDATE river.river_job SET attempt = $2, max_attempts = $3 WHERE kind = 'notification' AND args->>'notification_id' = $1`,
		id.String(), notificationMax()-1, notificationMax())

	// Somebody else owns the row, so the worker's Deliver answers ErrBusy and snoozes.
	if _, err := h.repo.Claim(context.Background(), h.tenantScope(), id, uuid.New(), 2*time.Minute); err != nil {
		t.Fatalf("foreign claim: %v", err)
	}

	h.startWorker(t, fastRetry)
	waitFor(t, "the job to be snoozed", func() bool {
		state, attempt, maxAttempts, _ := h.jobState(id)
		return state == "scheduled" && attempt == notificationMax()-1 && maxAttempts == notificationMax()
	})
	if row := h.row(id); row.Status != model.StatusQueued || row.Attempts != 0 {
		t.Errorf("row = status %s attempts %d; a snooze must change nothing", row.Status, row.Attempts)
	}

	// Release the foreign claim and let the row be worked once more: it is still attempt 6 of 6.
	h.sql(`UPDATE notification_logs SET claim_token = NULL, claimed_until = NULL WHERE id = $1`, id)
	h.sql(`UPDATE river.river_job SET state = 'available', scheduled_at = now() WHERE kind = 'notification' AND args->>'notification_id' = $1`, id.String())
	h.smtp.setOnMessage(func(string) (int, string, bool) { return 451, "try again later", false })

	waitForWithin(t, 60*time.Second, "the sixth attempt to write its attempt count", func() bool { return h.row(id).Attempts == notificationMax() })
}

func TestReconcileFailsOrphanedQueued(t *testing.T) {
	h := newHarness(t, harnessOpts{})
	ctx := context.Background()

	aged := func(id uuid.UUID) {
		h.sql(`UPDATE notification_logs SET created_at = now() - interval '2 hours' WHERE id = $1`, id)
	}

	t.Run("no job at all", func(t *testing.T) {
		id := h.enqueue(h.newNotification(eventKey("orphan-gone"), nil))
		h.sql(`DELETE FROM river.river_job WHERE kind='notification' AND args->>'notification_id' = $1`, id.String())
		aged(id)

		if _, err := h.svc.Reconcile(ctx); err != nil {
			t.Fatalf("reconcile: %v", err)
		}
		row := h.row(id)
		if row.Status != model.StatusFailed || row.SealedParams != nil || row.FinishedAt == nil {
			t.Errorf("row = status %s sealed %v finished %v; want failed, erased, finished", row.Status, row.SealedParams, row.FinishedAt)
		}
		if row.LastError == nil || *row.LastError != "reconciled: delivery job ended without a result" {
			t.Errorf("last_error = %v, want the fixed reconcile text", row.LastError)
		}
		if row.ClaimToken != nil || row.ClaimedUntil != nil {
			t.Errorf("claim fields not cleared: %v %v", row.ClaimToken, row.ClaimedUntil)
		}
	})

	for _, tc := range []struct {
		name  string
		state string
	}{
		{"discarded", "discarded"},
		{"cancelled", "cancelled"},
		{"anomalously completed", "completed"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			id := h.enqueue(h.newNotification(eventKey("orphan-"+tc.state), nil))
			h.sql(`UPDATE river.river_job SET state = $2::river.river_job_state, finalized_at = now()
			        WHERE kind='notification' AND args->>'notification_id' = $1`, id.String(), tc.state)
			aged(id)

			if _, err := h.svc.Reconcile(ctx); err != nil {
				t.Fatalf("reconcile: %v", err)
			}
			if row := h.row(id); row.Status != model.StatusFailed {
				t.Errorf("status = %s, want failed", row.Status)
			}
		})
	}

	t.Run("healthy and claimed rows are untouched", func(t *testing.T) {
		var ids []uuid.UUID
		for _, state := range []string{"available", "scheduled", "retryable", "running"} {
			id := h.enqueue(h.newNotification(eventKey("healthy-"+state), nil))
			h.sql(`UPDATE river.river_job SET state = $2::river.river_job_state, finalized_at = NULL
			        WHERE kind='notification' AND args->>'notification_id' = $1`, id.String(), state)
			aged(id)
			ids = append(ids, id)
		}
		claimed := h.enqueue(h.newNotification(eventKey("healthy-claimed"), nil))
		aged(claimed)
		if _, err := h.repo.Claim(ctx, h.tenantScope(), claimed, uuid.New(), 2*time.Minute); err != nil {
			t.Fatalf("claim: %v", err)
		}
		ids = append(ids, claimed)

		if _, err := h.svc.Reconcile(ctx); err != nil {
			t.Fatalf("reconcile: %v", err)
		}
		for _, id := range ids {
			if row := h.row(id); row.Status != model.StatusQueued {
				t.Errorf("row %s = status %s, want queued", id, row.Status)
			}
		}
	})

	t.Run("a job revived between scan and write keeps the row sealed", func(t *testing.T) {
		sealed := newHarness(t, harnessOpts{sealKey: testSealKey(6)})
		id := sealed.enqueue(sealed.newNotification(eventKey("revived"), func(n *service.NewNotification) {
			n.SecretParams = map[string]string{"otp": "112233"}
		}))
		sealed.sql(`UPDATE river.river_job SET state = 'discarded', finalized_at = now()
		        WHERE kind='notification' AND args->>'notification_id' = $1`, id.String())
		sealed.sql(`UPDATE notification_logs SET created_at = now() - interval '2 hours' WHERE id = $1`, id)

		candidates, err := sealed.repo.ReconcileCandidates(ctx, time.Now().Add(-service.ReconcileCutoff), 100)
		if err != nil {
			t.Fatalf("candidates: %v", err)
		}
		found := false
		for _, c := range candidates {
			if c.ID == id {
				found = true
			}
		}
		if !found {
			t.Fatal("the discarded row was not a reconcile candidate")
		}

		// River's JobRetry revives the job: the row must stay queued and keep its sealed payload.
		client := sealed.workerClient(t, fastRetry)
		var jobID int64
		if err := sealed.pool.QueryRow(ctx, `SELECT id FROM river.river_job WHERE kind='notification' AND args->>'notification_id' = $1`, id.String()).Scan(&jobID); err != nil {
			t.Fatalf("job id: %v", err)
		}
		if _, err := client.JobRetry(ctx, jobID); err != nil {
			t.Fatalf("job retry: %v", err)
		}

		ok, err := sealed.repo.FailOrphan(ctx, sealed.tenantScope(), id, "reconciled: delivery job ended without a result")
		if err != nil {
			t.Fatalf("fail orphan: %v", err)
		}
		if ok {
			t.Error("FailOrphan retired a row whose job had been revived")
		}
		row := sealed.row(id)
		if row.Status != model.StatusQueued || len(row.SealedParams) == 0 {
			t.Errorf("row = status %s sealed %d bytes; want queued and still sealed", row.Status, len(row.SealedParams))
		}
	})

	t.Run("a bounded scan retires the oldest of my rows first", func(t *testing.T) {
		// The scan is global, so lower the limit and assert on my rows, which are the oldest by far.
		limited := newHarness(t, harnessOpts{reconcileLimit: 3})
		ids := make([]uuid.UUID, 0, 5)
		for i := 0; i < 5; i++ {
			id := limited.enqueue(limited.newNotification(eventKey("bulk"), nil))
			limited.sql(`DELETE FROM river.river_job WHERE kind='notification' AND args->>'notification_id' = $1`, id.String())
			limited.sql(`UPDATE notification_logs SET created_at = now() - interval '10 years' + make_interval(secs => $2) WHERE id = $1`,
				id, float64(i))
			ids = append(ids, id)
		}

		if _, err := limited.svc.Reconcile(ctx); err != nil {
			t.Fatalf("reconcile: %v", err)
		}
		for i, id := range ids {
			want := model.StatusFailed
			if i >= 3 {
				want = model.StatusQueued
			}
			if row := limited.row(id); row.Status != want {
				t.Errorf("my row %d = status %s, want %s", i, row.Status, want)
			}
		}

		if _, err := limited.svc.Reconcile(ctx); err != nil {
			t.Fatalf("second reconcile: %v", err)
		}
		for i, id := range ids {
			if row := limited.row(id); row.Status != model.StatusFailed {
				t.Errorf("after a second run my row %d = status %s, want failed: the limit only bounds one run", i, row.Status)
			}
		}
	})
}

func TestUncertainOutcomeRecordedAndRetried(t *testing.T) {
	h := newHarness(t, harnessOpts{})
	ctx := context.Background()
	id := h.enqueue(h.newNotification(eventKey("uncertain"), nil))

	// The provider reads the whole message and hangs up before the final reply.
	h.smtp.setOnMessage(func(string) (int, string, bool) { return 0, "", true })

	err := h.svc.Deliver(ctx, id, service.Attempt{N: 1, Max: notificationMax()})
	var de *service.DeliveryError
	if !errors.As(err, &de) || de.Class != service.ClassUncertain {
		t.Fatalf("err = %v, want an uncertain delivery error", err)
	}
	row := h.row(id)
	if row.Status != model.StatusQueued || row.FinishedAt != nil {
		t.Fatalf("row = status %s finished %v; want queued and unfinished", row.Status, row.FinishedAt)
	}
	if row.LastOutcome == nil || *row.LastOutcome != model.OutcomeUncertain {
		t.Errorf("last_outcome = %v, want uncertain", row.LastOutcome)
	}
	if row.Attempts != 1 {
		t.Errorf("attempts = %d, want 1", row.Attempts)
	}

	h.smtp.setOnMessage(nil)
	if err := h.svc.Deliver(ctx, id, service.Attempt{N: 2, Max: notificationMax()}); err != nil {
		t.Fatalf("second attempt: %v", err)
	}
	if got := h.smtp.messageCount(); got != 2 {
		t.Errorf("messages = %d, want 2: an uncertain outcome may duplicate", got)
	}
	if row := h.row(id); row.Status != model.StatusSent {
		t.Errorf("status = %s, want sent", row.Status)
	}
}

func TestUnconfiguredChannelsReportUnavailable(t *testing.T) {
	t.Run("whatsapp", func(t *testing.T) {
		h := newHarness(t, harnessOpts{})
		id := h.enqueue(h.newNotification(eventKey("wa"), func(n *service.NewNotification) {
			n.Channel = string(model.ChannelWhatsApp)
			n.Recipient = "081234567890"
		}))
		if err := h.svc.Deliver(context.Background(), id, service.Attempt{N: 1, Max: notificationMax()}); err != nil {
			t.Fatalf("deliver: %v", err)
		}
		row := h.row(id)
		if row.Status != model.StatusUnavailable || row.FinishedAt == nil {
			t.Errorf("row = status %s finished %v; want unavailable and finished", row.Status, row.FinishedAt)
		}
		if row.LastOutcome != nil {
			t.Errorf("last_outcome = %v, want none: an unavailable channel is not a success", *row.LastOutcome)
		}
		if got := h.smtp.connectionCount(); got != 0 {
			t.Errorf("smtp connections = %d, want 0: whatsapp must never touch the email transport", got)
		}
	})

	t.Run("email without a host", func(t *testing.T) {
		h := newHarness(t, harnessOpts{noSMTP: true})
		id := h.enqueue(h.newNotification(eventKey("no-smtp"), nil))
		if err := h.svc.Deliver(context.Background(), id, service.Attempt{N: 1, Max: notificationMax()}); err != nil {
			t.Fatalf("deliver: %v", err)
		}
		row := h.row(id)
		if row.Status != model.StatusUnavailable {
			t.Errorf("status = %s, want unavailable when SMTP_HOST is unset", row.Status)
		}
		if row.LastOutcome != nil {
			t.Errorf("last_outcome = %v, want none: an unavailable channel is not a success", *row.LastOutcome)
		}
	})
}

func TestExpiredMessageIsNeverSent(t *testing.T) {
	h := newHarness(t, harnessOpts{})
	ctx := context.Background()

	id := h.enqueue(h.newNotification(eventKey("expired"), func(n *service.NewNotification) {
		at := time.Now().Add(time.Hour)
		n.ExpiresAt = &at
	}))
	h.sql(`UPDATE notification_logs SET expires_at = now() - interval '1 second' WHERE id = $1`, id)

	if err := h.svc.Deliver(ctx, id, service.Attempt{N: 1, Max: notificationMax()}); err != nil {
		t.Fatalf("deliver: %v", err)
	}
	row := h.row(id)
	if row.Status != model.StatusExpired || row.FinishedAt == nil {
		t.Errorf("row = status %s finished %v; want expired and finished", row.Status, row.FinishedAt)
	}
	if row.LastOutcome != nil {
		t.Errorf("last_outcome = %v, want none: an expiry is not a success", *row.LastOutcome)
	}
	if got := h.smtp.connectionCount(); got != 0 {
		t.Errorf("smtp connections = %d, want 0: an expired message is never sent", got)
	}

	control := h.enqueue(h.newNotification(eventKey("expired-control"), func(n *service.NewNotification) {
		at := time.Now().Add(time.Hour)
		n.ExpiresAt = &at
	}))
	if err := h.svc.Deliver(ctx, control, service.Attempt{N: 1, Max: notificationMax()}); err != nil {
		t.Fatalf("control deliver: %v", err)
	}
	if row := h.row(control); row.Status != model.StatusSent {
		t.Errorf("control status = %s, want sent", row.Status)
	}
}

func TestExpiryDuringRetryWaitIsHonored(t *testing.T) {
	h := newHarness(t, harnessOpts{})
	ctx := context.Background()

	id := h.enqueue(h.newNotification(eventKey("expiry-retry"), func(n *service.NewNotification) {
		at := time.Now().Add(time.Hour)
		n.ExpiresAt = &at
	}))
	h.smtp.setOnMessage(func(string) (int, string, bool) { return 450, "mailbox busy", false })

	if err := h.svc.Deliver(ctx, id, service.Attempt{N: 1, Max: notificationMax()}); err == nil {
		t.Fatal("first attempt succeeded, want a transient failure")
	}
	if row := h.row(id); row.Status != model.StatusQueued {
		t.Fatalf("status = %s, want queued for retry", row.Status)
	}

	// The message expires while it waits for the next attempt.
	h.sql(`UPDATE notification_logs SET expires_at = now() - interval '1 second' WHERE id = $1`, id)
	if err := h.svc.Deliver(ctx, id, service.Attempt{N: 2, Max: notificationMax()}); err != nil {
		t.Fatalf("second attempt: %v", err)
	}
	if row := h.row(id); row.Status != model.StatusExpired {
		t.Errorf("status = %s, want expired", row.Status)
	}
	if got := h.smtp.messageCount(); got != 1 {
		t.Errorf("provider calls = %d, want 1: the retry must not send a stale secret", got)
	}

	control := h.enqueue(h.newNotification(eventKey("expiry-retry-control"), func(n *service.NewNotification) {
		at := time.Now().Add(time.Hour)
		n.ExpiresAt = &at
	}))
	if err := h.svc.Deliver(ctx, control, service.Attempt{N: 1, Max: notificationMax()}); err == nil {
		t.Fatal("control attempt succeeded, want a transient failure")
	}
	h.smtp.setOnMessage(nil)
	if err := h.svc.Deliver(ctx, control, service.Attempt{N: 2, Max: notificationMax()}); err != nil {
		t.Fatalf("control second attempt: %v", err)
	}
	if row := h.row(control); row.Status != model.StatusSent {
		t.Errorf("control status = %s, want sent: a future expiry still delivers", row.Status)
	}
}

func testSealKey(fill byte) []byte {
	key := make([]byte, 32)
	for i := range key {
		key[i] = fill
	}
	return key
}

func TestSecretParamsSealedAndErased(t *testing.T) {
	const secret = "482913-otp-value"
	ctx := context.Background()

	t.Run("sealed at rest, present at the provider", func(t *testing.T) {
		h, logs := newLoggingHarness(t, harnessOpts{sealKey: testSealKey(1)})
		id := h.enqueue(h.newNotification(eventKey("sealed"), func(n *service.NewNotification) {
			n.SecretParams = map[string]string{"otp": secret}
			n.Params = map[string]string{"subject": "reset"}
		}))

		row := h.row(id)
		if _, leaked := row.Params["otp"]; leaked {
			t.Error("secret leaked into the public params column")
		}
		if len(row.SealedParams) == 0 {
			t.Fatal("sealed_params is empty, want the ciphertext")
		}
		if bytes.Contains(row.SealedParams, []byte(secret)) {
			t.Error("sealed_params contains the plaintext secret")
		}
		var args *string
		if err := h.pool.QueryRow(ctx, `SELECT args::text FROM river.river_job WHERE kind='notification' AND args->>'notification_id' = $1`, id.String()).Scan(&args); err != nil {
			t.Fatalf("read job args: %v", err)
		}
		if args == nil || strings.Contains(*args, secret) {
			t.Errorf("river job args = %v, want no secret", args)
		}
		if strings.Contains(h.logOutput(), secret) {
			t.Error("the secret appears in the logs")
		}

		if err := h.svc.Deliver(ctx, id, service.Attempt{N: 1, Max: notificationMax()}); err != nil {
			t.Fatalf("deliver: %v", err)
		}
		// The buffer has to be carrying this code path, or the check above proves nothing.
		if !strings.Contains(logs.String(), id.String()) {
			t.Fatalf("the captured logs say nothing about this notification, so the redaction check is vacuous: %q", logs.String())
		}
		if strings.Contains(logs.String(), secret) {
			t.Error("the secret appears in the logs")
		}
		mails := h.smtp.messages()
		if len(mails) != 1 || !strings.Contains(mails[0], secret) {
			t.Fatalf("delivered message does not carry the rendered secret: %v", mails)
		}
		if row := h.row(id); row.SealedParams != nil {
			t.Error("sealed_params survived a successful send")
		}
	})

	t.Run("erased after every terminal state", func(t *testing.T) {
		for _, tc := range []struct {
			name  string
			setup func(*harness) uuid.UUID
		}{
			{"sent", func(h *harness) uuid.UUID { return h.enqueue(h.sealed(eventKey("erase-sent"))) }},
			{"failed", func(h *harness) uuid.UUID {
				id := h.enqueue(h.sealed(eventKey("erase-failed")))
				h.smtp.setOnMessage(func(string) (int, string, bool) { return 550, "no such user", false })
				return id
			}},
			{"expired", func(h *harness) uuid.UUID {
				id := h.enqueue(h.sealed(eventKey("erase-expired")))
				h.sql(`UPDATE notification_logs SET expires_at = now() - interval '1 second' WHERE id = $1`, id)
				return id
			}},
			{"unavailable", func(h *harness) uuid.UUID {
				id := h.enqueue(h.sealed(eventKey("erase-unavailable")))
				h.sql(`UPDATE notification_logs SET channel = 'whatsapp' WHERE id = $1`, id)
				return id
			}},
		} {
			t.Run(tc.name, func(t *testing.T) {
				h := newHarness(t, harnessOpts{sealKey: testSealKey(2)})
				id := tc.setup(h)
				if row := h.row(id); len(row.SealedParams) == 0 {
					t.Fatal("fixture is not sealed")
				}
				if err := h.svc.Deliver(context.Background(), id, service.Attempt{N: 1, Max: notificationMax()}); err != nil && tc.name != "failed" {
					t.Fatalf("deliver: %v", err)
				}
				row := h.row(id)
				if row.Status != model.NotificationStatus(tc.name) {
					t.Fatalf("status = %s, want %s", row.Status, tc.name)
				}
				if row.SealedParams != nil {
					t.Errorf("sealed_params = %d bytes, want NULL after %s", len(row.SealedParams), tc.name)
				}
			})
		}
	})

	t.Run("enqueue without a key fails", func(t *testing.T) {
		h := newHarness(t, harnessOpts{})
		ctx := context.Background()
		tx, err := h.pool.Begin(ctx)
		if err != nil {
			t.Fatalf("begin: %v", err)
		}
		defer func() { _ = tx.Rollback(ctx) }()
		n := h.newNotification(eventKey("no-key"), func(n *service.NewNotification) {
			n.SecretParams = map[string]string{"otp": secret}
		})
		if err := h.enqueueTx(ctx, tx, n); !errors.Is(err, service.ErrSealKeyRequired) {
			t.Errorf("err = %v, want ErrSealKeyRequired", err)
		}
	})
}

// sealed is a notification whose render params include a secret.
func (h *harness) sealed(event string) service.NewNotification {
	return h.newNotification(event, func(n *service.NewNotification) {
		n.SecretParams = map[string]string{"otp": "482913-otp-value"}
	})
}

// newLoggingHarness builds a harness whose logger writes to a buffer the test can read.
func newLoggingHarness(t *testing.T, opts harnessOpts) (*harness, *syncBuffer) {
	t.Helper()
	logs := &syncBuffer{}
	opts.logger = slog.New(slog.NewTextHandler(logs, &slog.HandlerOptions{Level: slog.LevelDebug}))
	h := newHarness(t, opts)
	h.logOutput = func() string { return logs.String() }
	return h, logs
}

// assertNoLeak checks a payload never reaches the places an unopenable secret could be written to.
func assertNoLeak(t *testing.T, h *harness, logs *syncBuffer, id uuid.UUID, err error, secret string, sealed []byte) {
	t.Helper()
	needles := map[string]string{
		"secret value":           secret,
		"sealed bytes as hex":    hex.EncodeToString(sealed),
		"sealed bytes as base64": base64.StdEncoding.EncodeToString(sealed),
	}
	haystacks := map[string]string{
		"application logs": logs.String(),
		"last_error":       deref(h.row(id).LastError),
		"err.Error()":      err.Error(),
		"river job errors": h.jobErrors(id),
	}
	for name, hay := range haystacks {
		for what, needle := range needles {
			if needle != "" && strings.Contains(hay, needle) {
				t.Errorf("%s leaks the %s", name, what)
			}
		}
	}
}

func TestSealWrongKeyOrMissingWorkerKeyFailsClosed(t *testing.T) {
	const secret = "482913-otp-value"
	ctx := context.Background()

	h, logs := newLoggingHarness(t, harnessOpts{sealKey: testSealKey(3)})
	id := h.enqueue(h.sealed(eventKey("wrong-key")))
	sealed := h.row(id).SealedParams
	if len(sealed) == 0 {
		t.Fatal("the fixture is not sealed, so the wrong-key assertions would be vacuous")
	}

	// Rebuild the service with a different key: opening must fail closed, not fall through.
	other := h.serviceWithKey(testSealKey(4))
	err := other.Deliver(ctx, id, service.Attempt{N: 1, Max: notificationMax()})
	if !service.IsPermanent(err) {
		t.Fatalf("err = %v, want a permanent failure", err)
	}
	row := h.row(id)
	if row.Status != model.StatusFailed || row.SealedParams != nil {
		t.Errorf("row = status %s sealed %d bytes; want failed and erased", row.Status, len(row.SealedParams))
	}
	if got := h.smtp.connectionCount(); got != 0 {
		t.Errorf("smtp connections = %d, want 0: an unopenable payload is never sent", got)
	}
	if want := service.ErrSealOpen.Error(); row.LastError == nil || *row.LastError != want {
		t.Errorf("last_error = %q, want the fixed text %q", deref(row.LastError), want)
	}
	assertNoLeak(t, h, logs, id, err, secret, sealed)
	// The buffer has to be carrying this code path, or the leak checks above prove nothing.
	if !strings.Contains(logs.String(), "sealed payload could not be opened") {
		t.Errorf("the captured logs do not mention the failed open, so the checks are vacuous: %q", logs.String())
	}

	// No key at all behaves the same way.
	none := h.serviceWithKey(nil)
	second := h.enqueue(h.sealed(eventKey("no-worker-key")))
	sealedSecond := h.row(second).SealedParams
	if len(sealedSecond) == 0 {
		t.Fatal("the fixture is not sealed, so the missing-key assertions would be vacuous")
	}
	err = none.Deliver(ctx, second, service.Attempt{N: 1, Max: notificationMax()})
	if !service.IsPermanent(err) {
		t.Fatalf("err = %v, want a permanent failure", err)
	}
	row = h.row(second)
	if row.Status != model.StatusFailed || row.SealedParams != nil {
		t.Errorf("row = status %s sealed %d bytes; want failed and erased", row.Status, len(row.SealedParams))
	}
	if want := service.ErrNoSealKey.Error(); row.LastError == nil || *row.LastError != want {
		t.Errorf("last_error = %q, want the fixed text %q", deref(row.LastError), want)
	}
	assertNoLeak(t, h, logs, second, err, secret, sealedSecond)
}

func TestLogsAndErrorsAreRedacted(t *testing.T) {
	const (
		recipient = "leaky@example.com"
		password  = "s3cret-relay-password"
		secret    = "90210-secret-otp"
	)

	// assertClean checks the three places a leak could plausibly land.
	assertClean := func(t *testing.T, h *harness, logs *syncBuffer, id uuid.UUID) {
		t.Helper()
		haystacks := map[string]string{
			"application logs": logs.String(),
			"last_error":       deref(h.row(id).LastError),
			"river job errors": h.jobErrors(id),
		}
		for name, hay := range haystacks {
			for what, needle := range map[string]string{"recipient": recipient, "smtp password": password, "secret value": secret} {
				if strings.Contains(hay, needle) {
					t.Errorf("%s leaks the %s: %s", name, what, hay)
				}
			}
		}
	}

	newRedactionHarness := func(t *testing.T) (*harness, *syncBuffer) {
		t.Helper()
		return newLoggingHarness(t, harnessOpts{
			sealKey: testSealKey(5),
			smtpMutate: func(c *config.SMTPConfig) {
				c.Username = "relay"
				c.Password = password
			},
		})
	}

	t.Run("a 535 AUTH reply echoing secrets is sanitized", func(t *testing.T) {
		h, logs := newRedactionHarness(t)
		h.smtp.authEnabled = true
		h.smtp.authReplyCode = 535
		h.smtp.authReplyText = "auth failed for " + recipient + " password=" + password + " otp=" + secret

		id := h.enqueue(h.newNotification(eventKey("redact-auth"), func(n *service.NewNotification) {
			n.Recipient = recipient
			n.SecretParams = map[string]string{"otp": secret}
		}))

		// A working client means River's own logger and error column are captured too.
		h.startWorker(t, fastRetry)
		waitFor(t, "status failed", func() bool { return h.row(id).Status == model.StatusFailed })
		waitFor(t, "job cancelled", func() bool {
			state, _, _, _ := h.jobState(id)
			return state == "cancelled"
		})
		waitFor(t, "river to record the error", func() bool { return h.jobErrors(id) != "" })

		assertClean(t, h, logs, id)
		if !strings.Contains(logs.String(), "smtp: authentication failed") {
			t.Errorf("logs do not carry the fixed class text: %s", logs.String())
		}
		if got := h.smtp.messageCount(); got != 0 {
			t.Errorf("messages = %d, want 0: authentication failed before any body was sent", got)
		}
	})

	t.Run("a retryable reply echoing secrets is sanitized", func(t *testing.T) {
		h, logs := newRedactionHarness(t)
		h.smtp.setOnMessage(func(string) (int, string, bool) {
			return 451, "try later " + recipient + " password=" + password + " otp=" + secret, false
		})

		id := h.enqueue(h.newNotification(eventKey("redact-451"), func(n *service.NewNotification) {
			n.Recipient = recipient
			n.SecretParams = map[string]string{"otp": secret}
		}))

		// The 4xx is retryable, so River records the failure it was handed.
		h.startWorker(t, fastRetry)
		waitFor(t, "river to record the retry", func() bool { return h.jobErrors(id) != "" })
		if got := h.smtp.messageCount(); got == 0 {
			t.Fatal("the provider never saw the body")
		}
		assertClean(t, h, logs, id)
	})

	t.Run("post-data rejection echoes recipient and secret", func(t *testing.T) {
		h, logs := newRedactionHarness(t)
		h.smtp.setOnMessage(func(string) (int, string, bool) {
			return 550, "rejected " + recipient + " secret=" + secret, false
		})

		id := h.enqueue(h.newNotification(eventKey("redact-550"), func(n *service.NewNotification) {
			n.Recipient = recipient
			n.SecretParams = map[string]string{"otp": secret}
		}))

		h.startWorker(t, fastRetry)
		waitFor(t, "status failed", func() bool { return h.row(id).Status == model.StatusFailed })
		waitFor(t, "job cancelled", func() bool {
			state, _, _, _ := h.jobState(id)
			return state == "cancelled"
		})
		waitFor(t, "river to record the error", func() bool { return h.jobErrors(id) != "" })

		if got := h.smtp.messageCount(); got != 1 {
			t.Fatalf("messages = %d, want 1: the provider did see the body", got)
		}
		assertClean(t, h, logs, id)
	})
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func TestNextRetryIsAbsoluteTime(t *testing.T) {
	w := &notificationWorker{retryDelay: service.RetryDelay}
	job := &river.Job[service.NotificationJobArgs]{JobRow: &rivertype.JobRow{Attempt: 1}}

	before := time.Now()
	got := w.NextRetry(job)
	after := time.Now()
	if got.Before(before.Add(29*time.Second)) || got.After(after.Add(31*time.Second)) {
		t.Errorf("NextRetry(attempt 1) = %s, want an absolute wall-clock time about 30s ahead", got)
	}
	if job.Attempt != 1 {
		t.Fatalf("fixture attempt = %d", job.Attempt)
	}
}

func TestProductionScheduleSetsRiverScheduledAt(t *testing.T) {
	h := newHarness(t, harnessOpts{})
	ctx := context.Background()
	id := h.enqueue(h.newNotification(eventKey("schedule"), nil))
	h.smtp.setOnMessage(func(string) (int, string, bool) { return 450, "mailbox busy", false })

	// The production schedule, run through a real client: one failure, then a scheduled retry.
	h.startWorker(t, service.RetryDelay)
	waitForWithin(t, 30*time.Second, "the retry to be scheduled", func() bool {
		state, _, _, _ := h.jobState(id)
		return state == "scheduled" || state == "retryable"
	})

	var scheduledAt time.Time
	if err := h.pool.QueryRow(ctx, `SELECT scheduled_at FROM river.river_job WHERE kind='notification' AND args->>'notification_id' = $1`, id.String()).Scan(&scheduledAt); err != nil {
		t.Fatalf("read scheduled_at: %v", err)
	}
	delta := time.Until(scheduledAt)
	if delta < 25*time.Second || delta > 35*time.Second {
		t.Errorf("scheduled_at is %s away, want about 30s after the first failure", delta)
	}
}
