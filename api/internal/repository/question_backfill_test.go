package repository

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/google/uuid"

	"github.com/tarunaeduka/bimbel-tryout/api/internal/model"
)

func plainConvert(l LegacyVersion) (VersionDocs, error) {
	return VersionDocs{Content: model.DocFromLegacyText(l.Content), Explanation: model.EmptyDoc(), Options: []model.QuestionOption{}}, nil
}

func TestBackfillDocsDoesNotWaitForTheLockAndAlwaysReleasesIt(t *testing.T) {
	pool := testPool(t)
	repo := NewQuestionRepository(pool)
	ctx := context.Background()

	holder, err := pool.Acquire(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := holder.Exec(ctx, `SELECT pg_advisory_lock($1)`, backfillLockKey); err != nil {
		t.Fatal(err)
	}
	released := false
	release := func() {
		if !released {
			released = true
			_, _ = holder.Exec(ctx, `SELECT pg_advisory_unlock($1)`, backfillLockKey)
			holder.Release()
		}
	}
	defer release()

	result, err := repo.BackfillDocs(ctx, 10, plainConvert)
	if err != nil || result.Locked || result.Converted != 0 {
		t.Fatalf("backfill while another instance holds the lock = %+v, %v; want it skipped without error", result, err)
	}
	release()

	failing := func(LegacyVersion) (VersionDocs, error) { return VersionDocs{}, errors.New("cannot convert") }
	for i := 0; i < 2; i++ {
		result, err := repo.BackfillDocs(ctx, 10, failing)
		if err != nil || !result.Locked {
			t.Fatalf("backfill run %d = %+v, %v; want the lock taken", i, result, err)
		}
	}
	probe, err := pool.Acquire(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer probe.Release()
	var free bool
	if err := probe.QueryRow(ctx, `SELECT pg_try_advisory_lock($1)`, backfillLockKey).Scan(&free); err != nil || !free {
		t.Fatalf("lock after the runs: free = %v, err = %v; want it released", free, err)
	}
	_, _ = probe.Exec(ctx, `SELECT pg_advisory_unlock($1)`, backfillLockKey)
}

func TestBackfillDocsPagesPastSkippedRows(t *testing.T) {
	f := newQuestionFixture(t)
	ctx := context.Background()
	pool := f.repo.db
	var ids []uuid.UUID
	for i := 0; i < 5; i++ {
		var questionID uuid.UUID
		if err := pool.QueryRow(ctx, `INSERT INTO questions (tenant_id, current_version, created_by) VALUES ($1, 1, $2) RETURNING id`, f.tenant.ID, f.author.ID).Scan(&questionID); err != nil {
			t.Fatal(err)
		}
		var id uuid.UUID
		if err := pool.QueryRow(ctx,
			`INSERT INTO question_versions (question_id, tenant_id, version, program_id, subject_id, type, content, difficulty, explanation, options, correct, created_by)
			 VALUES ($1, $2, 1, $3, $4, 'pg', $5, 'easy', '', '[]', '[]', $6) RETURNING id`,
			questionID, f.tenant.ID, f.program.ID, f.subject.ID, fmt.Sprintf("soal %d", i), f.author.ID).Scan(&id); err != nil {
			t.Fatal(err)
		}
		ids = append(ids, id)
	}
	refuse := map[uuid.UUID]bool{ids[0]: true, ids[1]: true}
	convert := func(l LegacyVersion) (VersionDocs, error) {
		if refuse[l.ID] {
			return VersionDocs{}, errors.New("refused")
		}
		return plainConvert(l)
	}

	result, err := f.repo.BackfillDocs(ctx, 2, convert)
	if err != nil || result.Converted < 3 || len(result.Skipped) < 2 {
		t.Fatalf("backfill = %+v, %v; want at least the 3 good rows converted and the 2 refused ones skipped", result, err)
	}
	var pending int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM question_versions WHERE tenant_id = $1 AND content_doc IS NULL`, f.tenant.ID).Scan(&pending); err != nil || pending != 2 {
		t.Errorf("rows still without a document = %d (err %v), want only the 2 refused", pending, err)
	}
}
