package repository

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/tarunaeduka/bimbel-tryout/api/internal/model"
)

// Cleanup is registered child-first: tryout rows go before questions, curriculum, users and tenants.
type tryoutFixture struct {
	repo         *TryoutRepository
	pool         *pgxpool.Pool
	tenant       model.Tenant
	other        model.Tenant
	author       model.User
	otherBy      model.User
	program      model.Program
	otherProgram model.Program
	subject      model.Subject
	first        model.Question
	second       model.Question
	otherQ       model.Question
}

func newTryoutFixture(t *testing.T) tryoutFixture {
	t.Helper()
	questions := newQuestionFixture(t)
	mine := mustCreateTree(t, NewCurriculumRepository(questions.repo.db), questions.tenant.ID,
		programTree("TSD", "Tryout SD", subjectTree("Matematika", "Bilangan")))

	f := tryoutFixture{
		repo:         NewTryoutRepository(questions.repo.db),
		pool:         questions.repo.db,
		tenant:       questions.tenant,
		other:        questions.other,
		author:       questions.author,
		otherBy:      questions.otherBy,
		program:      mine.Program,
		otherProgram: questions.otherProgram,
		subject:      mine.Subjects[0].Subject,
	}
	t.Cleanup(func() {
		ctx := context.Background()
		ids := []uuid.UUID{f.tenant.ID, f.other.ID}
		for _, sql := range []string{
			`DELETE FROM tryout_questions WHERE tenant_id = ANY($1)`,
			`DELETE FROM tryout_subtests WHERE tenant_id = ANY($1)`,
			`DELETE FROM tryouts WHERE tenant_id = ANY($1)`,
		} {
			if _, err := f.pool.Exec(ctx, sql, ids); err != nil {
				t.Errorf("cleanup tryouts: %v", err)
			}
		}
	})

	f.first = questions.create(t, f.tenant.ID, f.author.ID, samplePG(f.program.ID, f.subject.ID))
	second := samplePG(f.program.ID, f.subject.ID)
	second.Content = "Hasil dari 3 + 3 adalah ..."
	f.second = questions.create(t, f.tenant.ID, f.author.ID, second)
	f.otherQ = questions.create(t, f.other.ID, f.otherBy.ID, samplePG(f.otherProgram.ID, questions.otherSubject.ID))
	return f
}

func (f tryoutFixture) draft(programID uuid.UUID, subtests ...model.Subtest) model.Tryout {
	return model.Tryout{
		ProgramID:        programID,
		Title:            "Tryout " + uuid.NewString()[:8],
		Description:      "Latihan",
		ScoringMode:      model.ScoringWeighted,
		ActiveUntil:      time.Now().Add(48 * time.Hour),
		RandomizeOptions: true,
		Subtests:         subtests,
	}
}

func subtestOf(name string, seconds int, questions ...model.Question) model.Subtest {
	out := model.Subtest{Name: name, DurationSeconds: seconds}
	for _, question := range questions {
		out.Questions = append(out.Questions, model.Assignment{QuestionID: question.ID, VersionID: question.VersionID})
	}
	return out
}

func (f tryoutFixture) create(t *testing.T, tryout model.Tryout) model.Tryout {
	t.Helper()
	created, err := f.repo.Create(context.Background(), f.tenant.ID, f.author.ID, tryout)
	if err != nil {
		t.Fatalf("create tryout: %v", err)
	}
	return created
}

func TestTryoutsRequireScope(t *testing.T) {
	pool := testPool(t)
	repo := NewTryoutRepository(pool)
	ctx := context.Background()

	if _, _, err := repo.List(ctx, uuid.Nil, TryoutFilter{Limit: 10}); !errors.Is(err, ErrScopeRequired) {
		t.Errorf("List(Nil): err = %v, want ErrScopeRequired", err)
	}
	if _, err := repo.Find(ctx, uuid.Nil, uuid.New()); !errors.Is(err, ErrScopeRequired) {
		t.Errorf("Find(Nil): err = %v, want ErrScopeRequired", err)
	}
	if _, err := repo.Create(ctx, uuid.Nil, uuid.New(), model.Tryout{ActiveUntil: time.Now()}); !errors.Is(err, ErrScopeRequired) {
		t.Errorf("Create(Nil): err = %v, want ErrScopeRequired", err)
	}
	if _, err := repo.CurrentQuestions(ctx, pool, uuid.Nil, []uuid.UUID{uuid.New()}); !errors.Is(err, ErrScopeRequired) {
		t.Errorf("CurrentQuestions(Nil): err = %v, want ErrScopeRequired", err)
	}
	if _, err := repo.AssignedVersions(ctx, pool, uuid.Nil, uuid.New()); !errors.Is(err, ErrScopeRequired) {
		t.Errorf("AssignedVersions(Nil): err = %v, want ErrScopeRequired", err)
	}

	inTx := func(fn func(pgx.Tx) error) error { return inCurriculumTx(t, pool, fn) }
	for name, call := range map[string]func(pgx.Tx) error{
		"FindTx":            func(tx pgx.Tx) error { _, err := repo.FindTx(ctx, tx, uuid.Nil, uuid.New()); return err },
		"LockTx":            func(tx pgx.Tx) error { _, err := repo.LockTx(ctx, tx, uuid.Nil, uuid.New()); return err },
		"UpdateHeadTx":      func(tx pgx.Tx) error { return repo.UpdateHeadTx(ctx, tx, uuid.Nil, uuid.New(), model.Tryout{}) },
		"ReplaceChildrenTx": func(tx pgx.Tx) error { return repo.ReplaceChildrenTx(ctx, tx, uuid.Nil, uuid.New(), nil) },
		"SetStatusTx":       func(tx pgx.Tx) error { return repo.SetStatusTx(ctx, tx, uuid.Nil, uuid.New(), model.TryoutPublished) },
		"DeleteTx":          func(tx pgx.Tx) error { return repo.DeleteTx(ctx, tx, uuid.Nil, uuid.New()) },
	} {
		if err := inTx(call); !errors.Is(err, ErrScopeRequired) {
			t.Errorf("%s(Nil): err = %v, want ErrScopeRequired", name, err)
		}
	}
}

func TestTryoutCompositeFKsRejectCrossTenantRefs(t *testing.T) {
	f := newTryoutFixture(t)
	ctx := context.Background()
	created := f.create(t, f.draft(f.program.ID, subtestOf("Subtes 1", 3600, f.first)))
	subtestID := created.Subtests[0].ID

	cases := []struct {
		name string
		sql  string
		args []any
	}{
		{
			name: "program of another tenant",
			sql: `INSERT INTO tryouts (tenant_id, program_id, title, scoring_mode, active_until, created_by)
			      VALUES ($1, $2, 'Silang', 'weighted', now() + interval '1 day', $3)`,
			args: []any{f.tenant.ID, f.otherProgram.ID, f.author.ID},
		},
		{
			name: "subtest of another tenant",
			sql: `INSERT INTO tryout_questions (subtest_id, tryout_id, tenant_id, position, question_id, question_version_id)
			      VALUES ($1, $2, $3, 2, $4, $5)`,
			args: []any{subtestID, created.ID, f.other.ID, f.otherQ.ID, f.otherQ.VersionID},
		},
		{
			name: "question version of another tenant",
			sql: `INSERT INTO tryout_questions (subtest_id, tryout_id, tenant_id, position, question_id, question_version_id)
			      VALUES ($1, $2, $3, 2, $4, $5)`,
			args: []any{subtestID, created.ID, f.tenant.ID, f.otherQ.ID, f.otherQ.VersionID},
		},
	}
	for _, tc := range cases {
		_, err := f.pool.Exec(ctx, tc.sql, tc.args...)
		if !isForeignKeyViolation(err, pgForeignKeyViolation) {
			t.Errorf("%s: err = %v, want SQLSTATE %s", tc.name, err, pgForeignKeyViolation)
		}
	}

	var tryouts, assignments int
	if err := f.pool.QueryRow(ctx, `SELECT count(*) FROM tryouts WHERE tenant_id = $1`, f.tenant.ID).Scan(&tryouts); err != nil {
		t.Fatalf("count tryouts: %v", err)
	}
	if err := f.pool.QueryRow(ctx, `SELECT count(*) FROM tryout_questions WHERE tryout_id = $1`, created.ID).Scan(&assignments); err != nil {
		t.Fatalf("count assignments: %v", err)
	}
	if tryouts != 1 || assignments != 1 {
		t.Errorf("tryouts = %d assignments = %d, want 1 and 1: every rejected insert must have rolled back", tryouts, assignments)
	}
}

func TestAssignmentVersionMustBelongToItsQuestion(t *testing.T) {
	f := newTryoutFixture(t)
	ctx := context.Background()
	created := f.create(t, f.draft(f.program.ID, subtestOf("Subtes 1", 3600, f.first)))
	subtestID := created.Subtests[0].ID
	third := f.questionOrCreate(t)

	_, err := f.pool.Exec(ctx, `
		INSERT INTO tryout_questions (subtest_id, tryout_id, tenant_id, position, question_id, question_version_id)
		VALUES ($1, $2, $3, 2, $4, $5)`,
		subtestID, created.ID, f.tenant.ID, third.ID, f.second.VersionID)
	if !isForeignKeyViolation(err, pgForeignKeyViolation) {
		t.Fatalf("assign another question's version: err = %v, want SQLSTATE %s", err, pgForeignKeyViolation)
	}

	reloaded, err := f.repo.Find(ctx, f.tenant.ID, created.ID)
	if err != nil {
		t.Fatalf("find: %v", err)
	}
	if got := reloaded.QuestionCount; got != 1 {
		t.Errorf("question count = %d after the rejected insert, want 1", got)
	}
	if got := reloaded.Subtests[0].Questions[0].VersionID; got != f.first.VersionID {
		t.Errorf("assignment version = %s, want the first question's own %s", got, f.first.VersionID)
	}
}

func TestQuestionTwiceInOneTryoutIsRejected(t *testing.T) {
	f := newTryoutFixture(t)
	ctx := context.Background()
	in := f.draft(f.program.ID,
		subtestOf("Subtes 1", 3600, f.first),
		subtestOf("Subtes 2", 3600, f.first),
	)

	_, err := f.repo.Create(ctx, f.tenant.ID, f.author.ID, in)
	if !isForeignKeyViolation(err, pgUniqueViolation) {
		t.Fatalf("assign the same question twice: err = %v, want SQLSTATE %s", err, pgUniqueViolation)
	}

	var tryouts, subtests int
	if err := f.pool.QueryRow(ctx, `SELECT count(*) FROM tryouts WHERE tenant_id = $1`, f.tenant.ID).Scan(&tryouts); err != nil {
		t.Fatalf("count tryouts: %v", err)
	}
	if err := f.pool.QueryRow(ctx, `SELECT count(*) FROM tryout_subtests WHERE tenant_id = $1`, f.tenant.ID).Scan(&subtests); err != nil {
		t.Fatalf("count subtests: %v", err)
	}
	if tryouts != 0 || subtests != 0 {
		t.Errorf("tryouts = %d subtests = %d after the rejected create, want nothing written", tryouts, subtests)
	}
}

func TestPositionsReorderWithinOneTransaction(t *testing.T) {
	f := newTryoutFixture(t)
	ctx := context.Background()
	created := f.create(t, f.draft(f.program.ID,
		subtestOf("Pertama", 3600, f.first, f.second),
		subtestOf("Kedua", 1800, f.questionOrCreate(t)),
	))
	firstID, secondID := created.Subtests[0].ID, created.Subtests[1].ID

	err := inCurriculumTx(t, f.pool, func(tx pgx.Tx) error {
		// The deferred unique keys are what allow a plain swap inside one transaction.
		if _, err := tx.Exec(ctx,
			`UPDATE tryout_subtests SET position = 3 - position WHERE tenant_id = $1 AND tryout_id = $2 AND position IN (1, 2)`,
			f.tenant.ID, created.ID); err != nil {
			return err
		}
		_, err := tx.Exec(ctx,
			`UPDATE tryout_questions SET position = 3 - position WHERE tenant_id = $1 AND subtest_id = $2 AND position IN (1, 2)`,
			f.tenant.ID, firstID)
		return err
	})
	if err != nil {
		t.Fatalf("swap positions: %v", err)
	}

	reloaded, err := f.repo.Find(ctx, f.tenant.ID, created.ID)
	if err != nil {
		t.Fatalf("find: %v", err)
	}
	if reloaded.Subtests[0].ID != secondID || reloaded.Subtests[0].Name != "Kedua" {
		t.Errorf("first subtest = %s %q, want the swapped Kedua", reloaded.Subtests[0].ID, reloaded.Subtests[0].Name)
	}
	if reloaded.Subtests[1].ID != firstID {
		t.Errorf("second subtest = %s, want the swap to move Pertama down", reloaded.Subtests[1].ID)
	}
	if got := reloaded.Subtests[1].Questions[0].QuestionID; got != f.second.ID {
		t.Errorf("first question = %s, want the swapped %s", got, f.second.ID)
	}
	if got := reloaded.Subtests[1].Questions[1].QuestionID; got != f.first.ID {
		t.Errorf("second question = %s, want %s", got, f.first.ID)
	}
}

// questionOrCreate is a third question, so a subtest can hold one without repeating f.first or f.second.
func (f tryoutFixture) questionOrCreate(t *testing.T) model.Question {
	t.Helper()
	question := samplePG(f.program.ID, f.subject.ID)
	question.Content = "Hasil dari 4 + 4 adalah ..."
	created, err := NewQuestionRepository(f.pool).Create(context.Background(), f.tenant.ID, f.author.ID, question)
	if err != nil {
		t.Fatalf("create question: %v", err)
	}
	return created
}

func TestDeleteTryoutDeletesChildrenAtomically(t *testing.T) {
	f := newTryoutFixture(t)
	ctx := context.Background()
	third := f.questionOrCreate(t)
	created := f.create(t, f.draft(f.program.ID,
		subtestOf("Subtes 1", 3600, f.first, f.second),
		subtestOf("Subtes 2", 1800, third),
	))

	if err := inCurriculumTx(t, f.pool, func(tx pgx.Tx) error {
		locked, err := f.repo.LockTx(ctx, tx, f.tenant.ID, created.ID)
		if err != nil {
			return err
		}
		if locked.Locked {
			t.Fatal("a fresh tryout must not be locked")
		}
		return f.repo.DeleteTx(ctx, tx, f.tenant.ID, created.ID)
	}); err != nil {
		t.Fatalf("delete tryout: %v", err)
	}

	for table, idColumn := range map[string]string{
		"tryouts":          "id",
		"tryout_subtests":  "tryout_id",
		"tryout_questions": "tryout_id",
	} {
		var count int
		if err := f.pool.QueryRow(ctx, `SELECT count(*) FROM `+table+` WHERE `+idColumn+` = $1`, created.ID).Scan(&count); err != nil {
			t.Fatalf("count %s: %v", table, err)
		}
		if count != 0 {
			t.Errorf("%s still holds %d rows of the deleted tryout", table, count)
		}
	}
	if _, err := f.repo.Find(ctx, f.tenant.ID, created.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("find after delete: err = %v, want ErrNotFound", err)
	}
}

func TestAssignmentTryoutMustMatchItsSubtest(t *testing.T) {
	f := newTryoutFixture(t)
	ctx := context.Background()
	one := f.create(t, f.draft(f.program.ID, subtestOf("Subtes 1", 3600, f.first)))
	other := f.create(t, f.draft(f.program.ID, subtestOf("Subtes 2", 3600, f.second)))
	third := f.questionOrCreate(t)

	_, err := f.pool.Exec(ctx, `
		INSERT INTO tryout_questions (subtest_id, tryout_id, tenant_id, position, question_id, question_version_id)
		VALUES ($1, $2, $3, 2, $4, $5)`,
		one.Subtests[0].ID, other.ID, f.tenant.ID, third.ID, third.VersionID)
	if !isForeignKeyViolation(err, pgForeignKeyViolation) {
		t.Fatalf("assignment naming another tryout than its subtest's: err = %v, want SQLSTATE %s", err, pgForeignKeyViolation)
	}
}
