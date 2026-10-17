package repository

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/tarunaeduka/bimbel-tryout/api/internal/model"
)

// Cleanup is registered child-first: questions go before users and curriculum.
type questionFixture struct {
	repo         *QuestionRepository
	tenant       model.Tenant
	other        model.Tenant
	author       model.User
	otherBy      model.User
	program      model.Program
	subject      model.Subject
	chapter      model.Chapter
	otherProgram model.Program
	otherSubject model.Subject
	otherChapter model.Chapter
	question     model.Question
}

func newQuestionFixture(t *testing.T) questionFixture {
	t.Helper()
	pool := testPool(t)
	curriculum := NewCurriculumRepository(pool)
	users := NewUserRepository(pool)

	tenant := newCurriculumTenant(t)
	other := newCurriculumTenant(t)
	created := mustCreateTree(t, curriculum, tenant.ID, programTree("QSD", "Sekolah Dasar", subjectTree("Matematika", "Bilangan")))
	madeOther := mustCreateTree(t, curriculum, other.ID, programTree("QSD", "Sekolah Dasar", subjectTree("Matematika", "Bilangan")))
	author := newUser(t, users, &tenant.ID, model.RoleAdmin, "author-"+uuid.NewString()[:8]+"@example.com")
	otherBy := newUser(t, users, &other.ID, model.RoleAdmin, "author-"+uuid.NewString()[:8]+"@example.com")

	repo := NewQuestionRepository(pool)
	// Registered before the curriculum and user cleanups so it runs after them.
	t.Cleanup(func() {
		ctx := context.Background()
		for _, sql := range []string{
			`DELETE FROM question_versions WHERE tenant_id = ANY($1)`,
			`DELETE FROM questions WHERE tenant_id = ANY($1)`,
		} {
			if _, err := pool.Exec(ctx, sql, []uuid.UUID{tenant.ID, other.ID}); err != nil {
				t.Errorf("cleanup questions: %v", err)
			}
		}
	})

	return questionFixture{
		repo:         repo,
		tenant:       tenant,
		other:        other,
		author:       author,
		otherBy:      otherBy,
		program:      created.Program,
		subject:      created.Subjects[0].Subject,
		chapter:      created.Subjects[0].Chapters[0],
		otherProgram: madeOther.Program,
		otherSubject: madeOther.Subjects[0].Subject,
		otherChapter: madeOther.Subjects[0].Chapters[0],

		question: model.Question{
			ProgramID: madeOther.Program.ID, SubjectID: madeOther.Subjects[0].Subject.ID,
			Type: model.QuestionPG, Content: "Berapa 2 + 2?", Difficulty: model.DifficultyEasy,
			Explanation: "Empat.", Options: []model.QuestionOption{
				{ID: "a", Label: "3"}, {ID: "b", Label: "4"},
			},
			Correct: []string{"b"},
		},
	}
}

func (f questionFixture) create(t *testing.T, tenantID, authorID uuid.UUID, q model.Question) model.Question {
	t.Helper()
	out, err := f.repo.Create(context.Background(), tenantID, authorID, q)
	if err != nil {
		t.Fatalf("create question: %v", err)
	}
	return out
}

func samplePG(programID, subjectID uuid.UUID) model.Question {
	return model.Question{
		ProgramID: programID, SubjectID: subjectID,
		Type: model.QuestionPG, Content: "Hasil dari 2 + 2 adalah ...", Difficulty: model.DifficultyEasy,
		Explanation: "2 + 2 = 4.",
		Options:     []model.QuestionOption{{ID: "a", Label: "3"}, {ID: "b", Label: "4"}},
		Correct:     []string{"b"},
	}
}

func TestQuestionsRequireScope(t *testing.T) {
	pool := testPool(t)
	repo := NewQuestionRepository(pool)
	ctx := context.Background()

	if _, _, err := repo.List(ctx, uuid.Nil, QuestionFilter{Limit: 10}); !errors.Is(err, ErrScopeRequired) {
		t.Errorf("List(Nil): err = %v, want ErrScopeRequired", err)
	}
	if _, err := repo.Find(ctx, uuid.Nil, uuid.New()); !errors.Is(err, ErrScopeRequired) {
		t.Errorf("Find(Nil): err = %v, want ErrScopeRequired", err)
	}
	if _, err := repo.Create(ctx, uuid.Nil, uuid.New(), samplePG(uuid.New(), uuid.New())); !errors.Is(err, ErrScopeRequired) {
		t.Errorf("Create(Nil): err = %v, want ErrScopeRequired", err)
	}
	if _, err := repo.ProgramExists(ctx, uuid.Nil, uuid.New()); !errors.Is(err, ErrScopeRequired) {
		t.Errorf("ProgramExists(Nil): err = %v, want ErrScopeRequired", err)
	}
	if _, err := repo.SubjectInProgram(ctx, uuid.Nil, uuid.New(), uuid.New()); !errors.Is(err, ErrScopeRequired) {
		t.Errorf("SubjectInProgram(Nil): err = %v, want ErrScopeRequired", err)
	}
	if _, err := repo.ChapterInSubject(ctx, uuid.Nil, uuid.New(), uuid.New()); !errors.Is(err, ErrScopeRequired) {
		t.Errorf("ChapterInSubject(Nil): err = %v, want ErrScopeRequired", err)
	}

	inTx := func(fn func(pgx.Tx) error) error { return inCurriculumTx(t, pool, fn) }
	for name, call := range map[string]func(pgx.Tx) error{
		"FindCurrentTx": func(tx pgx.Tx) error { _, err := repo.FindCurrentTx(ctx, tx, uuid.Nil, uuid.New(), true); return err },
		"InsertVersionTx": func(tx pgx.Tx) error {
			return repo.InsertVersionTx(ctx, tx, uuid.Nil, uuid.New(), 2, uuid.New(), samplePG(uuid.New(), uuid.New()))
		},
		"TouchHeadTx": func(tx pgx.Tx) error { return repo.TouchHeadTx(ctx, tx, uuid.Nil, uuid.New(), 2) },
		"DeleteTx":    func(tx pgx.Tx) error { return repo.DeleteTx(ctx, tx, uuid.Nil, uuid.New()) },
	} {
		if err := inTx(call); !errors.Is(err, ErrScopeRequired) {
			t.Errorf("%s(Nil): err = %v, want ErrScopeRequired", name, err)
		}
	}
}

func TestQuestionCompositeFKsRejectCrossTenantRefs(t *testing.T) {
	f := newQuestionFixture(t)
	ctx := context.Background()
	mine := f.create(t, f.tenant.ID, f.author.ID, samplePG(f.program.ID, f.subject.ID))
	foreign := f.create(t, f.other.ID, f.otherBy.ID, f.question)

	cases := []struct {
		name string
		sql  string
		args []any
	}{
		{
			name: "program of another tenant",
			sql: `INSERT INTO question_versions (question_id, tenant_id, version, program_id, subject_id, type,
			          content, difficulty, correct, created_by)
			      VALUES ($1, $2, 2, $3, $4, 'pg', 'x', 'easy', '[]', $5)`,
			args: []any{mine.ID, f.tenant.ID, foreign.ProgramID, f.subject.ID, f.author.ID},
		},
		{
			name: "subject of another tenant",
			sql: `INSERT INTO question_versions (question_id, tenant_id, version, program_id, subject_id, type,
			          content, difficulty, correct, created_by)
			      VALUES ($1, $2, 2, $3, $4, 'pg', 'x', 'easy', '[]', $5)`,
			args: []any{mine.ID, f.tenant.ID, f.program.ID, foreign.SubjectID, f.author.ID},
		},
		{
			name: "chapter of another tenant",
			sql: `INSERT INTO question_versions (question_id, tenant_id, version, program_id, subject_id, chapter_id,
			          type, content, difficulty, correct, created_by)
			      VALUES ($1, $2, 2, $3, $4, $5, 'pg', 'x', 'easy', '[]', $6)`,
			args: []any{mine.ID, f.tenant.ID, f.program.ID, f.subject.ID, f.otherChapter.ID, f.author.ID},
		},
		{
			name: "version for another tenant's question",
			sql: `INSERT INTO question_versions (question_id, tenant_id, version, program_id, subject_id, type,
			          content, difficulty, correct, created_by)
			      VALUES ($1, $2, 2, $3, $4, 'pg', 'x', 'easy', '[]', $5)`,
			args: []any{foreign.ID, f.tenant.ID, f.program.ID, f.subject.ID, f.author.ID},
		},
	}
	for _, tc := range cases {
		_, err := f.repo.db.Exec(ctx, tc.sql, tc.args...)
		if !isForeignKeyViolation(err, pgForeignKeyViolation) {
			t.Errorf("%s: err = %v, want SQLSTATE %s", tc.name, err, pgForeignKeyViolation)
		}
	}

	var versions int
	if err := f.repo.db.QueryRow(ctx, `SELECT count(*) FROM question_versions WHERE question_id = $1`, mine.ID).Scan(&versions); err != nil {
		t.Fatalf("count versions: %v", err)
	}
	if versions != 1 {
		t.Errorf("versions = %d, want 1: every rejected insert must have rolled back", versions)
	}
}

func TestQuestionVersionsAreImmutable(t *testing.T) {
	f := newQuestionFixture(t)
	ctx := context.Background()
	created := f.create(t, f.tenant.ID, f.author.ID, samplePG(f.program.ID, f.subject.ID))

	before := versionRow(t, f, created.ID, 1)
	if created.Version != 1 || created.Number < 1 {
		t.Fatalf("created version = %d number = %d, want version 1 and a positive number", created.Version, created.Number)
	}

	changed := samplePG(f.program.ID, f.subject.ID)
	changed.Content = "Hasil dari 3 + 3 adalah ..."
	changed.Correct = []string{"a"}
	for _, version := range []int{2, 3} {
		err := inCurriculumTx(t, f.repo.db, func(tx pgx.Tx) error {
			if err := f.repo.InsertVersionTx(ctx, tx, f.tenant.ID, created.ID, version, f.author.ID, changed); err != nil {
				return err
			}
			return f.repo.TouchHeadTx(ctx, tx, f.tenant.ID, created.ID, version)
		})
		if err != nil {
			t.Fatalf("insert version %d: %v", version, err)
		}
	}

	if after := versionRow(t, f, created.ID, 1); after != before {
		t.Errorf("version 1 changed after the update:\nbefore %s\nafter  %s", before, after)
	}
	if got := versionNumbers(t, f, created.ID); got != "1,2,3" {
		t.Errorf("versions = %q, want a gapless 1,2,3", got)
	}

	current, err := f.repo.Find(ctx, f.tenant.ID, created.ID)
	if err != nil {
		t.Fatalf("find: %v", err)
	}
	if current.Version != 3 || current.Content != changed.Content {
		t.Errorf("current = version %d %q, want version 3 with the last content", current.Version, current.Content)
	}
	if current.Number != created.Number {
		t.Errorf("number = %d, want the unchanged %d", current.Number, created.Number)
	}
}

func TestDeleteQuestionReferencedByVersionFKIsRejected(t *testing.T) {
	f := newQuestionFixture(t)
	ctx := context.Background()
	created := f.create(t, f.tenant.ID, f.author.ID, samplePG(f.program.ID, f.subject.ID))

	ref := "qref_" + uuid.NewString()[:8]
	if _, err := f.repo.db.Exec(ctx, fmt.Sprintf(`CREATE TABLE %s (question_version_id uuid REFERENCES question_versions (id))`, ref)); err != nil {
		t.Fatalf("create %s: %v", ref, err)
	}
	t.Cleanup(func() {
		if _, err := f.repo.db.Exec(context.Background(), `DROP TABLE `+ref); err != nil {
			t.Errorf("drop %s: %v", ref, err)
		}
	})
	if _, err := f.repo.db.Exec(ctx, fmt.Sprintf(`INSERT INTO %s (question_version_id) VALUES ($1)`, ref), created.VersionID); err != nil {
		t.Fatalf("reference version: %v", err)
	}

	err := inCurriculumTx(t, f.repo.db, func(tx pgx.Tx) error {
		return f.repo.DeleteTx(ctx, tx, f.tenant.ID, created.ID)
	})
	if !errors.Is(err, ErrReferenced) {
		t.Fatalf("delete a referenced question: err = %v, want ErrReferenced (23503)", err)
	}

	if _, err := f.repo.Find(ctx, f.tenant.ID, created.ID); err != nil {
		t.Errorf("question after the rejected delete: %v, want it still there", err)
	}
	if got := versionNumbers(t, f, created.ID); got != "1" {
		t.Errorf("versions after the rejected delete = %q, want 1", got)
	}

	// Once nothing points at the version, the same delete goes through.
	if _, err := f.repo.db.Exec(ctx, `DELETE FROM `+ref); err != nil {
		t.Fatalf("drop version reference: %v", err)
	}
	if err := inCurriculumTx(t, f.repo.db, func(tx pgx.Tx) error {
		return f.repo.DeleteTx(ctx, tx, f.tenant.ID, created.ID)
	}); err != nil {
		t.Fatalf("delete after the reference is gone: %v", err)
	}
	if _, err := f.repo.Find(ctx, f.tenant.ID, created.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("find after delete: err = %v, want ErrNotFound", err)
	}
}

// Same guard P05 put in place for tryouts; only the database can hold it without a race.
func TestChapterReferencedByQuestionBlocksCurriculumDelete(t *testing.T) {
	f := newQuestionFixture(t)
	ctx := context.Background()
	question := samplePG(f.program.ID, f.subject.ID)
	chapterID := f.chapter.ID
	question.ChapterID = &chapterID
	created := f.create(t, f.tenant.ID, f.author.ID, question)

	err := inCurriculumTx(t, f.repo.db, func(tx pgx.Tx) error {
		return NewCurriculumRepository(f.repo.db).DeleteChaptersTx(ctx, tx, f.tenant.ID, f.program.ID)
	})
	if !errors.Is(err, ErrReferenced) {
		t.Fatalf("delete a chapter a question uses: err = %v, want ErrReferenced (23503)", err)
	}
	if _, err := f.repo.Find(ctx, f.tenant.ID, created.ID); err != nil {
		t.Errorf("question after the rejected curriculum delete: %v, want it still there", err)
	}

	// The question itself can still be deleted, and then the chapter is free.
	if err := inCurriculumTx(t, f.repo.db, func(tx pgx.Tx) error {
		return f.repo.DeleteTx(ctx, tx, f.tenant.ID, created.ID)
	}); err != nil {
		t.Fatalf("delete the question: %v", err)
	}
	if err := inCurriculumTx(t, f.repo.db, func(tx pgx.Tx) error {
		return NewCurriculumRepository(f.repo.db).DeleteChaptersTx(ctx, tx, f.tenant.ID, f.program.ID)
	}); err != nil {
		t.Errorf("delete the chapter after its question is gone: %v", err)
	}
}

func versionRow(t *testing.T, f questionFixture, questionID uuid.UUID, version int) string {
	t.Helper()
	var row string
	if err := f.repo.db.QueryRow(context.Background(),
		`SELECT t::text FROM question_versions t WHERE question_id = $1 AND version = $2`,
		questionID, version).Scan(&row); err != nil {
		t.Fatalf("read version %d: %v", version, err)
	}
	return row
}

func versionNumbers(t *testing.T, f questionFixture, questionID uuid.UUID) string {
	t.Helper()
	rows, err := f.repo.db.Query(context.Background(),
		`SELECT version FROM question_versions WHERE question_id = $1 ORDER BY version`, questionID)
	if err != nil {
		t.Fatalf("read versions: %v", err)
	}
	defer rows.Close()
	out := ""
	for rows.Next() {
		var version int
		if err := rows.Scan(&version); err != nil {
			t.Fatalf("scan version: %v", err)
		}
		if out != "" {
			out += ","
		}
		out += fmt.Sprint(version)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("read versions: %v", err)
	}
	return out
}
