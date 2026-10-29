package infra

import (
	"bytes"
	"context"
	"fmt"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"

	"github.com/tarunaeduka/bimbel-tryout/api/internal/config"
	"github.com/tarunaeduka/bimbel-tryout/api/internal/service"
)

// Storage is any S3-compatible server: MinIO locally, GCS through its HMAC interoperability.
type Storage struct {
	client  *minio.Client
	presign *minio.Client
	bucket  string
}

var _ service.ObjectStore = (*Storage)(nil)

func NewStorage(cfg config.S3Config) (*Storage, error) {
	client, err := minio.New(cfg.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.AccessKey, cfg.SecretKey, ""),
		Secure: cfg.UseSSL,
		Region: cfg.Region,
	})
	if err != nil {
		return nil, fmt.Errorf("storage client: %w", err)
	}
	presign := client
	if cfg.PublicEndpoint != "" {
		region := cfg.Region
		if region == "" {
			// A known region keeps presigning offline; otherwise minio-go asks the public host, which the API may not reach.
			region = "us-east-1"
		}
		presign, err = minio.New(cfg.PublicEndpoint, &minio.Options{
			Creds:  credentials.NewStaticV4(cfg.AccessKey, cfg.SecretKey, ""),
			Secure: cfg.UseSSL,
			Region: region,
		})
		if err != nil {
			return nil, fmt.Errorf("storage presign client: %w", err)
		}
	}
	return &Storage{client: client, presign: presign, bucket: cfg.Bucket}, nil
}

// EnsureBucket never sets a policy: a bucket is private until someone deliberately opens it.
func (s *Storage) EnsureBucket(ctx context.Context, create bool) error {
	exists, err := s.client.BucketExists(ctx, s.bucket)
	if err != nil {
		return fmt.Errorf("check bucket %q: %w", s.bucket, err)
	}
	if exists {
		return nil
	}
	if !create {
		return fmt.Errorf("bucket %q does not exist", s.bucket)
	}
	if err := s.client.MakeBucket(ctx, s.bucket, minio.MakeBucketOptions{}); err != nil {
		return fmt.Errorf("create bucket %q: %w", s.bucket, err)
	}
	return nil
}

func (s *Storage) Put(ctx context.Context, key string, data []byte, contentType string) error {
	_, err := s.client.PutObject(ctx, s.bucket, key, bytes.NewReader(data), int64(len(data)),
		minio.PutObjectOptions{ContentType: contentType})
	return err
}

func (s *Storage) Delete(ctx context.Context, key string) error {
	return s.client.RemoveObject(ctx, s.bucket, key, minio.RemoveObjectOptions{})
}

func (s *Storage) PresignGet(ctx context.Context, key string, ttl time.Duration) (string, error) {
	u, err := s.presign.PresignedGetObject(ctx, s.bucket, key, ttl, nil)
	if err != nil {
		return "", err
	}
	return u.String(), nil
}
