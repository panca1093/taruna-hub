package service

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/tarunaeduka/bimbel-tryout/api/internal/model"
	"github.com/tarunaeduka/bimbel-tryout/api/internal/repository"
)

const (
	kecKolomSeconds = 120
	kecKolomCount   = 10
	kecSoalCount    = 50
	kecSlot         = kecKolomSeconds + KecermatanPauseSeconds
)

type kecHarness struct {
	orderHarness
	kec  *KecermatanService
	rt   *KecermatanRuntimeService
	repo *repository.KecermatanRuntimeRepository
}

func newKecHarness(t *testing.T) kecHarness {
	t.Helper()
	base := newOrderHarness(t, noJobs{}, testBank)
	testRepo := repository.NewKecermatanRepository(base.pool)
	h := kecHarness{orderHarness: base, repo: repository.NewKecermatanRuntimeRepository(base.pool)}
	h.kec = NewKecermatanService(base.pool, testRepo, repository.NewItemPackageRepository(base.pool))
	var nonces atomic.Int64
	h.rt = NewKecermatanRuntimeService(base.pool, h.repo, h.kec, testRepo, WithKecermatanRuntimeClock(base.clock.Now),
		WithKecermatanNonce(func() ([]byte, error) {
			n := nonces.Add(1)
			return []byte{byte(n), byte(n >> 8), 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7}, nil
		}))
	// Registered after the order fixture's cleanup, so it runs first and clears what points at the tests.
	t.Cleanup(func() {
		ctx := context.Background()
		ids := []uuid.UUID{h.t.q.a.ID, h.t.q.b.ID}
		for _, sql := range []string{
			`DELETE FROM kecermatan_results WHERE tenant_id = ANY($1)`,
			`DELETE FROM kecermatan_session_answers WHERE tenant_id = ANY($1)`,
			`DELETE FROM kecermatan_session_kolom WHERE tenant_id = ANY($1)`,
			`DELETE FROM kecermatan_sessions WHERE tenant_id = ANY($1)`,
			`DELETE FROM enrollment_grants WHERE tenant_id = ANY($1) AND enrollment_id IN (SELECT id FROM enrollments WHERE kecermatan_test_id IS NOT NULL)`,
			`DELETE FROM enrollments WHERE tenant_id = ANY($1) AND kecermatan_test_id IS NOT NULL`,
			`DELETE FROM orders WHERE tenant_id = ANY($1) AND kecermatan_test_id IS NOT NULL`,
			`DELETE FROM products WHERE tenant_id = ANY($1) AND type = 'kecermatan'`,
			`DELETE FROM kecermatan_tests WHERE tenant_id = ANY($1)`,
		} {
			if _, err := base.pool.Exec(ctx, sql, ids); err != nil {
				t.Errorf("cleanup kecermatan fixture: %v", err)
			}
		}
	})
	return h
}

func (h kecHarness) testIn(t *testing.T, tenant model.Tenant) model.KecermatanTest {
	t.Helper()
	test, err := h.kec.Create(context.Background(), h.scope(tenant), h.staff(tenant, model.RoleAdmin), KecermatanInput{
		ProgramID: h.t.q.program(tenant).ID, Title: "Kecermatan " + uuid.NewString()[:8], ItemType: model.KecChar,
		ItemLength: 2, OptionsPerSoal: 5, SoalPerKolom: kecSoalCount, KolomCount: kecKolomCount,
		KolomDurationSeconds: kecKolomSeconds, CoverFrom: "#f59e0b", CoverTo: "#ef4444",
	})
	if err != nil {
		t.Fatalf("create kecermatan test: %v", err)
	}
	return test
}

// holder is a student who bought credits for a test through the real issuance path.
func (h kecHarness) holder(t *testing.T, tenant model.Tenant, test model.KecermatanTest, credits int) (model.User, model.Principal) {
	t.Helper()
	user, p := h.student(t, tenant, "")
	h.grant(t, tenant, user, test, credits)
	return user, p
}

func (h kecHarness) grant(t *testing.T, tenant model.Tenant, user model.User, test model.KecermatanTest, credits int) {
	t.Helper()
	source := uuid.NewString()
	author := h.t.q.author[tenant.ID].ID
	err := h.inTx(func(tx pgx.Tx) error {
		_, err := h.svc.IssueEnrollmentsTx(context.Background(), tx, IssueInput{
			TenantID: tenant.ID, UserID: user.ID, KecermatanTestID: &test.ID, Attempts: credits,
			SourceType: model.GrantSourceStaff, SourceID: source, ActorKind: model.ActorStaff,
			ActorID: &author, Reason: "fixture",
		})
		return err
	})
	if err != nil {
		t.Fatalf("grant credits: %v", err)
	}
}

type creditRow struct{ purchased, used int }

func (h kecHarness) credit(t *testing.T, user model.User, test model.KecermatanTest) creditRow {
	t.Helper()
	var c creditRow
	if err := h.pool.QueryRow(context.Background(),
		`SELECT attempts_purchased, attempts_used FROM enrollments WHERE user_id = $1 AND kecermatan_test_id = $2`,
		user.ID, test.ID).Scan(&c.purchased, &c.used); err != nil {
		t.Fatalf("read credit: %v", err)
	}
	return c
}

func (h kecHarness) start(t *testing.T, tenant model.Tenant, p model.Principal, test model.KecermatanTest) (KecSessionView, bool) {
	t.Helper()
	view, created, err := h.rt.Start(context.Background(), h.scope(tenant), p, test.ID)
	if err != nil {
		t.Fatalf("start attempt: %v", err)
	}
	return view, created
}

func (h kecHarness) session(t *testing.T, tenant model.Tenant, p model.Principal, id uuid.UUID) KecSessionView {
	t.Helper()
	view, err := h.rt.Session(context.Background(), h.scope(tenant), p, id)
	if err != nil {
		t.Fatalf("read attempt: %v", err)
	}
	return view
}

func (h kecHarness) generation(t *testing.T, sess model.KecSession) KecGeneration {
	t.Helper()
	raw, err := h.repo.Columns(context.Background(), h.pool, sess.TenantID, sess.ID)
	if err != nil {
		t.Fatalf("read snapshot: %v", err)
	}
	var columns []KecColumn
	if err := json.Unmarshal(raw, &columns); err != nil {
		t.Fatalf("decode snapshot: %v", err)
	}
	return KecGeneration{KolomCount: sess.KolomCount, Columns: columns}
}

// answer sends the key (right) or a deliberately wrong label for soal 1..n of the running kolom.
func (h kecHarness) answer(t *testing.T, tenant model.Tenant, p model.Principal, sess model.KecSession, kolom, from, to int, right bool) {
	t.Helper()
	gen := h.generation(t, sess)
	column, _ := gen.Slot(kolom)
	for soal := from; soal <= to; soal++ {
		label := column.Soal[soal-1].Missing
		if !right {
			label = "A"
			if column.Soal[soal-1].Missing == "A" {
				label = "B"
			}
		}
		if _, err := h.rt.Answer(context.Background(), h.scope(tenant), p, sess.ID, kolom, soal, &label); err != nil {
			t.Fatalf("answer kolom %d soal %d: %v", kolom, soal, err)
		}
	}
}

func TestKecermatanAttemptSeedDerivation(t *testing.T) {
	testSeed := "00112233445566778899aabbccddeeff"
	a, err := deriveKecermatanAttemptSeed(testSeed, []byte("nonce-one-123456"))
	if err != nil {
		t.Fatalf("derive: %v", err)
	}
	again, _ := deriveKecermatanAttemptSeed(testSeed, []byte("nonce-one-123456"))
	other, _ := deriveKecermatanAttemptSeed(testSeed, []byte("nonce-two-123456"))
	if a != again || a == other || a == testSeed || !validKecermatanSeed(a) {
		t.Errorf("seeds = %q %q %q, want a stable valid seed that differs per nonce and from the test seed", a, again, other)
	}
	foreign, _ := deriveKecermatanAttemptSeed("ffeeddccbbaa99887766554433221100", []byte("nonce-one-123456"))
	if foreign == a {
		t.Error("two test seeds derived the same attempt seed")
	}
	if _, err := deriveKecermatanAttemptSeed("zz", []byte("x")); err == nil {
		t.Error("a malformed test seed was accepted")
	}
}

func TestKecermatanAdmissionSnapshotsAndReplays(t *testing.T) {
	h := newKecHarness(t)
	tenant := h.t.q.a
	test := h.testIn(t, tenant)
	user, p := h.holder(t, tenant, test, 2)

	if test.Locked {
		t.Fatal("a new test starts unlocked")
	}
	view, created := h.start(t, tenant, p, test)
	if !created || view.Session.AttemptNumber != 1 || view.Phase != KecPhaseKolom || view.Kolom.Number != 1 {
		t.Fatalf("start = %+v created=%v, want attempt 1 running kolom 1", view, created)
	}
	if view.Session.Seed == test.Seed {
		t.Error("the attempt seed equals the stored test seed")
	}
	locked, err := h.kec.Get(context.Background(), h.scope(tenant), h.staff(tenant, model.RoleAdmin), test.ID)
	if err != nil || !locked.Locked {
		t.Errorf("after the first attempt locked = %v err = %v, want the test locked (first_attempt_at stamped)", locked.Locked, err)
	}

	// The snapshot is what the stored seed and pinned pool regenerate: nothing hidden, nothing drifting.
	stored := h.generation(t, view.Session)
	pool, err := h.kec.Pool(context.Background(), h.pool, locked)
	if err != nil {
		t.Fatalf("pool: %v", err)
	}
	replayed, err := GenerateKecermatan(view.Session.Seed, pool, ParamsOf(locked))
	if err != nil {
		t.Fatalf("regenerate: %v", err)
	}
	if !reflect.DeepEqual(stored.Columns, replayed.Columns) {
		t.Error("the stored snapshot differs from a replay of the attempt seed")
	}
	if len(stored.Columns) != kecKolomCount/2 {
		t.Errorf("snapshot holds %d columns, want N/2 = %d", len(stored.Columns), kecKolomCount/2)
	}
	for kolom := 1; kolom <= kecKolomCount/2; kolom++ {
		first, _ := stored.Slot(kolom)
		second, _ := stored.Slot(kolom + kecKolomCount/2)
		if !reflect.DeepEqual(first, second) {
			t.Errorf("kolom %d and %d differ, want an exact replay", kolom, kolom+kecKolomCount/2)
		}
	}
	// What the student receives names the labels shown and never the missing one.
	body, _ := json.Marshal(view.Kolom)
	if strings.Contains(strings.ToLower(string(body)), "missing") || len(view.Kolom.Soal) != kecSoalCount || len(view.Kolom.Soal[0].Shown) != 4 {
		t.Errorf("student view = %s, want 50 soal each showing 4 of 5 labels and no key", body)
	}

	// Two attempts of one student draw different content.
	h.clock.Advance(time.Duration(kecKolomCount) * kecSlot * time.Second)
	if done := h.session(t, tenant, p, view.Session.ID); done.Phase != KecPhaseCompleted {
		t.Fatalf("after every kolom phase = %s, want completed", done.Phase)
	}
	second, created := h.start(t, tenant, p, test)
	if !created || second.Session.AttemptNumber != 2 || second.Session.Seed == view.Session.Seed {
		t.Errorf("second attempt = %+v, want a new attempt with its own seed", second.Session)
	}
	if reflect.DeepEqual(h.generation(t, second.Session).Columns, stored.Columns) {
		t.Error("two attempts drew identical content")
	}
	if c := h.credit(t, user, test); c.used != 1 {
		t.Errorf("credit = %+v, want exactly the first attempt used", c)
	}
}

func TestKecermatanStartNeedsAnEnrollmentAndACredit(t *testing.T) {
	h := newKecHarness(t)
	tenant := h.t.q.a
	test := h.testIn(t, tenant)
	ctx := context.Background()

	_, stranger := h.student(t, tenant, "")
	if _, _, err := h.rt.Start(ctx, h.scope(tenant), stranger, test.ID); !errors.Is(err, ErrKecNotEnrolled) {
		t.Errorf("start without an enrollment: err = %v, want ErrKecNotEnrolled", err)
	}
	if _, err := h.rt.Overview(ctx, h.scope(tenant), stranger, test.ID); !errors.Is(err, ErrKecNotEnrolled) {
		t.Errorf("overview without an enrollment: err = %v, want ErrKecNotEnrolled", err)
	}
	if _, _, err := h.rt.Start(ctx, h.scope(tenant), h.staff(tenant, model.RoleAdmin), test.ID); !errors.Is(err, ErrRuntimeForbidden) {
		t.Errorf("staff start: err = %v, want ErrRuntimeForbidden", err)
	}

	user, p := h.holder(t, tenant, test, 1)
	h.clock.Advance(time.Minute)
	first, _ := h.start(t, tenant, p, test)
	h.clock.Advance(time.Duration(kecKolomCount) * kecSlot * time.Second)
	if done := h.session(t, tenant, p, first.Session.ID); done.Phase != KecPhaseCompleted {
		t.Fatalf("phase = %s, want completed", done.Phase)
	}
	if _, _, err := h.rt.Start(ctx, h.scope(tenant), p, test.ID); !errors.Is(err, ErrKecNoCredit) {
		t.Errorf("start with the credit used: err = %v, want ErrKecNoCredit", err)
	}
	if n := h.count(t, `SELECT count(*) FROM kecermatan_sessions WHERE user_id = $1`, user.ID); n != 1 {
		t.Errorf("sessions = %d, want the refused start to create none", n)
	}
	// Buying again adds to the same enrollment and re-opens the door.
	h.grant(t, tenant, user, test, 2)
	if c := h.credit(t, user, test); c.purchased != 3 || c.used != 1 {
		t.Errorf("credit after a second purchase = %+v, want 3 purchased and 1 used on one row", c)
	}
	if n := h.enrollments(t, user.ID); n != 1 {
		t.Errorf("enrollments = %d, want one aggregate", n)
	}
	h.start(t, tenant, p, test)
}

func TestKecermatanConcurrentStartConsumesNothingAndMakesOneAttempt(t *testing.T) {
	h := newKecHarness(t)
	tenant := h.t.q.a
	test := h.testIn(t, tenant)
	user, p := h.holder(t, tenant, test, 1)

	const callers = 8
	ids := make([]uuid.UUID, callers)
	created := make([]bool, callers)
	var wg sync.WaitGroup
	for i := 0; i < callers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			view, c, err := h.rt.Start(context.Background(), h.scope(tenant), p, test.ID)
			if err != nil {
				t.Errorf("concurrent start %d: %v", i, err)
				return
			}
			ids[i], created[i] = view.Session.ID, c
		}()
	}
	wg.Wait()
	wins := 0
	for i := range ids {
		if ids[i] != ids[0] {
			t.Errorf("start %d returned attempt %s, want all the same %s", i, ids[i], ids[0])
		}
		if created[i] {
			wins++
		}
	}
	if wins != 1 {
		t.Errorf("%d starts created an attempt, want exactly one", wins)
	}
	if n := h.count(t, `SELECT count(*) FROM kecermatan_sessions WHERE user_id = $1`, user.ID); n != 1 {
		t.Errorf("sessions = %d, want 1", n)
	}
	if c := h.credit(t, user, test); c.used != 0 {
		t.Errorf("credit = %+v, want nothing consumed while the attempt is in progress", c)
	}
	// A resume is a start that finds the attempt: it consumes nothing either.
	again, created2 := h.start(t, tenant, p, test)
	if created2 || again.Session.ID != ids[0] {
		t.Errorf("resume created=%v id=%s, want the same attempt", created2, again.Session.ID)
	}
	if c := h.credit(t, user, test); c.used != 0 {
		t.Errorf("credit after resume = %+v, want 0 used", c)
	}
}

func TestKecermatanCompletionConsumesOnceAndScores(t *testing.T) {
	h := newKecHarness(t)
	tenant := h.t.q.a
	test := h.testIn(t, tenant)
	user, p := h.holder(t, tenant, test, 3)
	view, _ := h.start(t, tenant, p, test)
	sess := view.Session
	ctx := context.Background()

	// First half: 30 right, 10 wrong, 10 blank per kolom; second half: 20 right, 10 wrong, 20 blank.
	for kolom := 1; kolom <= kecKolomCount; kolom++ {
		right, wrong := 30, 10
		if kolom > kecKolomCount/2 {
			right, wrong = 20, 10
		}
		h.answer(t, tenant, p, sess, kolom, 1, right, true)
		h.answer(t, tenant, p, sess, kolom, right+1, right+wrong, false)
		// Skipping is stored and counts as blank.
		if kolom == 1 {
			if _, err := h.rt.Answer(ctx, h.scope(tenant), p, sess.ID, kolom, 50, nil); err != nil {
				t.Fatalf("skip: %v", err)
			}
		}
		h.clock.Advance(kecSlot * time.Second)
	}
	done := h.session(t, tenant, p, sess.ID)
	if done.Phase != KecPhaseCompleted || done.Result == nil {
		t.Fatalf("phase = %s result = %v, want completed with a result", done.Phase, done.Result)
	}
	want, err := ScoreKecermatan(1, append(
		uniformKolom(5, KecermatanKolomCount{30, 10, 10}), uniformKolom(5, KecermatanKolomCount{20, 10, 20})...))
	if err != nil {
		t.Fatalf("score: %v", err)
	}
	r := done.Result
	if r.BenarTotal != 250 || r.SalahTotal != 100 || r.KosongTotal != 150 || r.ScoringVersion != KecermatanScoringVersion ||
		r.NilaiAkhir != want.NilaiAkhir || r.Kecepatan != want.Kecepatan || r.Ketahanan != want.Ketahanan || r.Ketelitian != want.Ketelitian {
		t.Errorf("result = %+v, want 250/100/150 and the scores of %+v", r, want)
	}
	if c := h.credit(t, user, test); c.used != 1 {
		t.Errorf("credit = %+v, want exactly one used", c)
	}
	// Reading, resuming and the worker again must not consume or rewrite anything.
	h.session(t, tenant, p, sess.ID)
	if n, err := h.rt.FinalizeDue(ctx); err != nil || n != 0 {
		t.Errorf("FinalizeDue after completion = %d, %v, want nothing due", n, err)
	}
	if c := h.credit(t, user, test); c.used != 1 {
		t.Errorf("credit after re-reads = %+v, want still 1 used", c)
	}
	if n := h.count(t, `SELECT count(*) FROM kecermatan_results WHERE session_id = $1`, sess.ID); n != 1 {
		t.Errorf("result rows = %d, want 1", n)
	}
	if _, err := h.rt.Answer(ctx, h.scope(tenant), p, sess.ID, 10, 1, nil); !errors.Is(err, ErrKecSessionClosed) {
		t.Errorf("answer after completion: err = %v, want ErrKecSessionClosed", err)
	}
	// The counts cannot be rewritten, even by SQL.
	if _, err := h.pool.Exec(ctx, `UPDATE kecermatan_results SET benar_total = benar_total + 1 WHERE session_id = $1`, sess.ID); err == nil {
		t.Error("result counts were updated, want the immutability trigger to refuse")
	}
	// The history lists it with its final score and the remaining credit is 2.
	overview, err := h.rt.Overview(ctx, h.scope(tenant), p, test.ID)
	if err != nil || len(overview.Attempts) != 1 || overview.Attempts[0].NilaiAkhir == nil ||
		overview.Test.Purchased-overview.Test.Used != 2 || overview.Current != nil {
		t.Errorf("overview = %+v err = %v, want one scored attempt and 2 credits left", overview, err)
	}
}

func TestKecermatanExpiryRecordsKosongAndTimersNeverReset(t *testing.T) {
	h := newKecHarness(t)
	tenant := h.t.q.a
	test := h.testIn(t, tenant)
	_, p := h.holder(t, tenant, test, 1)
	ctx := context.Background()
	t0 := h.clock.Now()
	view, _ := h.start(t, tenant, p, test)
	sess := view.Session
	if !view.Kolom.DeadlineAt.Equal(t0.Add(kecKolomSeconds * time.Second)) {
		t.Fatalf("kolom 1 deadline = %v, want start + %ds", view.Kolom.DeadlineAt, kecKolomSeconds)
	}
	h.answer(t, tenant, p, sess, 1, 1, 3, true)

	// Closing the tab and returning inside the kolom shows the same deadline, not a fresh one.
	h.clock.Advance(90 * time.Second)
	back := h.session(t, tenant, p, sess.ID)
	if !back.Kolom.DeadlineAt.Equal(view.Kolom.DeadlineAt) || len(back.Kolom.Answers) != 3 {
		t.Errorf("resume = deadline %v answers %d, want the original deadline and the 3 stored answers", back.Kolom.DeadlineAt, len(back.Kolom.Answers))
	}

	// Past the deadline: the pause shows, the old kolom takes nothing, and the unanswered are kosong.
	h.clock.Advance(31 * time.Second)
	pause := h.session(t, tenant, p, sess.ID)
	if pause.Phase != KecPhasePause || pause.FinishedKolom != 1 || pause.NextKolom != 2 || pause.Kolom != nil {
		t.Fatalf("after the deadline = %+v, want the pause between kolom 1 and 2 and no kolom content", pause)
	}
	label := "A"
	if _, err := h.rt.Answer(ctx, h.scope(tenant), p, sess.ID, 1, 4, &label); !errors.Is(err, ErrKecKolomNotRunning) {
		t.Errorf("answer to an ended kolom: err = %v, want ErrKecKolomNotRunning", err)
	}
	if _, err := h.rt.Answer(ctx, h.scope(tenant), p, sess.ID, 2, 1, &label); !errors.Is(err, ErrKecKolomNotRunning) {
		t.Errorf("answer during the pause: err = %v, want ErrKecKolomNotRunning", err)
	}
	kolom, _ := h.repo.Kolom(ctx, h.pool, tenant.ID, sess.ID)
	if intOf(kolom[0].Benar) != 3 || intOf(kolom[0].Salah) != 0 || intOf(kolom[0].Kosong) != kecSoalCount-3 {
		t.Errorf("kolom 1 counts = %d/%d/%d, want 3 right and the other 47 kosong", intOf(kolom[0].Benar), intOf(kolom[0].Salah), intOf(kolom[0].Kosong))
	}

	// Staying away through the pause starts kolom 2 at the pause's end, so its clock has been running.
	h.clock.Advance(10 * time.Second)
	two := h.session(t, tenant, p, sess.ID)
	wantStart := t0.Add(kecSlot * time.Second)
	if two.Phase != KecPhaseKolom || two.Kolom.Number != 2 || !two.Kolom.StartedAt.Equal(wantStart) ||
		!two.Kolom.DeadlineAt.Equal(wantStart.Add(kecKolomSeconds*time.Second)) {
		t.Errorf("kolom 2 = %+v, want it started at %v by the server", two.Kolom, wantStart)
	}
	if len(two.Kolom.Answers) != 0 {
		t.Errorf("kolom 2 answers = %v, want none carried over", two.Kolom.Answers)
	}
}

func TestKecermatanLanjutStartsAtThePressNotEarlier(t *testing.T) {
	h := newKecHarness(t)
	tenant := h.t.q.a
	test := h.testIn(t, tenant)
	_, p := h.holder(t, tenant, test, 1)
	ctx := context.Background()
	view, _ := h.start(t, tenant, p, test)

	// Pressing Lanjut while a kolom still runs does nothing.
	h.clock.Advance(30 * time.Second)
	same, err := h.rt.Continue(ctx, h.scope(tenant), p, view.Session.ID)
	if err != nil || same.Kolom.Number != 1 || !same.Kolom.DeadlineAt.Equal(view.Kolom.DeadlineAt) {
		t.Fatalf("early continue = %+v, %v, want kolom 1 untouched", same.Kolom, err)
	}
	h.clock.Advance((kecKolomSeconds - 30 + 2) * time.Second)
	pressed := h.clock.Now()
	next, err := h.rt.Continue(ctx, h.scope(tenant), p, view.Session.ID)
	if err != nil || next.Phase != KecPhaseKolom || next.Kolom.Number != 2 || !next.Kolom.StartedAt.Equal(pressed) {
		t.Fatalf("continue = %+v, %v, want kolom 2 started at the press", next.Kolom, err)
	}
	h.clock.Advance(time.Second)
	again, _ := h.rt.Continue(ctx, h.scope(tenant), p, view.Session.ID)
	if again.Kolom.Number != 2 || !again.Kolom.StartedAt.Equal(pressed) {
		t.Errorf("a second continue moved kolom 2 to %v, want it left at %v", again.Kolom.StartedAt, pressed)
	}
}

func TestKecermatanWorkerFinalizesAbandonedAttempts(t *testing.T) {
	h := newKecHarness(t)
	tenant := h.t.q.a
	test := h.testIn(t, tenant)
	user, p := h.holder(t, tenant, test, 1)
	ctx := context.Background()
	view, _ := h.start(t, tenant, p, test)
	h.answer(t, tenant, p, view.Session, 1, 1, 5, true)

	if n, err := h.rt.FinalizeDue(ctx); err != nil || n != 0 {
		t.Errorf("early FinalizeDue = %d, %v, want nothing due", n, err)
	}
	h.clock.Advance(121 * time.Second)
	if n, err := h.rt.FinalizeDue(ctx); err != nil || n != 1 {
		t.Errorf("FinalizeDue at the first deadline = %d, %v, want 1 attempt moved", n, err)
	}
	h.clock.Advance(time.Duration(kecKolomCount) * kecSlot * time.Second)
	if n, err := h.rt.FinalizeDue(ctx); err != nil || n != 1 {
		t.Errorf("FinalizeDue after every kolom = %d, %v, want the attempt completed", n, err)
	}
	if n, _ := h.rt.FinalizeDue(ctx); n != 0 {
		t.Errorf("second FinalizeDue = %d, want 0", n)
	}
	var benar, salah, kosong int
	var completed time.Time
	if err := h.pool.QueryRow(ctx, `SELECT benar_total, salah_total, kosong_total, completed_at FROM kecermatan_results WHERE session_id = $1`,
		view.Session.ID).Scan(&benar, &salah, &kosong, &completed); err != nil {
		t.Fatalf("read abandoned result: %v", err)
	}
	if benar != 5 || salah != 0 || kosong != kecKolomCount*kecSoalCount-5 {
		t.Errorf("abandoned result = %d/%d/%d, want 5 right and everything else kosong", benar, salah, kosong)
	}
	if want := view.ServerTime.Add((kecKolomCount*kecSlot - KecermatanPauseSeconds) * time.Second); !completed.Equal(want) {
		t.Errorf("completed_at = %v, want the last kolom's deadline %v, not the worker's clock", completed, want)
	}
	if c := h.credit(t, user, test); c.used != 1 {
		t.Errorf("credit = %+v, want the abandoned attempt to consume exactly one", c)
	}
}

func TestKecermatanTimelineWhenAttemptIsOpenedLate(t *testing.T) {
	h := newKecHarness(t)
	tenant := h.t.q.a
	test := h.testIn(t, tenant)
	_, p := h.holder(t, tenant, test, 1)
	t0 := h.clock.Now()
	view, _ := h.start(t, tenant, p, test)

	h.clock.Advance(5*kecSlot*time.Second + 75*time.Second)
	back := h.session(t, tenant, p, view.Session.ID)
	start := t0.Add(5 * kecSlot * time.Second)
	if back.Kolom.Number != 6 || !back.Kolom.StartedAt.Equal(start) || !back.Kolom.DeadlineAt.Equal(start.Add(kecKolomSeconds*time.Second)) {
		t.Errorf("kolom after a long absence = %+v, want kolom 6 on the server timeline from %v", back.Kolom, start)
	}
}

func TestKecermatanAnswersAreWriteOnceAndValidated(t *testing.T) {
	h := newKecHarness(t)
	tenant := h.t.q.a
	test := h.testIn(t, tenant)
	_, p := h.holder(t, tenant, test, 1)
	ctx := context.Background()
	view, _ := h.start(t, tenant, p, test)
	id := view.Session.ID
	a, b, z := "A", "B", "Z"

	if _, err := h.rt.Answer(ctx, h.scope(tenant), p, id, 1, 1, &a); err != nil {
		t.Fatalf("first answer: %v", err)
	}
	if ack, err := h.rt.Answer(ctx, h.scope(tenant), p, id, 1, 1, &a); err != nil || ack.Answer == nil || *ack.Answer != "A" {
		t.Errorf("retry = %+v, %v, want an idempotent acknowledgement", ack, err)
	}
	if _, err := h.rt.Answer(ctx, h.scope(tenant), p, id, 1, 1, &b); !errors.Is(err, ErrKecAnswerLocked) {
		t.Errorf("changed answer: err = %v, want ErrKecAnswerLocked", err)
	}
	if _, err := h.rt.Answer(ctx, h.scope(tenant), p, id, 1, 2, nil); err != nil {
		t.Errorf("skip: %v", err)
	}
	if _, err := h.rt.Answer(ctx, h.scope(tenant), p, id, 1, 2, &a); !errors.Is(err, ErrKecAnswerLocked) {
		t.Errorf("answer after a skip: err = %v, want ErrKecAnswerLocked", err)
	}
	for name, call := range map[string]func() error{
		"label outside A-E": func() error { _, err := h.rt.Answer(ctx, h.scope(tenant), p, id, 1, 3, &z); return err },
		"soal 0":            func() error { _, err := h.rt.Answer(ctx, h.scope(tenant), p, id, 1, 0, &a); return err },
		"soal 51":           func() error { _, err := h.rt.Answer(ctx, h.scope(tenant), p, id, 1, 51, &a); return err },
	} {
		var input *InputError
		if err := call(); !errors.As(err, &input) {
			t.Errorf("%s: err = %v, want an InputError", name, err)
		}
	}
	if _, err := h.rt.Answer(ctx, h.scope(tenant), p, id, 2, 1, &a); !errors.Is(err, ErrKecKolomNotRunning) {
		t.Errorf("answer to a future kolom: err = %v, want ErrKecKolomNotRunning", err)
	}
	// Another student cannot touch this attempt.
	_, other := h.student(t, tenant, "")
	if _, err := h.rt.Answer(ctx, h.scope(tenant), other, id, 1, 4, &a); !errors.Is(err, ErrKecSessionNotFound) {
		t.Errorf("foreign answer: err = %v, want ErrKecSessionNotFound", err)
	}
	if _, err := h.rt.Session(ctx, h.scope(tenant), other, id); !errors.Is(err, ErrKecSessionNotFound) {
		t.Errorf("foreign read: err = %v, want ErrKecSessionNotFound", err)
	}
}

func TestKecermatanReviewReproducesTheAttemptForItsOwnerOnly(t *testing.T) {
	h := newKecHarness(t)
	tenant := h.t.q.a
	test := h.testIn(t, tenant)
	_, p := h.holder(t, tenant, test, 1)
	ctx := context.Background()
	view, _ := h.start(t, tenant, p, test)
	id := view.Session.ID

	if _, err := h.rt.Review(ctx, h.scope(tenant), p, id); !errors.Is(err, ErrKecNotCompleted) {
		t.Errorf("review in progress: err = %v, want ErrKecNotCompleted", err)
	}
	if _, err := h.rt.Result(ctx, h.scope(tenant), p, id); !errors.Is(err, ErrKecNotCompleted) {
		t.Errorf("result in progress: err = %v, want ErrKecNotCompleted", err)
	}
	h.answer(t, tenant, p, view.Session, 1, 1, 4, true)
	h.clock.Advance(kecSlot * time.Second)
	h.answer(t, tenant, p, view.Session, 2, 1, 2, false)
	h.clock.Advance(time.Duration(kecKolomCount) * kecSlot * time.Second)

	review, err := h.rt.Review(ctx, h.scope(tenant), p, id)
	if err != nil {
		t.Fatalf("review: %v", err)
	}
	gen := h.generation(t, view.Session)
	if len(review.Columns) != kecKolomCount/2 || len(review.Kolom) != kecKolomCount {
		t.Fatalf("review = %d columns %d kolom, want 5 and 10", len(review.Columns), len(review.Kolom))
	}
	if !reflect.DeepEqual(review.Columns, gen.Columns) {
		t.Error("the review's columns differ from the attempt's snapshot")
	}
	k1, k6 := review.Kolom[0], review.Kolom[5]
	if k1.SourceColumn != 1 || k6.SourceColumn != 1 || k1.Benar != 4 || k1.Kosong != 46 || review.Kolom[1].Salah != 2 {
		t.Errorf("review kolom = %+v / %+v, want kolom 6 replaying column 1 with its own, empty, answers", k1, k6)
	}
	column, _ := gen.Slot(1)
	if k1.Answers[0] == nil || *k1.Answers[0] != column.Soal[0].Missing || k6.Answers[0] != nil {
		t.Errorf("answers = %v / %v, want the stored key for kolom 1 soal 1 and none for kolom 6", k1.Answers[0], k6.Answers[0])
	}
	_, stranger := h.student(t, tenant, "")
	if _, err := h.rt.Review(ctx, h.scope(tenant), stranger, id); !errors.Is(err, ErrKecSessionNotFound) {
		t.Errorf("foreign review: err = %v, want ErrKecSessionNotFound", err)
	}
	if _, err := h.rt.Result(ctx, h.scope(tenant), stranger, id); !errors.Is(err, ErrKecSessionNotFound) {
		t.Errorf("foreign result: err = %v, want ErrKecSessionNotFound", err)
	}
	if _, err := h.rt.Review(ctx, h.scope(tenant), h.staff(tenant, model.RoleAdmin), id); !errors.Is(err, ErrRuntimeForbidden) {
		t.Errorf("staff student-review: err = %v, want ErrRuntimeForbidden", err)
	}
}

func TestKecermatanPaymentAndCompletionSerializeOnTheEnrollmentRow(t *testing.T) {
	h := newKecHarness(t)
	tenant := h.t.q.a
	test := h.testIn(t, tenant)
	ctx := context.Background()

	for round := 0; round < 6; round++ {
		user, p := h.student(t, tenant, "1000000")
		h.grant(t, tenant, user, test, 1)
		view, _ := h.start(t, tenant, p, test)
		product := h.create(t, tenant, func() ProductInput {
			in := h.input(tenant)
			in.Type, in.TryoutIDs = model.ProductKecermatan, nil
			in.KecermatanTestID, in.IncludedAttempts = test.ID, 3
			in.Price = "20000"
			return in
		}())
		h.clock.Advance(time.Duration(kecKolomCount) * kecSlot * time.Second)

		// The attempt is due, so reading it completes it while a purchase adds credits on the same row.
		var wg sync.WaitGroup
		wg.Add(2)
		go func() {
			defer wg.Done()
			if _, _, err := h.wallet(t, tenant, p, product); err != nil {
				t.Errorf("round %d purchase: %v", round, err)
			}
		}()
		go func() {
			defer wg.Done()
			if _, err := h.rt.Session(ctx, h.scope(tenant), p, view.Session.ID); err != nil {
				t.Errorf("round %d completion: %v", round, err)
			}
		}()
		wg.Wait()
		if c := h.credit(t, user, test); c.purchased != 4 || c.used != 1 {
			t.Fatalf("round %d credit = %+v, want 4 purchased (1 + 3 bought) and 1 used", round, c)
		}
	}
}

func TestKecermatanRecomputeRewritesOnlyScores(t *testing.T) {
	h := newKecHarness(t)
	tenant := h.t.q.a
	test := h.testIn(t, tenant)
	_, p := h.holder(t, tenant, test, 1)
	ctx := context.Background()
	view, _ := h.start(t, tenant, p, test)
	h.answer(t, tenant, p, view.Session, 1, 1, 20, true)
	h.clock.Advance(time.Duration(kecKolomCount) * kecSlot * time.Second)
	before, err := h.rt.Result(ctx, h.scope(tenant), p, view.Session.ID)
	if err != nil {
		t.Fatalf("result: %v", err)
	}

	// Damage the stored composite on purpose: the recompute must restore it from the raw counts.
	if _, err := h.pool.Exec(ctx, `UPDATE kecermatan_results SET nilai_akhir = 1, kecepatan = 1 WHERE session_id = $1`, view.Session.ID); err != nil {
		t.Fatalf("tamper: %v", err)
	}
	n, err := h.rt.RecomputeScores(ctx, tenant.ID, 1)
	if err != nil || n < 1 {
		t.Fatalf("recompute = %d, %v, want at least the one result", n, err)
	}
	after, _ := h.rt.Result(ctx, h.scope(tenant), p, view.Session.ID)
	if after.Result.NilaiAkhir != before.Result.NilaiAkhir || after.Result.Kecepatan != before.Result.Kecepatan ||
		after.Result.BenarTotal != before.Result.BenarTotal || after.Result.KosongTotal != before.Result.KosongTotal {
		t.Errorf("after recompute %+v, want %+v", after.Result, before.Result)
	}
	if _, err := h.rt.RecomputeScores(ctx, tenant.ID, 2); !errors.Is(err, ErrKecermatanScoringVersion) {
		t.Errorf("recompute to an unknown version: err = %v, want ErrKecermatanScoringVersion", err)
	}
}
