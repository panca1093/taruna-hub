package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/tarunaeduka/bimbel-tryout/api/internal/model"
)

// ErrBusy means another claim holds the row, or this caller's lease has already expired.
var ErrBusy = errors.New("notification is claimed by another attempt")

// ReconcileErrorText is the fixed, sanitized reason recorded for an orphaned queued row.
const ReconcileErrorText = "reconciled: delivery job ended without a result"

// ReconcileLimit bounds one maintenance scan.
const ReconcileLimit = 100

type NotificationRepository struct {
	db *pgxpool.Pool
}

func NewNotificationRepository(db *pgxpool.Pool) *NotificationRepository {
	return &NotificationRepository{db: db}
}

var columnList = []string{
	"id", "tenant_id", "event_key", "channel", "recipient", "template", "params", "sealed_params",
	"expires_at", "status", "attempts", "last_outcome", "last_error", "claim_token", "claimed_until",
	"created_at", "updated_at", "finished_at",
}

var notificationColumns = strings.Join(columnList, ", ")

// scopeClause compares the null tenant explicitly, so a platform row never matches a tenant and vice versa.
const scopeClause = `tenant_id IS NOT DISTINCT FROM $2::uuid`

// Create inserts in the caller's transaction. A suppressed duplicate returns uuid.Nil and inserts nothing.
func (r *NotificationRepository) Create(ctx context.Context, tx pgx.Tx, n model.Notification) (uuid.UUID, error) {
	params := n.Params
	if params == nil {
		params = map[string]string{}
	}
	var id uuid.UUID
	err := tx.QueryRow(ctx,
		`INSERT INTO notification_logs
			(id, tenant_id, event_key, channel, recipient, template, params, sealed_params, expires_at, status)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, 'queued')
		 ON CONFLICT ((coalesce(tenant_id, '00000000-0000-0000-0000-000000000000'::uuid)), event_key, recipient, channel)
		 DO NOTHING
		 RETURNING id`,
		n.ID, n.TenantID, n.EventKey, n.Channel, n.Recipient, n.Template, params, n.SealedParams, n.ExpiresAt,
	).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return uuid.Nil, nil
	}
	if err != nil {
		return uuid.Nil, fmt.Errorf("insert notification: %w", err)
	}
	return id, nil
}

// FindForDelivery is one of the two named unscoped reads. It takes a UUID only, so it cannot be
// asked about a tenant, a recipient or an event; it exists solely to discover the row's scope,
// which every later read and write in the delivery path must then pass.
func (r *NotificationRepository) FindForDelivery(ctx context.Context, id uuid.UUID) (model.Notification, error) {
	row := r.db.QueryRow(ctx, `SELECT `+notificationColumns+` FROM notification_logs WHERE id = $1`, id)
	return scanNotification(row)
}

// Find reads one row inside a scope.
func (r *NotificationRepository) Find(ctx context.Context, tenantID *uuid.UUID, id uuid.UUID) (model.Notification, error) {
	row := r.db.QueryRow(ctx,
		`SELECT `+notificationColumns+` FROM notification_logs WHERE id = $1 AND `+scopeClause, id, tenantID)
	return scanNotification(row)
}

// Claim takes the delivery lease and returns the row it now owns.
func (r *NotificationRepository) Claim(ctx context.Context, tenantID *uuid.UUID, id, token uuid.UUID, lease time.Duration) (model.Notification, error) {
	row := r.db.QueryRow(ctx,
		`UPDATE notification_logs
		    SET claim_token = $3, claimed_until = now() + make_interval(secs => $4), updated_at = now()
		  WHERE id = $1 AND `+scopeClause+`
		    AND status = 'queued'
		    AND (claimed_until IS NULL OR claimed_until < now())
		  RETURNING `+notificationColumns,
		id, tenantID, token, lease.Seconds())
	n, err := scanNotification(row)
	if errors.Is(err, ErrNotFound) {
		return model.Notification{}, ErrBusy
	}
	return n, err
}

// Finish writes a terminal state for the claim owner only; an expired lease no longer owns the row.
func (r *NotificationRepository) Finish(ctx context.Context, tenantID *uuid.UUID, id, token uuid.UUID, status model.NotificationStatus, attempts int, outcome model.Outcome, lastError *string) (bool, error) {
	tag, err := r.db.Exec(ctx,
		`UPDATE notification_logs
		    SET status = $4, attempts = $5, last_outcome = $6, last_error = $7,
		        finished_at = now(), sealed_params = NULL,
		        claim_token = NULL, claimed_until = NULL, updated_at = now()
		  WHERE id = $1 AND `+scopeClause+`
		    AND status = 'queued' AND claim_token = $3 AND claimed_until > now()`,
		id, tenantID, token, status, attempts, outcome, lastError)
	if err != nil {
		return false, fmt.Errorf("finish notification: %w", err)
	}
	return tag.RowsAffected() > 0, nil
}

// Release gives the row back for the next attempt: still queued, so still unfinished.
func (r *NotificationRepository) Release(ctx context.Context, tenantID *uuid.UUID, id, token uuid.UUID, attempts int, outcome model.Outcome, lastError *string) (bool, error) {
	tag, err := r.db.Exec(ctx,
		`UPDATE notification_logs
		    SET attempts = $4, last_outcome = $5, last_error = $6,
		        claim_token = NULL, claimed_until = NULL, updated_at = now()
		  WHERE id = $1 AND `+scopeClause+`
		    AND status = 'queued' AND claim_token = $3 AND claimed_until > now()`,
		id, tenantID, token, attempts, outcome, lastError)
	if err != nil {
		return false, fmt.Errorf("release notification claim: %w", err)
	}
	return tag.RowsAffected() > 0, nil
}

// ReconcileCandidates is the second named unscoped read: a bounded maintenance scan across scopes
// that selects queued rows whose River job is missing or already finished. It is reachable from no
// tenant-scoped repository method and from no service entry point a caller can supply a scope to.
func (r *NotificationRepository) ReconcileCandidates(ctx context.Context, cutoff time.Time, limit int) ([]model.Notification, error) {
	rows, err := r.db.Query(ctx,
		`SELECT `+prefixColumns("n")+`
		   FROM notification_logs n
		   LEFT JOIN river.river_job j
		     ON j.kind = 'notification' AND j.args->>'notification_id' = n.id::text
		  WHERE n.status = 'queued'
		    AND n.created_at < $1
		    AND (n.claimed_until IS NULL OR n.claimed_until < now())
		    AND (j.id IS NULL OR j.state IN ('discarded', 'cancelled', 'completed'))
		  ORDER BY n.created_at, n.id
		  LIMIT $2`, cutoff, limit)
	if err != nil {
		return nil, fmt.Errorf("scan reconcile candidates: %w", err)
	}
	defer rows.Close()

	var out []model.Notification
	for rows.Next() {
		n, err := scanNotification(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, n)
	}
	return out, rows.Err()
}

// FailOrphan is the scoped conditional write that retires one orphaned row. It rechecks state,
// liveness of the claim and job eligibility in the same statement, so a job revived between the
// scan and the write keeps the row queued and sealed.
func (r *NotificationRepository) FailOrphan(ctx context.Context, tenantID *uuid.UUID, id uuid.UUID, lastError string) (bool, error) {
	tag, err := r.db.Exec(ctx,
		`UPDATE notification_logs
		    SET status = 'failed', last_error = $3, finished_at = now(), sealed_params = NULL,
		        claim_token = NULL, claimed_until = NULL, updated_at = now()
		  WHERE id = $1 AND `+scopeClause+`
		    AND status = 'queued'
		    AND (claimed_until IS NULL OR claimed_until < now())
		    AND NOT EXISTS (
		          SELECT 1 FROM river.river_job j
		           WHERE j.kind = 'notification'
		             AND j.args->>'notification_id' = notification_logs.id::text
		             AND j.state NOT IN ('discarded', 'cancelled', 'completed'))`,
		id, tenantID, lastError)
	if err != nil {
		return false, fmt.Errorf("fail orphaned notification: %w", err)
	}
	return tag.RowsAffected() > 0, nil
}

func prefixColumns(alias string) string {
	out := ""
	for i, col := range columnList {
		if i > 0 {
			out += ", "
		}
		out += alias + "." + col
	}
	return out
}

func scanNotification(row scannable) (model.Notification, error) {
	var n model.Notification
	err := row.Scan(&n.ID, &n.TenantID, &n.EventKey, &n.Channel, &n.Recipient, &n.Template, &n.Params,
		&n.SealedParams, &n.ExpiresAt, &n.Status, &n.Attempts, &n.LastOutcome, &n.LastError,
		&n.ClaimToken, &n.ClaimedUntil, &n.CreatedAt, &n.UpdatedAt, &n.FinishedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return model.Notification{}, ErrNotFound
	}
	if err != nil {
		return model.Notification{}, fmt.Errorf("scan notification: %w", err)
	}
	return n, nil
}
