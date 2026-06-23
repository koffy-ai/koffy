package billing

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

type AdminAppItem struct {
	ID          int64     `json:"id"`
	AppCode     string    `json:"app_code"`
	Name        string    `json:"name"`
	Status      string    `json:"status"`
	BillingMode string    `json:"billing_mode"`
	Description string    `json:"description"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type CreateAppRequest struct {
	AppCode     string `json:"app_code"`
	Name        string `json:"name"`
	BillingMode string `json:"billing_mode"`
	Description string `json:"description"`
}

type CreateAPIKeyResponse struct {
	AppCode string `json:"app_code"`
	Key     string `json:"key"`
	Prefix  string `json:"prefix"`
}

type UpsertPricingRequest struct {
	ModelAlias  string `json:"model_alias"`
	TokenAmount int64  `json:"token_amount"`
	CoinAmount  int64  `json:"coin_amount"`
}

type UpsertUnitPricingRequest struct {
	ModelAlias string `json:"model_alias"`
	Unit       string `json:"unit"`
	UnitAmount int64  `json:"unit_amount"`
	CoinAmount int64  `json:"coin_amount"`
}

type TokenPricingItem struct {
	ID            int64     `json:"id"`
	AppCode       string    `json:"app_code"`
	ModelAlias    string    `json:"model_alias"`
	TokenAmount   int64     `json:"token_amount"`
	CoinAmount    int64     `json:"coin_amount"`
	Status        string    `json:"status"`
	EffectiveFrom time.Time `json:"effective_from"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

type UnitPricingItem struct {
	ID            int64     `json:"id"`
	AppCode       string    `json:"app_code"`
	ModelAlias    string    `json:"model_alias"`
	Unit          string    `json:"unit"`
	UnitAmount    int64     `json:"unit_amount"`
	CoinAmount    int64     `json:"coin_amount"`
	Status        string    `json:"status"`
	EffectiveFrom time.Time `json:"effective_from"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

type AdminUserAsset struct {
	User   UserProfile   `json:"user"`
	Wallet WalletSummary `json:"wallet"`
}

type AdminUserSearchItem struct {
	CasdoorUserID string `json:"casdoor_user_id"`
	Name          string `json:"name"`
	DisplayName   string `json:"display_name"`
	Email         string `json:"email"`
	Phone         string `json:"phone"`
	IsAdmin       bool   `json:"is_admin"`
}

type AdminAdjustCoinsRequest struct {
	UserID      string `json:"user_id"`
	AmountCoins int64  `json:"amount_coins"`
	Remark      string `json:"remark"`
}

type AdminUsageRequestFilter struct {
	AppCode string
	UserID  string
	Limit   int
}

type AdminRechargeOrderFilter struct {
	UserID string
	Status string
	Limit  int
}

type AdminPaymentEventFilter struct {
	OrderNo string
	Limit   int
}

type AdminMetricsSummary struct {
	Days             int                       `json:"days"`
	Since            time.Time                 `json:"since"`
	Until            time.Time                 `json:"until"`
	UsageByApp       []AdminUsageMetricItem    `json:"usage_by_app"`
	RechargeByStatus []AdminRechargeMetricItem `json:"recharge_by_status"`
}

type AdminUsageMetricItem struct {
	AppCode             string `json:"app_code"`
	AppName             string `json:"app_name"`
	RequestCount        int64  `json:"request_count"`
	CommittedCount      int64  `json:"committed_count"`
	CancelledCount      int64  `json:"cancelled_count"`
	FailedCount         int64  `json:"failed_count"`
	ChargedCoins        int64  `json:"charged_coins"`
	ChargedUnits        int64  `json:"charged_units"`
	ActualTotalTokens   int64  `json:"actual_total_tokens"`
	ActualImages        int64  `json:"actual_images"`
	ActualVideoSeconds  int64  `json:"actual_video_seconds"`
	ActualBusinessUnits int64  `json:"actual_business_units"`
}

type AdminRechargeMetricItem struct {
	Status      string `json:"status"`
	OrderCount  int64  `json:"order_count"`
	AmountCents int64  `json:"amount_cents"`
	Coins       int64  `json:"coins"`
}

func (s *Store) AdminListApps(ctx context.Context) ([]AdminAppItem, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, app_code, name, status, billing_mode, description, created_at, updated_at
		FROM apps
		ORDER BY id DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]AdminAppItem, 0)
	for rows.Next() {
		var item AdminAppItem
		if err := rows.Scan(&item.ID, &item.AppCode, &item.Name, &item.Status, &item.BillingMode, &item.Description, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Store) AdminCreateApp(ctx context.Context, actorUserID int64, req CreateAppRequest) (AdminAppItem, error) {
	if req.BillingMode == "" {
		req.BillingMode = "hybrid"
	}
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO apps (app_code, name, billing_mode, description)
		VALUES (?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE
			name = VALUES(name),
			billing_mode = VALUES(billing_mode),
			description = VALUES(description),
			status = 'active'`,
		req.AppCode,
		req.Name,
		req.BillingMode,
		req.Description,
	)
	if err != nil {
		return AdminAppItem{}, err
	}
	app, err := s.adminFindApp(ctx, req.AppCode)
	if err != nil {
		return AdminAppItem{}, err
	}
	_ = s.writeAuditLog(ctx, actorUserID, "admin.app.upsert", "app", app.AppCode, req)
	return app, nil
}

func (s *Store) AdminCreateAPIKey(ctx context.Context, actorUserID int64, appCode string) (CreateAPIKeyResponse, error) {
	app, err := s.adminFindApp(ctx, appCode)
	if err != nil {
		return CreateAPIKeyResponse{}, err
	}

	key, err := randomAPIKey()
	if err != nil {
		return CreateAPIKeyResponse{}, err
	}
	hash := sha256.Sum256([]byte(key))
	prefix := key
	if len(prefix) > 12 {
		prefix = prefix[:12]
	}

	_, err = s.db.ExecContext(ctx, `
		INSERT INTO app_api_keys (app_id, key_prefix, key_hash, status)
		VALUES (?, ?, ?, 'active')`,
		app.ID,
		prefix,
		hex.EncodeToString(hash[:]),
	)
	if err != nil {
		return CreateAPIKeyResponse{}, err
	}
	_ = s.writeAuditLog(ctx, actorUserID, "admin.app.api_key.create", "app", app.AppCode, map[string]string{"prefix": prefix})

	return CreateAPIKeyResponse{AppCode: app.AppCode, Key: key, Prefix: prefix}, nil
}

func (s *Store) AdminUpsertPricing(ctx context.Context, actorUserID int64, appCode string, req UpsertPricingRequest) error {
	app, err := s.adminFindApp(ctx, appCode)
	if err != nil {
		return err
	}
	if req.ModelAlias == "" {
		req.ModelAlias = "*"
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer rollback(tx)

	if _, err := tx.ExecContext(ctx, `
		UPDATE app_token_pricing
		SET status = 'disabled'
		WHERE app_id = ? AND model_alias = ? AND status = 'active'`,
		app.ID,
		req.ModelAlias,
	); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO app_token_pricing (app_id, model_alias, token_amount, coin_amount, status)
		VALUES (?, ?, ?, ?, 'active')`,
		app.ID,
		req.ModelAlias,
		req.TokenAmount,
		req.CoinAmount,
	); err != nil {
		return err
	}
	if err := s.writeAuditLogTx(ctx, tx, actorUserID, "admin.app.pricing.upsert", "app", app.AppCode, req); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) AdminUpsertUnitPricing(ctx context.Context, actorUserID int64, appCode string, req UpsertUnitPricingRequest) error {
	app, err := s.adminFindApp(ctx, appCode)
	if err != nil {
		return err
	}
	if req.ModelAlias == "" {
		req.ModelAlias = "*"
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer rollback(tx)

	if _, err := tx.ExecContext(ctx, `
		UPDATE app_unit_pricing
		SET status = 'disabled'
		WHERE app_id = ? AND model_alias = ? AND unit = ? AND status = 'active'`,
		app.ID,
		req.ModelAlias,
		req.Unit,
	); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO app_unit_pricing (app_id, model_alias, unit, unit_amount, coin_amount, status)
		VALUES (?, ?, ?, ?, ?, 'active')`,
		app.ID,
		req.ModelAlias,
		req.Unit,
		req.UnitAmount,
		req.CoinAmount,
	); err != nil {
		return err
	}
	if err := s.writeAuditLogTx(ctx, tx, actorUserID, "admin.app.unit_pricing.upsert", "app", app.AppCode, req); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) AdminListPricing(ctx context.Context, appCode string) ([]TokenPricingItem, []UnitPricingItem, error) {
	app, err := s.adminFindApp(ctx, appCode)
	if err != nil {
		return nil, nil, err
	}

	tokenRows, err := s.db.QueryContext(ctx, `
		SELECT p.id, a.app_code, p.model_alias, p.token_amount, p.coin_amount,
			p.status, p.effective_from, p.created_at, p.updated_at
		FROM app_token_pricing p
		JOIN apps a ON a.id = p.app_id
		WHERE p.app_id = ?
			AND p.status = 'active'
		ORDER BY p.model_alias, p.effective_from DESC, p.id DESC`,
		app.ID,
	)
	if err != nil {
		return nil, nil, err
	}
	defer tokenRows.Close()

	tokenItems := make([]TokenPricingItem, 0)
	for tokenRows.Next() {
		var item TokenPricingItem
		if err := tokenRows.Scan(
			&item.ID,
			&item.AppCode,
			&item.ModelAlias,
			&item.TokenAmount,
			&item.CoinAmount,
			&item.Status,
			&item.EffectiveFrom,
			&item.CreatedAt,
			&item.UpdatedAt,
		); err != nil {
			return nil, nil, err
		}
		tokenItems = append(tokenItems, item)
	}
	if err := tokenRows.Err(); err != nil {
		return nil, nil, err
	}

	unitRows, err := s.db.QueryContext(ctx, `
		SELECT p.id, a.app_code, p.model_alias, p.unit, p.unit_amount, p.coin_amount,
			p.status, p.effective_from, p.created_at, p.updated_at
		FROM app_unit_pricing p
		JOIN apps a ON a.id = p.app_id
		WHERE p.app_id = ?
			AND p.status = 'active'
		ORDER BY p.unit, p.model_alias, p.effective_from DESC, p.id DESC`,
		app.ID,
	)
	if err != nil {
		return nil, nil, err
	}
	defer unitRows.Close()

	unitItems := make([]UnitPricingItem, 0)
	for unitRows.Next() {
		var item UnitPricingItem
		if err := unitRows.Scan(
			&item.ID,
			&item.AppCode,
			&item.ModelAlias,
			&item.Unit,
			&item.UnitAmount,
			&item.CoinAmount,
			&item.Status,
			&item.EffectiveFrom,
			&item.CreatedAt,
			&item.UpdatedAt,
		); err != nil {
			return nil, nil, err
		}
		unitItems = append(unitItems, item)
	}
	if err := unitRows.Err(); err != nil {
		return nil, nil, err
	}
	return tokenItems, unitItems, nil
}

func (s *Store) AdminDeleteTokenPricing(ctx context.Context, actorUserID, pricingID int64) error {
	var appCode string
	err := s.db.QueryRowContext(ctx, `
		SELECT a.app_code
		FROM app_token_pricing p
		JOIN apps a ON a.id = p.app_id
		WHERE p.id = ?`,
		pricingID,
	).Scan(&appCode)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrPricingNotFound
	}
	if err != nil {
		return err
	}
	result, err := s.db.ExecContext(ctx, `
		UPDATE app_token_pricing
		SET status = 'disabled'
		WHERE id = ? AND status = 'active'`,
		pricingID,
	)
	if err != nil {
		return err
	}
	if rows, _ := result.RowsAffected(); rows == 0 {
		return ErrPricingNotFound
	}
	return s.writeAuditLog(ctx, actorUserID, "admin.app.pricing.delete", "app", appCode, map[string]int64{"pricing_id": pricingID})
}

func (s *Store) AdminDeleteUnitPricing(ctx context.Context, actorUserID, pricingID int64) error {
	var appCode string
	err := s.db.QueryRowContext(ctx, `
		SELECT a.app_code
		FROM app_unit_pricing p
		JOIN apps a ON a.id = p.app_id
		WHERE p.id = ?`,
		pricingID,
	).Scan(&appCode)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrPricingNotFound
	}
	if err != nil {
		return err
	}
	result, err := s.db.ExecContext(ctx, `
		UPDATE app_unit_pricing
		SET status = 'disabled'
		WHERE id = ? AND status = 'active'`,
		pricingID,
	)
	if err != nil {
		return err
	}
	if rows, _ := result.RowsAffected(); rows == 0 {
		return ErrPricingNotFound
	}
	return s.writeAuditLog(ctx, actorUserID, "admin.app.unit_pricing.delete", "app", appCode, map[string]int64{"pricing_id": pricingID})
}

func (s *Store) AdminUserAsset(ctx context.Context, casdoorUserID string) (AdminUserAsset, error) {
	user, err := s.UserByCasdoorID(ctx, casdoorUserID)
	if err != nil {
		return AdminUserAsset{}, err
	}
	wallet, err := s.WalletSummary(ctx, user.ID)
	if err != nil {
		return AdminUserAsset{}, err
	}
	return AdminUserAsset{User: user, Wallet: wallet}, nil
}

func (s *Store) UserByCasdoorID(ctx context.Context, casdoorUserID string) (UserProfile, error) {
	var user UserProfile
	err := s.db.QueryRowContext(ctx, `
		SELECT id, casdoor_user_id, casdoor_owner, name, display_name, display_name_custom, email, phone, avatar_url, avatar_custom, is_admin, created_at, updated_at
		FROM users
		WHERE casdoor_user_id = ?`,
		casdoorUserID,
	).Scan(&user.ID, &user.CasdoorUserID, &user.Owner, &user.Name, &user.DisplayName, &user.DisplayCustom, &user.Email, &user.Phone, &user.AvatarURL, &user.AvatarCustom, &user.IsAdmin, &user.CreatedAt, &user.UpdatedAt)
	return user, err
}

func (s *Store) AdminSearchUsers(ctx context.Context, keyword string, limit int) ([]AdminUserSearchItem, error) {
	keyword = strings.TrimSpace(keyword)
	if keyword == "" {
		return []AdminUserSearchItem{}, nil
	}
	pattern := "%" + keyword + "%"
	rows, err := s.db.QueryContext(ctx, `
		SELECT casdoor_user_id, name, display_name, email, phone, is_admin
		FROM users
		WHERE casdoor_user_id LIKE ?
			OR name LIKE ?
			OR display_name LIKE ?
			OR phone LIKE ?
		ORDER BY updated_at DESC, id DESC
		LIMIT ?`,
		pattern,
		pattern,
		pattern,
		pattern,
		limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]AdminUserSearchItem, 0)
	for rows.Next() {
		var item AdminUserSearchItem
		if err := rows.Scan(&item.CasdoorUserID, &item.Name, &item.DisplayName, &item.Email, &item.Phone, &item.IsAdmin); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Store) AdminUserWalletLedger(ctx context.Context, casdoorUserID string, limit int) ([]WalletLedgerItem, error) {
	user, err := s.UserByCasdoorID(ctx, casdoorUserID)
	if err != nil {
		return nil, err
	}
	return s.WalletLedger(ctx, user.ID, limit)
}

func (s *Store) AdminUserSubscriptions(ctx context.Context, casdoorUserID string) ([]SubscriptionItem, error) {
	user, err := s.UserByCasdoorID(ctx, casdoorUserID)
	if err != nil {
		return nil, err
	}
	return s.Subscriptions(ctx, user.ID)
}

func (s *Store) AdminUserEntitlements(ctx context.Context, casdoorUserID string) ([]EntitlementItem, error) {
	user, err := s.UserByCasdoorID(ctx, casdoorUserID)
	if err != nil {
		return nil, err
	}
	return s.Entitlements(ctx, user.ID)
}

func (s *Store) AdminUserEntitlementLedger(ctx context.Context, casdoorUserID string, limit int) ([]EntitlementLedgerItem, error) {
	user, err := s.UserByCasdoorID(ctx, casdoorUserID)
	if err != nil {
		return nil, err
	}
	return s.EntitlementLedger(ctx, user.ID, limit)
}

func (s *Store) AdminUserUsageRequests(ctx context.Context, casdoorUserID string, limit int) ([]UsageRequestItem, error) {
	user, err := s.UserByCasdoorID(ctx, casdoorUserID)
	if err != nil {
		return nil, err
	}
	return s.UsageRequests(ctx, user.ID, limit)
}

func (s *Store) AdminUserRechargeOrders(ctx context.Context, casdoorUserID string, limit int) ([]RechargeOrderItem, error) {
	user, err := s.UserByCasdoorID(ctx, casdoorUserID)
	if err != nil {
		return nil, err
	}
	return s.RechargeOrders(ctx, user.ID, limit)
}

func (s *Store) AdminAdjustCoins(ctx context.Context, actorUserID int64, req AdminAdjustCoinsRequest) (WalletSummary, error) {
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return WalletSummary{}, err
	}
	defer rollback(tx)

	user, err := ensureUser(ctx, tx, req.UserID)
	if err != nil {
		return WalletSummary{}, err
	}
	wallet, err := findWalletForUpdate(ctx, tx, user.ID)
	if err != nil {
		return WalletSummary{}, err
	}

	balanceAfter := wallet.BalanceCoins + req.AmountCoins
	if balanceAfter < wallet.ReservedCoins {
		return WalletSummary{}, ErrInsufficientBalance
	}

	if _, err := tx.ExecContext(ctx, `
		UPDATE wallets
		SET balance_coins = ?, version = version + 1
		WHERE id = ?`,
		balanceAfter,
		wallet.ID,
	); err != nil {
		return WalletSummary{}, err
	}

	direction := "credit"
	amount := req.AmountCoins
	if req.AmountCoins < 0 {
		direction = "debit"
		amount = -req.AmountCoins
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO wallet_ledger (
			user_id, wallet_id, direction, reason, amount_coins,
			balance_after, admin_user_id, remark
		)
		VALUES (?, ?, ?, 'admin_adjustment', ?, ?, ?, ?)`,
		user.ID,
		wallet.ID,
		direction,
		amount,
		balanceAfter,
		nullInt64(actorUserID),
		req.Remark,
	); err != nil {
		return WalletSummary{}, err
	}

	if err := tx.Commit(); err != nil {
		return WalletSummary{}, err
	}
	_ = s.writeAuditLog(ctx, actorUserID, "admin.wallet.adjust", "user", req.UserID, req)
	return WalletSummary{
		BalanceCoins:   balanceAfter,
		ReservedCoins:  wallet.ReservedCoins,
		AvailableCoins: balanceAfter - wallet.ReservedCoins,
	}, nil
}

func (s *Store) AdminUsageRequests(ctx context.Context, filter AdminUsageRequestFilter) ([]UsageRequestItem, error) {
	if filter.Limit <= 0 || filter.Limit > 200 {
		filter.Limit = 100
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT r.id, a.app_code, a.name, u.casdoor_user_id,
			r.status, r.billing_mode, r.model_alias, r.provider, r.provider_model, r.provider_job_id,
			r.estimated_total_tokens, r.estimated_images, r.estimated_video_seconds, r.estimated_business_units,
			r.actual_prompt_tokens, r.actual_completion_tokens, r.actual_total_tokens,
			r.actual_images, r.actual_video_seconds, r.actual_business_units,
			r.reserved_coins, r.charged_coins, r.reserved_units, r.charged_units,
			r.error_code, r.error_message, r.created_at, r.authorized_at, r.committed_at, r.cancelled_at
		FROM usage_requests r
		JOIN apps a ON a.id = r.app_id
		JOIN users u ON u.id = r.user_id
		WHERE (? = '' OR a.app_code = ?)
			AND (? = '' OR u.casdoor_user_id = ?)
		ORDER BY r.id DESC
		LIMIT ?`,
		filter.AppCode,
		filter.AppCode,
		filter.UserID,
		filter.UserID,
		filter.Limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanUsageRequestItems(rows)
}

func (s *Store) AdminRechargeOrders(ctx context.Context, filter AdminRechargeOrderFilter) ([]RechargeOrderItem, error) {
	if filter.Limit <= 0 || filter.Limit > 200 {
		filter.Limit = 100
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT o.id, o.order_no, u.casdoor_user_id, o.provider, o.amount_cents, o.coins,
			o.status, o.provider_trade_no, o.paid_at, o.created_at, o.updated_at
		FROM recharge_orders o
		JOIN users u ON u.id = o.user_id
		WHERE (? = '' OR u.casdoor_user_id = ?)
			AND (? = '' OR o.status = ?)
		ORDER BY o.id DESC
		LIMIT ?`,
		filter.UserID,
		filter.UserID,
		filter.Status,
		filter.Status,
		filter.Limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanRechargeOrderItems(rows)
}

func (s *Store) AdminPaymentEvents(ctx context.Context, filter AdminPaymentEventFilter) ([]PaymentEventItem, error) {
	if filter.Limit <= 0 || filter.Limit > 200 {
		filter.Limit = 100
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, provider, event_id, event_type, order_no, provider_trade_no, processed_at, created_at
		FROM payment_events
		WHERE (? = '' OR order_no = ?)
		ORDER BY id DESC
		LIMIT ?`,
		filter.OrderNo,
		filter.OrderNo,
		filter.Limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]PaymentEventItem, 0)
	for rows.Next() {
		var item PaymentEventItem
		var processedAt sql.NullTime
		if err := rows.Scan(
			&item.ID,
			&item.Provider,
			&item.EventID,
			&item.EventType,
			&item.OrderNo,
			&item.ProviderTradeNo,
			&processedAt,
			&item.CreatedAt,
		); err != nil {
			return nil, err
		}
		item.ProcessedAt = nullTimePtr(processedAt)
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Store) AdminMetricsSummary(ctx context.Context, days int) (AdminMetricsSummary, error) {
	if days <= 0 || days > 90 {
		days = 7
	}
	until := localNow()
	since := until.AddDate(0, 0, -days)

	usageRows, err := s.db.QueryContext(ctx, `
		SELECT a.app_code, a.name,
			COUNT(*) AS request_count,
			COALESCE(SUM(CASE WHEN r.status = 'committed' THEN 1 ELSE 0 END), 0) AS committed_count,
			COALESCE(SUM(CASE WHEN r.status = 'cancelled' THEN 1 ELSE 0 END), 0) AS cancelled_count,
			COALESCE(SUM(CASE WHEN r.status = 'failed' THEN 1 ELSE 0 END), 0) AS failed_count,
			COALESCE(SUM(r.charged_coins), 0) AS charged_coins,
			COALESCE(SUM(r.charged_units), 0) AS charged_units,
			COALESCE(SUM(r.actual_total_tokens), 0) AS actual_total_tokens,
			COALESCE(SUM(r.actual_images), 0) AS actual_images,
			COALESCE(SUM(r.actual_video_seconds), 0) AS actual_video_seconds,
			COALESCE(SUM(r.actual_business_units), 0) AS actual_business_units
		FROM usage_requests r
		JOIN apps a ON a.id = r.app_id
		WHERE r.created_at >= ?
		GROUP BY a.app_code, a.name
		ORDER BY charged_coins DESC, request_count DESC, a.app_code`,
		since,
	)
	if err != nil {
		return AdminMetricsSummary{}, err
	}
	defer usageRows.Close()

	usageItems := make([]AdminUsageMetricItem, 0)
	for usageRows.Next() {
		var item AdminUsageMetricItem
		if err := usageRows.Scan(
			&item.AppCode,
			&item.AppName,
			&item.RequestCount,
			&item.CommittedCount,
			&item.CancelledCount,
			&item.FailedCount,
			&item.ChargedCoins,
			&item.ChargedUnits,
			&item.ActualTotalTokens,
			&item.ActualImages,
			&item.ActualVideoSeconds,
			&item.ActualBusinessUnits,
		); err != nil {
			return AdminMetricsSummary{}, err
		}
		usageItems = append(usageItems, item)
	}
	if err := usageRows.Err(); err != nil {
		return AdminMetricsSummary{}, err
	}

	rechargeRows, err := s.db.QueryContext(ctx, `
		SELECT status, COUNT(*) AS order_count,
			COALESCE(SUM(amount_cents), 0) AS amount_cents,
			COALESCE(SUM(coins), 0) AS coins
		FROM recharge_orders
		WHERE created_at >= ?
		GROUP BY status
		ORDER BY status`,
		since,
	)
	if err != nil {
		return AdminMetricsSummary{}, err
	}
	defer rechargeRows.Close()

	rechargeItems := make([]AdminRechargeMetricItem, 0)
	for rechargeRows.Next() {
		var item AdminRechargeMetricItem
		if err := rechargeRows.Scan(&item.Status, &item.OrderCount, &item.AmountCents, &item.Coins); err != nil {
			return AdminMetricsSummary{}, err
		}
		rechargeItems = append(rechargeItems, item)
	}
	if err := rechargeRows.Err(); err != nil {
		return AdminMetricsSummary{}, err
	}

	return AdminMetricsSummary{
		Days:             days,
		Since:            since,
		Until:            until,
		UsageByApp:       usageItems,
		RechargeByStatus: rechargeItems,
	}, nil
}

func (s *Store) writeAuditLog(ctx context.Context, actorUserID int64, action, targetType, targetID string, detail any) error {
	return s.writeAuditLogTx(ctx, s.db, actorUserID, action, targetType, targetID, detail)
}

type auditLogExecutor interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
}

func (s *Store) writeAuditLogTx(ctx context.Context, exec auditLogExecutor, actorUserID int64, action, targetType, targetID string, detail any) error {
	detailJSON, err := json.Marshal(detail)
	if err != nil {
		return err
	}
	_, err = exec.ExecContext(ctx, `
		INSERT INTO audit_logs (actor_user_id, action, target_type, target_id, detail_json)
		VALUES (?, ?, ?, ?, ?)`,
		nullInt64(actorUserID),
		action,
		targetType,
		targetID,
		detailJSON,
	)
	return err
}

func (s *Store) adminFindApp(ctx context.Context, appCode string) (AdminAppItem, error) {
	var item AdminAppItem
	err := s.db.QueryRowContext(ctx, `
		SELECT id, app_code, name, status, billing_mode, description, created_at, updated_at
		FROM apps
		WHERE app_code = ?`,
		appCode,
	).Scan(&item.ID, &item.AppCode, &item.Name, &item.Status, &item.BillingMode, &item.Description, &item.CreatedAt, &item.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return AdminAppItem{}, ErrAppNotFound
	}
	return item, err
}

func randomAPIKey() (string, error) {
	var raw [24]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", err
	}
	return "bgw_" + strings.ToLower(hex.EncodeToString(raw[:])), nil
}

func nullInt64(value int64) sql.NullInt64 {
	if value <= 0 {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: value, Valid: true}
}

func validatePositivePricing(req UpsertPricingRequest) error {
	if req.TokenAmount <= 0 || req.CoinAmount <= 0 {
		return fmt.Errorf("token_amount and coin_amount must be positive")
	}
	return nil
}

func validatePositiveUnitPricing(req UpsertUnitPricingRequest) error {
	if req.Unit != "images" && req.Unit != "video_seconds" && req.Unit != "business_units" {
		return fmt.Errorf("unit must be images, video_seconds or business_units")
	}
	if req.UnitAmount <= 0 || req.CoinAmount <= 0 {
		return fmt.Errorf("unit_amount and coin_amount must be positive")
	}
	return nil
}
