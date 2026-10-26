package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"image"
	"image/png"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"net/url"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"

	"github.com/tarunaeduka/bimbel-tryout/api/internal/config"
	"github.com/tarunaeduka/bimbel-tryout/api/internal/httpx"
	"github.com/tarunaeduka/bimbel-tryout/api/internal/infra"
	"github.com/tarunaeduka/bimbel-tryout/api/internal/model"
	"github.com/tarunaeduka/bimbel-tryout/api/internal/repository"
	"github.com/tarunaeduka/bimbel-tryout/api/internal/service"
)

const fiveMiB = 5 << 20

// countingStore sees every write the service attempts, so a test can prove nothing was written.
type countingStore struct {
	service.ObjectStore
	puts       atomic.Int32
	failDelete bool
}

func (s *countingStore) Put(ctx context.Context, key string, data []byte, contentType string) error {
	s.puts.Add(1)
	return s.ObjectStore.Put(ctx, key, data, contentType)
}

func (s *countingStore) Delete(ctx context.Context, key string) error {
	if s.failDelete {
		return errors.New("storage down")
	}
	return s.ObjectStore.Delete(ctx, key)
}

type assetHTTPFixture struct {
	walletHTTPFixture
	store *countingStore
	infra *infra.Storage
}

func newAssetHTTPFixture(t *testing.T) assetHTTPFixture {
	t.Helper()
	endpoint := os.Getenv("S3_ENDPOINT")
	if endpoint == "" {
		t.Skip("S3_ENDPOINT not set")
	}
	f := assetHTTPFixture{walletHTTPFixture: newWalletHTTPFixture(t)}
	storage, err := infra.NewStorage(config.S3Config{
		Endpoint: endpoint, AccessKey: os.Getenv("S3_ACCESS_KEY"), SecretKey: os.Getenv("S3_SECRET_KEY"),
		Bucket: os.Getenv("S3_BUCKET"), UseSSL: os.Getenv("S3_USE_SSL") == "true",
	})
	if err != nil {
		t.Fatalf("new storage: %v", err)
	}
	if err := storage.EnsureBucket(context.Background(), true); err != nil {
		t.Fatalf("ensure bucket: %v", err)
	}
	f.infra, f.store = storage, &countingStore{ObjectStore: storage}
	// Keys start with the fixture tenant's id, so this removes only what this test wrote.
	t.Cleanup(func() {
		rows, err := f.pool.Query(context.Background(), `SELECT object_key FROM assets WHERE tenant_id = ANY($1)`,
			[]uuid.UUID{f.tenantA.ID, f.tenantB.ID})
		if err != nil {
			t.Errorf("list fixture objects: %v", err)
			return
		}
		defer rows.Close()
		for rows.Next() {
			var key string
			if err := rows.Scan(&key); err == nil {
				_ = storage.Delete(context.Background(), key)
			}
		}
	})
	return f
}

func (f assetHTTPFixture) server(t *testing.T, ttl time.Duration) *echo.Echo {
	t.Helper()
	return f.echoWithAssets(t, newAssetService(f.tenantFixture, f.store, ttl))
}

func pngBytes(t *testing.T, w, h int) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := png.Encode(&buf, image.NewGray(image.Rect(0, 0, w, h))); err != nil {
		t.Fatalf("encode png: %v", err)
	}
	return buf.Bytes()
}

// upload sends a multipart body whose part claims whatever name and type the test wants.
func upload(e *echo.Echo, host, token, filename, partType string, data []byte, fields map[string]string) *httptest.ResponseRecorder {
	var body bytes.Buffer
	w := multipart.NewWriter(&body)
	for name, value := range fields {
		_ = w.WriteField(name, value)
	}
	h := textproto.MIMEHeader{}
	h.Set("Content-Disposition", `form-data; name="file"; filename="`+filename+`"`)
	h.Set("Content-Type", partType)
	part, _ := w.CreatePart(h)
	_, _ = part.Write(data)
	_ = w.Close()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/assets", &body)
	req.Header.Set(echo.HeaderContentType, w.FormDataContentType())
	req.Host = host
	if token != "" {
		req.Header.Set(echo.HeaderAuthorization, "Bearer "+token)
	}
	return serve(e, req)
}

func assetCall(e *echo.Echo, method, host, token, path string) *httptest.ResponseRecorder {
	return sendTryoutJSON(e, method, host, path, nil, token)
}

func decodeAsset(t *testing.T, rec *httptest.ResponseRecorder) assetResponse {
	t.Helper()
	var a assetResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &a); err != nil {
		t.Fatalf("body %s is not an asset: %v", rec.Body.String(), err)
	}
	return a
}

func errorBody(t *testing.T, rec *httptest.ResponseRecorder) httpx.Body {
	t.Helper()
	var env httpx.Envelope
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatalf("body %s is not an error envelope: %v", rec.Body.String(), err)
	}
	return env.Error
}

func (f assetHTTPFixture) uploadPNG(t *testing.T, e *echo.Echo, host, token, purpose string) assetResponse {
	t.Helper()
	rec := upload(e, host, token, "a.png", "image/png", pngBytes(t, 4, 4), map[string]string{"purpose": purpose})
	if rec.Code != http.StatusCreated {
		t.Fatalf("upload %s = %d %s, want 201", purpose, rec.Code, rec.Body.String())
	}
	return decodeAsset(t, rec)
}

func fetch(t *testing.T, rawURL string) (int, []byte, http.Header) {
	t.Helper()
	resp, err := http.Get(rawURL)
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, body, resp.Header
}

func (f assetHTTPFixture) row(t *testing.T, id string) model.Asset {
	t.Helper()
	a, err := repository.NewAssetRepository(f.pool).Find(context.Background(), f.pool, f.tenantA.ID, uuid.MustParse(id))
	if err != nil {
		t.Fatalf("read asset %s: %v", id, err)
	}
	return a
}

func TestAssetUploadStoresTheDetectedTypeUnderAServerKey(t *testing.T) {
	f := newAssetHTTPFixture(t)
	e := f.server(t, time.Minute)
	data := pngBytes(t, 8, 8)

	rec := upload(e, f.hostA, f.tokenA, "totally.svg", "image/svg+xml", data, map[string]string{"purpose": "payment_proof"})
	if rec.Code != http.StatusCreated {
		t.Fatalf("upload = %d %s, want 201", rec.Code, rec.Body.String())
	}
	if got := rec.Header().Get("Cache-Control"); got != "no-store" {
		t.Errorf("Cache-Control = %q, want no-store", got)
	}
	a := decodeAsset(t, rec)
	if a.ContentType != "image/png" || a.Status != "staged" || a.Purpose != "payment_proof" || a.SizeBytes != int64(len(data)) {
		t.Errorf("asset = %+v, want a staged payment_proof, image/png, %d bytes", a, len(data))
	}
	row := f.row(t, a.ID.String())
	if want := f.tenantA.ID.String() + "/payment_proof/" + a.ID.String(); row.ObjectKey != want {
		t.Errorf("object key = %q, want %q", row.ObjectKey, want)
	}
	if row.UploadedBy == nil || *row.UploadedBy != f.studentA.ID {
		t.Errorf("uploaded_by = %v, want the caller %s", row.UploadedBy, f.studentA.ID)
	}
	if bytes.Contains(rec.Body.Bytes(), []byte(row.ObjectKey)) || bytes.Contains(rec.Body.Bytes(), []byte("object_key")) {
		t.Errorf("the response leaks the object key: %s", rec.Body.String())
	}
	signed, err := f.infra.PresignGet(context.Background(), row.ObjectKey, time.Minute)
	if err != nil {
		t.Fatalf("presign: %v", err)
	}
	if status, body, header := fetch(t, signed); status != 200 || !bytes.Equal(body, data) || header.Get("Content-Type") != "image/png" {
		t.Errorf("stored object = %d, %d bytes, %q; want 200, the upload, image/png", status, len(body), header.Get("Content-Type"))
	}
}

func TestAssetClientInputCannotChangeOwnerOrKey(t *testing.T) {
	f := newAssetHTTPFixture(t)
	e := f.server(t, time.Minute)
	forged := uuid.NewString()

	rec := upload(e, f.hostA, f.tokenA, "../../"+f.tenantB.ID.String()+"/question_image/x.png", "image/png", pngBytes(t, 4, 4), map[string]string{
		"purpose":     "payment_proof",
		"tenant_id":   f.tenantB.ID.String(),
		"id":          forged,
		"object_key":  f.tenantB.ID.String() + "/question_image/" + forged,
		"uploaded_by": f.secondStudent.ID.String(),
		"status":      "final",
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("upload = %d %s, want 201", rec.Code, rec.Body.String())
	}
	a := decodeAsset(t, rec)
	row := f.row(t, a.ID.String())
	if a.ID.String() == forged || row.TenantID != f.tenantA.ID || row.Status != model.AssetStaged ||
		row.UploadedBy == nil || *row.UploadedBy != f.studentA.ID ||
		row.ObjectKey != f.tenantA.ID.String()+"/payment_proof/"+a.ID.String() {
		t.Errorf("row = %+v, want tenant A, staged, the caller as uploader, a key built from the server's id", row)
	}

	for _, purpose := range []string{"../question_image", "payment_proof/../question_image", "PAYMENT_PROOF", ""} {
		rec := upload(e, f.hostA, f.tokenA, "a.png", "image/png", pngBytes(t, 4, 4), map[string]string{"purpose": purpose})
		if rec.Code != http.StatusUnprocessableEntity || errorBody(t, rec).Fields["purpose"] == "" {
			t.Errorf("purpose %q = %d %s, want 422 on purpose", purpose, rec.Code, rec.Body.String())
		}
	}
}

func TestAssetRefusesBadFilesBeforeAnythingIsStored(t *testing.T) {
	f := newAssetHTTPFixture(t)
	e := f.server(t, time.Minute)
	good := pngBytes(t, 32, 32)
	flipped := append([]byte(nil), good...)
	flipped[len(flipped)/2] ^= 0xff

	cases := map[string]struct {
		filename, partType string
		data               []byte
	}{
		"text named png":    {"proof.png", "image/png", []byte("this is not an image")},
		"svg named png":     {"proof.png", "image/png", []byte(`<svg xmlns="http://www.w3.org/2000/svg"><script>alert(1)</script></svg>`)},
		"svg named svg":     {"proof.svg", "image/svg+xml", []byte(`<svg xmlns="http://www.w3.org/2000/svg"/>`)},
		"html named jpg":    {"proof.jpg", "image/jpeg", []byte("<html><script>alert(1)</script></html>")},
		"pdf named png":     {"proof.png", "image/png", []byte("%PDF-1.7\n")},
		"corrupt png":       {"proof.png", "image/png", flipped},
		"truncated png":     {"proof.png", "image/png", good[:len(good)/2]},
		"absurd dimensions": {"proof.png", "image/png", pngBytes(t, 8001, 1)},
		"empty file":        {"proof.png", "image/png", nil},
	}
	for name, c := range cases {
		rec := upload(e, f.hostA, f.tokenA, c.filename, c.partType, c.data, map[string]string{"purpose": "payment_proof"})
		if rec.Code != http.StatusUnprocessableEntity || errorBody(t, rec).Fields["file"] == "" {
			t.Errorf("%s = %d %s, want 422 on file", name, rec.Code, rec.Body.String())
		}
	}
	if puts := f.store.puts.Load(); puts != 0 {
		t.Errorf("storage saw %d writes for rejected files, want 0", puts)
	}
	var rows int
	if err := f.pool.QueryRow(context.Background(), `SELECT count(*) FROM assets WHERE tenant_id = $1`, f.tenantA.ID).Scan(&rows); err != nil || rows != 0 {
		t.Errorf("rejected files left %d rows (err %v), want 0", rows, err)
	}
}

func TestAssetSizeLimitIsFiveMiBExactly(t *testing.T) {
	f := newAssetHTTPFixture(t)
	e := f.server(t, time.Minute)
	padded := func(size int) []byte {
		return append(pngBytes(t, 4, 4), make([]byte, size-len(pngBytes(t, 4, 4)))...)
	}
	fields := map[string]string{"purpose": "payment_proof"}

	if rec := upload(e, f.hostA, f.tokenA, "a.png", "image/png", padded(fiveMiB), fields); rec.Code != http.StatusCreated {
		t.Errorf("exactly 5 MiB = %d %s, want 201", rec.Code, rec.Body.String())
	}
	before := f.store.puts.Load()
	for name, size := range map[string]int{"5 MiB + 1 byte": fiveMiB + 1, "far over the body cap": 8 << 20} {
		rec := upload(e, f.hostA, f.tokenA, "a.png", "image/png", padded(size), fields)
		if rec.Code != http.StatusRequestEntityTooLarge || errorBody(t, rec).Code != httpx.CodeTooLarge {
			t.Errorf("%s = %d %s, want 413 payload_too_large", name, rec.Code, rec.Body.String())
		}
	}
	if f.store.puts.Load() != before {
		t.Error("an oversize upload reached storage")
	}
}

func TestAssetUploadChecksRoleAgainstPurpose(t *testing.T) {
	f := newAssetHTTPFixture(t)
	e := f.server(t, time.Minute)
	data := pngBytes(t, 4, 4)

	if rec := upload(e, f.hostA, f.tokenA, "a.png", "image/png", data, map[string]string{"purpose": "question_image"}); rec.Code != http.StatusForbidden {
		t.Errorf("student question_image = %d %s, want 403", rec.Code, rec.Body.String())
	}
	for name, token := range map[string]string{"admin": f.tokenAdminA, "operator": f.tokenOperatorA} {
		for _, purpose := range []string{"question_image", "payment_proof"} {
			if rec := upload(e, f.hostA, token, "a.png", "image/png", data, map[string]string{"purpose": purpose}); rec.Code != http.StatusCreated {
				t.Errorf("%s %s = %d %s, want 201", name, purpose, rec.Code, rec.Body.String())
			}
		}
	}
	if rec := upload(e, f.hostA, "", "a.png", "image/png", data, map[string]string{"purpose": "payment_proof"}); rec.Code != http.StatusUnauthorized {
		t.Errorf("anonymous = %d, want 401", rec.Code)
	}
	if rec := upload(e, f.hostB, f.tokenA, "a.png", "image/png", data, map[string]string{"purpose": "payment_proof"}); rec.Code != http.StatusUnauthorized {
		t.Errorf("tenant A's token on tenant B's host = %d, want 401", rec.Code)
	}
	req := httptest.NewRequest(http.MethodPost, "/api/v1/assets", bytes.NewReader(nil))
	req.Host = f.hostA
	req.Header.Set(echo.HeaderAuthorization, "Bearer "+f.tokenA)
	req.Header.Set(echo.HeaderContentType, "application/json")
	if rec := serve(e, req); rec.Code != http.StatusUnprocessableEntity {
		t.Errorf("a non-multipart body = %d %s, want 422", rec.Code, rec.Body.String())
	}
}

func TestAssetOfAnotherTenantIsNotFoundNotForbidden(t *testing.T) {
	f := newAssetHTTPFixture(t)
	e := f.server(t, time.Minute)
	a := f.uploadPNG(t, e, f.hostA, f.tokenA, "payment_proof")
	id := a.ID.String()

	unknown := assetCall(e, http.MethodGet, f.hostB, f.tokenAdmB, "/api/v1/assets/"+uuid.NewString()+"/url")
	if unknown.Code != http.StatusNotFound {
		t.Fatalf("unknown id = %d, want 404", unknown.Code)
	}
	for name, rec := range map[string]*httptest.ResponseRecorder{
		"B admin url":      assetCall(e, http.MethodGet, f.hostB, f.tokenAdmB, "/api/v1/assets/"+id+"/url"),
		"B student url":    assetCall(e, http.MethodGet, f.hostB, f.tokenB, "/api/v1/assets/"+id+"/url"),
		"B admin finalize": assetCall(e, http.MethodPost, f.hostB, f.tokenAdmB, "/api/v1/assets/"+id+"/finalize"),
	} {
		got := errorBody(t, rec)
		if rec.Code != http.StatusNotFound || got.Code != httpx.CodeNotFound || got.Message != errorBody(t, unknown).Message {
			t.Errorf("%s = %d %s, want the same 404 as an unknown id", name, rec.Code, rec.Body.String())
		}
	}
	if row := f.row(t, id); row.Status != model.AssetStaged {
		t.Errorf("status after tenant B's finalize = %q, want staged", row.Status)
	}
	if rec := assetCall(e, http.MethodGet, f.hostA, f.tokenAdminA, "/api/v1/assets/not-a-uuid/url"); rec.Code != http.StatusNotFound {
		t.Errorf("malformed id = %d, want 404", rec.Code)
	}
}

func TestAssetStagedIsHiddenFromEveryoneButTheUploader(t *testing.T) {
	f := newAssetHTTPFixture(t)
	e := f.server(t, time.Minute)
	a := f.uploadPNG(t, e, f.hostA, f.tokenA, "payment_proof")
	url := "/api/v1/assets/" + a.ID.String() + "/url"

	for name, token := range map[string]string{"other student": f.tokenSecond, "admin": f.tokenAdminA, "operator": f.tokenOperatorA} {
		if rec := assetCall(e, http.MethodGet, f.hostA, token, url); rec.Code != http.StatusNotFound {
			t.Errorf("%s reading a staged asset = %d %s, want 404", name, rec.Code, rec.Body.String())
		}
	}
	if rec := assetCall(e, http.MethodGet, f.hostA, f.tokenA, url); rec.Code != http.StatusOK {
		t.Fatalf("uploader reading its staged asset = %d %s, want 200", rec.Code, rec.Body.String())
	}

	if rec := assetCall(e, http.MethodPost, f.hostA, f.tokenSecond, "/api/v1/assets/"+a.ID.String()+"/finalize"); rec.Code != http.StatusNotFound {
		t.Errorf("other student finalizing = %d, want 404", rec.Code)
	}
	if rec := assetCall(e, http.MethodPost, f.hostA, f.tokenAdminA, "/api/v1/assets/"+a.ID.String()+"/finalize"); rec.Code != http.StatusOK {
		t.Fatalf("admin finalizing = %d %s, want 200", rec.Code, rec.Body.String())
	}
	for name, want := range map[string]int{"admin": 200, "operator": 200, "uploader": 200, "other student": 404} {
		token := map[string]string{"admin": f.tokenAdminA, "operator": f.tokenOperatorA, "uploader": f.tokenA, "other student": f.tokenSecond}[name]
		if rec := assetCall(e, http.MethodGet, f.hostA, token, url); rec.Code != want {
			t.Errorf("%s reading a final proof = %d, want %d", name, rec.Code, want)
		}
	}
}

func TestAssetFinalizeIsIdempotent(t *testing.T) {
	f := newAssetHTTPFixture(t)
	e := f.server(t, time.Minute)
	a := f.uploadPNG(t, e, f.hostA, f.tokenA, "payment_proof")
	path := "/api/v1/assets/" + a.ID.String() + "/finalize"

	first := assetCall(e, http.MethodPost, f.hostA, f.tokenA, path)
	if first.Code != http.StatusOK || decodeAsset(t, first).Status != "final" {
		t.Fatalf("first finalize = %d %s, want 200 final", first.Code, first.Body.String())
	}
	stamp := f.row(t, a.ID.String()).FinalizedAt
	second := assetCall(e, http.MethodPost, f.hostA, f.tokenA, path)
	if second.Code != http.StatusOK || decodeAsset(t, second).Status != "final" {
		t.Errorf("second finalize = %d %s, want 200 final", second.Code, second.Body.String())
	}
	if got := f.row(t, a.ID.String()).FinalizedAt; stamp == nil || !got.Equal(*stamp) {
		t.Errorf("finalized_at moved from %v to %v", stamp, got)
	}
}

func TestAssetFinalizeJoinsACallersTransaction(t *testing.T) {
	f := newAssetHTTPFixture(t)
	e := f.server(t, time.Minute)
	a := f.uploadPNG(t, e, f.hostA, f.tokenA, "payment_proof")
	assets := newAssetService(f.tenantFixture, f.store, time.Minute)
	principal, scope := model.Principal{UserID: f.studentA.ID, TenantID: &f.tenantA.ID, Role: model.RoleStudent}, service.HostScope{Tenant: f.tenantA}
	ctx := context.Background()

	tx, err := f.pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	if got, err := assets.FinalizeTx(ctx, tx, scope, principal, a.ID); err != nil || got.Status != model.AssetFinal {
		t.Fatalf("FinalizeTx = %+v, %v; want a final asset", got, err)
	}
	if err := tx.Rollback(ctx); err != nil {
		t.Fatalf("rollback: %v", err)
	}
	if got := f.row(t, a.ID.String()).Status; got != model.AssetStaged {
		t.Errorf("status after a rollback = %q, want staged", got)
	}

	tx, _ = f.pool.Begin(ctx)
	if _, err := assets.FinalizeTx(ctx, tx, scope, principal, a.ID); err != nil {
		t.Fatalf("FinalizeTx: %v", err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatalf("commit: %v", err)
	}
	if got := f.row(t, a.ID.String()).Status; got != model.AssetFinal {
		t.Errorf("status after a commit = %q, want final", got)
	}
}

func TestAssetURLIsShortLivedPrivateAndSigned(t *testing.T) {
	f := newAssetHTTPFixture(t)
	e := f.server(t, time.Minute)
	a := f.uploadPNG(t, e, f.hostA, f.tokenA, "payment_proof")

	before := time.Now()
	rec := assetCall(e, http.MethodGet, f.hostA, f.tokenA, "/api/v1/assets/"+a.ID.String()+"/url")
	if rec.Code != http.StatusOK {
		t.Fatalf("url = %d %s, want 200", rec.Code, rec.Body.String())
	}
	if got := rec.Header().Get("Cache-Control"); got != "private, no-store" {
		t.Errorf("Cache-Control = %q, want private, no-store", got)
	}
	var link struct {
		URL       string    `json:"url"`
		ExpiresAt time.Time `json:"expires_at"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &link); err != nil {
		t.Fatalf("body %s: %v", rec.Body.String(), err)
	}
	if d := link.ExpiresAt.Sub(before); d < 55*time.Second || d > 65*time.Second {
		t.Errorf("expires_at is %v from now, want about 60s", d)
	}
	if status, body, _ := fetch(t, link.URL); status != 200 || len(body) == 0 {
		t.Errorf("presigned GET = %d with %d bytes, want 200 with the object", status, len(body))
	}
	parsed, _ := url.Parse(link.URL)
	parsed.RawQuery = ""
	if status, _, _ := fetch(t, parsed.String()); status != http.StatusForbidden {
		t.Errorf("the same URL without its signature = %d, want 403", status)
	}
}

func TestAssetURLStopsWorkingAfterTheConfiguredTTL(t *testing.T) {
	f := newAssetHTTPFixture(t)
	e := f.server(t, time.Second)
	a := f.uploadPNG(t, e, f.hostA, f.tokenA, "payment_proof")
	rec := assetCall(e, http.MethodGet, f.hostA, f.tokenA, "/api/v1/assets/"+a.ID.String()+"/url")
	var link struct {
		URL string `json:"url"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &link)
	if status, _, _ := fetch(t, link.URL); status != 200 {
		t.Fatalf("GET inside the TTL = %d, want 200", status)
	}
	time.Sleep(2500 * time.Millisecond)
	if status, _, _ := fetch(t, link.URL); status != http.StatusForbidden {
		t.Errorf("GET after the TTL = %d, want 403", status)
	}
}

func TestAssetPurgeDeletesOldStagedAndKeepsFinal(t *testing.T) {
	f := newAssetHTTPFixture(t)
	e := f.server(t, time.Minute)
	oldStaged := f.uploadPNG(t, e, f.hostA, f.tokenA, "payment_proof")
	freshStaged := f.uploadPNG(t, e, f.hostA, f.tokenA, "payment_proof")
	oldFinal := f.uploadPNG(t, e, f.hostA, f.tokenA, "payment_proof")
	if rec := assetCall(e, http.MethodPost, f.hostA, f.tokenA, "/api/v1/assets/"+oldFinal.ID.String()+"/finalize"); rec.Code != 200 {
		t.Fatalf("finalize = %d", rec.Code)
	}
	keys := map[string]string{}
	for _, a := range []assetResponse{oldStaged, freshStaged, oldFinal} {
		keys[a.ID.String()] = f.row(t, a.ID.String()).ObjectKey
	}
	if _, err := f.pool.Exec(context.Background(), `UPDATE assets SET created_at = now() - interval '48 hours' WHERE id = ANY($1)`,
		[]uuid.UUID{oldStaged.ID, oldFinal.ID}); err != nil {
		t.Fatalf("age rows: %v", err)
	}
	assets := newAssetService(f.tenantFixture, f.store, time.Minute)
	reachable := func(key string) bool {
		signed, _ := f.infra.PresignGet(context.Background(), key, time.Minute)
		status, _, _ := fetch(t, signed)
		return status == 200
	}

	f.store.failDelete = true
	if _, err := assets.PurgeStaged(context.Background(), 24*time.Hour); err == nil {
		t.Error("a failing object delete was reported as success")
	}
	if _, err := repository.NewAssetRepository(f.pool).Find(context.Background(), f.pool, f.tenantA.ID, oldStaged.ID); err != nil {
		t.Errorf("row after a failed object delete: err = %v, want it kept so the purge can retry", err)
	}
	f.store.failDelete = false

	if _, err := assets.PurgeStaged(context.Background(), 24*time.Hour); err != nil {
		t.Fatalf("purge: %v", err)
	}
	repo := repository.NewAssetRepository(f.pool)
	if _, err := repo.Find(context.Background(), f.pool, f.tenantA.ID, oldStaged.ID); !errors.Is(err, repository.ErrNotFound) {
		t.Errorf("old staged row: err = %v, want deleted", err)
	}
	if reachable(keys[oldStaged.ID.String()]) {
		t.Error("the old staged object is still in storage")
	}
	for name, a := range map[string]assetResponse{"fresh staged": freshStaged, "old final": oldFinal} {
		if _, err := repo.Find(context.Background(), f.pool, f.tenantA.ID, a.ID); err != nil {
			t.Errorf("%s row: err = %v, want kept", name, err)
		}
		if !reachable(keys[a.ID.String()]) {
			t.Errorf("%s object is gone from storage", name)
		}
	}
}

func TestAssetRoutesAnswer503WhenStorageIsNotConfigured(t *testing.T) {
	f := newWalletHTTPFixture(t)
	e := f.echo(t)
	id := uuid.NewString()

	for name, rec := range map[string]*httptest.ResponseRecorder{
		"upload":   upload(e, f.hostA, f.tokenA, "a.png", "image/png", pngBytes(t, 4, 4), map[string]string{"purpose": "payment_proof"}),
		"finalize": assetCall(e, http.MethodPost, f.hostA, f.tokenA, "/api/v1/assets/"+id+"/finalize"),
		"url":      assetCall(e, http.MethodGet, f.hostA, f.tokenA, "/api/v1/assets/"+id+"/url"),
	} {
		if body := errorBody(t, rec); rec.Code != http.StatusServiceUnavailable || body.Code != httpx.CodeUnavailable || body.Message == "" {
			t.Errorf("%s = %d %s, want 503 service_unavailable in the envelope", name, rec.Code, rec.Body.String())
		}
	}
	if rec := assetCall(e, http.MethodGet, f.hostA, "", "/api/v1/assets/"+id+"/url"); rec.Code != http.StatusUnauthorized {
		t.Errorf("anonymous url = %d, want 401 before storage is considered", rec.Code)
	}
	if rec := assetCall(e, http.MethodGet, f.hostA, f.tokenA, "/api/v1/wallet"); rec.Code != http.StatusOK {
		t.Errorf("wallet with no storage = %d, want 200: nothing else may break", rec.Code)
	}
}
