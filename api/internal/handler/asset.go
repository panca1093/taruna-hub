package handler

import (
	"errors"
	"io"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"

	"github.com/tarunaeduka/bimbel-tryout/api/internal/httpx"
	"github.com/tarunaeduka/bimbel-tryout/api/internal/model"
	"github.com/tarunaeduka/bimbel-tryout/api/internal/service"
)

const (
	assetNotFound     = "Berkas tidak ditemukan."
	assetSizeRule     = "Ukuran berkas maksimal 5 MB."
	assetMissingRule  = "Berkas wajib diunggah."
	assetTypeRule     = "Berkas harus gambar PNG, JPEG, atau WebP."
	assetCorruptRule  = "Berkas gambar rusak."
	assetDimsRule     = "Dimensi gambar maksimal 8000 x 8000 piksel."
	assetPurposeRule  = "Tujuan unggahan harus question_image atau payment_proof."
	assetUnavailable  = "Penyimpanan berkas sedang tidak tersedia."
	assetFileField    = "file"
	assetPurposeField = "purpose"
)

// maxAssetBody leaves room for the multipart framing; the 5 MB rule is on the file itself.
const maxAssetBody = service.MaxAssetBytes + 64<<10

type assetResponse struct {
	ID          uuid.UUID `json:"id"`
	Purpose     string    `json:"purpose"`
	ContentType string    `json:"content_type"`
	SizeBytes   int64     `json:"size_bytes"`
	Status      string    `json:"status"`
}

type assetURLResponse struct {
	URL       string    `json:"url"`
	ExpiresAt time.Time `json:"expires_at"`
}

func assetResponseOf(a model.Asset) assetResponse {
	return assetResponse{ID: a.ID, Purpose: a.Purpose, ContentType: a.ContentType, SizeBytes: a.SizeBytes, Status: a.Status}
}

// UploadAsset reads only the file bytes and the purpose; the client's file name and every other field are ignored.
func (h *Handler) UploadAsset(c echo.Context) error {
	scope, principal, ok := walletCaller(c)
	if !ok {
		return httpx.Unauthorized(sessionInvalid)
	}
	c.Request().Body = http.MaxBytesReader(c.Response(), c.Request().Body, maxAssetBody)
	file, err := c.FormFile(assetFileField)
	if err != nil {
		return assetUploadError(err)
	}
	opened, err := file.Open()
	if err != nil {
		return assetUploadError(err)
	}
	defer func() { _ = opened.Close() }()
	data, err := io.ReadAll(io.LimitReader(opened, service.MaxAssetBytes+1))
	if err != nil {
		return assetUploadError(err)
	}
	asset, err := h.assets.Upload(c.Request().Context(), scope, principal, c.FormValue(assetPurposeField), data)
	if err != nil {
		return assetError(err)
	}
	return c.JSON(http.StatusCreated, assetResponseOf(asset))
}

func (h *Handler) FinalizeAsset(c echo.Context) error {
	scope, principal, ok := walletCaller(c)
	if !ok {
		return httpx.Unauthorized(sessionInvalid)
	}
	id, err := assetID(c)
	if err != nil {
		return err
	}
	asset, err := h.assets.Finalize(c.Request().Context(), scope, principal, id)
	if err != nil {
		return assetError(err)
	}
	return c.JSON(http.StatusOK, assetResponseOf(asset))
}

func (h *Handler) AssetURL(c echo.Context) error {
	scope, principal, ok := walletCaller(c)
	if !ok {
		return httpx.Unauthorized(sessionInvalid)
	}
	id, err := assetID(c)
	if err != nil {
		return err
	}
	link, err := h.assets.DownloadURL(c.Request().Context(), scope, principal, id)
	if err != nil {
		return assetError(err)
	}
	c.Response().Header().Set(echo.HeaderCacheControl, "private, no-store")
	return c.JSON(http.StatusOK, assetURLResponse{URL: link.URL, ExpiresAt: link.ExpiresAt})
}

// assetID is a 404 for anything that is not a uuid, never a 400 that would confirm a shape.
func assetID(c echo.Context) (uuid.UUID, error) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return uuid.Nil, httpx.NotFound(assetNotFound)
	}
	return id, nil
}

func assetUploadError(err error) error {
	var tooLarge *http.MaxBytesError
	if errors.As(err, &tooLarge) {
		return httpx.New(http.StatusRequestEntityTooLarge, httpx.CodeTooLarge, assetSizeRule)
	}
	return httpx.Validation(incomplete, map[string]string{assetFileField: assetMissingRule})
}

func assetError(err error) error {
	var input *service.InputError
	if errors.As(err, &input) {
		fields := make(map[string]string, len(input.Fields))
		for field, reason := range input.Fields {
			fields[field] = assetReason(reason)
		}
		return httpx.Validation(incomplete, fields)
	}
	switch {
	case errors.Is(err, service.ErrAssetNotFound):
		return httpx.NotFound(assetNotFound)
	case errors.Is(err, service.ErrTenantNotFound):
		return httpx.NotFound(tenantNotFound)
	case errors.Is(err, service.ErrAssetForbidden):
		return httpx.Forbidden(roleDenied)
	case errors.Is(err, service.ErrAssetTooLarge):
		return httpx.New(http.StatusRequestEntityTooLarge, httpx.CodeTooLarge, assetSizeRule)
	case errors.Is(err, service.ErrStorageUnavailable):
		return httpx.Unavailable(assetUnavailable)
	}
	return err
}

func assetReason(reason string) string {
	switch reason {
	case service.ReasonAssetType:
		return assetTypeRule
	case service.ReasonAssetCorrupt:
		return assetCorruptRule
	case service.ReasonAssetDimensions:
		return assetDimsRule
	case service.ReasonAssetPurpose:
		return assetPurposeRule
	}
	return reason
}
