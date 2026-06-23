package billing

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

type AvatarAsset struct {
	ContentType string
	Data        []byte
	SizeBytes   int
	Width       int
	Height      int
}

func (s *Store) DisplayNameExists(ctx context.Context, displayName string, exceptUserID int64) (bool, error) {
	var exists bool
	err := s.db.QueryRowContext(ctx, `
		SELECT EXISTS(
			SELECT 1
			FROM users
			WHERE display_name = ?
			  AND id <> ?
		)`,
		displayName,
		exceptUserID,
	).Scan(&exists)
	return exists, err
}

func (s *Store) UniqueDisplayName(ctx context.Context, preferred string, exceptUserID int64) (string, error) {
	base := normalizeDefaultDisplayName(preferred)
	for i := 0; i < 32; i++ {
		candidate := base
		if i > 0 {
			suffix, err := randomLowerAlpha(5)
			if err != nil {
				return "", err
			}
			candidate = displayNameWithSuffix(base, suffix)
		}
		exists, err := s.DisplayNameExists(ctx, candidate, exceptUserID)
		if err != nil {
			return "", err
		}
		if !exists {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("unable to generate unique display name")
}

func uniqueDisplayNameInTx(ctx context.Context, tx *sql.Tx, preferred string, exceptUserID int64) (string, error) {
	base := normalizeDefaultDisplayName(preferred)
	for i := 0; i < 32; i++ {
		candidate := base
		if i > 0 {
			suffix, err := randomLowerAlpha(5)
			if err != nil {
				return "", err
			}
			candidate = displayNameWithSuffix(base, suffix)
		}
		exists, err := displayNameExistsInTx(ctx, tx, candidate, exceptUserID)
		if err != nil {
			return "", err
		}
		if !exists {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("unable to generate unique display name")
}

func displayNameExistsInTx(ctx context.Context, tx *sql.Tx, displayName string, exceptUserID int64) (bool, error) {
	var exists bool
	err := tx.QueryRowContext(ctx, `
		SELECT EXISTS(
			SELECT 1
			FROM users
			WHERE display_name = ?
			  AND id <> ?
		)`,
		displayName,
		exceptUserID,
	).Scan(&exists)
	return exists, err
}

func normalizeDefaultDisplayName(preferred string) string {
	value := strings.TrimSpace(preferred)
	if value == "" {
		value = "微信用户"
	}
	runes := []rune(value)
	if len(runes) > 30 {
		return string(runes[:30])
	}
	return value
}

func displayNameWithSuffix(base string, suffix string) string {
	fullSuffix := "_" + suffix
	baseRunes := []rune(normalizeDefaultDisplayName(base))
	limit := 30 - len([]rune(fullSuffix))
	if limit < 1 {
		baseRunes = []rune("微信用户")
		limit = 30 - len([]rune(fullSuffix))
	}
	if len(baseRunes) > limit {
		baseRunes = baseRunes[:limit]
	}
	return string(baseRunes) + fullSuffix
}

func (s *Store) UpdateUserDisplayName(ctx context.Context, userID int64, displayName string) error {
	_, err := s.db.ExecContext(ctx, `
		UPDATE users
		SET display_name = ?, display_name_custom = TRUE
		WHERE id = ?`,
		displayName,
		userID,
	)
	return err
}

func (s *Store) SaveUserAvatarAsset(ctx context.Context, userID int64, avatarURL string, asset AvatarAsset) error {
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return err
	}
	defer rollback(tx)

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO user_avatar_assets (user_id, content_type, data, size_bytes, width, height)
		VALUES (?, ?, ?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE
			content_type = VALUES(content_type),
			data = VALUES(data),
			size_bytes = VALUES(size_bytes),
			width = VALUES(width),
			height = VALUES(height),
			updated_at = CURRENT_TIMESTAMP(3)`,
		userID,
		asset.ContentType,
		asset.Data,
		asset.SizeBytes,
		asset.Width,
		asset.Height,
	); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `
		UPDATE users
		SET avatar_url = ?, avatar_custom = TRUE, updated_at = CURRENT_TIMESTAMP(3)
		WHERE id = ?`,
		avatarURL,
		userID,
	); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) PublicAvatarAsset(ctx context.Context, casdoorUserID string) (AvatarAsset, error) {
	var asset AvatarAsset
	err := s.db.QueryRowContext(ctx, `
		SELECT a.content_type, a.data, a.size_bytes, a.width, a.height
		FROM user_avatar_assets a
		JOIN users u ON u.id = a.user_id
		WHERE u.casdoor_user_id = ?`,
		casdoorUserID,
	).Scan(&asset.ContentType, &asset.Data, &asset.SizeBytes, &asset.Width, &asset.Height)
	return asset, err
}
