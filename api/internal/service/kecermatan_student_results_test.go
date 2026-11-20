package service

import (
	"context"
	"errors"
	"reflect"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tarunaeduka/bimbel-tryout/api/internal/model"
)

var avatarKeyPattern = regexp.MustCompile(`^a(0[1-9]|[12][0-9]|3[0-5])$`)

func (h kecHarness) named(t *testing.T, tenant model.Tenant, test model.KecermatanTest, name string, credits int) (model.User, model.Principal) {
	t.Helper()
	user := walletUser(t, h.users, tenant.ID, model.RoleStudent, name)
	id := tenant.ID
	if credits > 0 {
		h.grant(t, tenant, user, test, credits)
	}
	return user, model.Principal{UserID: user.ID, TenantID: &id, Name: user.Name, Role: model.RoleStudent}
}

// finish runs one attempt to completion; right is how many soal of the first kolom are answered correctly, so
// attempts differ in score.
func (h kecHarness) finish(t *testing.T, tenant model.Tenant, p model.Principal, test model.KecermatanTest, right int) uuid.UUID {
	t.Helper()
	view, _ := h.start(t, tenant, p, test)
	if right > 0 {
		h.answer(t, tenant, p, view.Session, 1, 1, right, true)
	}
	h.clock.Advance(time.Duration(kecKolomCount*kecSlot) * time.Second)
	done := h.session(t, tenant, p, view.Session.ID)
	if done.Phase != KecPhaseCompleted {
		t.Fatalf("attempt phase = %s, want completed", done.Phase)
	}
	h.clock.Advance(time.Minute)
	return view.Session.ID
}

func (h kecHarness) staffRows(t *testing.T, tenant model.Tenant, filter KecResultFilter) ([]KecStudentSummary, int) {
	t.Helper()
	rows, total, err := h.rt.StaffResults(context.Background(), h.scope(tenant), h.t.principal(tenant, model.RoleOperator), filter)
	if err != nil {
		t.Fatalf("staff results %+v: %v", filter, err)
	}
	return rows, total
}

func TestKecermatanStaffResultsGroupPerStudentAndTest(t *testing.T) {
	h := newKecHarness(t)
	tenant := h.t.q.a
	t1, t2 := h.testIn(t, tenant), h.testIn(t, tenant)
	siti, sitiP := h.named(t, tenant, t1, "Siti Aminah", 3)
	h.grant(t, tenant, siti, t2, 1)
	_, rudiP := h.named(t, tenant, t1, "Rudi Hartono", 2)
	_, idleP := h.named(t, tenant, t1, "Idle Student", 1)

	h.finish(t, tenant, sitiP, t1, 10)
	h.finish(t, tenant, sitiP, t1, 40)
	h.finish(t, tenant, sitiP, t2, 25)
	rudiDone := h.finish(t, tenant, rudiP, t1, 20)
	h.start(t, tenant, rudiP, t1) // in progress: not a completed attempt
	h.start(t, tenant, idleP, t1) // never completed: no row at all

	rows, total := h.staffRows(t, tenant, KecResultFilter{})
	if total != 3 || len(rows) != 3 {
		t.Fatalf("rows = %d of %d, want 3: two attempts of one student on one test are one row, two tests two rows", len(rows), total)
	}
	if rows[0].StudentName != "Rudi Hartono" || rows[0].AttemptsCompleted != 1 {
		t.Errorf("first row = %s/%d, want the most recently active student (Rudi) with one completed attempt", rows[0].StudentName, rows[0].AttemptsCompleted)
	}
	for _, r := range rows {
		if r.StudentName == "Idle Student" {
			t.Error("a student with no completed attempt is listed")
		}
	}
	var sitiT1 KecStudentSummary
	for _, r := range rows {
		if r.StudentID == siti.ID && r.TestID == t1.ID {
			sitiT1 = r
		}
	}
	if sitiT1.AttemptsCompleted != 2 || sitiT1.RemainingAttempts != 1 {
		t.Errorf("Siti on test 1 = %d attempts, %d remaining, want 2 and 1", sitiT1.AttemptsCompleted, sitiT1.RemainingAttempts)
	}
	overview, err := h.rt.Overview(context.Background(), h.scope(tenant), sitiP, t1.ID)
	if err != nil {
		t.Fatalf("overview: %v", err)
	}
	if !reflect.DeepEqual(sitiT1.Progress, overview.Progress) || sitiT1.Progress.AttemptsCounted != 2 || sitiT1.Progress.NilaiGabunganPrevious == nil {
		t.Errorf("row progress = %+v, student overview = %+v, want identical", sitiT1.Progress, overview.Progress)
	}
	if !sitiT1.LastCompletedAt.Before(rows[0].LastCompletedAt) {
		t.Error("rows are not ordered by last_completed_at descending")
	}

	byTest, total := h.staffRows(t, tenant, KecResultFilter{TestID: &t2.ID})
	if total != 1 || len(byTest) != 1 || byTest[0].StudentID != siti.ID {
		t.Errorf("test 2 rows = %d of %d, want only Siti", len(byTest), total)
	}
	byProgram, total := h.staffRows(t, tenant, KecResultFilter{ProgramID: &t1.ProgramID})
	if total != 3 || len(byProgram) != 3 {
		t.Errorf("program rows = %d of %d, want 3", len(byProgram), total)
	}

	first, total := h.staffRows(t, tenant, KecResultFilter{Limit: 2})
	second, total2 := h.staffRows(t, tenant, KecResultFilter{Limit: 2, Offset: 2})
	if total != 3 || total2 != 3 || len(first) != 2 || len(second) != 1 {
		t.Fatalf("pages = %d + %d rows, totals %d/%d, want 2 + 1 and total 3 student rows", len(first), len(second), total, total2)
	}
	if second[0].StudentID == first[0].StudentID && second[0].TestID == first[0].TestID || second[0].StudentID == first[1].StudentID && second[0].TestID == first[1].TestID {
		t.Error("pages overlap")
	}
	if _, total := h.staffRows(t, tenant, KecResultFilter{Offset: 50}); total != 3 {
		t.Errorf("total past the last page = %d, want 3", total)
	}

	attempt, err := h.rt.StaffResult(context.Background(), h.scope(tenant), h.t.principal(tenant, model.RoleAdmin), rudiDone)
	if err != nil || attempt.Result.SessionID != rudiDone || len(attempt.Kolom) != kecKolomCount {
		t.Errorf("per-attempt detail = %+v, %v, want unchanged with %d kolom", attempt.Result.SessionID, err, kecKolomCount)
	}
}

func TestKecermatanStaffResultsNameSearch(t *testing.T) {
	h := newKecHarness(t)
	tenant := h.t.q.a
	test := h.testIn(t, tenant)
	for _, name := range []string{"Budi Santoso", "BUDI Lain", "Sari_Dewi", "Rina 100% Rajin", "Tono"} {
		_, p := h.named(t, tenant, test, name, 1)
		h.finish(t, tenant, p, test, 5)
	}
	names := func(q string) []string {
		rows, total := h.staffRows(t, tenant, KecResultFilter{Search: q})
		if total != len(rows) {
			t.Fatalf("q=%q: total %d, rows %d", q, total, len(rows))
		}
		out := []string{}
		for _, r := range rows {
			out = append(out, r.StudentName)
		}
		return out
	}
	if got := names("budi"); len(got) != 2 {
		t.Errorf("q=budi = %v, want both Budi rows regardless of case", got)
	}
	if got := names("SANTOSO"); len(got) != 1 || got[0] != "Budi Santoso" {
		t.Errorf("q=SANTOSO = %v, want Budi Santoso", got)
	}
	if got := names("%"); len(got) != 1 || got[0] != "Rina 100% Rajin" {
		t.Errorf("q=%% = %v, want only the name containing a literal percent sign", got)
	}
	if got := names("_"); len(got) != 1 || got[0] != "Sari_Dewi" {
		t.Errorf("q=_ = %v, want only the name containing a literal underscore", got)
	}
	if got := names("i_d"); len(got) != 1 || got[0] != "Sari_Dewi" {
		t.Errorf("q=i_d = %v, want Sari_Dewi, the underscore is not a wildcard", got)
	}
	if got := names("zzz"); len(got) != 0 {
		t.Errorf("q=zzz = %v, want none", got)
	}
	if got := names(""); len(got) != 5 {
		t.Errorf("no q = %d rows, want all 5", len(got))
	}
	var input *InputError
	_, _, err := h.rt.StaffResults(context.Background(), h.scope(tenant), h.t.principal(tenant, model.RoleAdmin),
		KecResultFilter{Search: strings.Repeat("x", 101)})
	if !errors.As(err, &input) || input.Fields["q"] == "" {
		t.Errorf("101-character q: err = %v, want a validation error on q", err)
	}
	if _, _, err := h.rt.StaffResults(context.Background(), h.scope(tenant), h.t.principal(tenant, model.RoleAdmin),
		KecResultFilter{Search: strings.Repeat("é", 100)}); err != nil {
		t.Errorf("100-character q: err = %v, want accepted (limit counts characters)", err)
	}
}

func TestKecermatanStaffResultsAvatarIsResolvedLikeMe(t *testing.T) {
	h := newKecHarness(t)
	tenant := h.t.q.a
	test := h.testIn(t, tenant)
	picked, pickedP := h.named(t, tenant, test, "Pilih Sendiri", 1)
	auto, autoP := h.named(t, tenant, test, "Otomatis", 1)
	young := time.Date(2012, 5, 1, 0, 0, 0, 0, time.UTC)
	female := model.GenderFemale
	if _, err := h.pool.Exec(context.Background(), `UPDATE users SET avatar_key = 'a07' WHERE id = $1`, picked.ID); err != nil {
		t.Fatalf("pick avatar: %v", err)
	}
	if _, err := h.pool.Exec(context.Background(), `UPDATE users SET gender = $2, birth_date = $3 WHERE id = $1`, auto.ID, female, young); err != nil {
		t.Fatalf("set profile: %v", err)
	}
	h.finish(t, tenant, pickedP, test, 5)
	h.finish(t, tenant, autoP, test, 5)

	rows, _ := h.staffRows(t, tenant, KecResultFilter{})
	if len(rows) != 2 {
		t.Fatalf("rows = %d, want the two students", len(rows))
	}
	want := map[uuid.UUID]string{picked.ID: "a07", auto.ID: model.DefaultAvatarKey(auto.ID, &female, &young, time.Now())}
	for _, r := range rows {
		if r.Avatar != want[r.StudentID] || !avatarKeyPattern.MatchString(r.Avatar) {
			t.Errorf("%s avatar = %q, want %q", r.StudentName, r.Avatar, want[r.StudentID])
		}
	}
	detail, err := h.rt.StaffStudentResults(context.Background(), h.scope(tenant), h.t.principal(tenant, model.RoleAdmin), auto.ID, test.ID)
	if err != nil || detail.Avatar != want[auto.ID] {
		t.Errorf("detail avatar = %q, %v, want %q", detail.Avatar, err, want[auto.ID])
	}
}

func TestKecermatanStaffStudentResultsDetail(t *testing.T) {
	h := newKecHarness(t)
	tenant := h.t.q.a
	test := h.testIn(t, tenant)
	user, p := h.named(t, tenant, test, "Detail Siswa", 4)
	ids := []uuid.UUID{h.finish(t, tenant, p, test, 5), h.finish(t, tenant, p, test, 30), h.finish(t, tenant, p, test, 15)}
	h.start(t, tenant, p, test) // in progress, must not be listed
	ctx := context.Background()
	admin := h.t.principal(tenant, model.RoleAdmin)

	got, err := h.rt.StaffStudentResults(ctx, h.scope(tenant), admin, user.ID, test.ID)
	if err != nil {
		t.Fatalf("detail: %v", err)
	}
	if got.StudentName != "Detail Siswa" || got.TestTitle != test.Title || got.ProgramName == "" || got.ProgramCode == "" || got.RemainingAttempts != 1 {
		t.Errorf("detail head = %+v, want student, test, program and 1 remaining", got.KecStudentRow)
	}
	if len(got.Attempts) != 3 || got.Attempts[0].SessionID != ids[2] || got.Attempts[2].SessionID != ids[0] ||
		got.Attempts[0].AttemptNumber < got.Attempts[1].AttemptNumber {
		t.Errorf("attempts = %+v, want the three completed attempts newest first", got.Attempts)
	}
	overview, _ := h.rt.Overview(ctx, h.scope(tenant), p, test.ID)
	if !reflect.DeepEqual(got.Progress, overview.Progress) {
		t.Errorf("detail progress = %+v, student overview = %+v, want identical", got.Progress, overview.Progress)
	}

	_, otherP := h.named(t, tenant, test, "Belum Selesai", 1)
	h.start(t, tenant, otherP, test)
	other := h.t.q.b
	adminB := h.t.principal(other, model.RoleAdmin)
	for name, call := range map[string]func() error{
		"another tenant's admin": func() error {
			_, err := h.rt.StaffStudentResults(ctx, h.scope(other), adminB, user.ID, test.ID)
			return err
		},
		"student without a completed attempt": func() error {
			_, err := h.rt.StaffStudentResults(ctx, h.scope(tenant), admin, otherP.UserID, test.ID)
			return err
		},
		"unknown test": func() error {
			_, err := h.rt.StaffStudentResults(ctx, h.scope(tenant), admin, user.ID, uuid.New())
			return err
		},
		"unknown student": func() error {
			_, err := h.rt.StaffStudentResults(ctx, h.scope(tenant), admin, uuid.New(), test.ID)
			return err
		},
	} {
		if err := call(); !errors.Is(err, ErrKecSessionNotFound) {
			t.Errorf("%s: err = %v, want not found", name, err)
		}
	}
	if rows, total, err := h.rt.StaffResults(ctx, h.scope(other), adminB, KecResultFilter{}); err != nil || total != 0 || len(rows) != 0 {
		t.Errorf("tenant B lists %d of %d rows (err %v), want none", len(rows), total, err)
	}
	if rows, _, _ := h.rt.StaffResults(ctx, h.scope(other), adminB, KecResultFilter{Search: "Detail"}); len(rows) != 0 {
		t.Error("tenant B finds tenant A's student by name")
	}

	if _, _, err := h.rt.StaffResults(ctx, h.scope(tenant), p, KecResultFilter{}); !errors.Is(err, ErrRuntimeForbidden) {
		t.Errorf("student lists results: err = %v, want forbidden", err)
	}
	if _, err := h.rt.StaffStudentResults(ctx, h.scope(tenant), p, user.ID, test.ID); !errors.Is(err, ErrRuntimeForbidden) {
		t.Errorf("student reads the detail: err = %v, want forbidden", err)
	}
}
