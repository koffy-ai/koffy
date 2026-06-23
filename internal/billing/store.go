package billing

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"

	"koffy/internal/contracts"
)

var (
	ErrAppNotFound           = errors.New("app not found")
	ErrInsufficientBalance   = errors.New("insufficient balance")
	ErrRequestConflict       = errors.New("usage request status conflict")
	ErrUsageNotFound         = errors.New("usage request not found")
	ErrPricingNotFound       = errors.New("pricing not found")
	ErrRechargeOrderNotFound = errors.New("recharge order not found")
	ErrVerificationTooSoon   = errors.New("verification code requested too frequently")
	ErrVerificationInvalid   = errors.New("verification code is invalid")
	ErrVerificationExpired   = errors.New("verification code is expired")
	ErrVerificationLocked    = errors.New("verification code has too many failed attempts")
)

type Store struct {
	db *sql.DB
}

func NewStore(db *sql.DB) *Store {
	return &Store{db: db}
}

type appRecord struct {
	ID          int64
	AppCode     string
	BillingMode contracts.BillingMode
}

type userRecord struct {
	ID int64
}

type walletRecord struct {
	ID            int64
	BalanceCoins  int64
	ReservedCoins int64
}

type pricingRecord struct {
	TokenAmount int64
	CoinAmount  int64
}

type unitPricingRecord struct {
	UnitAmount int64
	CoinAmount int64
}

type usageUnitDemand struct {
	Unit   string
	Amount int64
}

type usageRecord struct {
	ID                     int64
	AppID                  int64
	UserID                 int64
	Status                 string
	BillingMode            contracts.BillingMode
	ModelAlias             string
	ReservedCoins          int64
	ReservedUnits          int64
	ChargedCoins           int64
	ChargedUnits           int64
	ActualPromptTokens     int64
	ActualCompletionTokens int64
	ActualTotalTokens      int64
}

func (s *Store) Authorize(ctx context.Context, req contracts.AuthorizeRequest) (contracts.AuthorizeResponse, error) {
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return contracts.AuthorizeResponse{}, err
	}
	defer rollback(tx)

	app, err := findAppForUpdate(ctx, tx, req.AppID)
	if err != nil {
		return contracts.AuthorizeResponse{}, err
	}

	user, err := ensureUser(ctx, tx, req.UserID)
	if err != nil {
		return contracts.AuthorizeResponse{}, err
	}

	existing, err := findUsageForUpdate(ctx, tx, app.ID, user.ID, req.IdempotencyKey)
	if err != nil {
		return contracts.AuthorizeResponse{}, err
	}
	if existing != nil {
		return usageToAuthorizeResponse(*existing), tx.Commit()
	}

	billingMode := app.BillingMode
	if req.BillingMode != "" {
		billingMode = req.BillingMode
	}

	usageID, err := insertUsage(ctx, tx, app.ID, user.ID, req.IdempotencyKey, billingMode, req.Model, req.EstimatedUsage)
	if err != nil {
		return contracts.AuthorizeResponse{}, err
	}

	reservedUnits, remainingForCoins, err := reserveEntitlements(ctx, tx, usageID, app.ID, user.ID, billingMode, req.EstimatedUsage)
	if err != nil {
		return contracts.AuthorizeResponse{}, err
	}

	coinsForUsage, err := priceUsageCoins(ctx, tx, app.ID, req.Model, billingMode, remainingForCoins)
	if err != nil {
		return contracts.AuthorizeResponse{}, err
	}

	reservedCoins, err := reserveCoins(ctx, tx, usageID, user.ID, billingMode, coinsForUsage)
	if err != nil {
		return contracts.AuthorizeResponse{}, err
	}

	if _, err := tx.ExecContext(ctx, `
		UPDATE usage_requests
		SET reserved_coins = ?, reserved_units = ?, authorized_at = CURRENT_TIMESTAMP(3)
		WHERE id = ?`,
		reservedCoins, reservedUnits, usageID,
	); err != nil {
		return contracts.AuthorizeResponse{}, err
	}

	if err := tx.Commit(); err != nil {
		return contracts.AuthorizeResponse{}, err
	}

	return contracts.AuthorizeResponse{
		UsageRequestID: fmt.Sprintf("%d", usageID),
		Status:         "authorized",
		ReservedCoins:  reservedCoins,
		ReservedUnits:  reservedUnits,
	}, nil
}

func (s *Store) Commit(ctx context.Context, req contracts.CommitRequest) (contracts.CommitResponse, error) {
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return contracts.CommitResponse{}, err
	}
	defer rollback(tx)

	usageID, err := parseID(req.UsageRequestID)
	if err != nil {
		return contracts.CommitResponse{}, err
	}

	usage, err := findUsageByIDForUpdate(ctx, tx, usageID)
	if err != nil {
		return contracts.CommitResponse{}, err
	}
	if usage.Status == "committed" {
		return usageToCommitResponse(usage), tx.Commit()
	}
	if usage.Status != "authorized" {
		return contracts.CommitResponse{}, ErrRequestConflict
	}

	chargedUnits, remainingForCoins, err := consumeEntitlements(ctx, tx, usage, req.ActualUsage)
	if err != nil {
		return contracts.CommitResponse{}, err
	}

	coinsForUsage, err := priceUsageCoins(ctx, tx, usage.AppID, usage.ModelAlias, usage.BillingMode, remainingForCoins)
	if err != nil {
		return contracts.CommitResponse{}, err
	}

	chargedCoins, err := chargeCoins(ctx, tx, usage, coinsForUsage)
	if err != nil {
		return contracts.CommitResponse{}, err
	}

	if err := releaseUnusedReservations(ctx, tx, usageID); err != nil {
		return contracts.CommitResponse{}, err
	}

	if _, err := tx.ExecContext(ctx, `
		UPDATE usage_requests
		SET status = 'committed',
			provider = ?,
			provider_model = ?,
			provider_job_id = ?,
			actual_prompt_tokens = ?,
			actual_completion_tokens = ?,
			actual_total_tokens = ?,
			actual_images = ?,
			actual_video_seconds = ?,
			actual_business_units = ?,
			charged_coins = ?,
			charged_units = ?,
			committed_at = CURRENT_TIMESTAMP(3)
		WHERE id = ?`,
		req.Provider,
		req.Model,
		req.ProviderJobID,
		req.ActualUsage.PromptTokens,
		req.ActualUsage.CompletionTokens,
		req.ActualUsage.TotalTokens,
		req.ActualUsage.Images,
		req.ActualUsage.VideoSeconds,
		req.ActualUsage.BusinessUnits,
		chargedCoins,
		chargedUnits,
		usageID,
	); err != nil {
		return contracts.CommitResponse{}, err
	}

	if err := tx.Commit(); err != nil {
		return contracts.CommitResponse{}, err
	}

	return contracts.CommitResponse{
		UsageRequestID: req.UsageRequestID,
		Status:         "committed",
		ChargedCoins:   chargedCoins,
		ChargedUnits:   chargedUnits,
	}, nil
}

func (s *Store) Cancel(ctx context.Context, req contracts.CancelRequest) error {
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return err
	}
	defer rollback(tx)

	usageID, err := parseID(req.UsageRequestID)
	if err != nil {
		return err
	}

	usage, err := findUsageByIDForUpdate(ctx, tx, usageID)
	if err != nil {
		return err
	}
	if usage.Status == "cancelled" {
		return tx.Commit()
	}
	if usage.Status != "authorized" {
		return ErrRequestConflict
	}

	if err := releaseUnusedReservations(ctx, tx, usageID); err != nil {
		return err
	}

	_, err = tx.ExecContext(ctx, `
		UPDATE usage_requests
		SET status = 'cancelled', error_message = ?, cancelled_at = CURRENT_TIMESTAMP(3)
		WHERE id = ?`,
		req.Reason,
		usageID,
	)
	if err != nil {
		return err
	}

	return tx.Commit()
}

func (s *Store) Charge(ctx context.Context, req contracts.ChargeRequest) (contracts.CommitResponse, error) {
	auth, err := s.Authorize(ctx, contracts.AuthorizeRequest{
		AppID:          req.AppID,
		UserID:         req.UserID,
		IdempotencyKey: req.IdempotencyKey,
		Model:          req.Model,
		EstimatedUsage: req.ActualUsage,
	})
	if err != nil {
		return contracts.CommitResponse{}, err
	}

	return s.Commit(ctx, contracts.CommitRequest{
		UsageRequestID: auth.UsageRequestID,
		Model:          req.Model,
		ActualUsage:    req.ActualUsage,
	})
}

func findAppForUpdate(ctx context.Context, tx *sql.Tx, appCode string) (appRecord, error) {
	var app appRecord
	err := tx.QueryRowContext(ctx, `
		SELECT id, app_code, billing_mode
		FROM apps
		WHERE app_code = ? AND status = 'active'
		FOR UPDATE`,
		appCode,
	).Scan(&app.ID, &app.AppCode, &app.BillingMode)
	if errors.Is(err, sql.ErrNoRows) {
		return appRecord{}, ErrAppNotFound
	}
	return app, err
}

func ensureUser(ctx context.Context, tx *sql.Tx, casdoorUserID string) (userRecord, error) {
	var user userRecord
	err := tx.QueryRowContext(ctx, `
		SELECT id FROM users WHERE casdoor_user_id = ? FOR UPDATE`,
		casdoorUserID,
	).Scan(&user.ID)
	if err == nil {
		if err := ensureWallet(ctx, tx, user.ID); err != nil {
			return userRecord{}, err
		}
		return user, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return userRecord{}, err
	}

	result, err := tx.ExecContext(ctx, `
		INSERT INTO users (casdoor_user_id, casdoor_owner, name)
		VALUES (?, '', ?)`,
		casdoorUserID,
		casdoorUserID,
	)
	if err != nil {
		return userRecord{}, err
	}
	user.ID, err = result.LastInsertId()
	if err != nil {
		return userRecord{}, err
	}
	return user, ensureWallet(ctx, tx, user.ID)
}

func ensureWallet(ctx context.Context, tx *sql.Tx, userID int64) error {
	_, err := tx.ExecContext(ctx, `
		INSERT IGNORE INTO wallets (user_id, balance_coins, reserved_coins)
		VALUES (?, 0, 0)`,
		userID,
	)
	return err
}

func findUsageForUpdate(ctx context.Context, tx *sql.Tx, appID, userID int64, key string) (*usageRecord, error) {
	row := tx.QueryRowContext(ctx, `
		SELECT id, app_id, user_id, status, billing_mode, model_alias,
			reserved_coins, reserved_units, charged_coins, charged_units,
			actual_prompt_tokens, actual_completion_tokens, actual_total_tokens
		FROM usage_requests
		WHERE app_id = ? AND user_id = ? AND idempotency_key = ?
		FOR UPDATE`,
		appID,
		userID,
		key,
	)
	usage, err := scanUsage(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &usage, nil
}

func findUsageByIDForUpdate(ctx context.Context, tx *sql.Tx, usageID int64) (usageRecord, error) {
	row := tx.QueryRowContext(ctx, `
		SELECT id, app_id, user_id, status, billing_mode, model_alias,
			reserved_coins, reserved_units, charged_coins, charged_units,
			actual_prompt_tokens, actual_completion_tokens, actual_total_tokens
		FROM usage_requests
		WHERE id = ?
		FOR UPDATE`,
		usageID,
	)
	usage, err := scanUsage(row)
	if errors.Is(err, sql.ErrNoRows) {
		return usageRecord{}, ErrUsageNotFound
	}
	return usage, err
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanUsage(row rowScanner) (usageRecord, error) {
	var usage usageRecord
	err := row.Scan(
		&usage.ID,
		&usage.AppID,
		&usage.UserID,
		&usage.Status,
		&usage.BillingMode,
		&usage.ModelAlias,
		&usage.ReservedCoins,
		&usage.ReservedUnits,
		&usage.ChargedCoins,
		&usage.ChargedUnits,
		&usage.ActualPromptTokens,
		&usage.ActualCompletionTokens,
		&usage.ActualTotalTokens,
	)
	return usage, err
}

func insertUsage(ctx context.Context, tx *sql.Tx, appID, userID int64, key string, mode contracts.BillingMode, model string, usage contracts.Usage) (int64, error) {
	result, err := tx.ExecContext(ctx, `
		INSERT INTO usage_requests (
			app_id, user_id, idempotency_key, status, billing_mode, model_alias,
			estimated_total_tokens, estimated_images, estimated_video_seconds, estimated_business_units, authorized_at
		)
		VALUES (?, ?, ?, 'authorized', ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP(3))`,
		appID,
		userID,
		key,
		mode,
		model,
		usage.TotalTokens,
		usage.Images,
		usage.VideoSeconds,
		usage.BusinessUnits,
	)
	if err != nil {
		return 0, err
	}
	return result.LastInsertId()
}

func reserveEntitlements(ctx context.Context, tx *sql.Tx, usageID, appID, userID int64, mode contracts.BillingMode, usage contracts.Usage) (int64, map[string]int64, error) {
	demands := entitlementDemands(usage)
	if len(demands) == 0 || mode == contracts.BillingModeCoins {
		return 0, usageDemandMap(demands), nil
	}
	if err := ensureCurrentMonthEntitlementsTx(ctx, tx, userID, appID); err != nil {
		return 0, nil, err
	}

	var reserved int64
	remainingByUnit := usageDemandMap(demands)
	for _, demand := range demands {
		unitReserved, err := reserveEntitlementUnit(ctx, tx, usageID, appID, userID, demand.Unit, demand.Amount)
		if err != nil {
			return 0, nil, err
		}
		reserved += unitReserved
		remainingByUnit[demand.Unit] -= unitReserved
	}

	if err := rejectRemainingEntitlementUsage(mode, remainingByUnit); err != nil {
		return 0, nil, err
	}
	return reserved, remainingByUnit, nil
}

func reserveEntitlementUnit(ctx context.Context, tx *sql.Tx, usageID, appID, userID int64, unit string, amount int64) (int64, error) {
	if amount <= 0 {
		return 0, nil
	}

	period := localNow().Format("2006-01")
	rows, err := tx.QueryContext(ctx, `
		SELECT b.id, b.quota, b.used, b.reserved
		FROM entitlement_balances b
		JOIN user_subscriptions s ON s.id = b.subscription_id
		WHERE b.user_id = ?
			AND b.app_id = ?
			AND b.period_month = ?
			AND b.unit = ?
			AND s.status = 'active'
			AND s.starts_at <= CURRENT_TIMESTAMP(3)
			AND s.ends_at > CURRENT_TIMESTAMP(3)
		ORDER BY b.id
		FOR UPDATE`,
		userID,
		appID,
		period,
		unit,
	)
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	remaining := amount
	var reserved int64
	type entitlementBalance struct {
		id       int64
		quota    int64
		used     int64
		reserved int64
	}
	balances := make([]entitlementBalance, 0)
	for rows.Next() {
		var item entitlementBalance
		if err := rows.Scan(&item.id, &item.quota, &item.used, &item.reserved); err != nil {
			return 0, err
		}
		balances = append(balances, item)
	}
	if err := rows.Err(); err != nil {
		return 0, err
	}
	if err := rows.Close(); err != nil {
		return 0, err
	}

	for _, balance := range balances {
		if remaining <= 0 {
			break
		}

		available := balance.quota - balance.used - balance.reserved
		if available <= 0 {
			continue
		}

		amount := min64(available, remaining)
		if _, err := tx.ExecContext(ctx, `
			UPDATE entitlement_balances SET reserved = reserved + ? WHERE id = ?`,
			amount,
			balance.id,
		); err != nil {
			return 0, err
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO usage_reservations (usage_request_id, reservation_type, entitlement_balance_id, amount)
			VALUES (?, 'entitlement', ?, ?)`,
			usageID,
			balance.id,
			amount,
		); err != nil {
			return 0, err
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO entitlement_ledger (
				entitlement_balance_id, user_id, app_id, direction, amount,
				used_after, reserved_after, usage_request_id
			)
			VALUES (?, ?, ?, 'reserve', ?, ?, ?, ?)`,
			balance.id,
			userID,
			appID,
			amount,
			balance.used,
			balance.reserved+amount,
			usageID,
		); err != nil {
			return 0, err
		}
		reserved += amount
		remaining -= amount
	}

	return reserved, nil
}

func reserveCoins(ctx context.Context, tx *sql.Tx, usageID, userID int64, mode contracts.BillingMode, coins int64) (int64, error) {
	if coins <= 0 || mode == contracts.BillingModeEntitlement {
		return 0, nil
	}

	wallet, err := findWalletForUpdate(ctx, tx, userID)
	if err != nil {
		return 0, err
	}
	if wallet.BalanceCoins-wallet.ReservedCoins < coins {
		return 0, ErrInsufficientBalance
	}

	if _, err := tx.ExecContext(ctx, `
		UPDATE wallets
		SET reserved_coins = reserved_coins + ?, version = version + 1
		WHERE id = ?`,
		coins,
		wallet.ID,
	); err != nil {
		return 0, err
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO usage_reservations (usage_request_id, reservation_type, wallet_id, amount)
		VALUES (?, 'coins', ?, ?)`,
		usageID,
		wallet.ID,
		coins,
	); err != nil {
		return 0, err
	}
	_, err = tx.ExecContext(ctx, `
		INSERT INTO wallet_ledger (
			user_id, wallet_id, direction, reason, amount_coins,
			balance_after, usage_request_id
		)
		VALUES (?, ?, 'reserve', 'reservation', ?, ?, ?)`,
		userID,
		wallet.ID,
		coins,
		wallet.BalanceCoins,
		usageID,
	)
	return coins, err
}

func consumeEntitlements(ctx context.Context, tx *sql.Tx, usage usageRecord, actualUsage contracts.Usage) (int64, map[string]int64, error) {
	demands := entitlementDemands(actualUsage)
	if len(demands) == 0 || usage.BillingMode == contracts.BillingModeCoins {
		return 0, usageDemandMap(demands), nil
	}

	rows, err := tx.QueryContext(ctx, `
		SELECT r.id, r.entitlement_balance_id, r.amount, b.unit, b.used, b.reserved
		FROM usage_reservations r
		JOIN entitlement_balances b ON b.id = r.entitlement_balance_id
		WHERE r.usage_request_id = ? AND r.reservation_type = 'entitlement' AND r.status = 'reserved'
		ORDER BY r.id
		FOR UPDATE`,
		usage.ID,
	)
	if err != nil {
		return 0, nil, err
	}
	defer rows.Close()

	remainingByUnit := usageDemandMap(demands)
	var consumed int64
	type entitlementReservation struct {
		reservationID  int64
		balanceID      int64
		reservedAmount int64
		unit           string
		used           int64
		reserved       int64
	}
	reservations := make([]entitlementReservation, 0)
	for rows.Next() {
		var item entitlementReservation
		if err := rows.Scan(&item.reservationID, &item.balanceID, &item.reservedAmount, &item.unit, &item.used, &item.reserved); err != nil {
			return 0, nil, err
		}
		reservations = append(reservations, item)
	}
	if err := rows.Err(); err != nil {
		return 0, nil, err
	}
	if err := rows.Close(); err != nil {
		return 0, nil, err
	}

	for _, reservation := range reservations {
		remaining := remainingByUnit[reservation.unit]
		if remaining <= 0 {
			continue
		}
		amount := min64(reservation.reservedAmount, remaining)
		if _, err := tx.ExecContext(ctx, `
			UPDATE entitlement_balances
			SET used = used + ?, reserved = reserved - ?
			WHERE id = ?`,
			amount,
			reservation.reservedAmount,
			reservation.balanceID,
		); err != nil {
			return 0, nil, err
		}
		if _, err := tx.ExecContext(ctx, `
			UPDATE usage_reservations
			SET status = 'consumed', consumed_amount = ?, released_amount = ?
			WHERE id = ?`,
			amount,
			reservation.reservedAmount-amount,
			reservation.reservationID,
		); err != nil {
			return 0, nil, err
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO entitlement_ledger (
				entitlement_balance_id, user_id, app_id, direction, amount,
				used_after, reserved_after, usage_request_id
			)
			VALUES (?, ?, ?, 'consume', ?, ?, ?, ?)`,
			reservation.balanceID,
			usage.UserID,
			usage.AppID,
			amount,
			reservation.used+amount,
			reservation.reserved-reservation.reservedAmount,
			usage.ID,
		); err != nil {
			return 0, nil, err
		}
		consumed += amount
		remainingByUnit[reservation.unit] -= amount
	}

	if err := rejectRemainingEntitlementUsage(usage.BillingMode, remainingByUnit); err != nil {
		return 0, nil, err
	}
	return consumed, remainingByUnit, nil
}

func chargeCoins(ctx context.Context, tx *sql.Tx, usage usageRecord, coins int64) (int64, error) {
	if coins <= 0 || usage.BillingMode == contracts.BillingModeEntitlement {
		return 0, nil
	}

	wallet, err := findWalletForUpdate(ctx, tx, usage.UserID)
	if err != nil {
		return 0, err
	}

	reservedCoins, err := reservedCoinsForUsage(ctx, tx, usage.ID)
	if err != nil {
		return 0, err
	}
	extraCoins := max64(0, coins-reservedCoins)
	if wallet.BalanceCoins-wallet.ReservedCoins < extraCoins {
		return 0, ErrInsufficientBalance
	}

	if _, err := tx.ExecContext(ctx, `
		UPDATE wallets
		SET balance_coins = balance_coins - ?,
			reserved_coins = reserved_coins - ?,
			version = version + 1
		WHERE id = ?`,
		coins,
		reservedCoins,
		wallet.ID,
	); err != nil {
		return 0, err
	}

	if _, err := tx.ExecContext(ctx, `
		UPDATE usage_reservations
		SET status = 'consumed',
			consumed_amount = LEAST(amount, ?),
			released_amount = GREATEST(amount - ?, 0)
		WHERE usage_request_id = ? AND reservation_type = 'coins' AND status = 'reserved'`,
		coins,
		coins,
		usage.ID,
	); err != nil {
		return 0, err
	}

	balanceAfter := wallet.BalanceCoins - coins
	_, err = tx.ExecContext(ctx, `
		INSERT INTO wallet_ledger (
			user_id, wallet_id, direction, reason, amount_coins,
			balance_after, usage_request_id
		)
		VALUES (?, ?, 'debit', 'usage', ?, ?, ?)`,
		usage.UserID,
		wallet.ID,
		coins,
		balanceAfter,
		usage.ID,
	)
	return coins, err
}

func releaseUnusedReservations(ctx context.Context, tx *sql.Tx, usageID int64) error {
	rows, err := tx.QueryContext(ctx, `
		SELECT id, reservation_type, wallet_id, entitlement_balance_id, amount
		FROM usage_reservations
		WHERE usage_request_id = ? AND status = 'reserved'
		FOR UPDATE`,
		usageID,
	)
	if err != nil {
		return err
	}
	defer rows.Close()

	type reservation struct {
		id        int64
		kind      string
		walletID  sql.NullInt64
		balanceID sql.NullInt64
		amount    int64
	}
	var reservations []reservation
	for rows.Next() {
		var item reservation
		if err := rows.Scan(&item.id, &item.kind, &item.walletID, &item.balanceID, &item.amount); err != nil {
			return err
		}
		reservations = append(reservations, item)
	}
	if err := rows.Err(); err != nil {
		return err
	}

	for _, item := range reservations {
		if item.kind == "coins" && item.walletID.Valid {
			var userID, balanceCoins int64
			if err := tx.QueryRowContext(ctx, `
				SELECT user_id, balance_coins
				FROM wallets
				WHERE id = ?
				FOR UPDATE`,
				item.walletID.Int64,
			).Scan(&userID, &balanceCoins); err != nil {
				return err
			}
			if _, err := tx.ExecContext(ctx, `
				UPDATE wallets
				SET reserved_coins = reserved_coins - ?, version = version + 1
				WHERE id = ?`,
				item.amount,
				item.walletID.Int64,
			); err != nil {
				return err
			}
			if _, err := tx.ExecContext(ctx, `
				INSERT INTO wallet_ledger (
					user_id, wallet_id, direction, reason, amount_coins,
					balance_after, usage_request_id
				)
				VALUES (?, ?, 'release', 'reservation_release', ?, ?, ?)`,
				userID,
				item.walletID.Int64,
				item.amount,
				balanceCoins,
				usageID,
			); err != nil {
				return err
			}
		}
		if item.kind == "entitlement" && item.balanceID.Valid {
			var userID, appID, used, reserved int64
			if err := tx.QueryRowContext(ctx, `
				SELECT user_id, app_id, used, reserved
				FROM entitlement_balances
				WHERE id = ?
				FOR UPDATE`,
				item.balanceID.Int64,
			).Scan(&userID, &appID, &used, &reserved); err != nil {
				return err
			}
			if _, err := tx.ExecContext(ctx, `
				UPDATE entitlement_balances
				SET reserved = reserved - ?
				WHERE id = ?`,
				item.amount,
				item.balanceID.Int64,
			); err != nil {
				return err
			}
			if _, err := tx.ExecContext(ctx, `
				INSERT INTO entitlement_ledger (
					entitlement_balance_id, user_id, app_id, direction, amount,
					used_after, reserved_after, usage_request_id
				)
				VALUES (?, ?, ?, 'release', ?, ?, ?, ?)`,
				item.balanceID.Int64,
				userID,
				appID,
				item.amount,
				used,
				reserved-item.amount,
				usageID,
			); err != nil {
				return err
			}
		}
		if _, err := tx.ExecContext(ctx, `
			UPDATE usage_reservations
			SET status = 'released', released_amount = amount
			WHERE id = ?`,
			item.id,
		); err != nil {
			return err
		}
	}
	return nil
}

func findPricing(ctx context.Context, tx *sql.Tx, appID int64, model string) (pricingRecord, error) {
	var pricing pricingRecord
	err := tx.QueryRowContext(ctx, `
		SELECT token_amount, coin_amount
		FROM app_token_pricing
		WHERE app_id = ?
			AND status = 'active'
			AND effective_from <= CURRENT_TIMESTAMP(3)
			AND model_alias IN (?, '*')
		ORDER BY IF(model_alias = ?, 0, 1), effective_from DESC, id DESC
		LIMIT 1`,
		appID,
		model,
		model,
	).Scan(&pricing.TokenAmount, &pricing.CoinAmount)
	if errors.Is(err, sql.ErrNoRows) {
		return pricingRecord{}, ErrPricingNotFound
	}
	return pricing, err
}

func findUnitPricing(ctx context.Context, tx *sql.Tx, appID int64, model, unit string) (unitPricingRecord, error) {
	var pricing unitPricingRecord
	err := tx.QueryRowContext(ctx, `
		SELECT unit_amount, coin_amount
		FROM app_unit_pricing
		WHERE app_id = ?
			AND unit = ?
			AND status = 'active'
			AND effective_from <= CURRENT_TIMESTAMP(3)
			AND model_alias IN (?, '*')
		ORDER BY IF(model_alias = ?, 0, 1), effective_from DESC, id DESC
		LIMIT 1`,
		appID,
		unit,
		model,
		model,
	).Scan(&pricing.UnitAmount, &pricing.CoinAmount)
	if errors.Is(err, sql.ErrNoRows) {
		return unitPricingRecord{}, ErrPricingNotFound
	}
	return pricing, err
}

func findWalletForUpdate(ctx context.Context, tx *sql.Tx, userID int64) (walletRecord, error) {
	var wallet walletRecord
	err := tx.QueryRowContext(ctx, `
		SELECT id, balance_coins, reserved_coins
		FROM wallets
		WHERE user_id = ?
		FOR UPDATE`,
		userID,
	).Scan(&wallet.ID, &wallet.BalanceCoins, &wallet.ReservedCoins)
	return wallet, err
}

func reservedCoinsForUsage(ctx context.Context, tx *sql.Tx, usageID int64) (int64, error) {
	var amount sql.NullInt64
	err := tx.QueryRowContext(ctx, `
		SELECT SUM(amount)
		FROM usage_reservations
		WHERE usage_request_id = ? AND reservation_type = 'coins' AND status = 'reserved'`,
		usageID,
	).Scan(&amount)
	if err != nil {
		return 0, err
	}
	if !amount.Valid {
		return 0, nil
	}
	return amount.Int64, nil
}

func usageToAuthorizeResponse(usage usageRecord) contracts.AuthorizeResponse {
	return contracts.AuthorizeResponse{
		UsageRequestID: fmt.Sprintf("%d", usage.ID),
		Status:         usage.Status,
		ReservedCoins:  usage.ReservedCoins,
		ReservedUnits:  usage.ReservedUnits,
	}
}

func usageToCommitResponse(usage usageRecord) contracts.CommitResponse {
	return contracts.CommitResponse{
		UsageRequestID: fmt.Sprintf("%d", usage.ID),
		Status:         usage.Status,
		ChargedCoins:   usage.ChargedCoins,
		ChargedUnits:   usage.ChargedUnits,
	}
}

func entitlementDemands(usage contracts.Usage) []usageUnitDemand {
	demands := make([]usageUnitDemand, 0, 4)
	if usage.TotalTokens > 0 {
		demands = append(demands, usageUnitDemand{Unit: "tokens", Amount: usage.TotalTokens})
	}
	if usage.Images > 0 {
		demands = append(demands, usageUnitDemand{Unit: "images", Amount: usage.Images})
	}
	if usage.VideoSeconds > 0 {
		demands = append(demands, usageUnitDemand{Unit: "video_seconds", Amount: usage.VideoSeconds})
	}
	if usage.BusinessUnits > 0 {
		demands = append(demands, usageUnitDemand{Unit: "business_units", Amount: usage.BusinessUnits})
	}
	return demands
}

func usageDemandMap(demands []usageUnitDemand) map[string]int64 {
	result := map[string]int64{
		"tokens":         0,
		"images":         0,
		"video_seconds":  0,
		"business_units": 0,
	}
	for _, demand := range demands {
		result[demand.Unit] += demand.Amount
	}
	return result
}

func priceUsageCoins(ctx context.Context, tx *sql.Tx, appID int64, model string, mode contracts.BillingMode, remainingByUnit map[string]int64) (int64, error) {
	if mode == contracts.BillingModeEntitlement {
		return 0, nil
	}
	var coins int64
	for unit, amount := range remainingByUnit {
		if amount <= 0 {
			continue
		}
		if unit == "tokens" {
			pricing, err := findPricing(ctx, tx, appID, model)
			if err != nil {
				return 0, err
			}
			coins += contracts.CeilDiv(amount*pricing.CoinAmount, pricing.TokenAmount)
			continue
		}
		pricing, err := findUnitPricing(ctx, tx, appID, model, unit)
		if err != nil {
			return 0, err
		}
		coins += contracts.CeilDiv(amount*pricing.CoinAmount, pricing.UnitAmount)
	}
	return coins, nil
}

func rejectRemainingEntitlementUsage(mode contracts.BillingMode, remainingByUnit map[string]int64) error {
	if mode != contracts.BillingModeEntitlement {
		return nil
	}
	for _, remaining := range remainingByUnit {
		if remaining <= 0 {
			continue
		}
		return ErrInsufficientBalance
	}
	return nil
}

func rollback(tx *sql.Tx) {
	_ = tx.Rollback()
}

func parseID(value string) (int64, error) {
	id, err := strconv.ParseInt(value, 10, 64)
	if err != nil || id <= 0 {
		return 0, fmt.Errorf("invalid id: %s", value)
	}
	return id, nil
}

func min64(a, b int64) int64 {
	if a < b {
		return a
	}
	return b
}

func max64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}
