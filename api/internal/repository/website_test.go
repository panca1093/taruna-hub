package repository

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/tarunaeduka/bimbel-tryout/api/internal/model"
)

func websiteDraft(title string) model.WebsiteContent {
	program := model.WebsiteProgram{Title: "T", Summary: "S"}
	locale := model.WebsiteLocale{
		Home:     model.WebsiteHome{HeroTitle: title, HeroBody: "B", ProgramCTA: "P", PracticeCTA: "L"},
		Programs: map[string]model.WebsiteProgram{"tni": program, "polri": program, "kedinasan": program},
	}
	return model.WebsiteContent{SchemaVersion: 1, Locales: map[string]model.WebsiteLocale{"id": locale, "en": locale}}
}

func TestWebsiteRepositoryScopedPersistence(t *testing.T) {
	f := newWalletFixture(t)
	repo := NewWebsiteRepository(f.pool)
	ctx := context.Background()
	ids := []uuid.UUID{f.tenant.ID, f.other.ID}
	t.Cleanup(func() {
		for _, sql := range []string{`DELETE FROM website_publications WHERE tenant_id = ANY($1)`, `DELETE FROM website_content WHERE tenant_id = ANY($1)`} {
			if _, err := f.pool.Exec(ctx, sql, ids); err != nil {
				t.Errorf("cleanup: %v", err)
			}
		}
	})

	if _, err := repo.Find(ctx, uuid.Nil); !errors.Is(err, ErrScopeRequired) {
		t.Errorf("find without a tenant: err = %v, want ErrScopeRequired", err)
	}
	if _, err := repo.Publications(ctx, uuid.Nil, 0, 10); !errors.Is(err, ErrScopeRequired) {
		t.Errorf("history without a tenant: err = %v, want ErrScopeRequired", err)
	}
	if _, err := repo.Find(ctx, f.tenant.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("find before initialize: err = %v, want ErrNotFound", err)
	}

	tx, err := f.pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	for _, title := range []string{"first", "ignored"} {
		if err := repo.InitializeTx(ctx, tx, f.tenant.ID, websiteDraft(title)); err != nil {
			t.Fatalf("initialize %s: %v", title, err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatalf("commit: %v", err)
	}
	row, err := repo.Find(ctx, f.tenant.ID)
	if err != nil || row.DraftRevision != 1 || row.PublishedRevision != 0 || row.Published != nil || row.Draft.Locales["id"].Home.HeroTitle != "first" {
		t.Fatalf("initialized row = %+v, %v; want one draft at revision 1 that the second initialize did not replace", row, err)
	}
	if _, err := repo.Find(ctx, f.other.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("another tenant sees the row: err = %v", err)
	}

	tx, _ = f.pool.Begin(ctx)
	if _, err := repo.LockTx(ctx, tx, f.tenant.ID); err != nil {
		t.Fatalf("lock: %v", err)
	}
	if err := repo.SaveDraftTx(ctx, tx, f.tenant.ID, websiteDraft("second"), f.admin.ID); err != nil {
		t.Fatalf("save: %v", err)
	}
	pub, err := repo.PublishTx(ctx, tx, f.tenant.ID, 1, 2, f.admin.ID)
	if err != nil || pub.PublishedRevision != 1 || pub.SourceDraftRevision != 2 {
		t.Fatalf("publish = %+v, %v", pub, err)
	}
	if err := tx.Rollback(ctx); err != nil {
		t.Fatalf("rollback: %v", err)
	}
	row, _ = repo.Find(ctx, f.tenant.ID)
	if row.DraftRevision != 1 || row.Published != nil {
		t.Errorf("a rolled-back publish persisted: %+v", row)
	}
	if items, err := repo.Publications(ctx, f.tenant.ID, 0, 10); err != nil || len(items) != 0 {
		t.Errorf("a rolled-back publish left history %v, %v", items, err)
	}
}
