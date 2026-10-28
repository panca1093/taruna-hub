package handler

import (
	"errors"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"

	"github.com/tarunaeduka/bimbel-tryout/api/internal/httpx"
	"github.com/tarunaeduka/bimbel-tryout/api/internal/model"
	"github.com/tarunaeduka/bimbel-tryout/api/internal/service"
)

const (
	violationLimit   = "Batas pencatatan pelanggaran untuk sesi ini sudah tercapai."
	violationTypeMsg = "Jenis pelanggaran tidak dikenal."
	violationEvent   = "ID kejadian wajib berupa UUID."
	monitoringPage   = "Parameter halaman tidak valid (limit 1–200, offset 0 atau lebih)."
)

const maxViolationBody = 1 << 10

type violationRequest struct {
	EventID   uuid.UUID  `json:"event_id"`
	Type      string     `json:"type"`
	SubtestID *uuid.UUID `json:"subtest_id"`
}

type violationResponse struct {
	ID          uuid.UUID `json:"id"`
	SessionID   uuid.UUID `json:"session_id"`
	SubtestID   uuid.UUID `json:"subtest_id"`
	SubtestName string    `json:"subtest_name"`
	Type        string    `json:"type"`
	OccurredAt  time.Time `json:"occurred_at"`
}

type activeSubtestResponse struct {
	SubtestID uuid.UUID `json:"subtest_id"`
	Name      string    `json:"name"`
	Position  int       `json:"position"`
	StartedAt time.Time `json:"started_at"`
}

type activeSessionResponse struct {
	SessionID      uuid.UUID              `json:"session_id"`
	UserID         uuid.UUID              `json:"user_id"`
	StudentName    string                 `json:"student_name"`
	TryoutID       uuid.UUID              `json:"tryout_id"`
	TryoutTitle    string                 `json:"tryout_title"`
	AttemptNumber  int                    `json:"attempt_number"`
	StartedAt      time.Time              `json:"started_at"`
	Subtest        *activeSubtestResponse `json:"subtest"`
	ViolationCount int                    `json:"violation_count"`
}

type activeSessionListResponse struct {
	Sessions   []activeSessionResponse `json:"sessions"`
	Total      int                     `json:"total"`
	ServerTime time.Time               `json:"server_time"`
}

type violationTimelineResponse struct {
	SessionID     uuid.UUID           `json:"session_id"`
	UserID        uuid.UUID           `json:"user_id"`
	StudentName   string              `json:"student_name"`
	TryoutID      uuid.UUID           `json:"tryout_id"`
	TryoutTitle   string              `json:"tryout_title"`
	AttemptNumber int                 `json:"attempt_number"`
	Status        string              `json:"status"`
	StartedAt     time.Time           `json:"started_at"`
	CompletedAt   *time.Time          `json:"completed_at"`
	Total         int                 `json:"total"`
	Violations    []violationResponse `json:"violations"`
}

func violationOf(v model.Violation) violationResponse {
	return violationResponse{ID: v.ID, SessionID: v.SessionID, SubtestID: v.SubtestID, SubtestName: v.SubtestName, Type: v.Type, OccurredAt: v.OccurredAt}
}

func (h *Handler) ReportViolation(c echo.Context) error {
	scope, principal, ok := runtimeCaller(c)
	if !ok {
		return httpx.Unauthorized(sessionInvalid)
	}
	id, err := runtimeID(c, "id")
	if err != nil {
		return err
	}
	c.Request().Body = http.MaxBytesReader(c.Response(), c.Request().Body, maxViolationBody)
	var req violationRequest
	if err := bindStrict(c, &req); err != nil {
		return err
	}
	v, created, err := h.monitoring.RecordViolation(c.Request().Context(), scope, principal, id,
		service.ViolationInput{EventID: req.EventID, Type: req.Type, SubtestID: req.SubtestID})
	if err != nil {
		return monitoringError(err)
	}
	status := http.StatusOK
	if created {
		status = http.StatusCreated
	}
	return c.JSON(status, violationOf(v))
}

func (h *Handler) ActiveSessions(c echo.Context) error {
	scope, principal, ok := runtimeCaller(c)
	if !ok {
		return httpx.Unauthorized(sessionInvalid)
	}
	limit, offset, err := orderPageOf(c)
	if err != nil {
		return err
	}
	rows, total, err := h.monitoring.ActiveSessions(c.Request().Context(), scope, principal, limit, offset)
	if err != nil {
		return monitoringError(err)
	}
	body := activeSessionListResponse{Sessions: make([]activeSessionResponse, 0, len(rows)), Total: total, ServerTime: time.Now().UTC()}
	for _, r := range rows {
		item := activeSessionResponse{
			SessionID: r.SessionID, UserID: r.UserID, StudentName: r.StudentName, TryoutID: r.TryoutID, TryoutTitle: r.TryoutTitle,
			AttemptNumber: r.AttemptNumber, StartedAt: r.StartedAt, ViolationCount: r.ViolationCount,
		}
		if r.Subtest != nil {
			item.Subtest = &activeSubtestResponse{SubtestID: r.Subtest.ID, Name: r.Subtest.Name, Position: r.Subtest.Position, StartedAt: r.Subtest.StartedAt}
		}
		body.Sessions = append(body.Sessions, item)
	}
	return c.JSON(http.StatusOK, body)
}

func (h *Handler) SessionViolations(c echo.Context) error {
	scope, principal, ok := runtimeCaller(c)
	if !ok {
		return httpx.Unauthorized(sessionInvalid)
	}
	id, err := runtimeID(c, "id")
	if err != nil {
		return err
	}
	info, list, err := h.monitoring.Timeline(c.Request().Context(), scope, principal, id)
	if err != nil {
		return monitoringError(err)
	}
	body := violationTimelineResponse{
		SessionID: info.SessionID, UserID: info.UserID, StudentName: info.StudentName, TryoutID: info.TryoutID,
		TryoutTitle: info.TryoutTitle, AttemptNumber: info.AttemptNumber, Status: info.Status, StartedAt: info.StartedAt,
		CompletedAt: info.CompletedAt, Total: len(list), Violations: make([]violationResponse, 0, len(list)),
	}
	for _, v := range list {
		body.Violations = append(body.Violations, violationOf(v))
	}
	return c.JSON(http.StatusOK, body)
}

func monitoringError(err error) error {
	var input *service.InputError
	if errors.As(err, &input) {
		fields := make(map[string]string, len(input.Fields))
		for path, reason := range input.Fields {
			switch reason {
			case service.ReasonViolationType:
				fields[path] = violationTypeMsg
			case service.ReasonViolationEvent:
				fields[path] = violationEvent
			case service.ReasonMonitoringPage:
				fields[path] = monitoringPage
			default:
				fields[path] = invalidFormat
			}
		}
		return httpx.Validation(incomplete, fields)
	}
	if errors.Is(err, service.ErrViolationLimit) {
		return httpx.TooManyRequests(violationLimit)
	}
	return runtimeError(err)
}
