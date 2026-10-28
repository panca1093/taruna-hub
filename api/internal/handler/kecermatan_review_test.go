package handler

import (
	"errors"
	"fmt"
	"net/http"
	"testing"

	"github.com/tarunaeduka/bimbel-tryout/api/internal/httpx"

	"github.com/tarunaeduka/bimbel-tryout/api/internal/service"
)

func TestKecermatanPoolErrorIsAValidationErrorNotA500(t *testing.T) {
	err := kecermatanHTTPError(fmt.Errorf("generate: %w", service.ErrKecermatanPool))
	var he *httpx.Error
	if !errors.As(err, &he) || he.Status != http.StatusUnprocessableEntity || he.Fields["pool"] == "" {
		t.Errorf("ErrKecermatanPool mapped to %v, want a 422", err)
	}
}

// A symbol package never holds an emoji, so a mix that dedupes text by character cannot show one glyph twice,
// once as a symbol and once as an emoji.
func TestSymbolPackageRefusesEmojiSoMixDedupeIsSound(t *testing.T) {
	f := newPackageFixture(t)
	pkg := f.create(t, f.hostA, f.tokenAdminA, "symbol", "Simbol campur")
	commit := f.symbols(t, pkg.ID, "😀 🍎 ❤️ ✅ ♠")
	if commit.Added != 1 || commit.Rejected != 4 {
		t.Errorf("commit = %+v, want only the spade added and the four emoji rejected", commit)
	}
	for _, e := range f.detail(t, f.hostA, f.tokenAdminA, pkg.ID, "").Elements {
		if e.Value != "♠" {
			t.Errorf("symbol package holds %q, want only ♠", e.Value)
		}
	}
	preview := decodePreview(t, elementsCall(t, f.e, f.hostA, f.tokenAdminA, previewPath(pkg.ID), map[string][]string{"symbols": {"😀"}}))
	if len(preview.Items) != 1 || preview.Items[0].Reason != "emoji_in_symbol" {
		t.Errorf("preview of an emoji = %+v, want emoji_in_symbol", preview.Items)
	}
}
