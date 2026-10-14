package infra

import (
	"context"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"
)

// smokeArgs proves the pinned River modules compile and run against the migrated river schema.
type smokeArgs struct {
	NotificationID string `json:"notification_id"`
}

func (smokeArgs) Kind() string { return "notification" }

func testPool(t *testing.T) *pgxpool.Pool {
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
	pool := testPool(t)
	ctx := context.Background()

	client, err := river.NewClient[pgx.Tx](riverpgxv5.New(pool), &river.Config{Schema: "river"})
	if err != nil {
		t.Fatalf("new river client: %v", err)
	}

	// Per-run identities keep both the counting and the cleanup off every other package's jobs.
	keep := "smoke-" + uuid.NewString()
	drop := "smoke-" + uuid.NewString()

	countJobs := func(id string) int {
		t.Helper()
		var n int
		if err := pool.QueryRow(ctx, `SELECT count(*) FROM river.river_job WHERE args->>'notification_id' = $1`, id).Scan(&n); err != nil {
			t.Fatalf("count jobs: %v", err)
		}
		return n
	}
	var keptJobID int64
	t.Cleanup(func() { _, _ = pool.Exec(ctx, `DELETE FROM river.river_job WHERE id = $1`, keptJobID) })

	// A committed transaction keeps the job.
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	res, err := client.InsertTx(ctx, tx, smokeArgs{NotificationID: keep}, &river.InsertOpts{Queue: "notifications", MaxAttempts: 6})
	if err != nil {
		t.Fatalf("insert in tx: %v", err)
	}
	keptJobID = res.Job.ID
	if err := tx.Commit(ctx); err != nil {
		t.Fatalf("commit: %v", err)
	}
	if got := countJobs(keep); got != 1 {
		t.Fatalf("my jobs after commit = %d, want 1", got)
	}

	// A rolled-back transaction leaves nothing behind.
	tx, err = pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	if _, err := client.InsertTx(ctx, tx, smokeArgs{NotificationID: drop}, &river.InsertOpts{Queue: "notifications", MaxAttempts: 6}); err != nil {
		t.Fatalf("insert in tx: %v", err)
	}
	if err := tx.Rollback(ctx); err != nil {
		t.Fatalf("rollback: %v", err)
	}
	if got := countJobs(keep); got != 1 {
		t.Fatalf("my jobs after rollback = %d, want 1", got)
	}
	if got := countJobs(drop); got != 0 {
		t.Errorf("rolled-back insert left %d jobs, want 0", got)
	}
}
