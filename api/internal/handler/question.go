package handler

import (
	"encoding/json"
	"errors"
	"fmt"
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
	questionNotFound   = "Soal tidak ditemukan."
	questionConflict   = "Soal sudah diubah orang lain, muat ulang."
	questionReferenced = "Soal masih dipakai konten lain."
	questionTypeRule   = "Tipe soal tidak dikenal."
	questionContentEll = "Konten wajib diisi, maksimal 20000 karakter."
	questionImageRule  = "Gambar belum didukung."
	questionExplRule   = "Pembahasan maksimal 20000 karakter."
	docInvalidRule     = "Isian ini bukan dokumen yang valid."
	docNodeRule        = "Dokumen memuat elemen yang tidak didukung atau salah tempat."
	docMarkRule        = "Dokumen memuat format teks yang tidak didukung."
	docAttrRule        = "Dokumen memuat atribut yang tidak valid atau tidak diizinkan."
	docTooLargeRule    = "Dokumen terlalu besar atau terlalu dalam."
	docAssetRule       = "Gambar tidak ditemukan atau tidak bisa dipakai di sini."
	questionDiffRule   = "Kesulitan harus mudah, sedang, atau sulit."
	questionProgram    = "Program tidak ditemukan."
	questionSubject    = "Mata pelajaran tidak ada di program itu."
	questionChapter    = "Bab materi tidak ada di mata pelajaran itu."
	questionOptions    = "Jumlah opsi 2–8."
	questionLabel      = "Label opsi wajib diisi, maksimal 2000 karakter."
	questionLabelDup   = "Label opsi tidak boleh sama (tanpa membedakan huruf besar-kecil)."
	questionOptionOwn  = "Opsi ini bukan milik soal tersebut."
	questionPoints     = "Poin 0–100 dengan paling banyak 2 desimal."
	questionCorrect    = "Jawaban benar tidak sesuai dengan soal."
	questionWeighted   = "Bobot per opsi hanya untuk pilihan ganda."
	questionCase       = "Peka huruf besar-kecil hanya untuk tipe isian."
	questionListLimit  = "Batas 1–100."
	questionBase       = "Versi soal wajib diisi."
)

type questionOptionResponse struct {
	ID     string        `json:"id"`
	Label  model.RichDoc `json:"label"`
	Points *float64      `json:"points,omitempty"`
}

type questionResponse struct {
	ID             uuid.UUID                `json:"id"`
	Number         int64                    `json:"number"`
	Version        int                      `json:"version"`
	VersionID      uuid.UUID                `json:"version_id"`
	ProgramID      uuid.UUID                `json:"program_id"`
	SubjectID      uuid.UUID                `json:"subject_id"`
	ChapterID      *uuid.UUID               `json:"chapter_id"`
	Type           string                   `json:"type"`
	Content        model.RichDoc            `json:"content"`
	Difficulty     string                   `json:"difficulty"`
	Explanation    model.RichDoc            `json:"explanation"`
	OptionWeighted bool                     `json:"option_weighted"`
	CaseSensitive  bool                     `json:"case_sensitive"`
	Options        []questionOptionResponse `json:"options"`
	Correct        []string                 `json:"correct"`
	CreatedAt      time.Time                `json:"created_at"`
	UpdatedAt      time.Time                `json:"updated_at"`
	// Only the single-question read sets it, so the list does not pay a count per row.
	RescoreAttempts *int `json:"rescore_attempts,omitempty"`
}

type questionsResponse struct {
	Questions []questionResponse `json:"questions"`
	Total     int                `json:"total"`
}

type questionOptionRequest struct {
	ID     string          `json:"id"`
	Label  json.RawMessage `json:"label"`
	Points *float64        `json:"points"`
}

type createQuestionRequest struct {
	ProgramID      string                  `json:"program_id"`
	SubjectID      string                  `json:"subject_id"`
	ChapterID      *string                 `json:"chapter_id"`
	Type           string                  `json:"type"`
	Content        json.RawMessage         `json:"content"`
	Difficulty     string                  `json:"difficulty"`
	Explanation    json.RawMessage         `json:"explanation"`
	OptionWeighted bool                    `json:"option_weighted"`
	CaseSensitive  bool                    `json:"case_sensitive"`
	Options        []questionOptionRequest `json:"options"`
	Correct        []string                `json:"correct"`
}

type updateQuestionRequest struct {
	createQuestionRequest
	BaseVersion int `json:"base_version"`
}

func (h *Handler) ListQuestions(c echo.Context) error {
	scope, principal, ok := curriculumCaller(c)
	if !ok {
		return httpx.Unauthorized(sessionInvalid)
	}
	filter, err := questionFilterOf(c)
	if err != nil {
		return err
	}
	questions, total, err := h.questions.List(c.Request().Context(), scope, principal, filter)
	if err != nil {
		return questionError(err)
	}
	body := questionsResponse{Questions: make([]questionResponse, 0, len(questions)), Total: total}
	for _, question := range questions {
		body.Questions = append(body.Questions, questionResponseOf(question))
	}
	return c.JSON(http.StatusOK, body)
}

func (h *Handler) GetQuestion(c echo.Context) error {
	scope, principal, ok := curriculumCaller(c)
	if !ok {
		return httpx.Unauthorized(sessionInvalid)
	}
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return httpx.NotFound(questionNotFound)
	}
	question, err := h.questions.Get(c.Request().Context(), scope, principal, id)
	if err != nil {
		return questionError(err)
	}
	body := questionResponseOf(question)
	body.RescoreAttempts = &question.RescoreAttempts
	return c.JSON(http.StatusOK, body)
}

func (h *Handler) CreateQuestion(c echo.Context) error {
	scope, principal, ok := curriculumCaller(c)
	if !ok {
		return httpx.Unauthorized(sessionInvalid)
	}
	var req createQuestionRequest
	if err := bindStrict(c, &req); err != nil {
		return err
	}
	input, err := questionInputOf(req)
	if err != nil {
		return err
	}
	created, err := h.questions.Create(c.Request().Context(), scope, principal, input)
	if err != nil {
		return questionError(err)
	}
	return c.JSON(http.StatusCreated, questionResponseOf(created))
}

func (h *Handler) UpdateQuestion(c echo.Context) error {
	scope, principal, ok := curriculumCaller(c)
	if !ok {
		return httpx.Unauthorized(sessionInvalid)
	}
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return httpx.NotFound(questionNotFound)
	}
	var req updateQuestionRequest
	if err := bindStrict(c, &req); err != nil {
		return err
	}
	input, err := questionInputOf(req.createQuestionRequest)
	if err != nil {
		return err
	}
	input.BaseVersion = req.BaseVersion
	updated, err := h.questions.Update(c.Request().Context(), scope, principal, id, input)
	if err != nil {
		return questionError(err)
	}
	return c.JSON(http.StatusOK, questionResponseOf(updated))
}

func (h *Handler) DeleteQuestion(c echo.Context) error {
	scope, principal, ok := curriculumCaller(c)
	if !ok {
		return httpx.Unauthorized(sessionInvalid)
	}
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return httpx.NotFound(questionNotFound)
	}
	if err := h.questions.Delete(c.Request().Context(), scope, principal, id); err != nil {
		return questionError(err)
	}
	return c.NoContent(http.StatusNoContent)
}

// questionFilterOf reads the query string; a value that is not a number or uuid names its field.
func questionFilterOf(c echo.Context) (service.QuestionFilter, error) {
	filter := service.QuestionFilter{}
	fields := map[string]string{}

	parseID := func(name string) *uuid.UUID {
		raw := c.QueryParam(name)
		if raw == "" {
			return nil
		}
		id, err := uuid.Parse(raw)
		if err != nil {
			fields[name] = invalidFormat
			return nil
		}
		return &id
	}
	parseInt := func(name string) int {
		raw := c.QueryParam(name)
		if raw == "" {
			return 0
		}
		value, err := strconv.Atoi(raw)
		if err != nil {
			fields[name] = invalidFormat
			return 0
		}
		return value
	}

	filter.ProgramID = parseID("program_id")
	filter.SubjectID = parseID("subject_id")
	filter.ChapterID = parseID("chapter_id")
	filter.Type = c.QueryParam("type")
	filter.Difficulty = c.QueryParam("difficulty")
	filter.Search = c.QueryParam("q")
	filter.Limit = parseInt("limit")
	filter.Offset = parseInt("offset")
	if len(fields) > 0 {
		return service.QuestionFilter{}, httpx.Validation(incomplete, fields)
	}
	return filter, nil
}

// questionInputOf turns the wire body into the service's input, naming bad ids by their path.
func questionInputOf(req createQuestionRequest) (service.QuestionInput, error) {
	fields := map[string]string{}
	input := service.QuestionInput{
		Type: req.Type, Difficulty: req.Difficulty,
		OptionWeighted: req.OptionWeighted, CaseSensitive: req.CaseSensitive, Correct: req.Correct,
	}
	input.Content = docOf(req.Content, "content", fields)
	input.Explanation = docOf(req.Explanation, "explanation", fields)
	if req.ProgramID != "" {
		id, err := uuid.Parse(req.ProgramID)
		if err != nil {
			fields["program_id"] = invalidFormat
		} else {
			input.ProgramID = id
		}
	}
	if req.SubjectID != "" {
		id, err := uuid.Parse(req.SubjectID)
		if err != nil {
			fields["subject_id"] = invalidFormat
		} else {
			input.SubjectID = id
		}
	}
	if req.ChapterID != nil && *req.ChapterID != "" {
		id, err := uuid.Parse(*req.ChapterID)
		if err != nil {
			fields["chapter_id"] = invalidFormat
		} else {
			input.ChapterID = &id
		}
	}
	for i, option := range req.Options {
		input.Options = append(input.Options, service.QuestionOptionInput{
			ID: option.ID, Label: docOf(option.Label, fmt.Sprintf("options[%d].label", i), fields), Points: option.Points,
		})
	}
	if len(fields) > 0 {
		return service.QuestionInput{}, httpx.Validation(incomplete, fields)
	}
	return input, nil
}

// docOf reads one document field; an absent or null one is the empty document, anything that is not a document
// names its field.
func docOf(raw json.RawMessage, path string, fields map[string]string) model.RichDoc {
	if len(raw) == 0 || string(raw) == "null" {
		return model.EmptyDoc()
	}
	var doc model.RichDoc
	if err := json.Unmarshal(raw, &doc); err != nil {
		fields[path] = docInvalidRule
		return model.EmptyDoc()
	}
	return doc
}

func questionResponseOf(question model.Question) questionResponse {
	out := questionResponse{
		ID: question.ID, Number: question.Number, Version: question.Version, VersionID: question.VersionID,
		ProgramID: question.ProgramID, SubjectID: question.SubjectID, ChapterID: question.ChapterID,
		Type: question.Type, Content: question.Content, Difficulty: question.Difficulty,
		Explanation: question.Explanation, OptionWeighted: question.OptionWeighted,
		CaseSensitive: question.CaseSensitive, Correct: question.Correct,
		Options:   make([]questionOptionResponse, 0, len(question.Options)),
		CreatedAt: question.CreatedAt,
		UpdatedAt: question.UpdatedAt,
	}
	if out.Correct == nil {
		out.Correct = []string{}
	}
	for _, option := range question.Options {
		out.Options = append(out.Options, questionOptionResponse{ID: option.ID, Label: option.Doc, Points: option.Points})
	}
	return out
}

func questionError(err error) error {
	var input *service.InputError
	if errors.As(err, &input) {
		fields := make(map[string]string, len(input.Fields))
		for path, reason := range input.Fields {
			fields[path] = questionReason(reason)
		}
		return httpx.Validation(incomplete, fields)
	}
	switch {
	case errors.Is(err, service.ErrQuestionNotFound):
		return httpx.NotFound(questionNotFound)
	case errors.Is(err, service.ErrTenantNotFound):
		return httpx.NotFound(tenantNotFound)
	case errors.Is(err, service.ErrQuestionStale):
		return httpx.Conflict(questionConflict)
	case errors.Is(err, service.ErrQuestionReferenced):
		return httpx.Conflict(questionReferenced)
	}
	return err
}

func questionReason(reason string) string {
	switch reason {
	case service.ReasonQuestionType:
		return questionTypeRule
	case service.ReasonQuestionContent:
		return questionContentEll
	case service.ReasonQuestionImage:
		return questionImageRule
	case service.ReasonQuestionExpl:
		return questionExplRule
	case service.ReasonDocInvalid:
		return docInvalidRule
	case service.ReasonDocNode:
		return docNodeRule
	case service.ReasonDocMark:
		return docMarkRule
	case service.ReasonDocAttr:
		return docAttrRule
	case service.ReasonDocTooLarge:
		return docTooLargeRule
	case service.ReasonDocAsset:
		return docAssetRule
	case service.ReasonDifficulty:
		return questionDiffRule
	case service.ReasonProgramID:
		return questionProgram
	case service.ReasonSubjectID:
		return questionSubject
	case service.ReasonChapterID:
		return questionChapter
	case service.ReasonOptions:
		return questionOptions
	case service.ReasonOptionLabel:
		return questionLabel
	case service.ReasonOptionDuplicate:
		return questionLabelDup
	case service.ReasonOptionForeign:
		return questionOptionOwn
	case service.ReasonOptionPoints:
		return questionPoints
	case service.ReasonCorrect:
		return questionCorrect
	case service.ReasonWeighted:
		return questionWeighted
	case service.ReasonCaseSensitive:
		return questionCase
	case service.ReasonQueryLimit:
		return questionListLimit
	case service.ReasonBaseVersion:
		return questionBase
	}
	return invalidFormat
}
