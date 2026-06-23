package billing

import (
	"context"
	"database/sql"
	"time"

	"koffy/internal/auth"
)

type UserProfile struct {
	ID            int64     `json:"id"`
	CasdoorUserID string    `json:"casdoor_user_id"`
	Name          string    `json:"name"`
	DisplayName   string    `json:"display_name"`
	DisplayCustom bool      `json:"display_name_custom"`
	Email         string    `json:"email"`
	Phone         string    `json:"phone"`
	AvatarURL     string    `json:"avatar_url"`
	AvatarCustom  bool      `json:"avatar_custom"`
	Owner         string    `json:"owner"`
	IsAdmin       bool      `json:"is_admin"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

type WalletSummary struct {
	BalanceCoins   int64 `json:"balance_coins"`
	ReservedCoins  int64 `json:"reserved_coins"`
	AvailableCoins int64 `json:"available_coins"`
}

type WalletLedgerItem struct {
	ID           int64     `json:"id"`
	Direction    string    `json:"direction"`
	Reason       string    `json:"reason"`
	AmountCoins  int64     `json:"amount_coins"`
	BalanceAfter int64     `json:"balance_after"`
	Remark       string    `json:"remark"`
	CreatedAt    time.Time `json:"created_at"`
}

type SubscriptionItem struct {
	ID       int64     `json:"id"`
	AppCode  string    `json:"app_code"`
	AppName  string    `json:"app_name"`
	PlanCode string    `json:"plan_code"`
	PlanName string    `json:"plan_name"`
	Status   string    `json:"status"`
	StartsAt time.Time `json:"starts_at"`
	EndsAt   time.Time `json:"ends_at"`
}

type EntitlementItem struct {
	ID              int64  `json:"id"`
	AppCode         string `json:"app_code"`
	AppName         string `json:"app_name"`
	EntitlementCode string `json:"entitlement_code"`
	Unit            string `json:"unit"`
	PeriodMonth     string `json:"period_month"`
	Quota           int64  `json:"quota"`
	Used            int64  `json:"used"`
	Reserved        int64  `json:"reserved"`
	Available       int64  `json:"available"`
}

type UsageRequestItem struct {
	ID                     int64      `json:"id"`
	AppCode                string     `json:"app_code"`
	AppName                string     `json:"app_name"`
	UserID                 string     `json:"user_id,omitempty"`
	Status                 string     `json:"status"`
	BillingMode            string     `json:"billing_mode"`
	ModelAlias             string     `json:"model_alias"`
	Provider               string     `json:"provider"`
	ProviderModel          string     `json:"provider_model"`
	ProviderJobID          string     `json:"provider_job_id"`
	EstimatedTotalTokens   int64      `json:"estimated_total_tokens"`
	EstimatedImages        int64      `json:"estimated_images"`
	EstimatedVideoSeconds  int64      `json:"estimated_video_seconds"`
	EstimatedBusinessUnits int64      `json:"estimated_business_units"`
	ActualPromptTokens     int64      `json:"actual_prompt_tokens"`
	ActualCompletionTokens int64      `json:"actual_completion_tokens"`
	ActualTotalTokens      int64      `json:"actual_total_tokens"`
	ActualImages           int64      `json:"actual_images"`
	ActualVideoSeconds     int64      `json:"actual_video_seconds"`
	ActualBusinessUnits    int64      `json:"actual_business_units"`
	ReservedCoins          int64      `json:"reserved_coins"`
	ChargedCoins           int64      `json:"charged_coins"`
	ReservedUnits          int64      `json:"reserved_units"`
	ChargedUnits           int64      `json:"charged_units"`
	ErrorCode              string     `json:"error_code"`
	ErrorMessage           string     `json:"error_message"`
	CreatedAt              time.Time  `json:"created_at"`
	AuthorizedAt           *time.Time `json:"authorized_at,omitempty"`
	CommittedAt            *time.Time `json:"committed_at,omitempty"`
	CancelledAt            *time.Time `json:"cancelled_at,omitempty"`
}

type EntitlementLedgerItem struct {
	ID              int64     `json:"id"`
	AppCode         string    `json:"app_code"`
	AppName         string    `json:"app_name"`
	EntitlementCode string    `json:"entitlement_code"`
	Unit            string    `json:"unit"`
	Direction       string    `json:"direction"`
	Amount          int64     `json:"amount"`
	UsedAfter       int64     `json:"used_after"`
	ReservedAfter   int64     `json:"reserved_after"`
	UsageRequestID  *int64    `json:"usage_request_id,omitempty"`
	CreatedAt       time.Time `json:"created_at"`
}

func (s *Store) SyncUser(ctx context.Context, principal auth.Principal) (UserProfile, error) {
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return UserProfile{}, err
	}
	defer rollback(tx)

	name := firstNonEmpty(principal.Name, principal.ID)
	if name == "" {
		name = principal.ID
	}
	phone := internalPhoneFromCasdoorPhone(principal.Phone)
	allowPhoneOverwrite := principalCanOverwriteStoredPhone(principal)
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO users (
			casdoor_user_id, casdoor_owner, name, display_name, email, phone, is_admin, last_login_at
		)
		VALUES (?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP(3))
		ON DUPLICATE KEY UPDATE
			casdoor_owner = VALUES(casdoor_owner),
			name = VALUES(name),
			display_name = CASE
				WHEN display_name_custom THEN display_name
				WHEN VALUES(display_name) != '' THEN VALUES(display_name)
				ELSE display_name
			END,
			email = VALUES(email),
			phone = CASE
				WHEN VALUES(phone) = '' THEN phone
				WHEN ? THEN VALUES(phone)
				WHEN phone = '' THEN VALUES(phone)
				ELSE phone
			END,
			is_admin = VALUES(is_admin),
			last_login_at = CURRENT_TIMESTAMP(3)`,
		principal.ID,
		principal.Owner,
		name,
		principal.DisplayName,
		principal.Email,
		phone,
		principal.IsAdmin,
		allowPhoneOverwrite,
	); err != nil {
		return UserProfile{}, err
	}

	user, err := findUserProfileForUpdate(ctx, tx, principal.ID)
	if err != nil {
		return UserProfile{}, err
	}
	if err := ensureWallet(ctx, tx, user.ID); err != nil {
		return UserProfile{}, err
	}
	if err := tx.Commit(); err != nil {
		return UserProfile{}, err
	}
	return user, nil
}

func (s *Store) WalletSummary(ctx context.Context, userID int64) (WalletSummary, error) {
	var wallet WalletSummary
	err := s.db.QueryRowContext(ctx, `
		SELECT balance_coins, reserved_coins, balance_coins - reserved_coins
		FROM wallets
		WHERE user_id = ?`,
		userID,
	).Scan(&wallet.BalanceCoins, &wallet.ReservedCoins, &wallet.AvailableCoins)
	return wallet, err
}

func (s *Store) WalletLedger(ctx context.Context, userID int64, limit int) ([]WalletLedgerItem, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, direction, reason, amount_coins, balance_after, remark, created_at
		FROM wallet_ledger
		WHERE user_id = ?
		ORDER BY id DESC
		LIMIT ?`,
		userID,
		limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]WalletLedgerItem, 0)
	for rows.Next() {
		var item WalletLedgerItem
		if err := rows.Scan(&item.ID, &item.Direction, &item.Reason, &item.AmountCoins, &item.BalanceAfter, &item.Remark, &item.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Store) Subscriptions(ctx context.Context, userID int64) ([]SubscriptionItem, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT s.id, a.app_code, a.name, p.plan_code, p.name, s.status, s.starts_at, s.ends_at
		FROM user_subscriptions s
		JOIN apps a ON a.id = s.app_id
		JOIN plans p ON p.id = s.plan_id
		WHERE s.user_id = ?
		ORDER BY s.created_at DESC`,
		userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]SubscriptionItem, 0)
	for rows.Next() {
		var item SubscriptionItem
		if err := rows.Scan(&item.ID, &item.AppCode, &item.AppName, &item.PlanCode, &item.PlanName, &item.Status, &item.StartsAt, &item.EndsAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Store) Entitlements(ctx context.Context, userID int64) ([]EntitlementItem, error) {
	if err := s.EnsureCurrentMonthEntitlementsForUser(ctx, userID); err != nil {
		return nil, err
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT b.id, a.app_code, a.name, b.entitlement_code, b.unit, b.period_month,
			b.quota, b.used, b.reserved, b.quota - b.used - b.reserved
		FROM entitlement_balances b
		JOIN apps a ON a.id = b.app_id
		JOIN user_subscriptions s ON s.id = b.subscription_id
		WHERE b.user_id = ?
			AND s.status = 'active'
			AND s.starts_at <= CURRENT_TIMESTAMP(3)
			AND s.ends_at > CURRENT_TIMESTAMP(3)
		ORDER BY b.period_month DESC, b.id DESC`,
		userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]EntitlementItem, 0)
	for rows.Next() {
		var item EntitlementItem
		if err := rows.Scan(
			&item.ID,
			&item.AppCode,
			&item.AppName,
			&item.EntitlementCode,
			&item.Unit,
			&item.PeriodMonth,
			&item.Quota,
			&item.Used,
			&item.Reserved,
			&item.Available,
		); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Store) UsageRequests(ctx context.Context, userID int64, limit int) ([]UsageRequestItem, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT u.id, a.app_code, a.name, '' AS casdoor_user_id,
			u.status, u.billing_mode, u.model_alias, u.provider, u.provider_model, u.provider_job_id,
			u.estimated_total_tokens, u.estimated_images, u.estimated_video_seconds, u.estimated_business_units,
			u.actual_prompt_tokens, u.actual_completion_tokens, u.actual_total_tokens,
			u.actual_images, u.actual_video_seconds, u.actual_business_units,
			u.reserved_coins, u.charged_coins, u.reserved_units, u.charged_units,
			u.error_code, u.error_message, u.created_at, u.authorized_at, u.committed_at, u.cancelled_at
		FROM usage_requests u
		JOIN apps a ON a.id = u.app_id
		WHERE u.user_id = ?
		ORDER BY u.id DESC
		LIMIT ?`,
		userID,
		limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanUsageRequestItems(rows)
}

func (s *Store) EntitlementLedger(ctx context.Context, userID int64, limit int) ([]EntitlementLedgerItem, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT l.id, a.app_code, a.name, b.entitlement_code, b.unit,
			l.direction, l.amount, l.used_after, l.reserved_after, l.usage_request_id, l.created_at
		FROM entitlement_ledger l
		JOIN entitlement_balances b ON b.id = l.entitlement_balance_id
		JOIN apps a ON a.id = l.app_id
		WHERE l.user_id = ?
		ORDER BY l.id DESC
		LIMIT ?`,
		userID,
		limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]EntitlementLedgerItem, 0)
	for rows.Next() {
		var item EntitlementLedgerItem
		var usageRequestID sql.NullInt64
		if err := rows.Scan(
			&item.ID,
			&item.AppCode,
			&item.AppName,
			&item.EntitlementCode,
			&item.Unit,
			&item.Direction,
			&item.Amount,
			&item.UsedAfter,
			&item.ReservedAfter,
			&usageRequestID,
			&item.CreatedAt,
		); err != nil {
			return nil, err
		}
		item.UsageRequestID = nullInt64Ptr(usageRequestID)
		items = append(items, item)
	}
	return items, rows.Err()
}

func scanUsageRequestItems(rows *sql.Rows) ([]UsageRequestItem, error) {
	items := make([]UsageRequestItem, 0)
	for rows.Next() {
		var item UsageRequestItem
		var authorizedAt, committedAt, cancelledAt sql.NullTime
		if err := rows.Scan(
			&item.ID,
			&item.AppCode,
			&item.AppName,
			&item.UserID,
			&item.Status,
			&item.BillingMode,
			&item.ModelAlias,
			&item.Provider,
			&item.ProviderModel,
			&item.ProviderJobID,
			&item.EstimatedTotalTokens,
			&item.EstimatedImages,
			&item.EstimatedVideoSeconds,
			&item.EstimatedBusinessUnits,
			&item.ActualPromptTokens,
			&item.ActualCompletionTokens,
			&item.ActualTotalTokens,
			&item.ActualImages,
			&item.ActualVideoSeconds,
			&item.ActualBusinessUnits,
			&item.ReservedCoins,
			&item.ChargedCoins,
			&item.ReservedUnits,
			&item.ChargedUnits,
			&item.ErrorCode,
			&item.ErrorMessage,
			&item.CreatedAt,
			&authorizedAt,
			&committedAt,
			&cancelledAt,
		); err != nil {
			return nil, err
		}
		item.AuthorizedAt = nullTimePtr(authorizedAt)
		item.CommittedAt = nullTimePtr(committedAt)
		item.CancelledAt = nullTimePtr(cancelledAt)
		items = append(items, item)
	}
	return items, rows.Err()
}

func nullTimePtr(value sql.NullTime) *time.Time {
	if !value.Valid {
		return nil
	}
	return &value.Time
}

func nullInt64Ptr(value sql.NullInt64) *int64 {
	if !value.Valid {
		return nil
	}
	return &value.Int64
}

func findUserProfileForUpdate(ctx context.Context, tx *sql.Tx, casdoorUserID string) (UserProfile, error) {
	var user UserProfile
	err := tx.QueryRowContext(ctx, `
		SELECT id, casdoor_user_id, casdoor_owner, name, display_name, display_name_custom, email, phone, avatar_url, avatar_custom, is_admin, created_at, updated_at
		FROM users
		WHERE casdoor_user_id = ?
		FOR UPDATE`,
		casdoorUserID,
	).Scan(
		&user.ID,
		&user.CasdoorUserID,
		&user.Owner,
		&user.Name,
		&user.DisplayName,
		&user.DisplayCustom,
		&user.Email,
		&user.Phone,
		&user.AvatarURL,
		&user.AvatarCustom,
		&user.IsAdmin,
		&user.CreatedAt,
		&user.UpdatedAt,
	)
	return user, err
}

func findUserProfileByIDForUpdate(ctx context.Context, tx *sql.Tx, userID int64) (UserProfile, error) {
	var user UserProfile
	err := tx.QueryRowContext(ctx, `
		SELECT id, casdoor_user_id, casdoor_owner, name, display_name, display_name_custom, email, phone, avatar_url, avatar_custom, is_admin, created_at, updated_at
		FROM users
		WHERE id = ?
		FOR UPDATE`,
		userID,
	).Scan(
		&user.ID,
		&user.CasdoorUserID,
		&user.Owner,
		&user.Name,
		&user.DisplayName,
		&user.DisplayCustom,
		&user.Email,
		&user.Phone,
		&user.AvatarURL,
		&user.AvatarCustom,
		&user.IsAdmin,
		&user.CreatedAt,
		&user.UpdatedAt,
	)
	return user, err
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
