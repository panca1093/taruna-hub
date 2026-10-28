package service

import (
	"context"
	"errors"
	"fmt"
	"math/rand/v2"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/tarunaeduka/bimbel-tryout/api/internal/model"
	"github.com/tarunaeduka/bimbel-tryout/api/internal/repository"
)

var (
	ErrItemPackageNotFound  = errors.New("item package not found")
	ErrItemElementNotFound  = errors.New("item package element not found")
	ErrItemPackageNameTaken = errors.New("item package name taken")
	// ErrItemPackageInUse: a referencing test pins the package, so it cannot be deleted.
	ErrItemPackageInUse = errors.New("item package in use")
	// ErrItemPackageShrink: removing the element would leave fewer than a referencing test needs.
	ErrItemPackageShrink = errors.New("item package would shrink below a test's need")
)

// Item package field reasons; the transport turns each into the copy the page shows.
const (
	ReasonPackageType   = "package_type"
	ReasonPackageName   = "package_name"
	ReasonElementsEmpty = "elements_empty"
	ReasonElementsKind  = "elements_kind"
	ReasonUploadTooMany = "upload_too_many"
	ReasonPackageFull   = "package_full"
	ReasonSampleParams  = "sample_params"
	ReasonPoolTooSmall  = "pool_too_small"
	ReasonRevision      = "package_revision"
)

const (
	maxPackageNameLen = 100
	minItemLength     = 1
	maxItemLength     = 5
	minSampleOptions  = 2
	maxSampleSoal     = 100
)

type ItemPackageService struct {
	db     *pgxpool.Pool
	repo   *repository.ItemPackageRepository
	assets *AssetService
}

func NewItemPackageService(db *pgxpool.Pool, repo *repository.ItemPackageRepository, assets *AssetService) *ItemPackageService {
	return &ItemPackageService{db: db, repo: repo, assets: assets}
}

// ItemPackageDetail is a package with the elements of one frozen revision.
type ItemPackageDetail struct {
	model.ItemPackage
	ViewRevision int
	Elements     []model.ItemElement
}

func (s *ItemPackageService) List(ctx context.Context, scope HostScope, principal model.Principal, kind string) ([]model.ItemPackage, error) {
	tenantID, err := questionTenant(scope, principal)
	if err != nil {
		return nil, err
	}
	kind = strings.TrimSpace(kind)
	if kind != "" && !validPackageType(kind) {
		return nil, &InputError{Fields: map[string]string{"type": ReasonPackageType}}
	}
	return s.repo.List(ctx, tenantID, kind)
}

// Get reads the current revision, or an older frozen one when revision is positive.
func (s *ItemPackageService) Get(ctx context.Context, scope HostScope, principal model.Principal, id uuid.UUID, revision int) (ItemPackageDetail, error) {
	tenantID, err := questionTenant(scope, principal)
	if err != nil {
		return ItemPackageDetail{}, err
	}
	pkg, err := s.repo.Find(ctx, s.db, tenantID, id)
	if err != nil {
		return ItemPackageDetail{}, itemPackageError(err)
	}
	detail := ItemPackageDetail{ItemPackage: pkg, ViewRevision: pkg.Revision}
	if revision != 0 {
		if revision < 1 || revision > pkg.Revision {
			return ItemPackageDetail{}, &InputError{Fields: map[string]string{"revision": ReasonRevision}}
		}
		detail.ViewRevision = revision
	}
	detail.Elements, err = s.repo.Elements(ctx, s.db, tenantID, id, detail.ViewRevision)
	if err != nil {
		return ItemPackageDetail{}, err
	}
	return detail, nil
}

func (s *ItemPackageService) Create(ctx context.Context, scope HostScope, principal model.Principal, kind, name string) (model.ItemPackage, error) {
	tenantID, err := questionTenant(scope, principal)
	if err != nil {
		return model.ItemPackage{}, err
	}
	fields := map[string]string{}
	kind = strings.TrimSpace(kind)
	if !validPackageType(kind) {
		fields["type"] = ReasonPackageType
	}
	name, ok := packageName(name)
	if !ok {
		fields["name"] = ReasonPackageName
	}
	if len(fields) > 0 {
		return model.ItemPackage{}, &InputError{Fields: fields}
	}
	var created model.ItemPackage
	err = s.inTx(ctx, func(tx pgx.Tx) error {
		created, err = s.repo.CreateTx(ctx, tx, tenantID, principal.UserID, kind, name)
		return err
	})
	if err != nil {
		return model.ItemPackage{}, itemPackageError(err)
	}
	return created, nil
}

func (s *ItemPackageService) Rename(ctx context.Context, scope HostScope, principal model.Principal, id uuid.UUID, name string) (model.ItemPackage, error) {
	tenantID, err := questionTenant(scope, principal)
	if err != nil {
		return model.ItemPackage{}, err
	}
	name, ok := packageName(name)
	if !ok {
		return model.ItemPackage{}, &InputError{Fields: map[string]string{"name": ReasonPackageName}}
	}
	var renamed model.ItemPackage
	err = s.inTx(ctx, func(tx pgx.Tx) error {
		if err := s.repo.RenameTx(ctx, tx, tenantID, id, name); err != nil {
			return err
		}
		renamed, err = s.repo.Find(ctx, tx, tenantID, id)
		return err
	})
	if err != nil {
		return model.ItemPackage{}, itemPackageError(err)
	}
	return renamed, nil
}

// Delete refuses a package any test pins. The lock makes "is it pinned" and "delete it" one decision; the
// foreign keys are the backstop for a pin written by something that did not take the lock.
func (s *ItemPackageService) Delete(ctx context.Context, scope HostScope, principal model.Principal, id uuid.UUID) error {
	tenantID, err := questionTenant(scope, principal)
	if err != nil {
		return err
	}
	err = s.inTx(ctx, func(tx pgx.Tx) error {
		pkg, err := s.repo.Lock(ctx, tx, tenantID, id)
		if err != nil {
			return err
		}
		if pkg.InUse {
			return ErrItemPackageInUse
		}
		return s.repo.DeleteTx(ctx, tx, tenantID, id)
	})
	return itemPackageError(err)
}

// PreviewElements reports what a commit of the same upload would accept and why it would reject the rest.
// It reads the package and writes nothing: no row, no object, no revision.
func (s *ItemPackageService) PreviewElements(ctx context.Context, scope HostScope, principal model.Principal, id uuid.UUID, in ElementInput) (model.ElementPreview, error) {
	cands, _, err := s.classified(ctx, scope, principal, id, in)
	if err != nil {
		return model.ElementPreview{}, err
	}
	preview := model.ElementPreview{Items: make([]model.ElementCheck, 0, len(cands))}
	for _, c := range cands {
		preview.Items = append(preview.Items, c.ElementCheck)
		if c.Accepted {
			preview.Accepted++
		} else {
			preview.Rejected++
		}
	}
	return preview, nil
}

func (s *ItemPackageService) classified(ctx context.Context, scope HostScope, principal model.Principal, id uuid.UUID, in ElementInput) ([]candidate, model.ItemPackage, error) {
	tenantID, err := questionTenant(scope, principal)
	if err != nil {
		return nil, model.ItemPackage{}, err
	}
	pkg, err := s.repo.Find(ctx, s.db, tenantID, id)
	if err != nil {
		return nil, model.ItemPackage{}, itemPackageError(err)
	}
	cands, err := buildCandidates(ctx, pkg.Type, in)
	if err != nil {
		return nil, model.ItemPackage{}, err
	}
	existing, err := s.repo.Elements(ctx, s.db, tenantID, id, pkg.Revision)
	if err != nil {
		return nil, model.ItemPackage{}, err
	}
	return classify(cands, existing), pkg, nil
}

// AddElements inserts the accepted elements as one new revision, or nothing at all: re-sending the same
// upload finds everything already present and writes no revision. Image objects are stored staged first and
// finalized inside the revision's transaction, so a failure leaves only purgeable staged rows.
func (s *ItemPackageService) AddElements(ctx context.Context, scope HostScope, principal model.Principal, id uuid.UUID, in ElementInput) (model.ElementCommit, error) {
	cands, pkg, err := s.classified(ctx, scope, principal, id, in)
	if err != nil {
		return model.ElementCommit{}, err
	}
	tenantID := pkg.TenantID

	commit := model.ElementCommit{Revision: pkg.Revision}
	var accepted []candidate
	for _, c := range cands {
		switch {
		case c.Accepted:
			accepted = append(accepted, c)
		case c.Reason == ElemDuplicate || c.Reason == ElemDuplicateUpload:
			commit.AlreadyPresent++
		default:
			commit.Rejected++
		}
	}
	if len(accepted) == 0 {
		return commit, nil
	}

	assetIDs := make([]*uuid.UUID, len(accepted))
	if pkg.Type == model.ItemImage {
		for at, c := range accepted {
			asset, err := s.assets.Upload(ctx, scope, principal, model.PurposeItemImage, c.data)
			if err != nil {
				return model.ElementCommit{}, err
			}
			assetID := asset.ID
			assetIDs[at] = &assetID
		}
	}

	err = s.inTx(ctx, func(tx pgx.Tx) error {
		locked, err := s.repo.Lock(ctx, tx, tenantID, id)
		if err != nil {
			return err
		}
		current, err := s.repo.Elements(ctx, tx, tenantID, id, locked.Revision)
		if err != nil {
			return err
		}
		held := make(map[string]bool, len(current))
		for _, e := range current {
			held[e.Key] = true
		}
		next := current
		added := 0
		for at, c := range accepted {
			if held[c.Key] {
				commit.AlreadyPresent++
				continue
			}
			if len(next) >= MaxPackageElements {
				return &InputError{Fields: map[string]string{"elements": ReasonPackageFull}}
			}
			if assetIDs[at] != nil {
				if _, err := s.assets.FinalizeTx(ctx, tx, scope, principal, *assetIDs[at]); err != nil {
					return err
				}
			}
			next = append(next, model.ItemElement{
				ID: uuid.New(), Kind: c.Kind, Key: c.Key, Value: c.value, Label: c.Label, AssetID: assetIDs[at],
			})
			held[c.Key] = true
			added++
		}
		if added == 0 {
			return nil
		}
		commit.Added = added
		commit.Revision = locked.Revision + 1
		return s.repo.WriteRevisionTx(ctx, tx, tenantID, id, principal.UserID, commit.Revision, next)
	})
	if err != nil {
		return model.ElementCommit{}, itemPackageError(err)
	}
	return commit, nil
}

// RemoveElement writes a new revision without the element. Older revisions keep it, so a pinned test and its
// frozen assets stay renderable. A package a test pins cannot drop below the pool that test needs.
func (s *ItemPackageService) RemoveElement(ctx context.Context, scope HostScope, principal model.Principal, id, elementID uuid.UUID) (model.ItemPackage, error) {
	tenantID, err := questionTenant(scope, principal)
	if err != nil {
		return model.ItemPackage{}, err
	}
	var result model.ItemPackage
	err = s.inTx(ctx, func(tx pgx.Tx) error {
		locked, err := s.repo.Lock(ctx, tx, tenantID, id)
		if err != nil {
			return err
		}
		current, err := s.repo.Elements(ctx, tx, tenantID, id, locked.Revision)
		if err != nil {
			return err
		}
		next := make([]model.ItemElement, 0, len(current))
		for _, e := range current {
			if e.ID != elementID {
				next = append(next, e)
			}
		}
		if len(next) == len(current) {
			return ErrItemElementNotFound
		}
		if len(next) < locked.RequiredCount {
			return ErrItemPackageShrink
		}
		if err := s.repo.WriteRevisionTx(ctx, tx, tenantID, id, principal.UserID, locked.Revision+1, next); err != nil {
			return err
		}
		result, err = s.repo.Find(ctx, tx, tenantID, id)
		return err
	})
	if err != nil {
		return model.ItemPackage{}, itemPackageError(err)
	}
	return result, nil
}

// SampleInput are the test parameters the sample column is generated for.
type SampleInput struct {
	ItemLength     int
	OptionsPerSoal int
	SoalPerKolom   int
}

// SampleSoal is one question of the sample: options_per_soal items, each item_length distinct elements.
type SampleSoal struct {
	Items [][]model.ItemElement
}

// SampleColumn draws one column from the package's current pool and stores nothing. This is the package
// preview of FR-ADM-KEC-21, not the P10 generator: no seed, no per-session stability, no reference item.
func (s *ItemPackageService) SampleColumn(ctx context.Context, scope HostScope, principal model.Principal, id uuid.UUID, in SampleInput) ([]SampleSoal, error) {
	detail, err := s.Get(ctx, scope, principal, id, 0)
	if err != nil {
		return nil, err
	}
	if in.ItemLength < minItemLength || in.ItemLength > maxItemLength ||
		in.OptionsPerSoal < minSampleOptions || in.SoalPerKolom < 1 || in.SoalPerKolom > maxSampleSoal {
		return nil, &InputError{Fields: map[string]string{"params": ReasonSampleParams}}
	}
	need := in.OptionsPerSoal * in.ItemLength
	if need > len(detail.Elements) {
		return nil, &InputError{Fields: map[string]string{"elements": ReasonPoolTooSmall}}
	}
	column := make([]SampleSoal, 0, in.SoalPerKolom)
	for range in.SoalPerKolom {
		picked := rand.Perm(len(detail.Elements))[:need]
		soal := SampleSoal{Items: make([][]model.ItemElement, 0, in.OptionsPerSoal)}
		for item := 0; item < in.OptionsPerSoal; item++ {
			elements := make([]model.ItemElement, 0, in.ItemLength)
			for _, at := range picked[item*in.ItemLength : (item+1)*in.ItemLength] {
				elements = append(elements, detail.Elements[at])
			}
			soal.Items = append(soal.Items, elements)
		}
		column = append(column, soal)
	}
	return column, nil
}

// SymbolTemplate is the downloadable one-column file for symbol upload.
func (s *ItemPackageService) SymbolTemplate(format string) (ImportTemplate, error) {
	format = strings.ToLower(strings.TrimSpace(format))
	if format == "" {
		format = "xlsx"
	}
	if format != "xlsx" && format != "csv" {
		return ImportTemplate{}, &InputError{Fields: map[string]string{"format": ReasonImportFormat}}
	}
	rows := [][]string{{"simbol"}, {"♠"}, {"♥"}, {"★"}, {"▲"}}
	var data []byte
	var err error
	if format == "csv" {
		data, err = writeCSV(rows)
	} else {
		data, err = writeXLSX(rows)
	}
	if err != nil {
		return ImportTemplate{}, fmt.Errorf("build %s template: %w", format, err)
	}
	return ImportTemplate{FileName: "template-simbol." + format, ContentType: importContentType(format), Data: data}, nil
}

func validPackageType(kind string) bool {
	switch kind {
	case model.ItemSymbol, model.ItemImage, model.ItemEmoji, model.ItemShape:
		return true
	}
	return false
}

func packageName(name string) (string, bool) {
	name = strings.TrimSpace(name)
	n := utf8.RuneCountInString(name)
	return name, n >= 1 && n <= maxPackageNameLen && utf8.ValidString(name)
}

func itemPackageError(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, repository.ErrNotFound):
		return ErrItemPackageNotFound
	case errors.Is(err, repository.ErrScopeRequired):
		return ErrTenantNotFound
	case errors.Is(err, repository.ErrDuplicateName):
		return ErrItemPackageNameTaken
	case errors.Is(err, repository.ErrReferenced):
		return ErrItemPackageInUse
	}
	return err
}

func (s *ItemPackageService) inTx(ctx context.Context, fn func(pgx.Tx) error) error {
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
