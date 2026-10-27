package service

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/tarunaeduka/bimbel-tryout/api/internal/model"
	"github.com/tarunaeduka/bimbel-tryout/api/internal/repository"
)

var (
	ErrProductNotFound   = errors.New("product not found")
	ErrProductReferenced = errors.New("product still referenced")
)

// Product field reasons; the transport turns each into the copy its screens show.
const (
	ReasonProductTitle         = "product_title"
	ReasonProductDescription   = "product_description"
	ReasonProductType          = "product_type"
	ReasonProductProgram       = "product_program"
	ReasonProductPrice         = "product_price"
	ReasonProductOriginalPrice = "product_original_price"
	ReasonProductTryouts       = "product_tryouts"
	ReasonProductTryout        = "product_tryout"
	ReasonProductQuery         = "product_query"
)

const (
	maxProductTitleLen     = 200
	maxProductDescription  = 4000
	maxProductTryouts      = 20
	defaultProductLimit    = 50
	maxProductLimit        = 100
	maxProductPriceCents   = int64(999999999999)
	maxProductIntegerDigit = 10
)

type ProductInput struct {
	Type          string
	Title         string
	Description   string
	ProgramID     uuid.UUID
	Price         string
	OriginalPrice *string
	IsActive      bool
	ActiveUntil   *time.Time
	TryoutIDs     []uuid.UUID
}

type ProductFilter struct {
	ProgramID *uuid.UUID
	Type      string
	Status    string
	Search    string
	Limit     int
	Offset    int
}

type ProductOption func(*ProductService)

// WithProductClock replaces the clock; the public visibility window is time-based, so tests move time.
func WithProductClock(now func() time.Time) ProductOption {
	return func(s *ProductService) { s.now = now }
}

type ProductService struct {
	db   *pgxpool.Pool
	repo *repository.ProductRepository
	now  func() time.Time
}

func NewProductService(db *pgxpool.Pool, repo *repository.ProductRepository, opts ...ProductOption) *ProductService {
	s := &ProductService{db: db, repo: repo, now: time.Now}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

func (s *ProductService) List(ctx context.Context, scope HostScope, principal model.Principal, filter ProductFilter) ([]model.Product, int, error) {
	tenantID, err := questionTenant(scope, principal)
	if err != nil {
		return nil, 0, err
	}
	filter, err = validatedProductFilter(filter)
	if err != nil {
		return nil, 0, err
	}
	products, total, err := s.repo.List(ctx, tenantID, repository.ProductFilter{
		ProgramID: filter.ProgramID, Type: filter.Type, Status: filter.Status,
		Search: filter.Search, Limit: filter.Limit, Offset: filter.Offset,
	})
	if err != nil {
		return nil, 0, fmt.Errorf("list products: %w", err)
	}
	return products, total, nil
}

func (s *ProductService) Get(ctx context.Context, scope HostScope, principal model.Principal, id uuid.UUID) (model.Product, error) {
	tenantID, err := questionTenant(scope, principal)
	if err != nil {
		return model.Product{}, err
	}
	product, err := s.repo.Find(ctx, tenantID, id)
	if err != nil {
		return model.Product{}, productError(err)
	}
	return product, nil
}

func (s *ProductService) Create(ctx context.Context, scope HostScope, principal model.Principal, input ProductInput) (model.Product, error) {
	tenantID, err := questionTenant(scope, principal)
	if err != nil {
		return model.Product{}, err
	}
	input, err = s.checked(ctx, tenantID, uuid.Nil, input)
	if err != nil {
		return model.Product{}, err
	}

	var out model.Product
	err = s.inTx(ctx, func(tx pgx.Tx) error {
		created, err := s.repo.CreateTx(ctx, tx, tenantID, principal.UserID, productOf(input))
		if err != nil {
			return productError(err)
		}
		if err := s.checkActivation(ctx, tx, tenantID, created.ID, input.IsActive); err != nil {
			return err
		}
		out = created
		return nil
	})
	if err != nil {
		return model.Product{}, err
	}
	return out, nil
}

func (s *ProductService) Update(ctx context.Context, scope HostScope, principal model.Principal, id uuid.UUID, input ProductInput) (model.Product, error) {
	tenantID, err := questionTenant(scope, principal)
	if err != nil {
		return model.Product{}, err
	}
	input, err = s.checked(ctx, tenantID, id, input)
	if err != nil {
		return model.Product{}, err
	}

	var out model.Product
	err = s.inTx(ctx, func(tx pgx.Tx) error {
		if _, err := s.repo.LockTx(ctx, tx, tenantID, id); err != nil {
			return productError(err)
		}
		stored := productOf(input)
		if err := s.repo.UpdateHeadTx(ctx, tx, tenantID, id, stored); err != nil {
			return productError(err)
		}
		if err := s.repo.ReplaceTryoutsTx(ctx, tx, tenantID, id, stored.Tryouts); err != nil {
			return productError(err)
		}
		if err := s.checkActivation(ctx, tx, tenantID, id, input.IsActive); err != nil {
			return err
		}
		out, err = s.repo.FindTx(ctx, tx, tenantID, id)
		return productError(err)
	})
	if err != nil {
		return model.Product{}, err
	}
	return out, nil
}

func (s *ProductService) Delete(ctx context.Context, scope HostScope, principal model.Principal, id uuid.UUID) error {
	tenantID, err := questionTenant(scope, principal)
	if err != nil {
		return err
	}
	return s.inTx(ctx, func(tx pgx.Tx) error {
		if _, err := s.repo.LockTx(ctx, tx, tenantID, id); err != nil {
			return productError(err)
		}
		return productError(s.repo.DeleteTx(ctx, tx, tenantID, id))
	})
}

func (s *ProductService) CatalogProducts(ctx context.Context, scope HostScope, filter ProductFilter) ([]model.PublicProduct, int, error) {
	tenantID, err := publicTenant(scope)
	if err != nil {
		return nil, 0, err
	}
	filter, err = validatedProductFilter(filter)
	if err != nil {
		return nil, 0, err
	}
	products, total, err := s.repo.PublicList(ctx, tenantID, s.now(), repository.ProductFilter{
		ProgramID: filter.ProgramID, Type: filter.Type, Search: filter.Search,
		Limit: filter.Limit, Offset: filter.Offset,
	})
	if err != nil {
		return nil, 0, fmt.Errorf("list catalog: %w", err)
	}
	for i := range products {
		products[i].DiscountPercent = discountPercent(products[i].Price, products[i].OriginalPrice)
	}
	return products, total, nil
}

func (s *ProductService) CatalogProduct(ctx context.Context, scope HostScope, id uuid.UUID) (model.PublicProduct, error) {
	tenantID, err := publicTenant(scope)
	if err != nil {
		return model.PublicProduct{}, err
	}
	product, err := s.repo.PublicFind(ctx, tenantID, s.now(), id)
	if err != nil {
		return model.PublicProduct{}, productError(err)
	}
	product.DiscountPercent = discountPercent(product.Price, product.OriginalPrice)
	return product, nil
}

func (s *ProductService) CatalogSummary(ctx context.Context, scope HostScope) (model.CatalogSummary, error) {
	tenantID, err := publicTenant(scope)
	if err != nil {
		return model.CatalogSummary{}, err
	}
	return s.repo.PublicSummary(ctx, tenantID, s.now())
}

// checked applies every scalar rule, then the program, the title and the package.
func (s *ProductService) checked(ctx context.Context, tenantID, id uuid.UUID, input ProductInput) (ProductInput, error) {
	input, err := productScalars(input)
	if err != nil {
		return ProductInput{}, err
	}
	ok, err := s.repo.ProgramExists(ctx, tenantID, input.ProgramID)
	if err != nil {
		return ProductInput{}, err
	}
	if !ok {
		return ProductInput{}, &InputError{Fields: map[string]string{"program_id": ReasonProductProgram}}
	}
	taken, err := s.repo.TitleTaken(ctx, tenantID, input.Title, id)
	if err != nil {
		return ProductInput{}, err
	}
	if taken {
		return ProductInput{}, &InputError{Fields: map[string]string{"title": ReasonProductTitle}}
	}
	attached, err := s.repo.TryoutsForAttach(ctx, s.db, tenantID, input.TryoutIDs)
	if err != nil {
		return ProductInput{}, err
	}
	fields := map[string]string{}
	for i, tryoutID := range input.TryoutIDs {
		attach, found := attached[tryoutID]
		// A foreign, missing, draft or other-program tryout answers the same path and reason.
		if !found || attach.Status != model.TryoutPublished || attach.ProgramID != input.ProgramID {
			fields[fmt.Sprintf("tryout_ids[%d]", i)] = ReasonProductTryout
		}
	}
	if len(fields) > 0 {
		return ProductInput{}, &InputError{Fields: fields}
	}
	return input, nil
}

// checkActivation re-reads the written links, so a concurrent unpublish cannot slip through.
func (s *ProductService) checkActivation(ctx context.Context, tx pgx.Tx, tenantID, id uuid.UUID, isActive bool) error {
	if !isActive {
		return nil
	}
	unpublished, err := s.repo.UnpublishedAttached(ctx, tx, tenantID, id)
	if err != nil {
		return err
	}
	if unpublished > 0 {
		return &InputError{Fields: map[string]string{"tryout_ids": ReasonProductTryouts}}
	}
	return nil
}

func productScalars(input ProductInput) (ProductInput, error) {
	fields := map[string]string{}
	input.Title = strings.TrimSpace(input.Title)
	input.Description = strings.TrimSpace(input.Description)

	if n := utf8.RuneCountInString(input.Title); n < 1 || n > maxProductTitleLen {
		fields["title"] = ReasonProductTitle
	}
	if utf8.RuneCountInString(input.Description) > maxProductDescription {
		fields["description"] = ReasonProductDescription
	}
	if input.Type != model.ProductIndividual && input.Type != model.ProductBundle {
		fields["type"] = ReasonProductType
	}
	if input.ProgramID == uuid.Nil {
		fields["program_id"] = ReasonProductProgram
	}

	price, priceOK := parseMoney(input.Price)
	if !priceOK {
		fields["price"] = ReasonProductPrice
	}
	input.Price = price
	if input.OriginalPrice != nil {
		original, ok := parseMoney(*input.OriginalPrice)
		switch {
		case !ok:
			fields["original_price"] = ReasonProductOriginalPrice
		case priceOK && moneyCents(original) < moneyCents(price):
			fields["original_price"] = ReasonProductOriginalPrice
		default:
			input.OriginalPrice = &original
		}
	}

	seen := map[uuid.UUID]bool{}
	for i, tryoutID := range input.TryoutIDs {
		if tryoutID == uuid.Nil {
			fields[fmt.Sprintf("tryout_ids[%d]", i)] = ReasonProductTryout
			continue
		}
		if seen[tryoutID] {
			fields[fmt.Sprintf("tryout_ids[%d]", i)] = ReasonProductTryout
			continue
		}
		seen[tryoutID] = true
	}
	switch input.Type {
	case model.ProductIndividual:
		if len(input.TryoutIDs) != 1 {
			fields["tryout_ids"] = ReasonProductTryouts
		}
	case model.ProductBundle:
		if len(input.TryoutIDs) < 2 || len(input.TryoutIDs) > maxProductTryouts {
			fields["tryout_ids"] = ReasonProductTryouts
		}
	}
	if len(fields) > 0 {
		return ProductInput{}, &InputError{Fields: fields}
	}
	return input, nil
}

// parseMoney accepts a plain decimal with at most two fraction digits and returns it canonical.
func parseMoney(raw string) (string, bool) {
	if raw == "" || raw != strings.TrimSpace(raw) || strings.HasSuffix(raw, ".") {
		return "", false
	}
	whole, fraction, _ := strings.Cut(raw, ".")
	if whole == "" || len(whole) > maxProductIntegerDigit || strings.Contains(fraction, ".") || len(fraction) > 2 {
		return "", false
	}
	digits := whole + fraction
	for _, c := range digits {
		if c < '0' || c > '9' {
			return "", false
		}
	}
	cents, ok := decimalCents(whole, fraction)
	if !ok || cents <= 0 || cents > maxProductPriceCents {
		return "", false
	}
	return fmt.Sprintf("%d.%02d", cents/100, cents%100), true
}

func decimalCents(whole, fraction string) (int64, bool) {
	units, err := strconv.ParseInt(whole, 10, 64)
	if err != nil {
		return 0, false
	}
	cents := int64(0)
	if fraction != "" {
		padded := fraction + strings.Repeat("0", 2-len(fraction))
		if cents, err = strconv.ParseInt(padded, 10, 64); err != nil {
			return 0, false
		}
	}
	return units*100 + cents, true
}

// moneyCents reads back a canonical value the server itself produced.
func moneyCents(value string) int64 {
	whole, fraction, _ := strings.Cut(value, ".")
	cents, _ := decimalCents(whole, fraction)
	return cents
}

// discountPercent is integer arithmetic on cents, so no float ever touches money.
func discountPercent(price string, original *string) *int {
	if original == nil {
		return nil
	}
	cents, normal := moneyCents(price), moneyCents(*original)
	if normal <= cents {
		return nil
	}
	percent := int((normal-cents)*100*2+normal) / (2 * int(normal))
	return &percent
}

func validatedProductFilter(filter ProductFilter) (ProductFilter, error) {
	fields := map[string]string{}
	if filter.Limit == 0 {
		filter.Limit = defaultProductLimit
	}
	if filter.Limit < 1 || filter.Limit > maxProductLimit {
		fields["limit"] = ReasonProductQuery
	}
	if filter.Offset < 0 {
		fields["offset"] = ReasonProductQuery
	}
	if filter.Type != "" && filter.Type != model.ProductIndividual && filter.Type != model.ProductBundle {
		fields["type"] = ReasonProductQuery
	}
	if filter.Status != "" && filter.Status != "active" && filter.Status != "inactive" {
		fields["status"] = ReasonProductQuery
	}
	if len(fields) > 0 {
		return ProductFilter{}, &InputError{Fields: fields}
	}
	return filter, nil
}

func productOf(input ProductInput) model.Product {
	out := model.Product{
		Type: input.Type, Title: input.Title, Description: input.Description,
		ProgramID: input.ProgramID, Price: input.Price, OriginalPrice: input.OriginalPrice,
		IsActive: input.IsActive, ActiveUntil: input.ActiveUntil,
	}
	for _, id := range input.TryoutIDs {
		out.Tryouts = append(out.Tryouts, model.ProductTryout{ID: id})
	}
	return out
}

func publicTenant(scope HostScope) (uuid.UUID, error) {
	if scope.Tenant.ID == uuid.Nil {
		return uuid.Nil, ErrTenantNotFound
	}
	return scope.Tenant.ID, nil
}

func productError(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, repository.ErrNotFound):
		return ErrProductNotFound
	case errors.Is(err, repository.ErrDuplicateTitle):
		return &InputError{Fields: map[string]string{"title": ReasonProductTitle}}
	case errors.Is(err, repository.ErrReferenced):
		return ErrProductReferenced
	case errors.Is(err, repository.ErrScopeRequired):
		return ErrTenantNotFound
	}
	return err
}

func (s *ProductService) inTx(ctx context.Context, fn func(pgx.Tx) error) error {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin: %w", err)
	}
	defer func() { _ = tx.Rollback(context.WithoutCancel(ctx)) }()
	if err := fn(tx); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit: %w", err)
	}
	return nil
}
