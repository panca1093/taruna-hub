package service

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"image"
	_ "image/jpeg" // registers the decoder DecodeConfig and Decode dispatch on
	_ "image/png"  // registers the decoder DecodeConfig and Decode dispatch on
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/tarunaeduka/bimbel-tryout/api/internal/model"
	"github.com/tarunaeduka/bimbel-tryout/api/internal/repository"
)

var (
	ErrAssetNotFound      = errors.New("asset not found")
	ErrAssetForbidden     = errors.New("asset purpose not allowed for this role")
	ErrAssetTooLarge      = errors.New("asset too large")
	ErrStorageUnavailable = errors.New("object storage is not configured")
)

// Asset field reasons; the transport turns each into the copy its screens show.
const (
	ReasonAssetType       = "asset_type"
	ReasonAssetCorrupt    = "asset_corrupt"
	ReasonAssetDimensions = "asset_dimensions"
	ReasonAssetPurpose    = "asset_purpose"
	ReasonAssetMissing    = "asset_missing"
)

const (
	MaxAssetBytes     = 5 << 20
	maxAssetDimension = 4096
	assetPurgeBatch   = 500
	maxParallelDecode = 2
)

// decodeSlots bounds full decodes: a 4096 x 4096 image is tens of MB of pixels, and uploads run in parallel.
var decodeSlots = make(chan struct{}, maxParallelDecode)

// ObjectStore is the small slice of S3-compatible storage the asset rules need.
type ObjectStore interface {
	Put(ctx context.Context, key string, data []byte, contentType string) error
	// Delete succeeds on a missing object, so a retried purge can finish.
	Delete(ctx context.Context, key string) error
	PresignGet(ctx context.Context, key string, ttl time.Duration) (string, error)
}

// AssetLink is a presigned GET; it grants access to whoever holds it until ExpiresAt.
type AssetLink struct {
	URL       string
	ExpiresAt time.Time
}

type AssetService struct {
	db     *pgxpool.Pool
	repo   *repository.AssetRepository
	store  ObjectStore
	urlTTL time.Duration
}

// NewAssetService takes a nil store when storage is not configured; its routes then answer 503.
func NewAssetService(db *pgxpool.Pool, repo *repository.AssetRepository, store ObjectStore, urlTTL time.Duration) *AssetService {
	return &AssetService{db: db, repo: repo, store: store, urlTTL: urlTTL}
}

// Upload validates the bytes, records a staged row, then writes the object; the key is built here.
func (s *AssetService) Upload(ctx context.Context, scope HostScope, principal model.Principal, purpose string, data []byte) (model.Asset, error) {
	if s.store == nil {
		return model.Asset{}, ErrStorageUnavailable
	}
	tenantID, err := walletTenant(scope, principal)
	if err != nil {
		return model.Asset{}, err
	}
	allowed, known := purposeAllows(purpose, principal.Role)
	if !known {
		return model.Asset{}, &InputError{Fields: map[string]string{"purpose": ReasonAssetPurpose}}
	}
	if !allowed {
		return model.Asset{}, ErrAssetForbidden
	}
	if len(data) > MaxAssetBytes {
		return model.Asset{}, ErrAssetTooLarge
	}
	contentType, err := detectImage(ctx, data)
	if err != nil {
		return model.Asset{}, err
	}

	id := uuid.New()
	key := fmt.Sprintf("%s/%s/%s", tenantID, purpose, id)
	uploader := principal.UserID
	asset, err := s.repo.Insert(ctx, tenantID, model.Asset{
		ID: id, Purpose: purpose, ObjectKey: key, ContentType: contentType,
		SizeBytes: int64(len(data)), UploadedBy: &uploader,
	})
	if err != nil {
		return model.Asset{}, err
	}
	if err := s.store.Put(ctx, key, data, contentType); err != nil {
		s.discardStaged(context.WithoutCancel(ctx), tenantID, id)
		return model.Asset{}, fmt.Errorf("store asset: %w", err)
	}
	return asset, nil
}

// discardStaged is best effort: a row it misses has no object and is purged like any abandoned upload.
func (s *AssetService) discardStaged(ctx context.Context, tenantID, id uuid.UUID) {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := s.repo.DeleteStaged(ctx, tx, tenantID, id); err == nil {
		_ = tx.Commit(ctx)
	}
}

// FinalizeTx runs in the consumer's transaction so attachment and finalize commit together; it is idempotent.
func (s *AssetService) FinalizeTx(ctx context.Context, tx pgx.Tx, scope HostScope, principal model.Principal, id uuid.UUID) (model.Asset, error) {
	tenantID, err := walletTenant(scope, principal)
	if err != nil {
		return model.Asset{}, err
	}
	asset, err := s.repo.Find(ctx, tx, tenantID, id)
	if err != nil {
		return model.Asset{}, assetRepoError(err)
	}
	if !isUploader(asset, principal) && !isStaff(principal.Role) {
		return model.Asset{}, ErrAssetNotFound
	}
	asset, err = s.repo.Finalize(ctx, tx, tenantID, id)
	if err != nil {
		return model.Asset{}, assetRepoError(err)
	}
	return asset, nil
}

// DownloadURL re-authorizes on every call; a staged asset is the uploader's alone.
func (s *AssetService) DownloadURL(ctx context.Context, scope HostScope, principal model.Principal, id uuid.UUID) (AssetLink, error) {
	if s.store == nil {
		return AssetLink{}, ErrStorageUnavailable
	}
	tenantID, err := walletTenant(scope, principal)
	if err != nil {
		return AssetLink{}, err
	}
	asset, err := s.repo.Find(ctx, s.db, tenantID, id)
	if err != nil {
		return AssetLink{}, assetRepoError(err)
	}
	if !isUploader(asset, principal) && !(asset.Status == model.AssetFinal && isStaff(principal.Role)) {
		visible := false
		if asset.Status == model.AssetFinal && asset.Purpose == model.PurposeItemImage && principal.Role == model.RoleStudent {
			if visible, err = s.repo.KecermatanItemVisible(ctx, s.db, tenantID, principal.UserID, id); err != nil {
				return AssetLink{}, assetRepoError(err)
			}
		}
		if !visible {
			return AssetLink{}, ErrAssetNotFound
		}
	}
	expires := time.Now().Add(s.urlTTL)
	link, err := s.store.PresignGet(ctx, asset.ObjectKey, s.urlTTL)
	if err != nil {
		return AssetLink{}, fmt.Errorf("presign asset: %w", err)
	}
	return AssetLink{URL: link, ExpiresAt: expires}, nil
}

// PurgeStaged deletes staged rows older than the age together with their objects, never a final row.
func (s *AssetService) PurgeStaged(ctx context.Context, olderThan time.Duration) (int, error) {
	if s.store == nil {
		return 0, ErrStorageUnavailable
	}
	cutoff := time.Now().Add(-olderThan)
	candidates, err := s.repo.StagedBefore(ctx, cutoff, assetPurgeBatch)
	if err != nil {
		return 0, err
	}
	var failures []error
	purged := 0
	for _, c := range candidates {
		done, err := s.purgeOne(ctx, c, cutoff)
		if err != nil {
			failures = append(failures, err)
		} else if done {
			purged++
		}
	}
	return purged, errors.Join(failures...)
}

// purgeOne holds the row lock across the object delete, so a finalize cannot slip in between and leave
// a final row without its object; the object goes first, so a failure leaves a retryable row.
func (s *AssetService) purgeOne(ctx context.Context, c model.Asset, cutoff time.Time) (bool, error) {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return false, fmt.Errorf("begin: %w", err)
	}
	defer func() { _ = tx.Rollback(context.WithoutCancel(ctx)) }()
	asset, err := s.repo.LockStaged(ctx, tx, c.TenantID, c.ID, cutoff)
	if errors.Is(err, repository.ErrNotFound) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if err := s.store.Delete(ctx, asset.ObjectKey); err != nil {
		return false, fmt.Errorf("delete object of asset %s: %w", asset.ID, err)
	}
	if err := s.repo.DeleteStaged(ctx, tx, asset.TenantID, asset.ID); err != nil {
		return false, err
	}
	if err := tx.Commit(ctx); err != nil {
		return false, fmt.Errorf("commit: %w", err)
	}
	return true, nil
}

func purposeAllows(purpose string, role model.Role) (allowed, known bool) {
	switch purpose {
	case model.PurposePaymentProof:
		return role == model.RoleStudent || isStaff(role), true
	case model.PurposeQuestionImage, model.PurposeItemImage:
		return isStaff(role), true
	}
	return false, false
}

func isStaff(role model.Role) bool { return role == model.RoleAdmin || role == model.RoleOperator }

func isUploader(a model.Asset, p model.Principal) bool {
	return a.UploadedBy != nil && *a.UploadedBy == p.UserID
}

func assetRepoError(err error) error {
	switch {
	case errors.Is(err, repository.ErrNotFound):
		return ErrAssetNotFound
	case errors.Is(err, repository.ErrScopeRequired):
		return ErrTenantNotFound
	}
	return err
}

func assetInvalid(reason string) error {
	return &InputError{Fields: map[string]string{"file": reason}}
}

var (
	pngMagic  = []byte("\x89PNG\r\n\x1a\n")
	jpegMagic = []byte{0xff, 0xd8, 0xff}
)

// detectImage proves the type from the bytes: only PNG, JPEG and WebP pass, and only if they decode.
func detectImage(ctx context.Context, data []byte) (string, error) {
	switch {
	case bytes.HasPrefix(data, pngMagic):
		return "image/png", decodeRaster(ctx, data)
	case bytes.HasPrefix(data, jpegMagic):
		return "image/jpeg", decodeRaster(ctx, data)
	case len(data) >= 12 && string(data[:4]) == "RIFF" && string(data[8:12]) == "WEBP":
		return "image/webp", checkWebP(data)
	}
	return "", assetInvalid(ReasonAssetType)
}

// decodeRaster reads the size first, so an absurd image is refused before any pixel is allocated.
func decodeRaster(ctx context.Context, data []byte) error {
	cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return assetInvalid(ReasonAssetCorrupt)
	}
	if cfg.Width > maxAssetDimension || cfg.Height > maxAssetDimension {
		return assetInvalid(ReasonAssetDimensions)
	}
	select {
	case decodeSlots <- struct{}{}:
		defer func() { <-decodeSlots }()
	case <-ctx.Done():
		return ctx.Err()
	}
	if _, _, err := image.Decode(bytes.NewReader(data)); err != nil {
		return assetInvalid(ReasonAssetCorrupt)
	}
	return nil
}

// checkWebP reads the container and the first chunk's size by hand; no pixel data is decoded.
func checkWebP(data []byte) error {
	if len(data) < 20 || int(binary.LittleEndian.Uint32(data[4:8]))+8 > len(data) {
		return assetInvalid(ReasonAssetCorrupt)
	}
	size := int(binary.LittleEndian.Uint32(data[16:20]))
	if 20+size > len(data) {
		return assetInvalid(ReasonAssetCorrupt)
	}
	payload := data[20 : 20+size]
	var w, h int
	switch string(data[12:16]) {
	case "VP8 ":
		if len(payload) < 10 || !bytes.Equal(payload[3:6], []byte{0x9d, 0x01, 0x2a}) {
			return assetInvalid(ReasonAssetCorrupt)
		}
		w = int(binary.LittleEndian.Uint16(payload[6:8]) & 0x3fff)
		h = int(binary.LittleEndian.Uint16(payload[8:10]) & 0x3fff)
	case "VP8L":
		if len(payload) < 5 || payload[0] != 0x2f {
			return assetInvalid(ReasonAssetCorrupt)
		}
		bits := binary.LittleEndian.Uint32(payload[1:5])
		w, h = int(bits&0x3fff)+1, int(bits>>14&0x3fff)+1
	case "VP8X":
		if len(payload) < 10 {
			return assetInvalid(ReasonAssetCorrupt)
		}
		w = 1 + (int(payload[4]) | int(payload[5])<<8 | int(payload[6])<<16)
		h = 1 + (int(payload[7]) | int(payload[8])<<8 | int(payload[9])<<16)
	default:
		return assetInvalid(ReasonAssetCorrupt)
	}
	switch {
	case w == 0 || h == 0:
		return assetInvalid(ReasonAssetCorrupt)
	case w > maxAssetDimension || h > maxAssetDimension:
		return assetInvalid(ReasonAssetDimensions)
	}
	return nil
}
