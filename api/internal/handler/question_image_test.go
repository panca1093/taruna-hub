package handler

import (
	"net/http"
	"testing"
	"time"

	"github.com/tarunaeduka/bimbel-tryout/api/internal/httpx"
	"github.com/tarunaeduka/bimbel-tryout/api/internal/model"
)

func imageContent(assetID string) map[string]any {
	return map[string]any{"type": "doc", "content": []any{
		map[string]any{"type": "paragraph", "content": []any{map[string]any{"type": "text", "text": "Perhatikan gambar."}}},
		map[string]any{"type": "image", "attrs": map[string]any{"asset_id": assetID, "alt": "denah", "colwidth": nil}},
	}}
}

func TestQuestionImagesAttachAndStudentsReadFinalOnes(t *testing.T) {
	f := newAssetHTTPFixture(t)
	e := f.server(t, time.Minute)
	program := questionCurriculum(t, e, f.hostA, f.tokenAdminA)

	image := f.uploadPNG(t, e, f.hostA, f.tokenAdminA, "question_image")
	link := "/api/v1/assets/" + image.ID.String() + "/url"
	if rec := assetCall(e, http.MethodGet, f.hostA, f.tokenA, link); rec.Code != http.StatusNotFound {
		t.Errorf("student reading a staged question image = %d, want 404", rec.Code)
	}

	body := pgQuestionBody(program)
	body["content"] = imageContent(image.ID.String())
	rec := sendQuestionJSON(e, http.MethodPost, f.hostA, "/api/v1/questions", body, f.tokenAdminA)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create with an image = %d %s, want 201", rec.Code, rec.Body.String())
	}
	created := decodeQuestion(t, rec)
	if got := model.DocAssetIDs(created.Content); len(got) != 1 || got[0] != image.ID {
		t.Errorf("created content asset ids = %v, want the attached image", got)
	}
	if got := created.Content.Content[1].Attrs; got["colwidth"] != nil || got["alt"] != "denah" {
		t.Errorf("image attrs = %v, want the unknown attribute dropped and alt kept", got)
	}
	if row := f.row(t, image.ID.String()); row.Status != model.AssetFinal {
		t.Errorf("attached image status = %q, want final", row.Status)
	}

	for name, tc := range map[string]struct {
		host, token string
		want        int
	}{
		"student of the tenant": {f.hostA, f.tokenA, http.StatusOK},
		"admin of the tenant":   {f.hostA, f.tokenAdminA, http.StatusOK},
		"student of another":    {f.hostB, f.tokenB, http.StatusNotFound},
		"admin of another":      {f.hostB, f.tokenAdmB, http.StatusNotFound},
	} {
		if rec := assetCall(e, http.MethodGet, tc.host, tc.token, link); rec.Code != tc.want {
			t.Errorf("%s reading the final image = %d, want %d", name, rec.Code, tc.want)
		}
	}

	// Saving another question with the same, already final image by a different staff user still works.
	again := pgQuestionBody(program)
	again["content"] = imageContent(image.ID.String())
	if rec := sendQuestionJSON(e, http.MethodPost, f.hostA, "/api/v1/questions", again, f.tokenOperatorA); rec.Code != http.StatusCreated {
		t.Errorf("operator re-using a final image = %d %s, want 201", rec.Code, rec.Body.String())
	}

	proof := f.uploadPNG(t, e, f.hostA, f.tokenAdminA, "payment_proof")
	foreign := f.uploadPNG(t, e, f.hostB, f.tokenAdmB, "question_image")
	for name, assetID := range map[string]string{"another purpose": proof.ID.String(), "another tenant": foreign.ID.String()} {
		bad := pgQuestionBody(program)
		bad["explanation"] = imageContent(assetID)
		wantError(t, name, sendQuestionJSON(e, http.MethodPost, f.hostA, "/api/v1/questions", bad, f.tokenAdminA),
			http.StatusUnprocessableEntity, httpx.CodeValidation, "explanation")
	}
	inOption := pgQuestionBody(program)
	inOption["options"] = []any{
		map[string]any{"id": "a", "label": d("3")},
		map[string]any{"id": "b", "label": imageContent(proof.ID.String())},
	}
	wantError(t, "an option image of another purpose",
		sendQuestionJSON(e, http.MethodPost, f.hostA, "/api/v1/questions", inOption, f.tokenAdminA),
		http.StatusUnprocessableEntity, httpx.CodeValidation, "options[1].label")
	if row := f.row(t, proof.ID.String()); row.Status != model.AssetStaged {
		t.Errorf("rejected image status = %q, want it left staged", row.Status)
	}
}
