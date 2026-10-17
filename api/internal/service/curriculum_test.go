package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/tarunaeduka/bimbel-tryout/api/internal/model"
	"github.com/tarunaeduka/bimbel-tryout/api/internal/repository"
)

type curriculumHarness struct {
	svc  *CurriculumService
	pool *pgxpool.Pool
	a, b model.Tenant
}

func newCurriculumHarness(t *testing.T) curriculumHarness {
	t.Helper()
	tenants, pool := testTenantService(t)
	a := provisionTenant(t, tenants, pool)
	b := provisionTenant(t, tenants, pool)
	h := curriculumHarness{
		svc:  NewCurriculumService(pool, repository.NewCurriculumRepository(pool)),
		pool: pool,
		a:    a,
		b:    b,
	}
	// Runs before the tenant cleanup provisionTenant registered, so the parents are free by then.
	t.Cleanup(func() {
		ctx := context.Background()
		for _, sql := range []string{
			`DELETE FROM chapters WHERE tenant_id = ANY($1)`,
			`DELETE FROM subjects WHERE tenant_id = ANY($1)`,
			`DELETE FROM programs WHERE tenant_id = ANY($1)`,
		} {
			if _, err := pool.Exec(ctx, sql, []uuid.UUID{a.ID, b.ID}); err != nil {
				t.Errorf("cleanup curriculum: %v", err)
			}
		}
	})
	return h
}

func (h curriculumHarness) scope(tenant model.Tenant) HostScope { return HostScope{Tenant: tenant} }

func (h curriculumHarness) principal(tenant model.Tenant, role model.Role) model.Principal {
	id := tenant.ID
	return model.Principal{UserID: uuid.New(), TenantID: &id, Role: role}
}

func (h curriculumHarness) create(t *testing.T, tenant model.Tenant, input ProgramInput) model.ProgramTree {
	t.Helper()
	tree, err := h.svc.Create(context.Background(), h.scope(tenant), h.principal(tenant, model.RoleAdmin), input)
	if err != nil {
		t.Fatalf("create %s: %v", input.Code, err)
	}
	return tree
}

func (h curriculumHarness) list(t *testing.T, tenant model.Tenant) []model.ProgramTree {
	t.Helper()
	trees, err := h.svc.List(context.Background(), h.scope(tenant), h.principal(tenant, model.RoleOperator))
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	return trees
}

func sampleProgram(code string) ProgramInput {
	return ProgramInput{
		Code: code, Name: "Program " + code, Description: "Deskripsi",
		Subjects: []SubjectInput{{
			Name:     "Matematika",
			Chapters: []ChapterInput{{Name: "Bilangan"}, {Name: "Pecahan"}},
		}},
	}
}

func wantInputError(t *testing.T, err error, path string) {
	t.Helper()
	var input *InputError
	if !errors.As(err, &input) {
		t.Fatalf("err = %v, want *InputError", err)
	}
	if input.Fields[path] == "" {
		t.Errorf("fields = %v, want a reason for %q", input.Fields, path)
	}
}

func TestCurriculumValidation(t *testing.T) {
	h := newCurriculumHarness(t)
	ctx := context.Background()
	admin := h.principal(h.a, model.RoleAdmin)

	normalized, err := h.svc.Create(ctx, h.scope(h.a), admin, ProgramInput{
		Code: "  sd  ", Name: "  Sekolah Dasar  ",
	})
	if err != nil {
		t.Fatalf("normalization: %v", err)
	}
	if normalized.Program.Code != "SD" || normalized.Program.Name != "Sekolah Dasar" {
		t.Errorf("normalized = code %q name %q, want SD and Sekolah Dasar", normalized.Program.Code, normalized.Program.Name)
	}

	longName := strings.Repeat("a", maxNameLen+1)
	longDescription := strings.Repeat("d", maxDescLen+1)
	cases := []struct {
		name  string
		input ProgramInput
		path  string
	}{
		{"code too short", ProgramInput{Code: "S", Name: "Program"}, "code"},
		{"code too long", ProgramInput{Code: "ABCDEFGHIJKLM", Name: "Program"}, "code"},
		{"code with a space", ProgramInput{Code: "S D", Name: "Program"}, "code"},
		{"code with punctuation", ProgramInput{Code: "SD!", Name: "Program"}, "code"},
		{"empty code", ProgramInput{Name: "Program"}, "code"},
		{"empty name", ProgramInput{Code: "SD"}, "name"},
		{"name too long", ProgramInput{Code: "SD", Name: longName}, "name"},
		{"description too long", ProgramInput{Code: "SD", Name: "Program", Description: longDescription}, "description"},
		{"duplicate subject names", ProgramInput{Code: "SD", Name: "Program", Subjects: []SubjectInput{
			{Name: "Matematika"}, {Name: "  matematika "},
		}}, "subjects[1].name"},
		{"empty subject name", ProgramInput{Code: "SD", Name: "Program", Subjects: []SubjectInput{{Name: " "}}}, "subjects[0].name"},
		{"duplicate chapter names", ProgramInput{Code: "SD", Name: "Program", Subjects: []SubjectInput{
			{Name: "IPA", Chapters: []ChapterInput{{Name: "Energi"}, {Name: "ENERGI"}}},
		}}, "subjects[0].chapters[1].name"},
		{"too many subjects", ProgramInput{Code: "SD", Name: "Program", Subjects: manySubjects(maxSubjectsPerProgram + 1)}, "subjects"},
		{"too many chapters", ProgramInput{Code: "SD", Name: "Program", Subjects: []SubjectInput{
			{Name: "IPA", Chapters: manyChapters(maxChaptersPerSubject + 1)},
		}}, "subjects[0].chapters"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := h.svc.Create(ctx, h.scope(h.a), admin, tc.input); err != nil {
				wantInputError(t, err, tc.path)
			} else {
				t.Errorf("create %+v succeeded, want a %q validation error", tc.input.Code, tc.path)
			}
		})
	}

	// The caps themselves are allowed.
	atLimit := ProgramInput{Code: "CAP", Name: "Batas", Subjects: manySubjects(maxSubjectsPerProgram)}
	atLimit.Subjects[0].Chapters = manyChapters(maxChaptersPerSubject)
	if _, err := h.svc.Create(ctx, h.scope(h.a), admin, atLimit); err != nil {
		t.Errorf("create at the 50/200 caps: %v", err)
	}

	// The same chapter name under two different subjects is not a duplicate.
	tree := h.create(t, h.a, ProgramInput{Code: "DUP", Name: "Beda Induk", Subjects: []SubjectInput{
		{Name: "IPA", Chapters: []ChapterInput{{Name: "Pengantar"}}},
		{Name: "IPS", Chapters: []ChapterInput{{Name: "Pengantar"}}},
	}})
	if len(tree.Subjects) != 2 {
		t.Errorf("subjects = %d, want 2: a chapter name is unique only within its subject", len(tree.Subjects))
	}
}

func manySubjects(n int) []SubjectInput {
	out := make([]SubjectInput, 0, n)
	for i := range n {
		out = append(out, SubjectInput{Name: fmt.Sprintf("Mapel %d", i)})
	}
	return out
}

func manyChapters(n int) []ChapterInput {
	out := make([]ChapterInput, 0, n)
	for i := range n {
		out = append(out, ChapterInput{Name: fmt.Sprintf("Bab %d", i)})
	}
	return out
}

func TestReplaceRejectsForeignIds(t *testing.T) {
	h := newCurriculumHarness(t)
	ctx := context.Background()
	admin := h.principal(h.a, model.RoleAdmin)

	own := h.create(t, h.a, sampleProgram("OWN"))
	sibling := h.create(t, h.a, sampleProgram("SIB"))
	foreign := h.create(t, h.b, sampleProgram("FOR"))
	ownSub, siblingSub, foreignSub := own.Subjects[0], sibling.Subjects[0], foreign.Subjects[0]

	_, err := h.svc.Replace(ctx, h.scope(h.a), admin, own.Program.ID, ProgramInput{
		Code: "OWN", Name: "Program OWN",
		Subjects: []SubjectInput{
			{ID: siblingSub.Subject.ID, Name: "Dari Program Lain"},
			{ID: foreignSub.Subject.ID, Name: "Dari Tenant Lain"},
			{ID: ownSub.Subject.ID, Name: "Matematika", Chapters: []ChapterInput{
				{ID: siblingSub.Chapters[0].ID, Name: "Bab Program Lain"},
				{ID: foreignSub.Chapters[0].ID, Name: "Bab Tenant Lain"},
			}},
		},
	})
	if err == nil {
		t.Fatal("replace with foreign ids succeeded, want 422")
	}
	for _, path := range []string{
		"subjects[0].id", "subjects[1].id",
		"subjects[2].chapters[0].id", "subjects[2].chapters[1].id",
	} {
		wantInputError(t, err, path)
	}

	after := h.list(t, h.a)
	if len(after) != 2 {
		t.Fatalf("programs in tenant A = %d, want 2", len(after))
	}
	for _, tree := range after {
		if tree.Program.ID != own.Program.ID {
			continue
		}
		if tree.Program.Name != own.Program.Name || len(tree.Subjects) != 1 || tree.Subjects[0].Subject.Name != "Matematika" {
			t.Errorf("own program = %+v, want it untouched by the rejected replace", tree)
		}
	}
}

func TestReplaceDeletesMissingAndCreatesNew(t *testing.T) {
	h := newCurriculumHarness(t)
	ctx := context.Background()
	admin := h.principal(h.a, model.RoleAdmin)

	created := h.create(t, h.a, ProgramInput{
		Code: "SMP", Name: "SMP",
		Subjects: []SubjectInput{
			{Name: "Matematika", Chapters: []ChapterInput{{Name: "Bilangan"}, {Name: "Pecahan"}}},
			{Name: "IPA", Chapters: []ChapterInput{{Name: "Makhluk Hidup"}}},
		},
	})
	matematika, ipa := created.Subjects[0], created.Subjects[1]
	bilangan, pecahan := matematika.Chapters[0], matematika.Chapters[1]

	replaced, err := h.svc.Replace(ctx, h.scope(h.a), admin, created.Program.ID, ProgramInput{
		Code: "SMP", Name: "SMP Revisi",
		Subjects: []SubjectInput{
			{ID: matematika.Subject.ID, Name: "Matematika Wajib", Chapters: []ChapterInput{
				{ID: pecahan.ID, Name: "Pecahan"},
				{Name: "Pangkat"},
			}},
			{Name: "IPS", Chapters: []ChapterInput{{Name: "Peta"}}},
		},
	})
	if err != nil {
		t.Fatalf("replace: %v", err)
	}
	if replaced.Program.Name != "SMP Revisi" {
		t.Errorf("name = %q, want SMP Revisi", replaced.Program.Name)
	}
	if len(replaced.Subjects) != 2 || replaced.Subjects[0].Subject.ID != matematika.Subject.ID {
		t.Fatalf("subjects = %+v, want the kept subject first with its id", replaced.Subjects)
	}
	if replaced.Subjects[0].Subject.Name != "Matematika Wajib" || replaced.Subjects[0].Subject.Position != 0 {
		t.Errorf("kept subject = %+v, want it renamed and at position 0", replaced.Subjects[0].Subject)
	}
	if got := replaced.Subjects[0].Chapters; len(got) != 2 || got[0].ID != pecahan.ID || got[0].Name != "Pecahan" {
		t.Errorf("chapters = %+v, want the kept chapter first with its unchanged id", got)
	}
	if replaced.Subjects[0].Chapters[1].ID == uuid.Nil || replaced.Subjects[0].Chapters[1].Position != 1 {
		t.Errorf("new chapter = %+v, want a real id at position 1", replaced.Subjects[0].Chapters[1])
	}
	if replaced.Subjects[1].Subject.Name != "IPS" || replaced.Subjects[1].Subject.Position != 1 {
		t.Errorf("new subject = %+v, want IPS at position 1", replaced.Subjects[1].Subject)
	}

	for name, id := range map[string]uuid.UUID{
		"removed subject":         ipa.Subject.ID,
		"removed chapter":         ipa.Chapters[0].ID,
		"removed earlier chapter": bilangan.ID,
	} {
		var remaining int
		if err := h.pool.QueryRow(ctx,
			`SELECT (SELECT count(*) FROM subjects WHERE id = $1) + (SELECT count(*) FROM chapters WHERE id = $1)`,
			id).Scan(&remaining); err != nil {
			t.Fatalf("count %s: %v", name, err)
		}
		if remaining != 0 {
			t.Errorf("%s still exists (id %s)", name, id)
		}
	}
	var subjects, chapters int
	if err := h.pool.QueryRow(ctx, `SELECT count(*) FROM subjects WHERE program_id = $1`, created.Program.ID).Scan(&subjects); err != nil {
		t.Fatalf("count subjects: %v", err)
	}
	if err := h.pool.QueryRow(ctx, `SELECT count(*) FROM chapters WHERE subject_id = $1`, matematika.Subject.ID).Scan(&chapters); err != nil {
		t.Fatalf("count chapters: %v", err)
	}
	if subjects != 2 || chapters != 2 {
		t.Errorf("after replace subjects = %d chapters = %d, want 2 and 2", subjects, chapters)
	}
}

func TestDuplicateCodeConflictsPerTenant(t *testing.T) {
	h := newCurriculumHarness(t)
	ctx := context.Background()

	h.create(t, h.a, sampleProgram("SMA"))
	h.create(t, h.b, sampleProgram("SMA"))

	_, err := h.svc.Create(ctx, h.scope(h.a), h.principal(h.a, model.RoleAdmin), sampleProgram(" sma "))
	if !errors.Is(err, ErrProgramCodeTaken) {
		t.Errorf("duplicate code in tenant A: err = %v, want ErrProgramCodeTaken", err)
	}

	// Another tenant may reuse the code, and its own tree is separate.
	if trees := h.list(t, h.b); len(trees) != 1 || trees[0].Program.Code != "SMA" {
		t.Errorf("tenant B programs = %+v, want its own SMA", trees)
	}
}

func TestDeleteProgramRollsBackWhenAnyChildIsReferenced(t *testing.T) {
	h := newCurriculumHarness(t)
	ctx := context.Background()
	admin := h.principal(h.a, model.RoleAdmin)

	created := h.create(t, h.a, ProgramInput{
		Code: "DEL", Name: "Hapus",
		Subjects: []SubjectInput{
			{Name: "Matematika", Chapters: []ChapterInput{{Name: "Bilangan"}}},
			{Name: "IPA", Chapters: []ChapterInput{{Name: "Energi"}}},
		},
	})

	chapterRef := "ref_chapter_" + uuid.NewString()[:8]
	subjectRef := "ref_subject_" + uuid.NewString()[:8]
	for _, sql := range []string{
		fmt.Sprintf(`CREATE TABLE %s (chapter_id uuid REFERENCES chapters (id))`, chapterRef),
		fmt.Sprintf(`CREATE TABLE %s (subject_id uuid REFERENCES subjects (id))`, subjectRef),
	} {
		if _, err := h.pool.Exec(ctx, sql); err != nil {
			t.Fatalf("create ref table: %v", err)
		}
	}
	t.Cleanup(func() {
		for _, table := range []string{chapterRef, subjectRef} {
			if _, err := h.pool.Exec(context.Background(), `DROP TABLE `+table); err != nil {
				t.Errorf("drop %s: %v", table, err)
			}
		}
	})

	insert := func(sql string, id uuid.UUID) {
		t.Helper()
		if _, err := h.pool.Exec(ctx, sql, id); err != nil {
			t.Fatalf("reference: %v", err)
		}
	}
	insert(fmt.Sprintf(`INSERT INTO %s (chapter_id) VALUES ($1)`, chapterRef), created.Subjects[1].Chapters[0].ID)
	insert(fmt.Sprintf(`INSERT INTO %s (subject_id) VALUES ($1)`, subjectRef), created.Subjects[0].Subject.ID)

	if err := h.svc.Delete(ctx, h.scope(h.a), admin, created.Program.ID); !errors.Is(err, ErrProgramReferenced) {
		t.Fatalf("delete with referenced children: err = %v, want ErrProgramReferenced", err)
	}
	trees := h.list(t, h.a)
	if len(trees) != 1 || len(trees[0].Subjects) != 2 {
		t.Fatalf("tree after the rejected delete = %+v, want it fully intact", trees)
	}
	for _, subject := range trees[0].Subjects {
		if len(subject.Chapters) != 1 {
			t.Errorf("subject %q chapters = %d, want 1: the rolled-back delete kept them", subject.Subject.Name, len(subject.Chapters))
		}
	}

	// Once nothing points at it, the same delete goes through.
	if _, err := h.pool.Exec(ctx, `DELETE FROM `+chapterRef); err != nil {
		t.Fatalf("drop chapter reference: %v", err)
	}
	if _, err := h.pool.Exec(ctx, `DELETE FROM `+subjectRef); err != nil {
		t.Fatalf("drop subject reference: %v", err)
	}
	if err := h.svc.Delete(ctx, h.scope(h.a), admin, created.Program.ID); err != nil {
		t.Fatalf("delete after the references are gone: %v", err)
	}
	if trees := h.list(t, h.a); len(trees) != 0 {
		t.Errorf("programs after delete = %+v, want none", trees)
	}
}

// A program id from another tenant must look missing, never forbidden.
func TestCurriculumTenantMismatchIsNotFound(t *testing.T) {
	h := newCurriculumHarness(t)
	ctx := context.Background()
	other := h.create(t, h.b, sampleProgram("OTHER"))

	cases := map[string]error{
		"list": func() error {
			_, err := h.svc.List(ctx, h.scope(h.a), h.principal(h.b, model.RoleAdmin))
			return err
		}(),
		"replace": func() error {
			_, err := h.svc.Replace(ctx, h.scope(h.a), h.principal(h.a, model.RoleAdmin), other.Program.ID, sampleProgram("XX"))
			return err
		}(),
		"delete": h.svc.Delete(ctx, h.scope(h.a), h.principal(h.a, model.RoleAdmin), other.Program.ID),
	}
	for name, err := range cases {
		if !errors.Is(err, ErrProgramNotFound) && !errors.Is(err, ErrTenantNotFound) {
			t.Errorf("%s: err = %v, want ErrProgramNotFound", name, err)
		}
	}
}
