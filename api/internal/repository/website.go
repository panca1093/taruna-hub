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

// WebsiteRow is the stored state of one tenant's website; the service decides what any of it means.
type WebsiteRow struct {
	Draft             model.WebsiteContent
	DraftRevision     int64
	Published         *model.WebsiteContent
	PublishedRevision int64
	UpdatedAt         time.Time
	UpdatedBy         *uuid.UUID
	PublishedAt       *time.Time
	PublishedBy       *uuid.UUID
}

type WebsiteRepository struct {
	db *pgxpool.Pool
}

func NewWebsiteRepository(db *pgxpool.Pool) *WebsiteRepository {
	return &WebsiteRepository{db: db}
}

const websiteColumns = `draft, draft_revision, published, published_revision, updated_at, updated_by, published_at, published_by`

const publicationColumns = `published_revision, source_draft_revision, published_at, published_by`

func (r *WebsiteRepository) Find(ctx context.Context, tenantID uuid.UUID) (WebsiteRow, error) {
	return r.find(ctx, r.db, tenantID, false)
}

func (r *WebsiteRepository) FindTx(ctx context.Context, tx pgx.Tx, tenantID uuid.UUID) (WebsiteRow, error) {
	return r.find(ctx, tx, tenantID, false)
}

// LockTx takes the tenant's row, so concurrent saves and publishes queue behind each other.
func (r *WebsiteRepository) LockTx(ctx context.Context, tx pgx.Tx, tenantID uuid.UUID) (WebsiteRow, error) {
	return r.find(ctx, tx, tenantID, true)
}

func (r *WebsiteRepository) find(ctx context.Context, q queryer, tenantID uuid.UUID, lock bool) (WebsiteRow, error) {
	if tenantID == uuid.Nil {
		return WebsiteRow{}, ErrScopeRequired
	}
	sql := `SELECT ` + websiteColumns + ` FROM website_content WHERE tenant_id = $1`
	if lock {
		sql += ` FOR UPDATE`
	}
	var (
		row       WebsiteRow
		draft     []byte
		published []byte
	)
	err := q.QueryRow(ctx, sql, tenantID).Scan(&draft, &row.DraftRevision, &published, &row.PublishedRevision,
		&row.UpdatedAt, &row.UpdatedBy, &row.PublishedAt, &row.PublishedBy)
	if errors.Is(err, pgx.ErrNoRows) {
		return WebsiteRow{}, ErrNotFound
	}
	if err != nil {
		return WebsiteRow{}, fmt.Errorf("read website content: %w", err)
	}
	if err := json.Unmarshal(draft, &row.Draft); err != nil {
		return WebsiteRow{}, fmt.Errorf("decode website draft: %w", err)
	}
	if published != nil {
		var content model.WebsiteContent
		if err := json.Unmarshal(published, &content); err != nil {
			return WebsiteRow{}, fmt.Errorf("decode website snapshot: %w", err)
		}
		row.Published = &content
	}
	return row, nil
}

// InitializeTx creates the row once; a concurrent first reader waits on the key and then inserts nothing.
func (r *WebsiteRepository) InitializeTx(ctx context.Context, tx pgx.Tx, tenantID uuid.UUID, draft model.WebsiteContent) error {
	if tenantID == uuid.Nil {
		return ErrScopeRequired
	}
	body, err := json.Marshal(draft)
	if err != nil {
		return fmt.Errorf("encode website draft: %w", err)
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO website_content (tenant_id, schema_version, draft, draft_revision)
		VALUES ($1, $2, $3, 1)
		ON CONFLICT (tenant_id) DO NOTHING`, tenantID, draft.SchemaVersion, body); err != nil {
		return fmt.Errorf("initialize website content: %w", err)
	}
	return nil
}

// SaveDraftTx stores the draft as the next revision; the caller holds the row lock and has compared revisions.
func (r *WebsiteRepository) SaveDraftTx(ctx context.Context, tx pgx.Tx, tenantID uuid.UUID, draft model.WebsiteContent, actor uuid.UUID) error {
	if tenantID == uuid.Nil {
		return ErrScopeRequired
	}
	body, err := json.Marshal(draft)
	if err != nil {
		return fmt.Errorf("encode website draft: %w", err)
	}
	if _, err := tx.Exec(ctx, `
		UPDATE website_content
		   SET draft = $2, draft_revision = draft_revision + 1, updated_at = now(), updated_by = $3
		 WHERE tenant_id = $1`, tenantID, body, actor); err != nil {
		return fmt.Errorf("save website draft: %w", err)
	}
	return nil
}

// PublishTx copies the whole draft into the published columns and appends the history record, nothing else.
func (r *WebsiteRepository) PublishTx(ctx context.Context, tx pgx.Tx, tenantID uuid.UUID, publishedRevision, sourceDraftRevision int64, actor uuid.UUID) (model.WebsitePublication, error) {
	if tenantID == uuid.Nil {
		return model.WebsitePublication{}, ErrScopeRequired
	}
	if _, err := tx.Exec(ctx, `
		UPDATE website_content
		   SET published = draft, published_revision = $2, published_at = now(), published_by = $3
		 WHERE tenant_id = $1`, tenantID, publishedRevision, actor); err != nil {
		return model.WebsitePublication{}, fmt.Errorf("copy website draft: %w", err)
	}
	var p model.WebsitePublication
	err := tx.QueryRow(ctx, `
		INSERT INTO website_publications (tenant_id, published_revision, source_draft_revision, published_by, published_at)
		SELECT tenant_id, $2, $3, $4, published_at FROM website_content WHERE tenant_id = $1
		RETURNING `+publicationColumns, tenantID, publishedRevision, sourceDraftRevision, actor).
		Scan(&p.PublishedRevision, &p.SourceDraftRevision, &p.PublishedAt, &p.PublishedBy)
	if err != nil {
		return model.WebsitePublication{}, fmt.Errorf("record website publication: %w", err)
	}
	p.Changed = true
	return p, nil
}

// PublicationBySourceTx finds the publication a draft revision already produced.
func (r *WebsiteRepository) PublicationBySourceTx(ctx context.Context, tx pgx.Tx, tenantID uuid.UUID, sourceDraftRevision int64) (model.WebsitePublication, error) {
	return r.publication(ctx, tx, tenantID, `source_draft_revision`, sourceDraftRevision)
}

func (r *WebsiteRepository) PublicationTx(ctx context.Context, tx pgx.Tx, tenantID uuid.UUID, publishedRevision int64) (model.WebsitePublication, error) {
	return r.publication(ctx, tx, tenantID, `published_revision`, publishedRevision)
}

func (r *WebsiteRepository) publication(ctx context.Context, q queryer, tenantID uuid.UUID, column string, revision int64) (model.WebsitePublication, error) {
	if tenantID == uuid.Nil {
		return model.WebsitePublication{}, ErrScopeRequired
	}
	var p model.WebsitePublication
	err := q.QueryRow(ctx, `SELECT `+publicationColumns+` FROM website_publications WHERE tenant_id = $1 AND `+column+` = $2`,
		tenantID, revision).Scan(&p.PublishedRevision, &p.SourceDraftRevision, &p.PublishedAt, &p.PublishedBy)
	if errors.Is(err, pgx.ErrNoRows) {
		return model.WebsitePublication{}, ErrNotFound
	}
	if err != nil {
		return model.WebsitePublication{}, fmt.Errorf("read website publication: %w", err)
	}
	p.Changed = true
	return p, nil
}

// Publications returns up to limit records older than beforeRevision (0 means from the newest), newest first.
func (r *WebsiteRepository) Publications(ctx context.Context, tenantID uuid.UUID, beforeRevision int64, limit int) ([]model.WebsitePublication, error) {
	if tenantID == uuid.Nil {
		return nil, ErrScopeRequired
	}
	rows, err := r.db.Query(ctx, `
		SELECT `+publicationColumns+` FROM website_publications
		 WHERE tenant_id = $1 AND ($2::bigint = 0 OR published_revision < $2::bigint)
		 ORDER BY published_revision DESC LIMIT $3`, tenantID, beforeRevision, limit)
	if err != nil {
		return nil, fmt.Errorf("list website publications: %w", err)
	}
	defer rows.Close()
	items := []model.WebsitePublication{}
	for rows.Next() {
		p := model.WebsitePublication{Changed: true}
		if err := rows.Scan(&p.PublishedRevision, &p.SourceDraftRevision, &p.PublishedAt, &p.PublishedBy); err != nil {
			return nil, fmt.Errorf("scan website publication: %w", err)
		}
		items = append(items, p)
	}
	return items, rows.Err()
}
