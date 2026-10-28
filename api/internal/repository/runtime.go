package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/tarunaeduka/bimbel-tryout/api/internal/model"
)

// ErrActiveSession means another in-progress session of the same student and tryout won the race.
var ErrActiveSession = errors.New("active session exists")

type dbtx interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// RuntimeRepository persists sessions, parts, the snapshot, answers and results. Every rule (windows, deadlines,
// scoring, ownership beyond the scope arguments) lives in the service; this only reads, writes and locks.
type RuntimeRepository struct {
	db *pgxpool.Pool
}

func NewRuntimeRepository(db *pgxpool.Pool) *RuntimeRepository {
	return &RuntimeRepository{db: db}
}

func (r *RuntimeRepository) DB() *pgxpool.Pool { return r.db }

const attemptColumns = `s.id, s.tenant_id, s.user_id, s.tryout_id, s.attempt_number, s.status, s.scoring_mode,
	s.scoring_version, s.weight_easy::float8, s.weight_medium::float8, s.weight_hard::float8, s.seed,
	s.started_at, s.completed_at, t.title, t.active_until, t.grace_seconds`

func scanAttempt(row pgx.Row) (model.Attempt, error) {
	var a model.Attempt
	err := row.Scan(&a.ID, &a.TenantID, &a.UserID, &a.TryoutID, &a.AttemptNumber, &a.Status, &a.ScoringMode,
		&a.ScoringVersion, &a.Weights.Easy, &a.Weights.Medium, &a.Weights.Hard, &a.Seed,
		&a.StartedAt, &a.CompletedAt, &a.TryoutTitle, &a.ActiveUntil, &a.GraceSeconds)
	if errors.Is(err, pgx.ErrNoRows) {
		return model.Attempt{}, ErrNotFound
	}
	if err != nil {
		return model.Attempt{}, fmt.Errorf("scan session: %w", err)
	}
	return a, nil
}

// LockEnrollment serializes every start of one student on one tryout; no row means no access.
func (r *RuntimeRepository) LockEnrollment(ctx context.Context, q dbtx, tenantID, userID, tryoutID uuid.UUID) error {
	if tenantID == uuid.Nil {
		return ErrScopeRequired
	}
	var id uuid.UUID
	err := q.QueryRow(ctx, `
		SELECT id FROM enrollments WHERE tenant_id = $1 AND user_id = $2 AND tryout_id = $3 FOR UPDATE`,
		tenantID, userID, tryoutID).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("lock enrollment: %w", err)
	}
	return nil
}

// HasEnrollment is the read-only access check the overview uses.
func (r *RuntimeRepository) HasEnrollment(ctx context.Context, q dbtx, tenantID, userID, tryoutID uuid.UUID) (bool, error) {
	if tenantID == uuid.Nil {
		return false, ErrScopeRequired
	}
	var ok bool
	err := q.QueryRow(ctx, `
		SELECT EXISTS (SELECT 1 FROM enrollments WHERE tenant_id = $1 AND user_id = $2 AND tryout_id = $3)`,
		tenantID, userID, tryoutID).Scan(&ok)
	if err != nil {
		return false, fmt.Errorf("check enrollment: %w", err)
	}
	return ok, nil
}

// LockTryout takes the row lock P08's edits take. The first attempt stamps first_attempt_at, which needs the
// exclusive lock; every later admission only needs a shared one, so students of one tryout do not queue.
func (r *RuntimeRepository) LockTryout(ctx context.Context, q dbtx, tenantID, id uuid.UUID, at time.Time) (model.Tryout, error) {
	if tenantID == uuid.Nil {
		return model.Tryout{}, ErrScopeRequired
	}
	var stamped bool
	err := q.QueryRow(ctx,
		`SELECT first_attempt_at IS NOT NULL FROM tryouts WHERE tenant_id = $1 AND id = $2`, tenantID, id).Scan(&stamped)
	if errors.Is(err, pgx.ErrNoRows) {
		return model.Tryout{}, ErrNotFound
	}
	if err != nil {
		return model.Tryout{}, fmt.Errorf("read tryout stamp: %w", err)
	}
	mode := "FOR SHARE"
	if !stamped {
		mode = "FOR UPDATE"
	}
	out, err := scanTryoutHead(q.QueryRow(ctx,
		`SELECT `+tryoutHeadColumns+` FROM tryouts WHERE tenant_id = $1 AND id = $2 `+mode, tenantID, id), false)
	if err != nil {
		return model.Tryout{}, err
	}
	if !stamped {
		if _, err := q.Exec(ctx,
			`UPDATE tryouts SET first_attempt_at = $3 WHERE tenant_id = $1 AND id = $2 AND first_attempt_at IS NULL`,
			tenantID, id, at); err != nil {
			return model.Tryout{}, fmt.Errorf("stamp first attempt: %w", err)
		}
	}
	return out, nil
}

// Weights is the tenant's current setting, or the D04 defaults when it never saved one.
func (r *RuntimeRepository) Weights(ctx context.Context, q dbtx, tenantID uuid.UUID) (model.Weights, error) {
	if tenantID == uuid.Nil {
		return model.Weights{}, ErrScopeRequired
	}
	var w model.Weights
	err := q.QueryRow(ctx, `
		SELECT weight_easy::float8, weight_medium::float8, weight_hard::float8
		  FROM tenant_settings WHERE tenant_id = $1`, tenantID).Scan(&w.Easy, &w.Medium, &w.Hard)
	if errors.Is(err, pgx.ErrNoRows) {
		return model.DefaultWeights, nil
	}
	if err != nil {
		return model.Weights{}, fmt.Errorf("read weights: %w", err)
	}
	return w, nil
}

// SourceSubtests reads the frozen assignment the snapshot copies: subtests in order, questions in tryout order,
// each with the option ids of the version the tryout froze.
func (r *RuntimeRepository) SourceSubtests(ctx context.Context, q dbtx, tenantID, tryoutID uuid.UUID) ([]model.SourceSubtest, error) {
	if tenantID == uuid.Nil {
		return nil, ErrScopeRequired
	}
	rows, err := q.Query(ctx,
		`SELECT id, name, duration_seconds, position FROM tryout_subtests
		  WHERE tenant_id = $1 AND tryout_id = $2 ORDER BY position`, tenantID, tryoutID)
	if err != nil {
		return nil, fmt.Errorf("read source subtests: %w", err)
	}
	var out []model.SourceSubtest
	index := map[uuid.UUID]int{}
	for rows.Next() {
		var st model.SourceSubtest
		if err := rows.Scan(&st.ID, &st.Name, &st.DurationSeconds, &st.Position); err != nil {
			rows.Close()
			return nil, fmt.Errorf("scan source subtest: %w", err)
		}
		index[st.ID] = len(out)
		out = append(out, st)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read source subtests: %w", err)
	}

	rows, err = q.Query(ctx, `
		SELECT tq.subtest_id, tq.question_id, tq.question_version_id, v.type, v.options
		  FROM tryout_questions tq
		  JOIN question_versions v ON v.id = tq.question_version_id AND v.tenant_id = tq.tenant_id
		 WHERE tq.tenant_id = $1 AND tq.tryout_id = $2
		 ORDER BY tq.position`, tenantID, tryoutID)
	if err != nil {
		return nil, fmt.Errorf("read source questions: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var sq model.SourceQuestion
		var raw []byte
		if err := rows.Scan(&sq.SubtestID, &sq.QuestionID, &sq.VersionID, &sq.Type, &raw); err != nil {
			return nil, fmt.Errorf("scan source question: %w", err)
		}
		var options []model.QuestionOption
		if err := json.Unmarshal(raw, &options); err != nil {
			return nil, fmt.Errorf("decode source options: %w", err)
		}
		for _, option := range options {
			sq.OptionIDs = append(sq.OptionIDs, option.ID)
		}
		if at, ok := index[sq.SubtestID]; ok {
			out[at].Questions = append(out[at].Questions, sq)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read source questions: %w", err)
	}
	return out, nil
}

// ActiveAttempt is the student's in-progress session on this tryout, if any. A writer passes forUpdate to hold the
// session row before settling it, so the lock order stays enrollment, then session.
func (r *RuntimeRepository) ActiveAttempt(ctx context.Context, q dbtx, tenantID, userID, tryoutID uuid.UUID, forUpdate bool) (model.Attempt, error) {
	if tenantID == uuid.Nil {
		return model.Attempt{}, ErrScopeRequired
	}
	sql := `SELECT ` + attemptColumns + `
		  FROM tryout_sessions s
		  JOIN tryouts t ON t.id = s.tryout_id AND t.tenant_id = s.tenant_id
		 WHERE s.tenant_id = $1 AND s.user_id = $2 AND s.tryout_id = $3 AND s.status = 'in_progress'`
	if forUpdate {
		sql += ` FOR UPDATE OF s`
	}
	return scanAttempt(q.QueryRow(ctx, sql, tenantID, userID, tryoutID))
}

// NextAttemptNumber is only safe under LockEnrollment, which serializes the student's starts.
func (r *RuntimeRepository) NextAttemptNumber(ctx context.Context, q dbtx, tenantID, userID, tryoutID uuid.UUID) (int, error) {
	if tenantID == uuid.Nil {
		return 0, ErrScopeRequired
	}
	var n int
	err := q.QueryRow(ctx, `
		SELECT coalesce(max(attempt_number), 0) + 1 FROM tryout_sessions
		 WHERE tenant_id = $1 AND user_id = $2 AND tryout_id = $3`, tenantID, userID, tryoutID).Scan(&n)
	if err != nil {
		return 0, fmt.Errorf("next attempt number: %w", err)
	}
	return n, nil
}

// CreateAttempt writes the session, its parts and the whole snapshot. The partial unique index turns a lost race
// into ErrActiveSession.
func (r *RuntimeRepository) CreateAttempt(ctx context.Context, q dbtx, a model.Attempt, parts []model.AttemptPart, questions []OrderedQuestion) (uuid.UUID, error) {
	if a.TenantID == uuid.Nil {
		return uuid.Nil, ErrScopeRequired
	}
	var id uuid.UUID
	err := q.QueryRow(ctx, `
		INSERT INTO tryout_sessions (tenant_id, user_id, tryout_id, attempt_number, scoring_mode, scoring_version,
		        weight_easy, weight_medium, weight_hard, seed, started_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		RETURNING id`,
		a.TenantID, a.UserID, a.TryoutID, a.AttemptNumber, a.ScoringMode, a.ScoringVersion,
		a.Weights.Easy, a.Weights.Medium, a.Weights.Hard, a.Seed, a.StartedAt).Scan(&id)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == pgUniqueViolation && pgErr.ConstraintName == "tryout_sessions_active_key" {
			return uuid.Nil, ErrActiveSession
		}
		return uuid.Nil, fmt.Errorf("create session: %w", err)
	}
	batch := &pgx.Batch{}
	for _, p := range parts {
		batch.Queue(`
			INSERT INTO tryout_session_parts (session_id, tenant_id, subtest_id, position, name, duration_seconds, question_count)
			VALUES ($1, $2, $3, $4, $5, $6, $7)`,
			id, a.TenantID, p.SubtestID, p.Position, p.Name, p.DurationSeconds, p.QuestionCount)
	}
	if err := drainBatch(ctx, q, batch, "create session part"); err != nil {
		return uuid.Nil, err
	}
	batch = &pgx.Batch{}
	for _, oq := range questions {
		order, err := json.Marshal(oq.OptionOrder)
		if err != nil {
			return uuid.Nil, fmt.Errorf("encode option order: %w", err)
		}
		batch.Queue(`
			INSERT INTO tryout_session_questions (session_id, tenant_id, subtest_id, position, question_id,
			        question_version_id, option_order)
			VALUES ($1, $2, $3, $4, $5, $6, $7)`,
			id, a.TenantID, oq.SubtestID, oq.Position, oq.QuestionID, oq.VersionID, order)
	}
	if err := drainBatch(ctx, q, batch, "create session question"); err != nil {
		return uuid.Nil, err
	}
	return id, nil
}

// OrderedQuestion is one snapshot row: the question in the position and option order this session shows.
type OrderedQuestion struct {
	SubtestID   uuid.UUID
	Position    int
	QuestionID  uuid.UUID
	VersionID   uuid.UUID
	OptionOrder []string
}

func drainBatch(ctx context.Context, q dbtx, batch *pgx.Batch, what string) error {
	if batch.Len() == 0 {
		return nil
	}
	sender, ok := q.(interface {
		SendBatch(context.Context, *pgx.Batch) pgx.BatchResults
	})
	if !ok {
		return fmt.Errorf("%s: no batch support", what)
	}
	results := sender.SendBatch(ctx, batch)
	defer results.Close()
	for i := 0; i < batch.Len(); i++ {
		if _, err := results.Exec(); err != nil {
			return fmt.Errorf("%s: %w", what, err)
		}
	}
	return nil
}

// FindAttempt reads one session. A zero userID reads any student's (staff routes); otherwise the row must belong
// to that user, so another student's id is indistinguishable from a missing one.
func (r *RuntimeRepository) FindAttempt(ctx context.Context, q dbtx, tenantID, userID, id uuid.UUID, forUpdate bool) (model.Attempt, error) {
	if tenantID == uuid.Nil {
		return model.Attempt{}, ErrScopeRequired
	}
	sql := `SELECT ` + attemptColumns + `
		  FROM tryout_sessions s
		  JOIN tryouts t ON t.id = s.tryout_id AND t.tenant_id = s.tenant_id
		 WHERE s.tenant_id = $1 AND s.id = $2 AND ($3 = '00000000-0000-0000-0000-000000000000'::uuid OR s.user_id = $3)`
	if forUpdate {
		sql += ` FOR UPDATE OF s`
	}
	return scanAttempt(q.QueryRow(ctx, sql, tenantID, id, userID))
}

func (r *RuntimeRepository) Parts(ctx context.Context, q dbtx, tenantID, sessionID uuid.UUID) ([]model.AttemptPart, error) {
	if tenantID == uuid.Nil {
		return nil, ErrScopeRequired
	}
	rows, err := q.Query(ctx, `
		SELECT session_id, subtest_id, position, name, duration_seconds, status, started_at, deadline_at,
		       extra_seconds, submitted_at, coalesce(submit_reason, ''), question_count,
		       earned_points::float8, max_points::float8, correct_count
		  FROM tryout_session_parts
		 WHERE tenant_id = $1 AND session_id = $2 ORDER BY position`, tenantID, sessionID)
	if err != nil {
		return nil, fmt.Errorf("read session parts: %w", err)
	}
	defer rows.Close()
	out := []model.AttemptPart{}
	for rows.Next() {
		var p model.AttemptPart
		if err := rows.Scan(&p.SessionID, &p.SubtestID, &p.Position, &p.Name, &p.DurationSeconds, &p.Status,
			&p.StartedAt, &p.DeadlineAt, &p.ExtraSeconds, &p.SubmittedAt, &p.SubmitReason, &p.QuestionCount,
			&p.EarnedPoints, &p.MaxPoints, &p.CorrectCount); err != nil {
			return nil, fmt.Errorf("scan session part: %w", err)
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

const questionSelect = `
		SELECT sq.session_id, sq.subtest_id, sq.position, sq.question_id, sq.question_version_id,
		       v.type, v.difficulty, v.content, v.option_weighted, v.case_sensitive, v.options, v.correct,
		       sq.option_order
		  FROM tryout_session_questions sq
		  JOIN question_versions v ON v.id = sq.question_version_id AND v.tenant_id = sq.tenant_id
		  JOIN tryout_session_parts p ON p.session_id = sq.session_id AND p.subtest_id = sq.subtest_id
		 WHERE sq.tenant_id = $1 AND sq.session_id = $2`

func scanQuestions(rows pgx.Rows) ([]model.AttemptQuestion, error) {
	defer rows.Close()
	out := []model.AttemptQuestion{}
	for rows.Next() {
		var aq model.AttemptQuestion
		var options, correct, order []byte
		if err := rows.Scan(&aq.SessionID, &aq.SubtestID, &aq.Position, &aq.QuestionID, &aq.VersionID,
			&aq.Type, &aq.Difficulty, &aq.Content, &aq.OptionWeighted, &aq.CaseSensitive, &options, &correct,
			&order); err != nil {
			return nil, fmt.Errorf("scan session question: %w", err)
		}
		var stored []model.QuestionOption
		var ids []string
		if err := json.Unmarshal(options, &stored); err != nil {
			return nil, fmt.Errorf("decode session options: %w", err)
		}
		if err := json.Unmarshal(correct, &aq.Correct); err != nil {
			return nil, fmt.Errorf("decode session correct: %w", err)
		}
		if err := json.Unmarshal(order, &ids); err != nil {
			return nil, fmt.Errorf("decode option order: %w", err)
		}
		aq.Options = inOrder(stored, ids)
		out = append(out, aq)
	}
	return out, rows.Err()
}

// Questions reads the snapshot of one subtest, or of the whole session when subtestID is nil, in display order.
func (r *RuntimeRepository) Questions(ctx context.Context, q dbtx, tenantID, sessionID uuid.UUID, subtestID *uuid.UUID) ([]model.AttemptQuestion, error) {
	if tenantID == uuid.Nil {
		return nil, ErrScopeRequired
	}
	rows, err := q.Query(ctx, questionSelect+` AND ($3::uuid IS NULL OR sq.subtest_id = $3)
		 ORDER BY p.position, sq.position`, tenantID, sessionID, subtestID)
	if err != nil {
		return nil, fmt.Errorf("read session questions: %w", err)
	}
	return scanQuestions(rows)
}

// inOrder arranges the version's options by the session's persisted order; an id the order lacks keeps its place last.
func inOrder(options []model.QuestionOption, ids []string) []model.QuestionOption {
	byID := make(map[string]model.QuestionOption, len(options))
	for _, o := range options {
		byID[o.ID] = o
	}
	out := make([]model.QuestionOption, 0, len(options))
	seen := make(map[string]bool, len(options))
	for _, id := range ids {
		if o, ok := byID[id]; ok && !seen[id] {
			out = append(out, o)
			seen[id] = true
		}
	}
	for _, o := range options {
		if !seen[o.ID] {
			out = append(out, o)
		}
	}
	return out
}

// Question reads one snapshotted question; it is how an answer finds the part it belongs to.
func (r *RuntimeRepository) Question(ctx context.Context, q dbtx, tenantID, sessionID, questionID uuid.UUID) (model.AttemptQuestion, error) {
	if tenantID == uuid.Nil {
		return model.AttemptQuestion{}, ErrScopeRequired
	}
	rows, err := q.Query(ctx, questionSelect+` AND sq.question_id = $3`, tenantID, sessionID, questionID)
	if err != nil {
		return model.AttemptQuestion{}, fmt.Errorf("read session question: %w", err)
	}
	found, err := scanQuestions(rows)
	if err != nil {
		return model.AttemptQuestion{}, err
	}
	if len(found) == 0 {
		return model.AttemptQuestion{}, ErrNotFound
	}
	return found[0], nil
}

// Answers reads the saved answers of one subtest, or of the whole session when subtestID is nil.
func (r *RuntimeRepository) Answers(ctx context.Context, q dbtx, tenantID, sessionID uuid.UUID, subtestID *uuid.UUID) ([]model.AttemptAnswer, error) {
	if tenantID == uuid.Nil {
		return nil, ErrScopeRequired
	}
	rows, err := q.Query(ctx, `
		SELECT a.question_id, a.answer, a.revision, a.client_seq, a.updated_at
		  FROM tryout_session_answers a
		  JOIN tryout_session_questions sq ON sq.session_id = a.session_id AND sq.question_id = a.question_id
		 WHERE a.tenant_id = $1 AND a.session_id = $2 AND ($3::uuid IS NULL OR sq.subtest_id = $3)`,
		tenantID, sessionID, subtestID)
	if err != nil {
		return nil, fmt.Errorf("read answers: %w", err)
	}
	defer rows.Close()
	out := []model.AttemptAnswer{}
	for rows.Next() {
		var a model.AttemptAnswer
		var raw []byte
		if err := rows.Scan(&a.QuestionID, &raw, &a.Revision, &a.Seq, &a.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan answer: %w", err)
		}
		if err := json.Unmarshal(raw, &a.Answer); err != nil {
			return nil, fmt.Errorf("decode answer: %w", err)
		}
		if a.Answer == nil {
			a.Answer = []string{}
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

func (r *RuntimeRepository) Answer(ctx context.Context, q dbtx, tenantID, sessionID, questionID uuid.UUID) (model.AttemptAnswer, error) {
	if tenantID == uuid.Nil {
		return model.AttemptAnswer{}, ErrScopeRequired
	}
	var a model.AttemptAnswer
	var raw []byte
	err := q.QueryRow(ctx, `
		SELECT question_id, answer, revision, client_seq, updated_at FROM tryout_session_answers
		 WHERE tenant_id = $1 AND session_id = $2 AND question_id = $3`, tenantID, sessionID, questionID).
		Scan(&a.QuestionID, &raw, &a.Revision, &a.Seq, &a.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return model.AttemptAnswer{}, ErrNotFound
	}
	if err != nil {
		return model.AttemptAnswer{}, fmt.Errorf("read answer: %w", err)
	}
	if err := json.Unmarshal(raw, &a.Answer); err != nil {
		return model.AttemptAnswer{}, fmt.Errorf("decode answer: %w", err)
	}
	return a, nil
}

// ErrStaleWrite reports that a stored answer already carries an equal or newer client sequence.
var ErrStaleWrite = errors.New("stale answer write")

// PutAnswer inserts or replaces one answer. The service has already judged it fresh, and the WHERE repeats that
// as a DB guard, so a reordered write can never lower the stored sequence.
func (r *RuntimeRepository) PutAnswer(ctx context.Context, q dbtx, tenantID, sessionID uuid.UUID, a model.AttemptAnswer) error {
	if tenantID == uuid.Nil {
		return ErrScopeRequired
	}
	raw, err := json.Marshal(a.Answer)
	if err != nil {
		return fmt.Errorf("encode answer: %w", err)
	}
	tag, err := q.Exec(ctx, `
		INSERT INTO tryout_session_answers (session_id, tenant_id, question_id, answer, revision, client_seq, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (session_id, question_id)
		DO UPDATE SET answer = EXCLUDED.answer, revision = EXCLUDED.revision,
		              client_seq = EXCLUDED.client_seq, updated_at = EXCLUDED.updated_at
		   WHERE tryout_session_answers.client_seq < EXCLUDED.client_seq`,
		sessionID, tenantID, a.QuestionID, raw, a.Revision, a.Seq, a.UpdatedAt)
	if err != nil {
		return fmt.Errorf("put answer: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrStaleWrite
	}
	return nil
}

func (r *RuntimeRepository) StartPart(ctx context.Context, q dbtx, tenantID, sessionID, subtestID uuid.UUID, startedAt, deadlineAt time.Time) error {
	if tenantID == uuid.Nil {
		return ErrScopeRequired
	}
	tag, err := q.Exec(ctx, `
		UPDATE tryout_session_parts SET status = 'running', started_at = $4, deadline_at = $5
		 WHERE tenant_id = $1 AND session_id = $2 AND subtest_id = $3 AND status = 'pending'`,
		tenantID, sessionID, subtestID, startedAt, deadlineAt)
	if err != nil {
		return fmt.Errorf("start part: %w", err)
	}
	if tag.RowsAffected() != 1 {
		return ErrNotFound
	}
	return nil
}

// SettlePart makes a part immutable with its raw score. It only moves a pending or running part, so a second call
// changes nothing and reports false.
func (r *RuntimeRepository) SettlePart(ctx context.Context, q dbtx, tenantID, sessionID, subtestID uuid.UUID, reason string, at time.Time, earned, maximum float64, correct int) (bool, error) {
	if tenantID == uuid.Nil {
		return false, ErrScopeRequired
	}
	tag, err := q.Exec(ctx, `
		UPDATE tryout_session_parts
		   SET status = 'submitted', submitted_at = $4, submit_reason = $5,
		       earned_points = $6, max_points = $7, correct_count = $8
		 WHERE tenant_id = $1 AND session_id = $2 AND subtest_id = $3 AND status <> 'submitted'`,
		tenantID, sessionID, subtestID, at, reason, earned, maximum, correct)
	if err != nil {
		return false, fmt.Errorf("settle part: %w", err)
	}
	return tag.RowsAffected() == 1, nil
}

func (r *RuntimeRepository) CompleteAttempt(ctx context.Context, q dbtx, tenantID, id uuid.UUID, at time.Time) (bool, error) {
	if tenantID == uuid.Nil {
		return false, ErrScopeRequired
	}
	tag, err := q.Exec(ctx, `
		UPDATE tryout_sessions SET status = 'completed', completed_at = $3
		 WHERE tenant_id = $1 AND id = $2 AND status = 'in_progress'`, tenantID, id, at)
	if err != nil {
		return false, fmt.Errorf("complete session: %w", err)
	}
	return tag.RowsAffected() == 1, nil
}

// InsertResult stores the session's one result; false means one already existed and nothing changed.
func (r *RuntimeRepository) InsertResult(ctx context.Context, q dbtx, res model.Result) (bool, error) {
	if res.TenantID == uuid.Nil {
		return false, ErrScopeRequired
	}
	tag, err := q.Exec(ctx, `
		INSERT INTO tryout_results (session_id, tenant_id, user_id, tryout_id, attempt_number, earned_points,
		        max_points, score, scoring_mode, scoring_version, completed_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		ON CONFLICT (session_id) DO NOTHING`,
		res.SessionID, res.TenantID, res.UserID, res.TryoutID, res.AttemptNumber, res.EarnedPoints,
		res.MaxPoints, res.Score, res.ScoringMode, res.ScoringVersion, res.CompletedAt)
	if err != nil {
		return false, fmt.Errorf("insert result: %w", err)
	}
	return tag.RowsAffected() == 1, nil
}

// AddScoreTotal keeps the per-student, per-tryout sum and count P17 averages over.
func (r *RuntimeRepository) AddScoreTotal(ctx context.Context, q dbtx, tenantID, userID, tryoutID uuid.UUID, score float64) error {
	if tenantID == uuid.Nil {
		return ErrScopeRequired
	}
	_, err := q.Exec(ctx, `
		INSERT INTO tryout_score_totals (tenant_id, user_id, tryout_id, score_sum, result_count)
		VALUES ($1, $2, $3, $4, 1)
		ON CONFLICT (tenant_id, user_id, tryout_id)
		DO UPDATE SET score_sum = tryout_score_totals.score_sum + EXCLUDED.score_sum,
		              result_count = tryout_score_totals.result_count + 1, updated_at = now()`,
		tenantID, userID, tryoutID, score)
	if err != nil {
		return fmt.Errorf("add score total: %w", err)
	}
	return nil
}

// ScoreTotal is what P17 reads; it is exposed here so the integration tests can prove it moves once.
func (r *RuntimeRepository) ScoreTotal(ctx context.Context, q dbtx, tenantID, userID, tryoutID uuid.UUID) (float64, int, error) {
	if tenantID == uuid.Nil {
		return 0, 0, ErrScopeRequired
	}
	var sum float64
	var count int
	err := q.QueryRow(ctx, `
		SELECT score_sum::float8, result_count FROM tryout_score_totals
		 WHERE tenant_id = $1 AND user_id = $2 AND tryout_id = $3`, tenantID, userID, tryoutID).Scan(&sum, &count)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, 0, nil
	}
	if err != nil {
		return 0, 0, fmt.Errorf("read score total: %w", err)
	}
	return sum, count, nil
}

func (r *RuntimeRepository) Result(ctx context.Context, q dbtx, tenantID, sessionID uuid.UUID) (model.Result, error) {
	if tenantID == uuid.Nil {
		return model.Result{}, ErrScopeRequired
	}
	var res model.Result
	err := q.QueryRow(ctx, `
		SELECT session_id, tenant_id, user_id, tryout_id, attempt_number, earned_points::float8, max_points::float8,
		       score::float8, scoring_mode, scoring_version, completed_at
		  FROM tryout_results WHERE tenant_id = $1 AND session_id = $2`, tenantID, sessionID).
		Scan(&res.SessionID, &res.TenantID, &res.UserID, &res.TryoutID, &res.AttemptNumber, &res.EarnedPoints,
			&res.MaxPoints, &res.Score, &res.ScoringMode, &res.ScoringVersion, &res.CompletedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return model.Result{}, ErrNotFound
	}
	if err != nil {
		return model.Result{}, fmt.Errorf("read result: %w", err)
	}
	return res, nil
}

// Attempts lists one student's sessions, newest first, optionally for a single tryout.
func (r *RuntimeRepository) Attempts(ctx context.Context, q dbtx, tenantID, userID uuid.UUID, tryoutID *uuid.UUID) ([]model.AttemptSummary, error) {
	if tenantID == uuid.Nil {
		return nil, ErrScopeRequired
	}
	rows, err := q.Query(ctx, `
		SELECT s.id, s.tryout_id, s.attempt_number, s.status, s.started_at, s.completed_at, r.score::float8
		  FROM tryout_sessions s
		  LEFT JOIN tryout_results r ON r.session_id = s.id AND r.tenant_id = s.tenant_id
		 WHERE s.tenant_id = $1 AND s.user_id = $2 AND ($3::uuid IS NULL OR s.tryout_id = $3)
		 ORDER BY s.started_at DESC, s.id DESC`, tenantID, userID, tryoutID)
	if err != nil {
		return nil, fmt.Errorf("list attempts: %w", err)
	}
	defer rows.Close()
	out := []model.AttemptSummary{}
	for rows.Next() {
		var a model.AttemptSummary
		if err := rows.Scan(&a.SessionID, &a.TryoutID, &a.AttemptNumber, &a.Status, &a.StartedAt, &a.CompletedAt, &a.Score); err != nil {
			return nil, fmt.Errorf("scan attempt: %w", err)
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// AddExtra records one audited grant and adds it to the subtest that is running. It reports ErrNotFound when
// nothing is running, so a grant never lands on a part that cannot use it.
func (r *RuntimeRepository) AddExtra(ctx context.Context, q dbtx, tenantID, sessionID, grantedBy uuid.UUID, seconds int, reason string) (uuid.UUID, int, error) {
	if tenantID == uuid.Nil {
		return uuid.Nil, 0, ErrScopeRequired
	}
	var subtestID uuid.UUID
	var total int
	err := q.QueryRow(ctx, `
		UPDATE tryout_session_parts SET extra_seconds = extra_seconds + $3
		 WHERE tenant_id = $1 AND session_id = $2 AND status = 'running'
		RETURNING subtest_id, extra_seconds`, tenantID, sessionID, seconds).Scan(&subtestID, &total)
	if errors.Is(err, pgx.ErrNoRows) {
		return uuid.Nil, 0, ErrNotFound
	}
	if err != nil {
		return uuid.Nil, 0, fmt.Errorf("add extra seconds: %w", err)
	}
	if _, err := q.Exec(ctx, `
		INSERT INTO tryout_session_extensions (tenant_id, session_id, subtest_id, seconds, reason, granted_by)
		VALUES ($1, $2, $3, $4, $5, $6)`, tenantID, sessionID, subtestID, seconds, reason, grantedBy); err != nil {
		return uuid.Nil, 0, fmt.Errorf("record extension: %w", err)
	}
	return subtestID, total, nil
}

// DueSession names a session the worker should settle.
type DueSession struct {
	ID       uuid.UUID
	TenantID uuid.UUID
}

// DueSessions is the deadline worker's scan, most overdue first so a session that keeps failing cannot starve the
// rest. A running part is due once its deadline plus its own extension and the tryout's live grace has passed; a
// session with no running part is due at the tryout's active_until. The plain deadline_at bound is implied by the
// sum but lets the first branch use the partial index on running deadlines.
func (r *RuntimeRepository) DueSessions(ctx context.Context, q dbtx, now time.Time, limit int) ([]DueSession, error) {
	rows, err := q.Query(ctx, `
		SELECT id, tenant_id FROM (
		    SELECT s.id, s.tenant_id, p.deadline_at + make_interval(secs => p.extra_seconds + t.grace_seconds) AS due_at
		      FROM tryout_session_parts p
		      JOIN tryout_sessions s ON s.id = p.session_id AND s.tenant_id = p.tenant_id
		      JOIN tryouts t ON t.id = s.tryout_id AND t.tenant_id = s.tenant_id
		     WHERE p.status = 'running' AND p.deadline_at <= $1
		       AND p.deadline_at + make_interval(secs => p.extra_seconds + t.grace_seconds) <= $1
		    UNION
		    SELECT s.id, s.tenant_id, t.active_until AS due_at
		      FROM tryout_sessions s
		      JOIN tryouts t ON t.id = s.tryout_id AND t.tenant_id = s.tenant_id
		     WHERE s.status = 'in_progress' AND t.active_until <= $1
		       AND NOT EXISTS (SELECT 1 FROM tryout_session_parts p
		                        WHERE p.session_id = s.id AND p.tenant_id = s.tenant_id AND p.status = 'running')
		) due ORDER BY due_at, id LIMIT $2`, now, limit)
	if err != nil {
		return nil, fmt.Errorf("scan due sessions: %w", err)
	}
	defer rows.Close()
	var out []DueSession
	for rows.Next() {
		var d DueSession
		if err := rows.Scan(&d.ID, &d.TenantID); err != nil {
			return nil, fmt.Errorf("scan due session: %w", err)
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

// Overview is a published tryout's head, its program and its subtests with question counts: everything the
// student's overview needs and nothing of the questions themselves.
type Overview struct {
	Tryout      model.Tryout
	ProgramCode string
	ProgramName string
	Subtests    []model.OverviewSubtest
}

func (r *RuntimeRepository) TryoutOverview(ctx context.Context, q dbtx, tenantID, id uuid.UUID) (Overview, error) {
	if tenantID == uuid.Nil {
		return Overview{}, ErrScopeRequired
	}
	var out Overview
	var activeFrom, releaseAt *time.Time
	err := q.QueryRow(ctx, `
		SELECT t.id, t.tenant_id, t.program_id, t.title, t.description, t.scoring_mode, t.status, t.active_from,
		       t.active_until, t.explanation_release_at, t.grace_seconds, p.code, p.name
		  FROM tryouts t
		  JOIN programs p ON p.id = t.program_id AND p.tenant_id = t.tenant_id
		 WHERE t.tenant_id = $1 AND t.id = $2 AND t.status = 'published'`, tenantID, id).
		Scan(&out.Tryout.ID, &out.Tryout.TenantID, &out.Tryout.ProgramID, &out.Tryout.Title,
			&out.Tryout.Description, &out.Tryout.ScoringMode, &out.Tryout.Status, &activeFrom,
			&out.Tryout.ActiveUntil, &releaseAt, &out.Tryout.GraceSeconds, &out.ProgramCode, &out.ProgramName)
	if errors.Is(err, pgx.ErrNoRows) {
		return Overview{}, ErrNotFound
	}
	if err != nil {
		return Overview{}, fmt.Errorf("read tryout overview: %w", err)
	}
	out.Tryout.ActiveFrom, out.Tryout.ExplanationReleaseAt = activeFrom, releaseAt

	rows, err := q.Query(ctx, `
		SELECT s.id, s.name, s.duration_seconds, s.position,
		       (SELECT count(*) FROM tryout_questions tq WHERE tq.subtest_id = s.id AND tq.tenant_id = s.tenant_id)
		  FROM tryout_subtests s
		 WHERE s.tenant_id = $1 AND s.tryout_id = $2 ORDER BY s.position`, tenantID, id)
	if err != nil {
		return Overview{}, fmt.Errorf("read overview subtests: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var st model.OverviewSubtest
		if err := rows.Scan(&st.ID, &st.Name, &st.DurationSeconds, &st.Position, &st.QuestionCount); err != nil {
			return Overview{}, fmt.Errorf("scan overview subtest: %w", err)
		}
		out.Subtests = append(out.Subtests, st)
	}
	return out, rows.Err()
}
