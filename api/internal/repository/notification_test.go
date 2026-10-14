package repository

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/tarunaeduka/bimbel-tryout/api/internal/model"
)

// outOf returns the pointer form Finish takes for a recorded verdict.
func outOf(o model.Outcome) *model.Outcome { return &o }

// reconcileText is the caller-chosen reason the repository only stores; the text itself is service policy.
const reconcileText = "reconciled: delivery job ended without a result"

// insertNotification commits one row through the real positional-conflict insert.
func insertNotification(t *testing.T, pool *pgxpool.Pool, repo *NotificationRepository, n model.Notification) uuid.UUID {
	t.Helper()
	if n.ID == uuid.Nil {
		n.ID = uuid.New()
	}
	if n.Template == "" {
		n.Template = "smtp-test"
	}
	if n.Channel == "" {
		n.Channel = model.ChannelEmail
	}
	ctx := context.Background()
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	id, err := repo.Create(ctx, tx, n)
	if err != nil {
		_ = tx.Rollback(ctx)
		t.Fatalf("create notification: %v", err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatalf("commit: %v", err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(ctx, `DELETE FROM notification_logs WHERE id = $1`, n.ID) })
	return id
}

func find(t *testing.T, repo *NotificationRepository, tenantID *uuid.UUID, id uuid.UUID) (model.Notification, error) {
	t.Helper()
	return repo.Find(context.Background(), tenantID, id)
}

func rowTenant(t *testing.T, pool *pgxpool.Pool, id uuid.UUID) *uuid.UUID {
	t.Helper()
	var tenantID *uuid.UUID
	if err := pool.QueryRow(context.Background(), `SELECT tenant_id FROM notification_logs WHERE id = $1`, id).Scan(&tenantID); err != nil {
		t.Fatalf("read row: %v", err)
	}
	return tenantID
}

func TestRepoScopeIsolation(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	tenants := NewTenantRepository(pool)
	repo := NewNotificationRepository(pool)
	a, b := newTenant(t, tenants), newTenant(t, tenants)

	event := "order-created-" + uuid.NewString()[:8]
	rowA := insertNotification(t, pool, repo, model.Notification{TenantID: &a.ID, EventKey: event, Recipient: "a@example.com"})
	rowB := insertNotification(t, pool, repo, model.Notification{TenantID: &b.ID, EventKey: event, Recipient: "b@example.com"})
	rowP := insertNotification(t, pool, repo, model.Notification{TenantID: nil, EventKey: event, Recipient: "root@example.com"})

	if rowA == uuid.Nil || rowB == uuid.Nil || rowP == uuid.Nil {
		t.Fatal("fixtures collided; identities must differ across scope, event, recipient and channel")
	}

	// Cross-tenant and cross-scope reads are indistinguishable from a missing row.
	denied := []struct {
		name     string
		tenantID *uuid.UUID
		id       uuid.UUID
	}{
		{"A reads B", &a.ID, rowB},
		{"B reads A", &b.ID, rowA},
		{"A reads platform", &a.ID, rowP},
		{"platform reads A", nil, rowA},
		{"platform reads B", nil, rowB},
	}
	for _, tc := range denied {
		if _, err := find(t, repo, tc.tenantID, tc.id); !errors.Is(err, ErrNotFound) {
			t.Errorf("Find(%s): err = %v, want ErrNotFound", tc.name, err)
		}
	}

	for _, tc := range []struct {
		name     string
		tenantID *uuid.UUID
		id       uuid.UUID
	}{
		{"A reads A", &a.ID, rowA},
		{"B reads B", &b.ID, rowB},
		{"platform reads platform", nil, rowP},
	} {
		if _, err := find(t, repo, tc.tenantID, tc.id); err != nil {
			t.Errorf("Find(%s): err = %v, want nil", tc.name, err)
		}
	}

	// Every write path refuses a foreign scope, including a platform row against a tenant.
	token := uuid.New()
	if _, err := repo.Claim(ctx, &b.ID, rowA, token, time.Minute); !errors.Is(err, ErrNotFound) {
		t.Errorf("Claim(B, rowA): err = %v, want ErrNotFound: the claim found no row it could take", err)
	}
	errText := "x"
	for _, tc := range []struct {
		name     string
		tenantID *uuid.UUID
		id       uuid.UUID
	}{
		{"A", &a.ID, rowP},
		{"B", &b.ID, rowP},
		{"platform", nil, rowA},
		{"platform", nil, rowB},
	} {
		if ok, err := repo.Finish(ctx, tc.tenantID, tc.id, token, model.StatusSent, 1, outOf(model.OutcomeOK), &errText); err != nil || ok {
			t.Errorf("Finish(%s scope, %s row): ok = %v, err = %v; want false, nil", tc.name, tc.name, ok, err)
		}
		if ok, err := repo.Release(ctx, tc.tenantID, tc.id, token, 1, model.OutcomeTransient, &errText); err != nil || ok {
			t.Errorf("Release(%s scope, %s row): ok = %v, err = %v; want false, nil", tc.name, tc.name, ok, err)
		}
		if ok, err := repo.FailOrphan(ctx, tc.tenantID, tc.id, errText); err != nil || ok {
			t.Errorf("FailOrphan(%s scope, %s row): ok = %v, err = %v; want false, nil", tc.name, tc.name, ok, err)
		}
		if _, err := repo.Claim(ctx, tc.tenantID, tc.id, uuid.New(), time.Minute); !errors.Is(err, ErrNotFound) {
			t.Errorf("Claim(%s scope, %s row): err = %v, want ErrNotFound", tc.name, tc.name, err)
		}
	}
	if tenantID := rowTenant(t, pool, rowP); tenantID != nil {
		t.Fatalf("foreign-scope writes changed the row: tenant_id = %v", tenantID)
	}
	row, err := find(t, repo, nil, rowP)
	if err != nil || row.Status != model.StatusQueued || row.Attempts != 0 {
		t.Errorf("platform row after denied writes = status %s attempts %d err %v; want queued 0 nil", row.Status, row.Attempts, err)
	}

	// The two named exceptions are the only unscoped reads, and they are not scope-reachable.
	got, err := repo.FindForDelivery(ctx, rowB)
	if err != nil || got.ID != rowB {
		t.Errorf("FindForDelivery(rowB) = %v, %v; want rowB", got.ID, err)
	}
	if _, err := repo.FindForDelivery(ctx, uuid.New()); !errors.Is(err, ErrNotFound) {
		t.Errorf("FindForDelivery(unknown): err = %v, want ErrNotFound", err)
	}

	// A future cutoff keeps this test out of the one-hour window the worker's Reconcile owns.
	candidates, err := repo.ReconcileCandidates(ctx, time.Now().Add(time.Hour), 10000)
	if err != nil {
		t.Fatalf("ReconcileCandidates: %v", err)
	}
	seen := map[uuid.UUID]bool{}
	for _, c := range candidates {
		seen[c.ID] = true
	}
	for name, id := range map[string]uuid.UUID{"A": rowA, "B": rowB, "platform": rowP} {
		if !seen[id] {
			t.Errorf("reconcile scan missed the %s row; the maintenance exception must span scopes", name)
		}
	}
}

func TestIdentityIndexSuppressesDuplicates(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	tenants := NewTenantRepository(pool)
	repo := NewNotificationRepository(pool)
	a, b := newTenant(t, tenants), newTenant(t, tenants)

	base := model.Notification{EventKey: "order-created-" + uuid.NewString()[:8], Recipient: "same@example.com", Channel: model.ChannelEmail}

	first := insertNotification(t, pool, repo, model.Notification{
		TenantID: &a.ID, EventKey: base.EventKey, Recipient: base.Recipient, Channel: base.Channel,
	})
	if first == uuid.Nil {
		t.Fatal("first insert was suppressed")
	}
	duplicate := insertNotification(t, pool, repo, model.Notification{
		TenantID: &a.ID, EventKey: base.EventKey, Recipient: base.Recipient, Channel: base.Channel,
	})
	if duplicate != uuid.Nil {
		t.Errorf("same tenant/event/recipient/channel: id = %s, want uuid.Nil (suppressed)", duplicate)
	}

	otherTenant := insertNotification(t, pool, repo, model.Notification{
		TenantID: &b.ID, EventKey: base.EventKey, Recipient: base.Recipient, Channel: base.Channel,
	})
	if otherTenant == uuid.Nil {
		t.Error("another tenant with the same identity was suppressed; scope must be part of the identity")
	}

	otherChannel := insertNotification(t, pool, repo, model.Notification{
		TenantID: &a.ID, EventKey: base.EventKey, Recipient: base.Recipient, Channel: model.ChannelWhatsApp,
	})
	if otherChannel == uuid.Nil {
		t.Error("another channel with the same identity was suppressed; channel must be part of the identity")
	}

	platform := insertNotification(t, pool, repo, model.Notification{
		TenantID: nil, EventKey: base.EventKey, Recipient: base.Recipient, Channel: base.Channel,
	})
	if platform == uuid.Nil {
		t.Fatal("first platform insert was suppressed")
	}
	platformDuplicate := insertNotification(t, pool, repo, model.Notification{
		TenantID: nil, EventKey: base.EventKey, Recipient: base.Recipient, Channel: base.Channel,
	})
	if platformDuplicate != uuid.Nil {
		t.Errorf("same platform identity: id = %s, want uuid.Nil (suppressed)", platformDuplicate)
	}

	var count int
	if err := pool.QueryRow(ctx,
		`SELECT count(*) FROM notification_logs WHERE event_key = $1 AND recipient = $2`, base.EventKey, base.Recipient).Scan(&count); err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 4 {
		t.Errorf("rows for that event/recipient = %d, want 4 (A email, B email, A whatsapp, platform email)", count)
	}

	// The all-zero tenant is a sentinel inside the identity index, so it can never be a real scope.
	zero := uuid.Nil
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := repo.Create(ctx, tx, model.Notification{
		ID: uuid.New(), TenantID: &zero, EventKey: "zero-tenant", Recipient: "z@example.com",
		Channel: model.ChannelEmail, Template: "smtp-test",
	}); err == nil {
		t.Error("insert with the all-zero tenant succeeded, want a foreign-key rejection")
	}
}

// Deliver must be able to take the one unscoped lookup and still be unable to touch a foreign scope.
func TestFindForDeliveryIsTheOnlyByIDRead(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	tenants := NewTenantRepository(pool)
	repo := NewNotificationRepository(pool)
	a := newTenant(t, tenants)

	id := insertNotification(t, pool, repo, model.Notification{TenantID: &a.ID, EventKey: "evt-" + uuid.NewString()[:8], Recipient: "a@example.com"})
	got, err := repo.FindForDelivery(ctx, id)
	if err != nil {
		t.Fatalf("FindForDelivery: %v", err)
	}
	if got.TenantID == nil || *got.TenantID != a.ID {
		t.Fatalf("FindForDelivery scope = %v, want %s", got.TenantID, a.ID)
	}
	// The row's own scope is then sufficient for every later scoped call.
	token := uuid.New()
	if _, err := repo.Claim(ctx, got.TenantID, got.ID, token, time.Minute); err != nil {
		t.Fatalf("Claim with the discovered scope: %v", err)
	}
	if ok, err := repo.Finish(ctx, got.TenantID, got.ID, token, model.StatusSent, 1, outOf(model.OutcomeOK), nil); err != nil || !ok {
		t.Fatalf("Finish with the discovered scope: ok = %v, err = %v", ok, err)
	}
}

func TestClaimIsExclusiveAndLeaseBound(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	tenants := NewTenantRepository(pool)
	repo := NewNotificationRepository(pool)
	a := newTenant(t, tenants)
	id := insertNotification(t, pool, repo, model.Notification{TenantID: &a.ID, EventKey: "evt-" + uuid.NewString()[:8], Recipient: "a@example.com"})

	first, err := repo.Claim(ctx, &a.ID, id, uuid.New(), time.Minute)
	if err != nil {
		t.Fatalf("first claim: %v", err)
	}
	if first.ClaimToken == nil || first.ClaimedUntil == nil || !first.ClaimedUntil.After(time.Now()) {
		t.Fatalf("claim did not record a live lease: token %v until %v", first.ClaimToken, first.ClaimedUntil)
	}
	if _, err := repo.Claim(ctx, &a.ID, id, uuid.New(), time.Minute); !errors.Is(err, ErrNotFound) {
		t.Errorf("second claim: err = %v, want ErrNotFound: the claim found no row it could take", err)
	}

	// An expired lease stops owning the row.
	if _, err := pool.Exec(ctx, `UPDATE notification_logs SET claimed_until = now() - interval '1 second' WHERE id = $1`, id); err != nil {
		t.Fatalf("expire lease: %v", err)
	}
	if ok, err := repo.Finish(ctx, &a.ID, id, *first.ClaimToken, model.StatusSent, 1, outOf(model.OutcomeOK), nil); err != nil || ok {
		t.Errorf("Finish with an expired lease: ok = %v, err = %v; want false, nil", ok, err)
	}
	if ok, err := repo.Release(ctx, &a.ID, id, *first.ClaimToken, 1, model.OutcomeTransient, nil); err != nil || ok {
		t.Errorf("Release with an expired lease: ok = %v, err = %v; want false, nil", ok, err)
	}

	second, err := repo.Claim(ctx, &a.ID, id, uuid.New(), time.Minute)
	if err != nil {
		t.Fatalf("reclaim after expiry: %v", err)
	}
	if second.ClaimToken == nil || *second.ClaimToken == *first.ClaimToken {
		t.Error("reclaim reused the previous token")
	}
}

func TestFailOrphanSkipsLiveJobs(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	tenants := NewTenantRepository(pool)
	repo := NewNotificationRepository(pool)
	a := newTenant(t, tenants)
	id := insertNotification(t, pool, repo, model.Notification{TenantID: &a.ID, EventKey: "evt-" + uuid.NewString()[:8], Recipient: "a@example.com"})

	jobID := insertRiverJob(t, pool, id, "available", 0)
	if ok, err := repo.FailOrphan(ctx, &a.ID, id, reconcileText); err != nil || ok {
		t.Errorf("FailOrphan with a live job: ok = %v, err = %v; want false, nil", ok, err)
	}
	row, err := find(t, repo, &a.ID, id)
	if err != nil || row.Status != model.StatusQueued {
		t.Fatalf("row = status %s err %v; want queued", row.Status, err)
	}

	if _, err := pool.Exec(ctx, `UPDATE river.river_job SET state = 'discarded', finalized_at = now() WHERE id = $1`, jobID); err != nil {
		t.Fatalf("discard job: %v", err)
	}
	if ok, err := repo.FailOrphan(ctx, &a.ID, id, reconcileText); err != nil || !ok {
		t.Fatalf("FailOrphan with a discarded job: ok = %v, err = %v; want true, nil", ok, err)
	}
	row, err = find(t, repo, &a.ID, id)
	if err != nil {
		t.Fatalf("re-read: %v", err)
	}
	if row.Status != model.StatusFailed || row.FinishedAt == nil || row.LastError == nil || *row.LastError != reconcileText {
		t.Errorf("reconciled row = status %s finished %v error %v; want failed with the fixed reason", row.Status, row.FinishedAt, row.LastError)
	}
}

// insertRiverJob writes the minimal river_job row Reconcile reasons about, on a queue no worker listens on.
func insertRiverJob(t *testing.T, pool *pgxpool.Pool, notificationID uuid.UUID, state string, attempt int) int64 {
	t.Helper()
	ctx := context.Background()
	var id int64
	err := pool.QueryRow(ctx,
		`INSERT INTO river.river_job (state, attempt, max_attempts, kind, args, queue, finalized_at)
		 VALUES ($1::river.river_job_state, $2, 6, 'notification', jsonb_build_object('notification_id', $3::text), 'p02-repo-test',
		         CASE WHEN $1 IN ('completed','discarded','cancelled') THEN now() ELSE NULL END)
		 RETURNING id`, state, attempt, notificationID.String()).Scan(&id)
	if err != nil {
		t.Fatalf("insert river job: %v", err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(ctx, `DELETE FROM river.river_job WHERE id = $1`, id) })
	return id
}
