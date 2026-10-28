package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/tarunaeduka/bimbel-tryout/api/internal/model"
)

// KecermatanFilter is already validated by the service; the repository only turns it into SQL.
type KecermatanFilter struct {
	ProgramID *uuid.UUID
	Search    string
	Limit     int
	Offset    int
}

// KecermatanPin is one package a test pins: the frozen revision and the pool size it must keep providing.
type KecermatanPin struct {
	PackageID     uuid.UUID
	Revision      int
	RequiredCount int
}

type KecermatanRepository struct {
	db *pgxpool.Pool
}

func NewKecermatanRepository(db *pgxpool.Pool) *KecermatanRepository {
	return &KecermatanRepository{db: db}
}

const kecermatanColumns = `t.id, t.tenant_id, t.program_id, t.title, t.description, t.item_type, t.item_length,
	t.options_per_soal, t.soal_per_kolom, t.kolom_count, t.kolom_duration_seconds, t.builtin_sources,
	t.cover_from, t.cover_to, t.generation_seed, t.revision, (t.first_attempt_at IS NOT NULL),
	t.created_at, t.updated_at`

func scanKecermatan(row pgx.Row) (model.KecermatanTest, error) {
	var t model.KecermatanTest
	err := row.Scan(&t.ID, &t.TenantID, &t.ProgramID, &t.Title, &t.Description, &t.ItemType, &t.ItemLength,
		&t.OptionsPerSoal, &t.SoalPerKolom, &t.KolomCount, &t.KolomDurationSeconds, &t.BuiltinSources,
		&t.CoverFrom, &t.CoverTo, &t.Seed, &t.Revision, &t.Locked, &t.CreatedAt, &t.UpdatedAt)
	return t, err
}

func kecermatanError(what string, err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == pgForeignKeyViolation {
		return fmt.Errorf("%w: %s", ErrReferenced, what)
	}
	if err != nil {
		return fmt.Errorf("%s: %w", what, err)
	}
	return nil
}

func (r *KecermatanRepository) List(ctx context.Context, tenantID uuid.UUID, filter KecermatanFilter) ([]model.KecermatanTest, int, error) {
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
	if filter.Search != "" {
		add(`t.title ILIKE $%d ESCAPE '\'`, likePattern(filter.Search))
	}
	from := " FROM kecermatan_tests t WHERE " + strings.Join(where, " AND ")

	var total int
	if err := r.db.QueryRow(ctx, `SELECT count(*)`+from, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count kecermatan tests: %w", err)
	}
	args = append(args, filter.Limit, filter.Offset)
	rows, err := r.db.Query(ctx, `SELECT `+kecermatanColumns+from+
		fmt.Sprintf(` ORDER BY t.created_at DESC, t.id DESC LIMIT $%d OFFSET $%d`, len(args)-1, len(args)), args...)
	if err != nil {
		return nil, 0, fmt.Errorf("list kecermatan tests: %w", err)
	}
	defer rows.Close()
	out := make([]model.KecermatanTest, 0, filter.Limit)
	for rows.Next() {
		test, err := scanKecermatan(rows)
		if err != nil {
			return nil, 0, fmt.Errorf("scan kecermatan test: %w", err)
		}
		out = append(out, test)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("list kecermatan tests: %w", err)
	}
	return out, total, nil
}

// Find answers ErrNotFound for another tenant's id exactly as for an unknown one.
func (r *KecermatanRepository) Find(ctx context.Context, q queryer, tenantID, id uuid.UUID) (model.KecermatanTest, error) {
	return r.find(ctx, q, tenantID, id, "")
}

// LockTx takes the test row for the rest of the transaction; P18's admission must take the same lock, so an
// edit and a first attempt cannot both win.
func (r *KecermatanRepository) LockTx(ctx context.Context, tx pgx.Tx, tenantID, id uuid.UUID) (model.KecermatanTest, error) {
	return r.find(ctx, tx, tenantID, id, " FOR UPDATE OF t")
}

func (r *KecermatanRepository) find(ctx context.Context, q queryer, tenantID, id uuid.UUID, lock string) (model.KecermatanTest, error) {
	if tenantID == uuid.Nil {
		return model.KecermatanTest{}, ErrScopeRequired
	}
	test, err := scanKecermatan(q.QueryRow(ctx,
		`SELECT `+kecermatanColumns+` FROM kecermatan_tests t WHERE t.tenant_id = $1 AND t.id = $2`+lock, tenantID, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return model.KecermatanTest{}, ErrNotFound
	}
	if err != nil {
		return model.KecermatanTest{}, fmt.Errorf("find kecermatan test: %w", err)
	}
	rows, err := q.Query(ctx, `
		SELECT k.package_id, p.name, p.type, k.revision, r.element_count, k.required_count
		  FROM kecermatan_test_packages k
		  JOIN item_packages p ON p.id = k.package_id
		  JOIN item_package_revisions r ON r.package_id = k.package_id AND r.revision = k.revision
		 WHERE k.tenant_id = $1 AND k.test_id = $2
		 ORDER BY k.package_id`, tenantID, id)
	if err != nil {
		return model.KecermatanTest{}, fmt.Errorf("list kecermatan packages: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var ref model.KecermatanPackageRef
		if err := rows.Scan(&ref.PackageID, &ref.Name, &ref.Type, &ref.Revision, &ref.ElementCount, &ref.RequiredCount); err != nil {
			return model.KecermatanTest{}, fmt.Errorf("scan kecermatan package: %w", err)
		}
		test.Packages = append(test.Packages, ref)
	}
	return test, rows.Err()
}

func (r *KecermatanRepository) ProgramExists(ctx context.Context, tenantID, programID uuid.UUID) (bool, error) {
	if tenantID == uuid.Nil {
		return false, ErrScopeRequired
	}
	var ok bool
	if err := r.db.QueryRow(ctx,
		`SELECT EXISTS (SELECT 1 FROM programs WHERE tenant_id = $1 AND id = $2)`,
		tenantID, programID).Scan(&ok); err != nil {
		return false, fmt.Errorf("check kecermatan program: %w", err)
	}
	return ok, nil
}

func (r *KecermatanRepository) CreateTx(ctx context.Context, tx pgx.Tx, tenantID, createdBy uuid.UUID, t model.KecermatanTest) (uuid.UUID, error) {
	if tenantID == uuid.Nil {
		return uuid.Nil, ErrScopeRequired
	}
	var id uuid.UUID
	err := tx.QueryRow(ctx, `
		INSERT INTO kecermatan_tests (tenant_id, program_id, title, description, item_type, item_length,
		        options_per_soal, soal_per_kolom, kolom_count, kolom_duration_seconds, builtin_sources,
		        cover_from, cover_to, generation_seed, created_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15)
		RETURNING id`,
		tenantID, t.ProgramID, t.Title, t.Description, t.ItemType, t.ItemLength, t.OptionsPerSoal,
		t.SoalPerKolom, t.KolomCount, t.KolomDurationSeconds, nonNilStrings(t.BuiltinSources),
		t.CoverFrom, t.CoverTo, t.Seed, createdBy).Scan(&id)
	return id, kecermatanError("create kecermatan test", err)
}

// UpdateTx writes the editable fields and bumps the revision; the seed is never rewritten.
func (r *KecermatanRepository) UpdateTx(ctx context.Context, tx pgx.Tx, tenantID, id uuid.UUID, t model.KecermatanTest) error {
	if tenantID == uuid.Nil {
		return ErrScopeRequired
	}
	tag, err := tx.Exec(ctx, `
		UPDATE kecermatan_tests
		   SET program_id = $3, title = $4, description = $5, item_type = $6, item_length = $7,
		       options_per_soal = $8, soal_per_kolom = $9, kolom_count = $10, kolom_duration_seconds = $11,
		       builtin_sources = $12, cover_from = $13, cover_to = $14, revision = revision + 1, updated_at = now()
		 WHERE tenant_id = $1 AND id = $2`,
		tenantID, id, t.ProgramID, t.Title, t.Description, t.ItemType, t.ItemLength, t.OptionsPerSoal,
		t.SoalPerKolom, t.KolomCount, t.KolomDurationSeconds, nonNilStrings(t.BuiltinSources), t.CoverFrom, t.CoverTo)
	if err != nil {
		return kecermatanError("update kecermatan test", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// ReplacePinsTx swaps the test's package pins. The caller already holds the test row and every package row.
func (r *KecermatanRepository) ReplacePinsTx(ctx context.Context, tx pgx.Tx, tenantID, id uuid.UUID, pins []KecermatanPin) error {
	if tenantID == uuid.Nil {
		return ErrScopeRequired
	}
	if err := r.deletePinsTx(ctx, tx, tenantID, id); err != nil {
		return err
	}
	for _, pin := range pins {
		if _, err := tx.Exec(ctx, `
			INSERT INTO kecermatan_test_packages (test_id, tenant_id, package_id, revision, required_count)
			VALUES ($1, $2, $3, $4, $5)`, id, tenantID, pin.PackageID, pin.Revision, pin.RequiredCount); err != nil {
			return kecermatanError("pin kecermatan package", err)
		}
	}
	return nil
}

func (r *KecermatanRepository) deletePinsTx(ctx context.Context, tx pgx.Tx, tenantID, id uuid.UUID) error {
	if _, err := tx.Exec(ctx,
		`DELETE FROM kecermatan_test_packages WHERE tenant_id = $1 AND test_id = $2`, tenantID, id); err != nil {
		return kecermatanError("unpin kecermatan packages", err)
	}
	return nil
}

func (r *KecermatanRepository) DeleteTx(ctx context.Context, tx pgx.Tx, tenantID, id uuid.UUID) error {
	if tenantID == uuid.Nil {
		return ErrScopeRequired
	}
	if err := r.deletePinsTx(ctx, tx, tenantID, id); err != nil {
		return err
	}
	tag, err := tx.Exec(ctx, `DELETE FROM kecermatan_tests WHERE tenant_id = $1 AND id = $2`, tenantID, id)
	if err != nil {
		return kecermatanError("delete kecermatan test", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func nonNilStrings(in []string) []string {
	if in == nil {
		return []string{}
	}
	return in
}
