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

const (
	questionColumns = `q.id, q.seq, q.tenant_id, q.current_version, q.created_at, q.updated_at,
	v.id, v.program_id, v.subject_id, v.chapter_id, v.type, v.content, v.difficulty, v.explanation,
	v.option_weighted, v.case_sensitive, v.options, v.correct, v.created_by, v.content_doc, v.explanation_doc`
	questionHeadColumns    = `id, seq, tenant_id, current_version, created_at, updated_at`
	questionVersionColumns = `id, program_id, subject_id, chapter_id, type, content, difficulty, explanation,
	option_weighted, case_sensitive, options, correct, created_by, content_doc, explanation_doc`
)

// The tenant pair keeps a version of another tenant from ever joining.
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

func (r *QuestionRepository) FindCurrentTx(ctx context.Context, tx pgx.Tx, tenantID, id uuid.UUID, forUpdate bool) (model.Question, error) {
	if tenantID == uuid.Nil {
		return model.Question{}, ErrScopeRequired
	}
	return r.find(ctx, tx, tenantID, id, forUpdate)
}

func (r *QuestionRepository) Create(ctx context.Context, tenantID, createdBy uuid.UUID, q model.Question) (model.Question, error) {
	if tenantID == uuid.Nil {
		return model.Question{}, ErrScopeRequired
	}
	var out model.Question
	err := r.inTx(ctx, func(tx pgx.Tx) error {
		id, err := r.CreateTx(ctx, tx, tenantID, createdBy, q)
		if err != nil {
			return err
		}
		out, err = r.find(ctx, tx, tenantID, id, false)
		return err
	})
	return out, err
}

// CreateTx inserts the question head and its first version in the caller's transaction.
func (r *QuestionRepository) CreateTx(ctx context.Context, tx pgx.Tx, tenantID, createdBy uuid.UUID, q model.Question) (uuid.UUID, error) {
	if tenantID == uuid.Nil {
		return uuid.Nil, ErrScopeRequired
	}
	var id uuid.UUID
	if err := tx.QueryRow(ctx,
		`INSERT INTO questions (tenant_id, current_version, created_by) VALUES ($1, 1, $2) RETURNING id`,
		tenantID, createdBy).Scan(&id); err != nil {
		return uuid.Nil, mapQuestionError("create question", err)
	}
	if err := insertQuestionVersion(ctx, tx, tenantID, id, 1, createdBy, q); err != nil {
		return uuid.Nil, err
	}
	return id, nil
}

// InsertVersionTx appends one immutable version; the previous rows are never touched.
func (r *QuestionRepository) InsertVersionTx(ctx context.Context, tx pgx.Tx, tenantID, questionID uuid.UUID, version int, createdBy uuid.UUID, q model.Question) error {
	if tenantID == uuid.Nil {
		return ErrScopeRequired
	}
	return insertQuestionVersion(ctx, tx, tenantID, questionID, version, createdBy, q)
}

// ImportCreateTx inserts one imported row and its first version, or reports the row already imported.
func (r *QuestionRepository) ImportCreateTx(ctx context.Context, tx pgx.Tx, tenantID, createdBy uuid.UUID, importKey string, q model.Question) (uuid.UUID, bool, error) {
	if tenantID == uuid.Nil {
		return uuid.Nil, false, ErrScopeRequired
	}
	var id uuid.UUID
	err := tx.QueryRow(ctx,
		`INSERT INTO questions (tenant_id, current_version, created_by, import_key)
		 VALUES ($1, 1, $2, $3)
		 ON CONFLICT (tenant_id, import_key) WHERE import_key IS NOT NULL DO NOTHING
		 RETURNING id`, tenantID, createdBy, importKey).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return uuid.Nil, false, nil
	}
	if err != nil {
		return uuid.Nil, false, mapQuestionError("create imported question", err)
	}
	if err := insertQuestionVersion(ctx, tx, tenantID, id, 1, createdBy, q); err != nil {
		return uuid.Nil, false, err
	}
	return id, true, nil
}

// CurriculumRef is one program, subject or chapter an imported row may name, with its parent id.
type CurriculumRef struct {
	Kind     string
	ID       uuid.UUID
	ParentID *uuid.UUID
	Name     string
}

// CurriculumRefs loads the tenant's curriculum once, so a 500-row file names it without a query per row.
func (r *QuestionRepository) CurriculumRefs(ctx context.Context, tenantID uuid.UUID) ([]CurriculumRef, error) {
	if tenantID == uuid.Nil {
		return nil, ErrScopeRequired
	}
	rows, err := r.db.Query(ctx, `
		SELECT kind, id, parent_id, name FROM (
			SELECT 'program' AS kind, id, NULL::uuid AS parent_id, name, position, 0 AS rank
			  FROM programs WHERE tenant_id = $1
			UNION ALL
			SELECT 'subject', id, program_id, name, position, 1 FROM subjects WHERE tenant_id = $1
			UNION ALL
			SELECT 'chapter', id, subject_id, name, position, 2 FROM chapters WHERE tenant_id = $1
		) AS refs
		ORDER BY rank, position, name, id`, tenantID)
	if err != nil {
		return nil, fmt.Errorf("read curriculum refs: %w", err)
	}
	defer rows.Close()

	out := []CurriculumRef{}
	for rows.Next() {
		var ref CurriculumRef
		if err := rows.Scan(&ref.Kind, &ref.ID, &ref.ParentID, &ref.Name); err != nil {
			return nil, fmt.Errorf("scan curriculum ref: %w", err)
		}
		out = append(out, ref)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read curriculum refs: %w", err)
	}
	return out, nil
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

// Two statements on purpose: one FOR UPDATE join would miss a version row committed after its snapshot.
func (r *QuestionRepository) find(ctx context.Context, q queryer, tenantID, id uuid.UUID, forUpdate bool) (model.Question, error) {
	sql := `SELECT ` + questionHeadColumns + ` FROM questions WHERE tenant_id = $1 AND id = $2`
	if forUpdate {
		sql += ` FOR UPDATE`
	}
	var out model.Question
	err := q.QueryRow(ctx, sql, tenantID, id).Scan(&out.ID, &out.Number, &out.TenantID, &out.Version,
		&out.CreatedAt, &out.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return model.Question{}, ErrNotFound
	}
	if err != nil {
		return model.Question{}, fmt.Errorf("read question: %w", err)
	}

	var chapter *uuid.UUID
	var options, correct, contentDoc, explanationDoc []byte
	var content, explanation string
	err = q.QueryRow(ctx,
		`SELECT `+questionVersionColumns+` FROM question_versions
		  WHERE tenant_id = $1 AND question_id = $2 AND version = $3`,
		tenantID, out.ID, out.Version).Scan(&out.VersionID, &out.ProgramID, &out.SubjectID, &chapter,
		&out.Type, &content, &out.Difficulty, &explanation, &out.OptionWeighted,
		&out.CaseSensitive, &options, &correct, &out.CreatedBy, &contentDoc, &explanationDoc)
	if errors.Is(err, pgx.ErrNoRows) {
		return model.Question{}, ErrNotFound
	}
	if err != nil {
		return model.Question{}, fmt.Errorf("read question version: %w", err)
	}
	out.ChapterID = chapter
	if err := decodeQuestionJSON(&out, options, correct, content, explanation, contentDoc, explanationDoc); err != nil {
		return model.Question{}, err
	}
	return out, nil
}

func insertQuestionVersion(ctx context.Context, tx pgx.Tx, tenantID, questionID uuid.UUID, version int, createdBy uuid.UUID, q model.Question) error {
	stored := make([]model.QuestionOption, len(q.Options))
	copy(stored, q.Options)
	for i := range stored {
		stored[i].Label = model.DocLegacyText(stored[i].Doc)
	}
	options, err := json.Marshal(stored)
	if err != nil {
		return fmt.Errorf("encode options: %w", err)
	}
	correct, err := json.Marshal(q.Correct)
	if err != nil {
		return fmt.Errorf("encode correct: %w", err)
	}
	contentDoc, err := json.Marshal(q.Content)
	if err != nil {
		return fmt.Errorf("encode content: %w", err)
	}
	explanationDoc, err := json.Marshal(q.Explanation)
	if err != nil {
		return fmt.Errorf("encode explanation: %w", err)
	}
	_, err = tx.Exec(ctx,
		`INSERT INTO question_versions (question_id, tenant_id, version, program_id, subject_id, chapter_id,
		        type, content, difficulty, explanation, option_weighted, case_sensitive, options, correct, created_by,
		        content_doc, explanation_doc)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17)`,
		questionID, tenantID, version, q.ProgramID, q.SubjectID, q.ChapterID, q.Type, model.DocLegacyText(q.Content),
		q.Difficulty, model.DocLegacyText(q.Explanation), q.OptionWeighted, q.CaseSensitive, options, correct, createdBy,
		contentDoc, explanationDoc)
	if err != nil {
		return mapQuestionError("create question version", err)
	}
	return nil
}

func scanQuestion(row pgx.Row) (model.Question, error) {
	var out model.Question
	var chapter *uuid.UUID
	var options, correct, contentDoc, explanationDoc []byte
	var content, explanation string
	err := row.Scan(&out.ID, &out.Number, &out.TenantID, &out.Version, &out.CreatedAt, &out.UpdatedAt,
		&out.VersionID, &out.ProgramID, &out.SubjectID, &chapter, &out.Type, &content, &out.Difficulty,
		&explanation, &out.OptionWeighted, &out.CaseSensitive, &options, &correct, &out.CreatedBy,
		&contentDoc, &explanationDoc)
	if err != nil {
		return model.Question{}, fmt.Errorf("scan question: %w", err)
	}
	out.ChapterID = chapter
	if err := decodeQuestionJSON(&out, options, correct, content, explanation, contentDoc, explanationDoc); err != nil {
		return model.Question{}, err
	}
	return out, nil
}

func decodeQuestionJSON(q *model.Question, options, correct []byte, content, explanation string, contentDoc, explanationDoc []byte) error {
	var err error
	if q.Options, err = decodeOptions(options); err != nil {
		return err
	}
	if err := json.Unmarshal(correct, &q.Correct); err != nil {
		return fmt.Errorf("decode correct: %w", err)
	}
	if q.Correct == nil {
		q.Correct = []string{}
	}
	if q.Content, err = model.DocFromColumn(contentDoc, content); err != nil {
		return fmt.Errorf("decode content: %w", err)
	}
	if q.Explanation, err = model.DocFromColumn(explanationDoc, explanation); err != nil {
		return fmt.Errorf("decode explanation: %w", err)
	}
	return nil
}

// decodeOptions reads a stored options array; an option written before documents existed keeps its label as legacy
// text on its doc, for the service to convert.
func decodeOptions(raw []byte) ([]model.QuestionOption, error) {
	var options []model.QuestionOption
	if err := json.Unmarshal(raw, &options); err != nil {
		return nil, fmt.Errorf("decode options: %w", err)
	}
	if options == nil {
		return []model.QuestionOption{}, nil
	}
	return model.OptionsWithDocs(options), nil
}

// backfillLockKey is the advisory lock that keeps two API instances from converting the same rows.
const backfillLockKey = 7_001_030

// LegacyVersion is a question version stored before documents existed, as the columns hold it.
type LegacyVersion struct {
	ID          uuid.UUID
	Content     string
	Explanation string
	Options     []byte
}

// VersionDocs is what the service made of a LegacyVersion; the repository writes it as given.
type VersionDocs struct {
	Content     model.RichDoc
	Explanation model.RichDoc
	Options     []model.QuestionOption
}

// SkippedVersion is a legacy row the converter refused; it stays as it was and reads fall back to its text.
type SkippedVersion struct {
	ID  uuid.UUID
	Err error
}

// BackfillResult is what one backfill run did. Locked is false when another instance holds the lock, in which
// case nothing was read or written.
type BackfillResult struct {
	Locked    bool
	Converted int
	Skipped   []SkippedVersion
}

// BackfillDocs hands every version without documents to convert, batch rows at a time in id order, and writes
// what it returns. Only the document columns and the option docs change; the text columns stay as they were.
// It does not wait for the advisory lock: an instance that cannot take it reports Locked=false and the caller
// serves anyway, since reads fall back to the text. A row convert refuses is reported in Skipped and passed over.
//
// The scan is `content_doc IS NULL AND id > $last ORDER BY id` over the primary key, once per start-up; it ends
// at the first short batch, so a fully converted table costs one pass over the primary key index.
func (r *QuestionRepository) BackfillDocs(ctx context.Context, batch int, convert func(LegacyVersion) (VersionDocs, error)) (BackfillResult, error) {
	conn, err := r.db.Acquire(ctx)
	if err != nil {
		return BackfillResult{}, fmt.Errorf("acquire backfill connection: %w", err)
	}
	defer conn.Release()
	var locked bool
	if err := conn.QueryRow(ctx, `SELECT pg_try_advisory_lock($1)`, backfillLockKey).Scan(&locked); err != nil {
		return BackfillResult{}, fmt.Errorf("take backfill lock: %w", err)
	}
	if !locked {
		return BackfillResult{}, nil
	}
	defer func() {
		var released bool
		err := conn.QueryRow(context.WithoutCancel(ctx), `SELECT pg_advisory_unlock($1)`, backfillLockKey).Scan(&released)
		if err != nil || !released {
			// A connection that may still hold the lock must not go back to the pool; closing it frees the lock.
			_ = conn.Conn().Close(context.WithoutCancel(ctx))
		}
	}()

	result := BackfillResult{Locked: true}
	last := uuid.Nil
	for {
		n, next, err := r.backfillBatch(ctx, conn, last, batch, convert, &result)
		if err != nil {
			return result, err
		}
		last = next
		if n < batch {
			return result, nil
		}
	}
}

// backfillBatch returns how many rows it read and the id of the last one, converted or skipped.
func (r *QuestionRepository) backfillBatch(ctx context.Context, conn *pgxpool.Conn, after uuid.UUID, batch int, convert func(LegacyVersion) (VersionDocs, error), result *BackfillResult) (int, uuid.UUID, error) {
	tx, err := conn.Begin(ctx)
	if err != nil {
		return 0, after, fmt.Errorf("begin backfill: %w", err)
	}
	defer func() { _ = tx.Rollback(context.WithoutCancel(ctx)) }()

	rows, err := tx.Query(ctx,
		`SELECT id, content, explanation, options FROM question_versions
		  WHERE content_doc IS NULL AND id > $1 ORDER BY id LIMIT $2 FOR UPDATE`, after, batch)
	if err != nil {
		return 0, after, fmt.Errorf("read legacy versions: %w", err)
	}
	var found []LegacyVersion
	for rows.Next() {
		var l LegacyVersion
		if err := rows.Scan(&l.ID, &l.Content, &l.Explanation, &l.Options); err != nil {
			rows.Close()
			return 0, after, fmt.Errorf("scan legacy version: %w", err)
		}
		found = append(found, l)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return 0, after, fmt.Errorf("read legacy versions: %w", err)
	}

	converted := 0
	for _, l := range found {
		docs, err := convert(l)
		if err == nil {
			err = writeVersionDocs(ctx, tx, l.ID, docs)
		}
		if err != nil {
			result.Skipped = append(result.Skipped, SkippedVersion{ID: l.ID, Err: err})
			continue
		}
		converted++
	}
	if err := tx.Commit(ctx); err != nil {
		return 0, after, fmt.Errorf("commit backfill: %w", err)
	}
	result.Converted += converted
	if len(found) == 0 {
		return 0, after, nil
	}
	return len(found), found[len(found)-1].ID, nil
}

// writeVersionDocs runs inside a savepoint, so one row's failed write cannot abort the batch's transaction.
func writeVersionDocs(ctx context.Context, tx pgx.Tx, id uuid.UUID, docs VersionDocs) error {
	options, err := json.Marshal(docs.Options)
	if err != nil {
		return fmt.Errorf("encode options: %w", err)
	}
	content, err := json.Marshal(docs.Content)
	if err != nil {
		return fmt.Errorf("encode content: %w", err)
	}
	explanation, err := json.Marshal(docs.Explanation)
	if err != nil {
		return fmt.Errorf("encode explanation: %w", err)
	}
	sp, err := tx.Begin(ctx)
	if err != nil {
		return fmt.Errorf("savepoint: %w", err)
	}
	if _, err := sp.Exec(ctx,
		`UPDATE question_versions SET content_doc = $2, explanation_doc = $3, options = $4 WHERE id = $1`,
		id, content, explanation, options); err != nil {
		_ = sp.Rollback(context.WithoutCancel(ctx))
		return fmt.Errorf("write documents: %w", err)
	}
	return sp.Commit(ctx)
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
