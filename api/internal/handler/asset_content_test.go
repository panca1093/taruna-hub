package handler

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"

	"github.com/tarunaeduka/bimbel-tryout/api/internal/httpx"
	"github.com/tarunaeduka/bimbel-tryout/api/internal/model"
)

func contentCall(e http.Handler, host, token, id string, headers map[string]string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, "/api/v1/assets/"+id+"/content", nil)
	req.Host = host
	if token != "" {
		req.Header.Set(echo.HeaderAuthorization, "Bearer "+token)
	}
	for name, value := range headers {
		req.Header.Set(name, value)
	}
	return serve(e, req)
}

func TestAssetContentStreamsTheUploadedBytes(t *testing.T) {
	f := newAssetHTTPFixture(t)
	e := f.server(t, time.Minute)
	data := pngBytes(t, 8, 6)
	rec := upload(e, f.hostA, f.tokenAdminA, "a.png", "image/png", data, map[string]string{"purpose": "question_image"})
	if rec.Code != http.StatusCreated {
		t.Fatalf("upload = %d %s", rec.Code, rec.Body.String())
	}
	asset := decodeAsset(t, rec)
	id := asset.ID.String()

	got := contentCall(e, f.hostA, f.tokenAdminA, id, map[string]string{"Origin": "http://localhost:3000"})
	if got.Code != http.StatusOK || !bytes.Equal(got.Body.Bytes(), data) {
		t.Fatalf("uploader reading a staged question image = %d with %d bytes, want 200 and the %d uploaded bytes", got.Code, got.Body.Len(), len(data))
	}
	for header, want := range map[string]string{
		"Content-Type":                     "image/png",
		"Cache-Control":                    "private, no-store",
		"X-Content-Type-Options":           "nosniff",
		"Content-Length":                   strconv.Itoa(len(data)),
		"Access-Control-Allow-Origin":      "http://localhost:3000",
		"Access-Control-Allow-Credentials": "true",
	} {
		if have := got.Header().Get(header); have != want {
			t.Errorf("%s = %q, want %q", header, have, want)
		}
	}

	for name, token := range map[string]string{"operator": f.tokenOperatorA, "student": f.tokenA, "other student": f.tokenSecond} {
		if rec := contentCall(e, f.hostA, token, id, nil); rec.Code != http.StatusNotFound {
			t.Errorf("%s reading someone else's staged image = %d %s, want 404", name, rec.Code, rec.Body.String())
		}
	}

	if _, err := f.finalize(t, f.tenantA, staffOf(f.tenantA, model.RoleAdmin), asset.ID); err != nil {
		t.Fatalf("finalize: %v", err)
	}
	for name, token := range map[string]string{"operator": f.tokenOperatorA, "student": f.tokenA, "other student": f.tokenSecond, "uploader": f.tokenAdminA} {
		rec := contentCall(e, f.hostA, token, id, nil)
		if rec.Code != http.StatusOK || !bytes.Equal(rec.Body.Bytes(), data) || rec.Header().Get("Content-Type") != "image/png" {
			t.Errorf("%s reading a final question image = %d, %d bytes, %q; want 200 and the upload", name, rec.Code, rec.Body.Len(), rec.Header().Get("Content-Type"))
		}
	}
}

func TestAssetContentAnswersLikeTheLinkEndpoint(t *testing.T) {
	f := newAssetHTTPFixture(t)
	e := f.server(t, time.Minute)
	proof := f.uploadPNG(t, e, f.hostA, f.tokenA, "payment_proof")
	image := f.uploadPNG(t, e, f.hostA, f.tokenAdminA, "question_image")

	unknown := contentCall(e, f.hostB, f.tokenAdmB, uuid.NewString(), nil)
	if unknown.Code != http.StatusNotFound {
		t.Fatalf("unknown id = %d, want 404", unknown.Code)
	}
	linkUnknown := assetCall(e, http.MethodGet, f.hostB, f.tokenAdmB, "/api/v1/assets/"+uuid.NewString()+"/url")
	for name, rec := range map[string]*httptest.ResponseRecorder{
		"other tenant's proof":          contentCall(e, f.hostB, f.tokenAdmB, proof.ID.String(), nil),
		"other tenant's question image": contentCall(e, f.hostB, f.tokenB, image.ID.String(), nil),
		"malformed id":                  contentCall(e, f.hostA, f.tokenAdminA, "not-a-uuid", nil),
		"another student's proof":       contentCall(e, f.hostA, f.tokenSecond, proof.ID.String(), nil),
		"staff reading a staged proof":  contentCall(e, f.hostA, f.tokenAdminA, proof.ID.String(), nil),
	} {
		body := errorBody(t, rec)
		if rec.Code != http.StatusNotFound || body.Code != httpx.CodeNotFound || body.Message != errorBody(t, linkUnknown).Message {
			t.Errorf("%s = %d %s, want the 404 of an unknown id", name, rec.Code, rec.Body.String())
		}
	}
	if rec := contentCall(e, f.hostA, f.tokenA, proof.ID.String(), nil); rec.Code != http.StatusOK {
		t.Errorf("uploader reading its own proof = %d, want 200", rec.Code)
	}
	if rec := contentCall(e, f.hostA, "", image.ID.String(), nil); rec.Code != http.StatusUnauthorized {
		t.Errorf("anonymous = %d, want 401", rec.Code)
	}
	if rec := contentCall(e, f.hostB, f.tokenA, image.ID.String(), nil); rec.Code != http.StatusUnauthorized {
		t.Errorf("tenant A's token on tenant B's host = %d, want 401", rec.Code)
	}
}

func TestAssetContentAnswers503WhenStorageIsNotConfigured(t *testing.T) {
	f := newWalletHTTPFixture(t)
	rec := contentCall(f.echo(t), f.hostA, f.tokenA, uuid.NewString(), nil)
	if body := errorBody(t, rec); rec.Code != http.StatusServiceUnavailable || body.Code != httpx.CodeUnavailable {
		t.Errorf("content with no storage = %d %s, want 503 service_unavailable", rec.Code, rec.Body.String())
	}
}
