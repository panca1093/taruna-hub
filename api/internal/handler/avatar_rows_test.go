package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"regexp"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/tarunaeduka/bimbel-tryout/api/internal/model"
)

var avatarKeyPattern = regexp.MustCompile(`^a(0[1-9]|[12][0-9]|3[0-5])$`)

func pickAvatar(t *testing.T, pool *pgxpool.Pool, id uuid.UUID, key string) {
	t.Helper()
	if _, err := pool.Exec(context.Background(), `UPDATE users SET avatar_key = $2 WHERE id = $1`, id, key); err != nil {
		t.Fatalf("pick avatar: %v", err)
	}
}

// requireAvatars reads list[idKey] -> avatarKey of a response and checks the picked key and the automatic one.
func requireAvatars(t *testing.T, rec *httptest.ResponseRecorder, list, idKey, avatarKey string, picked, automatic uuid.UUID, key string) {
	t.Helper()
	if rec.Code != http.StatusOK {
		t.Fatalf("%s: status = %d body = %s", list, rec.Code, rec.Body.String())
	}
	var body map[string]json.RawMessage
	var rows []map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("%s: %v", list, err)
	}
	if err := json.Unmarshal(body[list], &rows); err != nil {
		t.Fatalf("%s rows: %v", list, err)
	}
	got := map[string]string{}
	for _, row := range rows {
		id, _ := row[idKey].(string)
		avatar, ok := row[avatarKey].(string)
		if !ok {
			t.Fatalf("%s row %v has no string %s", list, row, avatarKey)
		}
		got[id] = avatar
	}
	if got[picked.String()] != key {
		t.Errorf("%s: picked student's %s = %q, want %q", list, avatarKey, got[picked.String()], key)
	}
	if !avatarKeyPattern.MatchString(got[automatic.String()]) {
		t.Errorf("%s: automatic %s = %q, want a built-in key", list, avatarKey, got[automatic.String()])
	}
	if want := model.DefaultAvatarKey(automatic, nil, nil, time.Now()); got[automatic.String()] != want {
		t.Errorf("%s: automatic %s = %q, want %q, the same face the profile shows", list, avatarKey, got[automatic.String()], want)
	}
}

func TestStaffRowsCarryAvatarOverHTTP(t *testing.T) {
	f := newRuntimeHTTPFixture(t)
	pickAvatar(t, f.pool, f.studentA.ID, "a07")
	f.enroll(t, f.studentA)
	f.enroll(t, f.secondStudent)

	for _, token := range []string{f.tokenA, f.tokenSecond} {
		sid := f.startSession(t, token).ID
		f.call(http.MethodPost, sessionPath(sid, "/subtests/"+f.sub(0)+"/start"), nil, token)
	}
	requireAvatars(t, f.call(http.MethodGet, activeSessionsPath, nil, f.tokenAdminA), "sessions", "user_id", "student_avatar",
		f.studentA.ID, f.secondStudent.ID, "a07")
}

func TestAdminUserRowsCarryAvatarOverHTTP(t *testing.T) {
	f := newUserOpsFixture(t)
	pickAvatar(t, f.pool, f.studentA.ID, "a07")
	users := f.call(http.MethodGet, "/api/v1/admin/users?role=student&limit=100", nil, f.tokenAdminA)
	requireAvatars(t, users, "users", "id", "avatar", f.studentA.ID, f.secondStudent.ID, "a07")
}

func TestLeaderboardRowsCarryOnlyAvatarOverHTTP(t *testing.T) {
	f := newRuntimeHTTPFixture(t)
	pickAvatar(t, f.pool, f.studentA.ID, "a07")
	f.enroll(t, f.studentA)
	f.enroll(t, f.secondStudent)
	f.completeAttempt(t, f.tokenA)
	f.completeAttempt(t, f.tokenSecond)

	rec := f.call(http.MethodGet, "/api/v1/admin/tryouts/"+f.tryout.ID.String()+"/leaderboard", nil, f.tokenAdminA)
	requireAvatars(t, rec, "entries", "user_id", "student_avatar", f.studentA.ID, f.secondStudent.ID, "a07")
	var body struct {
		Entries []map[string]any `json:"entries"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	for _, entry := range body.Entries {
		if len(entry) != 6 {
			t.Errorf("leaderboard entry keys = %v, want rank, user_id, student_name, student_avatar, average_score, attempts_counted", entry)
		}
	}
}

func TestTopupRowsCarryAvatarOverHTTP(t *testing.T) {
	f := newTopupHTTPFixture(t)
	pickAvatar(t, f.pool, f.studentA.ID, "a07")
	f.request(t, f.tokenA, "50000")
	f.request(t, f.tokenSecond, "50000")
	requireAvatars(t, f.get("/api/v1/admin/topups", f.tokenAdminA), "topups", "student_id", "student_avatar",
		f.studentA.ID, f.secondStudent.ID, "a07")
}

func TestOrderRowsCarryAvatarOverHTTP(t *testing.T) {
	f := newOrderHTTPFixture(t)
	pickAvatar(t, f.pool, f.studentA.ID, "a07")
	for _, s := range []struct {
		user  model.User
		token string
	}{{f.studentA, f.tokenA}, {f.secondStudent, f.tokenSecond}} {
		f.fund(t, s.user, "250000.00")
		if rec := f.post("/api/v1/orders", checkoutBody(f.product.ID.String(), "wallet", nil), s.token); rec.Code != http.StatusCreated {
			t.Fatalf("checkout: status = %d body = %s", rec.Code, rec.Body.String())
		}
	}
	requireAvatars(t, f.get("/api/v1/admin/orders", f.tokenAdminA), "orders", "student_id", "student_avatar",
		f.studentA.ID, f.secondStudent.ID, "a07")
}
