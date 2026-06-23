package billing

import (
	"context"
	"database/sql"
	"errors"
)

const (
	legacyLogoAssetKey    = "logo"
	centerLogoAssetKey    = "center_logo"
	adminLogoAssetKey     = "admin_logo"
	centerFaviconAssetKey = "center_favicon"
	adminFaviconAssetKey  = "admin_favicon"
)

type BrandingAsset struct {
	ContentType      string
	Data             []byte
	SizeBytes        int
	Width            int
	Height           int
	OriginalFilename string
}

func (s *Store) BrandingAsset(ctx context.Context, assetKey string) (BrandingAsset, bool, error) {
	var asset BrandingAsset
	err := s.db.QueryRowContext(ctx, `
		SELECT content_type, data, size_bytes, width, height, COALESCE(original_filename, '')
		FROM branding_assets
		WHERE asset_key = ?`,
		assetKey,
	).Scan(&asset.ContentType, &asset.Data, &asset.SizeBytes, &asset.Width, &asset.Height, &asset.OriginalFilename)
	if errors.Is(err, sql.ErrNoRows) {
		return BrandingAsset{}, false, nil
	}
	if err != nil {
		return BrandingAsset{}, false, err
	}
	return asset, true, nil
}

func (s *Store) SaveBrandingAsset(ctx context.Context, actorUserID int64, assetKey string, asset BrandingAsset) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO branding_assets (
			asset_key, content_type, data, size_bytes, width, height, original_filename, updated_by_user_id
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE
			content_type = VALUES(content_type),
			data = VALUES(data),
			size_bytes = VALUES(size_bytes),
			width = VALUES(width),
			height = VALUES(height),
			original_filename = VALUES(original_filename),
			updated_by_user_id = VALUES(updated_by_user_id),
			updated_at = CURRENT_TIMESTAMP(3)`,
		assetKey,
		asset.ContentType,
		asset.Data,
		asset.SizeBytes,
		asset.Width,
		asset.Height,
		asset.OriginalFilename,
		actorUserID,
	)
	return err
}
