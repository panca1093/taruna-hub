package infra

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tarunaeduka/bimbel-tryout/api/internal/config"
)

func s3TestConfig(t *testing.T) config.S3Config {
	t.Helper()
	endpoint := os.Getenv("S3_ENDPOINT")
	if endpoint == "" {
		t.Skip("S3_ENDPOINT not set")
	}
	return config.S3Config{
		Endpoint:  endpoint,
		AccessKey: os.Getenv("S3_ACCESS_KEY"),
		SecretKey: os.Getenv("S3_SECRET_KEY"),
		Bucket:    os.Getenv("S3_BUCKET"),
		UseSSL:    os.Getenv("S3_USE_SSL") == "true",
	}
}

// testStorage uses the shared bucket and a key prefix of its own, so runs never collide.
func testStorage(t *testing.T) (*Storage, string) {
	t.Helper()
	s, err := NewStorage(s3TestConfig(t))
	if err != nil {
		t.Fatalf("new storage: %v", err)
	}
	if err := s.EnsureBucket(context.Background(), true); err != nil {
		t.Fatalf("ensure bucket: %v", err)
	}
	return s, "test-" + uuid.NewString() + "/"
}

func get(t *testing.T, url string) (int, []byte) {
	t.Helper()
	resp, err := http.Get(url)
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, body
}

func TestStoragePrivateObjectIsReadableOnlyThroughASignature(t *testing.T) {
	s, prefix := testStorage(t)
	ctx := context.Background()
	key := prefix + "proof"
	if err := s.Put(ctx, key, []byte("secret-bytes"), "image/png"); err != nil {
		t.Fatalf("put: %v", err)
	}
	t.Cleanup(func() { _ = s.Delete(ctx, key) })

	unsigned := s.client.EndpointURL().String() + "/" + s.bucket + "/" + key
	if status, _ := get(t, unsigned); status != http.StatusForbidden {
		t.Errorf("unsigned GET = %d, want 403", status)
	}

	signed, err := s.PresignGet(ctx, key, time.Minute)
	if err != nil {
		t.Fatalf("presign: %v", err)
	}
	if status, body := get(t, signed); status != http.StatusOK || !bytes.Equal(body, []byte("secret-bytes")) {
		t.Errorf("presigned GET = %d %q, want 200 with the object", status, body)
	}
	tampered := strings.Replace(signed, key, key+"x", 1)
	if status, _ := get(t, tampered); status == http.StatusOK {
		t.Errorf("a signature for one key opened another: status %d", status)
	}
}

func TestStoragePresignedURLStopsWorkingAfterExpiry(t *testing.T) {
	s, prefix := testStorage(t)
	ctx := context.Background()
	key := prefix + "short"
	if err := s.Put(ctx, key, []byte("x"), "image/png"); err != nil {
		t.Fatalf("put: %v", err)
	}
	t.Cleanup(func() { _ = s.Delete(ctx, key) })

	signed, err := s.PresignGet(ctx, key, time.Second)
	if err != nil {
		t.Fatalf("presign: %v", err)
	}
	if status, _ := get(t, signed); status != http.StatusOK {
		t.Fatalf("GET before expiry = %d, want 200", status)
	}
	time.Sleep(2500 * time.Millisecond)
	if status, _ := get(t, signed); status != http.StatusForbidden {
		t.Errorf("GET after expiry = %d, want 403", status)
	}
}

func TestStorageDeleteRemovesTheObjectAndToleratesAMissingOne(t *testing.T) {
	s, prefix := testStorage(t)
	ctx := context.Background()
	key := prefix + "gone"
	if err := s.Put(ctx, key, []byte("x"), "image/png"); err != nil {
		t.Fatalf("put: %v", err)
	}
	if err := s.Delete(ctx, key); err != nil {
		t.Fatalf("delete: %v", err)
	}
	signed, _ := s.PresignGet(ctx, key, time.Minute)
	if status, _ := get(t, signed); status != http.StatusNotFound {
		t.Errorf("GET after delete = %d, want 404", status)
	}
	if err := s.Delete(ctx, key); err != nil {
		t.Errorf("deleting a missing object: %v, want nil so a retry can finish", err)
	}
}

func TestStorageEnsureBucketCreatesOnlyWhenAsked(t *testing.T) {
	cfg := s3TestConfig(t)
	cfg.Bucket = "test-" + uuid.NewString()
	s, err := NewStorage(cfg)
	if err != nil {
		t.Fatalf("new storage: %v", err)
	}
	ctx := context.Background()
	if err := s.EnsureBucket(ctx, false); err == nil {
		t.Fatal("a missing bucket passed EnsureBucket(create=false), want an error")
	}
	if err := s.EnsureBucket(ctx, true); err != nil {
		t.Fatalf("EnsureBucket(create=true): %v", err)
	}
	t.Cleanup(func() { _ = s.client.RemoveBucket(ctx, cfg.Bucket) })
	if err := s.EnsureBucket(ctx, false); err != nil {
		t.Errorf("an existing bucket failed EnsureBucket(create=false): %v", err)
	}
}
