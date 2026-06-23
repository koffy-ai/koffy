package billing

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	"koffy/internal/config"
	"koffy/internal/contracts"
)

type CreateRechargeOrderRequest struct {
	AmountCents   int64  `json:"amount_cents"`
	Channel       string `json:"channel"`
	Description   string `json:"description"`
	OpenID        string `json:"openid"`
	WeChatPayCode string `json:"wechat_pay_code"`
}

type RechargeOrderResponse struct {
	OrderNo     string         `json:"order_no"`
	Provider    string         `json:"provider"`
	AmountCents int64          `json:"amount_cents"`
	Coins       int64          `json:"coins"`
	Status      string         `json:"status"`
	Payment     map[string]any `json:"payment"`
}

type RechargeOrderItem struct {
	ID              int64      `json:"id"`
	OrderNo         string     `json:"order_no"`
	UserID          string     `json:"user_id,omitempty"`
	Provider        string     `json:"provider"`
	AmountCents     int64      `json:"amount_cents"`
	Coins           int64      `json:"coins"`
	Status          string     `json:"status"`
	ProviderTradeNo string     `json:"provider_trade_no"`
	PaidAt          *time.Time `json:"paid_at,omitempty"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

type PaymentEventItem struct {
	ID              int64      `json:"id"`
	Provider        string     `json:"provider"`
	EventID         string     `json:"event_id"`
	EventType       string     `json:"event_type"`
	OrderNo         string     `json:"order_no"`
	ProviderTradeNo string     `json:"provider_trade_no"`
	ProcessedAt     *time.Time `json:"processed_at,omitempty"`
	CreatedAt       time.Time  `json:"created_at"`
}

type LocalWeChatNotifyRequest struct {
	EventID       string `json:"event_id"`
	EventType     string `json:"event_type"`
	OrderNo       string `json:"order_no"`
	TransactionID string `json:"transaction_id"`
	SuccessTime   string `json:"success_time"`
}

type WeChatPaymentSuccess struct {
	EventID       string
	EventType     string
	OrderNo       string
	TransactionID string
	AmountCents   int64
	SuccessTime   string
}

type PaymentNotifyResponse struct {
	Status        string `json:"status"`
	OrderNo       string `json:"order_no"`
	AlreadyPaid   bool   `json:"already_paid"`
	CreditedCoins int64  `json:"credited_coins"`
}

func (s *Store) CreateRechargeOrder(ctx context.Context, userID int64, req CreateRechargeOrderRequest, cfg config.Config) (RechargeOrderResponse, error) {
	orderNo, err := newRechargeOrderNo()
	if err != nil {
		return RechargeOrderResponse{}, err
	}
	coins := contracts.CeilDiv(req.AmountCents*cfg.CoinExchangeRateCNY, 100)
	if coins <= 0 {
		return RechargeOrderResponse{}, fmt.Errorf("calculated coins must be positive")
	}
	channel := req.Channel
	if channel == "" {
		channel = "wechat_native"
	}
	description := req.Description
	if description == "" {
		description = "compute coin recharge"
	}

	_, err = s.db.ExecContext(ctx, `
		INSERT INTO recharge_orders (order_no, user_id, provider, amount_cents, coins, status)
		VALUES (?, ?, 'wechat', ?, ?, 'pending')`,
		orderNo,
		userID,
		req.AmountCents,
		coins,
	)
	if err != nil {
		return RechargeOrderResponse{}, err
	}

	payment := map[string]any{
		"channel":     channel,
		"description": description,
		"notify_url":  cfg.WeChatPayNotifyURL,
		"mode":        "pending_wechat_prepay",
	}
	if cfg.AppEnv == "local" {
		payment["mode"] = "local_test"
		payment["test_notify_header"] = "X-WeChatPay-Test: true"
	}

	return RechargeOrderResponse{
		OrderNo:     orderNo,
		Provider:    "wechat",
		AmountCents: req.AmountCents,
		Coins:       coins,
		Status:      "pending",
		Payment:     payment,
	}, nil
}

func (s *Store) RechargeOrders(ctx context.Context, userID int64, limit int) ([]RechargeOrderItem, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, order_no, '' AS casdoor_user_id, provider, amount_cents, coins, status,
			provider_trade_no, paid_at, created_at, updated_at
		FROM recharge_orders
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
	return scanRechargeOrderItems(rows)
}

func (s *Store) ProcessLocalWeChatNotify(ctx context.Context, req LocalWeChatNotifyRequest) (PaymentNotifyResponse, error) {
	if req.EventType == "" {
		req.EventType = "TRANSACTION.SUCCESS"
	}
	return s.ProcessWeChatPaymentSuccess(ctx, WeChatPaymentSuccess{
		EventID:       req.EventID,
		EventType:     req.EventType,
		OrderNo:       req.OrderNo,
		TransactionID: req.TransactionID,
		SuccessTime:   req.SuccessTime,
	})
}

func scanRechargeOrderItems(rows *sql.Rows) ([]RechargeOrderItem, error) {
	items := make([]RechargeOrderItem, 0)
	for rows.Next() {
		var item RechargeOrderItem
		var paidAt sql.NullTime
		if err := rows.Scan(
			&item.ID,
			&item.OrderNo,
			&item.UserID,
			&item.Provider,
			&item.AmountCents,
			&item.Coins,
			&item.Status,
			&item.ProviderTradeNo,
			&paidAt,
			&item.CreatedAt,
			&item.UpdatedAt,
		); err != nil {
			return nil, err
		}
		item.PaidAt = nullTimePtr(paidAt)
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Store) ProcessWeChatPaymentSuccess(ctx context.Context, req WeChatPaymentSuccess) (PaymentNotifyResponse, error) {
	successTime := time.Now()
	if req.SuccessTime != "" {
		if parsed, err := time.Parse(time.RFC3339, req.SuccessTime); err == nil {
			successTime = parsed
		}
	}
	payload, err := json.Marshal(req)
	if err != nil {
		return PaymentNotifyResponse{}, err
	}

	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return PaymentNotifyResponse{}, err
	}
	defer rollback(tx)

	result, err := tx.ExecContext(ctx, `
		INSERT IGNORE INTO payment_events (
			provider, event_id, event_type, order_no, provider_trade_no, payload_json
		)
		VALUES ('wechat', ?, ?, ?, ?, ?)`,
		req.EventID,
		req.EventType,
		req.OrderNo,
		req.TransactionID,
		payload,
	)
	if err != nil {
		return PaymentNotifyResponse{}, err
	}
	inserted, err := result.RowsAffected()
	if err != nil {
		return PaymentNotifyResponse{}, err
	}
	if inserted == 0 {
		if err := tx.Commit(); err != nil {
			return PaymentNotifyResponse{}, err
		}
		return PaymentNotifyResponse{Status: "ok", OrderNo: req.OrderNo, AlreadyPaid: true}, nil
	}

	order, err := findRechargeOrderForUpdate(ctx, tx, req.OrderNo)
	if err != nil {
		return PaymentNotifyResponse{}, err
	}
	if req.AmountCents > 0 && req.AmountCents != order.AmountCents {
		return PaymentNotifyResponse{}, ErrRequestConflict
	}
	if order.Status == "paid" {
		if err := markPaymentEventProcessed(ctx, tx, req.EventID); err != nil {
			return PaymentNotifyResponse{}, err
		}
		if err := tx.Commit(); err != nil {
			return PaymentNotifyResponse{}, err
		}
		return PaymentNotifyResponse{Status: "ok", OrderNo: req.OrderNo, AlreadyPaid: true}, nil
	}
	if order.Status != "pending" {
		return PaymentNotifyResponse{}, ErrRequestConflict
	}

	wallet, err := findWalletForUpdate(ctx, tx, order.UserID)
	if err != nil {
		return PaymentNotifyResponse{}, err
	}
	balanceAfter := wallet.BalanceCoins + order.Coins
	if _, err := tx.ExecContext(ctx, `
		UPDATE wallets
		SET balance_coins = ?, version = version + 1
		WHERE id = ?`,
		balanceAfter,
		wallet.ID,
	); err != nil {
		return PaymentNotifyResponse{}, err
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE recharge_orders
		SET status = 'paid', provider_trade_no = ?, paid_at = ?
		WHERE id = ?`,
		req.TransactionID,
		successTime,
		order.ID,
	); err != nil {
		return PaymentNotifyResponse{}, err
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO wallet_ledger (
			user_id, wallet_id, direction, reason, amount_coins,
			balance_after, recharge_order_id, remark
		)
		VALUES (?, ?, 'credit', 'recharge', ?, ?, ?, ?)`,
		order.UserID,
		wallet.ID,
		order.Coins,
		balanceAfter,
		order.ID,
		"wechat recharge",
	); err != nil {
		return PaymentNotifyResponse{}, err
	}
	if err := markPaymentEventProcessed(ctx, tx, req.EventID); err != nil {
		return PaymentNotifyResponse{}, err
	}
	if err := tx.Commit(); err != nil {
		return PaymentNotifyResponse{}, err
	}
	return PaymentNotifyResponse{
		Status:        "ok",
		OrderNo:       req.OrderNo,
		CreditedCoins: order.Coins,
	}, nil
}

type rechargeOrderRecord struct {
	ID          int64
	UserID      int64
	AmountCents int64
	Coins       int64
	Status      string
}

func findRechargeOrderForUpdate(ctx context.Context, tx *sql.Tx, orderNo string) (rechargeOrderRecord, error) {
	var order rechargeOrderRecord
	err := tx.QueryRowContext(ctx, `
		SELECT id, user_id, amount_cents, coins, status
		FROM recharge_orders
		WHERE order_no = ?
		FOR UPDATE`,
		orderNo,
	).Scan(&order.ID, &order.UserID, &order.AmountCents, &order.Coins, &order.Status)
	if err == sql.ErrNoRows {
		return rechargeOrderRecord{}, ErrRechargeOrderNotFound
	}
	return order, err
}

func markPaymentEventProcessed(ctx context.Context, tx *sql.Tx, eventID string) error {
	_, err := tx.ExecContext(ctx, `
		UPDATE payment_events
		SET processed_at = CURRENT_TIMESTAMP(3)
		WHERE provider = 'wechat' AND event_id = ?`,
		eventID,
	)
	return err
}

func newRechargeOrderNo() (string, error) {
	var raw [8]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", err
	}
	return "kf" + time.Now().Format("20060102150405") + hex.EncodeToString(raw[:]), nil
}
