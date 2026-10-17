package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/tarunaeduka/bimbel-tryout/api/internal/model"
)

var (
	// ErrDuplicateCode is the per-tenant program code, so the service can answer 409 with fields.code.
	ErrDuplicateCode = fmt.Errorf("%w: code", ErrDuplicate)
	// ErrReferenced is SQLSTATE 23503: a row is still pointed at, so the delete must roll back.
	ErrReferenced = errors.New("referenced")
	// The two narrow forms let the 409 say which level of the tree is still in use.
	ErrReferencedChapters = fmt.Errorf("%w: chapter", ErrReferenced)
	ErrReferencedSubjects = fmt.Errorf("%w: subject", ErrReferenced)
)

const pgForeignKeyViolation = "23503"

type CurriculumRepository struct {
	db *pgxpool.Pool
}

func NewCurriculumRepository(db *pgxpool.Pool) *CurriculumRepository {
	return &CurriculumRepository{db: db}
}

const (
	programColumns = `id, tenant_id, code, name, description, position, created_at`
	subjectColumns = `id, tenant_id, program_id, name, position, created_at`
	chapterColumns = `id, tenant_id, subject_id, name, position, created_at`
)

func (r *CurriculumRepository) List(ctx context.Context, tenantID uuid.UUID) ([]model.ProgramTree, error) {
	if tenantID == uuid.Nil {
		return nil, ErrScopeRequired
	}
	programs, err := r.programs(ctx, r.db, tenantID)
	if err != nil {
		return nil, err
	}
	trees := make([]model.ProgramTree, len(programs))
	at := make(map[uuid.UUID]int, len(programs))
	for i, p := range programs {
		trees[i] = model.ProgramTree{Program: p}
		at[p.ID] = i
	}
	if err := r.attachSubjects(ctx, r.db, tenantID, trees, at); err != nil {
		return nil, err
	}
	if err := r.attachChapters(ctx, r.db, tenantID, trees); err != nil {
		return nil, err
	}
	return trees, nil
}

// FindTreeTx is the locked read the service diffs a replace against; the lock serializes writers.
func (r *CurriculumRepository) FindTreeTx(ctx context.Context, tx pgx.Tx, tenantID, id uuid.UUID, forUpdate bool) (model.ProgramTree, error) {
	return r.findTree(ctx, tx, tenantID, id, forUpdate)
}

func (r *CurriculumRepository) CreateTree(ctx context.Context, tenantID uuid.UUID, tree model.ProgramTree) (model.ProgramTree, error) {
	if tenantID == uuid.Nil {
		return model.ProgramTree{}, ErrScopeRequired
	}
	var out model.ProgramTree
	err := r.inTx(ctx, func(tx pgx.Tx) error {
		var id uuid.UUID
		// New programs append to the tenant's order; the wire has no position field for them.
		err := tx.QueryRow(ctx,
			`INSERT INTO programs (tenant_id, code, name, description, position)
			 VALUES ($1, $2, $3, $4,
			         (SELECT COALESCE(MAX(position) + 1, 0) FROM programs WHERE tenant_id = $1))
			 RETURNING id`,
			tenantID, tree.Program.Code, tree.Program.Name, tree.Program.Description).Scan(&id)
		if err != nil {
			return mapCurriculumError("create program", err)
		}
		for si, subject := range tree.Subjects {
			subjectID, err := insertSubject(ctx, tx, tenantID, id, subject.Subject.Name, si)
			if err != nil {
				return err
			}
			for ci, chapter := range subject.Chapters {
				if _, err := insertChapter(ctx, tx, tenantID, subjectID, chapter.Name, ci); err != nil {
					return err
				}
			}
		}
		out, err = r.findTree(ctx, tx, tenantID, id, false)
		return err
	})
	return out, err
}

func (r *CurriculumRepository) ReplaceTreeTx(ctx context.Context, tx pgx.Tx, tenantID, id uuid.UUID, tree model.ProgramTree) (model.ProgramTree, error) {
	if tenantID == uuid.Nil {
		return model.ProgramTree{}, ErrScopeRequired
	}
	_, err := tx.Exec(ctx,
		`UPDATE programs SET code = $3, name = $4, description = $5 WHERE tenant_id = $1 AND id = $2`,
		tenantID, id, tree.Program.Code, tree.Program.Name, tree.Program.Description)
	if err != nil {
		return model.ProgramTree{}, mapCurriculumError("update program", err)
	}

	// Name indexes are not deferrable: free every name first so one save can reuse or swap names.
	if _, err := tx.Exec(ctx,
		`UPDATE subjects SET name = '~' || id::text WHERE tenant_id = $1 AND program_id = $2`,
		tenantID, id); err != nil {
		return model.ProgramTree{}, mapCurriculumError("free subject names", err)
	}
	if _, err := tx.Exec(ctx,
		`UPDATE chapters SET name = '~' || id::text
		  WHERE tenant_id = $1 AND subject_id IN (SELECT id FROM subjects WHERE tenant_id = $1 AND program_id = $2)`,
		tenantID, id); err != nil {
		return model.ProgramTree{}, mapCurriculumError("free chapter names", err)
	}

	keptSubjects := make([]string, 0, len(tree.Subjects))
	subjectIDs := make([]uuid.UUID, len(tree.Subjects))
	for si, subject := range tree.Subjects {
		if subject.Subject.ID == uuid.Nil {
			subjectID, err := insertSubject(ctx, tx, tenantID, id, subject.Subject.Name, si)
			if err != nil {
				return model.ProgramTree{}, err
			}
			subjectIDs[si] = subjectID
		} else {
			_, err := tx.Exec(ctx,
				`UPDATE subjects SET name = $4, position = $5 WHERE tenant_id = $1 AND program_id = $2 AND id = $3`,
				tenantID, id, subject.Subject.ID, subject.Subject.Name, si)
			if err != nil {
				return model.ProgramTree{}, mapCurriculumError("update subject", err)
			}
			subjectIDs[si] = subject.Subject.ID
		}
		keptSubjects = append(keptSubjects, subjectIDs[si].String())
	}

	keptChapters := make([]string, 0)
	for si, subject := range tree.Subjects {
		for ci, chapter := range subject.Chapters {
			chapterID, err := upsertChapter(ctx, tx, tenantID, id, subjectIDs[si], chapter, ci)
			if err != nil {
				return model.ProgramTree{}, err
			}
			keptChapters = append(keptChapters, chapterID.String())
		}
	}

	if err := r.pruneChapters(ctx, tx, tenantID, id, keptChapters); err != nil {
		return model.ProgramTree{}, err
	}
	if err := r.pruneSubjects(ctx, tx, tenantID, id, keptSubjects); err != nil {
		return model.ProgramTree{}, err
	}
	return r.findTree(ctx, tx, tenantID, id, false)
}

func (r *CurriculumRepository) DeleteChaptersTx(ctx context.Context, tx pgx.Tx, tenantID, programID uuid.UUID) error {
	if tenantID == uuid.Nil {
		return ErrScopeRequired
	}
	err := execCurriculum(ctx, tx, "delete chapters",
		`DELETE FROM chapters c USING subjects s
		  WHERE c.subject_id = s.id AND s.tenant_id = $1 AND s.program_id = $2`, tenantID, programID)
	if errors.Is(err, ErrReferenced) {
		return ErrReferencedChapters
	}
	return err
}

func (r *CurriculumRepository) DeleteSubjectsTx(ctx context.Context, tx pgx.Tx, tenantID, programID uuid.UUID) error {
	if tenantID == uuid.Nil {
		return ErrScopeRequired
	}
	err := execCurriculum(ctx, tx, "delete subjects",
		`DELETE FROM subjects WHERE tenant_id = $1 AND program_id = $2`, tenantID, programID)
	if errors.Is(err, ErrReferenced) {
		return ErrReferencedSubjects
	}
	return err
}

func (r *CurriculumRepository) DeleteProgramTx(ctx context.Context, tx pgx.Tx, tenantID, id uuid.UUID) error {
	if tenantID == uuid.Nil {
		return ErrScopeRequired
	}
	return execCurriculum(ctx, tx, "delete program",
		`DELETE FROM programs WHERE tenant_id = $1 AND id = $2`, tenantID, id)
}

func upsertChapter(ctx context.Context, tx pgx.Tx, tenantID, programID, subjectID uuid.UUID, chapter model.Chapter, position int) (uuid.UUID, error) {
	if chapter.ID == uuid.Nil {
		return insertChapter(ctx, tx, tenantID, subjectID, chapter.Name, position)
	}
	tag, err := tx.Exec(ctx,
		`UPDATE chapters SET subject_id = $4, name = $5, position = $6
		  WHERE tenant_id = $1 AND id = $2 AND subject_id IN
		        (SELECT id FROM subjects WHERE tenant_id = $1 AND program_id = $3)`,
		tenantID, chapter.ID, programID, subjectID, chapter.Name, position)
	if err != nil {
		return uuid.Nil, mapCurriculumError("update chapter", err)
	}
	if tag.RowsAffected() == 0 {
		return uuid.Nil, ErrNotFound
	}
	return chapter.ID, nil
}

func insertSubject(ctx context.Context, tx pgx.Tx, tenantID, programID uuid.UUID, name string, position int) (uuid.UUID, error) {
	var id uuid.UUID
	err := tx.QueryRow(ctx,
		`INSERT INTO subjects (tenant_id, program_id, name, position) VALUES ($1, $2, $3, $4) RETURNING id`,
		tenantID, programID, name, position).Scan(&id)
	if err != nil {
		return uuid.Nil, mapCurriculumError("create subject", err)
	}
	return id, nil
}

func insertChapter(ctx context.Context, tx pgx.Tx, tenantID, subjectID uuid.UUID, name string, position int) (uuid.UUID, error) {
	var id uuid.UUID
	err := tx.QueryRow(ctx,
		`INSERT INTO chapters (tenant_id, subject_id, name, position) VALUES ($1, $2, $3, $4) RETURNING id`,
		tenantID, subjectID, name, position).Scan(&id)
	if err != nil {
		return uuid.Nil, mapCurriculumError("create chapter", err)
	}
	return id, nil
}

func (r *CurriculumRepository) findTree(ctx context.Context, q queryer, tenantID, id uuid.UUID, forUpdate bool) (model.ProgramTree, error) {
	if tenantID == uuid.Nil {
		return model.ProgramTree{}, ErrScopeRequired
	}
	sql := `SELECT ` + programColumns + ` FROM programs WHERE tenant_id = $1 AND id = $2`
	if forUpdate {
		sql += ` FOR UPDATE`
	}
	var p model.Program
	err := q.QueryRow(ctx, sql, tenantID, id).Scan(&p.ID, &p.TenantID, &p.Code, &p.Name, &p.Description, &p.Position, &p.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return model.ProgramTree{}, ErrNotFound
	}
	if err != nil {
		return model.ProgramTree{}, fmt.Errorf("read program: %w", err)
	}

	trees := []model.ProgramTree{{Program: p}}
	if err := r.attachSubjects(ctx, q, tenantID, trees, map[uuid.UUID]int{p.ID: 0}); err != nil {
		return model.ProgramTree{}, err
	}
	if err := r.attachChapters(ctx, q, tenantID, trees); err != nil {
		return model.ProgramTree{}, err
	}
	return trees[0], nil
}

func (r *CurriculumRepository) programs(ctx context.Context, q queryer, tenantID uuid.UUID) ([]model.Program, error) {
	rows, err := q.Query(ctx,
		`SELECT `+programColumns+` FROM programs WHERE tenant_id = $1 ORDER BY position, code`, tenantID)
	if err != nil {
		return nil, fmt.Errorf("list programs: %w", err)
	}
	defer rows.Close()

	var out []model.Program
	for rows.Next() {
		var p model.Program
		if err := rows.Scan(&p.ID, &p.TenantID, &p.Code, &p.Name, &p.Description, &p.Position, &p.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan program: %w", err)
		}
		out = append(out, p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list programs: %w", err)
	}
	return out, nil
}

func (r *CurriculumRepository) attachSubjects(ctx context.Context, q queryer, tenantID uuid.UUID, trees []model.ProgramTree, at map[uuid.UUID]int) error {
	if len(at) == 0 {
		return nil
	}
	ids := make([]string, 0, len(at))
	for id := range at {
		ids = append(ids, id.String())
	}
	rows, err := q.Query(ctx,
		`SELECT `+subjectColumns+` FROM subjects
		  WHERE tenant_id = $1 AND program_id = ANY($2::uuid[]) ORDER BY position, name`, tenantID, ids)
	if err != nil {
		return fmt.Errorf("list subjects: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var s model.Subject
		if err := rows.Scan(&s.ID, &s.TenantID, &s.ProgramID, &s.Name, &s.Position, &s.CreatedAt); err != nil {
			return fmt.Errorf("scan subject: %w", err)
		}
		i, ok := at[s.ProgramID]
		if !ok {
			continue
		}
		trees[i].Subjects = append(trees[i].Subjects, model.SubjectTree{Subject: s})
	}
	return rows.Err()
}

func (r *CurriculumRepository) attachChapters(ctx context.Context, q queryer, tenantID uuid.UUID, trees []model.ProgramTree) error {
	type ref struct{ tree, subject int }
	at := map[uuid.UUID]ref{}
	ids := make([]string, 0)
	for ti := range trees {
		for si, subject := range trees[ti].Subjects {
			at[subject.Subject.ID] = ref{tree: ti, subject: si}
			ids = append(ids, subject.Subject.ID.String())
		}
	}
	if len(ids) == 0 {
		return nil
	}
	rows, err := q.Query(ctx,
		`SELECT `+chapterColumns+` FROM chapters
		  WHERE tenant_id = $1 AND subject_id = ANY($2::uuid[]) ORDER BY position, name`, tenantID, ids)
	if err != nil {
		return fmt.Errorf("list chapters: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var c model.Chapter
		if err := rows.Scan(&c.ID, &c.TenantID, &c.SubjectID, &c.Name, &c.Position, &c.CreatedAt); err != nil {
			return fmt.Errorf("scan chapter: %w", err)
		}
		where, ok := at[c.SubjectID]
		if !ok {
			continue
		}
		subject := &trees[where.tree].Subjects[where.subject]
		subject.Chapters = append(subject.Chapters, c)
	}
	return rows.Err()
}

func (r *CurriculumRepository) pruneChapters(ctx context.Context, tx pgx.Tx, tenantID, programID uuid.UUID, kept []string) error {
	_, err := tx.Exec(ctx,
		`DELETE FROM chapters c USING subjects s
		  WHERE c.subject_id = s.id AND s.tenant_id = $1 AND s.program_id = $2
		    AND c.id <> ALL($3::uuid[])`, tenantID, programID, kept)
	err = mapCurriculumError("delete chapters", err)
	if errors.Is(err, ErrReferenced) {
		return ErrReferencedChapters
	}
	return err
}

func (r *CurriculumRepository) pruneSubjects(ctx context.Context, tx pgx.Tx, tenantID, programID uuid.UUID, kept []string) error {
	_, err := tx.Exec(ctx,
		`DELETE FROM subjects WHERE tenant_id = $1 AND program_id = $2 AND id <> ALL($3::uuid[])`,
		tenantID, programID, kept)
	err = mapCurriculumError("delete subjects", err)
	if errors.Is(err, ErrReferenced) {
		return ErrReferencedSubjects
	}
	return err
}

func (r *CurriculumRepository) inTx(ctx context.Context, fn func(pgx.Tx) error) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := fn(tx); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit: %w", err)
	}
	return nil
}

func execCurriculum(ctx context.Context, tx pgx.Tx, what, sql string, args ...any) error {
	tag, err := tx.Exec(ctx, sql, args...)
	if err != nil {
		return mapCurriculumError(what, err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func mapCurriculumError(what string, err error) error {
	if err == nil {
		return nil
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch {
		case pgErr.Code == pgUniqueViolation && pgErr.ConstraintName == "programs_tenant_code_key":
			return ErrDuplicateCode
		case pgErr.Code == pgUniqueViolation && (pgErr.ConstraintName == "subjects_program_name_key" || pgErr.ConstraintName == "chapters_subject_name_key"):
			return ErrDuplicate
		case pgErr.Code == pgForeignKeyViolation:
			return fmt.Errorf("%w: %s", ErrReferenced, what)
		}
	}
	if errors.Is(err, ErrNotFound) {
		return ErrNotFound
	}
	return fmt.Errorf("%s: %w", what, err)
}

type queryer interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}
