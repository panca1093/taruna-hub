package service

import (
	"context"
	"errors"
	"reflect"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/tarunaeduka/bimbel-tryout/api/internal/model"
	"github.com/tarunaeduka/bimbel-tryout/api/internal/repository"
)

type websiteHarness struct {
	walletHarness
	web *WebsiteService
}

func newWebsiteHarness(t *testing.T) websiteHarness {
	t.Helper()
	h := websiteHarness{walletHarness: newWalletHarness(t)}
	h.web = NewWebsiteService(h.pool, repository.NewWebsiteRepository(h.pool))
	// Registered after the wallet cleanup, so the website rows go before the users and tenants they reference.
	t.Cleanup(func() {
		wipeWebsite(t, h.pool, []uuid.UUID{h.tenant.ID, h.other.ID})
	})
	return h
}

func wipeWebsite(t *testing.T, pool *pgxpool.Pool, ids []uuid.UUID) {
	t.Helper()
	for _, sql := range []string{
		`DELETE FROM website_publications WHERE tenant_id = ANY($1)`,
		`DELETE FROM website_content WHERE tenant_id = ANY($1)`,
	} {
		if _, err := pool.Exec(context.Background(), sql, ids); err != nil {
			t.Errorf("cleanup website fixture: %v", err)
		}
	}
}

func (h websiteHarness) admin1() model.Principal { return h.principal(h.tenant, h.admin) }

func (h websiteHarness) workbench(t *testing.T) model.WebsiteWorkbench {
	t.Helper()
	wb, err := h.web.Workbench(context.Background(), h.scope(h.tenant), h.admin1())
	if err != nil {
		t.Fatalf("workbench: %v", err)
	}
	return wb
}

func (h websiteHarness) save(t *testing.T, expected int64, headline string) model.WebsiteWorkbench {
	t.Helper()
	content := DefaultWebsiteContent()
	content.Locales["id"] = withHeadline(content.Locales["id"], headline)
	wb, err := h.web.Save(context.Background(), h.scope(h.tenant), h.admin1(), SaveWebsiteInput{ExpectedDraftRevision: expected, Content: content})
	if err != nil {
		t.Fatalf("save %q: %v", headline, err)
	}
	return wb
}

func (h websiteHarness) publish(expectedDraft, expectedPublished int64) (model.WebsitePublication, error) {
	return h.web.Publish(context.Background(), h.scope(h.tenant), h.admin1(),
		PublishWebsiteInput{ExpectedDraftRevision: expectedDraft, ExpectedPublishedRevision: expectedPublished})
}

func (h websiteHarness) count(t *testing.T, table string, tenantID uuid.UUID) int {
	t.Helper()
	var n int
	if err := h.pool.QueryRow(context.Background(), `SELECT count(*) FROM `+table+` WHERE tenant_id = $1`, tenantID).Scan(&n); err != nil {
		t.Fatalf("count %s: %v", table, err)
	}
	return n
}

func withHeadline(l model.WebsiteLocale, headline string) model.WebsiteLocale {
	l.Home.HeroTitle = headline
	return l
}

func TestWebsiteInitializeConcurrent(t *testing.T) {
	h := newWebsiteHarness(t)
	ctx := context.Background()

	if _, err := h.web.Published(ctx, h.scope(h.tenant)); !errors.Is(err, ErrWebsiteNotPublished) {
		t.Fatalf("published before any publication: err = %v, want ErrWebsiteNotPublished", err)
	}
	if n := h.count(t, "website_content", h.tenant.ID); n != 0 {
		t.Fatalf("a public read created %d rows, want 0", n)
	}

	const readers = 12
	results := make([]model.WebsiteWorkbench, readers)
	errs := make([]error, readers)
	var wg sync.WaitGroup
	start := make(chan struct{})
	for i := range readers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			results[i], errs[i] = h.web.Workbench(ctx, h.scope(h.tenant), h.admin1())
		}()
	}
	close(start)
	wg.Wait()

	for i := range readers {
		if errs[i] != nil {
			t.Fatalf("reader %d: %v", i, errs[i])
		}
		if results[i].DraftRevision != 1 || results[i].PublishedRevision != 0 || results[i].Published != nil {
			t.Errorf("reader %d: revisions %d/%d published %v, want 1/0/nil", i, results[i].DraftRevision, results[i].PublishedRevision, results[i].Published)
		}
		if !reflect.DeepEqual(results[i].Draft, results[0].Draft) {
			t.Errorf("reader %d saw a different draft", i)
		}
	}
	if n := h.count(t, "website_content", h.tenant.ID); n != 1 {
		t.Errorf("rows = %d, want exactly one", n)
	}
	if n := h.count(t, "website_publications", h.tenant.ID); n != 0 {
		t.Errorf("history rows = %d, an initialization must never publish", n)
	}
	if _, err := h.web.Published(ctx, h.scope(h.tenant)); !errors.Is(err, ErrWebsiteNotPublished) {
		t.Errorf("published after initialization: err = %v, want ErrWebsiteNotPublished", err)
	}
	if _, err := normalizeWebsiteContent(results[0].Draft); err != nil {
		t.Errorf("the seeded default breaks its own contract: %v", err)
	}
}

func TestWebsiteSaveRevision(t *testing.T) {
	h := newWebsiteHarness(t)
	ctx, scope, admin := context.Background(), h.scope(h.tenant), h.admin1()

	first := h.workbench(t)
	if first.UpdatedBy != nil {
		t.Errorf("a fresh draft has updated_by %v, want nil", *first.UpdatedBy)
	}

	same, err := h.web.Save(ctx, scope, admin, SaveWebsiteInput{ExpectedDraftRevision: 1, Content: first.Draft})
	if err != nil {
		t.Fatalf("no-op save: %v", err)
	}
	if same.DraftRevision != 1 || !same.UpdatedAt.Equal(first.UpdatedAt) || same.UpdatedBy != nil {
		t.Errorf("no-op save moved revision/time/actor: %+v", same)
	}

	changed := h.save(t, 1, "  Judul baru  ")
	if changed.DraftRevision != 2 || changed.Draft.Locales["id"].Home.HeroTitle != "Judul baru" {
		t.Errorf("changed save: revision %d title %q, want 2 and the trimmed title", changed.DraftRevision, changed.Draft.Locales["id"].Home.HeroTitle)
	}
	if changed.UpdatedBy == nil || *changed.UpdatedBy != h.admin.ID {
		t.Errorf("updated_by = %v, want the authenticated admin", changed.UpdatedBy)
	}

	stale := DefaultWebsiteContent()
	stale.Locales["id"] = withHeadline(stale.Locales["id"], "Timpa diam-diam")
	if _, err := h.web.Save(ctx, scope, admin, SaveWebsiteInput{ExpectedDraftRevision: 1, Content: stale}); !errors.Is(err, ErrWebsiteConflict) {
		t.Fatalf("stale save: err = %v, want ErrWebsiteConflict", err)
	}
	if got := h.workbench(t); got.DraftRevision != 2 || got.Draft.Locales["id"].Home.HeroTitle != "Judul baru" {
		t.Errorf("a stale save overwrote the newer draft: %+v", got.Draft.Locales["id"].Home)
	}

	bad := DefaultWebsiteContent()
	bad.Locales["id"] = withHeadline(bad.Locales["id"], "   ")
	var input *InputError
	if _, err := h.web.Save(ctx, scope, admin, SaveWebsiteInput{ExpectedDraftRevision: 2, Content: bad}); !errors.As(err, &input) {
		t.Fatalf("invalid save: err = %v, want InputError", err)
	}
	if h.workbench(t).DraftRevision != 2 {
		t.Error("an invalid save moved the revision")
	}

	if _, err := h.publish(2, 0); err != nil {
		t.Fatalf("publish: %v", err)
	}
	h.save(t, 2, "Draf lanjutan")
	pub, err := h.web.Published(ctx, scope)
	if err != nil {
		t.Fatalf("published: %v", err)
	}
	if pub.PublishedRevision != 1 || pub.Content.Locales["id"].Home.HeroTitle != "Judul baru" {
		t.Errorf("saving touched the published snapshot: %+v", pub)
	}

	for _, p := range []model.Principal{h.principal(h.tenant, h.operator), h.principal(h.tenant, h.student)} {
		if _, err := h.web.Save(ctx, scope, p, SaveWebsiteInput{ExpectedDraftRevision: 3, Content: first.Draft}); !errors.Is(err, ErrWebsiteForbidden) {
			t.Errorf("%s save: err = %v, want ErrWebsiteForbidden", p.Role, err)
		}
		if _, err := h.web.Workbench(ctx, scope, p); !errors.Is(err, ErrWebsiteForbidden) {
			t.Errorf("%s workbench: err = %v, want ErrWebsiteForbidden", p.Role, err)
		}
	}
	if _, err := h.web.Workbench(ctx, scope, h.principal(h.other, h.otherStudent)); !errors.Is(err, ErrTenantNotFound) {
		t.Errorf("a principal of another tenant: err = %v, want ErrTenantNotFound", err)
	}
}

func TestWebsitePublishAtomic(t *testing.T) {
	h := newWebsiteHarness(t)
	ctx := context.Background()
	h.workbench(t)

	pub, err := h.publish(1, 0)
	if err != nil {
		t.Fatalf("first publish: %v", err)
	}
	if pub.PublishedRevision != 1 || pub.SourceDraftRevision != 1 || !pub.Changed || pub.PublishedBy != h.admin.ID {
		t.Errorf("first publication = %+v, want revision 1 from draft 1 by the admin", pub)
	}
	wb := h.workbench(t)
	if wb.DraftRevision != 1 || wb.PublishedRevision != 1 || wb.Published == nil || !reflect.DeepEqual(*wb.Published, wb.Draft) {
		t.Errorf("after publish: revisions %d/%d, published equals draft = %v", wb.DraftRevision, wb.PublishedRevision, wb.Published != nil && reflect.DeepEqual(*wb.Published, wb.Draft))
	}
	if n := h.count(t, "website_publications", h.tenant.ID); n != 1 {
		t.Errorf("history rows = %d, want 1", n)
	}

	h.save(t, 1, "Versi kedua")
	if _, err := h.publish(2, 0); !errors.Is(err, ErrWebsiteConflict) {
		t.Errorf("stale published revision: err = %v, want ErrWebsiteConflict", err)
	}
	if _, err := h.publish(1, 1); !errors.Is(err, ErrWebsiteConflict) {
		t.Errorf("stale draft revision: err = %v, want ErrWebsiteConflict", err)
	}

	// A history row already holding the next revision makes the insert fail after the snapshot was copied.
	if _, err := h.pool.Exec(ctx, `INSERT INTO website_publications (tenant_id, published_revision, source_draft_revision, published_by)
		VALUES ($1, 2, 99, $2)`, h.tenant.ID, h.admin.ID); err != nil {
		t.Fatalf("seed the colliding history row: %v", err)
	}
	if _, err := h.publish(2, 1); err == nil {
		t.Fatal("publish with a failing history insert succeeded")
	}
	got, err := h.web.Published(ctx, h.scope(h.tenant))
	if err != nil {
		t.Fatalf("published after the failed publish: %v", err)
	}
	if got.PublishedRevision != 1 || got.Content.Locales["id"].Home.HeroTitle == "Versi kedua" {
		t.Errorf("the failed publish leaked: revision %d title %q", got.PublishedRevision, got.Content.Locales["id"].Home.HeroTitle)
	}
	if n := h.count(t, "website_publications", h.tenant.ID); n != 2 {
		t.Errorf("history rows = %d, want the original and the seeded one only", n)
	}
	if _, err := h.pool.Exec(ctx, `DELETE FROM website_publications WHERE tenant_id = $1 AND source_draft_revision = 99`, h.tenant.ID); err != nil {
		t.Fatalf("remove the colliding row: %v", err)
	}

	second, err := h.publish(2, 1)
	if err != nil || second.PublishedRevision != 2 || second.SourceDraftRevision != 2 || !second.Changed {
		t.Fatalf("second publish = %+v, %v; want revision 2 from draft 2", second, err)
	}

	// An equal-text draft on a new revision has nothing to publish.
	h.save(t, 2, "Versi ketiga")
	h.save(t, 3, "Versi kedua")
	back := h.workbench(t)
	if back.DraftRevision != 4 {
		t.Fatalf("draft revision = %d, want 4", back.DraftRevision)
	}
	noop, err := h.publish(4, 2)
	if err != nil || noop.Changed || noop.PublishedRevision != 2 {
		t.Errorf("publish of a draft equal to the published text = %+v, %v; want changed=false at revision 2", noop, err)
	}
	if n := h.count(t, "website_publications", h.tenant.ID); n != 2 {
		t.Errorf("history rows = %d after an equal publish, want 2", n)
	}

	for _, p := range []model.Principal{h.principal(h.tenant, h.operator), h.principal(h.tenant, h.student)} {
		if _, err := h.web.Publish(ctx, h.scope(h.tenant), p, PublishWebsiteInput{ExpectedDraftRevision: 4, ExpectedPublishedRevision: 2}); !errors.Is(err, ErrWebsiteForbidden) {
			t.Errorf("%s publish: err = %v, want ErrWebsiteForbidden", p.Role, err)
		}
	}
}

func TestWebsitePublishRetry(t *testing.T) {
	h := newWebsiteHarness(t)
	h.workbench(t)

	first, err := h.publish(1, 0)
	if err != nil {
		t.Fatalf("publish: %v", err)
	}
	retry, err := h.publish(1, 0)
	if err != nil {
		t.Fatalf("lost-response retry: %v", err)
	}
	if !reflect.DeepEqual(retry, first) {
		t.Errorf("retry = %+v, want the recorded publication %+v", retry, first)
	}
	if n := h.count(t, "website_publications", h.tenant.ID); n != 1 {
		t.Errorf("history rows = %d after the retry, want 1", n)
	}

	h.save(t, 1, "Draf yang belum dilihat")
	if _, err := h.publish(1, 0); !errors.Is(err, ErrWebsiteConflict) {
		t.Fatalf("retry after the draft moved: err = %v, want ErrWebsiteConflict", err)
	}
	pub, err := h.web.Published(context.Background(), h.scope(h.tenant))
	if err != nil || pub.PublishedRevision != 1 || pub.Content.Locales["id"].Home.HeroTitle == "Draf yang belum dilihat" {
		t.Errorf("a retry published an unseen draft: %+v, %v", pub, err)
	}

	t.Run("concurrent same draft", func(t *testing.T) {
		c := newWebsiteHarness(t)
		c.workbench(t)
		const publishers = 10
		got := make([]model.WebsitePublication, publishers)
		errs := make([]error, publishers)
		var wg sync.WaitGroup
		start := make(chan struct{})
		for i := range publishers {
			wg.Add(1)
			go func() {
				defer wg.Done()
				<-start
				got[i], errs[i] = c.publish(1, 0)
			}()
		}
		close(start)
		wg.Wait()
		for i := range publishers {
			if errs[i] != nil {
				t.Fatalf("publisher %d: %v", i, errs[i])
			}
			if !reflect.DeepEqual(got[i], got[0]) || got[i].PublishedRevision != 1 {
				t.Errorf("publisher %d = %+v, want the one recorded publication %+v", i, got[i], got[0])
			}
		}
		if n := c.count(t, "website_publications", c.tenant.ID); n != 1 {
			t.Errorf("history rows = %d, want exactly one", n)
		}
	})
}

func TestWebsiteHistoryPagination(t *testing.T) {
	h := newWebsiteHarness(t)
	ctx, scope, admin := context.Background(), h.scope(h.tenant), h.admin1()

	empty, err := h.web.History(ctx, scope, admin, WebsiteHistoryInput{})
	if err != nil || len(empty.Items) != 0 || empty.Items == nil || empty.NextCursor != nil {
		t.Fatalf("history before any publication = %+v, %v; want an empty non-nil list", empty, err)
	}

	h.workbench(t)
	if _, err := h.publish(1, 0); err != nil {
		t.Fatalf("publish 1: %v", err)
	}
	for rev := int64(2); rev <= 5; rev++ {
		wb := h.save(t, rev-1, "Versi "+string(rune('a'+rev)))
		if _, err := h.publish(wb.DraftRevision, rev-1); err != nil {
			t.Fatalf("publish %d: %v", rev, err)
		}
	}

	var seen []int64
	cursor := ""
	for page := 0; page < 5; page++ {
		list, err := h.web.History(ctx, scope, admin, WebsiteHistoryInput{Limit: 2, Cursor: cursor})
		if err != nil {
			t.Fatalf("page %d: %v", page, err)
		}
		for _, item := range list.Items {
			seen = append(seen, item.PublishedRevision)
		}
		if list.NextCursor == nil {
			break
		}
		cursor = *list.NextCursor
	}
	if want := []int64{5, 4, 3, 2, 1}; !reflect.DeepEqual(seen, want) {
		t.Errorf("history order = %v, want %v", seen, want)
	}

	first, _ := h.web.History(ctx, scope, admin, WebsiteHistoryInput{Limit: 2})
	again, _ := h.web.History(ctx, scope, admin, WebsiteHistoryInput{Limit: 2})
	if first.NextCursor == nil || again.NextCursor == nil || *first.NextCursor != *again.NextCursor {
		t.Errorf("the cursor is not deterministic: %v vs %v", first.NextCursor, again.NextCursor)
	}
	if def, err := h.web.History(ctx, scope, admin, WebsiteHistoryInput{}); err != nil || len(def.Items) != 5 || def.NextCursor != nil {
		t.Errorf("default page = %d items, cursor %v, err %v; want all 5 and no cursor", len(def.Items), def.NextCursor, err)
	}

	var input *InputError
	for name, in := range map[string]WebsiteHistoryInput{
		"limit above the maximum": {Limit: 51},
		"negative limit":          {Limit: -1},
		"garbage cursor":          {Cursor: "not-a-cursor"},
	} {
		if _, err := h.web.History(ctx, scope, admin, in); !errors.As(err, &input) {
			t.Errorf("%s: err = %v, want InputError", name, err)
		}
	}

	if _, err := h.web.History(ctx, scope, h.principal(h.tenant, h.operator), WebsiteHistoryInput{}); !errors.Is(err, ErrWebsiteForbidden) {
		t.Errorf("operator history: err = %v, want ErrWebsiteForbidden", err)
	}
	otherAdmin := walletUser(t, repository.NewUserRepository(h.pool), h.other.ID, model.RoleAdmin, "Admin Lain")
	foreign, err := h.web.History(ctx, h.scope(h.other), h.principal(h.other, otherAdmin), WebsiteHistoryInput{})
	if err != nil || len(foreign.Items) != 0 {
		t.Errorf("another tenant's history = %+v, %v; want empty", foreign, err)
	}
}
