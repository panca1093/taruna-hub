package handler

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"

	"github.com/tarunaeduka/bimbel-tryout/api/internal/httpx"
	"github.com/tarunaeduka/bimbel-tryout/api/internal/model"
	"github.com/tarunaeduka/bimbel-tryout/api/internal/service"
)

const (
	tryoutNotFound    = "Tryout tidak ditemukan."
	tryoutConflict    = "Tryout sudah diubah orang lain, muat ulang."
	tryoutLocked      = "Tryout sudah dikerjakan siswa dan tidak bisa diubah."
	tryoutReferenced  = "Tryout masih dipakai konten lain."
	tryoutTitleRule   = "Judul 1–200 karakter."
	tryoutDescRule    = "Deskripsi maksimal 2000 karakter."
	tryoutScoreRule   = "Mode penilaian harus weighted atau IRT."
	tryoutProgram     = "Program tidak ditemukan."
	tryoutUntilRule   = "Tanggal berakhir wajib diisi."
	tryoutFromRule    = "Tanggal mulai harus sebelum tanggal berakhir."
	tryoutSubtestMax  = "Maksimal 30 subtes."
	tryoutQuestionMax = "Maksimal 200 soal per subtes dan 500 soal per tryout."
	tryoutNameRule    = "Nama subtes 1–200 karakter."
	tryoutDuration    = "Durasi 1–480 menit."
	tryoutQuestion    = "Soal tidak ditemukan."
	tryoutDuplicate   = "Soal yang sama tidak boleh muncul dua kali."
	tryoutVersion     = "Versi soal bukan milik tryout ini."
	tryoutEmpty       = "Tryout harus punya minimal satu subtes berisi soal."
	tryoutWindow      = "Tanggal berakhir harus setelah sekarang."
	tryoutRevision    = "Versi tryout wajib diisi."
	tryoutListLimit   = "Batas 1–100."
)

type assignmentResponse struct {
	QuestionID uuid.UUID `json:"question_id"`
	VersionID  uuid.UUID `json:"version_id"`
	Version    int       `json:"version"`
	Number     int64     `json:"number"`
	Type       string    `json:"type"`
	Difficulty string    `json:"difficulty"`
	Content    string    `json:"content"`
	Position   int       `json:"position"`
}

type subtestResponse struct {
	ID              uuid.UUID            `json:"id"`
	Name            string               `json:"name"`
	DurationSeconds int                  `json:"duration_seconds"`
	Position        int                  `json:"position"`
	Questions       []assignmentResponse `json:"questions"`
}

type tryoutSummaryResponse struct {
	ID                   uuid.UUID  `json:"id"`
	ProgramID            uuid.UUID  `json:"program_id"`
	Title                string     `json:"title"`
	Description          string     `json:"description"`
	ScoringMode          string     `json:"scoring_mode"`
	Status               string     `json:"status"`
	ActiveFrom           *time.Time `json:"active_from"`
	ActiveUntil          time.Time  `json:"active_until"`
	ExplanationReleaseAt *time.Time `json:"explanation_release_at"`
	RandomizeQuestions   bool       `json:"randomize_questions"`
	RandomizeOptions     bool       `json:"randomize_options"`
	Revision             int        `json:"revision"`
	Locked               bool       `json:"locked"`
	SubtestCount         int        `json:"subtest_count"`
	QuestionCount        int        `json:"question_count"`
	CreatedAt            time.Time  `json:"created_at"`
	UpdatedAt            time.Time  `json:"updated_at"`
}

type tryoutResponse struct {
	tryoutSummaryResponse
	Subtests []subtestResponse `json:"subtests"`
}

type tryoutsResponse struct {
	Tryouts []tryoutSummaryResponse `json:"tryouts"`
	Total   int                     `json:"total"`
}

type tryoutQuestionRequest struct {
	QuestionID string  `json:"question_id"`
	VersionID  *string `json:"version_id"`
}

type tryoutSubtestRequest struct {
	Name            string                  `json:"name"`
	DurationSeconds int                     `json:"duration_seconds"`
	Questions       []tryoutQuestionRequest `json:"questions"`
}

type createTryoutRequest struct {
	ProgramID            string                 `json:"program_id"`
	Title                string                 `json:"title"`
	Description          string                 `json:"description"`
	ScoringMode          string                 `json:"scoring_mode"`
	ActiveFrom           *string                `json:"active_from"`
	ActiveUntil          *string                `json:"active_until"`
	ExplanationReleaseAt *string                `json:"explanation_release_at"`
	RandomizeQuestions   bool                   `json:"randomize_questions"`
	RandomizeOptions     bool                   `json:"randomize_options"`
	Subtests             []tryoutSubtestRequest `json:"subtests"`
}

type updateTryoutRequest struct {
	createTryoutRequest
	BaseRevision int `json:"base_revision"`
}

func (h *Handler) ListTryouts(c echo.Context) error {
	scope, principal, ok := curriculumCaller(c)
	if !ok {
		return httpx.Unauthorized(sessionInvalid)
	}
	filter, err := tryoutFilterOf(c)
	if err != nil {
		return err
	}
	tryouts, total, err := h.tryouts.List(c.Request().Context(), scope, principal, filter)
	if err != nil {
		return tryoutError(err)
	}
	body := tryoutsResponse{Tryouts: make([]tryoutSummaryResponse, 0, len(tryouts)), Total: total}
	for _, tryout := range tryouts {
		body.Tryouts = append(body.Tryouts, tryoutSummaryOf(tryout))
	}
	return c.JSON(http.StatusOK, body)
}

func (h *Handler) GetTryout(c echo.Context) error {
	scope, principal, ok := curriculumCaller(c)
	if !ok {
		return httpx.Unauthorized(sessionInvalid)
	}
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return httpx.NotFound(tryoutNotFound)
	}
	tryout, err := h.tryouts.Get(c.Request().Context(), scope, principal, id)
	if err != nil {
		return tryoutError(err)
	}
	return c.JSON(http.StatusOK, tryoutResponseOf(tryout))
}

const maxTryoutBody = 256 << 10

func bindTryout(c echo.Context, dst any) error {
	c.Request().Body = http.MaxBytesReader(c.Response(), c.Request().Body, maxTryoutBody)
	return bindStrict(c, dst)
}

func (h *Handler) CreateTryout(c echo.Context) error {
	scope, principal, ok := curriculumCaller(c)
	if !ok {
		return httpx.Unauthorized(sessionInvalid)
	}
	var req createTryoutRequest
	if err := bindTryout(c, &req); err != nil {
		return err
	}
	input, err := tryoutInputOf(req)
	if err != nil {
		return err
	}
	created, err := h.tryouts.Create(c.Request().Context(), scope, principal, input)
	if err != nil {
		return tryoutError(err)
	}
	return c.JSON(http.StatusCreated, tryoutResponseOf(created))
}

func (h *Handler) UpdateTryout(c echo.Context) error {
	scope, principal, ok := curriculumCaller(c)
	if !ok {
		return httpx.Unauthorized(sessionInvalid)
	}
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return httpx.NotFound(tryoutNotFound)
	}
	var req updateTryoutRequest
	if err := bindTryout(c, &req); err != nil {
		return err
	}
	input, err := tryoutInputOf(req.createTryoutRequest)
	if err != nil {
		return err
	}
	input.BaseRevision = req.BaseRevision
	updated, err := h.tryouts.Update(c.Request().Context(), scope, principal, id, input)
	if err != nil {
		return tryoutError(err)
	}
	return c.JSON(http.StatusOK, tryoutResponseOf(updated))
}

func (h *Handler) DeleteTryout(c echo.Context) error {
	scope, principal, ok := curriculumCaller(c)
	if !ok {
		return httpx.Unauthorized(sessionInvalid)
	}
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return httpx.NotFound(tryoutNotFound)
	}
	if err := h.tryouts.Delete(c.Request().Context(), scope, principal, id); err != nil {
		return tryoutError(err)
	}
	return c.NoContent(http.StatusNoContent)
}

func (h *Handler) PublishTryout(c echo.Context) error {
	scope, principal, ok := curriculumCaller(c)
	if !ok {
		return httpx.Unauthorized(sessionInvalid)
	}
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return httpx.NotFound(tryoutNotFound)
	}
	tryout, err := h.tryouts.Publish(c.Request().Context(), scope, principal, id)
	if err != nil {
		return tryoutError(err)
	}
	return c.JSON(http.StatusOK, tryoutResponseOf(tryout))
}

func (h *Handler) UnpublishTryout(c echo.Context) error {
	scope, principal, ok := curriculumCaller(c)
	if !ok {
		return httpx.Unauthorized(sessionInvalid)
	}
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return httpx.NotFound(tryoutNotFound)
	}
	tryout, err := h.tryouts.Unpublish(c.Request().Context(), scope, principal, id)
	if err != nil {
		return tryoutError(err)
	}
	return c.JSON(http.StatusOK, tryoutResponseOf(tryout))
}

func tryoutFilterOf(c echo.Context) (service.TryoutFilter, error) {
	filter := service.TryoutFilter{}
	fields := map[string]string{}

	if raw := c.QueryParam("program_id"); raw != "" {
		id, err := uuid.Parse(raw)
		if err != nil {
			fields["program_id"] = invalidFormat
		} else {
			filter.ProgramID = &id
		}
	}
	if raw := c.QueryParam("limit"); raw != "" {
		value, err := strconv.Atoi(raw)
		if err != nil {
			fields["limit"] = invalidFormat
		} else {
			filter.Limit = value
		}
	}
	if raw := c.QueryParam("offset"); raw != "" {
		value, err := strconv.Atoi(raw)
		if err != nil {
			fields["offset"] = invalidFormat
		} else {
			filter.Offset = value
		}
	}
	filter.Status = c.QueryParam("status")
	filter.Search = c.QueryParam("q")
	if len(fields) > 0 {
		return service.TryoutFilter{}, httpx.Validation(incomplete, fields)
	}
	return filter, nil
}

// tryoutInputOf turns the wire body into the service's input, naming bad ids and dates by their path.
func tryoutInputOf(req createTryoutRequest) (service.TryoutInput, error) {
	fields := map[string]string{}
	input := service.TryoutInput{
		Title: req.Title, Description: req.Description, ScoringMode: req.ScoringMode,
		RandomizeQuestions: req.RandomizeQuestions, RandomizeOptions: req.RandomizeOptions,
	}
	if req.ProgramID != "" {
		id, err := uuid.Parse(req.ProgramID)
		if err != nil {
			fields["program_id"] = invalidFormat
		} else {
			input.ProgramID = id
		}
	}
	for name, raw := range map[string]*string{
		"active_from":            req.ActiveFrom,
		"active_until":           req.ActiveUntil,
		"explanation_release_at": req.ExplanationReleaseAt,
	} {
		parsed, ok := tryoutTimeOf(raw)
		if !ok {
			fields[name] = invalidFormat
			continue
		}
		switch name {
		case "active_from":
			input.ActiveFrom = parsed
		case "active_until":
			input.ActiveUntil = parsed
		default:
			input.ExplanationReleaseAt = parsed
		}
	}
	for i, subtest := range req.Subtests {
		in := service.TryoutSubtestInput{Name: subtest.Name, DurationSeconds: subtest.DurationSeconds}
		for j, question := range subtest.Questions {
			path := fmt.Sprintf("subtests[%d].questions[%d]", i, j)
			item := service.TryoutQuestionInput{}
			if question.QuestionID == "" {
				fields[path+".question_id"] = invalidFormat
			} else if id, err := uuid.Parse(question.QuestionID); err != nil {
				fields[path+".question_id"] = invalidFormat
			} else {
				item.QuestionID = id
			}
			if question.VersionID != nil && strings.TrimSpace(*question.VersionID) != "" {
				if id, err := uuid.Parse(strings.TrimSpace(*question.VersionID)); err != nil {
					fields[path+".version_id"] = invalidFormat
				} else {
					item.VersionID = &id
				}
			}
			in.Questions = append(in.Questions, item)
		}
		input.Subtests = append(input.Subtests, in)
	}
	if len(fields) > 0 {
		return service.TryoutInput{}, httpx.Validation(incomplete, fields)
	}
	return input, nil
}

func tryoutTimeOf(raw *string) (*time.Time, bool) {
	if raw == nil || strings.TrimSpace(*raw) == "" {
		return nil, true
	}
	parsed, err := time.Parse(time.RFC3339, strings.TrimSpace(*raw))
	if err != nil {
		return nil, false
	}
	return &parsed, true
}

func tryoutSummaryOf(tryout model.Tryout) tryoutSummaryResponse {
	return tryoutSummaryResponse{
		ID: tryout.ID, ProgramID: tryout.ProgramID, Title: tryout.Title,
		Description: tryout.Description, ScoringMode: tryout.ScoringMode, Status: tryout.Status,
		ActiveFrom: tryout.ActiveFrom, ActiveUntil: tryout.ActiveUntil,
		ExplanationReleaseAt: tryout.ExplanationReleaseAt, RandomizeQuestions: tryout.RandomizeQuestions,
		RandomizeOptions: tryout.RandomizeOptions, Revision: tryout.Revision, Locked: tryout.Locked,
		SubtestCount: tryout.SubtestCount, QuestionCount: tryout.QuestionCount,
		CreatedAt: tryout.CreatedAt, UpdatedAt: tryout.UpdatedAt,
	}
}

func tryoutResponseOf(tryout model.Tryout) tryoutResponse {
	out := tryoutResponse{
		tryoutSummaryResponse: tryoutSummaryOf(tryout),
		Subtests:              make([]subtestResponse, 0, len(tryout.Subtests)),
	}
	for _, subtest := range tryout.Subtests {
		item := subtestResponse{
			ID: subtest.ID, Name: subtest.Name, DurationSeconds: subtest.DurationSeconds,
			Position: subtest.Position, Questions: make([]assignmentResponse, 0, len(subtest.Questions)),
		}
		for _, question := range subtest.Questions {
			item.Questions = append(item.Questions, assignmentResponse{
				QuestionID: question.QuestionID, VersionID: question.VersionID, Version: question.Version,
				Number: question.Number, Type: question.Type, Difficulty: question.Difficulty,
				Content: question.Content, Position: question.Position,
			})
		}
		out.Subtests = append(out.Subtests, item)
	}
	return out
}

func tryoutError(err error) error {
	var input *service.InputError
	if errors.As(err, &input) {
		fields := make(map[string]string, len(input.Fields))
		for path, reason := range input.Fields {
			fields[path] = tryoutReason(reason)
		}
		return httpx.Validation(incomplete, fields)
	}
	switch {
	case errors.Is(err, service.ErrTryoutNotFound):
		return httpx.NotFound(tryoutNotFound)
	case errors.Is(err, service.ErrTenantNotFound):
		return httpx.NotFound(tenantNotFound)
	case errors.Is(err, service.ErrTryoutLocked):
		return httpx.Conflict(tryoutLocked)
	case errors.Is(err, service.ErrTryoutStale):
		return httpx.Conflict(tryoutConflict)
	case errors.Is(err, service.ErrTryoutReferenced):
		return httpx.Conflict(tryoutReferenced)
	}
	return err
}

func tryoutReason(reason string) string {
	switch reason {
	case service.ReasonTryoutTitle:
		return tryoutTitleRule
	case service.ReasonTryoutDescription:
		return tryoutDescRule
	case service.ReasonTryoutScoring:
		return tryoutScoreRule
	case service.ReasonTryoutProgram:
		return tryoutProgram
	case service.ReasonTryoutActiveUntil:
		return tryoutUntilRule
	case service.ReasonTryoutActiveFrom:
		return tryoutFromRule
	case service.ReasonTryoutSubtestMax:
		return tryoutSubtestMax
	case service.ReasonTryoutQuestionMax:
		return tryoutQuestionMax
	case service.ReasonTryoutSubtestName:
		return tryoutNameRule
	case service.ReasonTryoutDuration:
		return tryoutDuration
	case service.ReasonTryoutQuestion:
		return tryoutQuestion
	case service.ReasonTryoutDuplicate:
		return tryoutDuplicate
	case service.ReasonTryoutVersion:
		return tryoutVersion
	case service.ReasonTryoutEmpty:
		return tryoutEmpty
	case service.ReasonTryoutWindow:
		return tryoutWindow
	case service.ReasonTryoutRevision:
		return tryoutRevision
	case service.ReasonTryoutQuery:
		return tryoutListLimit
	}
	return invalidFormat
}
