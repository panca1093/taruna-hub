package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/tarunaeduka/bimbel-tryout/api/internal/model"
)

const exportColumns = `id, tenant_id, kind, params, status, requested_by, cut_off_at, object_key, content_type,
	file_name, size_bytes, error_code, attempts, created_at, updated_at, finished_at`

type ExportRepository struct {
	db *pgxpool.Pool
}

func NewExportRepository(db *pgxpool.Pool) *ExportRepository {
	return &ExportRepository{db: db}
}

func scanExport(row pgx.Row) (model.Export, error) {
	var e model.Export
	err := row.Scan(&e.ID, &e.TenantID, &e.Kind, &e.Params, &e.Status, &e.RequestedBy, &e.CutOffAt, &e.ObjectKey,
		&e.ContentType, &e.FileName, &e.SizeBytes, &e.ErrorCode, &e.Attempts, &e.CreatedAt, &e.UpdatedAt, &e.FinishedAt)
	return e, err
}

// Insert stamps the cut-off with the database clock, so it is the instant the row became visible to the job.
func (r *ExportRepository) Insert(ctx context.Context, tx pgx.Tx, e model.Export) (model.Export, error) {
	if e.TenantID == uuid.Nil {
		return model.Export{}, ErrScopeRequired
	}
	created, err := scanExport(tx.QueryRow(ctx, `
		INSERT INTO exports (tenant_id, kind, params, status, requested_by, cut_off_at)
		VALUES ($1, $2, $3, 'queued', $4, now())
		RETURNING `+exportColumns,
		e.TenantID, e.Kind, e.Params, e.RequestedBy))
	if err != nil {
		return model.Export{}, fmt.Errorf("insert export: %w", err)
	}
	return created, nil
}

// Find answers ErrNotFound for another tenant's id exactly as for an unknown one.
func (r *ExportRepository) Find(ctx context.Context, tenantID, id uuid.UUID) (model.Export, error) {
	if tenantID == uuid.Nil {
		return model.Export{}, ErrScopeRequired
	}
	e, err := scanExport(r.db.QueryRow(ctx,
		`SELECT `+exportColumns+` FROM exports WHERE tenant_id = $1 AND id = $2`, tenantID, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return model.Export{}, ErrNotFound
	}
	if err != nil {
		return model.Export{}, fmt.Errorf("find export: %w", err)
	}
	return e, nil
}

// List is newest first; an empty kind means every kind.
func (r *ExportRepository) List(ctx context.Context, tenantID uuid.UUID, kind string, limit int) ([]model.Export, error) {
	if tenantID == uuid.Nil {
		return nil, ErrScopeRequired
	}
	rows, err := r.db.Query(ctx, `
		SELECT `+exportColumns+` FROM exports
		 WHERE tenant_id = $1 AND ($2 = '' OR kind = $2)
		 ORDER BY created_at DESC, id DESC
		 LIMIT $3`, tenantID, kind, limit)
	if err != nil {
		return nil, fmt.Errorf("list exports: %w", err)
	}
	defer rows.Close()
	out := []model.Export{}
	for rows.Next() {
		e, err := scanExport(rows)
		if err != nil {
			return nil, fmt.Errorf("scan export: %w", err)
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// Claim moves an unfinished export to running and counts the attempt; a finished or unknown one is ErrNotFound.
func (r *ExportRepository) Claim(ctx context.Context, tenantID, id uuid.UUID) (model.Export, error) {
	if tenantID == uuid.Nil {
		return model.Export{}, ErrScopeRequired
	}
	e, err := scanExport(r.db.QueryRow(ctx, `
		UPDATE exports SET status = 'running', attempts = attempts + 1, updated_at = now()
		 WHERE tenant_id = $1 AND id = $2 AND status IN ('queued', 'running')
		RETURNING `+exportColumns, tenantID, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return model.Export{}, ErrNotFound
	}
	if err != nil {
		return model.Export{}, fmt.Errorf("claim export: %w", err)
	}
	return e, nil
}

// MarkReady runs in the caller's transaction so the ready state and its notification commit together.
func (r *ExportRepository) MarkReady(ctx context.Context, tx pgx.Tx, tenantID, id uuid.UUID, key, contentType, fileName string, size int64) (model.Export, error) {
	e, err := scanExport(tx.QueryRow(ctx, `
		UPDATE exports SET status = 'ready', object_key = $3, content_type = $4, file_name = $5, size_bytes = $6,
		       error_code = '', finished_at = now(), updated_at = now()
		 WHERE tenant_id = $1 AND id = $2 AND status = 'running'
		RETURNING `+exportColumns, tenantID, id, key, contentType, fileName, size))
	if errors.Is(err, pgx.ErrNoRows) {
		return model.Export{}, ErrNotFound
	}
	if err != nil {
		return model.Export{}, fmt.Errorf("mark export ready: %w", err)
	}
	return e, nil
}

// MarkAttemptFailed records the code and either returns the row to the queue for River's retry or ends it.
func (r *ExportRepository) MarkAttemptFailed(ctx context.Context, tenantID, id uuid.UUID, code string, final bool) error {
	status := model.ExportQueued
	if final {
		status = model.ExportFailed
	}
	_, err := r.db.Exec(ctx, `
		UPDATE exports SET status = $3, error_code = $4,
		       finished_at = CASE WHEN $3 = 'failed' THEN now() ELSE NULL END, updated_at = now()
		 WHERE tenant_id = $1 AND id = $2 AND status = 'running'`, tenantID, id, status, code)
	if err != nil {
		return fmt.Errorf("mark export attempt failed: %w", err)
	}
	return nil
}

// FailStale is cross-tenant on purpose: it is the worker's own sweep, never reachable from a request. Queued rows are
// left alone: River still holds their job, and a long outage or backlog must not fail exports that never started.
func (r *ExportRepository) FailStale(ctx context.Context, code string, olderThan time.Duration) (int, error) {
	tag, err := r.db.Exec(ctx, `
		UPDATE exports SET status = 'failed', error_code = $1, finished_at = now(), updated_at = now()
		 WHERE status = 'running' AND updated_at < now() - make_interval(secs => $2)`,
		code, olderThan.Seconds())
	if err != nil {
		return 0, fmt.Errorf("fail stale exports: %w", err)
	}
	return int(tag.RowsAffected()), nil
}

// Requester returns what the ready notice needs; email is nil when the account has none.
func (r *ExportRepository) Requester(ctx context.Context, tenantID, userID uuid.UUID) (email *string, tenantName string, err error) {
	err = r.db.QueryRow(ctx, `
		SELECT u.email, t.name FROM users u JOIN tenants t ON t.id = u.tenant_id
		 WHERE u.tenant_id = $1 AND u.id = $2`, tenantID, userID).Scan(&email, &tenantName)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, "", ErrNotFound
	}
	if err != nil {
		return nil, "", fmt.Errorf("find export requester: %w", err)
	}
	return email, tenantName, nil
}
