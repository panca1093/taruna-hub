package handler

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"
)

func (f kecHTTPFixture) imageAsset(t *testing.T) uuid.UUID {
	t.Helper()
	id := uuid.New()
	if _, err := f.pool.Exec(context.Background(), `
		INSERT INTO assets (id, tenant_id, purpose, object_key, content_type, size_bytes, status, uploaded_by, finalized_at)
		VALUES ($1, $2, 'item_image', $3, 'image/png', 10, 'final', $4, now())`,
		id, f.tenantA.ID, f.tenantA.ID.String()+"/item_image/"+id.String(), f.adminA.ID); err != nil {
		t.Fatalf("insert asset: %v", err)
	}
	return id
}

// place puts the asset into the first reference element of one generated column of the attempt's snapshot.
func (f kecHTTPFixture) place(t *testing.T, sessionID, assetID uuid.UUID, column int) {
	t.Helper()
	if _, err := f.pool.Exec(context.Background(), `
		UPDATE kecermatan_sessions
		   SET columns = jsonb_set(columns, ARRAY[$2::int::text, 'reference', '0', 'elements', '0'],
		                           jsonb_build_object('kind', 'image', 'key', 'k', 'value', '', 'asset_id', $3::text))
		 WHERE id = $1`, sessionID, column, assetID.String()); err != nil {
		t.Fatalf("place asset: %v", err)
	}
}

func TestKecermatanStudentReadsOnlyImagesItsAttemptShows(t *testing.T) {
	f := newKecHTTPFixture(t)
	f.credit(t, f.studentA, 1)
	session := decodeInto[kecSessionResponse](t, f.call(http.MethodPost, f.testPath("/sessions"), nil, f.tokenA))
	shown, future, unrelated := f.imageAsset(t), f.imageAsset(t), f.imageAsset(t)
	f.place(t, session.ID, shown, 0)
	f.place(t, session.ID, future, 1)
	url := func(id uuid.UUID) string { return "/api/v1/assets/" + id.String() + "/url" }

	if rec := f.call(http.MethodGet, url(shown), nil, f.tokenA); rec.Code != http.StatusOK {
		t.Errorf("own running kolom image = %d %s, want 200", rec.Code, rec.Body.String())
	}
	for name, id := range map[string]uuid.UUID{"a later kolom's image": future, "an unrelated image": unrelated} {
		if rec := f.call(http.MethodGet, url(id), nil, f.tokenA); rec.Code != http.StatusNotFound {
			t.Errorf("%s during the attempt = %d, want 404", name, rec.Code)
		}
	}
	if rec := f.call(http.MethodGet, url(shown), nil, f.tokenSecond); rec.Code != http.StatusNotFound {
		t.Errorf("another student reads the image = %d, want 404", rec.Code)
	}
	if rec := f.call(http.MethodGet, url(shown), nil, f.tokenOperatorA); rec.Code != http.StatusOK {
		t.Errorf("staff keeps reading item images = %d, want 200", rec.Code)
	}

	// Once completed, the whole snapshot is reviewable, still only by its owner.
	f.clock.Advance(10 * 125 * time.Second)
	if done := decodeInto[kecSessionResponse](t, f.call(http.MethodGet, kecSessionPath(session.ID, ""), nil, f.tokenA)); done.Phase != "completed" {
		t.Fatalf("phase = %s, want completed", done.Phase)
	}
	for name, id := range map[string]uuid.UUID{"shown": shown, "later": future} {
		if rec := f.call(http.MethodGet, url(id), nil, f.tokenA); rec.Code != http.StatusOK {
			t.Errorf("%s image in the review = %d, want 200", name, rec.Code)
		}
		if rec := f.call(http.MethodGet, url(id), nil, f.tokenSecond); rec.Code != http.StatusNotFound {
			t.Errorf("%s image for another student after completion = %d, want 404", name, rec.Code)
		}
	}
	if rec := f.call(http.MethodGet, url(unrelated), nil, f.tokenA); rec.Code != http.StatusNotFound {
		t.Errorf("an unrelated image after completion = %d, want 404", rec.Code)
	}
}
