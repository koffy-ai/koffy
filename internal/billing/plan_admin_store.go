package billing

import (
	"context"
	"database/sql"
	"time"
)

type PlanItem struct {
	ID           int64                 `json:"id"`
	AppCode      string                `json:"app_code"`
	PlanCode     string                `json:"plan_code"`
	Name         string                `json:"name"`
	Period       string                `json:"period"`
	PriceCents   int64                 `json:"price_cents"`
	Status       string                `json:"status"`
	Entitlements []PlanEntitlementItem `json:"entitlements,omitempty"`
	CreatedAt    time.Time             `json:"created_at"`
	UpdatedAt    time.Time             `json:"updated_at"`
}

type PlanEntitlementItem struct {
	ID              int64     `json:"id"`
	PlanCode        string    `json:"plan_code"`
	EntitlementCode string    `json:"entitlement_code"`
	Name            string    `json:"name"`
	Unit            string    `json:"unit"`
	MonthlyQuota    int64     `json:"monthly_quota"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

type PlanRequest struct {
	PlanCode   string `json:"plan_code"`
	Name       string `json:"name"`
	Period     string `json:"period"`
	PriceCents int64  `json:"price_cents"`
	Status     string `json:"status"`
}

type PlanEntitlementRequest struct {
	EntitlementCode string `json:"entitlement_code"`
	Name            string `json:"name"`
	Unit            string `json:"unit"`
	MonthlyQuota    int64  `json:"monthly_quota"`
}

type GrantSubscriptionRequest struct {
	UserID   string `json:"user_id"`
	AppCode  string `json:"app_code"`
	PlanCode string `json:"plan_code"`
	Months   int    `json:"months"`
}

type EntitlementMaintenanceResult struct {
	ExpiredSubscriptions int64 `json:"expired_subscriptions"`
	CreatedBalances      int64 `json:"created_balances"`
	UpdatedBalances      int64 `json:"updated_balances"`
}

func (s *Store) AdminListPlans(ctx context.Context, appCode string) ([]PlanItem, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT p.id, a.app_code, p.plan_code, p.name, p.period, p.price_cents,
			p.status, p.created_at, p.updated_at
		FROM plans p
		JOIN apps a ON a.id = p.app_id
		WHERE a.app_code = ?
		ORDER BY p.id DESC`,
		appCode,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]PlanItem, 0)
	for rows.Next() {
		var item PlanItem
		if err := rows.Scan(&item.ID, &item.AppCode, &item.PlanCode, &item.Name, &item.Period, &item.PriceCents, &item.Status, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for i := range items {
		entitlements, err := s.planEntitlements(ctx, items[i].ID)
		if err != nil {
			return nil, err
		}
		items[i].Entitlements = entitlements
	}
	return items, nil
}

func (s *Store) AdminUpsertPlan(ctx context.Context, actorUserID int64, appCode string, req PlanRequest) (PlanItem, error) {
	if req.Status == "" {
		req.Status = "active"
	}
	app, err := s.adminFindApp(ctx, appCode)
	if err != nil {
		return PlanItem{}, err
	}
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO plans (app_id, plan_code, name, period, price_cents, status)
		VALUES (?, ?, ?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE
			name = VALUES(name),
			period = VALUES(period),
			price_cents = VALUES(price_cents),
			status = VALUES(status)`,
		app.ID,
		req.PlanCode,
		req.Name,
		req.Period,
		req.PriceCents,
		req.Status,
	)
	if err != nil {
		return PlanItem{}, err
	}
	item, err := s.adminFindPlan(ctx, appCode, req.PlanCode)
	if err != nil {
		return PlanItem{}, err
	}
	_ = s.writeAuditLog(ctx, actorUserID, "admin.plan.upsert", "plan", appCode+"/"+req.PlanCode, req)
	return item, nil
}

func (s *Store) AdminUpsertPlanEntitlement(ctx context.Context, actorUserID int64, appCode, planCode string, req PlanEntitlementRequest) (PlanEntitlementItem, error) {
	plan, err := s.adminFindPlan(ctx, appCode, planCode)
	if err != nil {
		return PlanEntitlementItem{}, err
	}
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO plan_entitlements (plan_id, entitlement_code, name, unit, monthly_quota)
		VALUES (?, ?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE
			name = VALUES(name),
			unit = VALUES(unit),
			monthly_quota = VALUES(monthly_quota)`,
		plan.ID,
		req.EntitlementCode,
		req.Name,
		req.Unit,
		req.MonthlyQuota,
	)
	if err != nil {
		return PlanEntitlementItem{}, err
	}
	item, err := s.adminFindPlanEntitlement(ctx, plan.ID, req.EntitlementCode)
	if err != nil {
		return PlanEntitlementItem{}, err
	}
	_ = s.writeAuditLog(ctx, actorUserID, "admin.plan.entitlement.upsert", "plan", appCode+"/"+planCode, req)
	return item, nil
}

func (s *Store) AdminGrantSubscription(ctx context.Context, actorUserID int64, req GrantSubscriptionRequest) (SubscriptionItem, error) {
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return SubscriptionItem{}, err
	}
	defer rollback(tx)

	user, err := ensureUser(ctx, tx, req.UserID)
	if err != nil {
		return SubscriptionItem{}, err
	}
	app, err := findAppForUpdate(ctx, tx, req.AppCode)
	if err != nil {
		return SubscriptionItem{}, err
	}
	plan, err := findPlanForUpdate(ctx, tx, app.ID, req.PlanCode)
	if err != nil {
		return SubscriptionItem{}, err
	}

	months := req.Months
	if months <= 0 {
		months = 1
	}
	now := localNow()
	endsAt := now.AddDate(0, months, 0)
	var subscriptionID int64
	var currentEndsAt time.Time
	err = tx.QueryRowContext(ctx, `
		SELECT id, ends_at
		FROM user_subscriptions
		WHERE user_id = ?
			AND app_id = ?
			AND plan_id = ?
			AND status = 'active'
			AND ends_at > CURRENT_TIMESTAMP(3)
		ORDER BY ends_at DESC
		LIMIT 1
		FOR UPDATE`,
		user.ID,
		app.ID,
		plan.ID,
	).Scan(&subscriptionID, &currentEndsAt)
	if err == nil {
		if currentEndsAt.After(now) {
			endsAt = currentEndsAt.AddDate(0, months, 0)
		}
		if _, err := tx.ExecContext(ctx, `
			UPDATE user_subscriptions
			SET ends_at = ?
			WHERE id = ?`,
			endsAt,
			subscriptionID,
		); err != nil {
			return SubscriptionItem{}, err
		}
	} else if err == sql.ErrNoRows {
		result, err := tx.ExecContext(ctx, `
			INSERT INTO user_subscriptions (user_id, app_id, plan_id, status, starts_at, ends_at)
			VALUES (?, ?, ?, 'active', ?, ?)`,
			user.ID,
			app.ID,
			plan.ID,
			now,
			endsAt,
		)
		if err != nil {
			return SubscriptionItem{}, err
		}
		subscriptionID, err = result.LastInsertId()
		if err != nil {
			return SubscriptionItem{}, err
		}
	} else {
		return SubscriptionItem{}, err
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE user_subscriptions
		SET status = 'cancelled'
		WHERE user_id = ?
			AND app_id = ?
			AND plan_id = ?
			AND id <> ?
			AND status = 'active'
			AND ends_at > CURRENT_TIMESTAMP(3)`,
		user.ID,
		app.ID,
		plan.ID,
		subscriptionID,
	); err != nil {
		return SubscriptionItem{}, err
	}
	if _, _, err := ensureEntitlementBalances(ctx, tx, user.ID, app.ID, subscriptionID, plan.ID, now.Format("2006-01")); err != nil {
		return SubscriptionItem{}, err
	}
	if err := tx.Commit(); err != nil {
		return SubscriptionItem{}, err
	}
	_ = s.writeAuditLog(ctx, actorUserID, "admin.subscription.grant", "user", req.UserID, req)

	return SubscriptionItem{
		ID:       subscriptionID,
		AppCode:  req.AppCode,
		AppName:  app.AppCode,
		PlanCode: req.PlanCode,
		PlanName: plan.Name,
		Status:   "active",
		StartsAt: now,
		EndsAt:   endsAt,
	}, nil
}

type planRecord struct {
	ID     int64
	Name   string
	Period string
}

func findPlanForUpdate(ctx context.Context, tx *sql.Tx, appID int64, planCode string) (planRecord, error) {
	var plan planRecord
	err := tx.QueryRowContext(ctx, `
		SELECT id, name, period
		FROM plans
		WHERE app_id = ? AND plan_code = ? AND status = 'active'
		FOR UPDATE`,
		appID,
		planCode,
	).Scan(&plan.ID, &plan.Name, &plan.Period)
	return plan, err
}

func ensureEntitlementBalances(ctx context.Context, tx *sql.Tx, userID, appID, subscriptionID, planID int64, period string) (created int64, updated int64, err error) {
	rows, err := tx.QueryContext(ctx, `
		SELECT entitlement_code, name, unit, monthly_quota
		FROM plan_entitlements
		WHERE plan_id = ?
		ORDER BY id`,
		planID,
	)
	if err != nil {
		return 0, 0, err
	}
	defer rows.Close()

	type entitlementDef struct {
		code  string
		name  string
		unit  string
		quota int64
	}
	defs := make([]entitlementDef, 0)
	for rows.Next() {
		var item entitlementDef
		if err := rows.Scan(&item.code, &item.name, &item.unit, &item.quota); err != nil {
			return 0, 0, err
		}
		defs = append(defs, item)
	}
	if err := rows.Err(); err != nil {
		return 0, 0, err
	}
	if err := rows.Close(); err != nil {
		return 0, 0, err
	}

	for _, item := range defs {
		var balanceID int64
		err := tx.QueryRowContext(ctx, `
			SELECT id
			FROM entitlement_balances
			WHERE subscription_id = ? AND entitlement_code = ? AND period_month = ?
			FOR UPDATE`,
			subscriptionID,
			item.code,
			period,
		).Scan(&balanceID)
		if err != nil && err != sql.ErrNoRows {
			return 0, 0, err
		}
		if err == sql.ErrNoRows {
			result, err := tx.ExecContext(ctx, `
				INSERT INTO entitlement_balances (
					user_id, app_id, subscription_id, entitlement_code, unit, period_month, quota
				)
				VALUES (?, ?, ?, ?, ?, ?, ?)`,
				userID,
				appID,
				subscriptionID,
				item.code,
				item.unit,
				period,
				item.quota,
			)
			if err != nil {
				return 0, 0, err
			}
			balanceID, err = result.LastInsertId()
			if err != nil {
				return 0, 0, err
			}
			if _, err := tx.ExecContext(ctx, `
				INSERT INTO entitlement_ledger (
					entitlement_balance_id, user_id, app_id, direction, amount,
					used_after, reserved_after
				)
				VALUES (?, ?, ?, 'reset', ?, 0, 0)`,
				balanceID,
				userID,
				appID,
				item.quota,
			); err != nil {
				return 0, 0, err
			}
			created++
			continue
		}

		result, err := tx.ExecContext(ctx, `
			UPDATE entitlement_balances
			SET quota = ?, unit = ?
			WHERE id = ?`,
			item.quota,
			item.unit,
			balanceID,
		)
		if err != nil {
			return 0, 0, err
		}
		affected, err := result.RowsAffected()
		if err != nil {
			return 0, 0, err
		}
		if affected > 0 {
			updated++
		}
	}
	return created, updated, nil
}

func (s *Store) RunEntitlementMaintenance(ctx context.Context) (EntitlementMaintenanceResult, error) {
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return EntitlementMaintenanceResult{}, err
	}
	defer rollback(tx)

	result := EntitlementMaintenanceResult{}
	expired, err := tx.ExecContext(ctx, `
		UPDATE user_subscriptions
		SET status = 'expired'
		WHERE status = 'active'
			AND ends_at <= CURRENT_TIMESTAMP(3)`)
	if err != nil {
		return EntitlementMaintenanceResult{}, err
	}
	result.ExpiredSubscriptions, err = expired.RowsAffected()
	if err != nil {
		return EntitlementMaintenanceResult{}, err
	}

	rows, err := tx.QueryContext(ctx, `
		SELECT user_id, app_id, id, plan_id
		FROM user_subscriptions
		WHERE status = 'active'
			AND starts_at <= CURRENT_TIMESTAMP(3)
			AND ends_at > CURRENT_TIMESTAMP(3)
		FOR UPDATE`)
	if err != nil {
		return EntitlementMaintenanceResult{}, err
	}
	defer rows.Close()

	type activeSubscription struct {
		userID         int64
		appID          int64
		subscriptionID int64
		planID         int64
	}
	subscriptions := make([]activeSubscription, 0)
	for rows.Next() {
		var item activeSubscription
		if err := rows.Scan(&item.userID, &item.appID, &item.subscriptionID, &item.planID); err != nil {
			return EntitlementMaintenanceResult{}, err
		}
		subscriptions = append(subscriptions, item)
	}
	if err := rows.Err(); err != nil {
		return EntitlementMaintenanceResult{}, err
	}
	if err := rows.Close(); err != nil {
		return EntitlementMaintenanceResult{}, err
	}

	period := localNow().Format("2006-01")
	for _, item := range subscriptions {
		created, updated, err := ensureEntitlementBalances(ctx, tx, item.userID, item.appID, item.subscriptionID, item.planID, period)
		if err != nil {
			return EntitlementMaintenanceResult{}, err
		}
		result.CreatedBalances += created
		result.UpdatedBalances += updated
	}

	if err := tx.Commit(); err != nil {
		return EntitlementMaintenanceResult{}, err
	}
	return result, nil
}

func (s *Store) EnsureCurrentMonthEntitlements(ctx context.Context, userID, appID int64) error {
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return err
	}
	defer rollback(tx)
	if err := ensureCurrentMonthEntitlementsTx(ctx, tx, userID, appID); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) EnsureCurrentMonthEntitlementsForUser(ctx context.Context, userID int64) error {
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return err
	}
	defer rollback(tx)

	rows, err := tx.QueryContext(ctx, `
		SELECT DISTINCT app_id
		FROM user_subscriptions
		WHERE user_id = ?
			AND status = 'active'
			AND starts_at <= CURRENT_TIMESTAMP(3)
			AND ends_at > CURRENT_TIMESTAMP(3)
		FOR UPDATE`,
		userID,
	)
	if err != nil {
		return err
	}
	defer rows.Close()

	appIDs := make([]int64, 0)
	for rows.Next() {
		var appID int64
		if err := rows.Scan(&appID); err != nil {
			return err
		}
		appIDs = append(appIDs, appID)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if err := rows.Close(); err != nil {
		return err
	}
	for _, appID := range appIDs {
		if err := ensureCurrentMonthEntitlementsTx(ctx, tx, userID, appID); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func ensureCurrentMonthEntitlementsTx(ctx context.Context, tx *sql.Tx, userID, appID int64) error {
	period := localNow().Format("2006-01")
	rows, err := tx.QueryContext(ctx, `
		SELECT id, plan_id
		FROM user_subscriptions
		WHERE user_id = ?
			AND app_id = ?
			AND status = 'active'
			AND starts_at <= CURRENT_TIMESTAMP(3)
			AND ends_at > CURRENT_TIMESTAMP(3)
		FOR UPDATE`,
		userID,
		appID,
	)
	if err != nil {
		return err
	}
	defer rows.Close()

	type subscriptionPlan struct {
		subscriptionID int64
		planID         int64
	}
	items := make([]subscriptionPlan, 0)
	for rows.Next() {
		var item subscriptionPlan
		if err := rows.Scan(&item.subscriptionID, &item.planID); err != nil {
			return err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if err := rows.Close(); err != nil {
		return err
	}
	for _, item := range items {
		if _, _, err := ensureEntitlementBalances(ctx, tx, userID, appID, item.subscriptionID, item.planID, period); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) adminFindPlan(ctx context.Context, appCode, planCode string) (PlanItem, error) {
	var item PlanItem
	err := s.db.QueryRowContext(ctx, `
		SELECT p.id, a.app_code, p.plan_code, p.name, p.period, p.price_cents,
			p.status, p.created_at, p.updated_at
		FROM plans p
		JOIN apps a ON a.id = p.app_id
		WHERE a.app_code = ? AND p.plan_code = ?`,
		appCode,
		planCode,
	).Scan(&item.ID, &item.AppCode, &item.PlanCode, &item.Name, &item.Period, &item.PriceCents, &item.Status, &item.CreatedAt, &item.UpdatedAt)
	if err != nil {
		return PlanItem{}, err
	}
	item.Entitlements, err = s.planEntitlements(ctx, item.ID)
	return item, err
}

func (s *Store) planEntitlements(ctx context.Context, planID int64) ([]PlanEntitlementItem, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, entitlement_code, name, unit, monthly_quota, created_at, updated_at
		FROM plan_entitlements
		WHERE plan_id = ?
		ORDER BY id`,
		planID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]PlanEntitlementItem, 0)
	for rows.Next() {
		var item PlanEntitlementItem
		if err := rows.Scan(&item.ID, &item.EntitlementCode, &item.Name, &item.Unit, &item.MonthlyQuota, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Store) adminFindPlanEntitlement(ctx context.Context, planID int64, code string) (PlanEntitlementItem, error) {
	var item PlanEntitlementItem
	err := s.db.QueryRowContext(ctx, `
		SELECT e.id, p.plan_code, e.entitlement_code, e.name, e.unit, e.monthly_quota,
			e.created_at, e.updated_at
		FROM plan_entitlements e
		JOIN plans p ON p.id = e.plan_id
		WHERE e.plan_id = ? AND e.entitlement_code = ?`,
		planID,
		code,
	).Scan(&item.ID, &item.PlanCode, &item.EntitlementCode, &item.Name, &item.Unit, &item.MonthlyQuota, &item.CreatedAt, &item.UpdatedAt)
	return item, err
}

func localNow() time.Time {
	location, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		return time.Now()
	}
	return time.Now().In(location)
}
