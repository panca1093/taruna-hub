package infra

import (
	"context"
	"os"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"
)

// smokeArgs exists only to prove the pinned River modules still compile and run against the
// migrated `river` schema that golang-migrate owns.
type smokeArgs struct {
	NotificationID string `json:"notification_id"`
}

func (smokeArgs) Kind() string { return "notification" }

func smokePool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL not set")
	}
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool
}

func TestRiverTransactionalInsertSmoke(t *testing.T) {
	pool := smokePool(t)
	ctx := context.Background()

	client, err := river.NewClient[pgx.Tx](riverpgxv5.New(pool), &river.Config{Schema: "river"})
	if err != nil {
		t.Fatalf("new river client: %v", err)
	}

	countJobs := func() int {
		t.Helper()
		var n int
		if err := pool.QueryRow(ctx, `SELECT count(*) FROM river.river_job WHERE kind = 'notification'`).Scan(&n); err != nil {
			t.Fatalf("count jobs: %v", err)
		}
		return n
	}
	before := countJobs()
	t.Cleanup(func() { _, _ = pool.Exec(ctx, `DELETE FROM river.river_job WHERE kind = 'notification'`) })

	// A committed transaction keeps the job.
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	if _, err := client.InsertTx(ctx, tx, smokeArgs{NotificationID: "keep"}, &river.InsertOpts{Queue: "notifications", MaxAttempts: 6}); err != nil {
		t.Fatalf("insert in tx: %v", err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatalf("commit: %v", err)
	}
	if got := countJobs(); got != before+1 {
		t.Fatalf("jobs after commit = %d, want %d", got, before+1)
	}

	// A rolled-back transaction leaves nothing behind.
	tx, err = pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	if _, err := client.InsertTx(ctx, tx, smokeArgs{NotificationID: "drop"}, &river.InsertOpts{Queue: "notifications", MaxAttempts: 6}); err != nil {
		t.Fatalf("insert in tx: %v", err)
	}
	if err := tx.Rollback(ctx); err != nil {
		t.Fatalf("rollback: %v", err)
	}
	if got := countJobs(); got != before+1 {
		t.Fatalf("jobs after rollback = %d, want %d", got, before+1)
	}
}
