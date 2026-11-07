package handler

import (
	"net/http"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"

	"github.com/tarunaeduka/bimbel-tryout/api/internal/httpx"
	"github.com/tarunaeduka/bimbel-tryout/api/internal/model"
)

const (
	CodePDFUnavailable httpx.Code = "pdf_unavailable"

	pdfUnavailable = "Unduhan PDF sedang tidak tersedia."
)

// WithPDF records whether a converter is configured; without one a request could never succeed, so it is refused.
func (h *Handler) WithPDF(configured bool) *Handler {
	h.pdfConfigured = configured
	return h
}

// RequestTryoutPDF is the tryout PDF create endpoint. The tryout is read through the caller's tenant first, so a
// foreign or unknown id is the same 404 as everywhere else.
func (h *Handler) RequestTryoutPDF(c echo.Context) error {
	if !h.pdfConfigured {
		return httpx.New(http.StatusServiceUnavailable, CodePDFUnavailable, pdfUnavailable)
	}
	scope, principal, ok := curriculumCaller(c)
	if !ok {
		return httpx.Unauthorized(sessionInvalid)
	}
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return httpx.NotFound(tryoutNotFound)
	}
	params, err := h.tryouts.PreparePDF(c.Request().Context(), scope, principal, id)
	if err != nil {
		return tryoutError(err)
	}
	return h.acceptExport(c, model.ExportTryoutPDF, params)
}
