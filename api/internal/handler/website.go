package handler

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"reflect"
	"strconv"
	"strings"

	"github.com/labstack/echo/v4"

	"github.com/tarunaeduka/bimbel-tryout/api/internal/httpx"
	"github.com/tarunaeduka/bimbel-tryout/api/internal/model"
	"github.com/tarunaeduka/bimbel-tryout/api/internal/service"
)

const (
	codeWebsiteNotPublished httpx.Code = "website_not_published"
	codeWebsiteConflict     httpx.Code = "website_revision_conflict"
	codeWebsiteInvalid      httpx.Code = "website_content_invalid"

	websiteMaxBody = 64 << 10

	websiteNotPublished = "Website belum memiliki versi terbit."
	websiteConflict     = "Versi website sudah berubah. Muat ulang sebelum melanjutkan."
	websiteInvalid      = "Isi website belum sesuai ketentuan."
	websiteBodyUnread   = "Isi permintaan tidak dapat dibaca."
	websiteBodyTooLarge = "Isi permintaan maksimal 64 KiB."
)

type websiteSaveRequest struct {
	ExpectedDraftRevision int64                `json:"expected_draft_revision"`
	Content               model.WebsiteContent `json:"content"`
}

type websitePublishRequest struct {
	ExpectedDraftRevision     int64 `json:"expected_draft_revision"`
	ExpectedPublishedRevision int64 `json:"expected_published_revision"`
}

// WithWebsite attaches the website CMS service for the same reason as WithTopups.
func (h *Handler) WithWebsite(s *service.WebsiteService) *Handler {
	h.website = s
	return h
}

func (h *Handler) Website(c echo.Context) error {
	scope, ok := hostScope(c)
	if !ok {
		return httpx.NotFound(tenantNotFound)
	}
	published, err := h.website.Published(c.Request().Context(), scope)
	if err != nil {
		return websiteError(err)
	}
	return c.JSON(http.StatusOK, published)
}

func (h *Handler) WebsiteWorkbench(c echo.Context) error {
	scope, principal, ok := walletCaller(c)
	if !ok {
		return httpx.Unauthorized(sessionInvalid)
	}
	workbench, err := h.website.Workbench(c.Request().Context(), scope, principal)
	if err != nil {
		return websiteError(err)
	}
	return c.JSON(http.StatusOK, workbench)
}

func (h *Handler) SaveWebsite(c echo.Context) error {
	scope, principal, ok := walletCaller(c)
	if !ok {
		return httpx.Unauthorized(sessionInvalid)
	}
	var req websiteSaveRequest
	body, err := websiteBody(c, &req)
	if err != nil {
		return err
	}
	workbench, err := h.website.Save(c.Request().Context(), scope, principal,
		service.SaveWebsiteInput{ExpectedDraftRevision: req.ExpectedDraftRevision, Content: req.Content, MissingFields: missingOptionalFields(body)})
	if err != nil {
		return websiteError(err)
	}
	return c.JSON(http.StatusOK, workbench)
}

func (h *Handler) PublishWebsite(c echo.Context) error {
	scope, principal, ok := walletCaller(c)
	if !ok {
		return httpx.Unauthorized(sessionInvalid)
	}
	var req websitePublishRequest
	if _, err := websiteBody(c, &req); err != nil {
		return err
	}
	publication, err := h.website.Publish(c.Request().Context(), scope, principal,
		service.PublishWebsiteInput{ExpectedDraftRevision: req.ExpectedDraftRevision, ExpectedPublishedRevision: req.ExpectedPublishedRevision})
	if err != nil {
		return websiteError(err)
	}
	return c.JSON(http.StatusOK, publication)
}

func (h *Handler) WebsiteHistory(c echo.Context) error {
	scope, principal, ok := walletCaller(c)
	if !ok {
		return httpx.Unauthorized(sessionInvalid)
	}
	in := service.WebsiteHistoryInput{Cursor: c.QueryParam("cursor")}
	if raw := c.QueryParam("limit"); raw != "" {
		limit, err := strconv.Atoi(raw)
		if err != nil || limit < 1 {
			return httpx.Validation(incomplete, map[string]string{"limit": websiteReason(service.ReasonWebsiteLimit)})
		}
		in.Limit = limit
	}
	list, err := h.website.History(c.Request().Context(), scope, principal, in)
	if err != nil {
		return websiteError(err)
	}
	return c.JSON(http.StatusOK, list)
}

// websiteBody reads at most 64 KiB and refuses unknown fields and trailing data; the caller gets the raw bytes too.
func websiteBody(c echo.Context, dst any) ([]byte, error) {
	body, err := io.ReadAll(http.MaxBytesReader(c.Response(), c.Request().Body, websiteMaxBody))
	var tooLarge *http.MaxBytesError
	if errors.As(err, &tooLarge) {
		return nil, httpx.New(http.StatusRequestEntityTooLarge, httpx.CodeTooLarge, websiteBodyTooLarge)
	}
	if err != nil {
		return nil, httpx.Validation(websiteBodyUnread, map[string]string{"body": invalidFormat})
	}
	dec := json.NewDecoder(bytes.NewReader(body))
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil || dec.Decode(&struct{}{}) != io.EOF || !exactKeys(body, reflect.TypeOf(dst).Elem()) {
		return nil, httpx.Validation(websiteBodyUnread, map[string]string{"body": invalidFormat})
	}
	return body, nil
}

// exactKeys rejects object keys that only match a struct field case-insensitively, which encoding/json accepts.
func exactKeys(raw []byte, t reflect.Type) bool {
	switch t.Kind() {
	case reflect.Struct:
		var obj map[string]json.RawMessage
		if json.Unmarshal(raw, &obj) != nil {
			return true
		}
		tags := map[string]reflect.Type{}
		for i := 0; i < t.NumField(); i++ {
			tags[strings.Split(t.Field(i).Tag.Get("json"), ",")[0]] = t.Field(i).Type
		}
		for key, value := range obj {
			ft, ok := tags[key]
			if !ok || !exactKeys(value, ft) {
				return false
			}
		}
	case reflect.Map:
		var obj map[string]json.RawMessage
		if json.Unmarshal(raw, &obj) != nil {
			return true
		}
		for _, value := range obj {
			if !exactKeys(value, t.Elem()) {
				return false
			}
		}
	}
	return true
}

// missingOptionalFields finds optional program texts that are absent or null; Go decoding cannot tell them from empty.
func missingOptionalFields(body []byte) map[string]string {
	var shape struct {
		Content struct {
			Locales map[string]struct {
				Programs map[string]map[string]json.RawMessage `json:"programs"`
			} `json:"locales"`
		} `json:"content"`
	}
	fields := map[string]string{}
	if json.Unmarshal(body, &shape) != nil {
		return fields
	}
	for locale, l := range shape.Content.Locales {
		for slot, program := range l.Programs {
			for _, field := range []string{"schedule", "location", "fees", "contact"} {
				if raw, ok := program[field]; !ok || string(raw) == "null" {
					fields["locales."+locale+".programs."+slot+"."+field] = websiteReason(service.ReasonWebsiteRequired)
				}
			}
		}
	}
	return fields
}

func websiteError(err error) error {
	var input *service.InputError
	if errors.As(err, &input) {
		fields := make(map[string]string, len(input.Fields))
		query := false
		for path, reason := range input.Fields {
			fields[path] = websiteReason(reason)
			query = query || !strings.HasPrefix(path, "locales.") && path != "schema_version"
		}
		if query {
			return httpx.Validation(incomplete, fields)
		}
		return httpx.New(http.StatusUnprocessableEntity, codeWebsiteInvalid, websiteInvalid).WithFields(fields)
	}
	switch {
	case errors.Is(err, service.ErrWebsiteNotPublished):
		return httpx.New(http.StatusNotFound, codeWebsiteNotPublished, websiteNotPublished)
	case errors.Is(err, service.ErrWebsiteConflict):
		return httpx.New(http.StatusConflict, codeWebsiteConflict, websiteConflict)
	case errors.Is(err, service.ErrWebsiteForbidden):
		return httpx.Forbidden(roleDenied)
	case errors.Is(err, service.ErrTenantNotFound):
		return httpx.NotFound(tenantNotFound)
	}
	return err
}

func websiteReason(reason string) string {
	switch reason {
	case service.ReasonWebsiteRequired:
		return "Wajib diisi."
	case service.ReasonWebsiteTooLong:
		return "Melebihi batas karakter."
	case service.ReasonWebsiteSchema:
		return "Versi skema tidak didukung."
	case service.ReasonWebsiteLocale:
		return "Bahasa tidak didukung."
	case service.ReasonWebsiteSlot:
		return "Slot program tidak dikenal."
	case service.ReasonWebsiteRevision:
		return "Nomor revisi tidak valid."
	case service.ReasonWebsiteLimit:
		return "Batas halaman harus antara 1 dan 50."
	case service.ReasonWebsiteCursor:
		return "Kursor tidak valid."
	}
	return invalidFormat
}
