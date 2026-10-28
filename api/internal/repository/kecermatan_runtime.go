package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/tarunaeduka/bimbel-tryout/api/internal/model"
)

// ErrKecermatanActiveExists is the partial unique index that allows one attempt in progress per student and test.
var ErrKecermatanActiveExists = errors.New("an attempt is already in progress")

// ErrKecermatanCredit means a completion found no credit to consume; the CHECK on enrollments backs it.
var ErrKecermatanCredit = errors.New("no kecermatan credit to consume")

// KecermatanResultFilter is already validated by the service; the repository only turns it into SQL.
type KecermatanResultFilter struct {
	TestID    *uuid.UUID
	ProgramID *uuid.UUID
	Limit     int
	Offset    int
}

// KecermatanRuntimeRepository persists attempts, kolom timelines, answers and results. Every rule (credits,
// deadlines, scoring, ownership beyond the scope arguments) lives in the service; this only reads, writes and locks.
type KecermatanRuntimeRepository struct {
	db *pgxpool.Pool
}

func NewKecermatanRuntimeRepository(db *pgxpool.Pool) *KecermatanRuntimeRepository {
	return &KecermatanRuntimeRepository{db: db}
}

const kecSessionColumns = `s.id, s.tenant_id, s.user_id, s.test_id, t.title, s.attempt_number, s.status, s.seed,
	s.test_revision, s.kolom_count, s.soal_per_kolom, s.options_per_soal, s.kolom_duration_seconds, s.pause_seconds,
	s.due_at, s.started_at, s.completed_at`

const kecSessionFrom = ` FROM kecermatan_sessions s JOIN kecermatan_tests t ON t.id = s.test_id AND t.tenant_id = s.tenant_id`

func scanKecSession(row pgx.Row) (model.KecSession, error) {
	var s model.KecSession
	err := row.Scan(&s.ID, &s.TenantID, &s.UserID, &s.TestID, &s.TestTitle, &s.AttemptNumber, &s.Status, &s.Seed,
		&s.TestRevision, &s.KolomCount, &s.SoalPerKolom, &s.OptionsPerSoal, &s.KolomDurationSeconds, &s.PauseSeconds,
		&s.DueAt, &s.StartedAt, &s.CompletedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return model.KecSession{}, ErrNotFound
	}
	if err != nil {
		return model.KecSession{}, fmt.Errorf("scan kecermatan session: %w", err)
	}
	return s, nil
}

// LockCredit takes the student's enrollment row for the rest of the transaction; no row means no access.
// Every path that touches credits or an attempt takes this lock first, then the session row.
func (r *KecermatanRuntimeRepository) LockCredit(ctx context.Context, q dbtx, tenantID, userID, testID uuid.UUID) (model.KecCredit, error) {
	return r.credit(ctx, q, tenantID, userID, testID, " FOR UPDATE")
}

// Credit is the read-only view the overview uses.
func (r *KecermatanRuntimeRepository) Credit(ctx context.Context, q dbtx, tenantID, userID, testID uuid.UUID) (model.KecCredit, error) {
	return r.credit(ctx, q, tenantID, userID, testID, "")
}

func (r *KecermatanRuntimeRepository) credit(ctx context.Context, q dbtx, tenantID, userID, testID uuid.UUID, lock string) (model.KecCredit, error) {
	if tenantID == uuid.Nil {
		return model.KecCredit{}, ErrScopeRequired
	}
	var c model.KecCredit
	err := q.QueryRow(ctx, `
		SELECT id, COALESCE(attempts_purchased, 0), attempts_used FROM enrollments
		 WHERE tenant_id = $1 AND user_id = $2 AND kecermatan_test_id = $3`+lock,
		tenantID, userID, testID).Scan(&c.EnrollmentID, &c.Purchased, &c.Used)
	if errors.Is(err, pgx.ErrNoRows) {
		return model.KecCredit{}, ErrNotFound
	}
	if err != nil {
		return model.KecCredit{}, fmt.Errorf("read kecermatan credit: %w", err)
	}
	return c, nil
}

// ConsumeCredit adds one used attempt. It is the only place attempts_used moves, and the CHECK constraint
// attempts_used <= attempts_purchased makes an over-consume impossible even if the service were wrong.
func (r *KecermatanRuntimeRepository) ConsumeCredit(ctx context.Context, q dbtx, tenantID, enrollmentID uuid.UUID) error {
	tag, err := q.Exec(ctx, `
		UPDATE enrollments SET attempts_used = attempts_used + 1
		 WHERE tenant_id = $1 AND id = $2 AND attempts_used < attempts_purchased`, tenantID, enrollmentID)
	if err != nil {
		return fmt.Errorf("consume kecermatan credit: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrKecermatanCredit
	}
	return nil
}

// LockTest takes the row lock P10's edits take. The first attempt stamps first_attempt_at, which needs the
// exclusive lock; every later admission only needs a shared one, so students of one test do not queue.
func (r *KecermatanRuntimeRepository) LockTest(ctx context.Context, q dbtx, tenantID, id uuid.UUID, at time.Time) error {
	if tenantID == uuid.Nil {
		return ErrScopeRequired
	}
	var stamped bool
	err := q.QueryRow(ctx,
		`SELECT first_attempt_at IS NOT NULL FROM kecermatan_tests WHERE tenant_id = $1 AND id = $2`, tenantID, id).Scan(&stamped)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("read kecermatan stamp: %w", err)
	}
	mode := "FOR SHARE"
	if !stamped {
		mode = "FOR UPDATE"
	}
	var locked uuid.UUID
	if err := q.QueryRow(ctx,
		`SELECT id FROM kecermatan_tests WHERE tenant_id = $1 AND id = $2 `+mode, tenantID, id).Scan(&locked); err != nil {
		return fmt.Errorf("lock kecermatan test: %w", err)
	}
	if !stamped {
		if _, err := q.Exec(ctx,
			`UPDATE kecermatan_tests SET first_attempt_at = $3 WHERE tenant_id = $1 AND id = $2 AND first_attempt_at IS NULL`,
			tenantID, id, at); err != nil {
			return fmt.Errorf("stamp first kecermatan attempt: %w", err)
		}
	}
	return nil
}

// ActiveSession finds the student's attempt in progress on a test; lock takes its row.
func (r *KecermatanRuntimeRepository) ActiveSession(ctx context.Context, q dbtx, tenantID, userID, testID uuid.UUID, lock bool) (model.KecSession, error) {
	if tenantID == uuid.Nil {
		return model.KecSession{}, ErrScopeRequired
	}
	sql := `SELECT ` + kecSessionColumns + kecSessionFrom +
		` WHERE s.tenant_id = $1 AND s.user_id = $2 AND s.test_id = $3 AND s.status = 'in_progress'`
	if lock {
		sql += ` FOR UPDATE OF s`
	}
	return scanKecSession(q.QueryRow(ctx, sql, tenantID, userID, testID))
}

// FindSession reads one attempt; a zero userID skips the ownership filter (the worker and staff reads).
func (r *KecermatanRuntimeRepository) FindSession(ctx context.Context, q dbtx, tenantID, userID, id uuid.UUID, lock bool) (model.KecSession, error) {
	if tenantID == uuid.Nil {
		return model.KecSession{}, ErrScopeRequired
	}
	args := []any{tenantID, id}
	sql := `SELECT ` + kecSessionColumns + kecSessionFrom + ` WHERE s.tenant_id = $1 AND s.id = $2`
	if userID != uuid.Nil {
		args = append(args, userID)
		sql += ` AND s.user_id = $3`
	}
	if lock {
		sql += ` FOR UPDATE OF s`
	}
	return scanKecSession(q.QueryRow(ctx, sql, args...))
}

func (r *KecermatanRuntimeRepository) NextAttemptNumber(ctx context.Context, q dbtx, tenantID, userID, testID uuid.UUID) (int, error) {
	var n int
	if err := q.QueryRow(ctx, `
		SELECT COALESCE(max(attempt_number), 0) + 1 FROM kecermatan_sessions
		 WHERE tenant_id = $1 AND user_id = $2 AND test_id = $3`, tenantID, userID, testID).Scan(&n); err != nil {
		return 0, fmt.Errorf("next kecermatan attempt number: %w", err)
	}
	return n, nil
}

// CreateSession writes the attempt, its frozen columns and one pending row per kolom.
func (r *KecermatanRuntimeRepository) CreateSession(ctx context.Context, q dbtx, s model.KecSession, columns []byte) (uuid.UUID, error) {
	var id uuid.UUID
	err := q.QueryRow(ctx, `
		INSERT INTO kecermatan_sessions (tenant_id, user_id, test_id, attempt_number, seed, test_revision, kolom_count,
		        soal_per_kolom, options_per_soal, kolom_duration_seconds, pause_seconds, columns, due_at, started_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
		RETURNING id`,
		s.TenantID, s.UserID, s.TestID, s.AttemptNumber, s.Seed, s.TestRevision, s.KolomCount, s.SoalPerKolom,
		s.OptionsPerSoal, s.KolomDurationSeconds, s.PauseSeconds, columns, s.DueAt, s.StartedAt).Scan(&id)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == pgUniqueViolation && pgErr.ConstraintName == "kecermatan_sessions_active_key" {
			return uuid.Nil, ErrKecermatanActiveExists
		}
		return uuid.Nil, fmt.Errorf("create kecermatan session: %w", err)
	}
	if _, err := q.Exec(ctx, `
		INSERT INTO kecermatan_session_kolom (session_id, tenant_id, kolom_number)
		SELECT $1, $2, n FROM generate_series(1, $3::int) AS n`, id, s.TenantID, s.KolomCount); err != nil {
		return uuid.Nil, fmt.Errorf("create kecermatan kolom: %w", err)
	}
	return id, nil
}

// Columns is the frozen snapshot: the generated N/2 columns as JSON.
func (r *KecermatanRuntimeRepository) Columns(ctx context.Context, q dbtx, tenantID, sessionID uuid.UUID) ([]byte, error) {
	var raw []byte
	err := q.QueryRow(ctx, `SELECT columns FROM kecermatan_sessions WHERE tenant_id = $1 AND id = $2`,
		tenantID, sessionID).Scan(&raw)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("read kecermatan columns: %w", err)
	}
	return raw, nil
}

func (r *KecermatanRuntimeRepository) Kolom(ctx context.Context, q dbtx, tenantID, sessionID uuid.UUID) ([]model.KecKolom, error) {
	rows, err := q.Query(ctx, `
		SELECT kolom_number, status, started_at, deadline_at, benar, salah, kosong
		  FROM kecermatan_session_kolom WHERE tenant_id = $1 AND session_id = $2 ORDER BY kolom_number`, tenantID, sessionID)
	if err != nil {
		return nil, fmt.Errorf("read kecermatan kolom: %w", err)
	}
	defer rows.Close()
	var out []model.KecKolom
	for rows.Next() {
		var k model.KecKolom
		if err := rows.Scan(&k.Number, &k.Status, &k.StartedAt, &k.DeadlineAt, &k.Benar, &k.Salah, &k.Kosong); err != nil {
			return nil, fmt.Errorf("scan kecermatan kolom: %w", err)
		}
		out = append(out, k)
	}
	return out, rows.Err()
}

func (r *KecermatanRuntimeRepository) StartKolom(ctx context.Context, q dbtx, tenantID, sessionID uuid.UUID, number int, start, deadline time.Time) error {
	tag, err := q.Exec(ctx, `
		UPDATE kecermatan_session_kolom SET status = 'running', started_at = $4, deadline_at = $5
		 WHERE tenant_id = $1 AND session_id = $2 AND kolom_number = $3 AND status = 'pending'`,
		tenantID, sessionID, number, start, deadline)
	if err != nil {
		return fmt.Errorf("start kecermatan kolom: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// EndKolom writes the counts once; an ended row never changes again.
func (r *KecermatanRuntimeRepository) EndKolom(ctx context.Context, q dbtx, tenantID, sessionID uuid.UUID, number, benar, salah, kosong int) error {
	tag, err := q.Exec(ctx, `
		UPDATE kecermatan_session_kolom SET status = 'ended', benar = $4, salah = $5, kosong = $6
		 WHERE tenant_id = $1 AND session_id = $2 AND kolom_number = $3 AND status = 'running'`,
		tenantID, sessionID, number, benar, salah, kosong)
	if err != nil {
		return fmt.Errorf("end kecermatan kolom: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *KecermatanRuntimeRepository) SetDue(ctx context.Context, q dbtx, tenantID, sessionID uuid.UUID, due time.Time) error {
	if _, err := q.Exec(ctx, `UPDATE kecermatan_sessions SET due_at = $3 WHERE tenant_id = $1 AND id = $2`,
		tenantID, sessionID, due); err != nil {
		return fmt.Errorf("set kecermatan due: %w", err)
	}
	return nil
}

func (r *KecermatanRuntimeRepository) CompleteSession(ctx context.Context, q dbtx, tenantID, sessionID uuid.UUID, at time.Time) error {
	tag, err := q.Exec(ctx, `
		UPDATE kecermatan_sessions SET status = 'completed', completed_at = $3, due_at = NULL
		 WHERE tenant_id = $1 AND id = $2 AND status = 'in_progress'`, tenantID, sessionID, at)
	if err != nil {
		return fmt.Errorf("complete kecermatan session: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// Answers reads the stored answers of one kolom, or of every kolom when kolom is 0.
func (r *KecermatanRuntimeRepository) Answers(ctx context.Context, q dbtx, tenantID, sessionID uuid.UUID, kolom int) ([]model.KecAnswer, error) {
	sql := `SELECT kolom_number, soal_number, answer FROM kecermatan_session_answers WHERE tenant_id = $1 AND session_id = $2`
	args := []any{tenantID, sessionID}
	if kolom > 0 {
		sql += ` AND kolom_number = $3`
		args = append(args, kolom)
	}
	rows, err := q.Query(ctx, sql+` ORDER BY kolom_number, soal_number`, args...)
	if err != nil {
		return nil, fmt.Errorf("read kecermatan answers: %w", err)
	}
	defer rows.Close()
	var out []model.KecAnswer
	for rows.Next() {
		var a model.KecAnswer
		if err := rows.Scan(&a.Kolom, &a.Soal, &a.Answer); err != nil {
			return nil, fmt.Errorf("scan kecermatan answer: %w", err)
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// PutAnswer stores an answer once. It returns what is stored afterwards and whether this call wrote it, so a
// retry of the same answer is harmless and a different answer for the same soal is visible to the caller.
func (r *KecermatanRuntimeRepository) PutAnswer(ctx context.Context, q dbtx, tenantID, sessionID uuid.UUID, kolom, soal int, answer *string, at time.Time) (*string, bool, error) {
	tag, err := q.Exec(ctx, `
		INSERT INTO kecermatan_session_answers (session_id, tenant_id, kolom_number, soal_number, answer, answered_at)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (session_id, kolom_number, soal_number) DO NOTHING`,
		sessionID, tenantID, kolom, soal, answer, at)
	if err != nil {
		return nil, false, fmt.Errorf("store kecermatan answer: %w", err)
	}
	if tag.RowsAffected() == 1 {
		return answer, true, nil
	}
	var stored *string
	if err := q.QueryRow(ctx, `
		SELECT answer FROM kecermatan_session_answers
		 WHERE session_id = $1 AND kolom_number = $2 AND soal_number = $3`, sessionID, kolom, soal).Scan(&stored); err != nil {
		return nil, false, fmt.Errorf("read kecermatan answer: %w", err)
	}
	return stored, false, nil
}

// InsertResult writes the one result row of an attempt.
func (r *KecermatanRuntimeRepository) InsertResult(ctx context.Context, q dbtx, res model.KecResult, raw [3]float64) error {
	if _, err := q.Exec(ctx, `
		INSERT INTO kecermatan_results (session_id, tenant_id, user_id, test_id, attempt_number, benar_total, salah_total,
		        kosong_total, kecepatan, ketahanan, ketelitian, nilai_akhir, kecepatan_raw, ketahanan_raw, ketelitian_raw,
		        scoring_version, completed_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17)`,
		res.SessionID, res.TenantID, res.UserID, res.TestID, res.AttemptNumber, res.BenarTotal, res.SalahTotal,
		res.KosongTotal, res.Kecepatan, res.Ketahanan, res.Ketelitian, res.NilaiAkhir, raw[0], raw[1], raw[2],
		res.ScoringVersion, res.CompletedAt); err != nil {
		return fmt.Errorf("insert kecermatan result: %w", err)
	}
	return nil
}

// UpdateScores rewrites only the composite scores and the version; the counts are immutable by trigger.
func (r *KecermatanRuntimeRepository) UpdateScores(ctx context.Context, q dbtx, tenantID, sessionID uuid.UUID, version int, kecepatan, ketahanan, ketelitian, nilai float64, raw [3]float64) error {
	if _, err := q.Exec(ctx, `
		UPDATE kecermatan_results SET kecepatan = $3, ketahanan = $4, ketelitian = $5, nilai_akhir = $6,
		       kecepatan_raw = $7, ketahanan_raw = $8, ketelitian_raw = $9, scoring_version = $10
		 WHERE tenant_id = $1 AND session_id = $2`,
		tenantID, sessionID, kecepatan, ketahanan, ketelitian, nilai, raw[0], raw[1], raw[2], version); err != nil {
		return fmt.Errorf("update kecermatan scores: %w", err)
	}
	return nil
}

// ResultSessions pages the sessions that have a result, in id order, for a recompute.
func (r *KecermatanRuntimeRepository) ResultSessions(ctx context.Context, q dbtx, tenantID, after uuid.UUID, limit int) ([]uuid.UUID, error) {
	rows, err := q.Query(ctx, `
		SELECT session_id FROM kecermatan_results WHERE tenant_id = $1 AND session_id > $2 ORDER BY session_id LIMIT $3`,
		tenantID, after, limit)
	if err != nil {
		return nil, fmt.Errorf("page kecermatan results: %w", err)
	}
	defer rows.Close()
	var out []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan kecermatan result id: %w", err)
		}
		out = append(out, id)
	}
	return out, rows.Err()
}

const kecResultColumns = `r.session_id, r.tenant_id, r.user_id, u.name, r.test_id, t.title, t.program_id, pr.code, r.attempt_number,
	r.benar_total, r.salah_total, r.kosong_total, r.kecepatan::float8, r.ketahanan::float8, r.ketelitian::float8,
	r.nilai_akhir::float8, r.scoring_version, r.completed_at`

const kecResultFrom = ` FROM kecermatan_results r
	JOIN users u ON u.id = r.user_id AND u.tenant_id = r.tenant_id
	JOIN kecermatan_tests t ON t.id = r.test_id AND t.tenant_id = r.tenant_id
	JOIN programs pr ON pr.id = t.program_id AND pr.tenant_id = t.tenant_id`

func scanKecResult(row pgx.Row) (model.KecResultRow, error) {
	var r model.KecResultRow
	err := row.Scan(&r.SessionID, &r.TenantID, &r.UserID, &r.StudentName, &r.TestID, &r.TestTitle, &r.ProgramID,
		&r.ProgramCode, &r.AttemptNumber, &r.BenarTotal, &r.SalahTotal, &r.KosongTotal, &r.Kecepatan, &r.Ketahanan,
		&r.Ketelitian, &r.NilaiAkhir, &r.ScoringVersion, &r.CompletedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return model.KecResultRow{}, ErrNotFound
	}
	if err != nil {
		return model.KecResultRow{}, fmt.Errorf("scan kecermatan result: %w", err)
	}
	return r, nil
}

// Result reads one attempt's result; a zero userID skips the ownership filter (staff).
func (r *KecermatanRuntimeRepository) Result(ctx context.Context, q dbtx, tenantID, userID, sessionID uuid.UUID) (model.KecResultRow, error) {
	if tenantID == uuid.Nil {
		return model.KecResultRow{}, ErrScopeRequired
	}
	args := []any{tenantID, sessionID}
	sql := `SELECT ` + kecResultColumns + kecResultFrom + ` WHERE r.tenant_id = $1 AND r.session_id = $2`
	if userID != uuid.Nil {
		args = append(args, userID)
		sql += ` AND r.user_id = $3`
	}
	return scanKecResult(q.QueryRow(ctx, sql, args...))
}

func (r *KecermatanRuntimeRepository) ListResults(ctx context.Context, tenantID uuid.UUID, f KecermatanResultFilter) ([]model.KecResultRow, int, error) {
	if tenantID == uuid.Nil {
		return nil, 0, ErrScopeRequired
	}
	where := []string{"r.tenant_id = $1"}
	args := []any{tenantID}
	if f.TestID != nil {
		args = append(args, *f.TestID)
		where = append(where, fmt.Sprintf("r.test_id = $%d", len(args)))
	}
	if f.ProgramID != nil {
		args = append(args, *f.ProgramID)
		where = append(where, fmt.Sprintf("t.program_id = $%d", len(args)))
	}
	from := kecResultFrom + ` WHERE ` + strings.Join(where, " AND ")
	var total int
	if err := r.db.QueryRow(ctx, `SELECT count(*)`+from, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count kecermatan results: %w", err)
	}
	args = append(args, f.Limit, f.Offset)
	rows, err := r.db.Query(ctx, `SELECT `+kecResultColumns+from+
		fmt.Sprintf(` ORDER BY r.completed_at DESC, r.session_id DESC LIMIT $%d OFFSET $%d`, len(args)-1, len(args)), args...)
	if err != nil {
		return nil, 0, fmt.Errorf("list kecermatan results: %w", err)
	}
	defer rows.Close()
	out := make([]model.KecResultRow, 0, f.Limit)
	for rows.Next() {
		row, err := scanKecResult(rows)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, row)
	}
	return out, total, rows.Err()
}

// Attempts is a student's history on one test, newest first.
func (r *KecermatanRuntimeRepository) Attempts(ctx context.Context, q dbtx, tenantID, userID, testID uuid.UUID) ([]model.KecAttempt, error) {
	rows, err := q.Query(ctx, `
		SELECT s.id, s.attempt_number, s.status, s.started_at, s.completed_at, r.nilai_akhir::float8
		  FROM kecermatan_sessions s LEFT JOIN kecermatan_results r ON r.session_id = s.id
		 WHERE s.tenant_id = $1 AND s.user_id = $2 AND s.test_id = $3
		 ORDER BY s.attempt_number DESC`, tenantID, userID, testID)
	if err != nil {
		return nil, fmt.Errorf("list kecermatan attempts: %w", err)
	}
	defer rows.Close()
	out := []model.KecAttempt{}
	for rows.Next() {
		var a model.KecAttempt
		if err := rows.Scan(&a.SessionID, &a.AttemptNumber, &a.Status, &a.StartedAt, &a.CompletedAt, &a.NilaiAkhir); err != nil {
			return nil, fmt.Errorf("scan kecermatan attempt: %w", err)
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// StudentTests lists the kecermatan tests a student holds credits for.
func (r *KecermatanRuntimeRepository) StudentTests(ctx context.Context, tenantID, userID uuid.UUID) ([]model.KecStudentTest, error) {
	if tenantID == uuid.Nil {
		return nil, ErrScopeRequired
	}
	return r.studentTests(ctx, tenantID, userID, uuid.Nil)
}

// StudentTest is one such test, or ErrNotFound when the student holds none.
func (r *KecermatanRuntimeRepository) StudentTest(ctx context.Context, tenantID, userID, testID uuid.UUID) (model.KecStudentTest, error) {
	if tenantID == uuid.Nil {
		return model.KecStudentTest{}, ErrScopeRequired
	}
	list, err := r.studentTests(ctx, tenantID, userID, testID)
	if err != nil {
		return model.KecStudentTest{}, err
	}
	if len(list) == 0 {
		return model.KecStudentTest{}, ErrNotFound
	}
	return list[0], nil
}

func (r *KecermatanRuntimeRepository) studentTests(ctx context.Context, tenantID, userID, testID uuid.UUID) ([]model.KecStudentTest, error) {
	rows, err := r.db.Query(ctx, `
		SELECT t.id, t.title, t.description, pr.code, pr.name, t.options_per_soal, t.kolom_count, t.soal_per_kolom,
		       t.kolom_duration_seconds, e.attempts_purchased, e.attempts_used,
		       EXISTS (SELECT 1 FROM kecermatan_sessions s WHERE s.tenant_id = e.tenant_id AND s.user_id = e.user_id
		                AND s.test_id = t.id AND s.status = 'in_progress')
		  FROM enrollments e
		  JOIN kecermatan_tests t ON t.id = e.kecermatan_test_id AND t.tenant_id = e.tenant_id
		  JOIN programs pr ON pr.id = t.program_id AND pr.tenant_id = t.tenant_id
		 WHERE e.tenant_id = $1 AND e.user_id = $2 AND ($3 = '00000000-0000-0000-0000-000000000000'::uuid OR t.id = $3)
		 ORDER BY e.created_at DESC, e.id DESC`, tenantID, userID, testID)
	if err != nil {
		return nil, fmt.Errorf("list kecermatan enrollments: %w", err)
	}
	defer rows.Close()
	out := []model.KecStudentTest{}
	for rows.Next() {
		var t model.KecStudentTest
		if err := rows.Scan(&t.TestID, &t.Title, &t.Description, &t.ProgramCode, &t.ProgramName, &t.OptionsPerSoal,
			&t.KolomCount, &t.SoalPerKolom, &t.KolomDurationSeconds, &t.Purchased, &t.Used, &t.InProgress); err != nil {
			return nil, fmt.Errorf("scan kecermatan enrollment: %w", err)
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// SellableProduct is the product a student can buy more attempts from, or ErrNotFound.
func (r *KecermatanRuntimeRepository) SellableProduct(ctx context.Context, tenantID, testID uuid.UUID, now time.Time) (uuid.UUID, error) {
	if tenantID == uuid.Nil {
		return uuid.Nil, ErrScopeRequired
	}
	var id uuid.UUID
	err := r.db.QueryRow(ctx, `SELECT p.id FROM products p WHERE p.tenant_id = $1 AND `+publicVisible+
		` AND p.kecermatan_test_id = $3 ORDER BY p.created_at, p.id LIMIT 1`, tenantID, now, testID).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return uuid.Nil, ErrNotFound
	}
	if err != nil {
		return uuid.Nil, fmt.Errorf("read sellable kecermatan product: %w", err)
	}
	return id, nil
}

// DueSessions is the worker's scan: attempts whose next timeline instant has passed.
func (r *KecermatanRuntimeRepository) DueSessions(ctx context.Context, now time.Time, limit int) ([]model.KecSession, error) {
	rows, err := r.db.Query(ctx, `
		SELECT `+kecSessionColumns+kecSessionFrom+`
		 WHERE s.status = 'in_progress' AND s.due_at <= $1 ORDER BY s.due_at LIMIT $2`, now, limit)
	if err != nil {
		return nil, fmt.Errorf("scan due kecermatan sessions: %w", err)
	}
	defer rows.Close()
	var out []model.KecSession
	for rows.Next() {
		s, err := scanKecSession(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}
