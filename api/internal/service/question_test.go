package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/tarunaeduka/bimbel-tryout/api/internal/model"
	"github.com/tarunaeduka/bimbel-tryout/api/internal/repository"
)

type questionHarness struct {
	svc    *QuestionService
	pool   *pgxpool.Pool
	a, b   model.Tenant
	author map[uuid.UUID]model.User
	tree   map[uuid.UUID]model.ProgramTree
}

func newQuestionHarness(t *testing.T) questionHarness {
	t.Helper()
	tenants, pool := testTenantService(t)
	a := provisionTenant(t, tenants, pool)
	b := provisionTenant(t, tenants, pool)
	curriculum := repository.NewCurriculumRepository(pool)
	users := repository.NewUserRepository(pool)

	h := questionHarness{
		svc:    NewQuestionService(pool, repository.NewQuestionRepository(pool)),
		pool:   pool,
		a:      a,
		b:      b,
		author: map[uuid.UUID]model.User{},
		tree:   map[uuid.UUID]model.ProgramTree{},
	}
	// Registered last, so it runs before the tenant cleanup: questions, then their curriculum.
	t.Cleanup(func() {
		ctx := context.Background()
		ids := []uuid.UUID{a.ID, b.ID}
		for _, sql := range []string{
			`DELETE FROM question_versions WHERE tenant_id = ANY($1)`,
			`DELETE FROM questions WHERE tenant_id = ANY($1)`,
			`DELETE FROM chapters WHERE tenant_id = ANY($1)`,
			`DELETE FROM subjects WHERE tenant_id = ANY($1)`,
			`DELETE FROM programs WHERE tenant_id = ANY($1)`,
		} {
			if _, err := pool.Exec(ctx, sql, ids); err != nil {
				t.Errorf("cleanup questions fixture: %v", err)
			}
		}
	})

	for _, tenant := range []model.Tenant{a, b} {
		tree, err := curriculum.CreateTree(context.Background(), tenant.ID, model.ProgramTree{
			Program: model.Program{Code: "QD", Name: "Program Soal"},
			Subjects: []model.SubjectTree{
				{Subject: model.Subject{Name: "Matematika", Position: 0}, Chapters: []model.Chapter{{Name: "Bilangan", Position: 0}}},
				{Subject: model.Subject{Name: "Fisika", Position: 1}, Chapters: []model.Chapter{{Name: "Gerak", Position: 0}}},
			},
		})
		if err != nil {
			t.Fatalf("create curriculum: %v", err)
		}
		h.tree[tenant.ID] = tree
		email := "question-" + uuid.NewString()[:8] + "@example.com"
		user, err := users.Create(context.Background(), model.User{
			TenantID: &tenant.ID, Name: "Penulis Soal", Email: &email, PasswordHash: "x", Role: model.RoleAdmin,
		})
		if err != nil {
			t.Fatalf("create author: %v", err)
		}
		h.author[tenant.ID] = user
	}
	return h
}

func (h questionHarness) scope(tenant model.Tenant) HostScope { return HostScope{Tenant: tenant} }

func (h questionHarness) principal(tenant model.Tenant, role model.Role) model.Principal {
	id := tenant.ID
	return model.Principal{UserID: h.author[tenant.ID].ID, TenantID: &id, Role: role}
}

func (h questionHarness) program(tenant model.Tenant) model.Program { return h.tree[tenant.ID].Program }

func (h questionHarness) subject(tenant model.Tenant) model.Subject {
	return h.tree[tenant.ID].Subjects[0].Subject
}

func (h questionHarness) create(t *testing.T, tenant model.Tenant, input QuestionInput) model.Question {
	t.Helper()
	created, err := h.svc.Create(context.Background(), h.scope(tenant), h.principal(tenant, model.RoleAdmin), input)
	if err != nil {
		t.Fatalf("create question: %v", err)
	}
	return created
}

func (h questionHarness) pgInput(tenant model.Tenant) QuestionInput {
	return QuestionInput{
		ProgramID: h.program(tenant).ID, SubjectID: h.subject(tenant).ID,
		Type: model.QuestionPG, Content: "Hasil dari 2 + 2 adalah ...", Difficulty: model.DifficultyEasy,
		Explanation: "2 + 2 = 4.",
		Options:     []QuestionOptionInput{{ID: "a", Label: "3"}, {ID: "b", Label: "4"}},
		Correct:     []string{"b"},
	}
}

func (h questionHarness) createPG(t *testing.T, tenant model.Tenant) model.Question {
	t.Helper()
	return h.create(t, tenant, h.pgInput(tenant))
}

func (h questionHarness) update(t *testing.T, tenant model.Tenant, id uuid.UUID, input QuestionInput) (model.Question, error) {
	t.Helper()
	return h.svc.Update(context.Background(), h.scope(tenant), h.principal(tenant, model.RoleAdmin), id, input)
}

func (h questionHarness) list(t *testing.T, tenant model.Tenant, filter QuestionFilter) ([]model.Question, int) {
	t.Helper()
	questions, total, err := h.svc.List(context.Background(), h.scope(tenant), h.principal(tenant, model.RoleOperator), filter)
	if err != nil {
		t.Fatalf("list questions: %v", err)
	}
	return questions, total
}

func optionIDs(question model.Question) []string {
	out := make([]string, 0, len(question.Options))
	for _, option := range question.Options {
		out = append(out, option.ID)
	}
	return out
}

func TestQuestionValidation(t *testing.T) {
	h := newQuestionHarness(t)
	tenant := h.a
	base := h.pgInput(tenant)
	options := func(labels ...string) []QuestionOptionInput {
		out := make([]QuestionOptionInput, 0, len(labels))
		for _, label := range labels {
			out = append(out, QuestionOptionInput{Label: label})
		}
		return out
	}
	points := func(value float64) *float64 { return &value }

	cases := []struct {
		name  string
		input func() QuestionInput
		path  string
	}{
		{"unknown type", func() QuestionInput { in := base; in.Type = "essay"; return in }, "type"},
		{"unknown difficulty", func() QuestionInput { in := base; in.Difficulty = "extreme"; return in }, "difficulty"},
		{"empty content", func() QuestionInput { in := base; in.Content = "   "; return in }, "content"},
		{"content too long", func() QuestionInput { in := base; in.Content = strings.Repeat("a", maxContentLen+1); return in }, "content"},
		{"data image", func() QuestionInput {
			in := base
			in.Content = "Lihat gambar ![x](data:image/png;base64,AAAA)"
			return in
		}, "content"},
		{"javascript image", func() QuestionInput {
			in := base
			in.Content = "Lihat ![x](JaVaScRiPt:alert(1))"
			return in
		}, "content"},
		{"explanation too long", func() QuestionInput {
			in := base
			in.Explanation = strings.Repeat("e", maxExplanationLen+1)
			return in
		}, "explanation"},
		{"missing program", func() QuestionInput { in := base; in.ProgramID = uuid.Nil; return in }, "program_id"},
		{"missing subject", func() QuestionInput { in := base; in.SubjectID = uuid.Nil; return in }, "subject_id"},
		{"program of another tenant", func() QuestionInput {
			in := base
			in.ProgramID = h.program(h.b).ID
			return in
		}, "program_id"},
		{"subject not in program", func() QuestionInput {
			in := base
			in.SubjectID = h.tree[tenant.ID].Subjects[1].Subject.ID
			in.ProgramID = h.program(h.b).ID
			return in
		}, "program_id"},
		{"chapter not in subject", func() QuestionInput {
			in := base
			chapter := h.tree[tenant.ID].Subjects[1].Chapters[0].ID
			in.ChapterID = &chapter
			return in
		}, "chapter_id"},
		{"pg too few options", func() QuestionInput { in := base; in.Options = options("3"); return in }, "options"},
		{"pg too many options", func() QuestionInput {
			in := base
			in.Options = options("1", "2", "3", "4", "5", "6", "7", "8", "9")
			return in
		}, "options"},
		{"pg empty label", func() QuestionInput {
			in := base
			in.Options = []QuestionOptionInput{{ID: "a", Label: "3"}, {ID: "b", Label: "  "}}
			return in
		}, "options[1].label"},
		{"pg label too long", func() QuestionInput {
			in := base
			in.Options = []QuestionOptionInput{{ID: "a", Label: "3"}, {ID: "b", Label: strings.Repeat("b", maxOptionLabelLen+1)}}
			return in
		}, "options[1].label"},
		{"pg duplicate labels", func() QuestionInput {
			in := base
			in.Options = []QuestionOptionInput{{ID: "a", Label: "Empat"}, {ID: "b", Label: " empat "}}
			return in
		}, "options[1].label"},
		{"pg no correct answer", func() QuestionInput { in := base; in.Correct = nil; return in }, "correct"},
		{"pg two correct answers", func() QuestionInput { in := base; in.Correct = []string{"a", "b"}; return in }, "correct"},
		{"pg correct not an option", func() QuestionInput { in := base; in.Correct = []string{"z"}; return in }, "correct"},
		{"pg weighted with a correct answer", func() QuestionInput {
			in := base
			in.OptionWeighted = true
			return in
		}, "correct"},
		{"pg weighted points above 100", func() QuestionInput {
			in := base
			in.OptionWeighted = true
			in.Correct = nil
			in.Options[0].Points = points(101)
			return in
		}, "options[0].points"},
		{"pg weighted points with three decimals", func() QuestionInput {
			in := base
			in.OptionWeighted = true
			in.Correct = nil
			in.Options[0].Points = points(1.234)
			return in
		}, "options[0].points"},
		{"pg unweighted with points", func() QuestionInput { in := base; in.Options[0].Points = points(2); return in }, "options[0].points"},
		{"pg_multi single correct", func() QuestionInput {
			in := base
			in.Type = model.QuestionPGMulti
			return in
		}, "correct"},
		{"pg_multi duplicate correct", func() QuestionInput {
			in := base
			in.Type = model.QuestionPGMulti
			in.Correct = []string{"a", "a"}
			return in
		}, "correct"},
		{"pg_multi weighted", func() QuestionInput {
			in := base
			in.Type = model.QuestionPGMulti
			in.OptionWeighted = true
			in.Correct = []string{"a", "b"}
			return in
		}, "option_weighted"},
		{"isian with options", func() QuestionInput {
			in := base
			in.Type = model.QuestionIsian
			in.Correct = []string{"4"}
			return in
		}, "options"},
		{"isian without answers", func() QuestionInput {
			in := base
			in.Type = model.QuestionIsian
			in.Options = nil
			in.Correct = nil
			return in
		}, "correct"},
		{"isian too many answers", func() QuestionInput {
			in := base
			in.Type = model.QuestionIsian
			in.Options = nil
			for i := range maxIsianAnswers + 1 {
				in.Correct = append(in.Correct, fmt.Sprintf("jawaban %d", i))
			}
			return in
		}, "correct"},
		{"isian blank answer", func() QuestionInput {
			in := base
			in.Type = model.QuestionIsian
			in.Options = nil
			in.Correct = []string{"  "}
			return in
		}, "correct"},
		{"isian answer too long", func() QuestionInput {
			in := base
			in.Type = model.QuestionIsian
			in.Options = nil
			in.Correct = []string{strings.Repeat("a", maxIsianAnswerLen+1)}
			return in
		}, "correct"},
		{"isian duplicate answers ignoring case", func() QuestionInput {
			in := base
			in.Type = model.QuestionIsian
			in.Options = nil
			in.Correct = []string{"Empat", "empat"}
			return in
		}, "correct"},
		{"isian weighted", func() QuestionInput {
			in := base
			in.Type = model.QuestionIsian
			in.Options = nil
			in.Correct = []string{"4"}
			in.OptionWeighted = true
			return in
		}, "option_weighted"},
		{"case_sensitive outside isian", func() QuestionInput { in := base; in.CaseSensitive = true; return in }, "case_sensitive"},
		{"benar_salah without an answer", func() QuestionInput {
			in := base
			in.Type = model.QuestionTrueFalse
			in.Options = nil
			in.Correct = nil
			return in
		}, "correct"},
		{"benar_salah with both answers", func() QuestionInput {
			in := base
			in.Type = model.QuestionTrueFalse
			in.Options = nil
			in.Correct = []string{"benar", "salah"}
			return in
		}, "correct"},
		{"benar_salah with an outside answer", func() QuestionInput {
			in := base
			in.Type = model.QuestionTrueFalse
			in.Options = nil
			in.Correct = []string{"mungkin"}
			return in
		}, "correct"},
		{"benar_salah weighted", func() QuestionInput {
			in := base
			in.Type = model.QuestionTrueFalse
			in.Options = nil
			in.Correct = []string{"benar"}
			in.OptionWeighted = true
			return in
		}, "option_weighted"},
		{"benar_salah with other options", func() QuestionInput {
			in := base
			in.Type = model.QuestionTrueFalse
			in.Correct = []string{"benar"}
			return in
		}, "options"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := h.svc.Create(context.Background(), h.scope(tenant), h.principal(tenant, model.RoleAdmin), tc.input())
			if err == nil {
				t.Fatalf("create succeeded, want a %q validation error", tc.path)
			}
			wantInputError(t, err, tc.path)
		})
	}

	// The four types, as accepted.
	pg := h.createPG(t, tenant)
	if len(pg.Options) != 2 || pg.Correct[0] != pg.Options[1].ID {
		t.Errorf("pg = options %+v correct %v, want the placeholders replaced by ids", pg.Options, pg.Correct)
	}
	if pg.Content != base.Content || pg.Explanation != base.Explanation {
		t.Errorf("pg content = %q explanation = %q, want the trimmed text as sent", pg.Content, pg.Explanation)
	}

	multi := h.create(t, tenant, QuestionInput{
		ProgramID: h.program(tenant).ID, SubjectID: h.subject(tenant).ID,
		Type: model.QuestionPGMulti, Content: "Manakah bilangan genap?", Difficulty: model.DifficultyMedium,
		Options: []QuestionOptionInput{{ID: "a", Label: "2"}, {ID: "b", Label: "3"}, {ID: "c", Label: "4"}},
		Correct: []string{"a", "c"},
	})
	if len(multi.Correct) != 2 || !strings.Contains(strings.Join(optionIDs(multi), ","), multi.Correct[0]) {
		t.Errorf("pg_multi correct = %v options = %v, want two real option ids", multi.Correct, optionIDs(multi))
	}

	isian := h.create(t, tenant, QuestionInput{
		ProgramID: h.program(tenant).ID, SubjectID: h.subject(tenant).ID,
		Type: model.QuestionIsian, Content: "Hasil dari 3 + 4 adalah ...", Difficulty: model.DifficultyEasy,
		Correct: []string{" 7 ", "tujuh"}, CaseSensitive: true,
	})
	if len(isian.Options) != 0 || len(isian.Correct) != 2 || isian.Correct[0] != "7" {
		t.Errorf("isian = options %v correct %v, want no options and two trimmed answers", isian.Options, isian.Correct)
	}

	trueFalse := h.create(t, tenant, QuestionInput{
		ProgramID: h.program(tenant).ID, SubjectID: h.subject(tenant).ID,
		Type: model.QuestionTrueFalse, Content: "Bumi itu bulat.", Difficulty: model.DifficultyEasy,
		Correct: []string{"benar"},
	})
	if ids := optionIDs(trueFalse); len(ids) != 2 || ids[0] != "benar" || ids[1] != "salah" {
		t.Errorf("benar_salah options = %v, want the server's fixed benar/salah", ids)
	}

	// Weighted mode keeps the points the editor set, and an option without points stays absent so
	// scoring (P16) can default it to 1.
	weighted := h.create(t, tenant, QuestionInput{
		ProgramID: h.program(tenant).ID, SubjectID: h.subject(tenant).ID,
		Type: model.QuestionPG, Content: "Sikap paling tepat adalah ...", Difficulty: model.DifficultyEasy,
		OptionWeighted: true, Options: []QuestionOptionInput{
			{Label: "Membiarkan"}, {Label: "Melapor", Points: points(4)},
		},
	})
	if weighted.Options[0].Points != nil || weighted.Options[1].Points == nil || *weighted.Options[1].Points != 4 {
		t.Errorf("weighted points = %+v, want the first absent and the second 4", weighted.Options)
	}

	// An isian answer that differs only in case is legal once the question is case-sensitive.
	if _, err := h.svc.Create(context.Background(), h.scope(tenant), h.principal(tenant, model.RoleAdmin), QuestionInput{
		ProgramID: h.program(tenant).ID, SubjectID: h.subject(tenant).ID,
		Type: model.QuestionIsian, Content: "Tulis nama ibukota Jawa Barat.", Difficulty: model.DifficultyEasy,
		Correct: []string{"Bandung", "bandung"}, CaseSensitive: true,
	}); err != nil {
		t.Errorf("case-sensitive isian with answers differing only in case: %v", err)
	}
}

func TestOptionIdsAreStableAcrossVersions(t *testing.T) {
	h := newQuestionHarness(t)
	ctx := context.Background()
	tenant := h.a
	created := h.create(t, tenant, QuestionInput{
		ProgramID: h.program(tenant).ID, SubjectID: h.subject(tenant).ID,
		Type: model.QuestionPG, Content: "Pilih yang benar.", Difficulty: model.DifficultyEasy,
		Options: []QuestionOptionInput{{ID: "a", Label: "Salah"}, {ID: "b", Label: "Benar"}},
		Correct: []string{"b"},
	})

	first := optionIDs(created)
	if len(first) != 2 || !isUUID(first[0]) || !isUUID(first[1]) || created.Correct[0] != first[1] {
		t.Fatalf("created ids = %v correct = %v, want two uuids with the second correct", first, created.Correct)
	}

	// A save that keeps both ids and adds one new option: the kept ids survive, the new one gets one.
	update := QuestionInput{
		ProgramID: h.program(tenant).ID, SubjectID: h.subject(tenant).ID,
		Type: model.QuestionPG, Content: "Pilih yang benar.", Difficulty: model.DifficultyEasy,
		BaseVersion: created.Version,
		Options: []QuestionOptionInput{
			{ID: first[0], Label: "Salah"}, {ID: first[1], Label: "Benar"}, {Label: "Mungkin"},
		},
		Correct: []string{first[1]},
	}
	updated, err := h.update(t, tenant, created.ID, update)
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	got := optionIDs(updated)
	if updated.Version != 2 || len(got) != 3 {
		t.Fatalf("version = %d options = %v, want version 2 with three options", updated.Version, got)
	}
	if got[0] != first[0] || got[1] != first[1] {
		t.Errorf("option ids = %v, want the first two unchanged %v", got, first)
	}
	if !isUUID(got[2]) || got[2] == first[0] || got[2] == first[1] {
		t.Errorf("new option id = %q, want a fresh uuid", got[2])
	}

	// An id that belongs to another question is refused, not silently replaced.
	other := h.createPG(t, tenant)
	foreign := QuestionInput{
		ProgramID: h.program(tenant).ID, SubjectID: h.subject(tenant).ID,
		Type: model.QuestionPG, Content: "Pilih yang benar.", Difficulty: model.DifficultyEasy,
		BaseVersion: updated.Version,
		Options: []QuestionOptionInput{
			{ID: optionIDs(other)[0], Label: "Salah"}, {ID: first[1], Label: "Benar"},
		},
		Correct: []string{first[1]},
	}
	_, err = h.update(t, tenant, created.ID, foreign)
	if err == nil {
		t.Fatal("update with another question's option id succeeded, want 422")
	}
	wantInputError(t, err, "options[0].id")

	// benar_salah keeps the fixed ids across versions too.
	trueFalse := h.create(t, tenant, QuestionInput{
		ProgramID: h.program(tenant).ID, SubjectID: h.subject(tenant).ID,
		Type: model.QuestionTrueFalse, Content: "Air mendidih pada 100 derajat.", Difficulty: model.DifficultyEasy,
		Correct: []string{"benar"},
	})
	reopened, err := h.svc.Get(ctx, h.scope(tenant), h.principal(tenant, model.RoleOperator), trueFalse.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	changed, err := h.update(t, tenant, reopened.ID, QuestionInput{
		ProgramID: h.program(tenant).ID, SubjectID: h.subject(tenant).ID,
		Type: model.QuestionTrueFalse, Content: "Air mendidih pada 90 derajat.", Difficulty: model.DifficultyEasy,
		BaseVersion: reopened.Version, Correct: []string{"salah"},
	})
	if err != nil {
		t.Fatalf("update benar_salah: %v", err)
	}
	if ids := optionIDs(changed); len(ids) != 2 || ids[0] != "benar" || ids[1] != "salah" || changed.Correct[0] != "salah" {
		t.Errorf("benar_salah after update = options %v correct %v, want the fixed ids", ids, changed.Correct)
	}
}

func TestUpdateCreatesNewVersionOnlyWhenChanged(t *testing.T) {
	h := newQuestionHarness(t)
	ctx := context.Background()
	tenant := h.a
	created := h.createPG(t, tenant)

	unchanged := QuestionInput{
		ProgramID: h.program(tenant).ID, SubjectID: h.subject(tenant).ID,
		Type: model.QuestionPG, Content: created.Content, Difficulty: created.Difficulty,
		Explanation: created.Explanation, BaseVersion: created.Version,
		Options: []QuestionOptionInput{
			{ID: created.Options[0].ID, Label: created.Options[0].Label},
			{ID: created.Options[1].ID, Label: created.Options[1].Label},
		},
		Correct: []string{created.Options[1].ID},
	}
	same, err := h.update(t, tenant, created.ID, unchanged)
	if err != nil {
		t.Fatalf("unchanged update: %v", err)
	}
	if same.Version != 1 || same.VersionID != created.VersionID {
		t.Errorf("unchanged save = version %d id %s, want the current version 1 %s untouched", same.Version, same.VersionID, created.VersionID)
	}
	if rows := versionCount(t, h, created.ID); rows != 1 {
		t.Errorf("version rows = %d after an unchanged save, want 1", rows)
	}

	changed := unchanged
	changed.Content = "Hasil dari 4 + 4 adalah ..."
	changed.BaseVersion = same.Version
	next, err := h.update(t, tenant, created.ID, changed)
	if err != nil {
		t.Fatalf("changed update: %v", err)
	}
	if next.Version != 2 || next.VersionID == created.VersionID {
		t.Errorf("changed save = version %d id %s, want a new version 2", next.Version, next.VersionID)
	}
	if ids := optionIDs(next); ids[0] != created.Options[0].ID || ids[1] != created.Options[1].ID {
		t.Errorf("option ids = %v, want them kept", ids)
	}
	if rows := versionCount(t, h, created.ID); rows != 2 {
		t.Errorf("version rows = %d after one change, want 2", rows)
	}

	// The old version's content is still readable in the database row that P08 would reference.
	var oldContent string
	if err := h.pool.QueryRow(ctx,
		`SELECT content FROM question_versions WHERE id = $1`, created.VersionID).Scan(&oldContent); err != nil {
		t.Fatalf("read the first version: %v", err)
	}
	if oldContent != created.Content {
		t.Errorf("first version content = %q, want the original %q", oldContent, created.Content)
	}
}

func TestStaleBaseVersionConflicts(t *testing.T) {
	h := newQuestionHarness(t)
	tenant := h.a
	created := h.createPG(t, tenant)

	change := QuestionInput{
		ProgramID: h.program(tenant).ID, SubjectID: h.subject(tenant).ID,
		Type: model.QuestionPG, Content: "Isi baru.", Difficulty: model.DifficultyEasy,
		BaseVersion: created.Version,
		Options: []QuestionOptionInput{
			{ID: created.Options[0].ID, Label: "3"}, {ID: created.Options[1].ID, Label: "4"},
		},
		Correct: []string{created.Options[1].ID},
	}
	first, err := h.update(t, tenant, created.ID, change)
	if err != nil {
		t.Fatalf("first update: %v", err)
	}
	if first.Version != 2 {
		t.Fatalf("first update version = %d, want 2", first.Version)
	}

	stale := change
	stale.Content = "Isi lain."
	stale.BaseVersion = created.Version
	if _, err := h.update(t, tenant, created.ID, stale); !errors.Is(err, ErrQuestionStale) {
		t.Fatalf("stale update: err = %v, want ErrQuestionStale", err)
	}

	current, err := h.svc.Get(context.Background(), h.scope(tenant), h.principal(tenant, model.RoleAdmin), created.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if current.Version != 2 || current.Content != "Isi baru." {
		t.Errorf("current = version %d %q, want the first update's version 2", current.Version, current.Content)
	}
	if rows := versionCount(t, h, created.ID); rows != 2 {
		t.Errorf("version rows = %d after the rejected update, want 2", rows)
	}

	// A base version the editor invented is a conflict too, not a silent write.
	unknown := change
	unknown.BaseVersion = 99
	if _, err := h.update(t, tenant, created.ID, unknown); !errors.Is(err, ErrQuestionStale) {
		t.Errorf("base_version 99: err = %v, want ErrQuestionStale", err)
	}
}

func TestConcurrentUpdatesExactlyOneWinsPerBaseVersion(t *testing.T) {
	h := newQuestionHarness(t)
	tenant := h.a
	created := h.createPG(t, tenant)

	const writers = 8
	start := make(chan struct{})
	results := make([]error, writers)
	var wg sync.WaitGroup
	for i := range writers {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			in := QuestionInput{
				ProgramID: h.program(tenant).ID, SubjectID: h.subject(tenant).ID,
				Type: model.QuestionPG, Content: fmt.Sprintf("Versi dari penulis %d", i),
				Difficulty: model.DifficultyEasy, BaseVersion: created.Version,
				Options: []QuestionOptionInput{
					{ID: created.Options[0].ID, Label: "3"}, {ID: created.Options[1].ID, Label: "4"},
				},
				Correct: []string{created.Options[1].ID},
			}
			<-start
			_, results[i] = h.svc.Update(context.Background(), h.scope(tenant), h.principal(tenant, model.RoleAdmin), created.ID, in)
		}(i)
	}
	close(start)
	wg.Wait()

	won, stale := 0, 0
	for i, err := range results {
		switch {
		case err == nil:
			won++
		case errors.Is(err, ErrQuestionStale):
			stale++
		default:
			t.Errorf("writer %d: err = %v, want either success or ErrQuestionStale", i, err)
		}
	}
	if won != 1 || stale != writers-1 {
		t.Fatalf("winners = %d, conflicts = %d; want exactly one winner and %d conflicts", won, stale, writers-1)
	}

	current, err := h.svc.Get(context.Background(), h.scope(tenant), h.principal(tenant, model.RoleAdmin), created.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if current.Version != 2 {
		t.Errorf("current version = %d, want exactly one step to version 2", current.Version)
	}
	if rows := versionCount(t, h, created.ID); rows != 2 {
		t.Errorf("version rows = %d, want a gapless 1..2", rows)
	}
}

func TestQuestionFilters(t *testing.T) {
	h := newQuestionHarness(t)
	tenant := h.a
	tree := h.tree[tenant.ID]
	math, physics := tree.Subjects[0], tree.Subjects[1]

	create := func(content, subject string, difficulty string, kind string) model.Question {
		t.Helper()
		subjectID := math.Subject.ID
		if subject == "Fisika" {
			subjectID = physics.Subject.ID
		}
		in := QuestionInput{
			ProgramID: tree.Program.ID, SubjectID: subjectID, Type: kind,
			Content: content, Difficulty: difficulty,
		}
		if kind == model.QuestionIsian {
			in.Correct = []string{"jawaban"}
		} else {
			in.Options = []QuestionOptionInput{{ID: "a", Label: "Satu"}, {ID: "b", Label: "Dua"}}
			in.Correct = []string{"a"}
		}
		return h.create(t, tenant, in)
	}

	first := create("Diskon 100% dari harga awal", "Matematika", model.DifficultyEasy, model.QuestionPG)
	second := create("Nilai a_b pada tabel", "Matematika", model.DifficultyHard, model.QuestionIsian)
	third := create("Nilai axb pada tabel", "Fisika", model.DifficultyHard, model.QuestionIsian)

	all, total := h.list(t, tenant, QuestionFilter{})
	if len(all) != 3 || total != 3 {
		t.Fatalf("unfiltered list = %d (total %d), want 3", len(all), total)
	}
	if all[0].Number < all[2].Number {
		t.Errorf("numbers = %d..%d, want newest (seq desc) first", all[0].Number, all[2].Number)
	}

	cases := []struct {
		name   string
		filter QuestionFilter
		want   []uuid.UUID
	}{
		{"by program", QuestionFilter{ProgramID: &tree.Program.ID}, []uuid.UUID{third.ID, second.ID, first.ID}},
		{"by subject", QuestionFilter{SubjectID: &math.Subject.ID}, []uuid.UUID{second.ID, first.ID}},
		{"by type", QuestionFilter{Type: model.QuestionIsian}, []uuid.UUID{third.ID, second.ID}},
		{"by difficulty", QuestionFilter{Difficulty: model.DifficultyHard}, []uuid.UUID{third.ID, second.ID}},
		// % and _ are literal: a wildcard would match all three.
		{"percent is literal", QuestionFilter{Search: "100%"}, []uuid.UUID{first.ID}},
		{"percent alone is literal", QuestionFilter{Search: "%"}, []uuid.UUID{first.ID}},
		{"underscore is literal", QuestionFilter{Search: "a_b"}, []uuid.UUID{second.ID}},
		{"no wildcard match", QuestionFilter{Search: "a_b", Type: model.QuestionPG}, nil},
		{"case-insensitive match", QuestionFilter{Search: "NILAI"}, []uuid.UUID{third.ID, second.ID}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, count := h.list(t, tenant, tc.filter)
			if len(got) != len(tc.want) || count != len(tc.want) {
				t.Fatalf("list = %d questions (total %d), want %d", len(got), count, len(tc.want))
			}
			for i, id := range tc.want {
				if got[i].ID != id {
					t.Errorf("question %d = %s, want %s", i, got[i].ID, id)
				}
			}
		})
	}

	page, count := h.list(t, tenant, QuestionFilter{Limit: 2})
	if len(page) != 2 || count != 3 {
		t.Errorf("limit 2 = %d questions (total %d), want 2 of 3", len(page), count)
	}
	next, _ := h.list(t, tenant, QuestionFilter{Limit: 2, Offset: 2})
	if len(next) != 1 || next[0].ID != first.ID {
		t.Errorf("offset 2 = %+v, want the oldest question", next)
	}

	for name, filter := range map[string]QuestionFilter{
		"limit above the cap": {Limit: maxQuestionLimit + 1},
		"negative limit":      {Limit: -1},
		"negative offset":     {Offset: -1},
		"unknown type":        {Type: "essay"},
		"unknown difficulty":  {Difficulty: "extreme"},
	} {
		if _, _, err := h.svc.List(context.Background(), h.scope(tenant), h.principal(tenant, model.RoleAdmin), filter); err == nil {
			t.Errorf("%s: list succeeded, want a validation error", name)
		}
	}
}

func TestQuestionTenantMismatchIsNotFound(t *testing.T) {
	h := newQuestionHarness(t)
	ctx := context.Background()
	other := h.createPG(t, h.b)

	if _, err := h.svc.Get(ctx, h.scope(h.a), h.principal(h.a, model.RoleAdmin), other.ID); !errors.Is(err, ErrQuestionNotFound) {
		t.Errorf("get another tenant's question: err = %v, want ErrQuestionNotFound", err)
	}
	input := h.pgInput(h.a)
	input.BaseVersion = other.Version
	if _, err := h.svc.Update(ctx, h.scope(h.a), h.principal(h.a, model.RoleAdmin), other.ID, input); !errors.Is(err, ErrQuestionNotFound) {
		t.Errorf("update another tenant's question: err = %v, want ErrQuestionNotFound", err)
	}
	if err := h.svc.Delete(ctx, h.scope(h.a), h.principal(h.a, model.RoleAdmin), other.ID); !errors.Is(err, ErrQuestionNotFound) {
		t.Errorf("delete another tenant's question: err = %v, want ErrQuestionNotFound", err)
	}

	// A principal of another tenant never reaches this host's bank, whatever the path says.
	if _, err := h.svc.Get(ctx, h.scope(h.a), h.principal(h.b, model.RoleAdmin), other.ID); !errors.Is(err, ErrTenantNotFound) {
		t.Errorf("get with another tenant's principal: err = %v, want ErrTenantNotFound", err)
	}

	if questions, total := h.list(t, h.a, QuestionFilter{}); len(questions) != 0 || total != 0 {
		t.Errorf("tenant A bank = %+v (total %d), want it empty", questions, total)
	}
}

func versionCount(t *testing.T, h questionHarness, questionID uuid.UUID) int {
	t.Helper()
	var rows int
	if err := h.pool.QueryRow(context.Background(),
		`SELECT count(*) FROM question_versions WHERE question_id = $1`, questionID).Scan(&rows); err != nil {
		t.Fatalf("count versions: %v", err)
	}
	return rows
}
