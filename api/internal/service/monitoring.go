package service

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/tarunaeduka/bimbel-tryout/api/internal/model"
	"github.com/tarunaeduka/bimbel-tryout/api/internal/repository"
)

var ErrViolationLimit = errors.New("session violation limit reached")

const (
	ReasonViolationType  = "violation_type"
	ReasonViolationEvent = "violation_event_id"
	ReasonMonitoringPage = "monitoring_query"

	// MaxViolationsPerSession bounds the rows one session can hold, so a flooding client cannot grow the table.
	MaxViolationsPerSession = 100

	defaultMonitoringLimit = 100
	maxMonitoringLimit     = 200
)

// MonitoringService records violation evidence and serves the staff views of active sessions and timelines. It
// shares the runtime's transaction helper and clock, so a violation sees exactly the deadlines the exam does.
type MonitoringService struct {
	rt   *RuntimeService
	repo *repository.MonitoringRepository
}

func NewMonitoringService(rt *RuntimeService, repo *repository.MonitoringRepository) *MonitoringService {
	return &MonitoringService{rt: rt, repo: repo}
}

type ViolationInput struct {
	EventID uuid.UUID
	Type    string
	// SubtestID is the subtest the browser saw when it happened, so a late retry keeps its original subtest. An id
	// that is not a subtest of this session is ignored and the running subtest is used.
	SubtestID *uuid.UUID
}

// RecordViolation stores one violation of the student's own session, stamped with the server clock. It is accepted
// only while a subtest runs. A resend of a stored event returns the stored row with created=false, even at the cap.
func (s *MonitoringService) RecordViolation(ctx context.Context, scope HostScope, principal model.Principal, sessionID uuid.UUID, in ViolationInput) (model.Violation, bool, error) {
	tenantID, err := studentOf(scope, principal)
	if err != nil {
		return model.Violation{}, false, err
	}
	fields := map[string]string{}
	if in.Type != model.ViolationExitFullscreen && in.Type != model.ViolationScreenshotAttempt {
		fields["type"] = ReasonViolationType
	}
	if in.EventID == uuid.Nil {
		fields["event_id"] = ReasonViolationEvent
	}
	if len(fields) > 0 {
		return model.Violation{}, false, &InputError{Fields: fields}
	}

	var out model.Violation
	created := false
	err = s.rt.run(ctx, func(tx pgx.Tx) error {
		a, err := s.rt.repo.FindAttempt(ctx, tx, tenantID, principal.UserID, sessionID, true)
		if err != nil {
			return runtimeError(err)
		}
		now := s.rt.now()
		if _, err := s.rt.settleLocked(ctx, tx, a, now); err != nil {
			return err
		}
		if a, err = s.rt.repo.FindAttempt(ctx, tx, tenantID, principal.UserID, sessionID, true); err != nil {
			return runtimeError(err)
		}
		if a.Status != model.SessionInProgress {
			return keep{ErrSessionClosed}
		}
		parts, err := s.rt.repo.Parts(ctx, tx, tenantID, sessionID)
		if err != nil {
			return runtimeError(err)
		}
		var running *model.AttemptPart
		for i := range parts {
			if parts[i].Status == model.PartRunning {
				running = &parts[i]
			}
		}
		if running == nil {
			return keep{ErrPartNotRunning}
		}

		stored, err := s.repo.ViolationByEvent(ctx, tx, tenantID, sessionID, in.EventID)
		switch {
		case err == nil:
			out = stored
			return nil
		case !errors.Is(err, repository.ErrNotFound):
			return runtimeError(err)
		}
		n, err := s.repo.CountViolations(ctx, tx, tenantID, sessionID)
		if err != nil {
			return runtimeError(err)
		}
		if n >= MaxViolationsPerSession {
			return ErrViolationLimit
		}
		subtestID := running.SubtestID
		if in.SubtestID != nil {
			for _, p := range parts {
				if p.SubtestID == *in.SubtestID {
					subtestID = p.SubtestID
				}
			}
		}
		out, err = s.repo.InsertViolation(ctx, tx, tenantID, model.Violation{
			SessionID: sessionID, SubtestID: subtestID, Type: in.Type, EventID: in.EventID,
		}, now)
		if err != nil {
			return runtimeError(err)
		}
		created = true
		return nil
	})
	return out, created, err
}

// ActiveSessions lists the tenant's in-progress sessions for Admin and Operator, straight from PostgreSQL.
func (s *MonitoringService) ActiveSessions(ctx context.Context, scope HostScope, principal model.Principal, limit, offset int) ([]model.ActiveSession, int, error) {
	tenantID, err := staffTenant(scope, principal)
	if err != nil {
		return nil, 0, err
	}
	if limit == 0 {
		limit = defaultMonitoringLimit
	}
	if limit < 1 || limit > maxMonitoringLimit || offset < 0 {
		fields := map[string]string{}
		if limit < 1 || limit > maxMonitoringLimit {
			fields["limit"] = ReasonMonitoringPage
		}
		if offset < 0 {
			fields["offset"] = ReasonMonitoringPage
		}
		return nil, 0, &InputError{Fields: fields}
	}
	rows, total, err := s.repo.ActiveSessions(ctx, s.rt.db, tenantID, limit, offset)
	return rows, total, runtimeError(err)
}

// Timeline is one session's violations for Admin and Operator, whether the session is running or finished.
func (s *MonitoringService) Timeline(ctx context.Context, scope HostScope, principal model.Principal, sessionID uuid.UUID) (model.SessionInfo, []model.Violation, error) {
	tenantID, err := staffTenant(scope, principal)
	if err != nil {
		return model.SessionInfo{}, nil, err
	}
	info, err := s.repo.SessionInfo(ctx, s.rt.db, tenantID, sessionID)
	if err != nil {
		return model.SessionInfo{}, nil, runtimeError(err)
	}
	list, err := s.repo.Violations(ctx, s.rt.db, tenantID, sessionID)
	return info, list, runtimeError(err)
}
