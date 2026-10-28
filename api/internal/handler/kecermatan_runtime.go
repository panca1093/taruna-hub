package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"

	"github.com/tarunaeduka/bimbel-tryout/api/internal/httpx"
	"github.com/tarunaeduka/bimbel-tryout/api/internal/model"
	"github.com/tarunaeduka/bimbel-tryout/api/internal/service"
)

const (
	kecTestNotFound     = "Tes kecermatan tidak ditemukan."
	kecSessionMissing   = "Attempt kecermatan tidak ditemukan."
	kecNoCredit         = "Attempt kamu sudah habis. Beli attempt tambahan untuk melanjutkan."
	kecSessionDone      = "Attempt ini sudah selesai."
	kecKolomNotRunning  = "Kolom ini tidak sedang berjalan."
	kecAnswerLocked     = "Soal ini sudah dijawab dan tidak bisa diubah."
	kecNotCompleted     = "Hasil belum tersedia. Selesaikan attempt terlebih dahulu."
	kecAnswerRule       = "Jawaban harus label A sampai sesuai jumlah opsi, atau null untuk melewati."
	kecAnswerSoalRule   = "Nomor soal di luar jumlah soal kolom."
	kecAnswerBodyRule   = "Field answer wajib ada: label atau null."
	kecResultQueryRule  = "Parameter tidak valid."
	kecRuntimeForbidden = "Tindakan ini tidak diizinkan untuk peran kamu."
	maxKecAnswerBody    = 1 << 10
)

const (
	codeKecNoCredit        httpx.Code = "kecermatan_no_credit"
	codeKecSessionClosed   httpx.Code = "kecermatan_session_completed"
	codeKecKolomNotRunning httpx.Code = "kecermatan_kolom_not_running"
	codeKecAnswerLocked    httpx.Code = "kecermatan_answer_locked"
	codeKecNotCompleted    httpx.Code = "kecermatan_result_not_ready"
)

// WithKecermatanRuntime attaches the attempt service. It is a separate step so the long constructor, which
// every fixture calls, stays as it was.
func (h *Handler) WithKecermatanRuntime(s *service.KecermatanRuntimeService) *Handler {
	h.kecRuntime = s
	return h
}

type kecAttemptResponse struct {
	SessionID     uuid.UUID  `json:"session_id"`
	AttemptNumber int        `json:"attempt_number"`
	Status        string     `json:"status"`
	StartedAt     time.Time  `json:"started_at"`
	CompletedAt   *time.Time `json:"completed_at"`
	NilaiAkhir    *float64   `json:"nilai_akhir"`
}

type kecStudentTestResponse struct {
	TestID               uuid.UUID `json:"test_id"`
	Title                string    `json:"title"`
	Description          string    `json:"description"`
	ProgramCode          string    `json:"program_code"`
	ProgramName          string    `json:"program_name"`
	OptionsPerSoal       int       `json:"options_per_soal"`
	KolomCount           int       `json:"kolom_count"`
	SoalPerKolom         int       `json:"soal_per_kolom"`
	KolomDurationSeconds int       `json:"kolom_duration_seconds"`
	CoverFrom            string    `json:"cover_from"`
	CoverTo              string    `json:"cover_to"`
	AttemptsPurchased    int       `json:"attempts_purchased"`
	AttemptsUsed         int       `json:"attempts_used"`
	AttemptsRemaining    int       `json:"attempts_remaining"`
	InProgress           bool      `json:"in_progress"`
}

type kecStudentTestsResponse struct {
	Tests []kecStudentTestResponse `json:"tests"`
}

type kecOverviewResponse struct {
	Test      kecStudentTestResponse `json:"test"`
	Current   *kecSessionResponse    `json:"current"`
	Attempts  []kecAttemptResponse   `json:"attempts"`
	ProductID *uuid.UUID             `json:"product_id"`
}

type kecElementResponse struct {
	Kind    string     `json:"kind"`
	Value   string     `json:"value"`
	AssetID *uuid.UUID `json:"asset_id"`
}

type kecReferenceResponse struct {
	Label    string               `json:"label"`
	Elements []kecElementResponse `json:"elements"`
}

// kecSoalResponse names the labels a student sees on a soal. The missing label is never part of it.
type kecSoalResponse struct {
	Number int      `json:"number"`
	Shown  []string `json:"shown"`
}

type kecStoredAnswerResponse struct {
	Soal   int     `json:"soal"`
	Answer *string `json:"answer"`
}

type kecKolomResponse struct {
	Number     int                       `json:"number"`
	StartedAt  time.Time                 `json:"started_at"`
	DeadlineAt time.Time                 `json:"deadline_at"`
	Reference  []kecReferenceResponse    `json:"reference"`
	Soal       []kecSoalResponse         `json:"soal"`
	Answers    []kecStoredAnswerResponse `json:"answers"`
}

type kecScoresResponse struct {
	Kecepatan  float64 `json:"kecepatan"`
	Ketahanan  float64 `json:"ketahanan"`
	Ketelitian float64 `json:"ketelitian"`
	NilaiAkhir float64 `json:"nilai_akhir"`
}

type kecKolomResultResponse struct {
	Number int `json:"number"`
	Benar  int `json:"benar"`
	Salah  int `json:"salah"`
	Kosong int `json:"kosong"`
}

type kecResultResponse struct {
	SessionID      uuid.UUID                `json:"session_id"`
	TestID         uuid.UUID                `json:"test_id"`
	TestTitle      string                   `json:"test_title"`
	AttemptNumber  int                      `json:"attempt_number"`
	BenarTotal     int                      `json:"benar_total"`
	SalahTotal     int                      `json:"salah_total"`
	KosongTotal    int                      `json:"kosong_total"`
	Scores         kecScoresResponse        `json:"scores"`
	ScoringVersion int                      `json:"scoring_version"`
	CompletedAt    time.Time                `json:"completed_at"`
	Kolom          []kecKolomResultResponse `json:"kolom"`
}

type kecSessionResponse struct {
	ID                   uuid.UUID          `json:"id"`
	TestID               uuid.UUID          `json:"test_id"`
	TestTitle            string             `json:"test_title"`
	AttemptNumber        int                `json:"attempt_number"`
	Status               string             `json:"status"`
	Phase                string             `json:"phase"`
	StartedAt            time.Time          `json:"started_at"`
	CompletedAt          *time.Time         `json:"completed_at"`
	ServerTime           time.Time          `json:"server_time"`
	KolomCount           int                `json:"kolom_count"`
	SoalPerKolom         int                `json:"soal_per_kolom"`
	OptionsPerSoal       int                `json:"options_per_soal"`
	KolomDurationSeconds int                `json:"kolom_duration_seconds"`
	PauseSeconds         int                `json:"pause_seconds"`
	Kolom                *kecKolomResponse  `json:"kolom"`
	FinishedKolom        *int               `json:"finished_kolom"`
	NextKolom            *int               `json:"next_kolom"`
	NextStartsAt         *time.Time         `json:"next_starts_at"`
	Result               *kecResultResponse `json:"result"`
}

type kecAnswerRequest struct {
	Answer json.RawMessage `json:"answer"`
}

type kecAnswerResponse struct {
	Kolom      int       `json:"kolom"`
	Soal       int       `json:"soal"`
	Answer     *string   `json:"answer"`
	ServerTime time.Time `json:"server_time"`
}

type kecReviewKolomResponse struct {
	Number       int       `json:"number"`
	SourceColumn int       `json:"source_column"`
	Benar        int       `json:"benar"`
	Salah        int       `json:"salah"`
	Kosong       int       `json:"kosong"`
	Answers      []*string `json:"answers"`
}

type kecReviewSoalResponse struct {
	Shown   []string `json:"shown"`
	Missing string   `json:"missing"`
}

type kecReviewColumnResponse struct {
	Reference []kecReferenceResponse  `json:"reference"`
	Soal      []kecReviewSoalResponse `json:"soal"`
}

type kecReviewResponse struct {
	Result  kecResultResponse         `json:"result"`
	Columns []kecReviewColumnResponse `json:"columns"`
	Kolom   []kecReviewKolomResponse  `json:"kolom"`
}

type kecStaffResultResponse struct {
	SessionID      uuid.UUID         `json:"session_id"`
	StudentID      uuid.UUID         `json:"student_id"`
	StudentName    string            `json:"student_name"`
	TestID         uuid.UUID         `json:"test_id"`
	TestTitle      string            `json:"test_title"`
	ProgramID      uuid.UUID         `json:"program_id"`
	ProgramCode    string            `json:"program_code"`
	AttemptNumber  int               `json:"attempt_number"`
	BenarTotal     int               `json:"benar_total"`
	SalahTotal     int               `json:"salah_total"`
	KosongTotal    int               `json:"kosong_total"`
	Scores         kecScoresResponse `json:"scores"`
	ScoringVersion int               `json:"scoring_version"`
	CompletedAt    time.Time         `json:"completed_at"`
}

type kecStaffResultsResponse struct {
	Results []kecStaffResultResponse `json:"results"`
	Total   int                      `json:"total"`
}

type kecStaffResultDetailResponse struct {
	kecStaffResultResponse
	Kolom []kecKolomResultResponse `json:"kolom"`
}

func (h *Handler) MyKecermatanTests(c echo.Context) error {
	scope, principal, ok := runtimeCaller(c)
	if !ok {
		return httpx.Unauthorized(sessionInvalid)
	}
	tests, err := h.kecRuntime.Tests(c.Request().Context(), scope, principal)
	if err != nil {
		return kecRuntimeError(err)
	}
	body := kecStudentTestsResponse{Tests: make([]kecStudentTestResponse, 0, len(tests))}
	for _, t := range tests {
		body.Tests = append(body.Tests, kecStudentTestOf(t))
	}
	return c.JSON(http.StatusOK, body)
}

func (h *Handler) MyKecermatanTest(c echo.Context) error {
	scope, principal, ok := runtimeCaller(c)
	if !ok {
		return httpx.Unauthorized(sessionInvalid)
	}
	id, err := runtimeID(c, "id")
	if err != nil {
		return err
	}
	view, err := h.kecRuntime.Overview(c.Request().Context(), scope, principal, id)
	if err != nil {
		return kecRuntimeError(err)
	}
	body := kecOverviewResponse{Test: kecStudentTestOf(view.Test), ProductID: view.ProductID,
		Attempts: make([]kecAttemptResponse, 0, len(view.Attempts))}
	if view.Current != nil {
		current := kecSessionOf(*view.Current)
		body.Current = &current
	}
	for _, a := range view.Attempts {
		body.Attempts = append(body.Attempts, kecAttemptResponse{
			SessionID: a.SessionID, AttemptNumber: a.AttemptNumber, Status: a.Status, StartedAt: a.StartedAt,
			CompletedAt: a.CompletedAt, NilaiAkhir: a.NilaiAkhir,
		})
	}
	return c.JSON(http.StatusOK, body)
}

// StartKecermatanSession answers 201 for a new attempt and 200 when an attempt was already in progress.
func (h *Handler) StartKecermatanSession(c echo.Context) error {
	scope, principal, ok := runtimeCaller(c)
	if !ok {
		return httpx.Unauthorized(sessionInvalid)
	}
	id, err := runtimeID(c, "id")
	if err != nil {
		return err
	}
	view, created, err := h.kecRuntime.Start(c.Request().Context(), scope, principal, id)
	if err != nil {
		return kecRuntimeError(err)
	}
	status := http.StatusOK
	if created {
		status = http.StatusCreated
	}
	return c.JSON(status, kecSessionOf(view))
}

func (h *Handler) KecermatanSession(c echo.Context) error {
	scope, principal, ok := runtimeCaller(c)
	if !ok {
		return httpx.Unauthorized(sessionInvalid)
	}
	id, err := runtimeID(c, "id")
	if err != nil {
		return err
	}
	view, err := h.kecRuntime.Session(c.Request().Context(), scope, principal, id)
	if err != nil {
		return kecRuntimeError(err)
	}
	return c.JSON(http.StatusOK, kecSessionOf(view))
}

func (h *Handler) ContinueKecermatanSession(c echo.Context) error {
	scope, principal, ok := runtimeCaller(c)
	if !ok {
		return httpx.Unauthorized(sessionInvalid)
	}
	id, err := runtimeID(c, "id")
	if err != nil {
		return err
	}
	view, err := h.kecRuntime.Continue(c.Request().Context(), scope, principal, id)
	if err != nil {
		return kecRuntimeError(err)
	}
	return c.JSON(http.StatusOK, kecSessionOf(view))
}

func (h *Handler) AnswerKecermatan(c echo.Context) error {
	scope, principal, ok := runtimeCaller(c)
	if !ok {
		return httpx.Unauthorized(sessionInvalid)
	}
	id, err := runtimeID(c, "id")
	if err != nil {
		return err
	}
	kolom, kolomErr := strconv.Atoi(c.Param("kolom"))
	soal, soalErr := strconv.Atoi(c.Param("soal"))
	if kolomErr != nil || soalErr != nil {
		return httpx.NotFound(kecSessionMissing)
	}
	c.Request().Body = http.MaxBytesReader(c.Response(), c.Request().Body, maxKecAnswerBody)
	var req kecAnswerRequest
	if err := bindStrict(c, &req); err != nil {
		return err
	}
	if len(req.Answer) == 0 {
		return httpx.Validation(incomplete, map[string]string{"answer": kecAnswerBodyRule})
	}
	var answer *string
	if string(req.Answer) != "null" {
		var label string
		if err := json.Unmarshal(req.Answer, &label); err != nil {
			return httpx.Validation(incomplete, map[string]string{"answer": kecAnswerRule})
		}
		answer = &label
	}
	ack, err := h.kecRuntime.Answer(c.Request().Context(), scope, principal, id, kolom, soal, answer)
	if err != nil {
		return kecRuntimeError(err)
	}
	return c.JSON(http.StatusOK, kecAnswerResponse{Kolom: ack.Kolom, Soal: ack.Soal, Answer: ack.Answer, ServerTime: ack.ServerTime})
}

func (h *Handler) KecermatanResult(c echo.Context) error {
	scope, principal, ok := runtimeCaller(c)
	if !ok {
		return httpx.Unauthorized(sessionInvalid)
	}
	id, err := runtimeID(c, "id")
	if err != nil {
		return err
	}
	view, err := h.kecRuntime.Result(c.Request().Context(), scope, principal, id)
	if err != nil {
		return kecRuntimeError(err)
	}
	return c.JSON(http.StatusOK, kecResultOf(view.Result, view.Kolom))
}

func (h *Handler) KecermatanReview(c echo.Context) error {
	scope, principal, ok := runtimeCaller(c)
	if !ok {
		return httpx.Unauthorized(sessionInvalid)
	}
	id, err := runtimeID(c, "id")
	if err != nil {
		return err
	}
	review, err := h.kecRuntime.Review(c.Request().Context(), scope, principal, id)
	if err != nil {
		return kecRuntimeError(err)
	}
	body := kecReviewResponse{
		Result:  kecResultOf(review.Result, nil),
		Columns: make([]kecReviewColumnResponse, 0, len(review.Columns)),
		Kolom:   make([]kecReviewKolomResponse, 0, len(review.Kolom)),
	}
	for _, col := range review.Columns {
		out := kecReviewColumnResponse{Reference: kecReferenceOf(col.Reference), Soal: make([]kecReviewSoalResponse, 0, len(col.Soal))}
		for _, soal := range col.Soal {
			out.Soal = append(out.Soal, kecReviewSoalResponse{Shown: soal.Shown, Missing: soal.Missing})
		}
		body.Columns = append(body.Columns, out)
	}
	for _, k := range review.Kolom {
		body.Kolom = append(body.Kolom, kecReviewKolomResponse{
			Number: k.Number, SourceColumn: k.SourceColumn, Benar: k.Benar, Salah: k.Salah, Kosong: k.Kosong, Answers: k.Answers,
		})
	}
	return c.JSON(http.StatusOK, body)
}

func (h *Handler) StaffKecermatanResults(c echo.Context) error {
	scope, principal, ok := runtimeCaller(c)
	if !ok {
		return httpx.Unauthorized(sessionInvalid)
	}
	filter := service.KecResultFilter{}
	fields := map[string]string{}
	for name, target := range map[string]**uuid.UUID{"test_id": &filter.TestID, "program_id": &filter.ProgramID} {
		raw := c.QueryParam(name)
		if raw == "" {
			continue
		}
		id, err := uuid.Parse(raw)
		if err != nil {
			fields[name] = invalidFormat
			continue
		}
		*target = &id
	}
	for name, target := range map[string]*int{"limit": &filter.Limit, "offset": &filter.Offset} {
		raw := c.QueryParam(name)
		if raw == "" {
			continue
		}
		value, err := strconv.Atoi(raw)
		if err != nil {
			fields[name] = invalidFormat
			continue
		}
		*target = value
	}
	if len(fields) > 0 {
		return httpx.Validation(incomplete, fields)
	}
	rows, total, err := h.kecRuntime.StaffResults(c.Request().Context(), scope, principal, filter)
	if err != nil {
		return kecRuntimeError(err)
	}
	body := kecStaffResultsResponse{Results: make([]kecStaffResultResponse, 0, len(rows)), Total: total}
	for _, row := range rows {
		body.Results = append(body.Results, kecStaffResultOf(row))
	}
	return c.JSON(http.StatusOK, body)
}

func (h *Handler) StaffKecermatanResult(c echo.Context) error {
	scope, principal, ok := runtimeCaller(c)
	if !ok {
		return httpx.Unauthorized(sessionInvalid)
	}
	id, err := runtimeID(c, "id")
	if err != nil {
		return err
	}
	view, err := h.kecRuntime.StaffResult(c.Request().Context(), scope, principal, id)
	if err != nil {
		return kecRuntimeError(err)
	}
	body := kecStaffResultDetailResponse{kecStaffResultResponse: kecStaffResultOf(view.Result), Kolom: kecKolomResultsOf(view.Kolom)}
	return c.JSON(http.StatusOK, body)
}

func kecStudentTestOf(t model.KecStudentTest) kecStudentTestResponse {
	return kecStudentTestResponse{
		TestID: t.TestID, Title: t.Title, Description: t.Description, ProgramCode: t.ProgramCode, ProgramName: t.ProgramName,
		OptionsPerSoal: t.OptionsPerSoal, KolomCount: t.KolomCount, SoalPerKolom: t.SoalPerKolom,
		KolomDurationSeconds: t.KolomDurationSeconds, CoverFrom: t.CoverFrom, CoverTo: t.CoverTo, AttemptsPurchased: t.Purchased, AttemptsUsed: t.Used,
		AttemptsRemaining: t.Purchased - t.Used, InProgress: t.InProgress,
	}
}

func kecReferenceOf(items []service.KecItem) []kecReferenceResponse {
	out := make([]kecReferenceResponse, 0, len(items))
	for _, item := range items {
		ref := kecReferenceResponse{Label: item.Label, Elements: make([]kecElementResponse, 0, len(item.Elements))}
		for _, e := range item.Elements {
			ref.Elements = append(ref.Elements, kecElementResponse{Kind: e.Kind, Value: e.Value, AssetID: e.AssetID})
		}
		out = append(out, ref)
	}
	return out
}

func kecKolomResultsOf(kolom []model.KecKolom) []kecKolomResultResponse {
	out := make([]kecKolomResultResponse, 0, len(kolom))
	for _, k := range kolom {
		row := kecKolomResultResponse{Number: k.Number}
		if k.Benar != nil {
			row.Benar, row.Salah, row.Kosong = *k.Benar, *k.Salah, *k.Kosong
		}
		out = append(out, row)
	}
	return out
}

func kecResultOf(r model.KecResultRow, kolom []model.KecKolom) kecResultResponse {
	return kecResultResponse{
		SessionID: r.SessionID, TestID: r.TestID, TestTitle: r.TestTitle, AttemptNumber: r.AttemptNumber,
		BenarTotal: r.BenarTotal, SalahTotal: r.SalahTotal, KosongTotal: r.KosongTotal,
		Scores:         kecScoresResponse{Kecepatan: r.Kecepatan, Ketahanan: r.Ketahanan, Ketelitian: r.Ketelitian, NilaiAkhir: r.NilaiAkhir},
		ScoringVersion: r.ScoringVersion, CompletedAt: r.CompletedAt, Kolom: kecKolomResultsOf(kolom),
	}
}

func kecStaffResultOf(r model.KecResultRow) kecStaffResultResponse {
	return kecStaffResultResponse{
		SessionID: r.SessionID, StudentID: r.UserID, StudentName: r.StudentName, TestID: r.TestID, TestTitle: r.TestTitle,
		ProgramID: r.ProgramID, ProgramCode: r.ProgramCode, AttemptNumber: r.AttemptNumber,
		BenarTotal: r.BenarTotal, SalahTotal: r.SalahTotal, KosongTotal: r.KosongTotal,
		Scores:         kecScoresResponse{Kecepatan: r.Kecepatan, Ketahanan: r.Ketahanan, Ketelitian: r.Ketelitian, NilaiAkhir: r.NilaiAkhir},
		ScoringVersion: r.ScoringVersion, CompletedAt: r.CompletedAt,
	}
}

func kecSessionOf(v service.KecSessionView) kecSessionResponse {
	s := v.Session
	out := kecSessionResponse{
		ID: s.ID, TestID: s.TestID, TestTitle: s.TestTitle, AttemptNumber: s.AttemptNumber, Status: s.Status, Phase: v.Phase,
		StartedAt: s.StartedAt, CompletedAt: s.CompletedAt, ServerTime: v.ServerTime, KolomCount: s.KolomCount,
		SoalPerKolom: s.SoalPerKolom, OptionsPerSoal: s.OptionsPerSoal, KolomDurationSeconds: s.KolomDurationSeconds,
		PauseSeconds: s.PauseSeconds,
	}
	if k := v.Kolom; k != nil {
		kolom := kecKolomResponse{
			Number: k.Number, StartedAt: k.StartedAt, DeadlineAt: k.DeadlineAt, Reference: kecReferenceOf(k.Reference),
			Soal: make([]kecSoalResponse, 0, len(k.Soal)), Answers: make([]kecStoredAnswerResponse, 0, len(k.Answers)),
		}
		for _, soal := range k.Soal {
			kolom.Soal = append(kolom.Soal, kecSoalResponse{Number: soal.Number, Shown: soal.Shown})
		}
		for _, a := range k.Answers {
			kolom.Answers = append(kolom.Answers, kecStoredAnswerResponse{Soal: a.Soal, Answer: a.Answer})
		}
		out.Kolom = &kolom
	}
	if v.Phase == service.KecPhasePause {
		finished, next := v.FinishedKolom, v.NextKolom
		out.FinishedKolom, out.NextKolom, out.NextStartsAt = &finished, &next, v.NextStartsAt
	}
	if v.Result != nil {
		result := kecResultOf(*v.Result, nil)
		out.Result = &result
	}
	return out
}

func kecRuntimeError(err error) error {
	var input *service.InputError
	if errors.As(err, &input) {
		fields := make(map[string]string, len(input.Fields))
		for path, reason := range input.Fields {
			switch reason {
			case service.ReasonKecAnswer:
				fields[path] = kecAnswerRule
			case service.ReasonKecAnswerSoal:
				fields[path] = kecAnswerSoalRule
			case service.ReasonKecResultQuery:
				fields[path] = kecResultQueryRule
			default:
				fields[path] = invalidFormat
			}
		}
		return httpx.Validation(incomplete, fields)
	}
	switch {
	case errors.Is(err, service.ErrTenantNotFound):
		return httpx.NotFound(tenantNotFound)
	case errors.Is(err, service.ErrKecNotEnrolled):
		return httpx.NotFound(kecTestNotFound)
	case errors.Is(err, service.ErrKecSessionNotFound):
		return httpx.NotFound(kecSessionMissing)
	case errors.Is(err, service.ErrRuntimeForbidden):
		return httpx.Forbidden(kecRuntimeForbidden)
	case errors.Is(err, service.ErrKecNoCredit):
		return httpx.New(http.StatusConflict, codeKecNoCredit, kecNoCredit)
	case errors.Is(err, service.ErrKecSessionClosed):
		return httpx.New(http.StatusConflict, codeKecSessionClosed, kecSessionDone)
	case errors.Is(err, service.ErrKecKolomNotRunning):
		return httpx.New(http.StatusConflict, codeKecKolomNotRunning, kecKolomNotRunning)
	case errors.Is(err, service.ErrKecAnswerLocked):
		return httpx.New(http.StatusConflict, codeKecAnswerLocked, kecAnswerLocked)
	case errors.Is(err, service.ErrKecNotCompleted):
		return httpx.New(http.StatusConflict, codeKecNotCompleted, kecNotCompleted)
	}
	return err
}
