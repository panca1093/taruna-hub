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

// TryoutFilter is already validated by the service; the repository only turns it into SQL.
type TryoutFilter struct {
	ProgramID *uuid.UUID
	Status    string
	Search    string
	Limit     int
	Offset    int
}

type TryoutRepository struct {
	db *pgxpool.Pool
}

func NewTryoutRepository(db *pgxpool.Pool) *TryoutRepository {
	return &TryoutRepository{db: db}
}

const tryoutHeadColumns = `id, tenant_id, program_id, title, description, scoring_mode, status,
	active_from, active_until, explanation_release_at, randomize_questions, randomize_options,
	grace_seconds, revision, (first_attempt_at IS NOT NULL), created_at, updated_at`

const tryoutListColumns = `t.id, t.tenant_id, t.program_id, t.title, t.description, t.scoring_mode, t.status,
	t.active_from, t.active_until, t.explanation_release_at, t.randomize_questions, t.randomize_options,
	t.grace_seconds, t.revision, (t.first_attempt_at IS NOT NULL), t.created_at, t.updated_at`

func (r *TryoutRepository) List(ctx context.Context, tenantID uuid.UUID, filter TryoutFilter) ([]model.Tryout, int, error) {
	if tenantID == uuid.Nil {
		return nil, 0, ErrScopeRequired
	}
	where := []string{"t.tenant_id = $1"}
	args := []any{tenantID}
	add := func(condition string, value any) {
		args = append(args, value)
		where = append(where, fmt.Sprintf(condition, len(args)))
	}
	if filter.ProgramID != nil {
		add("t.program_id = $%d", *filter.ProgramID)
	}
	if filter.Status != "" {
		add("t.status = $%d", filter.Status)
	}
	if filter.Search != "" {
		add(`t.title ILIKE $%d ESCAPE '\'`, likePattern(filter.Search))
	}
	from := " FROM tryouts t WHERE " + strings.Join(where, " AND ")

	var total int
	if err := r.db.QueryRow(ctx, `SELECT count(*)`+from, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count tryouts: %w", err)
	}

	args = append(args, filter.Limit, filter.Offset)
	rows, err := r.db.Query(ctx, `SELECT `+tryoutListColumns+`,
		(SELECT count(*) FROM tryout_subtests s WHERE s.tenant_id = t.tenant_id AND s.tryout_id = t.id),
		(SELECT count(*) FROM tryout_questions q WHERE q.tenant_id = t.tenant_id AND q.tryout_id = t.id)`+
		from+fmt.Sprintf(` ORDER BY t.created_at DESC, t.id DESC LIMIT $%d OFFSET $%d`, len(args)-1, len(args)), args...)
	if err != nil {
		return nil, 0, fmt.Errorf("list tryouts: %w", err)
	}
	defer rows.Close()

	out := make([]model.Tryout, 0, filter.Limit)
	for rows.Next() {
		tryout, err := scanTryoutHead(rows, true)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, tryout)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("list tryouts: %w", err)
	}
	return out, total, nil
}

func (r *TryoutRepository) Find(ctx context.Context, tenantID, id uuid.UUID) (model.Tryout, error) {
	if tenantID == uuid.Nil {
		return model.Tryout{}, ErrScopeRequired
	}
	return r.find(ctx, r.db, tenantID, id, false)
}

func (r *TryoutRepository) FindTx(ctx context.Context, tx pgx.Tx, tenantID, id uuid.UUID) (model.Tryout, error) {
	if tenantID == uuid.Nil {
		return model.Tryout{}, ErrScopeRequired
	}
	return r.find(ctx, tx, tenantID, id, false)
}

// The row lock is the one P16's admission takes, so an edit and a first attempt cannot both win.
func (r *TryoutRepository) LockTx(ctx context.Context, tx pgx.Tx, tenantID, id uuid.UUID) (model.Tryout, error) {
	if tenantID == uuid.Nil {
		return model.Tryout{}, ErrScopeRequired
	}
	return r.find(ctx, tx, tenantID, id, true)
}

func (r *TryoutRepository) Create(ctx context.Context, tenantID, createdBy uuid.UUID, tryout model.Tryout) (model.Tryout, error) {
	if tenantID == uuid.Nil {
		return model.Tryout{}, ErrScopeRequired
	}
	var out model.Tryout
	err := r.inTx(ctx, func(tx pgx.Tx) error {
		id, err := r.insertHeadTx(ctx, tx, tenantID, createdBy, tryout)
		if err != nil {
			return err
		}
		if err := r.replaceChildrenTx(ctx, tx, tenantID, id, tryout.Subtests); err != nil {
			return err
		}
		created, err := r.find(ctx, tx, tenantID, id, false)
		out = created
		return err
	})
	return out, err
}

// UpdateHeadTx writes the editable fields and bumps the revision; the children are replaced beside it.
func (r *TryoutRepository) UpdateHeadTx(ctx context.Context, tx pgx.Tx, tenantID, id uuid.UUID, tryout model.Tryout) error {
	if tenantID == uuid.Nil {
		return ErrScopeRequired
	}
	return r.updateHeadTx(ctx, tx, tenantID, id, tryout)
}

func (r *TryoutRepository) ReplaceChildrenTx(ctx context.Context, tx pgx.Tx, tenantID, id uuid.UUID, subtests []model.Subtest) error {
	if tenantID == uuid.Nil {
		return ErrScopeRequired
	}
	return r.replaceChildrenTx(ctx, tx, tenantID, id, subtests)
}

func (r *TryoutRepository) DeleteTx(ctx context.Context, tx pgx.Tx, tenantID, id uuid.UUID) error {
	if tenantID == uuid.Nil {
		return ErrScopeRequired
	}
	if err := r.deleteChildrenTx(ctx, tx, tenantID, id); err != nil {
		return err
	}
	tag, err := tx.Exec(ctx, `DELETE FROM tryouts WHERE tenant_id = $1 AND id = $2`, tenantID, id)
	if err != nil {
		return mapTryoutError("delete tryout", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *TryoutRepository) SetStatusTx(ctx context.Context, tx pgx.Tx, tenantID, id uuid.UUID, status string) error {
	if tenantID == uuid.Nil {
		return ErrScopeRequired
	}
	tag, err := tx.Exec(ctx,
		`UPDATE tryouts SET status = $3, updated_at = now() WHERE tenant_id = $1 AND id = $2`,
		tenantID, id, status)
	if err != nil {
		return mapTryoutError("update tryout status", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// SetGraceSeconds is the one write a locked tryout still accepts: D10 lets Admin change it mid-exam.
func (r *TryoutRepository) SetGraceSeconds(ctx context.Context, tenantID, id uuid.UUID, seconds int) error {
	if tenantID == uuid.Nil {
		return ErrScopeRequired
	}
	tag, err := r.db.Exec(ctx,
		`UPDATE tryouts SET grace_seconds = $3, updated_at = now() WHERE tenant_id = $1 AND id = $2`,
		tenantID, id, seconds)
	if err != nil {
		return fmt.Errorf("set tryout grace: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// ProgramExists reports whether the program is this tenant's; the tryout's program FK needs it too.
func (r *TryoutRepository) ProgramExists(ctx context.Context, tenantID, programID uuid.UUID) (bool, error) {
	if tenantID == uuid.Nil {
		return false, ErrScopeRequired
	}
	var ok bool
	if err := r.db.QueryRow(ctx,
		`SELECT EXISTS (SELECT 1 FROM programs WHERE tenant_id = $1 AND id = $2)`,
		tenantID, programID).Scan(&ok); err != nil {
		return false, fmt.Errorf("check tryout program: %w", err)
	}
	return ok, nil
}

// CurrentQuestions resolves the tenant's questions to the version that is current right now.
func (r *TryoutRepository) CurrentQuestions(ctx context.Context, q queryer, tenantID uuid.UUID, ids []uuid.UUID) (map[uuid.UUID]model.QuestionKey, error) {
	if tenantID == uuid.Nil {
		return nil, ErrScopeRequired
	}
	rows, err := q.Query(ctx, `
		SELECT q.id, v.id, v.version
		  FROM questions q
		  JOIN question_versions v
		    ON v.question_id = q.id AND v.version = q.current_version AND v.tenant_id = q.tenant_id
		 WHERE q.tenant_id = $1 AND q.id = ANY($2)`, tenantID, ids)
	if err != nil {
		return nil, fmt.Errorf("read current question versions: %w", err)
	}
	defer rows.Close()
	out := make(map[uuid.UUID]model.QuestionKey, len(ids))
	for rows.Next() {
		var key model.QuestionKey
		if err := rows.Scan(&key.QuestionID, &key.VersionID, &key.Version); err != nil {
			return nil, fmt.Errorf("scan current question version: %w", err)
		}
		out[key.QuestionID] = key
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read current question versions: %w", err)
	}
	return out, nil
}

// AssignedVersions is what "an assignment already in the tryout keeps its version" compares against.
func (r *TryoutRepository) AssignedVersions(ctx context.Context, q queryer, tenantID, tryoutID uuid.UUID) (map[uuid.UUID]uuid.UUID, error) {
	if tenantID == uuid.Nil {
		return nil, ErrScopeRequired
	}
	rows, err := q.Query(ctx,
		`SELECT question_id, question_version_id FROM tryout_questions WHERE tenant_id = $1 AND tryout_id = $2`,
		tenantID, tryoutID)
	if err != nil {
		return nil, fmt.Errorf("read assigned versions: %w", err)
	}
	defer rows.Close()
	out := map[uuid.UUID]uuid.UUID{}
	for rows.Next() {
		var questionID, versionID uuid.UUID
		if err := rows.Scan(&questionID, &versionID); err != nil {
			return nil, fmt.Errorf("scan assigned version: %w", err)
		}
		out[questionID] = versionID
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read assigned versions: %w", err)
	}
	return out, nil
}

func (r *TryoutRepository) insertHeadTx(ctx context.Context, tx pgx.Tx, tenantID, createdBy uuid.UUID, tryout model.Tryout) (uuid.UUID, error) {
	var id uuid.UUID
	err := tx.QueryRow(ctx, `
		INSERT INTO tryouts (tenant_id, program_id, title, description, scoring_mode, active_from,
		        active_until, explanation_release_at, randomize_questions, randomize_options, grace_seconds, created_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
		RETURNING id`,
		tenantID, tryout.ProgramID, tryout.Title, tryout.Description, tryout.ScoringMode,
		tryout.ActiveFrom, tryout.ActiveUntil, tryout.ExplanationReleaseAt,
		tryout.RandomizeQuestions, tryout.RandomizeOptions, tryout.GraceSeconds, createdBy).Scan(&id)
	return id, mapTryoutError("create tryout", err)
}

func (r *TryoutRepository) updateHeadTx(ctx context.Context, tx pgx.Tx, tenantID, id uuid.UUID, tryout model.Tryout) error {
	tag, err := tx.Exec(ctx, `
		UPDATE tryouts
		   SET program_id = $3, title = $4, description = $5, scoring_mode = $6, active_from = $7,
		       active_until = $8, explanation_release_at = $9, randomize_questions = $10,
		       randomize_options = $11, grace_seconds = $12, revision = revision + 1, updated_at = now()
		 WHERE tenant_id = $1 AND id = $2`,
		tenantID, id, tryout.ProgramID, tryout.Title, tryout.Description, tryout.ScoringMode,
		tryout.ActiveFrom, tryout.ActiveUntil, tryout.ExplanationReleaseAt,
		tryout.RandomizeQuestions, tryout.RandomizeOptions, tryout.GraceSeconds)
	if err != nil {
		return mapTryoutError("update tryout", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *TryoutRepository) replaceChildrenTx(ctx context.Context, tx pgx.Tx, tenantID, id uuid.UUID, subtests []model.Subtest) error {
	if err := r.deleteChildrenTx(ctx, tx, tenantID, id); err != nil {
		return err
	}
	for i, subtest := range subtests {
		var subtestID uuid.UUID
		if err := tx.QueryRow(ctx, `
			INSERT INTO tryout_subtests (tryout_id, tenant_id, name, duration_seconds, position)
			VALUES ($1, $2, $3, $4, $5) RETURNING id`,
			id, tenantID, subtest.Name, subtest.DurationSeconds, i+1).Scan(&subtestID); err != nil {
			return mapTryoutError("create subtest", err)
		}
		for j, question := range subtest.Questions {
			if _, err := tx.Exec(ctx, `
				INSERT INTO tryout_questions (subtest_id, tryout_id, tenant_id, position, question_id, question_version_id)
				VALUES ($1, $2, $3, $4, $5, $6)`,
				subtestID, id, tenantID, j+1, question.QuestionID, question.VersionID); err != nil {
				return mapTryoutError("create assignment", err)
			}
		}
	}
	return nil
}

func (r *TryoutRepository) deleteChildrenTx(ctx context.Context, tx pgx.Tx, tenantID, id uuid.UUID) error {
	for _, sql := range []string{
		`DELETE FROM tryout_questions WHERE tenant_id = $1 AND tryout_id = $2`,
		`DELETE FROM tryout_subtests WHERE tenant_id = $1 AND tryout_id = $2`,
	} {
		if _, err := tx.Exec(ctx, sql, tenantID, id); err != nil {
			return mapTryoutError("delete tryout content", err)
		}
	}
	return nil
}

func (r *TryoutRepository) find(ctx context.Context, q queryer, tenantID, id uuid.UUID, forUpdate bool) (model.Tryout, error) {
	sql := `SELECT ` + tryoutHeadColumns + ` FROM tryouts WHERE tenant_id = $1 AND id = $2`
	if forUpdate {
		sql += ` FOR UPDATE`
	}
	out, err := scanTryoutHead(q.QueryRow(ctx, sql, tenantID, id), false)
	if err != nil {
		return model.Tryout{}, err
	}
	if forUpdate {
		return out, nil
	}
	if err := r.attachSubtests(ctx, q, tenantID, id, &out); err != nil {
		return model.Tryout{}, err
	}
	return out, nil
}

func (r *TryoutRepository) attachSubtests(ctx context.Context, q queryer, tenantID, id uuid.UUID, tryout *model.Tryout) error {
	rows, err := q.Query(ctx,
		`SELECT id, name, duration_seconds, position FROM tryout_subtests
		  WHERE tenant_id = $1 AND tryout_id = $2 ORDER BY position`, tenantID, id)
	if err != nil {
		return fmt.Errorf("read subtests: %w", err)
	}
	defer rows.Close()
	index := map[uuid.UUID]int{}
	for rows.Next() {
		var subtest model.Subtest
		if err := rows.Scan(&subtest.ID, &subtest.Name, &subtest.DurationSeconds, &subtest.Position); err != nil {
			return fmt.Errorf("scan subtest: %w", err)
		}
		subtest.Questions = []model.Assignment{}
		index[subtest.ID] = len(tryout.Subtests)
		tryout.Subtests = append(tryout.Subtests, subtest)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("read subtests: %w", err)
	}

	// The version row is joined by id, never by the question's current version: that is the freeze.
	rows, err = q.Query(ctx, `
		SELECT tq.subtest_id, tq.position, tq.question_id, tq.question_version_id,
		       v.version, q.seq, v.type, v.difficulty, v.content
		  FROM tryout_questions tq
		  JOIN question_versions v ON v.id = tq.question_version_id AND v.tenant_id = tq.tenant_id
		  JOIN questions q ON q.id = tq.question_id AND q.tenant_id = tq.tenant_id
		 WHERE tq.tenant_id = $1 AND tq.tryout_id = $2
		 ORDER BY tq.position`, tenantID, id)
	if err != nil {
		return fmt.Errorf("read assignments: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var subtestID uuid.UUID
		var assignment model.Assignment
		if err := rows.Scan(&subtestID, &assignment.Position, &assignment.QuestionID, &assignment.VersionID,
			&assignment.Version, &assignment.Number, &assignment.Type, &assignment.Difficulty,
			&assignment.Content); err != nil {
			return fmt.Errorf("scan assignment: %w", err)
		}
		if at, ok := index[subtestID]; ok {
			tryout.Subtests[at].Questions = append(tryout.Subtests[at].Questions, assignment)
		}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("read assignments: %w", err)
	}
	for _, subtest := range tryout.Subtests {
		tryout.SubtestCount++
		tryout.QuestionCount += len(subtest.Questions)
	}
	return nil
}

func scanTryoutHead(row pgx.Row, withCounts bool) (model.Tryout, error) {
	var out model.Tryout
	var activeFrom, releaseAt *time.Time
	targets := []any{&out.ID, &out.TenantID, &out.ProgramID, &out.Title, &out.Description,
		&out.ScoringMode, &out.Status, &activeFrom, &out.ActiveUntil, &releaseAt,
		&out.RandomizeQuestions, &out.RandomizeOptions, &out.GraceSeconds, &out.Revision, &out.Locked,
		&out.CreatedAt, &out.UpdatedAt}
	if withCounts {
		targets = append(targets, &out.SubtestCount, &out.QuestionCount)
	}
	if err := row.Scan(targets...); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return model.Tryout{}, ErrNotFound
		}
		return model.Tryout{}, fmt.Errorf("scan tryout: %w", err)
	}
	out.ActiveFrom, out.ExplanationReleaseAt = activeFrom, releaseAt
	return out, nil
}

func (r *TryoutRepository) inTx(ctx context.Context, fn func(pgx.Tx) error) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin: %w", err)
	}
	defer func() { _ = tx.Rollback(context.WithoutCancel(ctx)) }()
	if err := fn(tx); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit: %w", err)
	}
	return nil
}

func mapTryoutError(what string, err error) error {
	if err == nil {
		return nil
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == pgForeignKeyViolation {
		return fmt.Errorf("%w: %s", ErrReferenced, what)
	}
	if errors.Is(err, ErrNotFound) {
		return ErrNotFound
	}
	return fmt.Errorf("%s: %w", what, err)
}
