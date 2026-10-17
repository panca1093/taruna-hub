package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/tarunaeduka/bimbel-tryout/api/internal/model"
)

// QuestionFilter is already validated by the service; the repository only turns it into SQL.
type QuestionFilter struct {
	ProgramID  *uuid.UUID
	SubjectID  *uuid.UUID
	Type       string
	Difficulty string
	Search     string
	Limit      int
	Offset     int
}

type QuestionRepository struct {
	db *pgxpool.Pool
}

func NewQuestionRepository(db *pgxpool.Pool) *QuestionRepository {
	return &QuestionRepository{db: db}
}

const questionColumns = `q.id, q.seq, q.tenant_id, q.current_version, q.created_at, q.updated_at,
	v.id, v.program_id, v.subject_id, v.chapter_id, v.type, v.content, v.difficulty, v.explanation,
	v.option_weighted, v.case_sensitive, v.options, v.correct, v.created_by`

// questionSource joins the head row to the version it currently points at; the tenant pair keeps a
// version of another tenant from ever joining.
const questionSource = ` FROM questions q
	JOIN question_versions v
	  ON v.question_id = q.id AND v.version = q.current_version AND v.tenant_id = q.tenant_id`

func (r *QuestionRepository) List(ctx context.Context, tenantID uuid.UUID, filter QuestionFilter) ([]model.Question, int, error) {
	if tenantID == uuid.Nil {
		return nil, 0, ErrScopeRequired
	}
	where := []string{"q.tenant_id = $1"}
	args := []any{tenantID}
	add := func(condition string, value any) {
		args = append(args, value)
		where = append(where, fmt.Sprintf(condition, len(args)))
	}
	if filter.ProgramID != nil {
		add("v.program_id = $%d", *filter.ProgramID)
	}
	if filter.SubjectID != nil {
		add("v.subject_id = $%d", *filter.SubjectID)
	}
	if filter.Type != "" {
		add("v.type = $%d", filter.Type)
	}
	if filter.Difficulty != "" {
		add("v.difficulty = $%d", filter.Difficulty)
	}
	if filter.Search != "" {
		// ESCAPE is explicit so a user's % or _ is a literal character, not a wildcard.
		add(`v.content ILIKE $%d ESCAPE '\'`, likePattern(filter.Search))
	}
	from := questionSource + " WHERE " + strings.Join(where, " AND ")

	var total int
	if err := r.db.QueryRow(ctx, `SELECT count(*)`+from, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count questions: %w", err)
	}

	args = append(args, filter.Limit, filter.Offset)
	rows, err := r.db.Query(ctx, `SELECT `+questionColumns+from+
		fmt.Sprintf(` ORDER BY q.seq DESC LIMIT $%d OFFSET $%d`, len(args)-1, len(args)), args...)
	if err != nil {
		return nil, 0, fmt.Errorf("list questions: %w", err)
	}
	defer rows.Close()

	out := make([]model.Question, 0, filter.Limit)
	for rows.Next() {
		q, err := scanQuestion(rows)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, q)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("list questions: %w", err)
	}
	return out, total, nil
}

func (r *QuestionRepository) Find(ctx context.Context, tenantID, id uuid.UUID) (model.Question, error) {
	if tenantID == uuid.Nil {
		return model.Question{}, ErrScopeRequired
	}
	return r.find(ctx, r.db, tenantID, id, false)
}

// FindCurrentTx is the locked read an update diffs against; the lock serializes writers on the head
// row, so two saves of the same base version cannot both insert the next one.
func (r *QuestionRepository) FindCurrentTx(ctx context.Context, tx pgx.Tx, tenantID, id uuid.UUID, forUpdate bool) (model.Question, error) {
	if tenantID == uuid.Nil {
		return model.Question{}, ErrScopeRequired
	}
	return r.find(ctx, tx, tenantID, id, forUpdate)
}

// Create writes the head row and version 1 in one transaction, so a question never exists without
// content.
func (r *QuestionRepository) Create(ctx context.Context, tenantID, createdBy uuid.UUID, q model.Question) (model.Question, error) {
	if tenantID == uuid.Nil {
		return model.Question{}, ErrScopeRequired
	}
	var out model.Question
	err := r.inTx(ctx, func(tx pgx.Tx) error {
		var id uuid.UUID
		if err := tx.QueryRow(ctx,
			`INSERT INTO questions (tenant_id, current_version, created_by) VALUES ($1, 1, $2) RETURNING id`,
			tenantID, createdBy).Scan(&id); err != nil {
			return mapQuestionError("create question", err)
		}
		if err := insertQuestionVersion(ctx, tx, tenantID, id, 1, createdBy, q); err != nil {
			return err
		}
		var err error
		out, err = r.find(ctx, tx, tenantID, id, false)
		return err
	})
	return out, err
}

// InsertVersionTx appends one immutable version; the previous rows are never touched.
func (r *QuestionRepository) InsertVersionTx(ctx context.Context, tx pgx.Tx, tenantID, questionID uuid.UUID, version int, createdBy uuid.UUID, q model.Question) error {
	if tenantID == uuid.Nil {
		return ErrScopeRequired
	}
	return insertQuestionVersion(ctx, tx, tenantID, questionID, version, createdBy, q)
}

// TouchHeadTx moves the head to the version that was just written.
func (r *QuestionRepository) TouchHeadTx(ctx context.Context, tx pgx.Tx, tenantID, questionID uuid.UUID, version int) error {
	if tenantID == uuid.Nil {
		return ErrScopeRequired
	}
	tag, err := tx.Exec(ctx,
		`UPDATE questions SET current_version = $3, updated_at = now() WHERE tenant_id = $1 AND id = $2`,
		tenantID, questionID, version)
	if err != nil {
		return mapQuestionError("update question", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// DeleteTx removes the versions and then the head in one transaction; anything that still points at
// a version makes the database refuse, and the whole delete rolls back.
func (r *QuestionRepository) DeleteTx(ctx context.Context, tx pgx.Tx, tenantID, id uuid.UUID) error {
	if tenantID == uuid.Nil {
		return ErrScopeRequired
	}
	if _, err := tx.Exec(ctx,
		`DELETE FROM question_versions WHERE tenant_id = $1 AND question_id = $2`, tenantID, id); err != nil {
		return mapQuestionError("delete question versions", err)
	}
	tag, err := tx.Exec(ctx, `DELETE FROM questions WHERE tenant_id = $1 AND id = $2`, tenantID, id)
	if err != nil {
		return mapQuestionError("delete question", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// ProgramExists reports whether the program is this tenant's.
func (r *QuestionRepository) ProgramExists(ctx context.Context, tenantID, programID uuid.UUID) (bool, error) {
	return r.exists(ctx,
		`SELECT EXISTS (SELECT 1 FROM programs WHERE tenant_id = $1 AND id = $2)`, tenantID, programID)
}

// SubjectInProgram reports whether the subject is this tenant's and actually belongs to the program.
func (r *QuestionRepository) SubjectInProgram(ctx context.Context, tenantID, programID, subjectID uuid.UUID) (bool, error) {
	return r.exists(ctx,
		`SELECT EXISTS (SELECT 1 FROM subjects WHERE tenant_id = $1 AND id = $2 AND program_id = $3)`,
		tenantID, subjectID, programID)
}

// ChapterInSubject reports whether the chapter is this tenant's and actually belongs to the subject.
func (r *QuestionRepository) ChapterInSubject(ctx context.Context, tenantID, subjectID, chapterID uuid.UUID) (bool, error) {
	return r.exists(ctx,
		`SELECT EXISTS (SELECT 1 FROM chapters WHERE tenant_id = $1 AND id = $2 AND subject_id = $3)`,
		tenantID, chapterID, subjectID)
}

func (r *QuestionRepository) exists(ctx context.Context, sql string, args ...any) (bool, error) {
	if len(args) > 0 {
		if tenantID, ok := args[0].(uuid.UUID); ok && tenantID == uuid.Nil {
			return false, ErrScopeRequired
		}
	}
	var ok bool
	if err := r.db.QueryRow(ctx, sql, args...).Scan(&ok); err != nil {
		return false, fmt.Errorf("check question reference: %w", err)
	}
	return ok, nil
}

func (r *QuestionRepository) find(ctx context.Context, q queryer, tenantID, id uuid.UUID, forUpdate bool) (model.Question, error) {
	sql := `SELECT ` + questionColumns + questionSource + ` WHERE q.tenant_id = $1 AND q.id = $2`
	if forUpdate {
		sql += ` FOR UPDATE OF q`
	}
	row := q.QueryRow(ctx, sql, tenantID, id)
	var out model.Question
	var chapter *uuid.UUID
	var options, correct []byte
	err := row.Scan(&out.ID, &out.Number, &out.TenantID, &out.Version, &out.CreatedAt, &out.UpdatedAt,
		&out.VersionID, &out.ProgramID, &out.SubjectID, &chapter, &out.Type, &out.Content, &out.Difficulty,
		&out.Explanation, &out.OptionWeighted, &out.CaseSensitive, &options, &correct, &out.CreatedBy)
	if errors.Is(err, pgx.ErrNoRows) {
		return model.Question{}, ErrNotFound
	}
	if err != nil {
		return model.Question{}, fmt.Errorf("read question: %w", err)
	}
	out.ChapterID = chapter
	if err := decodeQuestionJSON(&out, options, correct); err != nil {
		return model.Question{}, err
	}
	return out, nil
}

func insertQuestionVersion(ctx context.Context, tx pgx.Tx, tenantID, questionID uuid.UUID, version int, createdBy uuid.UUID, q model.Question) error {
	options, err := json.Marshal(q.Options)
	if err != nil {
		return fmt.Errorf("encode options: %w", err)
	}
	correct, err := json.Marshal(q.Correct)
	if err != nil {
		return fmt.Errorf("encode correct: %w", err)
	}
	_, err = tx.Exec(ctx,
		`INSERT INTO question_versions (question_id, tenant_id, version, program_id, subject_id, chapter_id,
		        type, content, difficulty, explanation, option_weighted, case_sensitive, options, correct, created_by)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15)`,
		questionID, tenantID, version, q.ProgramID, q.SubjectID, q.ChapterID, q.Type, q.Content,
		q.Difficulty, q.Explanation, q.OptionWeighted, q.CaseSensitive, options, correct, createdBy)
	if err != nil {
		return mapQuestionError("create question version", err)
	}
	return nil
}

func scanQuestion(row pgx.Row) (model.Question, error) {
	var out model.Question
	var chapter *uuid.UUID
	var options, correct []byte
	err := row.Scan(&out.ID, &out.Number, &out.TenantID, &out.Version, &out.CreatedAt, &out.UpdatedAt,
		&out.VersionID, &out.ProgramID, &out.SubjectID, &chapter, &out.Type, &out.Content, &out.Difficulty,
		&out.Explanation, &out.OptionWeighted, &out.CaseSensitive, &options, &correct, &out.CreatedBy)
	if err != nil {
		return model.Question{}, fmt.Errorf("scan question: %w", err)
	}
	out.ChapterID = chapter
	if err := decodeQuestionJSON(&out, options, correct); err != nil {
		return model.Question{}, err
	}
	return out, nil
}

func decodeQuestionJSON(q *model.Question, options, correct []byte) error {
	if err := json.Unmarshal(options, &q.Options); err != nil {
		return fmt.Errorf("decode options: %w", err)
	}
	if err := json.Unmarshal(correct, &q.Correct); err != nil {
		return fmt.Errorf("decode correct: %w", err)
	}
	if q.Options == nil {
		q.Options = []model.QuestionOption{}
	}
	if q.Correct == nil {
		q.Correct = []string{}
	}
	return nil
}

// likePattern escapes the wildcards so `%` and `_` only ever match themselves.
func likePattern(s string) string {
	escaped := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(s)
	return "%" + escaped + "%"
}

func (r *QuestionRepository) inTx(ctx context.Context, fn func(pgx.Tx) error) error {
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

func mapQuestionError(what string, err error) error {
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
