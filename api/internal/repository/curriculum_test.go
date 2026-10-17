package repository

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/tarunaeduka/bimbel-tryout/api/internal/model"
)

// newCurriculumTenant owns one tenant and every curriculum row under it, children first on cleanup.
func newCurriculumTenant(t *testing.T) model.Tenant {
	t.Helper()
	pool := testPool(t)
	tenant := newTenant(t, NewTenantRepository(pool))
	t.Cleanup(func() {
		ctx := context.Background()
		for _, sql := range []string{
			`DELETE FROM chapters WHERE tenant_id = $1`,
			`DELETE FROM subjects WHERE tenant_id = $1`,
			`DELETE FROM programs WHERE tenant_id = $1`,
		} {
			if _, err := pool.Exec(ctx, sql, tenant.ID); err != nil {
				t.Errorf("cleanup curriculum of tenant %s: %v", tenant.ID, err)
			}
		}
	})
	return tenant
}

func programTree(code, name string, subjects ...model.SubjectTree) model.ProgramTree {
	return model.ProgramTree{
		Program:  model.Program{Code: code, Name: name, Description: "deskripsi " + code},
		Subjects: subjects,
	}
}

func subjectTree(name string, chapters ...string) model.SubjectTree {
	out := model.SubjectTree{Subject: model.Subject{Name: name}}
	for _, chapter := range chapters {
		out.Chapters = append(out.Chapters, model.Chapter{Name: chapter})
	}
	return out
}

func mustCreateTree(t *testing.T, repo *CurriculumRepository, tenantID uuid.UUID, tree model.ProgramTree) model.ProgramTree {
	t.Helper()
	created, err := repo.CreateTree(context.Background(), tenantID, tree)
	if err != nil {
		t.Fatalf("create tree %s: %v", tree.Program.Code, err)
	}
	return created
}

func inCurriculumTx(t *testing.T, pool interface {
	Begin(context.Context) (pgx.Tx, error)
}, fn func(pgx.Tx) error) error {
	t.Helper()
	ctx := context.Background()
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := fn(tx); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func TestCurriculumRequiresScope(t *testing.T) {
	pool := testPool(t)
	repo := NewCurriculumRepository(pool)
	ctx := context.Background()

	if _, err := repo.List(ctx, uuid.Nil); !errors.Is(err, ErrScopeRequired) {
		t.Errorf("List(Nil): err = %v, want ErrScopeRequired", err)
	}
	if _, err := repo.CreateTree(ctx, uuid.Nil, programTree("NIL", "Nol")); !errors.Is(err, ErrScopeRequired) {
		t.Errorf("CreateTree(Nil): err = %v, want ErrScopeRequired", err)
	}
	inTx := func(fn func(pgx.Tx) error) error { return inCurriculumTx(t, pool, fn) }

	if err := inTx(func(tx pgx.Tx) error {
		_, err := repo.FindTreeTx(ctx, tx, uuid.Nil, uuid.New(), false)
		return err
	}); !errors.Is(err, ErrScopeRequired) {
		t.Errorf("FindTreeTx(Nil): err = %v, want ErrScopeRequired", err)
	}
	if err := inTx(func(tx pgx.Tx) error {
		_, err := repo.ReplaceTreeTx(ctx, tx, uuid.Nil, uuid.New(), programTree("NIL", "Nol"))
		return err
	}); !errors.Is(err, ErrScopeRequired) {
		t.Errorf("ReplaceTreeTx(Nil): err = %v, want ErrScopeRequired", err)
	}
	for name, call := range map[string]func(pgx.Tx) error{
		"DeleteChaptersTx": func(tx pgx.Tx) error { return repo.DeleteChaptersTx(ctx, tx, uuid.Nil, uuid.New()) },
		"DeleteSubjectsTx": func(tx pgx.Tx) error { return repo.DeleteSubjectsTx(ctx, tx, uuid.Nil, uuid.New()) },
		"DeleteProgramTx":  func(tx pgx.Tx) error { return repo.DeleteProgramTx(ctx, tx, uuid.Nil, uuid.New()) },
	} {
		if err := inTx(call); !errors.Is(err, ErrScopeRequired) {
			t.Errorf("%s(Nil): err = %v, want ErrScopeRequired", name, err)
		}
	}
}

func TestCurriculumCompositeFKRejectsCrossTenantParent(t *testing.T) {
	pool := testPool(t)
	repo := NewCurriculumRepository(pool)
	ctx := context.Background()
	owner, other := newCurriculumTenant(t), newCurriculumTenant(t)
	created := mustCreateTree(t, repo, owner.ID, programTree("SD", "Sekolah Dasar", subjectTree("Matematika", "Bilangan")))
	subjectID := created.Subjects[0].Subject.ID

	cases := []struct {
		name string
		sql  string
		args []any
	}{
		{
			name: "subject under another tenant's program",
			sql:  `INSERT INTO subjects (tenant_id, program_id, name, position) VALUES ($1, $2, 'Fisika', 1)`,
			args: []any{other.ID, created.Program.ID},
		},
		{
			name: "chapter under another tenant's subject",
			sql:  `INSERT INTO chapters (tenant_id, subject_id, name, position) VALUES ($1, $2, 'Aljabar', 1)`,
			args: []any{other.ID, subjectID},
		},
	}
	for _, tc := range cases {
		_, err := pool.Exec(ctx, tc.sql, tc.args...)
		var pgErr *pgconn.PgError
		if !errors.As(err, &pgErr) || pgErr.Code != pgForeignKeyViolation {
			t.Errorf("%s: err = %v, want SQLSTATE %s", tc.name, err, pgForeignKeyViolation)
		}
	}

	var subjects int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM subjects WHERE tenant_id = $1`, owner.ID).Scan(&subjects); err != nil {
		t.Fatalf("count subjects: %v", err)
	}
	if subjects != 1 {
		t.Errorf("owner subjects = %d, want 1: the rejected inserts must not have landed", subjects)
	}
}

func TestDeleteReferencedIsRejected(t *testing.T) {
	pool := testPool(t)
	repo := NewCurriculumRepository(pool)
	ctx := context.Background()
	tenant := newCurriculumTenant(t)
	created := mustCreateTree(t, repo, tenant.ID, programTree("SMP", "SMP", subjectTree("IPA", "Zat")))
	chapterID := created.Subjects[0].Chapters[0].ID

	ref := "ref_" + uuid.NewString()[:8]
	if _, err := pool.Exec(ctx, fmt.Sprintf(`CREATE TABLE %s (chapter_id uuid REFERENCES chapters (id))`, ref)); err != nil {
		t.Fatalf("create %s: %v", ref, err)
	}
	t.Cleanup(func() {
		if _, err := pool.Exec(context.Background(), `DROP TABLE `+ref); err != nil {
			t.Errorf("drop %s: %v", ref, err)
		}
	})
	if _, err := pool.Exec(ctx, fmt.Sprintf(`INSERT INTO %s (chapter_id) VALUES ($1)`, ref), chapterID); err != nil {
		t.Fatalf("reference chapter: %v", err)
	}

	if _, err := pool.Exec(ctx, `DELETE FROM chapters WHERE id = $1`, chapterID); isForeignKeyViolation(err, pgForeignKeyViolation) == false {
		t.Errorf("raw chapter delete: err = %v, want SQLSTATE %s", err, pgForeignKeyViolation)
	}

	attempts := map[string]func(pgx.Tx) error{
		"chapter": func(tx pgx.Tx) error { return repo.DeleteChaptersTx(ctx, tx, tenant.ID, created.Program.ID) },
		"subject": func(tx pgx.Tx) error { return repo.DeleteSubjectsTx(ctx, tx, tenant.ID, created.Program.ID) },
		"program": func(tx pgx.Tx) error { return repo.DeleteProgramTx(ctx, tx, tenant.ID, created.Program.ID) },
	}
	for name, call := range attempts {
		if err := inCurriculumTx(t, pool, call); !errors.Is(err, ErrReferenced) {
			t.Errorf("%s delete: err = %v, want ErrReferenced (23503)", name, err)
		}
	}

	trees, err := repo.List(ctx, tenant.ID)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(trees) != 1 || len(trees[0].Subjects) != 1 || len(trees[0].Subjects[0].Chapters) != 1 {
		t.Fatalf("tree = %+v, want the program, subject and chapter still intact", trees)
	}

	// A subject nobody references is still protected by its own chapters; ON DELETE CASCADE would
	// quietly take them along, which is exactly what RESTRICT exists to prevent.
	clean := newCurriculumTenant(t)
	owned := mustCreateTree(t, repo, clean.ID, programTree("SMA", "SMA", subjectTree("Fisika", "Mekanika")))
	if err := inCurriculumTx(t, pool, func(tx pgx.Tx) error {
		return repo.DeleteSubjectsTx(ctx, tx, clean.ID, owned.Program.ID)
	}); !errors.Is(err, ErrReferenced) {
		t.Errorf("deleting a subject that still has chapters: err = %v, want ErrReferenced (23503)", err)
	}
}

// The unique index on (parent, lower(name)) is what makes the case-insensitive rule hold in the
// database, not only in the service.
func TestCurriculumNameUniquenessIsCaseInsensitive(t *testing.T) {
	pool := testPool(t)
	repo := NewCurriculumRepository(pool)
	ctx := context.Background()
	tenant := newCurriculumTenant(t)
	created := mustCreateTree(t, repo, tenant.ID, programTree("SD", "SD", subjectTree("Matematika", "Bilangan")))
	subjectID := created.Subjects[0].Subject.ID

	cases := []struct {
		name string
		sql  string
		args []any
	}{
		{
			name: "subject name in another case",
			sql:  `INSERT INTO subjects (tenant_id, program_id, name, position) VALUES ($1, $2, 'matematika', 1)`,
			args: []any{tenant.ID, created.Program.ID},
		},
		{
			name: "chapter name in another case",
			sql:  `INSERT INTO chapters (tenant_id, subject_id, name, position) VALUES ($1, $2, 'bilangan', 1)`,
			args: []any{tenant.ID, subjectID},
		},
	}
	for _, tc := range cases {
		_, err := pool.Exec(ctx, tc.sql, tc.args...)
		if !isForeignKeyViolation(err, pgUniqueViolation) {
			t.Errorf("%s: err = %v, want SQLSTATE %s", tc.name, err, pgUniqueViolation)
		}
	}
}

func isForeignKeyViolation(err error, code string) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == code
}

func TestReplaceTreeKeepsIdsAndPositions(t *testing.T) {
	pool := testPool(t)
	repo := NewCurriculumRepository(pool)
	ctx := context.Background()
	tenant := newCurriculumTenant(t)
	created := mustCreateTree(t, repo, tenant.ID, programTree("SMA", "SMA",
		subjectTree("Fisika", "Mekanika", "Listrik"),
		subjectTree("Kimia", "Tabel Periodik"),
	))
	fisika, kimia := created.Subjects[0], created.Subjects[1]
	mekanika, listrik := fisika.Chapters[0], fisika.Chapters[1]
	periodik := kimia.Chapters[0]

	// Kimia first and renamed, Fisika keeps its chapters in the other order, Matematika is new.
	replaced, err := inCurriculumT(t, pool, repo, tenant.ID, created.Program.ID, model.ProgramTree{
		Program: model.Program{Code: "SMA", Name: "SMA Revisi", Description: "baru"},
		Subjects: []model.SubjectTree{
			{Subject: model.Subject{ID: kimia.Subject.ID, Name: "Kimia Dasar"}, Chapters: []model.Chapter{
				{ID: periodik.ID, Name: "Tabel Periodik"},
			}},
			{Subject: model.Subject{ID: fisika.Subject.ID, Name: "Fisika"}, Chapters: []model.Chapter{
				{ID: listrik.ID, Name: "Listrik"},
				{ID: mekanika.ID, Name: "Mekanika"},
				{Name: "Gelombang"},
			}},
			{Subject: model.Subject{Name: "Matematika"}, Chapters: []model.Chapter{{Name: "Logaritma"}}},
		},
	})
	if err != nil {
		t.Fatalf("replace: %v", err)
	}

	if replaced.Program.Name != "SMA Revisi" || replaced.Program.Description != "baru" {
		t.Errorf("program = %+v, want the renamed header", replaced.Program)
	}
	if got := names(replaced); got != "Kimia Dasar/0, Fisika/1, Matematika/2" {
		t.Errorf("subjects = %q, want the body order rewritten as positions", got)
	}
	if replaced.Subjects[0].Subject.ID != kimia.Subject.ID || replaced.Subjects[1].Subject.ID != fisika.Subject.ID {
		t.Error("a kept subject changed its id")
	}
	if replaced.Subjects[2].Subject.ID == uuid.Nil {
		t.Error("the created subject has no id")
	}
	if got, want := chapterIDs(replaced.Subjects[1]), []uuid.UUID{listrik.ID, mekanika.ID}; got[0] != want[0] || got[1] != want[1] {
		t.Errorf("Fisika chapters = %v, want the kept ids in the new order", got)
	}
	if got := chapterNames(replaced.Subjects[1]); got != "Listrik, Mekanika, Gelombang" {
		t.Errorf("Fisika chapter names = %q, want Listrik, Mekanika, Gelombang", got)
	}
	if replaced.Subjects[1].Chapters[2].ID == uuid.Nil {
		t.Error("the created chapter has no id")
	}

	trees, err := repo.List(ctx, tenant.ID)
	if err != nil || len(trees) != 1 {
		t.Fatalf("list = %v, %v; want the one program", trees, err)
	}
	if got := len(trees[0].Subjects[1].Chapters); got != 3 {
		t.Errorf("re-read Fisika chapters = %d, want the 3 the replace left", got)
	}

	// A body that names nothing deletes the whole tree, chapters before subjects.
	emptied, err := inCurriculumT(t, pool, repo, tenant.ID, created.Program.ID, model.ProgramTree{
		Program: model.Program{Code: "SMA", Name: "SMA"},
	})
	if err != nil {
		t.Fatalf("replace with no subjects: %v", err)
	}
	if len(emptied.Subjects) != 0 {
		t.Errorf("subjects after an empty body = %d, want 0", len(emptied.Subjects))
	}
	var chapters int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM chapters WHERE tenant_id = $1`, tenant.ID).Scan(&chapters); err != nil {
		t.Fatalf("count chapters: %v", err)
	}
	if chapters != 0 {
		t.Errorf("chapters left = %d, want 0", chapters)
	}
}

func inCurriculumT(t *testing.T, pool interface {
	Begin(context.Context) (pgx.Tx, error)
}, repo *CurriculumRepository, tenantID, programID uuid.UUID, tree model.ProgramTree) (model.ProgramTree, error) {
	t.Helper()
	var out model.ProgramTree
	err := inCurriculumTx(t, pool, func(tx pgx.Tx) error {
		var err error
		if _, err = repo.FindTreeTx(context.Background(), tx, tenantID, programID, true); err != nil {
			return err
		}
		out, err = repo.ReplaceTreeTx(context.Background(), tx, tenantID, programID, tree)
		return err
	})
	return out, err
}

func names(tree model.ProgramTree) string {
	out := ""
	for i, subject := range tree.Subjects {
		if i > 0 {
			out += ", "
		}
		out += fmt.Sprintf("%s/%d", subject.Subject.Name, subject.Subject.Position)
	}
	return out
}

func chapterIDs(subject model.SubjectTree) []uuid.UUID {
	out := make([]uuid.UUID, 0, len(subject.Chapters))
	for _, chapter := range subject.Chapters {
		out = append(out, chapter.ID)
	}
	return out
}

func chapterNames(subject model.SubjectTree) string {
	out := ""
	for i, chapter := range subject.Chapters {
		if i > 0 {
			out += ", "
		}
		out += chapter.Name
	}
	return out
}
