package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/tarunaeduka/bimbel-tryout/api/internal/model"
	"github.com/tarunaeduka/bimbel-tryout/api/internal/repository"
)

type tryoutHarness struct {
	svc        *TryoutService
	questions  *QuestionService
	curriculum *CurriculumService
	pool       *pgxpool.Pool
	clock      *testClock
	q          questionHarness
}

func newTryoutHarness(t *testing.T) tryoutHarness {
	t.Helper()
	q := newQuestionHarness(t)
	h := tryoutHarness{
		questions:  q.svc,
		curriculum: NewCurriculumService(q.pool, repository.NewCurriculumRepository(q.pool)),
		pool:       q.pool,
		clock:      newTestClock(),
		q:          q,
	}
	h.svc = NewTryoutService(q.pool, repository.NewTryoutRepository(q.pool), WithTryoutClock(h.clock.Now))
	// Registered last, so it runs before the question and curriculum cleanups.
	t.Cleanup(func() {
		ctx := context.Background()
		ids := []uuid.UUID{h.q.a.ID, h.q.b.ID}
		for _, sql := range []string{
			`DELETE FROM tryout_questions WHERE tenant_id = ANY($1)`,
			`DELETE FROM tryout_subtests WHERE tenant_id = ANY($1)`,
			`DELETE FROM tryouts WHERE tenant_id = ANY($1)`,
		} {
			if _, err := h.pool.Exec(ctx, sql, ids); err != nil {
				t.Errorf("cleanup tryout fixture: %v", err)
			}
		}
	})
	return h
}

func (h tryoutHarness) scope(tenant model.Tenant) HostScope { return h.q.scope(tenant) }

func (h tryoutHarness) principal(tenant model.Tenant, role model.Role) model.Principal {
	return h.q.principal(tenant, role)
}

func (h tryoutHarness) question(t *testing.T, tenant model.Tenant) model.Question {
	t.Helper()
	return h.q.createPG(t, tenant)
}

// tryoutInput is one valid draft: a future window and no subtests unless the caller adds them.
func (h tryoutHarness) tryoutInput(tenant model.Tenant, subtests ...TryoutSubtestInput) TryoutInput {
	until := h.clock.Now().Add(48 * time.Hour)
	return TryoutInput{
		ProgramID:   h.q.program(tenant).ID,
		Title:       "Tryout " + uuid.NewString()[:8],
		Description: "Latihan tryout",
		ScoringMode: model.ScoringWeighted,
		ActiveUntil: &until,
		Subtests:    subtests,
	}
}

func subtestInput(name string, seconds int, questions ...model.Question) TryoutSubtestInput {
	out := TryoutSubtestInput{Name: name, DurationSeconds: seconds}
	for _, question := range questions {
		out.Questions = append(out.Questions, TryoutQuestionInput{QuestionID: question.ID})
	}
	return out
}

func (h tryoutHarness) create(t *testing.T, tenant model.Tenant, input TryoutInput) model.Tryout {
	t.Helper()
	created, err := h.svc.Create(context.Background(), h.scope(tenant), h.principal(tenant, model.RoleAdmin), input)
	if err != nil {
		t.Fatalf("create tryout: %v", err)
	}
	return created
}

func (h tryoutHarness) createTryout(t *testing.T, tenant model.Tenant, input TryoutInput) (model.Tryout, error) {
	t.Helper()
	return h.svc.Create(context.Background(), h.scope(tenant), h.principal(tenant, model.RoleAdmin), input)
}

func (h tryoutHarness) update(tenant model.Tenant, id uuid.UUID, input TryoutInput) (model.Tryout, error) {
	return h.svc.Update(context.Background(), h.scope(tenant), h.principal(tenant, model.RoleAdmin), id, input)
}

func (h tryoutHarness) publish(tenant model.Tenant, id uuid.UUID) (model.Tryout, error) {
	return h.svc.Publish(context.Background(), h.scope(tenant), h.principal(tenant, model.RoleAdmin), id)
}

func (h tryoutHarness) unpublish(tenant model.Tenant, id uuid.UUID) (model.Tryout, error) {
	return h.svc.Unpublish(context.Background(), h.scope(tenant), h.principal(tenant, model.RoleAdmin), id)
}

func (h tryoutHarness) get(t *testing.T, tenant model.Tenant, id uuid.UUID) model.Tryout {
	t.Helper()
	tryout, err := h.svc.Get(context.Background(), h.scope(tenant), h.principal(tenant, model.RoleOperator), id)
	if err != nil {
		t.Fatalf("get tryout: %v", err)
	}
	return tryout
}

func wantTryoutField(t *testing.T, err error, paths ...string) {
	t.Helper()
	var input *InputError
	if !errors.As(err, &input) {
		t.Fatalf("err = %v, want an InputError naming %v", err, paths)
	}
	for _, path := range paths {
		if input.Fields[path] == "" {
			t.Errorf("no message for %s in %v", path, input.Fields)
		}
	}
}

func TestTryoutValidation(t *testing.T) {
	h := newTryoutHarness(t)
	tenant, other := h.q.a, h.q.b
	question := h.question(t, tenant)
	foreign := h.question(t, other)

	cases := []struct {
		name   string
		mutate func(*TryoutInput)
		path   string
	}{
		{"empty title", func(in *TryoutInput) { in.Title = "   " }, "title"},
		{"title too long", func(in *TryoutInput) { in.Title = strings.Repeat("a", 201) }, "title"},
		{"description too long", func(in *TryoutInput) { in.Description = strings.Repeat("a", 2001) }, "description"},
		{"unknown scoring mode", func(in *TryoutInput) { in.ScoringMode = "raw" }, "scoring_mode"},
		{"missing active_until", func(in *TryoutInput) { in.ActiveUntil = nil }, "active_until"},
		{"window reversed", func(in *TryoutInput) {
			at := in.ActiveUntil.Add(48 * time.Hour)
			in.ActiveFrom = &at
		}, "active_from"},
		{"too many subtests", func(in *TryoutInput) {
			for i := 0; i < maxSubtests+1; i++ {
				in.Subtests = append(in.Subtests, TryoutSubtestInput{Name: fmt.Sprint("S", i), DurationSeconds: 600})
			}
		}, "subtests"},
		{"too many questions in one subtest", func(in *TryoutInput) {
			questions := make([]TryoutQuestionInput, 0, maxQuestionsPerSubtest+1)
			for i := 0; i <= maxQuestionsPerSubtest; i++ {
				questions = append(questions, TryoutQuestionInput{QuestionID: uuid.New()})
			}
			in.Subtests = append(in.Subtests, TryoutSubtestInput{Name: "Besar", DurationSeconds: 600, Questions: questions})
		}, "subtests[0].questions"},
		{"too many questions in the tryout", func(in *TryoutInput) {
			for i := 0; i < 3; i++ {
				questions := make([]TryoutQuestionInput, 0, 170)
				for j := 0; j < 170; j++ {
					questions = append(questions, TryoutQuestionInput{QuestionID: uuid.New()})
				}
				in.Subtests = append(in.Subtests, TryoutSubtestInput{Name: fmt.Sprint("S", i), DurationSeconds: 600, Questions: questions})
			}
		}, "subtests"},
		{"subtest duration too short", func(in *TryoutInput) {
			in.Subtests = []TryoutSubtestInput{{Name: "Singkat", DurationSeconds: 59}}
		}, "subtests[0].duration_seconds"},
		{"subtest duration too long", func(in *TryoutInput) {
			in.Subtests = []TryoutSubtestInput{{Name: "Panjang", DurationSeconds: 28801}}
		}, "subtests[0].duration_seconds"},
		{"empty subtest name", func(in *TryoutInput) {
			in.Subtests = []TryoutSubtestInput{{Name: " ", DurationSeconds: 600}}
		}, "subtests[0].name"},
		{"another tenant's question", func(in *TryoutInput) {
			in.Subtests = []TryoutSubtestInput{subtestInput("Subtes", 600, foreign)}
		}, "subtests[0].questions[0].question_id"},
		{"missing question id", func(in *TryoutInput) {
			in.Subtests = []TryoutSubtestInput{{Name: "Subtes", DurationSeconds: 600,
				Questions: []TryoutQuestionInput{{QuestionID: uuid.New()}}}}
		}, "subtests[0].questions[0].question_id"},
		{"duplicate question", func(in *TryoutInput) {
			in.Subtests = []TryoutSubtestInput{
				subtestInput("Satu", 600, question),
				subtestInput("Dua", 600, question),
			}
		}, "subtests[1].questions[0].question_id"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			input := h.tryoutInput(tenant)
			tc.mutate(&input)
			_, err := h.createTryout(t, tenant, input)
			wantTryoutField(t, err, tc.path)
		})
	}

	t.Run("program of another tenant", func(t *testing.T) {
		input := h.tryoutInput(tenant)
		input.ProgramID = h.q.program(other).ID
		_, err := h.createTryout(t, tenant, input)
		wantTryoutField(t, err, "program_id")
	})

	t.Run("unknown program", func(t *testing.T) {
		input := h.tryoutInput(tenant)
		input.ProgramID = uuid.New()
		_, err := h.createTryout(t, tenant, input)
		wantTryoutField(t, err, "program_id")
	})
}

func TestPublishValidation(t *testing.T) {
	h := newTryoutHarness(t)
	tenant := h.q.a
	question := h.question(t, tenant)

	t.Run("no subtest", func(t *testing.T) {
		created := h.create(t, tenant, h.tryoutInput(tenant))
		_, err := h.publish(tenant, created.ID)
		wantTryoutField(t, err, "subtests")
	})

	t.Run("empty subtest", func(t *testing.T) {
		created := h.create(t, tenant, h.tryoutInput(tenant,
			TryoutSubtestInput{Name: "Kosong", DurationSeconds: 600}))
		_, err := h.publish(tenant, created.ID)
		wantTryoutField(t, err, "subtests[0].questions")
	})

	t.Run("window already over", func(t *testing.T) {
		input := h.tryoutInput(tenant, subtestInput("Subtes", 600, question))
		until := h.clock.Now().Add(time.Hour)
		input.ActiveUntil = &until
		created := h.create(t, tenant, input)
		h.clock.Advance(2 * time.Hour)
		_, err := h.publish(tenant, created.ID)
		wantTryoutField(t, err, "active_until")
	})

	t.Run("publish is idempotent and unpublish returns the draft", func(t *testing.T) {
		input := h.tryoutInput(tenant, subtestInput("Subtes", 600, question))
		created := h.create(t, tenant, input)
		published, err := h.publish(tenant, created.ID)
		if err != nil {
			t.Fatalf("publish: %v", err)
		}
		if published.Status != model.TryoutPublished {
			t.Errorf("status = %q, want published", published.Status)
		}
		again, err := h.publish(tenant, created.ID)
		if err != nil {
			t.Fatalf("publish an already published tryout: %v", err)
		}
		if again.Status != model.TryoutPublished || again.Revision != published.Revision {
			t.Errorf("second publish = %+v, want the same published tryout", again)
		}
		draft, err := h.unpublish(tenant, created.ID)
		if err != nil {
			t.Fatalf("unpublish: %v", err)
		}
		if draft.Status != model.TryoutDraft {
			t.Errorf("status = %q after unpublish, want draft", draft.Status)
		}
	})

	t.Run("published tryout is re-validated on save", func(t *testing.T) {
		input := h.tryoutInput(tenant, subtestInput("Subtes", 600, question))
		created := h.create(t, tenant, input)
		if _, err := h.publish(tenant, created.ID); err != nil {
			t.Fatalf("publish: %v", err)
		}
		empty := h.tryoutInput(tenant)
		empty.BaseRevision = created.Revision
		_, err := h.update(tenant, created.ID, empty)
		wantTryoutField(t, err, "subtests")
	})
}

func TestAssignmentFreezesTheQuestionVersion(t *testing.T) {
	h := newTryoutHarness(t)
	tenant := h.q.a
	question := h.question(t, tenant)
	created := h.create(t, tenant, h.tryoutInput(tenant, subtestInput("Subtes", 600, question)))

	frozen := h.get(t, tenant, created.ID).Subtests[0].Questions[0]
	if frozen.Version != 1 || frozen.Content != question.Content {
		t.Fatalf("assignment = version %d %q, want version 1 with the original content", frozen.Version, frozen.Content)
	}

	edited := h.q.pgInput(tenant)
	edited.Content = "Isi soal yang sudah berubah."
	edited.BaseVersion = question.Version
	updated, err := h.q.update(t, tenant, question.ID, edited)
	if err != nil {
		t.Fatalf("edit the question through P07a: %v", err)
	}
	if updated.Version != 2 {
		t.Fatalf("question version = %d after the edit, want 2", updated.Version)
	}

	reloaded := h.get(t, tenant, created.ID).Subtests[0].Questions[0]
	if reloaded.Version != 1 || reloaded.VersionID != frozen.VersionID || reloaded.Content != question.Content {
		t.Errorf("assignment after the edit = version %d %q, want the frozen version 1 %q",
			reloaded.Version, reloaded.Content, question.Content)
	}

	// Saving the tryout again with the version the API returned keeps it, even though v2 is current.
	saved := h.tryoutInput(tenant)
	saved.BaseRevision = created.Revision
	versionID := frozen.VersionID
	saved.Subtests = []TryoutSubtestInput{{Name: "Subtes", DurationSeconds: 600,
		Questions: []TryoutQuestionInput{{QuestionID: question.ID, VersionID: &versionID}}}}
	kept, err := h.update(tenant, created.ID, saved)
	if err != nil {
		t.Fatalf("save the tryout with the frozen version: %v", err)
	}
	if got := kept.Subtests[0].Questions[0]; got.Version != 1 || got.VersionID != frozen.VersionID || got.Content != question.Content {
		t.Errorf("saved assignment = version %d %q, want the kept frozen version", got.Version, got.Content)
	}

	// A question new to the tryout takes its current version, which is v2 after this edit.
	fresh := h.question(t, tenant)
	freshEdit := h.q.pgInput(tenant)
	freshEdit.Content = "Soal baru yang sudah direvisi."
	freshEdit.BaseVersion = fresh.Version
	current, err := h.q.update(t, tenant, fresh.ID, freshEdit)
	if err != nil {
		t.Fatalf("edit the new question: %v", err)
	}
	added := h.tryoutInput(tenant)
	added.BaseRevision = kept.Revision
	added.Subtests = []TryoutSubtestInput{
		{Name: "Subtes", DurationSeconds: 600, Questions: []TryoutQuestionInput{
			{QuestionID: question.ID, VersionID: &versionID},
			{QuestionID: fresh.ID},
		}},
	}
	withNew, err := h.update(tenant, created.ID, added)
	if err != nil {
		t.Fatalf("assign a new question: %v", err)
	}
	got := withNew.Subtests[0].Questions[1]
	if got.Version != 2 || got.VersionID != current.VersionID || got.Content != freshEdit.Content {
		t.Errorf("new assignment = version %d %q, want the current version 2 %q", got.Version, got.Content, freshEdit.Content)
	}
}

func TestEditingAFrozenQuestionNeverChangesAssignedContent(t *testing.T) {
	h := newTryoutHarness(t)
	tenant := h.q.a
	question := h.question(t, tenant)
	created := h.create(t, tenant, h.tryoutInput(tenant, subtestInput("Subtes", 600, question)))
	frozen := h.get(t, tenant, created.ID).Subtests[0].Questions[0]

	for i := 2; i <= 4; i++ {
		next := h.q.pgInput(tenant)
		next.Content = fmt.Sprintf("Revisi ke-%d.", i)
		next.BaseVersion = i - 1
		if _, err := h.q.update(t, tenant, question.ID, next); err != nil {
			t.Fatalf("edit %d: %v", i, err)
		}
	}
	current, err := h.q.svc.Get(context.Background(), h.scope(tenant), h.principal(tenant, model.RoleAdmin), question.ID)
	if err != nil {
		t.Fatalf("read the edited question: %v", err)
	}
	if current.Version != 4 {
		t.Fatalf("question version = %d, want 4", current.Version)
	}

	reloaded := h.get(t, tenant, created.ID).Subtests[0].Questions[0]
	if reloaded.VersionID != frozen.VersionID || reloaded.Version != 1 || reloaded.Content != frozen.Content {
		t.Errorf("assignment = %+v, want the untouched frozen version 1", reloaded)
	}

	var stored uuid.UUID
	if err := h.pool.QueryRow(context.Background(),
		`SELECT question_version_id FROM tryout_questions WHERE tryout_id = $1 AND question_id = $2`,
		created.ID, question.ID).Scan(&stored); err != nil {
		t.Fatalf("read the stored assignment: %v", err)
	}
	if stored != frozen.VersionID {
		t.Errorf("stored version = %s, want the frozen %s", stored, frozen.VersionID)
	}
}

func TestAssignmentOfArbitraryVersionIsRejected(t *testing.T) {
	h := newTryoutHarness(t)
	tenant, other := h.q.a, h.q.b
	first := h.question(t, tenant)
	second := h.question(t, tenant)
	foreign := h.question(t, other)

	created := h.create(t, tenant, h.tryoutInput(tenant,
		subtestInput("Satu", 600, first),
		subtestInput("Dua", 600, second),
	))
	otherTryout := h.create(t, tenant, h.tryoutInput(tenant, subtestInput("Lain", 600, second)))

	cases := []struct {
		name     string
		question model.Question
		version  uuid.UUID
	}{
		{"an arbitrary id", first, uuid.New()},
		{"another question's version", first, second.VersionID},
		{"a version from another tryout", first, otherTryout.Subtests[0].Questions[0].VersionID},
		{"a version of another tenant", first, foreign.VersionID},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			input := h.tryoutInput(tenant)
			input.BaseRevision = created.Revision
			versionID := tc.version
			input.Subtests = []TryoutSubtestInput{{Name: "Satu", DurationSeconds: 600,
				Questions: []TryoutQuestionInput{{QuestionID: tc.question.ID, VersionID: &versionID}}}}
			_, err := h.update(tenant, created.ID, input)
			wantTryoutField(t, err, "subtests[0].questions[0].version_id")
		})
	}

	reloaded := h.get(t, tenant, created.ID)
	if reloaded.Revision != created.Revision || reloaded.QuestionCount != 2 {
		t.Errorf("tryout = revision %d with %d questions, want the untouched original", reloaded.Revision, reloaded.QuestionCount)
	}
}

func TestLockedTryoutRefusesEverything(t *testing.T) {
	h := newTryoutHarness(t)
	tenant := h.q.a
	question := h.question(t, tenant)
	created := h.create(t, tenant, h.tryoutInput(tenant, subtestInput("Subtes", 600, question)))
	if _, err := h.publish(tenant, created.ID); err != nil {
		t.Fatalf("publish: %v", err)
	}
	if _, err := h.pool.Exec(context.Background(),
		`UPDATE tryouts SET first_attempt_at = now() WHERE id = $1`, created.ID); err != nil {
		t.Fatalf("stamp first_attempt_at: %v", err)
	}

	input := h.tryoutInput(tenant, subtestInput("Baru", 900, question))
	input.BaseRevision = created.Revision
	if _, err := h.update(tenant, created.ID, input); !errors.Is(err, ErrTryoutLocked) {
		t.Errorf("PUT a locked tryout: err = %v, want ErrTryoutLocked", err)
	}
	if err := h.svc.Delete(context.Background(), h.scope(tenant), h.principal(tenant, model.RoleAdmin), created.ID); !errors.Is(err, ErrTryoutLocked) {
		t.Errorf("DELETE a locked tryout: err = %v, want ErrTryoutLocked", err)
	}
	if _, err := h.publish(tenant, created.ID); !errors.Is(err, ErrTryoutLocked) {
		t.Errorf("publish a locked tryout: err = %v, want ErrTryoutLocked", err)
	}
	if _, err := h.unpublish(tenant, created.ID); !errors.Is(err, ErrTryoutLocked) {
		t.Errorf("unpublish a locked tryout: err = %v, want ErrTryoutLocked", err)
	}

	var title, status string
	var revision, subtests, assignments int
	if err := h.pool.QueryRow(context.Background(), `
		SELECT t.title, t.status, t.revision,
		       (SELECT count(*) FROM tryout_subtests s WHERE s.tryout_id = t.id AND s.tenant_id = t.tenant_id),
		       (SELECT count(*) FROM tryout_questions q WHERE q.tryout_id = t.id AND q.tenant_id = t.tenant_id)
		  FROM tryouts t WHERE t.id = $1`, created.ID).Scan(&title, &status, &revision, &subtests, &assignments); err != nil {
		t.Fatalf("read the locked tryout: %v", err)
	}
	if title != created.Title || status != model.TryoutPublished || revision != created.Revision || subtests != 1 || assignments != 1 {
		t.Errorf("locked tryout = %q %q revision %d %d subtests %d questions, want the original untouched",
			title, status, revision, subtests, assignments)
	}
}

func TestStaleBaseRevisionConflicts(t *testing.T) {
	h := newTryoutHarness(t)
	tenant := h.q.a
	question := h.question(t, tenant)
	created := h.create(t, tenant, h.tryoutInput(tenant, subtestInput("Subtes", 600, question)))

	input := h.tryoutInput(tenant, subtestInput("Baru", 900, question))
	input.BaseRevision = created.Revision + 5
	if _, err := h.update(tenant, created.ID, input); !errors.Is(err, ErrTryoutStale) {
		t.Fatalf("PUT with a stale base_revision: err = %v, want ErrTryoutStale", err)
	}

	reloaded := h.get(t, tenant, created.ID)
	if reloaded.Revision != created.Revision || reloaded.Subtests[0].Name != "Subtes" {
		t.Errorf("tryout = %+v, want the untouched original", reloaded)
	}
}

func TestConcurrentUpdatesExactlyOneWinsPerBaseRevision(t *testing.T) {
	h := newTryoutHarness(t)
	tenant := h.q.a
	question := h.question(t, tenant)
	created := h.create(t, tenant, h.tryoutInput(tenant, subtestInput("Subtes", 600, question)))

	const writers = 6
	titles := make([]string, writers)
	errs := make([]error, writers)
	start := make(chan struct{})
	var wg sync.WaitGroup
	for i := 0; i < writers; i++ {
		titles[i] = fmt.Sprintf("Penulis %d", i)
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			input := h.tryoutInput(tenant, subtestInput("Subtes", 600))
			input.Title = titles[i]
			input.BaseRevision = created.Revision
			_, errs[i] = h.update(tenant, created.ID, input)
		}(i)
	}
	close(start)
	wg.Wait()

	won, stale := 0, 0
	for i, err := range errs {
		switch {
		case err == nil:
			won++
		case errors.Is(err, ErrTryoutStale):
			stale++
		default:
			t.Errorf("writer %d: err = %v, want nil or ErrTryoutStale", i, err)
		}
	}
	if won != 1 || stale != writers-1 {
		t.Fatalf("winners = %d stale = %d, want exactly one winner and %d stale", won, stale, writers-1)
	}

	reloaded := h.get(t, tenant, created.ID)
	if reloaded.Revision != created.Revision+1 {
		t.Errorf("revision = %d, want the single %d", reloaded.Revision, created.Revision+1)
	}
	winner := false
	for _, title := range titles {
		if reloaded.Title == title {
			winner = true
		}
	}
	if !winner {
		t.Errorf("title = %q, want one of the writers' titles", reloaded.Title)
	}
}

func TestStampingFirstAttemptBlocksAConcurrentUpdate(t *testing.T) {
	h := newTryoutHarness(t)
	tenant := h.q.a
	ctx := context.Background()
	question := h.question(t, tenant)
	created := h.create(t, tenant, h.tryoutInput(tenant, subtestInput("Subtes", 600, question)))

	tx, err := h.pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if _, err := h.svc.repo.LockTx(ctx, tx, tenant.ID, created.ID); err != nil {
		t.Fatalf("lock the tryout: %v", err)
	}
	if _, err := tx.Exec(ctx, `UPDATE tryouts SET first_attempt_at = now() WHERE id = $1`, created.ID); err != nil {
		t.Fatalf("stamp first_attempt_at: %v", err)
	}

	done := make(chan error, 1)
	go func() {
		input := h.tryoutInput(tenant, subtestInput("Baru", 900, question))
		input.BaseRevision = created.Revision
		_, err := h.update(tenant, created.ID, input)
		done <- err
	}()

	// The update must already wait on the row lock when the stamp commits, or the test proves nothing.
	waitForTryoutLockWaiter(t, h.pool)
	if err := tx.Commit(ctx); err != nil {
		t.Fatalf("commit the stamp: %v", err)
	}

	if err := <-done; !errors.Is(err, ErrTryoutLocked) {
		t.Fatalf("waiting PUT after the stamp committed: err = %v, want ErrTryoutLocked", err)
	}

	var revision int
	if err := h.pool.QueryRow(ctx, `SELECT revision FROM tryouts WHERE id = $1`, created.ID).Scan(&revision); err != nil {
		t.Fatalf("read revision: %v", err)
	}
	if revision != created.Revision {
		t.Errorf("revision = %d, want the untouched %d: the waiting update must not write", revision, created.Revision)
	}
}

// waitForTryoutLockWaiter spins on pg_stat_activity until a statement waits on the tryout row lock.
func waitForTryoutLockWaiter(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		var waiting int
		err := pool.QueryRow(context.Background(), `
			SELECT count(*) FROM pg_stat_activity
			 WHERE datname = current_database()
			   AND state = 'active'
			   AND wait_event_type = 'Lock'
			   AND query LIKE '%FROM tryouts%'
			   AND query LIKE '%FOR UPDATE%'`).Scan(&waiting)
		if err != nil {
			t.Fatalf("read pg_stat_activity: %v", err)
		}
		if waiting > 0 {
			return
		}
	}
	t.Fatal("no statement ever waited on the tryout row lock")
}

func TestReplaceAllKeepsOrder(t *testing.T) {
	h := newTryoutHarness(t)
	tenant := h.q.a
	first := h.question(t, tenant)
	second := h.question(t, tenant)
	third := h.question(t, tenant)

	created := h.create(t, tenant, h.tryoutInput(tenant,
		subtestInput("Pertama", 600, first, second),
		subtestInput("Kedua", 900, third),
	))
	input := h.tryoutInput(tenant,
		subtestInput("Kedua", 900, third),
		subtestInput("Pertama", 600, second, first),
	)
	input.BaseRevision = created.Revision
	from := h.clock.Now().Add(time.Hour)
	release := h.clock.Now().Add(72 * time.Hour)
	input.ActiveFrom = &from
	input.ExplanationReleaseAt = &release
	input.RandomizeQuestions = true
	input.RandomizeOptions = true
	updated, err := h.update(tenant, created.ID, input)
	if err != nil {
		t.Fatalf("replace the tryout: %v", err)
	}

	if len(updated.Subtests) != 2 || updated.Subtests[0].Name != "Kedua" || updated.Subtests[1].Name != "Pertama" {
		t.Fatalf("subtests = %+v, want Kedua then Pertama", updated.Subtests)
	}
	if updated.Subtests[0].Position != 1 || updated.Subtests[1].Position != 2 {
		t.Errorf("positions = %d,%d, want 1,2", updated.Subtests[0].Position, updated.Subtests[1].Position)
	}
	questions := updated.Subtests[1].Questions
	if len(questions) != 2 || questions[0].QuestionID != second.ID || questions[1].QuestionID != first.ID {
		t.Errorf("questions = %+v, want second then first", questions)
	}
	if questions[0].Position != 1 || questions[1].Position != 2 {
		t.Errorf("question positions = %d,%d, want 1,2", questions[0].Position, questions[1].Position)
	}

	reloaded := h.get(t, tenant, created.ID)
	if reloaded.Subtests[0].Name != "Kedua" || reloaded.Subtests[1].Questions[0].QuestionID != second.ID {
		t.Errorf("reload = %+v, want the same order after a reload", reloaded.Subtests)
	}
	if reloaded.ActiveFrom == nil || !reloaded.ActiveFrom.Equal(from) || !reloaded.ActiveUntil.Equal(*input.ActiveUntil) {
		t.Errorf("window = %v..%v, want it to survive the reload", reloaded.ActiveFrom, reloaded.ActiveUntil)
	}
	if reloaded.ExplanationReleaseAt == nil || !reloaded.ExplanationReleaseAt.Equal(release) {
		t.Errorf("explanation release = %v, want %v", reloaded.ExplanationReleaseAt, release)
	}
	if !reloaded.RandomizeQuestions || !reloaded.RandomizeOptions {
		t.Errorf("randomize flags = %v/%v, want both true after the reload", reloaded.RandomizeQuestions, reloaded.RandomizeOptions)
	}
	if reloaded.Subtests[1].DurationSeconds != 600 {
		t.Errorf("duration = %d, want 600 seconds", reloaded.Subtests[1].DurationSeconds)
	}
}

func TestTryoutFilters(t *testing.T) {
	h := newTryoutHarness(t)
	tenant := h.q.a
	question := h.question(t, tenant)

	tree, err := h.curriculum.Create(context.Background(), h.scope(tenant), h.principal(tenant, model.RoleAdmin),
		ProgramInput{Code: "TF", Name: "Program Filter"})
	if err != nil {
		t.Fatalf("create the second program: %v", err)
	}

	matematika := h.tryoutInput(tenant, subtestInput("Subtes", 600, question))
	matematika.Title = "Matematika Dasar"
	h.create(t, tenant, matematika)

	percent := h.tryoutInput(tenant, subtestInput("Subtes", 600, question))
	percent.Title = "Tryout 100% Matematika"
	published := h.create(t, tenant, percent)
	if _, err := h.publish(tenant, published.ID); err != nil {
		t.Fatalf("publish: %v", err)
	}

	underscore := h.tryoutInput(tenant)
	underscore.Title = "Tryout_A"
	h.create(t, tenant, underscore)

	elsewhere := h.tryoutInput(tenant)
	elsewhere.Title = "Program Lain"
	elsewhere.ProgramID = tree.Program.ID
	h.create(t, tenant, elsewhere)

	const total = 4

	cases := []struct {
		name   string
		filter TryoutFilter
		want   int
	}{
		{"all", TryoutFilter{}, total},
		{"by program", TryoutFilter{ProgramID: &tree.Program.ID}, 1},
		{"by status published", TryoutFilter{Status: model.TryoutPublished}, 1},
		{"by status draft", TryoutFilter{Status: model.TryoutDraft}, total - 1},
		{"literal percent", TryoutFilter{Search: "100%"}, 1},
		{"literal underscore", TryoutFilter{Search: "Tryout_"}, 1},
		{"case insensitive", TryoutFilter{Search: "matematika"}, 2},
		{"no match", TryoutFilter{Search: "tidak ada"}, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			filter := tc.filter
			filter.Limit = defaultTryoutLimit
			rows, got, err := h.svc.List(context.Background(), h.scope(tenant), h.principal(tenant, model.RoleOperator), filter)
			if err != nil {
				t.Fatalf("list: %v", err)
			}
			if got != tc.want || len(rows) != tc.want {
				t.Errorf("total = %d rows = %d, want %d", got, len(rows), tc.want)
			}
		})
	}

	all, _, err := h.svc.List(context.Background(), h.scope(tenant), h.principal(tenant, model.RoleOperator),
		TryoutFilter{Limit: defaultTryoutLimit})
	if err != nil {
		t.Fatalf("list all: %v", err)
	}
	page, totalCount, err := h.svc.List(context.Background(), h.scope(tenant), h.principal(tenant, model.RoleOperator),
		TryoutFilter{Limit: 2, Offset: 1})
	if err != nil {
		t.Fatalf("paginate: %v", err)
	}
	if len(page) != 2 || totalCount != total {
		t.Errorf("page = %d rows total = %d, want 2 of %d", len(page), totalCount, total)
	}
	if page[0].ID != all[1].ID || page[1].ID != all[2].ID {
		t.Errorf("page = %s,%s, want the same order as the full list from offset 1", page[0].ID, page[1].ID)
	}

	if _, _, err := h.svc.List(context.Background(), h.scope(tenant), h.principal(tenant, model.RoleOperator),
		TryoutFilter{Limit: 101}); err == nil {
		t.Error("limit 101 was accepted, want a validation error")
	}
}

func TestQuestionDeleteBlockedByAssignment(t *testing.T) {
	h := newTryoutHarness(t)
	tenant := h.q.a
	question := h.question(t, tenant)
	h.create(t, tenant, h.tryoutInput(tenant, subtestInput("Subtes", 600, question)))

	err := h.questions.Delete(context.Background(), h.scope(tenant), h.principal(tenant, model.RoleAdmin), question.ID)
	if !errors.Is(err, ErrQuestionReferenced) {
		t.Fatalf("delete an assigned question: err = %v, want ErrQuestionReferenced", err)
	}
	if _, err := h.questions.Get(context.Background(), h.scope(tenant), h.principal(tenant, model.RoleAdmin), question.ID); err != nil {
		t.Errorf("question after the rejected delete: %v, want it still there", err)
	}
}

func TestProgramDeleteBlockedByTryout(t *testing.T) {
	h := newTryoutHarness(t)
	tenant := h.q.a
	question := h.question(t, tenant)
	tree, err := h.curriculum.Create(context.Background(), h.scope(tenant), h.principal(tenant, model.RoleAdmin),
		ProgramInput{Code: "TB", Name: "Program Tryout", Subjects: []SubjectInput{{
			Name: "Matematika", Chapters: []ChapterInput{{Name: "Bilangan"}},
		}}})
	if err != nil {
		t.Fatalf("create the program: %v", err)
	}
	input := h.tryoutInput(tenant, subtestInput("Subtes", 600, question))
	input.ProgramID = tree.Program.ID
	h.create(t, tenant, input)

	err = h.curriculum.Delete(context.Background(), h.scope(tenant), h.principal(tenant, model.RoleAdmin), tree.Program.ID)
	if !errors.Is(err, ErrProgramReferenced) {
		t.Fatalf("delete a program a tryout uses: err = %v, want ErrProgramReferenced", err)
	}
	if _, err := h.curriculum.List(context.Background(), h.scope(tenant), h.principal(tenant, model.RoleAdmin)); err != nil {
		t.Fatalf("list the curriculum after the rejected delete: %v", err)
	}
}
