package service

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/tarunaeduka/bimbel-tryout/api/internal/model"
	"github.com/tarunaeduka/bimbel-tryout/api/internal/repository"
)

type walletHarness struct {
	svc          *WalletService
	repo         *repository.WalletRepository
	pool         *pgxpool.Pool
	tenant       model.Tenant
	other        model.Tenant
	student      model.User
	second       model.User
	otherStudent model.User
	admin        model.User
	operator     model.User
}

func newWalletHarness(t *testing.T) walletHarness {
	t.Helper()
	tenants, pool := testTenantService(t)
	h := walletHarness{
		repo:   repository.NewWalletRepository(pool),
		pool:   pool,
		tenant: provisionTenant(t, tenants, pool),
		other:  provisionTenant(t, tenants, pool),
	}
	h.svc = NewWalletService(pool, h.repo)
	users := repository.NewUserRepository(pool)
	h.student = walletUser(t, users, h.tenant.ID, model.RoleStudent, "Naila Ramadhani")
	h.second = walletUser(t, users, h.tenant.ID, model.RoleStudent, "Budi Santoso")
	h.otherStudent = walletUser(t, users, h.other.ID, model.RoleStudent, "Naila Sekolah B")
	h.admin = walletUser(t, users, h.tenant.ID, model.RoleAdmin, "Siti Aminah")
	h.operator = walletUser(t, users, h.tenant.ID, model.RoleOperator, "Rizky Operator")
	// Registered last, so the ledger rows go before the tenant cleanup deletes their users.
	t.Cleanup(func() { wipeLedger(t, pool, []uuid.UUID{h.tenant.ID, h.other.ID}) })
	return h
}

func walletUser(t *testing.T, users *repository.UserRepository, tenantID uuid.UUID, role model.Role, name string) model.User {
	t.Helper()
	email := "wallet-" + uuid.NewString()[:8] + "@example.com"
	id := uuid.New()
	phone := fmt.Sprintf("+628%09d", binary.BigEndian.Uint32(id[:4])%1000000000)
	user, err := users.Create(context.Background(), model.User{
		TenantID: &tenantID, Name: name, Email: &email, Phone: &phone, PasswordHash: "x", Role: role,
	})
	if err != nil {
		t.Fatalf("create %s user: %v", role, err)
	}
	return user
}

// wipeLedger switches the immutability trigger off for one cleanup connection, never for the table.
func wipeLedger(t *testing.T, pool *pgxpool.Pool, tenantIDs []uuid.UUID) {
	t.Helper()
	ctx := context.Background()
	conn, err := pool.Acquire(ctx)
	if err != nil {
		t.Errorf("acquire cleanup connection: %v", err)
		return
	}
	defer conn.Release()
	if _, err := conn.Exec(ctx, `SET session_replication_role = replica`); err != nil {
		t.Errorf("disable the ledger trigger for cleanup: %v", err)
		return
	}
	defer func() {
		if _, err := conn.Exec(ctx, `RESET session_replication_role`); err != nil {
			t.Errorf("restore the ledger trigger after cleanup: %v", err)
		}
	}()
	if _, err := conn.Exec(ctx, `DELETE FROM wallet_entries WHERE tenant_id = ANY($1)`, tenantIDs); err != nil {
		t.Errorf("cleanup wallet entries: %v", err)
	}
}

func (h walletHarness) scope(tenant model.Tenant) HostScope { return HostScope{Tenant: tenant} }

func (h walletHarness) principal(tenant model.Tenant, user model.User) model.Principal {
	id := tenant.ID
	return model.Principal{UserID: user.ID, TenantID: &id, Name: user.Name, Role: user.Role}
}

// move runs one movement in its own transaction, which is how every caller composes Apply.
func (h walletHarness) move(t *testing.T, m Movement) (model.WalletEntry, bool, error) {
	t.Helper()
	var entry model.WalletEntry
	var replayed bool
	err := h.inTx(func(tx pgx.Tx) error {
		applied, again, err := h.svc.Apply(context.Background(), tx, m)
		entry, replayed = applied, again
		return err
	})
	return entry, replayed, err
}

func (h walletHarness) inTx(fn func(tx pgx.Tx) error) error {
	ctx := context.Background()
	tx, err := h.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := fn(tx); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (h walletHarness) credit(t *testing.T, user model.User, amount string) model.WalletEntry {
	t.Helper()
	entry, _, err := h.move(t, Movement{
		TenantID: h.tenant.ID, UserID: user.ID, Amount: amount, Kind: model.KindTopup,
		ActorKind: model.ActorProvider, SourceType: "topup", SourceID: uuid.NewString(),
	})
	if err != nil {
		t.Fatalf("credit %s: %v", amount, err)
	}
	return entry
}

func (h walletHarness) balance(t *testing.T, userID uuid.UUID) string {
	t.Helper()
	var balance string
	if err := h.pool.QueryRow(context.Background(),
		`SELECT balance::text FROM users WHERE id = $1`, userID).Scan(&balance); err != nil {
		t.Fatalf("read balance: %v", err)
	}
	return balance
}

func (h walletHarness) chain(t *testing.T, userID uuid.UUID) []model.WalletEntry {
	t.Helper()
	rows, err := h.pool.Query(context.Background(), `
		SELECT id, seq, user_id, amount::text, balance_after::text FROM wallet_entries
		 WHERE user_id = $1 ORDER BY seq`, userID)
	if err != nil {
		t.Fatalf("read the ledger: %v", err)
	}
	defer rows.Close()
	var out []model.WalletEntry
	for rows.Next() {
		var entry model.WalletEntry
		if err := rows.Scan(&entry.ID, &entry.Seq, &entry.UserID, &entry.Amount, &entry.BalanceAfter); err != nil {
			t.Fatalf("scan the ledger: %v", err)
		}
		out = append(out, entry)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("read the ledger: %v", err)
	}
	return out
}

func cents(t *testing.T, amount string) int64 {
	t.Helper()
	negative := strings.HasPrefix(amount, "-")
	whole, fraction, _ := strings.Cut(strings.TrimPrefix(amount, "-"), ".")
	var units, rest int64
	if _, err := fmt.Sscanf(whole, "%d", &units); err != nil {
		t.Fatalf("cents of %q: %v", amount, err)
	}
	if fraction != "" {
		if _, err := fmt.Sscanf((fraction + "00")[:2], "%d", &rest); err != nil {
			t.Fatalf("cents of %q: %v", amount, err)
		}
	}
	out := units*100 + rest
	if negative {
		return -out
	}
	return out
}

func canonical(c int64) string {
	sign := ""
	if c < 0 {
		sign, c = "-", -c
	}
	return fmt.Sprintf("%s%d.%02d", sign, c/100, c%100)
}

func TestAmountGrammar(t *testing.T) {
	accepted := map[string]string{
		"0.01": "0.01", "1": "1.00", "1.5": "1.50", "99999.99": "99999.99",
		"9999999999.99": "9999999999.99", "5": "5.00", "5.5": "5.50",
		"-0.01": "-0.01", "-1": "-1.00", "-25000": "-25000.00",
	}
	for raw, want := range accepted {
		got, ok := parseAmount(raw)
		if !ok || got != want {
			t.Errorf("parseAmount(%q) = %q, %v; want %q, true", raw, got, ok, want)
		}
	}
	rejected := []string{
		"0", "0.00", "-0", "-0.00", "", " ", " 5", "5 ", "+5", "1e3", "NaN", "Inf", "1.234",
		"1.", ".5", "-.5", "1,5", "--5", "-", "0.1.1", "99999999999.99", "10000000000.00",
		"١٢٣", "５", "1 0", "\t5", "5\n", "- 5", "1E3",
	}
	for _, raw := range rejected {
		if got, ok := parseAmount(raw); ok {
			t.Errorf("parseAmount(%q) = %q, want it rejected", raw, got)
		}
	}
}

func TestNoFloatTouchesMoney(t *testing.T) {
	h := newWalletHarness(t)
	user := h.second
	total := int64(0)
	for _, amount := range []string{"0.10", "0.01", "1234567890.12", "9999999999.99"} {
		entry, _, err := h.move(t, Movement{
			TenantID: h.tenant.ID, UserID: user.ID, Amount: amount, Kind: model.KindTopup,
			ActorKind: model.ActorProvider, SourceType: "topup", SourceID: uuid.NewString(),
		})
		if err != nil {
			t.Fatalf("credit %s: %v", amount, err)
		}
		total += cents(t, amount)
		if want := canonical(total); entry.Amount != amount || entry.BalanceAfter != want {
			t.Errorf("credit %s: entry = %s / %s, want %s / %s", amount, entry.Amount, entry.BalanceAfter, amount, want)
		}
		if got := h.balance(t, user.ID); got != canonical(total) {
			t.Fatalf("balance after %s = %s, want %s", amount, got, canonical(total))
		}
	}

	// Ten thousand one-cent credits, one transaction: the sum must land exactly on 100.00.
	const steps = 10000
	var last model.WalletEntry
	err := h.inTx(func(tx pgx.Tx) error {
		for i := 0; i < steps; i++ {
			entry, _, err := h.svc.Apply(context.Background(), tx, Movement{
				TenantID: h.tenant.ID, UserID: user.ID, Amount: "0.01", Kind: model.KindTopup,
				ActorKind: model.ActorProvider, SourceType: "topup", SourceID: uuid.NewString(),
			})
			if err != nil {
				return fmt.Errorf("credit %d: %w", i, err)
			}
			last = entry
		}
		return nil
	})
	if err != nil {
		t.Fatalf("ten thousand one-cent credits: %v", err)
	}
	want := canonical(total + steps)
	if last.BalanceAfter != want {
		t.Errorf("last balance_after = %s, want %s", last.BalanceAfter, want)
	}
	if got := h.balance(t, user.ID); got != want {
		t.Errorf("balance after %d one-cent credits = %s, want %s", steps, got, want)
	}
}

func TestDebitBeyondBalanceIsRejectedNotClamped(t *testing.T) {
	h := newWalletHarness(t)
	user := h.student
	h.credit(t, user, "10.00")
	before := h.chain(t, user.ID)

	_, _, err := h.move(t, Movement{
		TenantID: h.tenant.ID, UserID: user.ID, Amount: "-10.01", Kind: model.KindSpend,
		ActorKind: model.ActorStudent, SourceType: "order", SourceID: uuid.NewString(),
	})
	if !errors.Is(err, ErrInsufficientBalance) {
		t.Fatalf("debit beyond the balance: err = %v, want ErrInsufficientBalance", err)
	}
	if got := h.balance(t, user.ID); got != "10.00" {
		t.Errorf("balance after the refused debit = %s, want the untouched 10.00", got)
	}
	if after := h.chain(t, user.ID); len(after) != len(before) {
		t.Errorf("the refused debit wrote %d rows, want 0", len(after)-len(before))
	}

	entry, _, err := h.move(t, Movement{
		TenantID: h.tenant.ID, UserID: user.ID, Amount: "-10.00", Kind: model.KindSpend,
		ActorKind: model.ActorStudent, SourceType: "order", SourceID: uuid.NewString(),
	})
	if err != nil {
		t.Fatalf("debit to exactly zero: %v", err)
	}
	if entry.BalanceAfter != "0.00" || h.balance(t, user.ID) != "0.00" {
		t.Errorf("debit to exactly zero: balance_after = %s, balance = %s; want 0.00", entry.BalanceAfter, h.balance(t, user.ID))
	}
}

func TestConcurrentMovementsSerialize(t *testing.T) {
	h := newWalletHarness(t)
	user := h.student
	h.credit(t, user, "10.00")

	// Twenty-four of the credits are one retried top-up: a burst that must move money exactly once.
	const credits, debits, retries = 30, 25, 24
	retriedKey := "retry-" + uuid.NewString()[:8]
	// A start barrier puts every caller at its replay check at nearly the same moment.
	start := make(chan struct{})
	var wg sync.WaitGroup
	var mu sync.Mutex
	fresh, replays, rejected, failures := 0, 0, 0, []error{}
	retried := map[uuid.UUID]bool{}
	record := func(entry model.WalletEntry, replayed bool, err error) {
		mu.Lock()
		defer mu.Unlock()
		switch {
		case err == nil && replayed:
			replays++
		case err == nil:
			fresh++
		case errors.Is(err, ErrInsufficientBalance):
			rejected++
		default:
			failures = append(failures, err)
		}
	}
	for i := 0; i < credits; i++ {
		key := uuid.NewString()
		shared := i < retries
		if shared {
			key = retriedKey
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			entry, replayed, err := h.move(t, Movement{
				TenantID: h.tenant.ID, UserID: user.ID, Amount: "10.00", Kind: model.KindTopup,
				ActorKind: model.ActorProvider, SourceType: "topup", SourceID: key,
			})
			record(entry, replayed, err)
			if shared && err == nil {
				mu.Lock()
				retried[entry.ID] = true
				mu.Unlock()
			}
		}()
	}
	for i := 0; i < debits; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			entry, replayed, err := h.move(t, Movement{
				TenantID: h.tenant.ID, UserID: user.ID, Amount: "-100.00", Kind: model.KindSpend,
				ActorKind: model.ActorStudent, SourceType: "order", SourceID: uuid.NewString(),
			})
			record(entry, replayed, err)
		}()
	}
	close(start)
	wg.Wait()

	for _, err := range failures {
		t.Errorf("movement: %v", err)
	}
	if rejected == 0 {
		t.Errorf("no debit was refused; the mix did not exercise the guard")
	}
	if replays != retries-1 {
		t.Errorf("%d replays for %d calls on one key, want %d", replays, retries, retries-1)
	}
	if len(retried) != 1 {
		t.Errorf("the retried key produced %d different entries, want 1", len(retried))
	}

	chain := h.chain(t, user.ID)
	if len(chain) != fresh+1 {
		t.Errorf("%d ledger rows for %d fresh movements, want one row each plus the opening credit", len(chain), fresh)
	}
	running := int64(0)
	for i, entry := range chain {
		running += cents(t, entry.Amount)
		if entry.BalanceAfter != canonical(running) {
			t.Fatalf("entry %d (seq %d, amount %s) has balance_after %s, want %s",
				i, entry.Seq, entry.Amount, entry.BalanceAfter, canonical(running))
		}
		if running < 0 {
			t.Fatalf("the chain went negative at entry %d", i)
		}
	}
	if got := h.balance(t, user.ID); got != canonical(running) {
		t.Errorf("balance = %s, ledger sum = %s", got, canonical(running))
	}

	// One key, two overlapping transactions: the row lock is what lets the second caller replay.
	overlap := "overlap-" + uuid.NewString()[:8]
	ctx := context.Background()
	first, err := h.pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin the first transaction: %v", err)
	}
	defer func() { _ = first.Rollback(context.Background()) }()
	if _, _, err := h.svc.Apply(ctx, first, Movement{
		TenantID: h.tenant.ID, UserID: user.ID, Amount: "10.00", Kind: model.KindTopup,
		ActorKind: model.ActorProvider, SourceType: "topup", SourceID: overlap,
	}); err != nil {
		t.Fatalf("apply inside the first transaction: %v", err)
	}
	pid := make(chan int, 1)
	second := make(chan error, 1)
	go func() {
		tx, err := h.pool.Begin(context.Background())
		if err != nil {
			second <- err
			return
		}
		defer func() { _ = tx.Rollback(context.Background()) }()
		var backend int
		if err := tx.QueryRow(context.Background(), `SELECT pg_backend_pid()`).Scan(&backend); err != nil {
			second <- err
			return
		}
		pid <- backend
		if _, _, err := h.svc.Apply(context.Background(), tx, Movement{
			TenantID: h.tenant.ID, UserID: user.ID, Amount: "10.00", Kind: model.KindTopup,
			ActorKind: model.ActorProvider, SourceType: "topup", SourceID: overlap,
		}); err != nil {
			second <- err
			return
		}
		second <- tx.Commit(context.Background())
	}()
	waitBlocked(t, h.pool, <-pid)
	if err := first.Commit(ctx); err != nil {
		t.Fatalf("commit the first transaction: %v", err)
	}
	if err := <-second; err != nil {
		t.Fatalf("the overlapping caller: %v", err)
	}
	rows := int64(0)
	if err := h.pool.QueryRow(ctx, `SELECT count(*) FROM wallet_entries WHERE user_id = $1 AND source_id = $2`,
		user.ID, overlap).Scan(&rows); err != nil {
		t.Fatalf("count the overlapping rows: %v", err)
	}
	if rows != 1 {
		t.Errorf("%d rows for one overlapping key, want 1", rows)
	}
	if got, want := h.balance(t, user.ID), canonical(running+1000); got != want {
		t.Errorf("balance after the overlap = %s, want %s (the retry must not move money twice)", got, want)
	}
}

// waitBlocked waits until one backend is waiting on a lock, so the interleaving is deterministic.
func waitBlocked(t *testing.T, pool *pgxpool.Pool, backend int) {
	t.Helper()
	for deadline := time.Now().Add(5 * time.Second); time.Now().Before(deadline); {
		var waiting bool
		if err := pool.QueryRow(context.Background(),
			`SELECT wait_event_type = 'Lock' FROM pg_stat_activity WHERE pid = $1`, backend).Scan(&waiting); err != nil {
			t.Fatalf("read pg_stat_activity: %v", err)
		}
		if waiting {
			return
		}
		runtime.Gosched()
	}
	t.Fatalf("backend %d never waited on a lock", backend)
}

func TestDuplicateSourceCreatesOneEntry(t *testing.T) {
	h := newWalletHarness(t)
	user := h.student
	source := "adjust-" + uuid.NewString()[:8]

	first, replayed, err := h.move(t, Movement{
		TenantID: h.tenant.ID, UserID: user.ID, Amount: "50000.00", Kind: model.KindTopup,
		ActorKind: model.ActorProvider, SourceType: "topup", SourceID: source,
	})
	if err != nil || replayed {
		t.Fatalf("first movement = %s (replayed %v), %v; want a fresh entry", first.ID, replayed, err)
	}
	second, replayed, err := h.move(t, Movement{
		TenantID: h.tenant.ID, UserID: user.ID, Amount: "50000.00", Kind: model.KindTopup,
		ActorKind: model.ActorProvider, SourceType: "topup", SourceID: source,
	})
	if err != nil || !replayed || second.ID != first.ID {
		t.Fatalf("replay = %s (replayed %v), %v; want the original %s", second.ID, replayed, err, first.ID)
	}
	if got := h.chain(t, user.ID); len(got) != 1 {
		t.Fatalf("sequential replay wrote %d rows, want 1", len(got))
	}
	if got := h.balance(t, user.ID); got != "50000.00" {
		t.Errorf("balance after a replay = %s, want 50000.00", got)
	}

	// Twenty callers race on one fresh key: one row, one entry, and no error at all.
	key := "race-" + uuid.NewString()[:8]
	var wg sync.WaitGroup
	var mu sync.Mutex
	entries := map[uuid.UUID]bool{}
	fresh, failure := 0, []error{}
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			entry, replayed, err := h.move(t, Movement{
				TenantID: h.tenant.ID, UserID: user.ID, Amount: "1000.00", Kind: model.KindTopup,
				ActorKind: model.ActorProvider, SourceType: "topup", SourceID: key,
			})
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				failure = append(failure, err)
				return
			}
			if !replayed {
				fresh++
			}
			entries[entry.ID] = true
		}()
	}
	wg.Wait()
	for _, err := range failure {
		t.Errorf("racing duplicate: %v", err)
	}
	if fresh != 1 {
		t.Errorf("%d callers reported a fresh entry, want exactly 1", fresh)
	}
	if len(entries) != 1 {
		t.Errorf("the 20 callers saw %d different entries, want 1", len(entries))
	}
	if got := h.chain(t, user.ID); len(got) != 2 {
		t.Errorf("ledger rows = %d, want the sequential one and the raced one", len(got))
	}
}

func TestSameKeyDifferentBodyConflicts(t *testing.T) {
	h := newWalletHarness(t)
	user := h.student
	key := "conflict-" + uuid.NewString()[:8]
	h.credit(t, user, "10.00")
	if _, _, err := h.move(t, Movement{
		TenantID: h.tenant.ID, UserID: user.ID, Amount: "10.00", Kind: model.KindTopup,
		ActorKind: model.ActorProvider, SourceType: "topup", SourceID: key,
	}); err != nil {
		t.Fatalf("first movement: %v", err)
	}
	before := h.balance(t, user.ID)
	_, _, err := h.move(t, Movement{
		TenantID: h.tenant.ID, UserID: user.ID, Amount: "20.00", Kind: model.KindTopup,
		ActorKind: model.ActorProvider, SourceType: "topup", SourceID: key,
	})
	if !errors.Is(err, ErrWalletConflict) {
		t.Fatalf("same key with another amount: err = %v, want ErrWalletConflict", err)
	}
	if got := h.balance(t, user.ID); got != before {
		t.Errorf("balance = %s, want the untouched %s", got, before)
	}
}

func TestRollbackLeavesProjectionAndLedgerUnchanged(t *testing.T) {
	h := newWalletHarness(t)
	user := h.student
	h.credit(t, user, "25.00")
	before, rows := h.balance(t, user.ID), len(h.chain(t, user.ID))

	ctx := context.Background()
	tx, err := h.pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	if _, _, err := h.svc.Apply(ctx, tx, Movement{
		TenantID: h.tenant.ID, UserID: user.ID, Amount: "10.00", Kind: model.KindTopup,
		ActorKind: model.ActorProvider, SourceType: "topup", SourceID: uuid.NewString(),
	}); err != nil {
		t.Fatalf("apply inside the caller transaction: %v", err)
	}
	if err := tx.Rollback(ctx); err != nil {
		t.Fatalf("rollback: %v", err)
	}

	if got := h.balance(t, user.ID); got != before {
		t.Errorf("balance after rollback = %s, want %s", got, before)
	}
	if got := len(h.chain(t, user.ID)); got != rows {
		t.Errorf("ledger rows after rollback = %d, want %d", got, rows)
	}
}

func TestApplyComposesInsideCallerTransaction(t *testing.T) {
	h := newWalletHarness(t)
	user := h.student
	ctx := context.Background()

	tx, err := h.pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	credit, _, err := h.svc.Apply(ctx, tx, Movement{
		TenantID: h.tenant.ID, UserID: user.ID, Amount: "50.00", Kind: model.KindTopup,
		ActorKind: model.ActorProvider, SourceType: "topup", SourceID: uuid.NewString(),
	})
	if err != nil {
		t.Fatalf("credit inside the caller transaction: %v", err)
	}
	debit, _, err := h.svc.Apply(ctx, tx, Movement{
		TenantID: h.tenant.ID, UserID: user.ID, Amount: "-20.00", Kind: model.KindSpend,
		ActorKind: model.ActorStudent, SourceType: "order", SourceID: uuid.NewString(),
	})
	if err != nil {
		t.Fatalf("debit inside the caller transaction: %v", err)
	}
	if credit.BalanceAfter != "50.00" || debit.BalanceAfter != "30.00" {
		t.Errorf("balance_after = %s then %s, want 50.00 then 30.00", credit.BalanceAfter, debit.BalanceAfter)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatalf("commit: %v", err)
	}
	if got := h.balance(t, user.ID); got != "30.00" {
		t.Errorf("balance = %s, want 30.00", got)
	}
	if got := len(h.chain(t, user.ID)); got != 2 {
		t.Errorf("ledger rows = %d, want 2", got)
	}
}

func TestAdjustmentNeedsReason(t *testing.T) {
	h := newWalletHarness(t)
	user := h.student
	principal := h.principal(h.tenant, h.admin)
	h.credit(t, user, "10.00")
	before, rows := h.balance(t, user.ID), len(h.chain(t, user.ID))

	for _, reason := range []string{"", "   ", "\t", "\n"} {
		_, _, _, err := h.svc.Adjust(context.Background(), h.scope(h.tenant), principal, user.ID, AdjustmentInput{
			Type: "credit", Amount: "5.00", Reason: reason, IdempotencyKey: "reason-" + uuid.NewString()[:8],
		})
		var input *InputError
		if !errors.As(err, &input) || input.Fields["reason"] == "" {
			t.Errorf("reason %q: err = %v, want a 422 on reason", reason, err)
		}
	}
	if got := h.balance(t, user.ID); got != before {
		t.Errorf("balance = %s, want the untouched %s", got, before)
	}
	if got := len(h.chain(t, user.ID)); got != rows {
		t.Errorf("ledger rows = %d, want %d", got, rows)
	}
}

func TestActorKindAndIdentityAreRecorded(t *testing.T) {
	h := newWalletHarness(t)
	user := h.student
	key := "actor-" + uuid.NewString()[:8]
	entry, replayed, _, err := h.svc.Adjust(context.Background(), h.scope(h.tenant),
		h.principal(h.tenant, h.admin), user.ID, AdjustmentInput{
			Type: "credit", Amount: "25000", Reason: "  Hadiah juara kelas  ", IdempotencyKey: key,
		})
	if err != nil || replayed {
		t.Fatalf("adjustment = %s (replayed %v), %v", entry.ID, replayed, err)
	}
	if entry.Kind != model.KindAdjustment || entry.ActorKind != model.ActorStaff ||
		entry.ActorID == nil || *entry.ActorID != h.admin.ID {
		t.Errorf("adjustment recorded %s/%s/%v, want adjustment/staff/%s", entry.Kind, entry.ActorKind, entry.ActorID, h.admin.ID)
	}
	if entry.ActorName == nil || *entry.ActorName != h.admin.Name {
		t.Errorf("actor_name = %v, want %s", entry.ActorName, h.admin.Name)
	}
	if entry.Reason != "Hadiah juara kelas" || entry.SourceType != "adjustment" || entry.SourceID != key {
		t.Errorf("entry = %+v, want the trimmed reason and the key as the source", entry)
	}

	// A migrated balance is explained by an opening row, written by the system without an actor.
	var actorKind string
	var actorID *uuid.UUID
	if err := h.pool.QueryRow(context.Background(), `
		INSERT INTO wallet_entries (tenant_id, user_id, amount, balance_after, kind, actor_kind,
		        reason, source_type, source_id)
		VALUES ($1, $2, 150000.00, 175000.00, 'opening', 'system', '', 'opening', $3)
		RETURNING actor_kind, actor_id`, h.tenant.ID, h.second.ID, h.second.ID.String()).Scan(&actorKind, &actorID); err != nil {
		t.Fatalf("write an opening row: %v", err)
	}
	if actorKind != model.ActorSystem || actorID != nil {
		t.Errorf("opening row = %s / %v, want system with no actor", actorKind, actorID)
	}
}

func TestOnlyStudentsHaveWallets(t *testing.T) {
	h := newWalletHarness(t)
	cases := []struct {
		name     string
		tenantID uuid.UUID
		userID   uuid.UUID
	}{
		{"an admin", h.tenant.ID, h.admin.ID},
		{"an operator", h.tenant.ID, h.operator.ID},
		{"another tenant's student", h.tenant.ID, h.otherStudent.ID},
	}
	for _, tc := range cases {
		_, _, err := h.move(t, Movement{
			TenantID: tc.tenantID, UserID: tc.userID, Amount: "10.00", Kind: model.KindTopup,
			ActorKind: model.ActorProvider, SourceType: "topup", SourceID: uuid.NewString(),
		})
		if !errors.Is(err, ErrWalletNotFound) {
			t.Errorf("%s: err = %v, want ErrWalletNotFound", tc.name, err)
		}
		_, _, _, err = h.svc.Adjust(context.Background(), h.scope(h.tenant),
			h.principal(h.tenant, h.admin), tc.userID, AdjustmentInput{
				Type: "credit", Amount: "10.00", Reason: "Koreksi", IdempotencyKey: "only-" + uuid.NewString()[:8],
			})
		if !errors.Is(err, ErrWalletNotFound) {
			t.Errorf("%s adjustment: err = %v, want ErrWalletNotFound", tc.name, err)
		}
	}
}

func TestStudentsListFilter(t *testing.T) {
	h := newWalletHarness(t)
	users := repository.NewUserRepository(h.pool)
	percent := walletUser(t, users, h.tenant.ID, model.RoleStudent, "Persen % Siswa")
	underscore := walletUser(t, users, h.tenant.ID, model.RoleStudent, "Garis_Bawah Siswa")
	h.credit(t, h.student, "10.00")
	h.credit(t, percent, "20.00")

	principal := h.principal(h.tenant, h.operator)
	list := func(t *testing.T, filter repository.WalletFilter) ([]model.WalletStudent, int) {
		t.Helper()
		students, total, err := h.svc.Students(context.Background(), h.scope(h.tenant), principal, filter)
		if err != nil {
			t.Fatalf("list students: %v", err)
		}
		return students, total
	}

	all, total := list(t, repository.WalletFilter{})
	if total != 4 || len(all) != 4 {
		t.Fatalf("students = %d (total %d), want the tenant's four", len(all), total)
	}
	for _, student := range all {
		if student.ID == h.otherStudent.ID {
			t.Fatal("the list carries another tenant's student")
		}
	}

	if got, total := list(t, repository.WalletFilter{Search: "%"}); total != 1 || got[0].ID != percent.ID {
		t.Errorf("q=%% matched %d students (total %d), want only the literal %% one", len(got), total)
	}
	if got, total := list(t, repository.WalletFilter{Search: "_"}); total != 1 || got[0].ID != underscore.ID {
		t.Errorf("q=_ matched %d students (total %d), want only the literal _ one", len(got), total)
	}
	if _, total := list(t, repository.WalletFilter{Search: "naila sekolah"}); total != 0 {
		t.Errorf("q matched %d students of another tenant, want 0", total)
	}

	page, total := list(t, repository.WalletFilter{Limit: 2, Offset: 0})
	if total != 4 || len(page) != 2 {
		t.Errorf("first page = %d of %d, want 2 of 4", len(page), total)
	}
	second, _ := list(t, repository.WalletFilter{Limit: 2, Offset: 2})
	if len(second) != 2 || page[0].ID == second[0].ID {
		t.Errorf("second page repeats the first: %s and %s", page[0].ID, second[0].ID)
	}

	if _, _, err := h.svc.Students(context.Background(), h.scope(h.tenant), principal,
		repository.WalletFilter{Limit: 101}); err == nil {
		t.Error("limit 101 was accepted, want a 422")
	}
}

func TestListNeverLeaksStaffContactDetails(t *testing.T) {
	h := newWalletHarness(t)
	user := h.student
	key := "leak-" + uuid.NewString()[:8]
	if _, _, _, err := h.svc.Adjust(context.Background(), h.scope(h.tenant),
		h.principal(h.tenant, h.admin), user.ID, AdjustmentInput{
			Type: "credit", Amount: "1000.00", Reason: "Koreksi", IdempotencyKey: key,
		}); err != nil {
		t.Fatalf("adjustment: %v", err)
	}

	entries, _, err := h.svc.Entries(context.Background(), h.scope(h.tenant), h.principal(h.tenant, user), 50, 0)
	if err != nil || len(entries) != 1 {
		t.Fatalf("entries = %d, %v; want the one adjustment", len(entries), err)
	}
	entry := entries[0]
	if entry.ActorName == nil || *entry.ActorName != h.admin.Name {
		t.Fatalf("actor_name = %v, want %s", entry.ActorName, h.admin.Name)
	}
	body, err := json.Marshal(entry)
	if err != nil {
		t.Fatalf("marshal the entry: %v", err)
	}
	for name, secret := range map[string]string{"email": *h.admin.Email, "phone": *h.admin.Phone} {
		if strings.Contains(string(body), secret) {
			t.Errorf("the student's entry carries the staff %s: %s", name, body)
		}
	}
	if strings.Contains(string(body), "password") {
		t.Errorf("the entry carries a password field: %s", body)
	}
}

func TestSameSourceForTwoStudentsMovesMoneyForOnlyOne(t *testing.T) {
	h := newWalletHarness(t)
	ctx := context.Background()
	other := walletUser(t, repository.NewUserRepository(h.pool), h.tenant.ID, model.RoleStudent, "Raka Pratama")
	source := uuid.NewString()
	movement := func(user model.User) Movement {
		return Movement{
			TenantID: h.tenant.ID, UserID: user.ID, Amount: "10.00", Kind: model.KindTopup,
			ActorKind: model.ActorProvider, SourceType: "topup", SourceID: source,
		}
	}

	first, err := h.pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer func() { _ = first.Rollback(ctx) }()
	if _, _, err := h.svc.Apply(ctx, first, movement(h.student)); err != nil {
		t.Fatalf("first apply: %v", err)
	}

	done := make(chan error, 1)
	go func() {
		done <- h.inTx(func(tx pgx.Tx) error { _, _, err := h.svc.Apply(ctx, tx, movement(other)); return err })
	}()
	waitForLockWaiter(t, h.pool)
	if err := first.Commit(ctx); err != nil {
		t.Fatalf("commit first: %v", err)
	}

	if err := <-done; !errors.Is(err, ErrWalletConflict) {
		t.Fatalf("second student with the same source: err = %v, want ErrWalletConflict", err)
	}
	if got := h.balance(t, other.ID); got != "0.00" {
		t.Errorf("second student balance = %s, want 0.00: the conflicting movement must roll back", got)
	}
	if got := h.balance(t, h.student.ID); got != "10.00" {
		t.Errorf("first student balance = %s, want 10.00", got)
	}
}

func waitForLockWaiter(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		var waiting int
		if err := pool.QueryRow(context.Background(), `
			SELECT count(*) FROM pg_stat_activity
			 WHERE datname = current_database()
			   AND wait_event_type = 'Lock'
			   AND query LIKE '%INSERT INTO wallet_entries%'`).Scan(&waiting); err != nil {
			t.Fatalf("read pg_stat_activity: %v", err)
		}
		if waiting > 0 {
			return
		}
	}
	t.Fatal("no insert ever waited on the first transaction's source row")
}
