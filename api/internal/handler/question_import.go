package handler

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"

	"github.com/tarunaeduka/bimbel-tryout/api/internal/httpx"
	"github.com/tarunaeduka/bimbel-tryout/api/internal/model"
	"github.com/tarunaeduka/bimbel-tryout/api/internal/service"
)

const (
	importFileRule    = "Format file tidak dikenali."
	importEmptyRule   = "File tidak berisi baris soal."
	importRowsRule    = "Pilih baris yang valid."
	importHeaderRule  = "Header tidak sesuai template."
	importSizeRule    = "Ukuran file maksimal 5 MB."
	importFormatRule  = "Format template harus xlsx, csv, atau docx."
	importBoolRule    = "Isi kolom ini dengan yes atau no."
	importMissingFile = "File soal wajib diunggah."
)

// maxImportUpload leaves room for the multipart framing; the 5 MB rule is on the file itself.
const maxImportUpload = 6 << 20

type importRowErrorResponse struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

type importRowPreviewResponse struct {
	Type       string `json:"type"`
	Content    string `json:"content"`
	Program    string `json:"program"`
	Subject    string `json:"subject"`
	Difficulty string `json:"difficulty"`
}

type importRowResponse struct {
	Row     int                       `json:"row"`
	Valid   bool                      `json:"valid"`
	Errors  []importRowErrorResponse  `json:"errors"`
	Preview *importRowPreviewResponse `json:"preview,omitempty"`
}

type importPreviewResponse struct {
	FileName string              `json:"file_name"`
	Format   string              `json:"format"`
	Total    int                 `json:"total"`
	Valid    int                 `json:"valid"`
	Invalid  int                 `json:"invalid"`
	Rows     []importRowResponse `json:"rows"`
}

type importCommitResponse struct {
	Created         int         `json:"created"`
	AlreadyImported int         `json:"already_imported"`
	QuestionIDs     []uuid.UUID `json:"question_ids"`
}

func (h *Handler) PreviewQuestionImport(c echo.Context) error {
	scope, principal, ok := curriculumCaller(c)
	if !ok {
		return httpx.Unauthorized(sessionInvalid)
	}
	data, fileName, err := importUpload(c)
	if err != nil {
		return err
	}
	preview, err := h.questions.PreviewImport(c.Request().Context(), scope, principal, fileName, data)
	if err != nil {
		return importError(err)
	}
	return c.JSON(http.StatusOK, importPreviewResponseOf(preview))
}

func (h *Handler) CommitQuestionImport(c echo.Context) error {
	scope, principal, ok := curriculumCaller(c)
	if !ok {
		return httpx.Unauthorized(sessionInvalid)
	}
	data, fileName, err := importUpload(c)
	if err != nil {
		return err
	}
	selected, err := importSelection(c.FormValue("rows"))
	if err != nil {
		return err
	}
	commit, err := h.questions.CommitImport(c.Request().Context(), scope, principal, fileName, data, selected)
	if err != nil {
		return importError(err)
	}
	return c.JSON(http.StatusOK, importCommitResponse{
		Created: commit.Created, AlreadyImported: commit.AlreadyImported, QuestionIDs: commit.QuestionIDs,
	})
}

func (h *Handler) QuestionImportTemplate(c echo.Context) error {
	scope, principal, ok := curriculumCaller(c)
	if !ok {
		return httpx.Unauthorized(sessionInvalid)
	}
	template, err := h.questions.ImportTemplate(c.Request().Context(), scope, principal, c.QueryParam("format"))
	if err != nil {
		return importError(err)
	}
	c.Response().Header().Set(echo.HeaderContentDisposition,
		`attachment; filename="`+template.FileName+`"`)
	return c.Blob(http.StatusOK, template.ContentType, template.Data)
}

// importUpload reads the multipart file under a hard cap and never trusts the client's content type.
func importUpload(c echo.Context) ([]byte, string, error) {
	c.Request().Body = http.MaxBytesReader(c.Response(), c.Request().Body, maxImportUpload)
	file, err := c.FormFile("file")
	if err != nil {
		return nil, "", importUploadError(err)
	}
	opened, err := file.Open()
	if err != nil {
		return nil, "", importUploadError(err)
	}
	defer func() { _ = opened.Close() }()

	data, err := io.ReadAll(opened)
	if err != nil {
		return nil, "", importUploadError(err)
	}
	return data, file.Filename, nil
}

func importUploadError(err error) error {
	var tooLarge *http.MaxBytesError
	if errors.As(err, &tooLarge) {
		return httpx.New(http.StatusRequestEntityTooLarge, httpx.CodeTooLarge, importSizeRule)
	}
	return httpx.Validation(incomplete, map[string]string{"file": importMissingFile})
}

// importSelection reads the JSON row numbers of the confirmed set.
func importSelection(raw string) ([]int, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, httpx.Validation(incomplete, map[string]string{"rows": importRowsRule})
	}
	var rows []int
	if err := json.Unmarshal([]byte(raw), &rows); err != nil || len(rows) == 0 {
		return nil, httpx.Validation(incomplete, map[string]string{"rows": importRowsRule})
	}
	return rows, nil
}

func importPreviewResponseOf(preview model.ImportPreview) importPreviewResponse {
	out := importPreviewResponse{
		FileName: preview.FileName, Format: preview.Format,
		Total: preview.Total, Valid: preview.Valid, Invalid: preview.Invalid,
		Rows: make([]importRowResponse, 0, len(preview.Rows)),
	}
	for _, row := range preview.Rows {
		item := importRowResponse{
			Row: row.Row, Valid: row.Valid, Errors: make([]importRowErrorResponse, 0, len(row.Errors)),
		}
		for _, failure := range row.Errors {
			item.Errors = append(item.Errors, importRowErrorResponse{Field: failure.Field, Message: importReason(failure.Reason)})
		}
		if row.Preview != nil {
			item.Preview = &importRowPreviewResponse{
				Type: row.Preview.Type, Content: row.Preview.Content, Program: row.Preview.Program,
				Subject: row.Preview.Subject, Difficulty: row.Preview.Difficulty,
			}
		}
		out.Rows = append(out.Rows, item)
	}
	return out
}

func importError(err error) error {
	var file *service.ImportFileError
	if errors.As(err, &file) {
		return httpx.Validation(importFileRule, map[string]string{"file": importReason(file.Reason)})
	}
	var rows *service.ImportRowsError
	if errors.As(err, &rows) {
		return httpx.Validation(importRowsRule, map[string]string{"rows": importRowsMessage(rows.Rows)})
	}
	var input *service.InputError
	if errors.As(err, &input) {
		fields := make(map[string]string, len(input.Fields))
		for field, reason := range input.Fields {
			fields[field] = importReason(reason)
		}
		return httpx.Validation(incomplete, fields)
	}
	if errors.Is(err, service.ErrTenantNotFound) {
		return httpx.NotFound(tenantNotFound)
	}
	return err
}

func importRowsMessage(rows []int) string {
	names := make([]string, 0, len(rows))
	for _, row := range rows {
		names = append(names, strconv.Itoa(row))
	}
	return "Baris " + strings.Join(names, ", ") + " tidak valid."
}

func importReason(reason string) string {
	switch reason {
	case service.ReasonImportFile:
		return importFileRule
	case service.ReasonImportEmpty:
		return importEmptyRule
	case service.ReasonImportRows:
		return importRowsRule
	case service.ReasonImportHeader:
		return importHeaderRule
	case service.ReasonImportSize:
		return importSizeRule
	case service.ReasonImportFormat:
		return importFormatRule
	case service.ReasonImportBool:
		return importBoolRule
	}
	return questionReason(reason)
}
