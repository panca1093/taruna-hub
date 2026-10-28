package service

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/tarunaeduka/bimbel-tryout/api/internal/model"
	"github.com/tarunaeduka/bimbel-tryout/api/internal/repository"
)

var (
	ErrKecermatanNotFound = errors.New("kecermatan test not found")
	ErrKecermatanStale    = errors.New("kecermatan test changed by someone else")
	ErrKecermatanLocked   = errors.New("kecermatan test already attempted")
)

// Kecermatan field reasons; the transport turns each into the copy the builder shows.
const (
	ReasonKecTitle       = "kecermatan_title"
	ReasonKecDescription = "kecermatan_description"
	ReasonKecProgram     = "kecermatan_program"
	ReasonKecItemType    = "kecermatan_item_type"
	ReasonKecItemLength  = "kecermatan_item_length"
	ReasonKecOptions     = "kecermatan_options"
	ReasonKecSoal        = "kecermatan_soal"
	ReasonKecKolomRange  = "kecermatan_kolom_range"
	ReasonKecKolomOdd    = "kecermatan_kolom_odd"
	ReasonKecDuration    = "kecermatan_duration"
	ReasonKecCover       = "kecermatan_cover"
	ReasonKecSources     = "kecermatan_sources"
	ReasonKecPackage     = "kecermatan_package"
	ReasonKecPackageType = "kecermatan_package_type"
	ReasonKecPackageDup  = "kecermatan_package_dup"
	ReasonKecPool        = "kecermatan_pool"
	ReasonKecRevision    = "kecermatan_base_revision"
	ReasonKecQuery       = "kecermatan_query"
	ReasonKecPreview     = "kecermatan_preview"
)

const (
	maxKecermatanTitleLen = 200
	maxKecermatanDesc     = 2000
	minKolomCount         = 10
	maxKolomCount         = 50
	minSoalPerKolom       = 50
	maxSoalPerKolom       = 100
	minOptionsPerSoal     = 3
	maxOptionsPerSoal     = 8
	maxKolomSeconds       = 3600
	maxKecermatanPackages = 10
	defaultKecermatanList = 50
	maxKecermatanList     = 100
)

var coverColor = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)

type KecermatanInput struct {
	ProgramID            uuid.UUID
	Title                string
	Description          string
	ItemType             string
	ItemLength           int
	OptionsPerSoal       int
	SoalPerKolom         int
	KolomCount           int
	KolomDurationSeconds int
	BuiltinSources       []string
	PackageIDs           []uuid.UUID
	CoverFrom            string
	CoverTo              string
	BaseRevision         int
}

type KecermatanFilter struct {
	ProgramID *uuid.UUID
	Search    string
	Limit     int
	Offset    int
}

// KecermatanPreview is one generated column for staff: nothing of it is stored.
type KecermatanPreview struct {
	Seed        string
	Kolom       int
	SourceKolom int
	Column      KecColumn
}

type KecermatanService struct {
	db       *pgxpool.Pool
	repo     *repository.KecermatanRepository
	packages *repository.ItemPackageRepository
}

func NewKecermatanService(db *pgxpool.Pool, repo *repository.KecermatanRepository, packages *repository.ItemPackageRepository) *KecermatanService {
	return &KecermatanService{db: db, repo: repo, packages: packages}
}

func (s *KecermatanService) List(ctx context.Context, scope HostScope, principal model.Principal, filter KecermatanFilter) ([]model.KecermatanTest, int, error) {
	tenantID, err := questionTenant(scope, principal)
	if err != nil {
		return nil, 0, err
	}
	fields := map[string]string{}
	if filter.Limit == 0 {
		filter.Limit = defaultKecermatanList
	}
	if filter.Limit < 1 || filter.Limit > maxKecermatanList {
		fields["limit"] = ReasonKecQuery
	}
	if filter.Offset < 0 {
		fields["offset"] = ReasonKecQuery
	}
	if len(fields) > 0 {
		return nil, 0, &InputError{Fields: fields}
	}
	tests, total, err := s.repo.List(ctx, tenantID, repository.KecermatanFilter{
		ProgramID: filter.ProgramID, Search: filter.Search, Limit: filter.Limit, Offset: filter.Offset,
	})
	if err != nil {
		return nil, 0, fmt.Errorf("list kecermatan tests: %w", err)
	}
	return tests, total, nil
}

func (s *KecermatanService) Get(ctx context.Context, scope HostScope, principal model.Principal, id uuid.UUID) (model.KecermatanTest, error) {
	tenantID, err := questionTenant(scope, principal)
	if err != nil {
		return model.KecermatanTest{}, err
	}
	test, err := s.repo.Find(ctx, s.db, tenantID, id)
	if err != nil {
		return model.KecermatanTest{}, kecermatanError(err)
	}
	return test, nil
}

func (s *KecermatanService) Create(ctx context.Context, scope HostScope, principal model.Principal, input KecermatanInput) (model.KecermatanTest, error) {
	tenantID, err := questionTenant(scope, principal)
	if err != nil {
		return model.KecermatanTest{}, err
	}
	input, err = kecermatanScalars(input)
	if err != nil {
		return model.KecermatanTest{}, err
	}
	if err := s.checkProgram(ctx, tenantID, input.ProgramID); err != nil {
		return model.KecermatanTest{}, err
	}
	seed, err := newKecermatanSeed()
	if err != nil {
		return model.KecermatanTest{}, err
	}
	var out model.KecermatanTest
	err = s.inTx(ctx, func(tx pgx.Tx) error {
		test, pins, err := s.pin(ctx, tx, tenantID, input)
		if err != nil {
			return err
		}
		test.Seed = seed
		id, err := s.repo.CreateTx(ctx, tx, tenantID, principal.UserID, test)
		if err != nil {
			return kecermatanError(err)
		}
		if err := s.repo.ReplacePinsTx(ctx, tx, tenantID, id, pins); err != nil {
			return kecermatanError(err)
		}
		out, err = s.repo.Find(ctx, tx, tenantID, id)
		return kecermatanError(err)
	})
	if err != nil {
		return model.KecermatanTest{}, err
	}
	return out, nil
}

// Update replaces the whole definition in one transaction under the test row lock. Packages are re-pinned
// at their current revision on every save, which is why a pool is judged again here.
func (s *KecermatanService) Update(ctx context.Context, scope HostScope, principal model.Principal, id uuid.UUID, input KecermatanInput) (model.KecermatanTest, error) {
	tenantID, err := questionTenant(scope, principal)
	if err != nil {
		return model.KecermatanTest{}, err
	}
	if input.BaseRevision < 1 {
		return model.KecermatanTest{}, &InputError{Fields: map[string]string{"base_revision": ReasonKecRevision}}
	}
	input, err = kecermatanScalars(input)
	if err != nil {
		return model.KecermatanTest{}, err
	}
	if err := s.checkProgram(ctx, tenantID, input.ProgramID); err != nil {
		return model.KecermatanTest{}, err
	}
	var out model.KecermatanTest
	err = s.inTx(ctx, func(tx pgx.Tx) error {
		current, err := s.repo.LockTx(ctx, tx, tenantID, id)
		if err != nil {
			return kecermatanError(err)
		}
		if current.Locked {
			return ErrKecermatanLocked
		}
		if current.Revision != input.BaseRevision {
			return ErrKecermatanStale
		}
		next, pins, err := s.pin(ctx, tx, tenantID, input)
		if err != nil {
			return err
		}
		if err := s.repo.UpdateTx(ctx, tx, tenantID, id, next); err != nil {
			return kecermatanError(err)
		}
		if err := s.repo.ReplacePinsTx(ctx, tx, tenantID, id, pins); err != nil {
			return kecermatanError(err)
		}
		out, err = s.repo.Find(ctx, tx, tenantID, id)
		return kecermatanError(err)
	})
	if err != nil {
		return model.KecermatanTest{}, err
	}
	return out, nil
}

func (s *KecermatanService) Delete(ctx context.Context, scope HostScope, principal model.Principal, id uuid.UUID) error {
	tenantID, err := questionTenant(scope, principal)
	if err != nil {
		return err
	}
	return s.inTx(ctx, func(tx pgx.Tx) error {
		current, err := s.repo.LockTx(ctx, tx, tenantID, id)
		if err != nil {
			return kecermatanError(err)
		}
		if current.Locked {
			return ErrKecermatanLocked
		}
		return kecermatanError(s.repo.DeleteTx(ctx, tx, tenantID, id))
	})
}

// Preview generates one column of the saved test from its pinned packages and stores nothing. A kolom in the
// second half shows the first half's column, which is how staff see the repetition (FR-KEC-16). With no seed
// a fresh one is drawn, so the preview never reveals the test's stored seed.
func (s *KecermatanService) Preview(ctx context.Context, scope HostScope, principal model.Principal, id uuid.UUID, kolom int, seed string) (KecermatanPreview, error) {
	tenantID, err := questionTenant(scope, principal)
	if err != nil {
		return KecermatanPreview{}, err
	}
	test, err := s.repo.Find(ctx, s.db, tenantID, id)
	if err != nil {
		return KecermatanPreview{}, kecermatanError(err)
	}
	if kolom == 0 {
		kolom = 1
	}
	fields := map[string]string{}
	if kolom < 1 || kolom > test.KolomCount {
		fields["kolom"] = ReasonKecPreview
	}
	if seed == "" {
		if seed, err = newKecermatanSeed(); err != nil {
			return KecermatanPreview{}, err
		}
	} else if !validKecermatanSeed(seed) {
		fields["seed"] = ReasonKecPreview
	}
	if len(fields) > 0 {
		return KecermatanPreview{}, &InputError{Fields: fields}
	}
	pool, err := s.Pool(ctx, s.db, test)
	if err != nil {
		return KecermatanPreview{}, err
	}
	source := (kolom-1)%(test.KolomCount/2) + 1
	column, err := GenerateKecermatanColumn(seed, pool, ParamsOf(test), source)
	if err != nil {
		return KecermatanPreview{}, err
	}
	return KecermatanPreview{Seed: seed, Kolom: kolom, SourceKolom: source, Column: column}, nil
}

// ParamsOf is the structure the generator reads from a stored test.
func ParamsOf(test model.KecermatanTest) KecermatanParams {
	return KecermatanParams{
		ItemLength: test.ItemLength, OptionsPerSoal: test.OptionsPerSoal,
		SoalPerKolom: test.SoalPerKolom, KolomCount: test.KolomCount,
	}
}

// Pool rebuilds the deduplicated generation pool of a stored test from its built-in sources and the package
// revisions it pins, in a fixed order: P18 snapshots content from this same pool.
func (s *KecermatanService) Pool(ctx context.Context, q interface {
	Query(context.Context, string, ...any) (pgx.Rows, error)
	QueryRow(context.Context, string, ...any) pgx.Row
}, test model.KecermatanTest) ([]model.KecermatanElement, error) {
	sources := make([][]model.KecermatanElement, 0, len(test.Packages))
	for _, ref := range test.Packages {
		elements, err := s.packages.Elements(ctx, q, test.TenantID, ref.PackageID, ref.Revision)
		if err != nil {
			return nil, fmt.Errorf("read pinned package: %w", err)
		}
		sources = append(sources, packageElements(ref.Type, elements))
	}
	return kecermatanPool(test.BuiltinSources, sources), nil
}

// pin locks and reads every package the input names, checks the pool and returns the row to store beside the
// pins. Package rows are locked in id order, the one order any two concurrent savers share.
func (s *KecermatanService) pin(ctx context.Context, tx pgx.Tx, tenantID uuid.UUID, input KecermatanInput) (model.KecermatanTest, []repository.KecermatanPin, error) {
	test := model.KecermatanTest{
		ProgramID: input.ProgramID, Title: input.Title, Description: input.Description, ItemType: input.ItemType,
		ItemLength: input.ItemLength, OptionsPerSoal: input.OptionsPerSoal, SoalPerKolom: input.SoalPerKolom,
		KolomCount: input.KolomCount, KolomDurationSeconds: input.KolomDurationSeconds,
		CoverFrom: input.CoverFrom, CoverTo: input.CoverTo, BuiltinSources: effectiveBuiltins(input),
	}
	ids := append([]uuid.UUID(nil), input.PackageIDs...)
	sort.Slice(ids, func(i, j int) bool { return ids[i].String() < ids[j].String() })

	fields := map[string]string{}
	type pinned struct {
		ref      model.KecermatanPackageRef
		elements []model.KecermatanElement
	}
	packs := make([]pinned, 0, len(ids))
	for _, id := range ids {
		path := fmt.Sprintf("package_ids[%d]", indexOfID(input.PackageIDs, id))
		pkg, err := s.packages.Lock(ctx, tx, tenantID, id)
		if errors.Is(err, repository.ErrNotFound) {
			fields[path] = ReasonKecPackage
			continue
		}
		if err != nil {
			return model.KecermatanTest{}, nil, kecermatanError(err)
		}
		if !packageFits(input.ItemType, pkg.Type) {
			fields[path] = ReasonKecPackageType
			continue
		}
		elements, err := s.packages.Elements(ctx, tx, tenantID, id, pkg.Revision)
		if err != nil {
			return model.KecermatanTest{}, nil, kecermatanError(err)
		}
		packs = append(packs, pinned{
			ref:      model.KecermatanPackageRef{PackageID: id, Name: pkg.Name, Type: pkg.Type, Revision: pkg.Revision, ElementCount: len(elements)},
			elements: packageElements(pkg.Type, elements),
		})
	}
	if len(fields) > 0 {
		return model.KecermatanTest{}, nil, &InputError{Fields: fields}
	}

	sources := make([][]model.KecermatanElement, len(packs))
	for i, p := range packs {
		sources[i] = p.elements
	}
	need := input.OptionsPerSoal * input.ItemLength
	if len(kecermatanPool(test.BuiltinSources, sources)) < need {
		return model.KecermatanTest{}, nil, &InputError{Fields: map[string]string{"pool": ReasonKecPool}}
	}

	pins := make([]repository.KecermatanPin, 0, len(packs))
	for i, p := range packs {
		// What the rest of the sources already provide; the package only has to cover the remainder.
		others := make([][]model.KecermatanElement, 0, len(packs)-1)
		others = append(others, sources[:i]...)
		others = append(others, sources[i+1:]...)
		required := max(1, need-len(kecermatanPool(test.BuiltinSources, others)))
		pins = append(pins, repository.KecermatanPin{PackageID: p.ref.PackageID, Revision: p.ref.Revision, RequiredCount: required})
	}
	return test, pins, nil
}

func (s *KecermatanService) checkProgram(ctx context.Context, tenantID, programID uuid.UUID) error {
	ok, err := s.repo.ProgramExists(ctx, tenantID, programID)
	if err != nil {
		return err
	}
	if !ok {
		return &InputError{Fields: map[string]string{"program_id": ReasonKecProgram}}
	}
	return nil
}

func indexOfID(ids []uuid.UUID, id uuid.UUID) int {
	for i, candidate := range ids {
		if candidate == id {
			return i
		}
	}
	return -1
}

// packageFits is the type rule of FR-ADM-KEC-13: a packaged test takes a package of its own type, a shape
// test may take a shape package, and a mix takes any.
func packageFits(itemType, packageType string) bool {
	switch itemType {
	case model.KecSymbol, model.KecEmoji, model.KecImage, model.KecShape:
		return packageType == itemType
	case model.KecMix:
		return true
	}
	return false
}

// effectiveBuiltins is what the pool starts from: char and number are the built-ins themselves, shape uses
// all seven unless a shape package narrows it, and mix names its own.
func effectiveBuiltins(input KecermatanInput) []string {
	switch input.ItemType {
	case model.KecChar, model.KecNumber:
		return []string{input.ItemType}
	case model.KecShape:
		if len(input.PackageIDs) == 0 {
			return []string{model.KecShape}
		}
	case model.KecMix:
		out := make([]string, 0, len(input.BuiltinSources))
		for _, name := range []string{model.KecChar, model.KecNumber, model.KecShape} {
			for _, given := range input.BuiltinSources {
				if given == name {
					out = append(out, name)
					break
				}
			}
		}
		return out
	}
	return []string{}
}

func packageElements(packageType string, elements []model.ItemElement) []model.KecermatanElement {
	out := make([]model.KecermatanElement, 0, len(elements))
	for _, e := range elements {
		out = append(out, model.KecermatanElement{Kind: packageType, Key: e.Key, Value: e.Value, AssetID: e.AssetID})
	}
	return out
}

// kecermatanPool concatenates the built-ins in canonical order, then the package sources, and deduplicates.
func kecermatanPool(builtins []string, sources [][]model.KecermatanElement) []model.KecermatanElement {
	var pool []model.KecermatanElement
	for _, name := range []string{model.KecChar, model.KecNumber, model.KecShape} {
		for _, given := range builtins {
			if given != name {
				continue
			}
			switch name {
			case model.KecChar:
				pool = append(pool, kecermatanChars()...)
			case model.KecNumber:
				pool = append(pool, kecermatanNumbers()...)
			case model.KecShape:
				pool = append(pool, kecermatanShapes()...)
			}
		}
	}
	for _, source := range sources {
		pool = append(pool, source...)
	}
	return dedupeKecermatanPool(pool)
}

// kecermatanScalars validates everything that needs no database: the structure bounds of FR-ADM-KEC-06 and
// D09, and which sources the item type allows.
func kecermatanScalars(input KecermatanInput) (KecermatanInput, error) {
	fields := map[string]string{}
	input.Title = strings.TrimSpace(input.Title)
	input.Description = strings.TrimSpace(input.Description)

	if n := utf8.RuneCountInString(input.Title); n < 1 || n > maxKecermatanTitleLen {
		fields["title"] = ReasonKecTitle
	}
	if utf8.RuneCountInString(input.Description) > maxKecermatanDesc {
		fields["description"] = ReasonKecDescription
	}
	if input.ProgramID == uuid.Nil {
		fields["program_id"] = ReasonKecProgram
	}
	if input.ItemLength < 1 || input.ItemLength > 5 {
		fields["item_length"] = ReasonKecItemLength
	}
	if input.OptionsPerSoal < minOptionsPerSoal || input.OptionsPerSoal > maxOptionsPerSoal {
		fields["options_per_soal"] = ReasonKecOptions
	}
	if input.SoalPerKolom < minSoalPerKolom || input.SoalPerKolom > maxSoalPerKolom {
		fields["soal_per_kolom"] = ReasonKecSoal
	}
	switch {
	case input.KolomCount < minKolomCount || input.KolomCount > maxKolomCount:
		fields["kolom_count"] = ReasonKecKolomRange
	case input.KolomCount%2 != 0:
		fields["kolom_count"] = ReasonKecKolomOdd
	}
	if input.KolomDurationSeconds < 1 || input.KolomDurationSeconds > maxKolomSeconds {
		fields["kolom_duration_seconds"] = ReasonKecDuration
	}
	if !coverColor.MatchString(input.CoverFrom) {
		fields["cover_from"] = ReasonKecCover
	}
	if !coverColor.MatchString(input.CoverTo) {
		fields["cover_to"] = ReasonKecCover
	}
	for path, reason := range kecermatanSources(input) {
		fields[path] = reason
	}
	if len(fields) > 0 {
		return KecermatanInput{}, &InputError{Fields: fields}
	}
	return input, nil
}

func kecermatanSources(input KecermatanInput) map[string]string {
	fields := map[string]string{}
	seen := map[uuid.UUID]bool{}
	for i, id := range input.PackageIDs {
		if id == uuid.Nil {
			fields[fmt.Sprintf("package_ids[%d]", i)] = ReasonKecPackage
		} else if seen[id] {
			fields[fmt.Sprintf("package_ids[%d]", i)] = ReasonKecPackageDup
		}
		seen[id] = true
	}
	if len(input.PackageIDs) > maxKecermatanPackages {
		fields["package_ids"] = ReasonKecSources
	}
	builtinSeen := map[string]bool{}
	for _, name := range input.BuiltinSources {
		if (name != model.KecChar && name != model.KecNumber && name != model.KecShape) || builtinSeen[name] {
			fields["builtin_sources"] = ReasonKecSources
		}
		builtinSeen[name] = true
	}

	switch input.ItemType {
	case model.KecChar, model.KecNumber:
		if len(input.BuiltinSources) > 0 || len(input.PackageIDs) > 0 {
			fields["package_ids"] = ReasonKecSources
		}
	case model.KecSymbol, model.KecEmoji, model.KecImage:
		if len(input.BuiltinSources) > 0 || len(input.PackageIDs) != 1 {
			fields["package_ids"] = ReasonKecSources
		}
	case model.KecShape:
		if len(input.BuiltinSources) > 0 || len(input.PackageIDs) > 1 {
			fields["package_ids"] = ReasonKecSources
		}
	case model.KecMix:
		if len(input.BuiltinSources)+len(input.PackageIDs) < 1 {
			fields["package_ids"] = ReasonKecSources
		}
	default:
		fields["item_type"] = ReasonKecItemType
	}
	return fields
}

func kecermatanError(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, repository.ErrNotFound):
		return ErrKecermatanNotFound
	case errors.Is(err, repository.ErrScopeRequired):
		return ErrTenantNotFound
	}
	return err
}

func (s *KecermatanService) inTx(ctx context.Context, fn func(pgx.Tx) error) error {
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
