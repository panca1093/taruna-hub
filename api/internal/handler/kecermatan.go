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
	kecNotFound     = "Tes kecermatan tidak ditemukan."
	kecConflict     = "Tes kecermatan sudah diubah orang lain, muat ulang."
	kecLocked       = "Tes kecermatan sudah dikerjakan siswa dan tidak bisa diubah."
	kecTitleRule    = "Judul 1–200 karakter."
	kecDescRule     = "Deskripsi maksimal 2000 karakter."
	kecProgram      = "Program tidak ditemukan."
	kecItemType     = "Tipe item harus char, number, symbol, emoji, image, shape, atau mix."
	kecItemLength   = "Panjang item 1–5."
	kecOptionsRule  = "Jumlah opsi per soal 3–8."
	kecSoalRule     = "Soal per kolom 50–100."
	kecKolomRange   = "Jumlah kolom 10–50."
	kecKolomOdd     = "Jumlah kolom harus genap."
	kecDurationRule = "Durasi kolom 1–3600 detik."
	kecCoverRule    = "Warna sampul harus berformat #rrggbb."
	kecSourcesRule  = "Sumber elemen tidak sesuai dengan tipe item."
	kecPackageRule  = "Paket item tidak ditemukan."
	kecPackageType  = "Tipe paket tidak sesuai dengan tipe item."
	kecPackageDup   = "Paket yang sama tidak boleh dipilih dua kali."
	kecPoolRule     = "Jumlah elemen kurang dari opsi per soal × panjang item."
	kecRevision     = "Versi tes wajib diisi."
	kecQueryRule    = "Batas 1–100."
	kecPreviewRule  = "Kolom di luar jumlah kolom tes, atau seed bukan 32 karakter heksadesimal."
)

type kecermatanSummaryResponse struct {
	ID                   uuid.UUID `json:"id"`
	ProgramID            uuid.UUID `json:"program_id"`
	Title                string    `json:"title"`
	Description          string    `json:"description"`
	ItemType             string    `json:"item_type"`
	ItemLength           int       `json:"item_length"`
	OptionsPerSoal       int       `json:"options_per_soal"`
	SoalPerKolom         int       `json:"soal_per_kolom"`
	KolomCount           int       `json:"kolom_count"`
	KolomDurationSeconds int       `json:"kolom_duration_seconds"`
	BuiltinSources       []string  `json:"builtin_sources"`
	CoverFrom            string    `json:"cover_from"`
	CoverTo              string    `json:"cover_to"`
	Revision             int       `json:"revision"`
	Locked               bool      `json:"locked"`
	CreatedAt            time.Time `json:"created_at"`
	UpdatedAt            time.Time `json:"updated_at"`
}

type kecermatanPackageResponse struct {
	PackageID    uuid.UUID `json:"package_id"`
	Name         string    `json:"name"`
	Type         string    `json:"type"`
	Revision     int       `json:"revision"`
	ElementCount int       `json:"element_count"`
}

type kecermatanResponse struct {
	kecermatanSummaryResponse
	Packages []kecermatanPackageResponse `json:"packages"`
}

type kecermatanListResponse struct {
	Tests []kecermatanSummaryResponse `json:"tests"`
	Total int                         `json:"total"`
}

type kecermatanElementResponse struct {
	Kind    string     `json:"kind"`
	Value   string     `json:"value"`
	AssetID *uuid.UUID `json:"asset_id,omitempty"`
}

type kecermatanItemResponse struct {
	Label    string                      `json:"label"`
	Elements []kecermatanElementResponse `json:"elements"`
}

type kecermatanSoalResponse struct {
	Shown   []string `json:"shown"`
	Missing string   `json:"missing"`
}

type kecermatanPreviewResponse struct {
	Seed        string                   `json:"seed"`
	Kolom       int                      `json:"kolom"`
	SourceKolom int                      `json:"source_kolom"`
	Reference   []kecermatanItemResponse `json:"reference"`
	Soal        []kecermatanSoalResponse `json:"soal"`
}

type kecermatanRequest struct {
	ProgramID            string   `json:"program_id"`
	Title                string   `json:"title"`
	Description          string   `json:"description"`
	ItemType             string   `json:"item_type"`
	ItemLength           int      `json:"item_length"`
	OptionsPerSoal       int      `json:"options_per_soal"`
	SoalPerKolom         int      `json:"soal_per_kolom"`
	KolomCount           int      `json:"kolom_count"`
	KolomDurationSeconds int      `json:"kolom_duration_seconds"`
	BuiltinSources       []string `json:"builtin_sources"`
	PackageIDs           []string `json:"package_ids"`
	CoverFrom            string   `json:"cover_from"`
	CoverTo              string   `json:"cover_to"`
}

type updateKecermatanRequest struct {
	kecermatanRequest
	BaseRevision int `json:"base_revision"`
}

type kecermatanPreviewRequest struct {
	Kolom int    `json:"kolom"`
	Seed  string `json:"seed"`
}

const maxKecermatanBody = 64 << 10

func bindKecermatan(c echo.Context, dst any) error {
	c.Request().Body = http.MaxBytesReader(c.Response(), c.Request().Body, maxKecermatanBody)
	return bindStrict(c, dst)
}

func (h *Handler) ListKecermatanTests(c echo.Context) error {
	scope, principal, ok := curriculumCaller(c)
	if !ok {
		return httpx.Unauthorized(sessionInvalid)
	}
	filter := service.KecermatanFilter{Search: c.QueryParam("q")}
	fields := map[string]string{}
	if raw := c.QueryParam("program_id"); raw != "" {
		if id, err := uuid.Parse(raw); err != nil {
			fields["program_id"] = invalidFormat
		} else {
			filter.ProgramID = &id
		}
	}
	for name, dst := range map[string]*int{"limit": &filter.Limit, "offset": &filter.Offset} {
		if raw := c.QueryParam(name); raw != "" {
			value, err := strconv.Atoi(raw)
			if err != nil {
				fields[name] = invalidFormat
				continue
			}
			*dst = value
		}
	}
	if len(fields) > 0 {
		return httpx.Validation(incomplete, fields)
	}
	tests, total, err := h.kecermatan.List(c.Request().Context(), scope, principal, filter)
	if err != nil {
		return kecermatanHTTPError(err)
	}
	body := kecermatanListResponse{Tests: make([]kecermatanSummaryResponse, 0, len(tests)), Total: total}
	for _, test := range tests {
		body.Tests = append(body.Tests, kecermatanSummaryOf(test))
	}
	return c.JSON(http.StatusOK, body)
}

func (h *Handler) GetKecermatanTest(c echo.Context) error {
	scope, principal, id, err := kecermatanTarget(c)
	if err != nil {
		return err
	}
	test, err := h.kecermatan.Get(c.Request().Context(), scope, principal, id)
	if err != nil {
		return kecermatanHTTPError(err)
	}
	return c.JSON(http.StatusOK, kecermatanResponseOf(test))
}

func (h *Handler) CreateKecermatanTest(c echo.Context) error {
	scope, principal, ok := curriculumCaller(c)
	if !ok {
		return httpx.Unauthorized(sessionInvalid)
	}
	var req kecermatanRequest
	if err := bindKecermatan(c, &req); err != nil {
		return err
	}
	input, err := kecermatanInputOf(req)
	if err != nil {
		return err
	}
	created, err := h.kecermatan.Create(c.Request().Context(), scope, principal, input)
	if err != nil {
		return kecermatanHTTPError(err)
	}
	return c.JSON(http.StatusCreated, kecermatanResponseOf(created))
}

func (h *Handler) UpdateKecermatanTest(c echo.Context) error {
	scope, principal, id, err := kecermatanTarget(c)
	if err != nil {
		return err
	}
	var req updateKecermatanRequest
	if err := bindKecermatan(c, &req); err != nil {
		return err
	}
	input, err := kecermatanInputOf(req.kecermatanRequest)
	if err != nil {
		return err
	}
	input.BaseRevision = req.BaseRevision
	updated, err := h.kecermatan.Update(c.Request().Context(), scope, principal, id, input)
	if err != nil {
		return kecermatanHTTPError(err)
	}
	return c.JSON(http.StatusOK, kecermatanResponseOf(updated))
}

func (h *Handler) DeleteKecermatanTest(c echo.Context) error {
	scope, principal, id, err := kecermatanTarget(c)
	if err != nil {
		return err
	}
	if err := h.kecermatan.Delete(c.Request().Context(), scope, principal, id); err != nil {
		return kecermatanHTTPError(err)
	}
	return c.NoContent(http.StatusNoContent)
}

func (h *Handler) PreviewKecermatanTest(c echo.Context) error {
	scope, principal, id, err := kecermatanTarget(c)
	if err != nil {
		return err
	}
	var req kecermatanPreviewRequest
	if err := bindKecermatan(c, &req); err != nil {
		return err
	}
	preview, err := h.kecermatan.Preview(c.Request().Context(), scope, principal, id, req.Kolom, strings.TrimSpace(req.Seed))
	if err != nil {
		return kecermatanHTTPError(err)
	}
	return c.JSON(http.StatusOK, kecermatanPreviewOf(preview))
}

func kecermatanTarget(c echo.Context) (service.HostScope, model.Principal, uuid.UUID, error) {
	scope, principal, ok := curriculumCaller(c)
	if !ok {
		return service.HostScope{}, model.Principal{}, uuid.Nil, httpx.Unauthorized(sessionInvalid)
	}
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return service.HostScope{}, model.Principal{}, uuid.Nil, httpx.NotFound(kecNotFound)
	}
	return scope, principal, id, nil
}

func kecermatanInputOf(req kecermatanRequest) (service.KecermatanInput, error) {
	fields := map[string]string{}
	input := service.KecermatanInput{
		Title: req.Title, Description: req.Description, ItemType: req.ItemType, ItemLength: req.ItemLength,
		OptionsPerSoal: req.OptionsPerSoal, SoalPerKolom: req.SoalPerKolom, KolomCount: req.KolomCount,
		KolomDurationSeconds: req.KolomDurationSeconds, BuiltinSources: req.BuiltinSources,
		CoverFrom: req.CoverFrom, CoverTo: req.CoverTo,
	}
	if req.ProgramID != "" {
		if id, err := uuid.Parse(req.ProgramID); err != nil {
			fields["program_id"] = invalidFormat
		} else {
			input.ProgramID = id
		}
	}
	for i, raw := range req.PackageIDs {
		id, err := uuid.Parse(strings.TrimSpace(raw))
		if err != nil {
			fields[fmt.Sprintf("package_ids[%d]", i)] = invalidFormat
			continue
		}
		input.PackageIDs = append(input.PackageIDs, id)
	}
	if len(fields) > 0 {
		return service.KecermatanInput{}, httpx.Validation(incomplete, fields)
	}
	return input, nil
}

func kecermatanSummaryOf(test model.KecermatanTest) kecermatanSummaryResponse {
	builtins := test.BuiltinSources
	if builtins == nil {
		builtins = []string{}
	}
	return kecermatanSummaryResponse{
		ID: test.ID, ProgramID: test.ProgramID, Title: test.Title, Description: test.Description,
		ItemType: test.ItemType, ItemLength: test.ItemLength, OptionsPerSoal: test.OptionsPerSoal,
		SoalPerKolom: test.SoalPerKolom, KolomCount: test.KolomCount, KolomDurationSeconds: test.KolomDurationSeconds,
		BuiltinSources: builtins, CoverFrom: test.CoverFrom, CoverTo: test.CoverTo,
		Revision: test.Revision, Locked: test.Locked, CreatedAt: test.CreatedAt, UpdatedAt: test.UpdatedAt,
	}
}

func kecermatanResponseOf(test model.KecermatanTest) kecermatanResponse {
	out := kecermatanResponse{
		kecermatanSummaryResponse: kecermatanSummaryOf(test),
		Packages:                  make([]kecermatanPackageResponse, 0, len(test.Packages)),
	}
	for _, ref := range test.Packages {
		out.Packages = append(out.Packages, kecermatanPackageResponse{
			PackageID: ref.PackageID, Name: ref.Name, Type: ref.Type, Revision: ref.Revision, ElementCount: ref.ElementCount,
		})
	}
	return out
}

func kecermatanPreviewOf(preview service.KecermatanPreview) kecermatanPreviewResponse {
	out := kecermatanPreviewResponse{
		Seed: preview.Seed, Kolom: preview.Kolom, SourceKolom: preview.SourceKolom,
		Reference: make([]kecermatanItemResponse, 0, len(preview.Column.Reference)),
		Soal:      make([]kecermatanSoalResponse, 0, len(preview.Column.Soal)),
	}
	for _, item := range preview.Column.Reference {
		resp := kecermatanItemResponse{Label: item.Label, Elements: make([]kecermatanElementResponse, 0, len(item.Elements))}
		for _, e := range item.Elements {
			resp.Elements = append(resp.Elements, kecermatanElementResponse{Kind: e.Kind, Value: e.Value, AssetID: e.AssetID})
		}
		out.Reference = append(out.Reference, resp)
	}
	for _, soal := range preview.Column.Soal {
		out.Soal = append(out.Soal, kecermatanSoalResponse{Shown: soal.Shown, Missing: soal.Missing})
	}
	return out
}

func kecermatanHTTPError(err error) error {
	var input *service.InputError
	if errors.As(err, &input) {
		fields := make(map[string]string, len(input.Fields))
		for path, reason := range input.Fields {
			fields[path] = kecermatanReason(reason)
		}
		return httpx.Validation(incomplete, fields)
	}
	switch {
	case errors.Is(err, service.ErrKecermatanNotFound):
		return httpx.NotFound(kecNotFound)
	case errors.Is(err, service.ErrTenantNotFound):
		return httpx.NotFound(tenantNotFound)
	case errors.Is(err, service.ErrKecermatanPool):
		return httpx.Validation(incomplete, map[string]string{"pool": kecPoolRule})
	case errors.Is(err, service.ErrKecermatanLocked):
		return httpx.Conflict(kecLocked)
	case errors.Is(err, service.ErrKecermatanStale):
		return httpx.Conflict(kecConflict)
	}
	return err
}

func kecermatanReason(reason string) string {
	switch reason {
	case service.ReasonKecTitle:
		return kecTitleRule
	case service.ReasonKecDescription:
		return kecDescRule
	case service.ReasonKecProgram:
		return kecProgram
	case service.ReasonKecItemType:
		return kecItemType
	case service.ReasonKecItemLength:
		return kecItemLength
	case service.ReasonKecOptions:
		return kecOptionsRule
	case service.ReasonKecSoal:
		return kecSoalRule
	case service.ReasonKecKolomRange:
		return kecKolomRange
	case service.ReasonKecKolomOdd:
		return kecKolomOdd
	case service.ReasonKecDuration:
		return kecDurationRule
	case service.ReasonKecCover:
		return kecCoverRule
	case service.ReasonKecSources:
		return kecSourcesRule
	case service.ReasonKecPackage:
		return kecPackageRule
	case service.ReasonKecPackageType:
		return kecPackageType
	case service.ReasonKecPackageDup:
		return kecPackageDup
	case service.ReasonKecPool:
		return kecPoolRule
	case service.ReasonKecRevision:
		return kecRevision
	case service.ReasonKecQuery:
		return kecQueryRule
	case service.ReasonKecPreview:
		return kecPreviewRule
	}
	return invalidFormat
}
