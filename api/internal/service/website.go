package service

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/tarunaeduka/bimbel-tryout/api/internal/model"
	"github.com/tarunaeduka/bimbel-tryout/api/internal/repository"
)

var (
	ErrWebsiteNotPublished = errors.New("website has no published snapshot")
	ErrWebsiteConflict     = errors.New("website revision no longer matches")
	ErrWebsiteForbidden    = errors.New("website action needs the admin role")
)

const (
	ReasonWebsiteRequired = "website_required"
	ReasonWebsiteTooLong  = "website_too_long"
	ReasonWebsiteSchema   = "website_schema_version"
	ReasonWebsiteLocale   = "website_locale"
	ReasonWebsiteSlot     = "website_program_slot"
	ReasonWebsiteRevision = "website_revision"
	ReasonWebsiteLimit    = "website_limit"
	ReasonWebsiteCursor   = "website_cursor"
)

const (
	maxWebsiteRevision  = 1<<53 - 1
	defaultWebsiteLimit = 20
	maxWebsiteLimit     = 50
)

var (
	websiteLocales = []string{"id", "en"}
	websiteSlots   = []string{"tni", "polri", "kedinasan"}
)

// websiteContent collects field errors while it trims and measures each text.
type websiteContent struct {
	fields map[string]string
}

func (w *websiteContent) text(path, value string, max int, required bool) string {
	value = strings.TrimSpace(value)
	switch {
	case required && value == "":
		w.fields[path] = ReasonWebsiteRequired
	case utf8.RuneCountInString(value) > max:
		w.fields[path] = ReasonWebsiteTooLong
	}
	return value
}

// normalizeWebsiteContent trims every text and validates the whole content; it never mutates its argument.
func normalizeWebsiteContent(content model.WebsiteContent) (model.WebsiteContent, error) {
	w := websiteContent{fields: map[string]string{}}
	if content.SchemaVersion != model.WebsiteSchemaVersion {
		w.fields["schema_version"] = ReasonWebsiteSchema
	}
	out := model.WebsiteContent{SchemaVersion: content.SchemaVersion, Locales: map[string]model.WebsiteLocale{}}

	for key := range content.Locales {
		if key != "id" && key != "en" {
			w.fields["locales."+key] = ReasonWebsiteLocale
		}
	}
	for _, key := range websiteLocales {
		locale, ok := content.Locales[key]
		if !ok {
			w.fields["locales."+key] = ReasonWebsiteRequired
			continue
		}
		out.Locales[key] = w.locale("locales."+key, locale)
	}

	if len(w.fields) > 0 {
		return model.WebsiteContent{}, &InputError{Fields: w.fields}
	}
	return out, nil
}

func (w *websiteContent) locale(prefix string, in model.WebsiteLocale) model.WebsiteLocale {
	home := prefix + ".home."
	out := model.WebsiteLocale{
		Home: model.WebsiteHome{
			HeroTitle:   w.text(home+"hero_title", in.Home.HeroTitle, 90, true),
			HeroBody:    w.text(home+"hero_body", in.Home.HeroBody, 360, true),
			ProgramCTA:  w.text(home+"program_cta", in.Home.ProgramCTA, 36, true),
			PracticeCTA: w.text(home+"practice_cta", in.Home.PracticeCTA, 36, true),
		},
		Programs: map[string]model.WebsiteProgram{},
	}

	for slot := range in.Programs {
		if slot != "tni" && slot != "polri" && slot != "kedinasan" {
			w.fields[prefix+".programs."+slot] = ReasonWebsiteSlot
		}
	}
	for _, slot := range websiteSlots {
		p, ok := in.Programs[slot]
		path := prefix + ".programs." + slot
		if !ok {
			w.fields[path] = ReasonWebsiteRequired
			continue
		}
		out.Programs[slot] = model.WebsiteProgram{
			Title:    w.text(path+".title", p.Title, 80, true),
			Summary:  w.text(path+".summary", p.Summary, 320, true),
			Schedule: w.text(path+".schedule", p.Schedule, 160, false),
			Location: w.text(path+".location", p.Location, 160, false),
			Fees:     w.text(path+".fees", p.Fees, 160, false),
			Contact:  w.text(path+".contact", p.Contact, 160, false),
		}
	}
	return out
}

type SaveWebsiteInput struct {
	ExpectedDraftRevision int64
	Content               model.WebsiteContent
}

type PublishWebsiteInput struct {
	ExpectedDraftRevision     int64
	ExpectedPublishedRevision int64
}

type WebsiteHistoryInput struct {
	Limit  int
	Cursor string
}

// WebsiteService owns the revision, publication and permission rules; the repository only persists.
type WebsiteService struct {
	db   *pgxpool.Pool
	repo *repository.WebsiteRepository
}

func NewWebsiteService(db *pgxpool.Pool, repo *repository.WebsiteRepository) *WebsiteService {
	return &WebsiteService{db: db, repo: repo}
}

// Published is the public read: the last snapshot only, never the draft.
func (s *WebsiteService) Published(ctx context.Context, scope HostScope) (model.PublishedWebsite, error) {
	row, err := s.repo.Find(ctx, scope.Tenant.ID)
	if errors.Is(err, repository.ErrNotFound) || err == nil && row.Published == nil {
		return model.PublishedWebsite{}, ErrWebsiteNotPublished
	}
	if err != nil {
		return model.PublishedWebsite{}, websiteStoreError(err)
	}
	return model.PublishedWebsite{Content: *row.Published, PublishedRevision: row.PublishedRevision, PublishedAt: *row.PublishedAt}, nil
}

// Workbench creates the row with the reviewed default draft on the first read; it never publishes.
func (s *WebsiteService) Workbench(ctx context.Context, scope HostScope, principal model.Principal) (model.WebsiteWorkbench, error) {
	tenantID, err := websiteAdmin(scope, principal)
	if err != nil {
		return model.WebsiteWorkbench{}, err
	}
	row, err := s.repo.Find(ctx, tenantID)
	if errors.Is(err, repository.ErrNotFound) {
		row, err = s.initialize(ctx, tenantID)
	}
	if err != nil {
		return model.WebsiteWorkbench{}, websiteStoreError(err)
	}
	return workbenchOf(row), nil
}

func (s *WebsiteService) initialize(ctx context.Context, tenantID uuid.UUID) (repository.WebsiteRow, error) {
	var row repository.WebsiteRow
	err := s.inTx(ctx, func(tx pgx.Tx) (err error) {
		if err = s.repo.InitializeTx(ctx, tx, tenantID, DefaultWebsiteContent()); err != nil {
			return err
		}
		row, err = s.repo.FindTx(ctx, tx, tenantID)
		return err
	})
	return row, err
}

func (s *WebsiteService) Save(ctx context.Context, scope HostScope, principal model.Principal, in SaveWebsiteInput) (model.WebsiteWorkbench, error) {
	tenantID, err := websiteAdmin(scope, principal)
	if err != nil {
		return model.WebsiteWorkbench{}, err
	}
	if in.ExpectedDraftRevision < 1 || in.ExpectedDraftRevision > maxWebsiteRevision {
		return model.WebsiteWorkbench{}, &InputError{Fields: map[string]string{"expected_draft_revision": ReasonWebsiteRevision}}
	}
	content, err := normalizeWebsiteContent(in.Content)
	if err != nil {
		return model.WebsiteWorkbench{}, err
	}

	var row repository.WebsiteRow
	err = s.inTx(ctx, func(tx pgx.Tx) error {
		var err error
		if row, err = s.lock(ctx, tx, tenantID); err != nil {
			return err
		}
		if row.DraftRevision != in.ExpectedDraftRevision {
			return ErrWebsiteConflict
		}
		if reflect.DeepEqual(row.Draft, content) {
			return nil
		}
		if row.DraftRevision >= maxWebsiteRevision {
			return errors.New("website draft revision limit reached")
		}
		if err := s.repo.SaveDraftTx(ctx, tx, tenantID, content, principal.UserID); err != nil {
			return err
		}
		row, err = s.repo.FindTx(ctx, tx, tenantID)
		return err
	})
	if err != nil {
		return model.WebsiteWorkbench{}, websiteStoreError(err)
	}
	return workbenchOf(row), nil
}

// Publish copies the whole draft to the public snapshot. A request for a draft revision that was already
// published returns that record, so a lost response can be retried; a moved draft is a conflict, never published.
func (s *WebsiteService) Publish(ctx context.Context, scope HostScope, principal model.Principal, in PublishWebsiteInput) (model.WebsitePublication, error) {
	tenantID, err := websiteAdmin(scope, principal)
	if err != nil {
		return model.WebsitePublication{}, err
	}
	if in.ExpectedDraftRevision < 1 || in.ExpectedDraftRevision > maxWebsiteRevision {
		return model.WebsitePublication{}, &InputError{Fields: map[string]string{"expected_draft_revision": ReasonWebsiteRevision}}
	}
	if in.ExpectedPublishedRevision < 0 || in.ExpectedPublishedRevision > maxWebsiteRevision {
		return model.WebsitePublication{}, &InputError{Fields: map[string]string{"expected_published_revision": ReasonWebsiteRevision}}
	}

	var out model.WebsitePublication
	err = s.inTx(ctx, func(tx pgx.Tx) error {
		row, err := s.lock(ctx, tx, tenantID)
		if err != nil {
			return err
		}
		if row.DraftRevision != in.ExpectedDraftRevision {
			return ErrWebsiteConflict
		}
		recorded, err := s.repo.PublicationBySourceTx(ctx, tx, tenantID, row.DraftRevision)
		if err == nil {
			out = recorded
			return nil
		}
		if !errors.Is(err, repository.ErrNotFound) {
			return err
		}
		if row.PublishedRevision != in.ExpectedPublishedRevision {
			return ErrWebsiteConflict
		}
		if _, err := normalizeWebsiteContent(row.Draft); err != nil {
			return err
		}
		if row.Published != nil && reflect.DeepEqual(*row.Published, row.Draft) {
			current, err := s.repo.PublicationTx(ctx, tx, tenantID, row.PublishedRevision)
			current.Changed = false
			out = current
			return err
		}
		if row.PublishedRevision >= maxWebsiteRevision {
			return errors.New("website published revision limit reached")
		}
		out, err = s.repo.PublishTx(ctx, tx, tenantID, row.PublishedRevision+1, row.DraftRevision, principal.UserID)
		return err
	})
	if err != nil {
		return model.WebsitePublication{}, websiteStoreError(err)
	}
	return out, nil
}

func (s *WebsiteService) History(ctx context.Context, scope HostScope, principal model.Principal, in WebsiteHistoryInput) (model.WebsitePublicationList, error) {
	tenantID, err := websiteAdmin(scope, principal)
	if err != nil {
		return model.WebsitePublicationList{}, err
	}
	limit := in.Limit
	if limit == 0 {
		limit = defaultWebsiteLimit
	}
	if limit < 1 || limit > maxWebsiteLimit {
		return model.WebsitePublicationList{}, &InputError{Fields: map[string]string{"limit": ReasonWebsiteLimit}}
	}
	var before int64
	if in.Cursor != "" {
		if before, err = decodeWebsiteCursor(in.Cursor); err != nil {
			return model.WebsitePublicationList{}, &InputError{Fields: map[string]string{"cursor": ReasonWebsiteCursor}}
		}
	}
	items, err := s.repo.Publications(ctx, tenantID, before, limit+1)
	if err != nil {
		return model.WebsitePublicationList{}, websiteStoreError(err)
	}
	list := model.WebsitePublicationList{Items: items}
	if len(items) > limit {
		list.Items = items[:limit]
		next := encodeWebsiteCursor(list.Items[limit-1].PublishedRevision)
		list.NextCursor = &next
	}
	return list, nil
}

// lock takes the tenant's row, creating it first when a save or publish arrives before any read.
func (s *WebsiteService) lock(ctx context.Context, tx pgx.Tx, tenantID uuid.UUID) (repository.WebsiteRow, error) {
	if err := s.repo.InitializeTx(ctx, tx, tenantID, DefaultWebsiteContent()); err != nil {
		return repository.WebsiteRow{}, err
	}
	return s.repo.LockTx(ctx, tx, tenantID)
}

func (s *WebsiteService) inTx(ctx context.Context, fn func(pgx.Tx) error) error {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin website transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(context.WithoutCancel(ctx)) }()
	if err := fn(tx); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func websiteAdmin(scope HostScope, principal model.Principal) (uuid.UUID, error) {
	tenantID, err := walletTenant(scope, principal)
	if err != nil {
		return uuid.Nil, err
	}
	if principal.Role != model.RoleAdmin {
		return uuid.Nil, ErrWebsiteForbidden
	}
	return tenantID, nil
}

func websiteStoreError(err error) error {
	if errors.Is(err, repository.ErrScopeRequired) {
		return ErrTenantNotFound
	}
	return err
}

func workbenchOf(row repository.WebsiteRow) model.WebsiteWorkbench {
	return model.WebsiteWorkbench{
		Draft: row.Draft, DraftRevision: row.DraftRevision, PublishedRevision: row.PublishedRevision,
		Published: row.Published, UpdatedAt: row.UpdatedAt, UpdatedBy: row.UpdatedBy,
	}
}

func encodeWebsiteCursor(revision int64) string {
	return base64.RawURLEncoding.EncodeToString([]byte(strconv.FormatInt(revision, 10)))
}

func decodeWebsiteCursor(cursor string) (int64, error) {
	raw, err := base64.RawURLEncoding.DecodeString(cursor)
	if err != nil {
		return 0, err
	}
	revision, err := strconv.ParseInt(string(raw), 10, 64)
	if err != nil || revision < 1 || revision > maxWebsiteRevision {
		return 0, errors.New("cursor out of range")
	}
	return revision, nil
}

// DefaultWebsiteContent is the reviewed starting draft: neutral copy, no claims, optional program facts empty.
func DefaultWebsiteContent() model.WebsiteContent {
	locale := func(home model.WebsiteHome, tni, polri, kedinasan [2]string) model.WebsiteLocale {
		program := func(t [2]string) model.WebsiteProgram { return model.WebsiteProgram{Title: t[0], Summary: t[1]} }
		return model.WebsiteLocale{
			Home:     home,
			Programs: map[string]model.WebsiteProgram{"tni": program(tni), "polri": program(polri), "kedinasan": program(kedinasan)},
		}
	}
	return model.WebsiteContent{
		SchemaVersion: model.WebsiteSchemaVersion,
		Locales: map[string]model.WebsiteLocale{
			"id": locale(model.WebsiteHome{
				HeroTitle:   "Persiapanmu, sesuai caramu.",
				HeroBody:    "Kenali program bimbel dan diklat TNI, POLRI, atau Kedinasan. Lengkapi persiapanmu dengan tryout bertimer dan latihan Kecermatan dalam satu ruang belajar.",
				ProgramCTA:  "Kenali program bimbel",
				PracticeCTA: "Pilih latihan mandiri",
			},
				[2]string{"Persiapan TNI", "Program persiapan seleksi TNI. Informasi jadwal, lokasi, biaya, dan kontak menunggu materi resmi."},
				[2]string{"Persiapan POLRI", "Program persiapan seleksi POLRI. Informasi jadwal, lokasi, biaya, dan kontak menunggu materi resmi."},
				[2]string{"Persiapan Kedinasan", "Program persiapan seleksi Kedinasan. Informasi jadwal, lokasi, biaya, dan kontak menunggu materi resmi."}),
			"en": locale(model.WebsiteHome{
				HeroTitle:   "Your preparation, your way.",
				HeroBody:    "Explore TNI, POLRI and civil-service preparation programs. Complement your preparation with timed tryouts and accuracy practice in one study space.",
				ProgramCTA:  "Explore our programs",
				PracticeCTA: "Explore independent practice",
			},
				[2]string{"TNI preparation", "Preparation program for TNI selection. Schedule, location, fee and contact details are awaiting official material."},
				[2]string{"POLRI preparation", "Preparation program for POLRI selection. Schedule, location, fee and contact details are awaiting official material."},
				[2]string{"Civil-service preparation", "Preparation program for civil-service selection. Schedule, location, fee and contact details are awaiting official material."}),
		},
	}
}
