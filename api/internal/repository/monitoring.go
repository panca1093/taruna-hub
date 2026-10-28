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

// MonitoringRepository persists violations and reads the staff views. The recording rules (ownership, running
// subtest, the per-session cap) live in the service.
type MonitoringRepository struct {
	db *pgxpool.Pool
}

func NewMonitoringRepository(db *pgxpool.Pool) *MonitoringRepository {
	return &MonitoringRepository{db: db}
}

const violationColumns = `v.id, v.session_id, v.subtest_id, p.name, v.type, v.event_id, v.occurred_at`

const violationFrom = `
	  FROM tryout_session_violations v
	  JOIN tryout_session_parts p ON p.session_id = v.session_id AND p.subtest_id = v.subtest_id`

func scanViolation(row pgx.Row) (model.Violation, error) {
	var v model.Violation
	err := row.Scan(&v.ID, &v.SessionID, &v.SubtestID, &v.SubtestName, &v.Type, &v.EventID, &v.OccurredAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return model.Violation{}, ErrNotFound
	}
	if err != nil {
		return model.Violation{}, fmt.Errorf("scan violation: %w", err)
	}
	return v, nil
}

// ViolationByEvent finds the row a browser retry already created.
func (r *MonitoringRepository) ViolationByEvent(ctx context.Context, q dbtx, tenantID, sessionID, eventID uuid.UUID) (model.Violation, error) {
	if tenantID == uuid.Nil {
		return model.Violation{}, ErrScopeRequired
	}
	return scanViolation(q.QueryRow(ctx, `SELECT `+violationColumns+violationFrom+`
		 WHERE v.tenant_id = $1 AND v.session_id = $2 AND v.event_id = $3`, tenantID, sessionID, eventID))
}

func (r *MonitoringRepository) CountViolations(ctx context.Context, q dbtx, tenantID, sessionID uuid.UUID) (int, error) {
	if tenantID == uuid.Nil {
		return 0, ErrScopeRequired
	}
	var n int
	if err := q.QueryRow(ctx, `SELECT count(*) FROM tryout_session_violations WHERE tenant_id = $1 AND session_id = $2`,
		tenantID, sessionID).Scan(&n); err != nil {
		return 0, fmt.Errorf("count violations: %w", err)
	}
	return n, nil
}

// InsertViolation appends one row and returns it. The caller holds the session row lock, so the unique event key
// cannot race the lookup that precedes this call.
func (r *MonitoringRepository) InsertViolation(ctx context.Context, q dbtx, tenantID uuid.UUID, v model.Violation, at time.Time) (model.Violation, error) {
	if tenantID == uuid.Nil {
		return model.Violation{}, ErrScopeRequired
	}
	if _, err := q.Exec(ctx, `
		INSERT INTO tryout_session_violations (tenant_id, session_id, subtest_id, type, event_id, occurred_at)
		VALUES ($1, $2, $3, $4, $5, $6)`, tenantID, v.SessionID, v.SubtestID, v.Type, v.EventID, at); err != nil {
		return model.Violation{}, fmt.Errorf("insert violation: %w", err)
	}
	return r.ViolationByEvent(ctx, q, tenantID, v.SessionID, v.EventID)
}

// ActiveSessions lists the in-progress sessions of one tenant, newest first, with the running subtest and the
// violation count. total counts every in-progress session, not just the page.
func (r *MonitoringRepository) ActiveSessions(ctx context.Context, q dbtx, tenantID uuid.UUID, limit, offset int) ([]model.ActiveSession, int, error) {
	if tenantID == uuid.Nil {
		return nil, 0, ErrScopeRequired
	}
	var total int
	if err := q.QueryRow(ctx, `SELECT count(*) FROM tryout_sessions WHERE tenant_id = $1 AND status = 'in_progress'`,
		tenantID).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count active sessions: %w", err)
	}
	rows, err := q.Query(ctx, `
		SELECT s.id, s.user_id, u.name, s.tryout_id, t.title, s.attempt_number, s.started_at,
		       p.subtest_id, p.name, p.position, p.started_at,
		       (SELECT count(*) FROM tryout_session_violations v WHERE v.tenant_id = s.tenant_id AND v.session_id = s.id)
		  FROM tryout_sessions s
		  JOIN users u ON u.id = s.user_id AND u.tenant_id = s.tenant_id
		  JOIN tryouts t ON t.id = s.tryout_id AND t.tenant_id = s.tenant_id
		  LEFT JOIN tryout_session_parts p ON p.session_id = s.id AND p.tenant_id = s.tenant_id AND p.status = 'running'
		 WHERE s.tenant_id = $1 AND s.status = 'in_progress'
		 ORDER BY s.started_at DESC, s.id
		 LIMIT $2 OFFSET $3`, tenantID, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("list active sessions: %w", err)
	}
	defer rows.Close()
	out := []model.ActiveSession{}
	for rows.Next() {
		var a model.ActiveSession
		var subtestID *uuid.UUID
		var name *string
		var position *int
		var partStarted *time.Time
		if err := rows.Scan(&a.SessionID, &a.UserID, &a.StudentName, &a.TryoutID, &a.TryoutTitle, &a.AttemptNumber,
			&a.StartedAt, &subtestID, &name, &position, &partStarted, &a.ViolationCount); err != nil {
			return nil, 0, fmt.Errorf("scan active session: %w", err)
		}
		if subtestID != nil {
			a.Subtest = &model.ActiveSubtest{ID: *subtestID, Name: *name, Position: *position, StartedAt: *partStarted}
		}
		out = append(out, a)
	}
	return out, total, rows.Err()
}

// SessionInfo reads one session of any status in the tenant; another tenant's id is simply not found.
func (r *MonitoringRepository) SessionInfo(ctx context.Context, q dbtx, tenantID, sessionID uuid.UUID) (model.SessionInfo, error) {
	if tenantID == uuid.Nil {
		return model.SessionInfo{}, ErrScopeRequired
	}
	var s model.SessionInfo
	err := q.QueryRow(ctx, `
		SELECT s.id, s.user_id, u.name, s.tryout_id, t.title, s.attempt_number, s.status, s.started_at, s.completed_at
		  FROM tryout_sessions s
		  JOIN users u ON u.id = s.user_id AND u.tenant_id = s.tenant_id
		  JOIN tryouts t ON t.id = s.tryout_id AND t.tenant_id = s.tenant_id
		 WHERE s.tenant_id = $1 AND s.id = $2`, tenantID, sessionID).
		Scan(&s.SessionID, &s.UserID, &s.StudentName, &s.TryoutID, &s.TryoutTitle, &s.AttemptNumber, &s.Status, &s.StartedAt, &s.CompletedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return model.SessionInfo{}, ErrNotFound
	}
	if err != nil {
		return model.SessionInfo{}, fmt.Errorf("read session info: %w", err)
	}
	return s, nil
}

// Violations is a session's timeline, oldest first.
func (r *MonitoringRepository) Violations(ctx context.Context, q dbtx, tenantID, sessionID uuid.UUID) ([]model.Violation, error) {
	if tenantID == uuid.Nil {
		return nil, ErrScopeRequired
	}
	rows, err := q.Query(ctx, `SELECT `+violationColumns+violationFrom+`
		 WHERE v.tenant_id = $1 AND v.session_id = $2
		 ORDER BY v.occurred_at, v.seq`, tenantID, sessionID)
	if err != nil {
		return nil, fmt.Errorf("list violations: %w", err)
	}
	defer rows.Close()
	out := []model.Violation{}
	for rows.Next() {
		v, err := scanViolation(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}
