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
	CodeExportNotReady httpx.Code = "export_not_ready"

	exportNotFound    = "Ekspor tidak ditemukan."
	exportNotReady    = "Ekspor belum siap diunduh."
	exportKindRule    = "Jenis ekspor tidak dikenal."
	exportLimitRule   = "Batas harus bilangan bulat 1 sampai 50."
	exportUnavailable = "Ekspor sedang tidak tersedia."

	exportDefaultLimit = 20
	exportMaxLimit     = 50
)

type exportResponse struct {
	ID          uuid.UUID       `json:"id"`
	Kind        string          `json:"kind"`
	Status      string          `json:"status"`
	Params      json.RawMessage `json:"params"`
	RequestedBy uuid.UUID       `json:"requested_by"`
	CutOffAt    time.Time       `json:"cut_off_at"`
	FileName    *string         `json:"file_name"`
	ContentType *string         `json:"content_type"`
	SizeBytes   *int64          `json:"size_bytes"`
	ErrorCode   string          `json:"error_code"`
	CreatedAt   time.Time       `json:"created_at"`
	FinishedAt  *time.Time      `json:"finished_at"`
}

type exportListResponse struct {
	Exports []exportResponse `json:"exports"`
}

func exportResponseOf(e model.Export) exportResponse {
	return exportResponse{
		ID: e.ID, Kind: e.Kind, Status: e.Status, Params: e.Params, RequestedBy: e.RequestedBy, CutOffAt: e.CutOffAt,
		FileName: e.FileName, ContentType: e.ContentType, SizeBytes: e.SizeBytes, ErrorCode: e.ErrorCode,
		CreatedAt: e.CreatedAt, FinishedAt: e.FinishedAt,
	}
}

// WithExports attaches the export service for the same reason as WithTopups.
func (h *Handler) WithExports(s *service.ExportService) *Handler {
	h.exports = s
	return h
}

// acceptExport is what a kind's create endpoint calls once it has validated its own params: it records the
// request and answers 202 with the export the client then polls.
func (h *Handler) acceptExport(c echo.Context, kind string, params any) error {
	if h.exports == nil {
		return httpx.Unavailable(exportUnavailable)
	}
	scope, principal, ok := walletCaller(c)
	if !ok {
		return httpx.Unauthorized(sessionInvalid)
	}
	created, err := h.exports.Request(c.Request().Context(), scope, principal, kind, params)
	if err != nil {
		return exportError(err)
	}
	return c.JSON(http.StatusAccepted, exportResponseOf(created))
}

func (h *Handler) ListExports(c echo.Context) error {
	if h.exports == nil {
		return httpx.Unavailable(exportUnavailable)
	}
	scope, principal, ok := walletCaller(c)
	if !ok {
		return httpx.Unauthorized(sessionInvalid)
	}
	limit := exportDefaultLimit
	if raw := c.QueryParam("limit"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 1 || n > exportMaxLimit {
			return httpx.Validation(incomplete, map[string]string{"limit": exportLimitRule})
		}
		limit = n
	}
	exports, err := h.exports.List(c.Request().Context(), scope, principal, c.QueryParam("kind"), limit)
	if err != nil {
		return exportError(err)
	}
	body := exportListResponse{Exports: make([]exportResponse, 0, len(exports))}
	for _, e := range exports {
		body.Exports = append(body.Exports, exportResponseOf(e))
	}
	return c.JSON(http.StatusOK, body)
}

func (h *Handler) GetExport(c echo.Context) error {
	if h.exports == nil {
		return httpx.Unavailable(exportUnavailable)
	}
	scope, principal, ok := walletCaller(c)
	if !ok {
		return httpx.Unauthorized(sessionInvalid)
	}
	id, err := exportID(c)
	if err != nil {
		return err
	}
	e, err := h.exports.Get(c.Request().Context(), scope, principal, id)
	if err != nil {
		return exportError(err)
	}
	return c.JSON(http.StatusOK, exportResponseOf(e))
}

func (h *Handler) ExportDownloadLink(c echo.Context) error {
	if h.exports == nil {
		return httpx.Unavailable(exportUnavailable)
	}
	scope, principal, ok := walletCaller(c)
	if !ok {
		return httpx.Unauthorized(sessionInvalid)
	}
	id, err := exportID(c)
	if err != nil {
		return err
	}
	link, err := h.exports.DownloadLink(c.Request().Context(), scope, principal, id)
	if err != nil {
		return exportError(err)
	}
	return c.JSON(http.StatusOK, assetURLResponse{URL: link.URL, ExpiresAt: link.ExpiresAt})
}

// exportID is a 404 for anything that is not a uuid, never a 400 that would confirm a shape.
func exportID(c echo.Context) (uuid.UUID, error) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return uuid.Nil, httpx.NotFound(exportNotFound)
	}
	return id, nil
}

func exportError(err error) error {
	var input *service.InputError
	if errors.As(err, &input) {
		fields := make(map[string]string, len(input.Fields))
		for field, reason := range input.Fields {
			if reason == service.ReasonExportKind {
				reason = exportKindRule
			}
			fields[field] = reason
		}
		return httpx.Validation(incomplete, fields)
	}
	switch {
	case errors.Is(err, service.ErrExportNotFound):
		return httpx.NotFound(exportNotFound)
	case errors.Is(err, service.ErrTenantNotFound):
		return httpx.NotFound(tenantNotFound)
	case errors.Is(err, service.ErrExportForbidden):
		return httpx.Forbidden(roleDenied)
	case errors.Is(err, service.ErrExportNotReady):
		return httpx.New(http.StatusConflict, CodeExportNotReady, exportNotReady)
	case errors.Is(err, service.ErrStorageUnavailable):
		return httpx.Unavailable(assetUnavailable)
	}
	return err
}
