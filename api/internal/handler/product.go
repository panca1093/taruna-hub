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
	productNotFound       = "Produk tidak ditemukan."
	productReferenced     = "Produk masih dipakai konten lain."
	productTitleRule      = "Judul 1–200 karakter dan belum dipakai produk lain."
	productDescription    = "Deskripsi maksimal 4000 karakter."
	productTypeRule       = "Tipe produk harus individual atau bundle."
	productProgram        = "Program tidak ditemukan."
	productPriceRule      = "Harga harus angka lebih dari 0 dengan maksimal 2 desimal."
	productOriginalRule   = "Harga coret harus angka dan tidak lebih kecil dari harga."
	productTryoutsRule    = "Produk individual butuh 1 tryout dan bundle 2–20 tryout."
	productTryoutRule     = "Tryout tidak ditemukan atau belum dipublikasikan pada program ini."
	productListRule       = "Batas 1–100."
	productCatalogMissing = "Katalog tidak ditemukan."
)

type productTryoutResponse struct {
	ID            uuid.UUID `json:"id"`
	Title         string    `json:"title"`
	Status        string    `json:"status"`
	SubtestCount  int       `json:"subtest_count"`
	QuestionCount int       `json:"question_count"`
}

type productResponse struct {
	ID            uuid.UUID               `json:"id"`
	Type          string                  `json:"type"`
	Title         string                  `json:"title"`
	Description   string                  `json:"description"`
	ProgramID     uuid.UUID               `json:"program_id"`
	Price         string                  `json:"price"`
	OriginalPrice *string                 `json:"original_price"`
	IsActive      bool                    `json:"is_active"`
	ActiveUntil   *time.Time              `json:"active_until"`
	Tryouts       []productTryoutResponse `json:"tryouts"`
	CreatedAt     time.Time               `json:"created_at"`
	UpdatedAt     time.Time               `json:"updated_at"`
}

type productsResponse struct {
	Products []productResponse `json:"products"`
	Total    int               `json:"total"`
}

type publicTryoutResponse struct {
	ID                   uuid.UUID `json:"id"`
	Title                string    `json:"title"`
	SubtestCount         int       `json:"subtest_count"`
	QuestionCount        int       `json:"question_count"`
	DurationSecondsTotal int       `json:"duration_seconds_total"`
}

type publicProductResponse struct {
	ID              uuid.UUID              `json:"id"`
	Type            string                 `json:"type"`
	Title           string                 `json:"title"`
	Description     string                 `json:"description"`
	ProgramID       uuid.UUID              `json:"program_id"`
	ProgramCode     string                 `json:"program_code"`
	ProgramName     string                 `json:"program_name"`
	Price           string                 `json:"price"`
	OriginalPrice   *string                `json:"original_price"`
	DiscountPercent *int                   `json:"discount_percent"`
	ActiveUntil     *time.Time             `json:"active_until"`
	Tryouts         []publicTryoutResponse `json:"tryouts"`
}

type publicProductsResponse struct {
	Products []publicProductResponse `json:"products"`
	Total    int                     `json:"total"`
}

type catalogSummaryResponse struct {
	Programs int `json:"programs"`
	Products int `json:"products"`
	Tryouts  int `json:"tryouts"`
}

type productRequest struct {
	Type          string   `json:"type"`
	Title         string   `json:"title"`
	Description   string   `json:"description"`
	ProgramID     string   `json:"program_id"`
	Price         string   `json:"price"`
	OriginalPrice *string  `json:"original_price"`
	IsActive      bool     `json:"is_active"`
	ActiveUntil   *string  `json:"active_until"`
	TryoutIDs     []string `json:"tryout_ids"`
}

func (h *Handler) ListProducts(c echo.Context) error {
	scope, principal, ok := curriculumCaller(c)
	if !ok {
		return httpx.Unauthorized(sessionInvalid)
	}
	filter, err := productFilterOf(c)
	if err != nil {
		return err
	}
	products, total, err := h.products.List(c.Request().Context(), scope, principal, filter)
	if err != nil {
		return productError(err)
	}
	body := productsResponse{Products: make([]productResponse, 0, len(products)), Total: total}
	for _, product := range products {
		body.Products = append(body.Products, productResponseOf(product))
	}
	return c.JSON(http.StatusOK, body)
}

func (h *Handler) GetProduct(c echo.Context) error {
	scope, principal, ok := curriculumCaller(c)
	if !ok {
		return httpx.Unauthorized(sessionInvalid)
	}
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return httpx.NotFound(productNotFound)
	}
	product, err := h.products.Get(c.Request().Context(), scope, principal, id)
	if err != nil {
		return productError(err)
	}
	return c.JSON(http.StatusOK, productResponseOf(product))
}

// The body is capped before it is mapped, so an oversized package is one error, not thousands.
const maxProductBody = 64 << 10

func bindProduct(c echo.Context, dst any) error {
	c.Request().Body = http.MaxBytesReader(c.Response(), c.Request().Body, maxProductBody)
	return bindStrict(c, dst)
}

func (h *Handler) CreateProduct(c echo.Context) error {
	scope, principal, ok := curriculumCaller(c)
	if !ok {
		return httpx.Unauthorized(sessionInvalid)
	}
	var req productRequest
	if err := bindProduct(c, &req); err != nil {
		return err
	}
	input, err := productInputOf(req)
	if err != nil {
		return err
	}
	created, err := h.products.Create(c.Request().Context(), scope, principal, input)
	if err != nil {
		return productError(err)
	}
	return c.JSON(http.StatusCreated, productResponseOf(created))
}

func (h *Handler) UpdateProduct(c echo.Context) error {
	scope, principal, ok := curriculumCaller(c)
	if !ok {
		return httpx.Unauthorized(sessionInvalid)
	}
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return httpx.NotFound(productNotFound)
	}
	var req productRequest
	if err := bindProduct(c, &req); err != nil {
		return err
	}
	input, err := productInputOf(req)
	if err != nil {
		return err
	}
	updated, err := h.products.Update(c.Request().Context(), scope, principal, id, input)
	if err != nil {
		return productError(err)
	}
	return c.JSON(http.StatusOK, productResponseOf(updated))
}

func (h *Handler) DeleteProduct(c echo.Context) error {
	scope, principal, ok := curriculumCaller(c)
	if !ok {
		return httpx.Unauthorized(sessionInvalid)
	}
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return httpx.NotFound(productNotFound)
	}
	if err := h.products.Delete(c.Request().Context(), scope, principal, id); err != nil {
		return productError(err)
	}
	return c.NoContent(http.StatusNoContent)
}

// The catalog is anonymous: the tenant comes from configuration and nothing else.
func (h *Handler) CatalogProducts(c echo.Context) error {
	scope, ok := hostScope(c)
	if !ok {
		return httpx.NotFound(productCatalogMissing)
	}
	filter, err := productFilterOf(c)
	if err != nil {
		return err
	}
	products, total, err := h.products.CatalogProducts(c.Request().Context(), scope, filter)
	if err != nil {
		return productError(err)
	}
	body := publicProductsResponse{Products: make([]publicProductResponse, 0, len(products)), Total: total}
	for _, product := range products {
		body.Products = append(body.Products, publicProductOf(product))
	}
	return c.JSON(http.StatusOK, body)
}

func (h *Handler) CatalogProduct(c echo.Context) error {
	scope, ok := hostScope(c)
	if !ok {
		return httpx.NotFound(productCatalogMissing)
	}
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return httpx.NotFound(productNotFound)
	}
	product, err := h.products.CatalogProduct(c.Request().Context(), scope, id)
	if err != nil {
		return productError(err)
	}
	return c.JSON(http.StatusOK, publicProductOf(product))
}

func (h *Handler) CatalogSummary(c echo.Context) error {
	scope, ok := hostScope(c)
	if !ok {
		return httpx.NotFound(productCatalogMissing)
	}
	summary, err := h.products.CatalogSummary(c.Request().Context(), scope)
	if err != nil {
		return productError(err)
	}
	return c.JSON(http.StatusOK, catalogSummaryResponse{
		Programs: summary.Programs, Products: summary.Products, Tryouts: summary.Tryouts,
	})
}

func productFilterOf(c echo.Context) (service.ProductFilter, error) {
	filter := service.ProductFilter{}
	fields := map[string]string{}
	if raw := c.QueryParam("program_id"); raw != "" {
		id, err := uuid.Parse(raw)
		if err != nil {
			fields["program_id"] = invalidFormat
		} else {
			filter.ProgramID = &id
		}
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
	filter.Type = c.QueryParam("type")
	filter.Status = c.QueryParam("status")
	filter.Search = c.QueryParam("q")
	if len(fields) > 0 {
		return service.ProductFilter{}, httpx.Validation(incomplete, fields)
	}
	return filter, nil
}

func productInputOf(req productRequest) (service.ProductInput, error) {
	fields := map[string]string{}
	input := service.ProductInput{
		Type: req.Type, Title: req.Title, Description: req.Description,
		Price: req.Price, OriginalPrice: req.OriginalPrice, IsActive: req.IsActive,
	}
	if req.ProgramID == "" {
		fields["program_id"] = invalidFormat
	} else if id, err := uuid.Parse(req.ProgramID); err != nil {
		fields["program_id"] = invalidFormat
	} else {
		input.ProgramID = id
	}
	if req.ActiveUntil != nil && strings.TrimSpace(*req.ActiveUntil) != "" {
		parsed, err := time.Parse(time.RFC3339, strings.TrimSpace(*req.ActiveUntil))
		if err != nil {
			fields["active_until"] = invalidFormat
		} else {
			input.ActiveUntil = &parsed
		}
	}
	for i, raw := range req.TryoutIDs {
		id, err := uuid.Parse(strings.TrimSpace(raw))
		if err != nil {
			fields[fmt.Sprintf("tryout_ids[%d]", i)] = invalidFormat
			continue
		}
		input.TryoutIDs = append(input.TryoutIDs, id)
	}
	if len(fields) > 0 {
		return service.ProductInput{}, httpx.Validation(incomplete, fields)
	}
	return input, nil
}

func productResponseOf(product model.Product) productResponse {
	out := productResponse{
		ID: product.ID, Type: product.Type, Title: product.Title, Description: product.Description,
		ProgramID: product.ProgramID, Price: product.Price, OriginalPrice: product.OriginalPrice,
		IsActive: product.IsActive, ActiveUntil: product.ActiveUntil,
		Tryouts:   make([]productTryoutResponse, 0, len(product.Tryouts)),
		CreatedAt: product.CreatedAt, UpdatedAt: product.UpdatedAt,
	}
	for _, tryout := range product.Tryouts {
		out.Tryouts = append(out.Tryouts, productTryoutResponse{
			ID: tryout.ID, Title: tryout.Title, Status: tryout.Status,
			SubtestCount: tryout.SubtestCount, QuestionCount: tryout.QuestionCount,
		})
	}
	return out
}

func publicProductOf(product model.PublicProduct) publicProductResponse {
	out := publicProductResponse{
		ID: product.ID, Type: product.Type, Title: product.Title, Description: product.Description,
		ProgramID: product.ProgramID, ProgramCode: product.ProgramCode, ProgramName: product.ProgramName,
		Price: product.Price, OriginalPrice: product.OriginalPrice,
		DiscountPercent: product.DiscountPercent, ActiveUntil: product.ActiveUntil,
		Tryouts: make([]publicTryoutResponse, 0, len(product.Tryouts)),
	}
	for _, tryout := range product.Tryouts {
		out.Tryouts = append(out.Tryouts, publicTryoutResponse{
			ID: tryout.ID, Title: tryout.Title, SubtestCount: tryout.SubtestCount,
			QuestionCount: tryout.QuestionCount, DurationSecondsTotal: tryout.DurationSecondsTotal,
		})
	}
	return out
}

func productError(err error) error {
	var input *service.InputError
	if errors.As(err, &input) {
		fields := make(map[string]string, len(input.Fields))
		for path, reason := range input.Fields {
			fields[path] = productReason(reason)
		}
		return httpx.Validation(incomplete, fields)
	}
	switch {
	case errors.Is(err, service.ErrProductNotFound):
		return httpx.NotFound(productNotFound)
	case errors.Is(err, service.ErrTenantNotFound):
		return httpx.NotFound(tenantNotFound)
	case errors.Is(err, service.ErrProductReferenced):
		return httpx.Conflict(productReferenced)
	}
	return err
}

func productReason(reason string) string {
	switch reason {
	case service.ReasonProductTitle:
		return productTitleRule
	case service.ReasonProductDescription:
		return productDescription
	case service.ReasonProductType:
		return productTypeRule
	case service.ReasonProductProgram:
		return productProgram
	case service.ReasonProductPrice:
		return productPriceRule
	case service.ReasonProductOriginalPrice:
		return productOriginalRule
	case service.ReasonProductTryouts:
		return productTryoutsRule
	case service.ReasonProductTryout:
		return productTryoutRule
	case service.ReasonProductQuery:
		return productListRule
	}
	return invalidFormat
}
