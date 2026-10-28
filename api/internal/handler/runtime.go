package handler

import (
	"context"
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
	sessionNotFound   = "Sesi tryout tidak ditemukan."
	runtimeForbidden  = "Tindakan ini tidak diizinkan untuk peran kamu."
	tryoutNotOpen     = "Tryout belum dibuka."
	tryoutClosed      = "Masa pengerjaan tryout sudah berakhir."
	sessionCompleted  = "Sesi ini sudah selesai."
	subtestNotRunning = "Subtes ini tidak sedang berjalan."
	deadlinePassed    = "Waktu subtes sudah habis."
	staleAnswer       = "Jawaban ini sudah digantikan yang lebih baru."
	subtestOrder      = "Selesaikan subtes sebelumnya terlebih dahulu."
	resultNotReady    = "Hasil belum tersedia."
	answerSeqRule     = "Nomor urut jawaban harus 1 atau lebih."
	answerShapeRule   = "Jawaban tidak sesuai dengan jenis soal."
	extensionSeconds  = "Tambahan waktu 1–14400 detik."
	extensionReason   = "Alasan wajib diisi, maksimal 500 karakter."
	extensionTotal    = "Total tambahan waktu sesi maksimal 28800 detik."
	weightRule        = "Bobot harus lebih dari 0, maksimal 1000, dengan dua desimal."
)

const maxAnswerBody = 8 << 10

const (
	codeTryoutNotOpen     httpx.Code = "tryout_not_open"
	codeTryoutClosed      httpx.Code = "tryout_closed"
	codeSessionCompleted  httpx.Code = "session_completed"
	codeSubtestNotRunning httpx.Code = "subtest_not_running"
	codeDeadlinePassed    httpx.Code = "deadline_passed"
	codeStaleAnswer       httpx.Code = "stale_answer"
	codeSubtestOrder      httpx.Code = "subtest_order"
	codeResultNotReady    httpx.Code = "result_not_ready"
)

type runtimePartResponse struct {
	SubtestID       uuid.UUID  `json:"subtest_id"`
	Name            string     `json:"name"`
	Position        int        `json:"position"`
	DurationSeconds int        `json:"duration_seconds"`
	QuestionCount   int        `json:"question_count"`
	Status          string     `json:"status"`
	StartedAt       *time.Time `json:"started_at"`
	DeadlineAt      *time.Time `json:"deadline_at"`
	SubmittedAt     *time.Time `json:"submitted_at"`
	Score           *float64   `json:"score"`
	CorrectCount    *int       `json:"correct_count"`
}

type resultSubtestResponse struct {
	SubtestID     uuid.UUID `json:"subtest_id"`
	Name          string    `json:"name"`
	Position      int       `json:"position"`
	Score         float64   `json:"score"`
	CorrectCount  int       `json:"correct_count"`
	QuestionCount int       `json:"question_count"`
}

type resultResponse struct {
	SessionID      uuid.UUID               `json:"session_id"`
	TryoutID       uuid.UUID               `json:"tryout_id"`
	AttemptNumber  int                     `json:"attempt_number"`
	Score          float64                 `json:"score"`
	ScoringMode    string                  `json:"scoring_mode"`
	ScoringVersion int                     `json:"scoring_version"`
	CompletedAt    time.Time               `json:"completed_at"`
	Subtests       []resultSubtestResponse `json:"subtests"`
}

type sessionResponse struct {
	ID            uuid.UUID             `json:"id"`
	TryoutID      uuid.UUID             `json:"tryout_id"`
	TryoutTitle   string                `json:"tryout_title"`
	AttemptNumber int                   `json:"attempt_number"`
	Status        string                `json:"status"`
	StartedAt     time.Time             `json:"started_at"`
	CompletedAt   *time.Time            `json:"completed_at"`
	ActiveUntil   time.Time             `json:"active_until"`
	ServerTime    time.Time             `json:"server_time"`
	Subtests      []runtimePartResponse `json:"subtests"`
	Result        *resultResponse       `json:"result"`
}

type attemptSummaryResponse struct {
	SessionID     uuid.UUID  `json:"session_id"`
	TryoutID      uuid.UUID  `json:"tryout_id"`
	AttemptNumber int        `json:"attempt_number"`
	Status        string     `json:"status"`
	StartedAt     time.Time  `json:"started_at"`
	CompletedAt   *time.Time `json:"completed_at"`
	Score         *float64   `json:"score"`
}

type attemptListResponse struct {
	Attempts []attemptSummaryResponse `json:"attempts"`
}

type overviewSubtestResponse struct {
	ID              uuid.UUID `json:"id"`
	Name            string    `json:"name"`
	DurationSeconds int       `json:"duration_seconds"`
	Position        int       `json:"position"`
	QuestionCount   int       `json:"question_count"`
}

type studentTryoutResponse struct {
	ID          uuid.UUID                 `json:"id"`
	Title       string                    `json:"title"`
	Description string                    `json:"description"`
	ScoringMode string                    `json:"scoring_mode"`
	ProgramCode string                    `json:"program_code"`
	ProgramName string                    `json:"program_name"`
	ActiveFrom  *time.Time                `json:"active_from"`
	ActiveUntil time.Time                 `json:"active_until"`
	Subtests    []overviewSubtestResponse `json:"subtests"`
	Current     *sessionResponse          `json:"current"`
	Attempts    []attemptSummaryResponse  `json:"attempts"`
}

type runtimeOptionResponse struct {
	ID    string `json:"id"`
	Label string `json:"label"`
}

// runtimeQuestionResponse is the student's question: no key, no option points, no explanation, no version ids.
type runtimeQuestionResponse struct {
	ID       uuid.UUID               `json:"id"`
	Position int                     `json:"position"`
	Type     string                  `json:"type"`
	Content  string                  `json:"content"`
	Options  []runtimeOptionResponse `json:"options"`
	Answer   []string                `json:"answer"`
	Revision int                     `json:"revision"`
	Seq      int64                   `json:"seq"`
}

type runtimeSubtestResponse struct {
	SessionID  uuid.UUID                 `json:"session_id"`
	Subtest    runtimePartResponse       `json:"subtest"`
	ServerTime time.Time                 `json:"server_time"`
	Questions  []runtimeQuestionResponse `json:"questions"`
}

type answerRequest struct {
	Seq    int64    `json:"seq"`
	Answer []string `json:"answer"`
}

type answerResponse struct {
	QuestionID uuid.UUID `json:"question_id"`
	Revision   int       `json:"revision"`
	Seq        int64     `json:"seq"`
	ServerTime time.Time `json:"server_time"`
}

type extensionRequest struct {
	Seconds int    `json:"seconds"`
	Reason  string `json:"reason"`
}

type extensionResponse struct {
	SessionID    uuid.UUID `json:"session_id"`
	SubtestID    uuid.UUID `json:"subtest_id"`
	Seconds      int       `json:"seconds"`
	TotalSeconds int       `json:"total_seconds"`
}

func runtimeCaller(c echo.Context) (service.HostScope, model.Principal, bool) {
	scope, scoped := hostScope(c)
	principal, signedIn := PrincipalFrom(c)
	return scope, principal, scoped && signedIn
}

// runtimeID is a 404 for anything that is not a uuid, never a 400 that would confirm a shape.
func runtimeID(c echo.Context, name string) (uuid.UUID, error) {
	id, err := uuid.Parse(c.Param(name))
	if err != nil {
		return uuid.Nil, httpx.NotFound(sessionNotFound)
	}
	return id, nil
}

func (h *Handler) StudentTryout(c echo.Context) error {
	scope, principal, ok := runtimeCaller(c)
	if !ok {
		return httpx.Unauthorized(sessionInvalid)
	}
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return httpx.NotFound(tryoutNotFound)
	}
	overview, err := h.runtime.Overview(c.Request().Context(), scope, principal, id)
	if err != nil {
		return runtimeError(err)
	}
	body := studentTryoutResponse{
		ID: overview.Tryout.ID, Title: overview.Tryout.Title, Description: overview.Tryout.Description,
		ScoringMode: overview.Tryout.ScoringMode, ProgramCode: overview.ProgramCode, ProgramName: overview.ProgramName,
		ActiveFrom: overview.Tryout.ActiveFrom, ActiveUntil: overview.Tryout.ActiveUntil,
		Subtests: make([]overviewSubtestResponse, 0, len(overview.Subtests)),
		Attempts: attemptSummariesOf(overview.Attempts),
	}
	for _, st := range overview.Subtests {
		body.Subtests = append(body.Subtests, overviewSubtestResponse{
			ID: st.ID, Name: st.Name, DurationSeconds: st.DurationSeconds, Position: st.Position, QuestionCount: st.QuestionCount,
		})
	}
	if overview.Current != nil {
		current := sessionResponseOf(*overview.Current)
		body.Current = &current
	}
	return c.JSON(http.StatusOK, body)
}

func (h *Handler) StartTryoutSession(c echo.Context) error {
	scope, principal, ok := runtimeCaller(c)
	if !ok {
		return httpx.Unauthorized(sessionInvalid)
	}
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return httpx.NotFound(tryoutNotFound)
	}
	view, err := h.runtime.Start(c.Request().Context(), scope, principal, id)
	if err != nil {
		return runtimeError(err)
	}
	return c.JSON(http.StatusOK, sessionResponseOf(view))
}

func (h *Handler) MyAttempts(c echo.Context) error {
	scope, principal, ok := runtimeCaller(c)
	if !ok {
		return httpx.Unauthorized(sessionInvalid)
	}
	attempts, err := h.runtime.Attempts(c.Request().Context(), scope, principal)
	if err != nil {
		return runtimeError(err)
	}
	return c.JSON(http.StatusOK, attemptListResponse{Attempts: attemptSummariesOf(attempts)})
}

func (h *Handler) TryoutSession(c echo.Context) error {
	scope, principal, ok := runtimeCaller(c)
	if !ok {
		return httpx.Unauthorized(sessionInvalid)
	}
	id, err := runtimeID(c, "id")
	if err != nil {
		return err
	}
	view, err := h.runtime.Session(c.Request().Context(), scope, principal, id)
	if err != nil {
		return runtimeError(err)
	}
	return c.JSON(http.StatusOK, sessionResponseOf(view))
}

func (h *Handler) TryoutSessionResult(c echo.Context) error {
	scope, principal, ok := runtimeCaller(c)
	if !ok {
		return httpx.Unauthorized(sessionInvalid)
	}
	id, err := runtimeID(c, "id")
	if err != nil {
		return err
	}
	result, err := h.runtime.Result(c.Request().Context(), scope, principal, id)
	if err != nil {
		return runtimeError(err)
	}
	return c.JSON(http.StatusOK, resultResponseOf(result))
}

func (h *Handler) TryoutSessionSubtest(c echo.Context) error {
	scope, principal, ok := runtimeCaller(c)
	if !ok {
		return httpx.Unauthorized(sessionInvalid)
	}
	sessionID, err := runtimeID(c, "id")
	if err != nil {
		return err
	}
	subtestID, err := runtimeID(c, "subtestId")
	if err != nil {
		return err
	}
	view, err := h.runtime.Subtest(c.Request().Context(), scope, principal, sessionID, subtestID)
	if err != nil {
		return runtimeError(err)
	}
	body := runtimeSubtestResponse{
		SessionID: sessionID, Subtest: partResponseOf(view.Part), ServerTime: view.ServerTime,
		Questions: make([]runtimeQuestionResponse, 0, len(view.Questions)),
	}
	for _, q := range view.Questions {
		item := runtimeQuestionResponse{
			ID: q.QuestionID, Position: q.Position, Type: q.Type, Content: q.Content,
			Options: make([]runtimeOptionResponse, 0, len(q.Options)), Answer: []string{},
		}
		for _, o := range q.Options {
			item.Options = append(item.Options, runtimeOptionResponse{ID: o.ID, Label: o.Label})
		}
		if saved, ok := view.Answers[q.QuestionID]; ok {
			item.Answer, item.Revision, item.Seq = saved.Answer, saved.Revision, saved.Seq
		}
		body.Questions = append(body.Questions, item)
	}
	return c.JSON(http.StatusOK, body)
}

func (h *Handler) StartSubtest(c echo.Context) error {
	return h.subtestAction(c, h.runtime.StartSubtest)
}

func (h *Handler) SubmitSubtest(c echo.Context) error {
	return h.subtestAction(c, h.runtime.SubmitSubtest)
}

type subtestAction func(ctx context.Context, scope service.HostScope, principal model.Principal, sessionID, subtestID uuid.UUID) (service.AttemptView, error)

func (h *Handler) subtestAction(c echo.Context, act subtestAction) error {
	scope, principal, ok := runtimeCaller(c)
	if !ok {
		return httpx.Unauthorized(sessionInvalid)
	}
	sessionID, err := runtimeID(c, "id")
	if err != nil {
		return err
	}
	subtestID, err := runtimeID(c, "subtestId")
	if err != nil {
		return err
	}
	view, err := act(c.Request().Context(), scope, principal, sessionID, subtestID)
	if err != nil {
		return runtimeError(err)
	}
	return c.JSON(http.StatusOK, sessionResponseOf(view))
}

func (h *Handler) SaveAnswer(c echo.Context) error {
	scope, principal, ok := runtimeCaller(c)
	if !ok {
		return httpx.Unauthorized(sessionInvalid)
	}
	sessionID, err := runtimeID(c, "id")
	if err != nil {
		return err
	}
	questionID, err := runtimeID(c, "questionId")
	if err != nil {
		return err
	}
	c.Request().Body = http.MaxBytesReader(c.Response(), c.Request().Body, maxAnswerBody)
	var req answerRequest
	if err := bindStrict(c, &req); err != nil {
		return err
	}
	ack, err := h.runtime.SaveAnswer(c.Request().Context(), scope, principal, sessionID, questionID,
		service.AnswerInput{Seq: req.Seq, Answer: req.Answer})
	if err != nil {
		return runtimeError(err)
	}
	return c.JSON(http.StatusOK, answerResponse{
		QuestionID: ack.QuestionID, Revision: ack.Revision, Seq: ack.Seq, ServerTime: ack.ServerTime,
	})
}

func (h *Handler) ExtendSession(c echo.Context) error {
	scope, principal, ok := runtimeCaller(c)
	if !ok {
		return httpx.Unauthorized(sessionInvalid)
	}
	id, err := runtimeID(c, "id")
	if err != nil {
		return err
	}
	var req extensionRequest
	if err := bindStrict(c, &req); err != nil {
		return err
	}
	ext, err := h.runtime.Extend(c.Request().Context(), scope, principal, id,
		service.ExtensionInput{Seconds: req.Seconds, Reason: req.Reason})
	if err != nil {
		return runtimeError(err)
	}
	return c.JSON(http.StatusOK, extensionResponse{SessionID: ext.SessionID, SubtestID: ext.SubtestID, Seconds: ext.Seconds, TotalSeconds: ext.TotalExtra})
}

func attemptSummariesOf(list []model.AttemptSummary) []attemptSummaryResponse {
	out := make([]attemptSummaryResponse, 0, len(list))
	for _, a := range list {
		out = append(out, attemptSummaryResponse{
			SessionID: a.SessionID, TryoutID: a.TryoutID, AttemptNumber: a.AttemptNumber, Status: a.Status,
			StartedAt: a.StartedAt, CompletedAt: a.CompletedAt, Score: a.Score,
		})
	}
	return out
}

func partResponseOf(p service.PartView) runtimePartResponse {
	return runtimePartResponse{
		SubtestID: p.SubtestID, Name: p.Name, Position: p.Position, DurationSeconds: p.DurationSeconds,
		QuestionCount: p.QuestionCount, Status: p.Status, StartedAt: p.StartedAt, DeadlineAt: p.Deadline,
		SubmittedAt: p.SubmittedAt, Score: p.Score, CorrectCount: p.CorrectCount,
	}
}

func sessionResponseOf(v service.AttemptView) sessionResponse {
	a := v.Attempt
	body := sessionResponse{
		ID: a.ID, TryoutID: a.TryoutID, TryoutTitle: a.TryoutTitle, AttemptNumber: a.AttemptNumber,
		Status: a.Status, StartedAt: a.StartedAt, CompletedAt: a.CompletedAt, ActiveUntil: a.ActiveUntil,
		ServerTime: v.ServerTime, Subtests: make([]runtimePartResponse, 0, len(v.Parts)),
	}
	for _, p := range v.Parts {
		part := partResponseOf(p)
		if a.Status != model.SessionCompleted {
			part.CorrectCount = nil
		}
		body.Subtests = append(body.Subtests, part)
	}
	if v.Result != nil {
		result := resultResponseOf(*v.Result)
		body.Result = &result
	}
	return body
}

func resultResponseOf(r model.Result) resultResponse {
	body := resultResponse{
		SessionID: r.SessionID, TryoutID: r.TryoutID, AttemptNumber: r.AttemptNumber, Score: r.Score,
		ScoringMode: r.ScoringMode, ScoringVersion: r.ScoringVersion, CompletedAt: r.CompletedAt,
		Subtests: make([]resultSubtestResponse, 0, len(r.Parts)),
	}
	for _, p := range r.Parts {
		body.Subtests = append(body.Subtests, resultSubtestResponse{
			SubtestID: p.SubtestID, Name: p.Name, Position: p.Position, Score: p.Score,
			CorrectCount: p.CorrectCount, QuestionCount: p.QuestionCount,
		})
	}
	return body
}

func runtimeError(err error) error {
	var input *service.InputError
	if errors.As(err, &input) {
		fields := make(map[string]string, len(input.Fields))
		for path, reason := range input.Fields {
			fields[path] = runtimeReason(reason)
		}
		return httpx.Validation(incomplete, fields)
	}
	switch {
	case errors.Is(err, service.ErrTenantNotFound):
		return httpx.NotFound(tenantNotFound)
	case errors.Is(err, service.ErrTryoutNotFound):
		return httpx.NotFound(tryoutNotFound)
	case errors.Is(err, service.ErrSessionNotFound):
		return httpx.NotFound(sessionNotFound)
	case errors.Is(err, service.ErrRuntimeForbidden):
		return httpx.Forbidden(runtimeForbidden)
	case errors.Is(err, service.ErrTryoutNotOpen):
		return httpx.New(http.StatusConflict, codeTryoutNotOpen, tryoutNotOpen)
	case errors.Is(err, service.ErrTryoutClosed):
		return httpx.New(http.StatusConflict, codeTryoutClosed, tryoutClosed)
	case errors.Is(err, service.ErrSessionClosed):
		return httpx.New(http.StatusConflict, codeSessionCompleted, sessionCompleted)
	case errors.Is(err, service.ErrPartNotRunning):
		return httpx.New(http.StatusConflict, codeSubtestNotRunning, subtestNotRunning)
	case errors.Is(err, service.ErrDeadlinePassed):
		return httpx.New(http.StatusConflict, codeDeadlinePassed, deadlinePassed)
	case errors.Is(err, service.ErrStaleAnswer):
		return httpx.New(http.StatusConflict, codeStaleAnswer, staleAnswer)
	case errors.Is(err, service.ErrPartOrder):
		return httpx.New(http.StatusConflict, codeSubtestOrder, subtestOrder)
	case errors.Is(err, service.ErrNotCompleted):
		return httpx.New(http.StatusConflict, codeResultNotReady, resultNotReady)
	}
	return err
}

func runtimeReason(reason string) string {
	switch reason {
	case service.ReasonAnswerSeq:
		return answerSeqRule
	case service.ReasonAnswerShape:
		return answerShapeRule
	case service.ReasonExtraSeconds:
		return extensionSeconds
	case service.ReasonExtraReason:
		return extensionReason
	case service.ReasonExtraTotal:
		return extensionTotal
	case service.ReasonWeight:
		return weightRule
	}
	return invalidFormat
}
