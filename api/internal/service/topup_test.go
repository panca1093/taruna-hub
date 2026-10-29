package service

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tarunaeduka/bimbel-tryout/api/internal/model"
	"github.com/tarunaeduka/bimbel-tryout/api/internal/repository"
)

type topupHarness struct {
	walletHarness
	svc    *TopupService
	assets *AssetService
}

func newTopupHarness(t *testing.T, jobs JobInserter) topupHarness {
	t.Helper()
	h := topupHarness{walletHarness: newWalletHarness(t)}
	h.assets = NewAssetService(h.pool, repository.NewAssetRepository(h.pool), &memStore{}, time.Minute)
	notes, err := NewNotificationService(repository.NewNotificationRepository(h.pool), jobs, nil,
		bytes.Repeat([]byte{5}, SealKeyBytes), slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatalf("notification service: %v", err)
	}
	h.svc = NewTopupService(h.pool, repository.NewTopupRepository(h.pool), h.walletHarness.svc, h.assets, notes)
	// Registered after the wallet cleanup, so requests and proofs go before the users and ledger they reference.
	t.Cleanup(func() {
		ids := []uuid.UUID{h.tenant.ID, h.other.ID}
		for _, sql := range []string{
			`DELETE FROM topup_requests WHERE tenant_id = ANY($1)`,
			`DELETE FROM assets WHERE tenant_id = ANY($1)`,
			`DELETE FROM tenant_settings WHERE tenant_id = ANY($1)`,
		} {
			if _, err := h.pool.Exec(context.Background(), sql, ids); err != nil {
				t.Errorf("cleanup top-up fixture: %v", err)
			}
		}
	})
	return h
}

func (h topupHarness) proof(t *testing.T, p model.Principal) uuid.UUID {
	t.Helper()
	asset, err := h.assets.Upload(context.Background(), h.scope(h.tenant), p, model.PurposePaymentProof, testPNG(t, 8, 8))
	if err != nil {
		t.Fatalf("upload proof: %v", err)
	}
	return asset.ID
}

func (h topupHarness) request(t *testing.T, p model.Principal, amount string) model.Topup {
	t.Helper()
	proof := h.proof(t, p)
	topup, _, err := h.svc.Create(context.Background(), h.scope(h.tenant), p, CreateTopupInput{
		Amount: amount, Key: "key-" + uuid.NewString()[:12], ProofAssetID: &proof,
	})
	if err != nil {
		t.Fatalf("create top-up %s: %v", amount, err)
	}
	return topup
}

func (h topupHarness) count(t *testing.T, sql string, args ...any) int {
	t.Helper()
	var n int
	if err := h.pool.QueryRow(context.Background(), sql, args...).Scan(&n); err != nil {
		t.Fatalf("count: %v", err)
	}
	return n
}

func (h topupHarness) ledger(t *testing.T, userID uuid.UUID) int {
	return h.count(t, `SELECT count(*) FROM wallet_entries WHERE user_id = $1`, userID)
}

func (h topupHarness) notifications(t *testing.T, template string) int {
	return h.count(t, `SELECT count(*) FROM notification_logs WHERE tenant_id = $1 AND template = $2`, h.tenant.ID, template)
}

func (h topupHarness) setMinimum(t *testing.T, minimum string) {
	t.Helper()
	if _, err := h.pool.Exec(context.Background(),
		`INSERT INTO tenant_settings (tenant_id, min_topup) VALUES ($1, $2::numeric)`, h.tenant.ID, minimum); err != nil {
		t.Fatalf("set minimum: %v", err)
	}
}

func TestParseWholeRupiah(t *testing.T) {
	good := map[string]string{"10000": "10000.00", "10000.00": "10000.00", "10000.0": "10000.00", "1": "1.00", "999999999": "999999999.00"}
	for in, want := range good {
		if got, ok := parseWholeRupiah(in); !ok || got != want {
			t.Errorf("parseWholeRupiah(%q) = %q, %v; want %q", in, got, ok, want)
		}
	}
	for _, in := range []string{"", " 10000", "10000 ", "0", "0.00", "-10000", "+10000", "1e4", "NaN", ".5", "10000.", "10000.50", "10000.01", "10,000", "１２３４５", "10000.000", "99999999999999"} {
		if got, ok := parseWholeRupiah(in); ok {
			t.Errorf("parseWholeRupiah(%q) = %q, want it refused", in, got)
		}
	}
}

func TestTopupCreateValidatesAmountKeyAndProof(t *testing.T) {
	h := newTopupHarness(t, noJobs{})
	p := h.principal(h.tenant, h.student)
	other := h.principal(h.tenant, h.second)
	ctx := context.Background()
	proof := h.proof(t, p)
	foreign := h.proof(t, other)
	create := func(in CreateTopupInput) error {
		_, _, err := h.svc.Create(ctx, h.scope(h.tenant), p, in)
		return err
	}
	key := func() string { return "key-" + uuid.NewString()[:12] }

	for _, amount := range []string{"", "0", "-50000", "abc", "1e5", "50000.50", "9999", "9999.00"} {
		wantInputField(t, create(CreateTopupInput{Amount: amount, Key: key(), ProofAssetID: &proof}), "amount")
	}
	wantInputField(t, create(CreateTopupInput{Amount: "50000", Key: "x", ProofAssetID: &proof}), "idempotency_key")
	wantInputField(t, create(CreateTopupInput{Amount: "50000", Key: key()}), "proof_asset_id")
	missing := uuid.New()
	wantInputField(t, create(CreateTopupInput{Amount: "50000", Key: key(), ProofAssetID: &missing}), "proof_asset_id")
	wantInputField(t, create(CreateTopupInput{Amount: "50000", Key: key(), ProofAssetID: &foreign}), "proof_asset_id")
	if n := h.count(t, `SELECT count(*) FROM topup_requests WHERE user_id = $1`, h.student.ID); n != 0 {
		t.Errorf("%d requests written by rejected input, want 0", n)
	}

	// The minimum is checked on the server and follows the Admin setting.
	h.setMinimum(t, "50000")
	wantInputField(t, create(CreateTopupInput{Amount: "49999", Key: key(), ProofAssetID: &proof}), "amount")
	if err := create(CreateTopupInput{Amount: "50000", Key: key(), ProofAssetID: &proof}); err != nil {
		t.Errorf("an amount equal to the minimum: %v, want it accepted", err)
	}
	if _, _, err := h.svc.Create(ctx, h.scope(h.tenant), h.principal(h.tenant, h.admin), CreateTopupInput{Amount: "50000", Key: key(), ProofAssetID: &proof}); !errors.Is(err, ErrTopupForbidden) {
		t.Errorf("staff creating a top-up: %v, want ErrTopupForbidden", err)
	}
}

func TestTopupAcceptsLargeWholeRupiahWithoutUpperBound(t *testing.T) {
	h := newTopupHarness(t, noJobs{})
	p := h.principal(h.tenant, h.student)
	if topup := h.request(t, p, "5000000"); topup.Amount != "5000000.00" || topup.Status != model.TopupPending {
		t.Errorf("large top-up = %+v, want a pending 5000000.00 request", topup)
	}
}

func TestTopupCreateCreditsNothingAndIsIdempotent(t *testing.T) {
	h := newTopupHarness(t, noJobs{})
	p := h.principal(h.tenant, h.student)
	ctx := context.Background()
	proof := h.proof(t, p)
	in := CreateTopupInput{Amount: "75000", Key: "retry-key-0001", ProofAssetID: &proof}

	first, replayed, err := h.svc.Create(ctx, h.scope(h.tenant), p, in)
	if err != nil || replayed {
		t.Fatalf("create = %v, replayed %v", err, replayed)
	}
	if first.Status != model.TopupPending || first.Amount != "75000.00" || first.Method != model.TopupManual ||
		first.UserID != h.student.ID || TopupCode(first.Number) == "" {
		t.Fatalf("top-up = %+v, want a pending manual request of the student", first)
	}
	if h.balance(t, h.student.ID) != "0.00" || h.ledger(t, h.student.ID) != 0 {
		t.Errorf("a pending request moved money: balance %s, ledger %d", h.balance(t, h.student.ID), h.ledger(t, h.student.ID))
	}
	if h.count(t, `SELECT count(*) FROM orders WHERE user_id = $1`, h.student.ID) != 0 ||
		h.count(t, `SELECT count(*) FROM enrollments WHERE user_id = $1`, h.student.ID) != 0 {
		t.Error("a top-up created an order or an enrollment")
	}

	again, replayed, err := h.svc.Create(ctx, h.scope(h.tenant), p, in)
	if err != nil || !replayed || again.ID != first.ID {
		t.Errorf("retry = %+v replayed %v err %v, want the same request", again, replayed, err)
	}
	if n := h.count(t, `SELECT count(*) FROM topup_requests WHERE user_id = $1`, h.student.ID); n != 1 {
		t.Errorf("%d requests after a retry, want 1", n)
	}
	changed := in
	changed.Amount = "80000"
	if _, _, err := h.svc.Create(ctx, h.scope(h.tenant), p, changed); !errors.Is(err, ErrTopupKeyConflict) {
		t.Errorf("same key with another amount: %v, want ErrTopupKeyConflict", err)
	}
}

func TestTopupProofBacksOneRequest(t *testing.T) {
	h := newTopupHarness(t, noJobs{})
	p := h.principal(h.tenant, h.student)
	proof := h.proof(t, p)
	create := func() error {
		_, _, err := h.svc.Create(context.Background(), h.scope(h.tenant), p, CreateTopupInput{
			Amount: "20000", Key: "key-" + uuid.NewString()[:12], ProofAssetID: &proof,
		})
		return err
	}
	if err := create(); err != nil {
		t.Fatalf("first request: %v", err)
	}
	wantInputField(t, create(), "proof_asset_id")
}

func TestTopupApproveCreditsOnceAndRecordsActor(t *testing.T) {
	h := newTopupHarness(t, noJobs{})
	p := h.principal(h.tenant, h.student)
	ctx, scope := context.Background(), h.scope(h.tenant)
	topup := h.request(t, p, "60000")

	approved, err := h.svc.Approve(ctx, scope, h.principal(h.tenant, h.operator), topup.ID)
	if err != nil || approved.Status != model.TopupApproved || approved.ApprovedAt == nil || approved.ReviewedBy == nil || *approved.ReviewedBy != h.operator.ID {
		t.Fatalf("approve = %+v (err %v), want an approved request reviewed by the operator", approved, err)
	}
	if got := h.balance(t, h.student.ID); got != "60000.00" {
		t.Errorf("balance = %s, want 60000.00", got)
	}
	entry, err := repository.NewWalletRepository(h.pool).FindBySource(ctx, h.pool, h.tenant.ID, model.WalletSourceTopup, topup.ID.String())
	if err != nil || h.ledger(t, h.student.ID) != 1 {
		t.Fatalf("ledger entry of the request: %v, entries %d, want exactly 1", err, h.ledger(t, h.student.ID))
	}
	if entry.Kind != model.KindTopup || entry.Amount != "60000.00" || entry.BalanceAfter != "60000.00" ||
		entry.ActorKind != model.ActorStaff || entry.ActorID == nil || *entry.ActorID != h.operator.ID ||
		entry.SourceType != model.WalletSourceTopup || entry.SourceID != topup.ID.String() {
		t.Errorf("entry = %+v, want a staff topup entry sourced by the request", entry)
	}
	if n := h.notifications(t, TemplateTopupApproved); n != 2 {
		t.Errorf("approval notifications = %d, want email and WhatsApp", n)
	}

	for i := 0; i < 2; i++ {
		again, err := h.svc.Approve(ctx, scope, h.principal(h.tenant, h.admin), topup.ID)
		if err != nil || again.Status != model.TopupApproved || *again.ReviewedBy != h.operator.ID {
			t.Fatalf("repeated approve = %+v (err %v), want the first review untouched", again, err)
		}
	}
	if h.balance(t, h.student.ID) != "60000.00" || h.ledger(t, h.student.ID) != 1 || h.notifications(t, TemplateTopupApproved) != 2 {
		t.Errorf("a repeated approve changed money or notifications: balance %s ledger %d", h.balance(t, h.student.ID), h.ledger(t, h.student.ID))
	}
	if _, err := h.svc.Reject(ctx, scope, h.principal(h.tenant, h.admin), topup.ID, "terlambat"); !errors.Is(err, ErrTopupState) {
		t.Errorf("reject after approve: %v, want ErrTopupState", err)
	}
	if _, err := h.svc.Approve(ctx, scope, p, topup.ID); !errors.Is(err, ErrTopupForbidden) {
		t.Errorf("student approving: %v, want ErrTopupForbidden", err)
	}
}

func TestTopupRejectNeedsReasonAndCreditsNothing(t *testing.T) {
	h := newTopupHarness(t, noJobs{})
	p := h.principal(h.tenant, h.student)
	ctx, scope, admin := context.Background(), h.scope(h.tenant), h.principal(h.tenant, h.admin)
	topup := h.request(t, p, "30000")

	for _, reason := range []string{"", "   ", string(make([]rune, 501))} {
		if _, err := h.svc.Reject(ctx, scope, admin, topup.ID, reason); err == nil {
			t.Errorf("reject with reason of %d characters succeeded", len(reason))
		} else {
			wantInputField(t, err, "reason")
		}
	}
	if got := h.count(t, `SELECT count(*) FROM topup_requests WHERE id = $1 AND status = 'pending'`, topup.ID); got != 1 {
		t.Fatal("an invalid rejection moved the request")
	}

	rejected, err := h.svc.Reject(ctx, scope, admin, topup.ID, "  Nominal tidak sesuai  ")
	if err != nil || rejected.Status != model.TopupRejected || rejected.RejectReason != "Nominal tidak sesuai" || rejected.ReviewedBy == nil {
		t.Fatalf("reject = %+v (err %v), want a rejected request with the trimmed reason", rejected, err)
	}
	if h.balance(t, h.student.ID) != "0.00" || h.ledger(t, h.student.ID) != 0 {
		t.Error("a rejection moved money")
	}
	if n := h.notifications(t, TemplateTopupRejected); n != 2 {
		t.Errorf("rejection notifications = %d, want email and WhatsApp", n)
	}
	again, err := h.svc.Reject(ctx, scope, admin, topup.ID, "alasan lain")
	if err != nil || again.RejectReason != "Nominal tidak sesuai" {
		t.Errorf("repeated reject = %+v (err %v), want the first reason kept", again, err)
	}
	if _, err := h.svc.Approve(ctx, scope, admin, topup.ID); !errors.Is(err, ErrTopupState) {
		t.Errorf("approve after reject: %v, want ErrTopupState", err)
	}
	if h.balance(t, h.student.ID) != "0.00" {
		t.Error("an approval after a rejection credited the wallet")
	}
}

func TestTopupConcurrentApprovalsCreditOnce(t *testing.T) {
	h := newTopupHarness(t, noJobs{})
	topup := h.request(t, h.principal(h.tenant, h.student), "100000")
	staff := []model.Principal{h.principal(h.tenant, h.admin), h.principal(h.tenant, h.operator)}

	var wg sync.WaitGroup
	errs := make([]error, 8)
	for i := range errs {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, errs[i] = h.svc.Approve(context.Background(), h.scope(h.tenant), staff[i%2], topup.ID)
		}()
	}
	wg.Wait()
	for i, err := range errs {
		if err != nil {
			t.Errorf("approval %d: %v, want success or a no-op replay", i, err)
		}
	}
	if got := h.balance(t, h.student.ID); got != "100000.00" {
		t.Errorf("balance = %s, want exactly one credit of 100000.00", got)
	}
	chain := h.chain(t, h.student.ID)
	if len(chain) != 1 || chain[0].BalanceAfter != "100000.00" {
		t.Errorf("ledger = %+v, want one entry whose balance matches the projection", chain)
	}
	if n := h.notifications(t, TemplateTopupApproved); n != 2 {
		t.Errorf("approval notifications = %d, want 2 from the single credit", n)
	}
}

func TestTopupApproveRollsBackWhenNotificationFails(t *testing.T) {
	h := newTopupHarness(t, failingJobs{})
	topup := h.request(t, h.principal(h.tenant, h.student), "40000")

	if _, err := h.svc.Approve(context.Background(), h.scope(h.tenant), h.principal(h.tenant, h.admin), topup.ID); err == nil {
		t.Fatal("approve succeeded although the notification could not be queued")
	}
	if h.balance(t, h.student.ID) != "0.00" || h.ledger(t, h.student.ID) != 0 {
		t.Error("a failed approval left a credit behind")
	}
	if n := h.count(t, `SELECT count(*) FROM topup_requests WHERE id = $1 AND status = 'pending'`, topup.ID); n != 1 {
		t.Error("a failed approval left the request non-pending")
	}
}

func TestTopupLedgerChainStaysConsistentAcrossApprovals(t *testing.T) {
	h := newTopupHarness(t, noJobs{})
	p := h.principal(h.tenant, h.student)
	ctx, scope, admin := context.Background(), h.scope(h.tenant), h.principal(h.tenant, h.admin)
	for _, amount := range []string{"25000", "35000", "15000"} {
		if _, err := h.svc.Approve(ctx, scope, admin, h.request(t, p, amount).ID); err != nil {
			t.Fatalf("approve %s: %v", amount, err)
		}
	}
	chain := h.chain(t, h.student.ID)
	wantAfter := []string{"25000.00", "60000.00", "75000.00"}
	if len(chain) != len(wantAfter) {
		t.Fatalf("ledger has %d entries, want %d", len(chain), len(wantAfter))
	}
	for i, entry := range chain {
		if entry.BalanceAfter != wantAfter[i] {
			t.Errorf("entry %d balance_after = %s, want %s", i, entry.BalanceAfter, wantAfter[i])
		}
	}
	if h.balance(t, h.student.ID) != "75000.00" {
		t.Errorf("balance = %s, want 75000.00", h.balance(t, h.student.ID))
	}
}

func TestTopupVisibilityAndStaffList(t *testing.T) {
	h := newTopupHarness(t, noJobs{})
	a, b := h.principal(h.tenant, h.student), h.principal(h.tenant, h.second)
	ctx, scope, admin := context.Background(), h.scope(h.tenant), h.principal(h.tenant, h.admin)
	mine := h.request(t, a, "20000")
	theirs := h.request(t, b, "30000")
	if _, err := h.svc.Approve(ctx, scope, admin, theirs.ID); err != nil {
		t.Fatalf("approve: %v", err)
	}

	list, total, minimum, err := h.svc.Topups(ctx, scope, a, 0, 0)
	if err != nil || total != 1 || len(list) != 1 || list[0].ID != mine.ID || minimum != model.DefaultMinTopup {
		t.Errorf("student list = %+v total %d minimum %q err %v, want only their own request and the default minimum", list, total, minimum, err)
	}
	if _, err := h.svc.Topup(ctx, scope, a, theirs.ID); !errors.Is(err, ErrTopupNotFound) {
		t.Errorf("student reading another's request: %v, want ErrTopupNotFound", err)
	}
	if got, err := h.svc.Topup(ctx, scope, admin, theirs.ID); err != nil || got.ID != theirs.ID {
		t.Errorf("staff reading a request: %+v err %v", got, err)
	}
	if _, _, _, err := h.svc.Topups(ctx, scope, admin, 0, 0); !errors.Is(err, ErrTopupForbidden) {
		t.Errorf("staff on the student list: %v, want ErrTopupForbidden", err)
	}

	all, total, _, err := h.svc.StaffTopups(ctx, scope, admin, TopupFilter{})
	if err != nil || total != 2 || len(all) != 2 {
		t.Errorf("staff list total %d err %v, want both requests", total, err)
	}
	pending, total, _, err := h.svc.StaffTopups(ctx, scope, admin, TopupFilter{Status: model.TopupPending})
	if err != nil || total != 1 || pending[0].ID != mine.ID {
		t.Errorf("pending filter = %+v total %d err %v, want only the pending request", pending, total, err)
	}
	if found, total, _, err := h.svc.StaffTopups(ctx, scope, admin, TopupFilter{Search: TopupCode(theirs.Number)}); err != nil || total != 1 || found[0].ID != theirs.ID {
		t.Errorf("search by code = %+v total %d err %v", found, total, err)
	}
	wantInputField(t, func() error {
		_, _, _, err := h.svc.StaffTopups(ctx, scope, admin, TopupFilter{Status: "paid"})
		return err
	}(), "status")
	if _, _, _, err := h.svc.StaffTopups(ctx, scope, a, TopupFilter{}); !errors.Is(err, ErrTopupForbidden) {
		t.Errorf("student on the staff list: %v, want ErrTopupForbidden", err)
	}
}

func TestRupiahFormat(t *testing.T) {
	for in, want := range map[string]string{"1.00": "Rp1", "999.00": "Rp999", "1000.00": "Rp1.000", "50000.00": "Rp50.000", "1234567.00": "Rp1.234.567"} {
		if got := rupiah(in); got != want {
			t.Errorf("rupiah(%q) = %q, want %q", in, got, want)
		}
	}
}
