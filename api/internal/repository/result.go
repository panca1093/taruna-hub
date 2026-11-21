package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/tarunaeduka/bimbel-tryout/api/internal/model"
)

type ResultRepository struct {
	db *pgxpool.Pool
}

func NewResultRepository(db *pgxpool.Pool) *ResultRepository {
	return &ResultRepository{db: db}
}

// Average is one student's mean on a tryout, rounded to 2 decimals. Attempts is how many results it averages, so
// the sum of Attempts over a tryout moves by exactly one with every completed attempt and versions a cached copy.
type Average struct {
	UserID   uuid.UUID
	Average  float64
	Attempts int
}

// averageSQL is the one definition of the ranking: tied averages share a rank and the next rank skips (1, 2, 2, 4).
const averageSQL = `
	SELECT user_id, tryout_id, round(score_sum / result_count, 2) AS average, result_count,
	       RANK() OVER (PARTITION BY tryout_id ORDER BY round(score_sum / result_count, 2) DESC) AS rnk,
	       COUNT(*) OVER (PARTITION BY tryout_id) AS participants
	  FROM tryout_score_totals
	 WHERE tenant_id = $1`

// Averages is every participant of a tryout with the average the ranking uses. It is the rebuild source of the cache.
func (r *ResultRepository) Averages(ctx context.Context, q dbtx, tenantID, tryoutID uuid.UUID) ([]Average, error) {
	if tenantID == uuid.Nil {
		return nil, ErrScopeRequired
	}
	rows, err := q.Query(ctx, `
		SELECT user_id, round(score_sum / result_count, 2)::float8, result_count
		  FROM tryout_score_totals WHERE tenant_id = $1 AND tryout_id = $2`, tenantID, tryoutID)
	if err != nil {
		return nil, fmt.Errorf("read averages: %w", err)
	}
	defer rows.Close()
	out := []Average{}
	for rows.Next() {
		var a Average
		if err := rows.Scan(&a.UserID, &a.Average, &a.Attempts); err != nil {
			return nil, fmt.Errorf("scan average: %w", err)
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// Ranking reads one student's standing straight from PostgreSQL. ok is false when the student has no result yet.
func (r *ResultRepository) Ranking(ctx context.Context, q dbtx, tenantID, tryoutID, userID uuid.UUID) (model.Ranking, bool, error) {
	if tenantID == uuid.Nil {
		return model.Ranking{}, false, ErrScopeRequired
	}
	var out model.Ranking
	err := q.QueryRow(ctx, `
		WITH st AS (`+averageSQL+` AND tryout_id = $2)
		SELECT rnk, participants, average::float8, result_count FROM st WHERE user_id = $3`,
		tenantID, tryoutID, userID).Scan(&out.Rank, &out.Participants, &out.Average, &out.Attempts)
	if errors.Is(err, pgx.ErrNoRows) {
		return model.Ranking{}, false, nil
	}
	if err != nil {
		return model.Ranking{}, false, fmt.Errorf("read ranking: %w", err)
	}
	return out, true, nil
}

// Leaderboard is one page of a tryout's standings, best first, and the participant count.
func (r *ResultRepository) Leaderboard(ctx context.Context, q dbtx, tenantID, tryoutID uuid.UUID, limit, offset int) ([]model.Standing, int, error) {
	if tenantID == uuid.Nil {
		return nil, 0, ErrScopeRequired
	}
	rows, err := q.Query(ctx, `
		WITH st AS (`+averageSQL+` AND tryout_id = $2)
		SELECT st.user_id, u.name, u.avatar_key, u.gender, u.birth_date, st.average::float8, st.result_count, st.rnk, st.participants
		  FROM st JOIN users u ON u.id = st.user_id AND u.tenant_id = $1
		 ORDER BY st.rnk, u.name, st.user_id
		 LIMIT $3 OFFSET $4`, tenantID, tryoutID, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("read leaderboard: %w", err)
	}
	defer rows.Close()
	out := []model.Standing{}
	total := 0
	for rows.Next() {
		var s model.Standing
		if err := rows.Scan(&s.UserID, &s.Name, &s.AvatarKey, &s.Gender, &s.BirthDate, &s.Average, &s.Attempts, &s.Rank, &total); err != nil {
			return nil, 0, fmt.Errorf("scan standing: %w", err)
		}
		out = append(out, s)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	if len(out) == 0 {
		if err := q.QueryRow(ctx, `SELECT count(*) FROM tryout_score_totals WHERE tenant_id = $1 AND tryout_id = $2`,
			tenantID, tryoutID).Scan(&total); err != nil {
			return nil, 0, fmt.Errorf("count participants: %w", err)
		}
	}
	return out, total, nil
}

// TryoutRelease reads when explanations open: nil means immediately. ErrNotFound when the tryout is not the tenant's.
func (r *ResultRepository) TryoutRelease(ctx context.Context, q dbtx, tenantID, tryoutID uuid.UUID) (*time.Time, error) {
	if tenantID == uuid.Nil {
		return nil, ErrScopeRequired
	}
	var at *time.Time
	err := q.QueryRow(ctx, `SELECT explanation_release_at FROM tryouts WHERE tenant_id = $1 AND id = $2`, tenantID, tryoutID).Scan(&at)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("read explanation release: %w", err)
	}
	return at, nil
}

// ReviewQuestions reads the session's snapshot with each version's explanation, in display order. Only the service
// that has checked the release time may call it.
func (r *ResultRepository) ReviewQuestions(ctx context.Context, q dbtx, tenantID, sessionID uuid.UUID) ([]model.ReviewQuestion, error) {
	if tenantID == uuid.Nil {
		return nil, ErrScopeRequired
	}
	rows, err := q.Query(ctx, `
		SELECT sq.subtest_id, sq.position, sq.question_id, sq.question_version_id, v.type, v.difficulty, v.content,
		       v.option_weighted, v.case_sensitive, v.options, v.correct, sq.option_order, v.explanation,
		       v.content_doc, v.explanation_doc
		  FROM tryout_session_questions sq
		  JOIN question_versions v ON v.id = sq.question_version_id AND v.tenant_id = sq.tenant_id
		  JOIN tryout_session_parts p ON p.session_id = sq.session_id AND p.subtest_id = sq.subtest_id
		 WHERE sq.tenant_id = $1 AND sq.session_id = $2
		 ORDER BY p.position, sq.position`, tenantID, sessionID)
	if err != nil {
		return nil, fmt.Errorf("read review questions: %w", err)
	}
	defer rows.Close()
	out := []model.ReviewQuestion{}
	for rows.Next() {
		var rq model.ReviewQuestion
		rq.SessionID = sessionID
		var options, correct, order, contentDoc, explanationDoc []byte
		var content, explanation string
		if err := rows.Scan(&rq.SubtestID, &rq.Position, &rq.QuestionID, &rq.VersionID, &rq.Type, &rq.Difficulty,
			&content, &rq.OptionWeighted, &rq.CaseSensitive, &options, &correct, &order, &explanation,
			&contentDoc, &explanationDoc); err != nil {
			return nil, fmt.Errorf("scan review question: %w", err)
		}
		var ids []string
		stored, err := decodeOptions(options)
		if err != nil {
			return nil, fmt.Errorf("decode review options: %w", err)
		}
		if rq.Content, err = model.DocFromColumn(contentDoc, content); err != nil {
			return nil, fmt.Errorf("decode review content: %w", err)
		}
		if rq.Explanation, err = model.DocFromColumn(explanationDoc, explanation); err != nil {
			return nil, fmt.Errorf("decode review explanation: %w", err)
		}
		if err := json.Unmarshal(correct, &rq.Correct); err != nil {
			return nil, fmt.Errorf("decode review correct: %w", err)
		}
		if err := json.Unmarshal(order, &ids); err != nil {
			return nil, fmt.Errorf("decode review option order: %w", err)
		}
		rq.Options = inOrder(stored, ids)
		out = append(out, rq)
	}
	return out, rows.Err()
}

// StaffResultFilter is already validated by the service. StudentID narrows to one student for the detail.
type StaffResultFilter struct {
	TryoutID  *uuid.UUID
	ProgramID *uuid.UUID
	StudentID *uuid.UUID
	Search    string
	Limit     int
	Offset    int
}

// StaffStudentResults lists one row per student and tryout with at least one completed attempt. With a tryout filter
// the best current rank comes first; without one ranks of different tryouts are not comparable, so the most recently
// active comes first. Rank and average come from the same averageSQL as the leaderboard.
func (r *ResultRepository) StaffStudentResults(ctx context.Context, q dbtx, tenantID uuid.UUID, f StaffResultFilter) ([]model.StaffStudentResult, int, error) {
	if tenantID == uuid.Nil {
		return nil, 0, ErrScopeRequired
	}
	search := ""
	if f.Search != "" {
		search = likePattern(f.Search)
	}
	const from = `
		  FROM g
		  JOIN st ON st.user_id = g.user_id AND st.tryout_id = g.tryout_id
		  JOIN users u ON u.id = g.user_id AND u.tenant_id = $1
		  JOIN tryouts t ON t.id = g.tryout_id AND t.tenant_id = $1
		  JOIN programs p ON p.id = t.program_id AND p.tenant_id = $1
		 WHERE ($3 = '' OR u.name ILIKE $3 ESCAPE '\') AND ($4::uuid IS NULL OR t.program_id = $4)`
	const cte = `WITH st AS (` + averageSQL + ` AND ($2::uuid IS NULL OR tryout_id = $2)),
		g AS (
		    SELECT user_id, tryout_id, count(*) AS attempts, max(completed_at) AS last_at,
		           (array_agg(score ORDER BY completed_at DESC, attempt_number DESC))[1] AS latest
		      FROM tryout_results
		     WHERE tenant_id = $1 AND ($2::uuid IS NULL OR tryout_id = $2) AND ($5::uuid IS NULL OR user_id = $5)
		     GROUP BY user_id, tryout_id)`
	var total int
	if err := q.QueryRow(ctx, cte+` SELECT count(*)`+from, tenantID, f.TryoutID, search, f.ProgramID, f.StudentID).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count staff student results: %w", err)
	}
	rows, err := q.Query(ctx, cte+`
		SELECT g.user_id, u.name, u.avatar_key, u.gender, u.birth_date, g.tryout_id, t.title, t.program_id, p.code, p.name,
		       t.scoring_mode, g.attempts, st.result_count, st.average::float8, g.latest::float8, st.rnk, st.participants, g.last_at`+from+`
		 ORDER BY CASE WHEN $2::uuid IS NULL THEN 0 ELSE st.rnk END, g.last_at DESC, u.name, g.user_id, g.tryout_id
		 LIMIT $6 OFFSET $7`, tenantID, f.TryoutID, search, f.ProgramID, f.StudentID, f.Limit, f.Offset)
	if err != nil {
		return nil, 0, fmt.Errorf("list staff student results: %w", err)
	}
	defer rows.Close()
	out := []model.StaffStudentResult{}
	for rows.Next() {
		var s model.StaffStudentResult
		if err := rows.Scan(&s.StudentID, &s.StudentName, &s.AvatarKey, &s.Gender, &s.BirthDate, &s.TryoutID, &s.TryoutTitle,
			&s.ProgramID, &s.ProgramCode, &s.ProgramName, &s.ScoringMode, &s.AttemptsCompleted, &s.AttemptsCounted, &s.Average, &s.LatestScore,
			&s.Rank, &s.Participants, &s.LastCompletedAt); err != nil {
			return nil, 0, fmt.Errorf("scan staff student result: %w", err)
		}
		out = append(out, s)
	}
	return out, total, rows.Err()
}

// StudentTryout names one student on one tryout.
type StudentTryout struct{ StudentID, TryoutID uuid.UUID }

// StaffAttempts reads the completed attempts of the given students on the given tryouts, newest first.
func (r *ResultRepository) StaffAttempts(ctx context.Context, q dbtx, tenantID uuid.UUID, pairs []StudentTryout) (map[StudentTryout][]model.StaffAttempt, error) {
	if tenantID == uuid.Nil {
		return nil, ErrScopeRequired
	}
	students := make([]uuid.UUID, len(pairs))
	tryouts := make([]uuid.UUID, len(pairs))
	for i, p := range pairs {
		students[i], tryouts[i] = p.StudentID, p.TryoutID
	}
	rows, err := q.Query(ctx, `
		SELECT r.user_id, r.tryout_id, r.session_id, r.attempt_number, r.score::float8, r.scoring_mode, r.completed_at
		  FROM tryout_results r
		  JOIN unnest($2::uuid[], $3::uuid[]) AS want(user_id, tryout_id) ON want.user_id = r.user_id AND want.tryout_id = r.tryout_id
		 WHERE r.tenant_id = $1
		 ORDER BY r.completed_at DESC, r.attempt_number DESC`, tenantID, students, tryouts)
	if err != nil {
		return nil, fmt.Errorf("list staff attempts: %w", err)
	}
	defer rows.Close()
	out := map[StudentTryout][]model.StaffAttempt{}
	for rows.Next() {
		var key StudentTryout
		var a model.StaffAttempt
		if err := rows.Scan(&key.StudentID, &key.TryoutID, &a.SessionID, &a.AttemptNumber, &a.Score, &a.ScoringMode, &a.CompletedAt); err != nil {
			return nil, fmt.Errorf("scan staff attempt: %w", err)
		}
		out[key] = append(out[key], a)
	}
	return out, rows.Err()
}

// SessionParts reads the settled subtest scores of the given sessions, in subtest order.
func (r *ResultRepository) SessionParts(ctx context.Context, q dbtx, tenantID uuid.UUID, sessionIDs []uuid.UUID) (map[uuid.UUID][]model.PartResult, error) {
	if tenantID == uuid.Nil {
		return nil, ErrScopeRequired
	}
	rows, err := q.Query(ctx, `
		SELECT session_id, subtest_id, name, position, earned_points::float8, max_points::float8, correct_count,
		       question_count, submit_reason
		  FROM tryout_session_parts
		 WHERE tenant_id = $1 AND session_id = ANY($2) AND status = 'submitted'
		 ORDER BY session_id, position`, tenantID, sessionIDs)
	if err != nil {
		return nil, fmt.Errorf("read session parts: %w", err)
	}
	defer rows.Close()
	out := map[uuid.UUID][]model.PartResult{}
	for rows.Next() {
		var sessionID uuid.UUID
		var p model.PartResult
		if err := rows.Scan(&sessionID, &p.SubtestID, &p.Name, &p.Position, &p.EarnedPoints, &p.MaxPoints,
			&p.CorrectCount, &p.QuestionCount, &p.SubmitReason); err != nil {
			return nil, fmt.Errorf("scan session part: %w", err)
		}
		out[sessionID] = append(out[sessionID], p)
	}
	return out, rows.Err()
}

// IRTUnderThreshold maps each session to whether any of its questions was answered by fewer than threshold distinct
// students. A student counts once per question version however many completed attempts answered it; a blank answer,
// an attempt still in progress and another version of the same question do not count.
func (r *ResultRepository) IRTUnderThreshold(ctx context.Context, q dbtx, tenantID uuid.UUID, sessionIDs []uuid.UUID, threshold int) (map[uuid.UUID]bool, error) {
	if tenantID == uuid.Nil {
		return nil, ErrScopeRequired
	}
	rows, err := q.Query(ctx, `
		WITH page AS (
		    SELECT session_id, question_version_id
		      FROM tryout_session_questions
		     WHERE tenant_id = $1 AND session_id = ANY($2)),
		counts AS (
		    SELECT q2.question_version_id, count(DISTINCT s.user_id) AS n
		      FROM tryout_session_questions q2
		      JOIN tryout_sessions s ON s.id = q2.session_id AND s.tenant_id = q2.tenant_id AND s.status = 'completed'
		      JOIN tryout_session_answers a ON a.session_id = q2.session_id AND a.question_id = q2.question_id
		     WHERE q2.tenant_id = $1 AND q2.question_version_id IN (SELECT question_version_id FROM page)
		       AND EXISTS (SELECT 1 FROM jsonb_array_elements_text(a.answer) e WHERE btrim(e) <> '')
		     GROUP BY q2.question_version_id)
		SELECT page.session_id, bool_or(coalesce(counts.n, 0) < $3)
		  FROM page LEFT JOIN counts ON counts.question_version_id = page.question_version_id
		 GROUP BY page.session_id`, tenantID, sessionIDs, threshold)
	if err != nil {
		return nil, fmt.Errorf("count irt respondents: %w", err)
	}
	defer rows.Close()
	out := make(map[uuid.UUID]bool, len(sessionIDs))
	for rows.Next() {
		var id uuid.UUID
		var under bool
		if err := rows.Scan(&id, &under); err != nil {
			return nil, fmt.Errorf("scan irt count: %w", err)
		}
		out[id] = under
	}
	return out, rows.Err()
}
