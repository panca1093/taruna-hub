package service

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tarunaeduka/bimbel-tryout/api/internal/model"
	"github.com/tarunaeduka/bimbel-tryout/api/internal/repository"
)

func (h resultHarness) named(t *testing.T, tr model.Tryout, name string) (model.User, model.Principal) {
	t.Helper()
	user := walletUser(t, h.users, h.tenant().ID, model.RoleStudent, name)
	h.enroll(t, user, tr.ID)
	id := h.tenant().ID
	return user, model.Principal{UserID: user.ID, TenantID: &id, Name: user.Name, Role: model.RoleStudent}
}

func (h resultHarness) staffRows(t *testing.T, f StaffResultFilter) ([]StaffStudentRow, int) {
	t.Helper()
	rows, total, err := h.rs.StaffResults(context.Background(), h.scopeA(), h.staff(model.RoleOperator), f)
	if err != nil {
		t.Fatalf("staff results %+v: %v", f, err)
	}
	return rows, total
}

func rowOf(t *testing.T, rows []StaffStudentRow, student, tryout uuid.UUID) StaffStudentRow {
	t.Helper()
	for _, r := range rows {
		if r.StudentID == student && r.TryoutID == tryout {
			return r
		}
	}
	t.Fatalf("no row for student %s on tryout %s", student, tryout)
	return StaffStudentRow{}
}

func TestStaffResultsGroupPerStudentAndTryoutWithTheLeaderboardRank(t *testing.T) {
	h := newResultHarness(t)
	q1, q2 := h.question(t), h.question(t)
	t1, t2 := h.tryoutOf(t, nil, q1), h.tryoutOf(t, nil, q2)
	right1, wrong1 := map[uuid.UUID]string{q1.ID: optID(t, q1, "4")}, map[uuid.UUID]string{q1.ID: optID(t, q1, "3")}
	right2 := map[uuid.UUID]string{q2.ID: optID(t, q2, "4")}
	tick := func() { h.clock.Advance(time.Minute) }

	ada, adaP := h.named(t, t1, "Ada Lovelace")
	h.enroll(t, ada, t2.ID)
	budi, budiP := h.named(t, t1, "Budi Santoso")
	cici, ciciP := h.named(t, t1, "Cici Paramita")
	idle, idleP := h.named(t, t1, "Idle Student")

	h.take(t, adaP, t1, wrong1)
	tick()
	h.take(t, adaP, t1, right1)
	tick()
	h.take(t, adaP, t2, right2)
	tick()
	h.take(t, budiP, t1, right1)
	tick()
	h.take(t, ciciP, t1, wrong1)
	tick()
	h.take(t, ciciP, t1, right1)
	tick()
	if _, err := h.rt.Start(context.Background(), h.scopeA(), idleP, t1.ID); err != nil {
		t.Fatalf("start idle attempt: %v", err)
	}

	rows, total := h.staffRows(t, StaffResultFilter{})
	if total != 4 || len(rows) != 4 {
		t.Fatalf("rows = %d of %d, want 4: two attempts on one tryout are one row, two tryouts are two rows", len(rows), total)
	}
	for _, r := range rows {
		if r.StudentID == idle.ID {
			t.Error("a student with no completed attempt is listed")
		}
	}

	adaT1 := rowOf(t, rows, ada.ID, t1.ID)
	if adaT1.AttemptsCompleted != 2 || adaT1.LatestScore != 100 || adaT1.Average != 50 || adaT1.TryoutTitle != t1.Title || adaT1.ProgramCode == "" || adaT1.ScoringMode != "weighted" {
		t.Errorf("Ada on tryout 1 = %+v, want 2 attempts, latest 100, average 50", adaT1)
	}
	if r := rowOf(t, rows, ada.ID, t2.ID); r.AttemptsCompleted != 1 || r.LatestScore != 100 || r.Rank != 1 || r.Participants != 1 {
		t.Errorf("Ada on tryout 2 = %+v, want one attempt scoring 100, rank 1 of 1", r)
	}

	board, _, err := h.rs.Leaderboard(context.Background(), h.scopeA(), h.staff(model.RoleAdmin), t1.ID, 0, 0)
	if err != nil {
		t.Fatalf("leaderboard: %v", err)
	}
	wantRank := map[uuid.UUID]int{budi.ID: 1, ada.ID: 2, cici.ID: 2}
	if len(board) != 3 {
		t.Fatalf("leaderboard = %d entries, want 3", len(board))
	}
	for _, e := range board {
		r := rowOf(t, rows, e.UserID, t1.ID)
		own := h.ranking(t, t1.ID, e.UserID)
		if r.Rank != e.Rank || r.Average != e.Average || r.Rank != wantRank[e.UserID] || r.Participants != 3 ||
			r.Rank != own.Rank || r.Average != own.Average || r.Participants != own.Participants {
			t.Errorf("%s: row rank/average/participants = %d/%v/%d, leaderboard %d/%v, own %+v, want rank %d of 3 (1, 2, 2)",
				e.Name, r.Rank, r.Average, r.Participants, e.Rank, e.Average, own, wantRank[e.UserID])
		}
	}

	onT1, total := h.staffRows(t, StaffResultFilter{TryoutID: &t1.ID})
	if total != 3 || len(onT1) != 3 {
		t.Fatalf("tryout 1 rows = %d of %d, want 3", len(onT1), total)
	}
	if onT1[0].StudentID != budi.ID || onT1[1].StudentID != cici.ID || onT1[2].StudentID != ada.ID {
		t.Errorf("order = %s, %s, %s, want Budi (rank 1), then the tied Cici (active later) before Ada",
			onT1[0].StudentName, onT1[1].StudentName, onT1[2].StudentName)
	}
	if byProgram, total := h.staffRows(t, StaffResultFilter{ProgramID: &t1.ProgramID}); total != 4 || len(byProgram) != 4 {
		t.Errorf("program rows = %d of %d, want 4", len(byProgram), total)
	}
	other := uuid.New()
	if byProgram, total := h.staffRows(t, StaffResultFilter{ProgramID: &other}); total != 0 || len(byProgram) != 0 {
		t.Errorf("another program = %d of %d, want none", len(byProgram), total)
	}
	for _, r := range rows {
		if r.IRTWarning {
			t.Errorf("%s: a weighted tryout must not carry an IRT warning", r.StudentName)
		}
	}
}

func TestStaffResultsExportStaysPerAttempt(t *testing.T) {
	h := newResultHarness(t)
	q := h.question(t)
	tr := h.tryoutOf(t, nil, q)
	_, p := h.named(t, tr, "Dua Kali")
	h.take(t, p, tr, map[uuid.UUID]string{q.ID: optID(t, q, "3")})
	h.clock.Advance(time.Minute)
	h.take(t, p, tr, map[uuid.UUID]string{q.ID: optID(t, q, "4")})

	if rows, total := h.staffRows(t, StaffResultFilter{}); len(rows) != 1 || total != 1 {
		t.Errorf("list = %d of %d, want one student row", len(rows), total)
	}
	exported, err := h.results.ExportRows(context.Background(), h.pool, h.tenant().ID,
		repository.ExportFilter{TryoutID: &tr.ID, CutOff: h.clock.Now().Add(time.Hour), Limit: 100})
	if err != nil || len(exported) != 2 {
		t.Errorf("export rows = %d (err %v), want one per attempt (2)", len(exported), err)
	}
}

func TestStaffResultsFlagIRTAcrossAttempts(t *testing.T) {
	h := newResultHarness(t)
	q := h.question(t)
	tr := h.tryoutOf(t, func(in *TryoutInput) { in.ScoringMode = model.ScoringIRT }, q)
	right := map[uuid.UUID]string{q.ID: optID(t, q, "4")}

	ada, adaP := h.named(t, tr, "Ada")
	h.take(t, adaP, tr, right)
	h.take(t, adaP, tr, right)
	_, bP := h.named(t, tr, "Budi")
	h.take(t, bP, tr, right)

	rows, _ := h.staffRows(t, StaffResultFilter{})
	if len(rows) != 2 || !rows[0].IRTWarning || !rows[1].IRTWarning || rows[0].ScoringMode != "irt" {
		t.Fatalf("rows = %+v, want two IRT rows with a warning (2 distinct respondents < 3, a retake counts once)", rows)
	}
	detail, err := h.rs.StaffStudentResults(context.Background(), h.scopeA(), h.staff(model.RoleAdmin), ada.ID, tr.ID)
	if err != nil || len(detail.Attempts) != 2 || !detail.Attempts[0].IRTWarning || !detail.Attempts[1].IRTWarning {
		t.Errorf("detail attempts = %+v (err %v), want both attempts warned", detail.Attempts, err)
	}

	_, cP := h.named(t, tr, "Cici")
	h.take(t, cP, tr, right)
	rows, _ = h.staffRows(t, StaffResultFilter{})
	for _, r := range rows {
		if r.IRTWarning {
			t.Errorf("%s still warned with 3 distinct respondents", r.StudentName)
		}
	}
	detail, _ = h.rs.StaffStudentResults(context.Background(), h.scopeA(), h.staff(model.RoleAdmin), ada.ID, tr.ID)
	if detail.Attempts[0].IRTWarning || detail.Attempts[1].IRTWarning {
		t.Error("detail attempts still warned with 3 distinct respondents")
	}
}

func TestStaffResultsNameSearchPaginationAndOrder(t *testing.T) {
	h := newResultHarness(t)
	q := h.question(t)
	tr := h.tryoutOf(t, nil, q)
	right := map[uuid.UUID]string{q.ID: optID(t, q, "4")}
	for _, name := range []string{"Zed Last", "Abe First", "Lia_Satu", "Mia Satu", "Percent 100%"} {
		_, p := h.named(t, tr, name)
		h.take(t, p, tr, right) // same clock instant: rank and completion tie, so name decides
	}

	all, total := h.staffRows(t, StaffResultFilter{})
	names := []string{}
	for _, r := range all {
		names = append(names, r.StudentName)
	}
	if total != 5 || strings.Join(names, ",") != "Abe First,Lia_Satu,Mia Satu,Percent 100%,Zed Last" {
		t.Errorf("order = %v of %d, want last activity, then name (rank first only when one tryout is selected)", names, total)
	}
	again, _ := h.staffRows(t, StaffResultFilter{})
	for i := range all {
		if all[i].StudentID != again[i].StudentID {
			t.Fatal("the order is not deterministic")
		}
	}

	for q, want := range map[string]string{"IA": "Lia_Satu,Mia Satu", "a_s": "Lia_Satu", "%": "Percent 100%", "t 100%": "Percent 100%", "nobody": ""} {
		rows, total := h.staffRows(t, StaffResultFilter{Search: q})
		got := []string{}
		for _, r := range rows {
			got = append(got, r.StudentName)
		}
		if strings.Join(got, ",") != want || total != len(got) {
			t.Errorf("search %q = %v of %d, want %q", q, got, total, want)
		}
	}

	first, total1 := h.staffRows(t, StaffResultFilter{Limit: 2})
	second, total2 := h.staffRows(t, StaffResultFilter{Limit: 2, Offset: 2})
	last, total3 := h.staffRows(t, StaffResultFilter{Limit: 2, Offset: 4})
	if total1 != 5 || total2 != 5 || total3 != 5 || len(first) != 2 || len(second) != 2 || len(last) != 1 {
		t.Fatalf("pages = %d/%d/%d rows, totals %d/%d/%d, want 2/2/1 and total 5", len(first), len(second), len(last), total1, total2, total3)
	}
	if first[1].StudentID == second[0].StudentID || second[1].StudentID == last[0].StudentID {
		t.Error("pages overlap")
	}
	if rows, total := h.staffRows(t, StaffResultFilter{Offset: 50}); len(rows) != 0 || total != 5 {
		t.Errorf("past the end = %d rows of %d, want none of 5", len(rows), total)
	}

	ctx, admin := context.Background(), h.staff(model.RoleAdmin)
	if _, _, err := h.rs.StaffResults(ctx, h.scopeA(), admin, StaffResultFilter{Search: strings.Repeat("é", 100)}); err != nil {
		t.Errorf("100 runes: %v", err)
	}
	var input *InputError
	if _, _, err := h.rs.StaffResults(ctx, h.scopeA(), admin, StaffResultFilter{Search: strings.Repeat("é", 101)}); !errors.As(err, &input) || input.Fields["q"] != ReasonResultQuery {
		t.Errorf("101 runes err = %v, want a q field error", err)
	}
	for _, f := range []StaffResultFilter{{Limit: 101}, {Limit: -1}, {Offset: -1}} {
		if _, _, err := h.rs.StaffResults(ctx, h.scopeA(), admin, f); !errors.As(err, &input) {
			t.Errorf("filter %+v err = %v, want a validation error", f, err)
		}
	}
}

func TestStaffResultsAvatarIsResolvedLikeMe(t *testing.T) {
	h := newResultHarness(t)
	q := h.question(t)
	tr := h.tryoutOf(t, nil, q)
	picked, pickedP := h.named(t, tr, "Pilih Sendiri")
	auto, autoP := h.named(t, tr, "Otomatis")
	young := time.Date(2012, 5, 1, 0, 0, 0, 0, time.UTC)
	female := model.GenderFemale
	if _, err := h.pool.Exec(context.Background(), `UPDATE users SET avatar_key = 'a07' WHERE id = $1`, picked.ID); err != nil {
		t.Fatalf("pick avatar: %v", err)
	}
	if _, err := h.pool.Exec(context.Background(), `UPDATE users SET gender = $2, birth_date = $3 WHERE id = $1`, auto.ID, female, young); err != nil {
		t.Fatalf("set profile: %v", err)
	}
	right := map[uuid.UUID]string{q.ID: optID(t, q, "4")}
	h.take(t, pickedP, tr, right)
	h.take(t, autoP, tr, right)

	want := map[uuid.UUID]string{picked.ID: "a07", auto.ID: model.DefaultAvatarKey(auto.ID, &female, &young, time.Now())}
	rows, _ := h.staffRows(t, StaffResultFilter{})
	if len(rows) != 2 {
		t.Fatalf("rows = %d, want 2", len(rows))
	}
	for _, r := range rows {
		if r.Avatar != want[r.StudentID] || !avatarKeyPattern.MatchString(r.Avatar) {
			t.Errorf("%s avatar = %q, want %q", r.StudentName, r.Avatar, want[r.StudentID])
		}
	}
	detail, err := h.rs.StaffStudentResults(context.Background(), h.scopeA(), h.staff(model.RoleAdmin), auto.ID, tr.ID)
	if err != nil || detail.Student.Avatar != want[auto.ID] {
		t.Errorf("detail avatar = %q, %v, want %q", detail.Student.Avatar, err, want[auto.ID])
	}
}

func TestStaffStudentResultsDetail(t *testing.T) {
	h := newResultHarness(t)
	q := h.question(t)
	tr := h.tryoutOf(t, nil, q)
	user, p := h.named(t, tr, "Detail Siswa")
	_, rivalP := h.named(t, tr, "Rival")
	_, idleP := h.named(t, tr, "Belum Selesai")

	first := h.take(t, p, tr, map[uuid.UUID]string{q.ID: optID(t, q, "3")})
	h.clock.Advance(time.Minute)
	second := h.take(t, p, tr, map[uuid.UUID]string{q.ID: optID(t, q, "4")})
	h.clock.Advance(time.Minute)
	h.take(t, rivalP, tr, map[uuid.UUID]string{q.ID: optID(t, q, "4")})
	if _, err := h.rt.Start(context.Background(), h.scopeA(), idleP, tr.ID); err != nil {
		t.Fatalf("start idle attempt: %v", err)
	}
	ctx, admin := context.Background(), h.staff(model.RoleAdmin)

	got, err := h.rs.StaffStudentResults(ctx, h.scopeA(), admin, user.ID, tr.ID)
	if err != nil {
		t.Fatalf("detail: %v", err)
	}
	if got.Student.StudentName != "Detail Siswa" || got.Student.TryoutTitle != tr.Title || got.Student.ProgramName == "" || got.Student.ProgramCode == "" || got.Student.ScoringMode != "weighted" {
		t.Errorf("detail head = %+v, want student, tryout and program", got.Student)
	}
	own, err := h.rs.Detail(ctx, h.scopeA(), p, second)
	if err != nil || got.Ranking != own.Ranking || got.Ranking.Attempts != 2 || got.Ranking.Average != 50 || got.Ranking.Rank != 2 || got.Ranking.Participants != 2 {
		t.Errorf("ranking = %+v, student's own = %+v (err %v), want the same: average 50 over 2 attempts, rank 2 of 2", got.Ranking, own.Ranking, err)
	}
	if len(got.Attempts) != 2 || got.Attempts[0].SessionID != second || got.Attempts[1].SessionID != first {
		t.Fatalf("attempts = %+v, want the two completed ones, newest first", got.Attempts)
	}
	for i, wantScore := range []float64{100, 0} {
		a := got.Attempts[i]
		if a.Score != wantScore || a.AttemptNumber != 2-i || a.IRTWarning || len(a.Subtests) != 1 {
			t.Fatalf("attempt %d = %+v, want score %v, number %d and one subtest", i, a, wantScore, 2-i)
		}
		if s := a.Subtests[0]; s.Score != wantScore || s.QuestionCount != 1 || s.Name == "" || (wantScore == 100) != (s.CorrectCount == 1) {
			t.Errorf("attempt %d subtest = %+v, want score %v over 1 question", i, s, wantScore)
		}
	}

	other := h.q.b
	adminB := h.principal(other, model.RoleAdmin)
	for name, call := range map[string]func() error{
		"another tenant's admin": func() error {
			_, err := h.rs.StaffStudentResults(ctx, h.scope(other), adminB, user.ID, tr.ID)
			return err
		},
		"student without a completed attempt": func() error {
			_, err := h.rs.StaffStudentResults(ctx, h.scopeA(), admin, idleP.UserID, tr.ID)
			return err
		},
		"unknown tryout": func() error {
			_, err := h.rs.StaffStudentResults(ctx, h.scopeA(), admin, user.ID, uuid.New())
			return err
		},
		"unknown student": func() error {
			_, err := h.rs.StaffStudentResults(ctx, h.scopeA(), admin, uuid.New(), tr.ID)
			return err
		},
	} {
		if err := call(); !errors.Is(err, ErrStudentResultNotFound) {
			t.Errorf("%s: err = %v, want not found", name, err)
		}
	}
	if rows, total, err := h.rs.StaffResults(ctx, h.scope(other), adminB, StaffResultFilter{}); err != nil || total != 0 || len(rows) != 0 {
		t.Errorf("tenant B lists %d of %d rows (err %v), want none", len(rows), total, err)
	}
	if rows, _, _ := h.rs.StaffResults(ctx, h.scope(other), adminB, StaffResultFilter{Search: "Detail"}); len(rows) != 0 {
		t.Error("tenant B finds tenant A's student by name")
	}
	if _, _, err := h.rs.StaffResults(ctx, h.scopeA(), p, StaffResultFilter{}); !errors.Is(err, ErrRuntimeForbidden) {
		t.Errorf("student lists results: err = %v, want forbidden", err)
	}
	if _, err := h.rs.StaffStudentResults(ctx, h.scopeA(), p, user.ID, tr.ID); !errors.Is(err, ErrRuntimeForbidden) {
		t.Errorf("student reads the detail: err = %v, want forbidden", err)
	}
}

func TestStaffResultsOrderByActivityUnlessATryoutIsSelected(t *testing.T) {
	h := newResultHarness(t)
	q1, q2 := h.question(t), h.question(t)
	old, recent := h.tryoutOf(t, nil, q1), h.tryoutOf(t, nil, q2)
	right1, wrong1 := map[uuid.UUID]string{q1.ID: optID(t, q1, "4")}, map[uuid.UUID]string{q1.ID: optID(t, q1, "3")}
	right2, wrong2 := map[uuid.UUID]string{q2.ID: optID(t, q2, "4")}, map[uuid.UUID]string{q2.ID: optID(t, q2, "3")}

	_, adaP := h.named(t, old, "Ada")
	_, budiP := h.named(t, old, "Budi")
	_, ciciP := h.named(t, recent, "Cici")
	_, dodiP := h.named(t, recent, "Dodi")
	h.take(t, adaP, old, right1)
	h.clock.Advance(time.Minute)
	h.take(t, budiP, old, wrong1)
	h.clock.Advance(time.Minute)
	h.take(t, ciciP, recent, right2)
	h.clock.Advance(time.Minute)
	h.take(t, dodiP, recent, wrong2)

	names := func(f StaffResultFilter) string {
		rows, _ := h.staffRows(t, f)
		got := make([]string, len(rows))
		for i, r := range rows {
			got[i] = r.StudentName
		}
		return strings.Join(got, ",")
	}
	if got := names(StaffResultFilter{}); got != "Dodi,Cici,Budi,Ada" {
		t.Errorf("unfiltered order = %s, want most recent activity first (the new tryout's rank 2 before the old tryout's rank 1)", got)
	}
	if got := names(StaffResultFilter{TryoutID: &old.ID}); got != "Ada,Budi" {
		t.Errorf("old tryout order = %s, want rank first", got)
	}
	if got := names(StaffResultFilter{TryoutID: &recent.ID}); got != "Cici,Dodi" {
		t.Errorf("recent tryout order = %s, want rank first although Dodi was active last", got)
	}
}

func TestStaffResultsMixedScoringModesFlagIRTOnlyOnIRTRows(t *testing.T) {
	h := newResultHarness(t)
	q1, q2 := h.question(t), h.question(t)
	weighted := h.tryoutOf(t, nil, q1)
	irt := h.tryoutOf(t, func(in *TryoutInput) { in.ScoringMode = model.ScoringIRT }, q2)

	ada, adaP := h.named(t, weighted, "Ada")
	h.enroll(t, ada, irt.ID)
	budi, budiP := h.named(t, weighted, "Budi")
	h.take(t, adaP, weighted, map[uuid.UUID]string{q1.ID: optID(t, q1, "4")})
	h.take(t, budiP, weighted, map[uuid.UUID]string{q1.ID: optID(t, q1, "3")})
	h.take(t, adaP, irt, map[uuid.UUID]string{q2.ID: optID(t, q2, "4")})

	rows, total := h.staffRows(t, StaffResultFilter{})
	if total != 3 || len(rows) != 3 {
		t.Fatalf("rows = %d of %d, want 3", len(rows), total)
	}
	w, i, b := rowOf(t, rows, ada.ID, weighted.ID), rowOf(t, rows, ada.ID, irt.ID), rowOf(t, rows, budi.ID, weighted.ID)
	if w.IRTWarning || b.IRTWarning || !i.IRTWarning {
		t.Errorf("warnings weighted=%v budi=%v irt=%v, want only the IRT row warned (1 respondent < 3)", w.IRTWarning, b.IRTWarning, i.IRTWarning)
	}
	if w.Rank != 1 || w.Participants != 2 || w.Average != 100 || b.Rank != 2 || b.Participants != 2 {
		t.Errorf("weighted rows = %+v / %+v, want ranks 1 and 2 of 2", w, b)
	}

	ctx, admin := context.Background(), h.staff(model.RoleAdmin)
	if _, _, err := h.rs.Leaderboard(ctx, h.scopeA(), adaP, weighted.ID, 0, 0); !errors.Is(err, ErrRuntimeForbidden) {
		t.Errorf("student leaderboard err = %v, want forbidden", err)
	}
	if _, _, err := h.rs.Leaderboard(ctx, h.scopeA(), admin, uuid.New(), 0, 0); !errors.Is(err, ErrTryoutNotFound) {
		t.Errorf("unknown tryout leaderboard err = %v, want tryout not found", err)
	}
	if standings, total, err := h.rs.Leaderboard(ctx, h.scopeA(), admin, weighted.ID, 0, 0); err != nil || total != 2 || len(standings) != 2 {
		t.Errorf("leaderboard = %d of %d (err %v), want 2", len(standings), total, err)
	}
}
