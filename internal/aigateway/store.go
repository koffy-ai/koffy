package aigateway

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
)

var ErrInvalidAppKey = errors.New("invalid app key")
var ErrModelNotAllowed = errors.New("model is not allowed for app")

type Store struct {
	db *sql.DB
}

type AppIdentity struct {
	ID          int64
	AppCode     string
	BillingMode string
}

func NewStore(db *sql.DB) *Store {
	return &Store{db: db}
}

func (s *Store) ResolveAppKey(ctx context.Context, appKey string) (AppIdentity, error) {
	keyHash := sha256.Sum256([]byte(appKey))

	var app AppIdentity
	err := s.db.QueryRowContext(ctx, `
		SELECT a.id, a.app_code, a.billing_mode
		FROM app_api_keys k
		JOIN apps a ON a.id = k.app_id
		WHERE k.key_hash = ?
			AND k.status = 'active'
			AND a.status = 'active'
			AND (k.expires_at IS NULL OR k.expires_at > CURRENT_TIMESTAMP(3))
		LIMIT 1`,
		hex.EncodeToString(keyHash[:]),
	).Scan(&app.ID, &app.AppCode, &app.BillingMode)
	if errors.Is(err, sql.ErrNoRows) {
		return AppIdentity{}, ErrInvalidAppKey
	}
	if err != nil {
		return AppIdentity{}, err
	}

	_, _ = s.db.ExecContext(ctx, `
		UPDATE app_api_keys
		SET last_used_at = CURRENT_TIMESTAMP(3)
		WHERE key_hash = ?`,
		hex.EncodeToString(keyHash[:]),
	)

	return app, nil
}

func (s *Store) EnsureModelAllowed(ctx context.Context, appID int64, modelAlias string) error {
	var configuredRoutes int
	if err := s.db.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM app_model_routes
		WHERE app_id = ? AND status = 'active'`,
		appID,
	).Scan(&configuredRoutes); err != nil {
		return err
	}
	if configuredRoutes == 0 {
		return nil
	}

	var matchedRoutes int
	if err := s.db.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM app_model_routes r
		JOIN ai_models m ON m.id = r.model_id
		WHERE r.app_id = ?
			AND r.status = 'active'
			AND m.status = 'active'
			AND m.model_alias = ?`,
		appID,
		modelAlias,
	).Scan(&matchedRoutes); err != nil {
		return err
	}
	if matchedRoutes == 0 {
		return ErrModelNotAllowed
	}
	return nil
}
