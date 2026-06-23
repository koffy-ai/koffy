package billing

import (
	"context"
	"database/sql"
	"time"
)

type AIProviderItem struct {
	ID           int64     `json:"id"`
	ProviderCode string    `json:"provider_code"`
	Name         string    `json:"name"`
	Status       string    `json:"status"`
	BaseURL      string    `json:"base_url"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type AIProviderRequest struct {
	ProviderCode string `json:"provider_code"`
	Name         string `json:"name"`
	Status       string `json:"status"`
	BaseURL      string `json:"base_url"`
}

type AIModelItem struct {
	ID            int64     `json:"id"`
	ProviderCode  string    `json:"provider_code"`
	ProviderName  string    `json:"provider_name"`
	ModelAlias    string    `json:"model_alias"`
	ProviderModel string    `json:"provider_model"`
	Capability    string    `json:"capability"`
	Status        string    `json:"status"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

type AIModelRequest struct {
	ProviderCode  string `json:"provider_code"`
	ModelAlias    string `json:"model_alias"`
	ProviderModel string `json:"provider_model"`
	Capability    string `json:"capability"`
	Status        string `json:"status"`
}

type AppModelRouteItem struct {
	ID            int64     `json:"id"`
	AppCode       string    `json:"app_code"`
	ModelAlias    string    `json:"model_alias"`
	ProviderCode  string    `json:"provider_code"`
	ProviderModel string    `json:"provider_model"`
	Capability    string    `json:"capability"`
	Status        string    `json:"status"`
	CreatedAt     time.Time `json:"created_at"`
}

type AppModelRouteRequest struct {
	ModelAlias string `json:"model_alias"`
	Status     string `json:"status"`
}

func (s *Store) AdminListAIProviders(ctx context.Context) ([]AIProviderItem, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, provider_code, name, status, base_url, created_at, updated_at
		FROM ai_providers
		ORDER BY id DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]AIProviderItem, 0)
	for rows.Next() {
		var item AIProviderItem
		if err := rows.Scan(&item.ID, &item.ProviderCode, &item.Name, &item.Status, &item.BaseURL, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Store) AdminUpsertAIProvider(ctx context.Context, actorUserID int64, req AIProviderRequest) (AIProviderItem, error) {
	if req.Status == "" {
		req.Status = "active"
	}
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO ai_providers (provider_code, name, status, base_url)
		VALUES (?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE
			name = VALUES(name),
			status = VALUES(status),
			base_url = VALUES(base_url)`,
		req.ProviderCode,
		req.Name,
		req.Status,
		req.BaseURL,
	)
	if err != nil {
		return AIProviderItem{}, err
	}
	item, err := s.adminFindAIProvider(ctx, req.ProviderCode)
	if err != nil {
		return AIProviderItem{}, err
	}
	_ = s.writeAuditLog(ctx, actorUserID, "admin.ai.provider.upsert", "ai_provider", item.ProviderCode, req)
	return item, nil
}

func (s *Store) AdminListAIModels(ctx context.Context) ([]AIModelItem, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT m.id, p.provider_code, p.name, m.model_alias, m.provider_model,
			m.capability, m.status, m.created_at, m.updated_at
		FROM ai_models m
		JOIN ai_providers p ON p.id = m.provider_id
		ORDER BY m.id DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]AIModelItem, 0)
	for rows.Next() {
		var item AIModelItem
		if err := rows.Scan(
			&item.ID,
			&item.ProviderCode,
			&item.ProviderName,
			&item.ModelAlias,
			&item.ProviderModel,
			&item.Capability,
			&item.Status,
			&item.CreatedAt,
			&item.UpdatedAt,
		); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Store) AdminUpsertAIModel(ctx context.Context, actorUserID int64, req AIModelRequest) (AIModelItem, error) {
	if req.Status == "" {
		req.Status = "active"
	}
	provider, err := s.adminFindAIProvider(ctx, req.ProviderCode)
	if err != nil {
		return AIModelItem{}, err
	}
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO ai_models (provider_id, model_alias, provider_model, capability, status)
		VALUES (?, ?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE
			provider_id = VALUES(provider_id),
			provider_model = VALUES(provider_model),
			capability = VALUES(capability),
			status = VALUES(status)`,
		provider.ID,
		req.ModelAlias,
		req.ProviderModel,
		req.Capability,
		req.Status,
	)
	if err != nil {
		return AIModelItem{}, err
	}
	item, err := s.adminFindAIModel(ctx, req.ModelAlias)
	if err != nil {
		return AIModelItem{}, err
	}
	_ = s.writeAuditLog(ctx, actorUserID, "admin.ai.model.upsert", "ai_model", item.ModelAlias, req)
	return item, nil
}

func (s *Store) AdminListAppModelRoutes(ctx context.Context, appCode string) ([]AppModelRouteItem, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT r.id, a.app_code, m.model_alias, p.provider_code, m.provider_model,
			m.capability, r.status, r.created_at
		FROM app_model_routes r
		JOIN apps a ON a.id = r.app_id
		JOIN ai_models m ON m.id = r.model_id
		JOIN ai_providers p ON p.id = m.provider_id
		WHERE a.app_code = ?
		ORDER BY r.id DESC`,
		appCode,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]AppModelRouteItem, 0)
	for rows.Next() {
		var item AppModelRouteItem
		if err := rows.Scan(
			&item.ID,
			&item.AppCode,
			&item.ModelAlias,
			&item.ProviderCode,
			&item.ProviderModel,
			&item.Capability,
			&item.Status,
			&item.CreatedAt,
		); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Store) AdminUpsertAppModelRoute(ctx context.Context, actorUserID int64, appCode string, req AppModelRouteRequest) (AppModelRouteItem, error) {
	if req.Status == "" {
		req.Status = "active"
	}
	app, err := s.adminFindApp(ctx, appCode)
	if err != nil {
		return AppModelRouteItem{}, err
	}
	model, err := s.adminFindAIModel(ctx, req.ModelAlias)
	if err != nil {
		return AppModelRouteItem{}, err
	}
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO app_model_routes (app_id, model_id, status)
		VALUES (?, ?, ?)
		ON DUPLICATE KEY UPDATE status = VALUES(status)`,
		app.ID,
		model.ID,
		req.Status,
	)
	if err != nil {
		return AppModelRouteItem{}, err
	}
	item, err := s.adminFindAppModelRoute(ctx, app.AppCode, model.ModelAlias)
	if err != nil {
		return AppModelRouteItem{}, err
	}
	_ = s.writeAuditLog(ctx, actorUserID, "admin.ai.app_model_route.upsert", "app", app.AppCode, req)
	return item, nil
}

func (s *Store) adminFindAIProvider(ctx context.Context, providerCode string) (AIProviderItem, error) {
	var item AIProviderItem
	err := s.db.QueryRowContext(ctx, `
		SELECT id, provider_code, name, status, base_url, created_at, updated_at
		FROM ai_providers
		WHERE provider_code = ?`,
		providerCode,
	).Scan(&item.ID, &item.ProviderCode, &item.Name, &item.Status, &item.BaseURL, &item.CreatedAt, &item.UpdatedAt)
	return item, err
}

func (s *Store) adminFindAIModel(ctx context.Context, modelAlias string) (AIModelItem, error) {
	var item AIModelItem
	err := s.db.QueryRowContext(ctx, `
		SELECT m.id, p.provider_code, p.name, m.model_alias, m.provider_model,
			m.capability, m.status, m.created_at, m.updated_at
		FROM ai_models m
		JOIN ai_providers p ON p.id = m.provider_id
		WHERE m.model_alias = ?`,
		modelAlias,
	).Scan(
		&item.ID,
		&item.ProviderCode,
		&item.ProviderName,
		&item.ModelAlias,
		&item.ProviderModel,
		&item.Capability,
		&item.Status,
		&item.CreatedAt,
		&item.UpdatedAt,
	)
	return item, err
}

func (s *Store) adminFindAppModelRoute(ctx context.Context, appCode, modelAlias string) (AppModelRouteItem, error) {
	var item AppModelRouteItem
	err := s.db.QueryRowContext(ctx, `
		SELECT r.id, a.app_code, m.model_alias, p.provider_code, m.provider_model,
			m.capability, r.status, r.created_at
		FROM app_model_routes r
		JOIN apps a ON a.id = r.app_id
		JOIN ai_models m ON m.id = r.model_id
		JOIN ai_providers p ON p.id = m.provider_id
		WHERE a.app_code = ? AND m.model_alias = ?`,
		appCode,
		modelAlias,
	).Scan(
		&item.ID,
		&item.AppCode,
		&item.ModelAlias,
		&item.ProviderCode,
		&item.ProviderModel,
		&item.Capability,
		&item.Status,
		&item.CreatedAt,
	)
	if err == sql.ErrNoRows {
		return AppModelRouteItem{}, ErrPricingNotFound
	}
	return item, err
}
