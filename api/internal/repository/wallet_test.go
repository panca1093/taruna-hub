package repository

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/tarunaeduka/bimbel-tryout/api/internal/model"
)

// walletFixture owns two tenants, each with students, and one admin for staff-made entries.
type walletFixture struct {
	repo         *WalletRepository
	pool         *pgxpool.Pool
	tenant       model.Tenant
	other        model.Tenant
	student      model.User
	second       model.User
	otherStudent model.User
	admin        model.User
}

func newWalletFixture(t *testing.T) walletFixture {
	t.Helper()
	pool := testPool(t)
	tenants := NewTenantRepository(pool)
	users := NewUserRepository(pool)
	f := walletFixture{
		repo:   NewWalletRepository(pool),
		pool:   pool,
		tenant: newTenant(t, tenants),
		other:  newTenant(t, tenants),
	}
	f.student = newUser(t, users, &f.tenant.ID, model.RoleStudent, "w-"+uuid.NewString()[:8]+"@example.com")
	f.second = newUser(t, users, &f.tenant.ID, model.RoleStudent, "w-"+uuid.NewString()[:8]+"@example.com")
	f.otherStudent = newUser(t, users, &f.other.ID, model.RoleStudent, "w-"+uuid.NewString()[:8]+"@example.com")
	f.admin = newUser(t, users, &f.tenant.ID, model.RoleAdmin, "w-"+uuid.NewString()[:8]+"@example.com")
	// Registered after the tenant cleanups, so the ledger rows go before the users they reference.
	t.Cleanup(func() { deleteLedgerRows(t, pool, []uuid.UUID{f.tenant.ID, f.other.ID}) })
	return f
}

// deleteLedgerRows disables the immutability trigger for one connection only, never for the table.
func deleteLedgerRows(t *testing.T, pool *pgxpool.Pool, tenantIDs []uuid.UUID) {
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

// openingStatement is the migration's opening-row insert, applied here to a migrated balance.
const openingStatement = `
	INSERT INTO wallet_entries (tenant_id, user_id, amount, balance_after, kind, actor_kind, reason,
	        source_type, source_id)
	SELECT u.tenant_id, u.id, u.balance, u.balance, 'opening', 'system', '', 'opening', u.id::text
	FROM users u
	WHERE u.tenant_id = ANY($1)
	  AND u.balance <> 0
	  AND NOT EXISTS (
	      SELECT 1 FROM wallet_entries e
	       WHERE e.tenant_id = u.tenant_id
	         AND e.source_type = 'opening'
	         AND e.source_id = u.id::text)`

func (f walletFixture) setBalance(t *testing.T, userID uuid.UUID, balance string) {
	t.Helper()
	if _, err := f.pool.Exec(context.Background(),
		`UPDATE users SET balance = $2::numeric WHERE id = $1`, userID, balance); err != nil {
		t.Fatalf("set balance: %v", err)
	}
}

func (f walletFixture) insert(t *testing.T, entry model.WalletEntry) {
	t.Helper()
	if _, err := f.pool.Exec(context.Background(), `
		INSERT INTO wallet_entries (tenant_id, user_id, amount, balance_after, kind, actor_kind,
		        actor_id, reason, source_type, source_id)
		VALUES ($1, $2, $3::numeric, $4::numeric, $5, $6, $7, $8, $9, $10)`,
		entry.TenantID, entry.UserID, entry.Amount, entry.BalanceAfter, entry.Kind, entry.ActorKind,
		entry.ActorID, entry.Reason, entry.SourceType, entry.SourceID); err != nil {
		t.Fatalf("insert entry: %v", err)
	}
}

func (f walletFixture) seedEntry(userID uuid.UUID, amount, balanceAfter string) model.WalletEntry {
	return model.WalletEntry{
		TenantID: f.tenant.ID, UserID: userID, Amount: amount, BalanceAfter: balanceAfter,
		Kind: model.KindTopup, ActorKind: model.ActorProvider, SourceType: "topup",
		SourceID: uuid.NewString(),
	}
}

func TestWalletRequiresScope(t *testing.T) {
	pool := testPool(t)
	repo := NewWalletRepository(pool)
	ctx := context.Background()

	if _, err := repo.Wallet(ctx, uuid.Nil, uuid.New()); !errors.Is(err, ErrScopeRequired) {
		t.Errorf("Wallet(Nil): err = %v, want ErrScopeRequired", err)
	}
	if _, _, err := repo.Entries(ctx, uuid.Nil, uuid.New(), 10, 0); !errors.Is(err, ErrScopeRequired) {
		t.Errorf("Entries(Nil): err = %v, want ErrScopeRequired", err)
	}
	if _, err := repo.Student(ctx, uuid.Nil, uuid.New()); !errors.Is(err, ErrScopeRequired) {
		t.Errorf("Student(Nil): err = %v, want ErrScopeRequired", err)
	}
	if _, _, err := repo.Students(ctx, uuid.Nil, WalletFilter{Limit: 10}); !errors.Is(err, ErrScopeRequired) {
		t.Errorf("Students(Nil): err = %v, want ErrScopeRequired", err)
	}
	if _, err := repo.FindBySource(ctx, pool, uuid.Nil, "topup", "x"); !errors.Is(err, ErrScopeRequired) {
		t.Errorf("FindBySource(Nil): err = %v, want ErrScopeRequired", err)
	}

	if err := inCurriculumTx(t, pool, func(tx pgx.Tx) error {
		if _, err := repo.LockStudentTx(ctx, tx, uuid.Nil, uuid.New()); !errors.Is(err, ErrScopeRequired) {
			t.Errorf("LockStudentTx(Nil): err = %v, want ErrScopeRequired", err)
		}
		if _, _, err := repo.AdjustBalanceTx(ctx, tx, uuid.Nil, uuid.New(), "1.00"); !errors.Is(err, ErrScopeRequired) {
			t.Errorf("AdjustBalanceTx(Nil): err = %v, want ErrScopeRequired", err)
		}
		if _, _, err := repo.InsertEntryTx(ctx, tx, uuid.Nil, model.WalletEntry{}); !errors.Is(err, ErrScopeRequired) {
			t.Errorf("InsertEntryTx(Nil): err = %v, want ErrScopeRequired", err)
		}
		return nil
	}); err != nil {
		t.Fatalf("scoped-in-tx checks: %v", err)
	}
}

func TestLedgerEntriesAreImmutable(t *testing.T) {
	f := newWalletFixture(t)
	ctx := context.Background()
	f.setBalance(t, f.student.ID, "500.00")
	f.insert(t, f.seedEntry(f.student.ID, "500.00", "500.00"))

	var id uuid.UUID
	if err := f.pool.QueryRow(ctx,
		`SELECT id FROM wallet_entries WHERE user_id = $1`, f.student.ID).Scan(&id); err != nil {
		t.Fatalf("read the entry: %v", err)
	}

	if _, err := f.pool.Exec(ctx,
		`UPDATE wallet_entries SET amount = 1.00 WHERE id = $1`, id); err == nil {
		t.Error("UPDATE on wallet_entries succeeded, want the trigger to refuse it")
	}
	if _, err := f.pool.Exec(ctx, `DELETE FROM wallet_entries WHERE id = $1`, id); err == nil {
		t.Error("DELETE on wallet_entries succeeded, want the trigger to refuse it")
	}

	var amount, balanceAfter string
	var reason string
	if err := f.pool.QueryRow(ctx,
		`SELECT amount::text, balance_after::text, reason FROM wallet_entries WHERE id = $1`, id).
		Scan(&amount, &balanceAfter, &reason); err != nil {
		t.Fatalf("read the row after the refused changes: %v", err)
	}
	if amount != "500.00" || balanceAfter != "500.00" {
		t.Errorf("row = %s / %s, want the original 500.00 / 500.00", amount, balanceAfter)
	}
}

func TestProjectionCannotGoNegative(t *testing.T) {
	f := newWalletFixture(t)
	if _, err := f.pool.Exec(context.Background(),
		`UPDATE users SET balance = -0.01 WHERE id = $1`, f.student.ID); err == nil {
		t.Fatal("a negative balance was stored, want the CHECK to refuse it")
	}
	var balance string
	if err := f.pool.QueryRow(context.Background(),
		`SELECT balance::text FROM users WHERE id = $1`, f.student.ID).Scan(&balance); err != nil {
		t.Fatalf("read balance: %v", err)
	}
	if balance != "0.00" {
		t.Errorf("balance = %s, want the refusal to leave 0.00", balance)
	}
}

func TestEntryCompositeFKRejectsCrossTenantUser(t *testing.T) {
	f := newWalletFixture(t)
	if _, err := f.pool.Exec(context.Background(), `
		INSERT INTO wallet_entries (tenant_id, user_id, amount, balance_after, kind, actor_kind,
		        reason, source_type, source_id)
		VALUES ($1, $2, 10.00, 10.00, 'topup', 'provider', '', 'topup', $3)`,
		f.tenant.ID, f.otherStudent.ID, uuid.NewString()); err == nil {
		t.Fatal("an entry paired tenant A with a user of tenant B, want the composite FK to refuse it")
	}
}

func TestSourceUniquenessIsIndependentOfAnyCache(t *testing.T) {
	f := newWalletFixture(t)
	f.setBalance(t, f.student.ID, "20.00")
	entry := f.seedEntry(f.student.ID, "10.00", "10.00")
	f.insert(t, entry)

	_, err := f.pool.Exec(context.Background(), `
		INSERT INTO wallet_entries (tenant_id, user_id, amount, balance_after, kind, actor_kind,
		        reason, source_type, source_id)
		VALUES ($1, $2, 10.00, 20.00, 'topup', 'provider', '', 'topup', $3)`,
		f.tenant.ID, f.student.ID, entry.SourceID)
	if err == nil {
		t.Fatal("a second row with the same (tenant, source_type, source_id) was stored")
	}
	var rows int
	if err := f.pool.QueryRow(context.Background(),
		`SELECT count(*) FROM wallet_entries WHERE tenant_id = $1 AND source_type = 'topup' AND source_id = $2`,
		f.tenant.ID, entry.SourceID).Scan(&rows); err != nil {
		t.Fatalf("count rows of the source: %v", err)
	}
	if rows != 1 {
		t.Errorf("rows for one source = %d, want exactly 1", rows)
	}

	// The same source in another tenant is a different financial operation and stays legal.
	if _, err := f.pool.Exec(context.Background(), `
		INSERT INTO wallet_entries (tenant_id, user_id, amount, balance_after, kind, actor_kind,
		        reason, source_type, source_id)
		VALUES ($1, $2, 10.00, 10.00, 'topup', 'provider', '', 'topup', $3)`,
		f.other.ID, f.otherStudent.ID, entry.SourceID); err != nil {
		t.Errorf("the same source in another tenant: %v", err)
	}
}

func TestAdjustmentRowsNeedReasonAndStaffActor(t *testing.T) {
	f := newWalletFixture(t)
	f.setBalance(t, f.student.ID, "100.00")
	source := func() string { return uuid.NewString() }

	cases := []struct {
		name string
		row  model.WalletEntry
	}{
		{"empty reason", model.WalletEntry{Kind: model.KindAdjustment, ActorKind: model.ActorStaff,
			ActorID: &f.admin.ID, Reason: "", SourceType: "adjustment", SourceID: source()}},
		{"blank reason", model.WalletEntry{Kind: model.KindAdjustment, ActorKind: model.ActorStaff,
			ActorID: &f.admin.ID, Reason: "   ", SourceType: "adjustment", SourceID: source()}},
		{"system actor", model.WalletEntry{Kind: model.KindAdjustment, ActorKind: model.ActorSystem,
			Reason: "Koreksi hadiah", SourceType: "adjustment", SourceID: source()}},
		{"no actor", model.WalletEntry{Kind: model.KindAdjustment, ActorKind: model.ActorStaff,
			Reason: "Koreksi hadiah", SourceType: "adjustment", SourceID: source()}},
	}
	for _, tc := range cases {
		row := tc.row
		row.TenantID, row.UserID, row.Amount, row.BalanceAfter = f.tenant.ID, f.student.ID, "10.00", "110.00"
		if _, err := f.pool.Exec(context.Background(), `
			INSERT INTO wallet_entries (tenant_id, user_id, amount, balance_after, kind, actor_kind,
			        actor_id, reason, source_type, source_id)
			VALUES ($1, $2, $3::numeric, $4::numeric, $5, $6, $7, $8, $9, $10)`,
			row.TenantID, row.UserID, row.Amount, row.BalanceAfter, row.Kind, row.ActorKind,
			row.ActorID, row.Reason, row.SourceType, row.SourceID); err == nil {
			t.Errorf("%s: the row was stored, want the CHECK to refuse it", tc.name)
		}
	}

	f.insert(t, model.WalletEntry{
		TenantID: f.tenant.ID, UserID: f.student.ID, Amount: "10.00", BalanceAfter: "110.00",
		Kind: model.KindAdjustment, ActorKind: model.ActorStaff, ActorID: &f.admin.ID,
		Reason: "Koreksi hadiah", SourceType: "adjustment", SourceID: source(),
	})
}

func TestOpeningEntriesExplainMigratedBalances(t *testing.T) {
	f := newWalletFixture(t)
	ctx := context.Background()
	f.setBalance(t, f.student.ID, "75000.00")
	f.setBalance(t, f.second.ID, "1250.50")

	if _, err := f.pool.Exec(ctx, openingStatement, []uuid.UUID{f.tenant.ID, f.other.ID}); err != nil {
		t.Fatalf("apply the opening statement: %v", err)
	}
	// Re-running it is a no-op, which is what makes the seed safe to repeat.
	if _, err := f.pool.Exec(ctx, openingStatement, []uuid.UUID{f.tenant.ID, f.other.ID}); err != nil {
		t.Fatalf("apply the opening statement twice: %v", err)
	}

	rows, err := f.pool.Query(ctx, `
		SELECT u.id, u.balance::text,
		       COALESCE(sum(e.amount), 0)::numeric(14,2)::text,
		       count(e.id) FILTER (WHERE e.kind = 'opening'),
		       count(e.id) FILTER (WHERE e.actor_kind = 'system')
		  FROM users u
		  LEFT JOIN wallet_entries e ON e.user_id = u.id AND e.tenant_id = u.tenant_id
		 WHERE u.tenant_id = ANY($1)
		 GROUP BY u.id, u.balance`, []uuid.UUID{f.tenant.ID, f.other.ID})
	if err != nil {
		t.Fatalf("read balances against the ledger: %v", err)
	}
	defer rows.Close()

	checked := 0
	for rows.Next() {
		var id uuid.UUID
		var balance, sum string
		var openings, systems int
		if err := rows.Scan(&id, &balance, &sum, &openings, &systems); err != nil {
			t.Fatalf("scan: %v", err)
		}
		if balance != sum {
			t.Errorf("user %s: balance %s but the ledger sums to %s", id, balance, sum)
		}
		if balance != "0.00" && (openings != 1 || systems != 1) {
			t.Errorf("user %s: %d opening rows and %d system rows, want exactly one of each", id, openings, systems)
		}
		checked++
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("read balances against the ledger: %v", err)
	}
	if checked != 4 {
		t.Fatalf("checked %d users, want the fixture's four", checked)
	}
}

func TestLedgerCannotBeTruncated(t *testing.T) {
	f := newWalletFixture(t)
	ctx := context.Background()
	f.setBalance(t, f.student.ID, "500.00")
	f.insert(t, f.seedEntry(f.student.ID, "500.00", "500.00"))

	tx, err := f.pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	if _, err := tx.Exec(ctx, `TRUNCATE wallet_entries`); err == nil {
		t.Error("TRUNCATE on wallet_entries succeeded, want the trigger to refuse it")
	}
	_ = tx.Rollback(ctx)
	var rows int
	if err := f.pool.QueryRow(ctx, `SELECT count(*) FROM wallet_entries WHERE user_id = $1`, f.student.ID).Scan(&rows); err != nil {
		t.Fatalf("count entries: %v", err)
	}
	if rows != 1 {
		t.Errorf("entries after the refused TRUNCATE = %d, want 1", rows)
	}
}
