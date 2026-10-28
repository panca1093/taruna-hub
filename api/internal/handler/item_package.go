package handler

import (
	"errors"
	"io"
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
	codePackageInUse    httpx.Code = "package_in_use"
	codePackageTooSmall httpx.Code = "package_too_small"
)

const (
	packageNotFound  = "Paket item tidak ditemukan."
	elementNotFound  = "Elemen tidak ditemukan di paket ini."
	packageNameTaken = "Nama paket sudah dipakai."
	packageInUse     = "Paket dipakai oleh tes dan tidak dapat dihapus."
	packageTooSmall  = "Elemen tidak dapat dihapus: jumlahnya akan kurang dari kebutuhan tes yang memakai paket ini."
	packageTypeRule  = "Tipe paket harus symbol, image, emoji, atau shape."
	packageNameRule  = "Nama paket 1–100 karakter."
	elementsEmpty    = "Tidak ada elemen yang dikirim."
	elementsKindRule = "Isian ini tidak sesuai dengan tipe paket."
	uploadTooMany    = "Maksimal 200 berkas per unggahan."
	packageFullRule  = "Paket sudah penuh (maksimal 500 elemen)."
	sampleParamsRule = "Panjang item 1–5, opsi minimal 2, dan soal per kolom 1–100."
	poolTooSmallRule = "Jumlah elemen kurang dari opsi per soal x panjang item."
	revisionRule     = "Revisi tidak ada untuk paket ini."
	symbolFileRule   = "File simbol harus .csv, .txt, atau .xlsx satu kolom."
	symbolSizeRule   = "Ukuran file simbol maksimal 5 MB."
	symbolRowsRule   = "Maksimal 1000 simbol per unggahan."
	symbolEmptyRule  = "File tidak berisi simbol."
	packageSizeRule  = "Ukuran unggahan maksimal 64 MB."
	templateFormat   = "Format template harus xlsx atau csv."
)

// maxElementsBody is the 10 MB ZIP ceiling over the 5 MB images, with room for the multipart framing; the
// per-file limits are enforced on the content by the service.
const maxElementsBody = 64 << 20

type itemPackageResponse struct {
	ID            uuid.UUID `json:"id"`
	Type          string    `json:"type"`
	Name          string    `json:"name"`
	Revision      int       `json:"revision"`
	ElementCount  int       `json:"element_count"`
	RequiredCount int       `json:"required_count"`
	InUse         bool      `json:"in_use"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

type itemPackagesResponse struct {
	Packages []itemPackageResponse `json:"packages"`
}

type itemElementResponse struct {
	ID       uuid.UUID  `json:"id"`
	Position int        `json:"position"`
	Kind     string     `json:"kind"`
	Value    string     `json:"value"`
	Label    string     `json:"label"`
	AssetID  *uuid.UUID `json:"asset_id,omitempty"`
}

type itemPackageDetailResponse struct {
	Package  itemPackageResponse   `json:"package"`
	Revision int                   `json:"revision"`
	Elements []itemElementResponse `json:"elements"`
}

type itemElementCheckResponse struct {
	Label    string `json:"label"`
	Kind     string `json:"kind"`
	Accepted bool   `json:"accepted"`
	Reason   string `json:"reason,omitempty"`
	Message  string `json:"message,omitempty"`
}

type itemElementPreviewResponse struct {
	Accepted int                        `json:"accepted"`
	Rejected int                        `json:"rejected"`
	Items    []itemElementCheckResponse `json:"items"`
}

type itemElementCommitResponse struct {
	Added          int `json:"added"`
	AlreadyPresent int `json:"already_present"`
	Rejected       int `json:"rejected"`
	Revision       int `json:"revision"`
}

type itemCatalogResponse struct {
	Emoji            []catalogItemResponse `json:"emoji"`
	EmojiAttribution string                `json:"emoji_attribution"`
	Shapes           []catalogItemResponse `json:"shapes"`
	SymbolPalette    []string              `json:"symbol_palette"`
}

type catalogItemResponse struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type createItemPackageRequest struct {
	Type string `json:"type"`
	Name string `json:"name"`
}

type renameItemPackageRequest struct {
	Name string `json:"name"`
}

type sampleColumnRequest struct {
	ItemLength     int `json:"item_length"`
	OptionsPerSoal int `json:"options_per_soal"`
	SoalPerKolom   int `json:"soal_per_kolom"`
}

type sampleItemResponse struct {
	Elements []itemElementResponse `json:"elements"`
}

type sampleSoalResponse struct {
	Items []sampleItemResponse `json:"items"`
}

type sampleColumnResponse struct {
	Soal []sampleSoalResponse `json:"soal"`
}

func itemPackageResponseOf(p model.ItemPackage) itemPackageResponse {
	return itemPackageResponse{
		ID: p.ID, Type: p.Type, Name: p.Name, Revision: p.Revision, ElementCount: p.ElementCount,
		RequiredCount: p.RequiredCount, InUse: p.InUse, CreatedAt: p.CreatedAt, UpdatedAt: p.UpdatedAt,
	}
}

func itemElementResponses(elements []model.ItemElement) []itemElementResponse {
	out := make([]itemElementResponse, 0, len(elements))
	for _, e := range elements {
		out = append(out, itemElementResponse{
			ID: e.ID, Position: e.Position, Kind: e.Kind, Value: e.Value, Label: e.Label, AssetID: e.AssetID,
		})
	}
	return out
}

func (h *Handler) ListItemPackages(c echo.Context) error {
	scope, principal, ok := curriculumCaller(c)
	if !ok {
		return httpx.Unauthorized(sessionInvalid)
	}
	packages, err := h.itemPackages.List(c.Request().Context(), scope, principal, c.QueryParam("type"))
	if err != nil {
		return itemPackageHTTPError(err)
	}
	out := itemPackagesResponse{Packages: make([]itemPackageResponse, 0, len(packages))}
	for _, p := range packages {
		out.Packages = append(out.Packages, itemPackageResponseOf(p))
	}
	return c.JSON(http.StatusOK, out)
}

func (h *Handler) CreateItemPackage(c echo.Context) error {
	scope, principal, ok := curriculumCaller(c)
	if !ok {
		return httpx.Unauthorized(sessionInvalid)
	}
	var req createItemPackageRequest
	if err := bindStrict(c, &req); err != nil {
		return err
	}
	created, err := h.itemPackages.Create(c.Request().Context(), scope, principal, req.Type, req.Name)
	if err != nil {
		return itemPackageHTTPError(err)
	}
	return c.JSON(http.StatusCreated, itemPackageResponseOf(created))
}

func (h *Handler) ItemCatalog(c echo.Context) error {
	if _, _, ok := curriculumCaller(c); !ok {
		return httpx.Unauthorized(sessionInvalid)
	}
	out := itemCatalogResponse{
		Emoji: []catalogItemResponse{}, EmojiAttribution: service.EmojiAttribution,
		Shapes: []catalogItemResponse{}, SymbolPalette: service.SymbolPalette(),
	}
	for _, e := range service.EmojiCatalog() {
		out.Emoji = append(out.Emoji, catalogItemResponse{ID: e.ID, Name: e.Name})
	}
	for _, s := range service.ShapeCatalog() {
		out.Shapes = append(out.Shapes, catalogItemResponse{ID: s.ID, Name: s.Name})
	}
	return c.JSON(http.StatusOK, out)
}

func (h *Handler) ItemSymbolTemplate(c echo.Context) error {
	if _, _, ok := curriculumCaller(c); !ok {
		return httpx.Unauthorized(sessionInvalid)
	}
	template, err := h.itemPackages.SymbolTemplate(c.QueryParam("format"))
	if err != nil {
		return itemPackageHTTPError(err)
	}
	c.Response().Header().Set(echo.HeaderContentDisposition, `attachment; filename="`+template.FileName+`"`)
	return c.Blob(http.StatusOK, template.ContentType, template.Data)
}

func (h *Handler) GetItemPackage(c echo.Context) error {
	scope, principal, ok := curriculumCaller(c)
	if !ok {
		return httpx.Unauthorized(sessionInvalid)
	}
	id, err := itemPackageID(c)
	if err != nil {
		return err
	}
	revision := 0
	if raw := c.QueryParam("revision"); raw != "" {
		revision, err = strconv.Atoi(raw)
		if err != nil || revision < 1 {
			return httpx.Validation(incomplete, map[string]string{"revision": revisionRule})
		}
	}
	detail, err := h.itemPackages.Get(c.Request().Context(), scope, principal, id, revision)
	if err != nil {
		return itemPackageHTTPError(err)
	}
	return c.JSON(http.StatusOK, itemPackageDetailResponse{
		Package: itemPackageResponseOf(detail.ItemPackage), Revision: detail.ViewRevision,
		Elements: itemElementResponses(detail.Elements),
	})
}

func (h *Handler) RenameItemPackage(c echo.Context) error {
	scope, principal, ok := curriculumCaller(c)
	if !ok {
		return httpx.Unauthorized(sessionInvalid)
	}
	id, err := itemPackageID(c)
	if err != nil {
		return err
	}
	var req renameItemPackageRequest
	if err := bindStrict(c, &req); err != nil {
		return err
	}
	renamed, err := h.itemPackages.Rename(c.Request().Context(), scope, principal, id, req.Name)
	if err != nil {
		return itemPackageHTTPError(err)
	}
	return c.JSON(http.StatusOK, itemPackageResponseOf(renamed))
}

func (h *Handler) DeleteItemPackage(c echo.Context) error {
	scope, principal, ok := curriculumCaller(c)
	if !ok {
		return httpx.Unauthorized(sessionInvalid)
	}
	id, err := itemPackageID(c)
	if err != nil {
		return err
	}
	if err := h.itemPackages.Delete(c.Request().Context(), scope, principal, id); err != nil {
		return itemPackageHTTPError(err)
	}
	return c.NoContent(http.StatusNoContent)
}

func (h *Handler) PreviewItemElements(c echo.Context) error {
	scope, principal, ok := curriculumCaller(c)
	if !ok {
		return httpx.Unauthorized(sessionInvalid)
	}
	id, err := itemPackageID(c)
	if err != nil {
		return err
	}
	in, err := elementInput(c)
	if err != nil {
		return err
	}
	preview, err := h.itemPackages.PreviewElements(c.Request().Context(), scope, principal, id, in)
	if err != nil {
		return itemPackageHTTPError(err)
	}
	out := itemElementPreviewResponse{
		Accepted: preview.Accepted, Rejected: preview.Rejected,
		Items: make([]itemElementCheckResponse, 0, len(preview.Items)),
	}
	for _, item := range preview.Items {
		out.Items = append(out.Items, itemElementCheckResponse{
			Label: item.Label, Kind: item.Kind, Accepted: item.Accepted,
			Reason: item.Reason, Message: elementReason(item.Reason),
		})
	}
	return c.JSON(http.StatusOK, out)
}

func (h *Handler) CommitItemElements(c echo.Context) error {
	scope, principal, ok := curriculumCaller(c)
	if !ok {
		return httpx.Unauthorized(sessionInvalid)
	}
	id, err := itemPackageID(c)
	if err != nil {
		return err
	}
	in, err := elementInput(c)
	if err != nil {
		return err
	}
	commit, err := h.itemPackages.AddElements(c.Request().Context(), scope, principal, id, in)
	if err != nil {
		return itemPackageHTTPError(err)
	}
	return c.JSON(http.StatusOK, itemElementCommitResponse{
		Added: commit.Added, AlreadyPresent: commit.AlreadyPresent, Rejected: commit.Rejected, Revision: commit.Revision,
	})
}

func (h *Handler) RemoveItemElement(c echo.Context) error {
	scope, principal, ok := curriculumCaller(c)
	if !ok {
		return httpx.Unauthorized(sessionInvalid)
	}
	id, err := itemPackageID(c)
	if err != nil {
		return err
	}
	elementID, err := uuid.Parse(c.Param("elementId"))
	if err != nil {
		return httpx.NotFound(elementNotFound)
	}
	pkg, err := h.itemPackages.RemoveElement(c.Request().Context(), scope, principal, id, elementID)
	if err != nil {
		return itemPackageHTTPError(err)
	}
	return c.JSON(http.StatusOK, itemPackageResponseOf(pkg))
}

func (h *Handler) SampleItemColumn(c echo.Context) error {
	scope, principal, ok := curriculumCaller(c)
	if !ok {
		return httpx.Unauthorized(sessionInvalid)
	}
	id, err := itemPackageID(c)
	if err != nil {
		return err
	}
	var req sampleColumnRequest
	if err := bindStrict(c, &req); err != nil {
		return err
	}
	column, err := h.itemPackages.SampleColumn(c.Request().Context(), scope, principal, id, service.SampleInput{
		ItemLength: req.ItemLength, OptionsPerSoal: req.OptionsPerSoal, SoalPerKolom: req.SoalPerKolom,
	})
	if err != nil {
		return itemPackageHTTPError(err)
	}
	out := sampleColumnResponse{Soal: make([]sampleSoalResponse, 0, len(column))}
	for _, soal := range column {
		item := sampleSoalResponse{Items: make([]sampleItemResponse, 0, len(soal.Items))}
		for _, elements := range soal.Items {
			item.Items = append(item.Items, sampleItemResponse{Elements: itemElementResponses(elements)})
		}
		out.Soal = append(out.Soal, item)
	}
	return c.JSON(http.StatusOK, out)
}

// itemPackageID is a 404 for anything that is not a uuid, never a 400 that would confirm a shape.
func itemPackageID(c echo.Context) (uuid.UUID, error) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return uuid.Nil, httpx.NotFound(packageNotFound)
	}
	return id, nil
}

// elementInput reads the multipart parts the service judges. Part names and bytes are all it takes: the
// client's content types and every other field are ignored.
func elementInput(c echo.Context) (service.ElementInput, error) {
	c.Request().Body = http.MaxBytesReader(c.Response(), c.Request().Body, maxElementsBody)
	form, err := c.MultipartForm()
	if err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			return service.ElementInput{}, httpx.New(http.StatusRequestEntityTooLarge, httpx.CodeTooLarge, packageSizeRule)
		}
		return service.ElementInput{}, httpx.Validation(incomplete, map[string]string{"elements": elementsEmpty})
	}
	defer func() { _ = form.RemoveAll() }()

	in := service.ElementInput{IDs: form.Value["ids"]}
	if symbols := form.Value["symbols"]; len(symbols) > 0 {
		in.Text = symbols[0]
	}
	headers := append(form.File["file"], form.File["files"]...)
	if len(headers) > 200 {
		return service.ElementInput{}, httpx.Validation(incomplete, map[string]string{"elements": uploadTooMany})
	}
	for _, header := range headers {
		opened, err := header.Open()
		if err != nil {
			return service.ElementInput{}, httpx.Validation(incomplete, map[string]string{"elements": elementsEmpty})
		}
		data, err := io.ReadAll(io.LimitReader(opened, 10<<20+1))
		_ = opened.Close()
		if err != nil {
			return service.ElementInput{}, httpx.Validation(incomplete, map[string]string{"elements": elementsEmpty})
		}
		in.Files = append(in.Files, service.UploadedFile{Name: header.Filename, Data: data})
	}
	return in, nil
}

func itemPackageHTTPError(err error) error {
	var file *service.ImportFileError
	if errors.As(err, &file) {
		return httpx.Validation(incomplete, map[string]string{"file": packageFileReason(file.Reason)})
	}
	var input *service.InputError
	if errors.As(err, &input) {
		fields := make(map[string]string, len(input.Fields))
		for field, reason := range input.Fields {
			fields[field] = packageReason(reason)
		}
		return httpx.Validation(incomplete, fields)
	}
	switch {
	case errors.Is(err, service.ErrItemPackageNotFound):
		return httpx.NotFound(packageNotFound)
	case errors.Is(err, service.ErrItemElementNotFound):
		return httpx.NotFound(elementNotFound)
	case errors.Is(err, service.ErrItemPackageNameTaken):
		return httpx.Conflict(packageNameTaken).WithFields(map[string]string{"name": packageNameTaken})
	case errors.Is(err, service.ErrItemPackageInUse):
		return httpx.New(http.StatusConflict, codePackageInUse, packageInUse)
	case errors.Is(err, service.ErrItemPackageShrink):
		return httpx.New(http.StatusConflict, codePackageTooSmall, packageTooSmall)
	case errors.Is(err, service.ErrTenantNotFound):
		return httpx.NotFound(tenantNotFound)
	case errors.Is(err, service.ErrStorageUnavailable):
		return httpx.Unavailable(assetUnavailable)
	}
	return err
}

func packageFileReason(reason string) string {
	switch reason {
	case service.ReasonImportSize:
		return symbolSizeRule
	case service.ReasonImportRows:
		return symbolRowsRule
	case service.ReasonImportEmpty:
		return symbolEmptyRule
	}
	return symbolFileRule
}

func packageReason(reason string) string {
	switch reason {
	case service.ReasonPackageType:
		return packageTypeRule
	case service.ReasonPackageName:
		return packageNameRule
	case service.ReasonElementsEmpty:
		return elementsEmpty
	case service.ReasonElementsKind:
		return elementsKindRule
	case service.ReasonUploadTooMany:
		return uploadTooMany
	case service.ReasonPackageFull:
		return packageFullRule
	case service.ReasonSampleParams:
		return sampleParamsRule
	case service.ReasonPoolTooSmall:
		return poolTooSmallRule
	case service.ReasonRevision:
		return revisionRule
	case service.ReasonImportFormat:
		return templateFormat
	}
	return assetReason(reason)
}

func elementReason(reason string) string {
	switch reason {
	case "":
		return ""
	case service.ElemDuplicate:
		return "Sudah ada di paket ini (isinya sama)."
	case service.ElemDuplicateUpload:
		return "Muncul lebih dari sekali dalam unggahan ini."
	case service.ElemSymbolEmoji:
		return "Emoji tidak boleh dipakai sebagai simbol. Buat paket Emoji dan pilih dari katalog."
	case service.ElemSymbolMulti:
		return "Satu elemen harus satu karakter. Pisahkan simbol dengan spasi atau baris baru."
	case service.ElemSymbolInvalid:
		return "Karakter ini tidak dapat dipakai sebagai simbol."
	case service.ElemImageType:
		return "Format harus PNG, JPG, atau WebP (SVG tidak diterima)."
	case service.ElemImageTooLarge:
		return "Ukuran gambar maksimal 5 MB."
	case service.ElemImageCorrupt:
		return "Berkas gambar rusak."
	case service.ElemImageDimensions:
		return "Dimensi gambar maksimal 4096 x 4096 piksel."
	case service.ElemPathTraversal:
		return "Nama berkas dalam ZIP memuat path yang tidak aman."
	case service.ElemSymlink:
		return "Tautan simbolik (symlink) dalam ZIP tidak diterima."
	case service.ElemNestedArchive:
		return "Arsip di dalam arsip tidak diterima."
	case service.ElemArchiveCorrupt:
		return "Berkas ZIP rusak atau tidak berisi gambar."
	case service.ElemArchiveTooLarge:
		return "Ukuran ZIP maksimal 10 MB."
	case service.ElemArchiveTooMany:
		return "ZIP memuat lebih dari 200 berkas."
	case service.ElemArchiveExtractBig:
		return "Total ukuran hasil ekstrak ZIP melebihi 50 MB."
	case service.ElemNotInCatalog:
		return "Tidak ada dalam katalog."
	case service.ElemPackageFull:
		return packageFullRule
	}
	return reason
}
