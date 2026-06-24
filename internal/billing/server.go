package billing

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"koffy/internal/auth"
	"koffy/internal/config"
	"koffy/internal/contracts"
	"koffy/internal/httpx"
	alipaypay "koffy/internal/payments/alipay"
	"koffy/internal/payments/wechatpay"
)

type Server struct {
	cfg      config.Config
	store    *Store
	auth     *auth.Authenticator
	wechatMu sync.Mutex
	wechat   *wechatpay.Client
	alipayMu sync.Mutex
	alipay   *alipaypay.Client
}

func NewServer(cfg config.Config, db *sql.DB) *Server {
	return &Server{
		cfg:   cfg,
		store: NewStore(db),
		auth:  auth.NewAuthenticator(cfg),
	}
}

func (s *Server) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /", s.console)
	mux.HandleFunc("GET /console", s.console)
	mux.HandleFunc("GET /auth/login", s.login)
	mux.HandleFunc("GET /auth/register", s.register)
	mux.HandleFunc("GET /auth/callback", s.authCallback)
	mux.HandleFunc("POST /auth/logout", s.logout)
	mux.HandleFunc("GET /api/v1/auth/config", s.authConfig)
	mux.HandleFunc("POST /api/v1/auth/login", s.loginAccount)
	mux.HandleFunc("POST /api/v1/auth/session-exchange", s.exchangeSession)
	mux.HandleFunc("POST /api/v1/auth/phone-code", s.sendPhoneCode)
	mux.HandleFunc("POST /api/v1/auth/register", s.registerAccount)
	mux.HandleFunc("POST /api/v1/auth/forgot-password/code", s.sendResetPasswordCode)
	mux.HandleFunc("POST /api/v1/auth/forgot-password/reset", s.resetPassword)
	mux.HandleFunc("GET /api/v1/auth/wechat/start", s.startWeChatAuth)
	mux.HandleFunc("GET /api/v1/auth/wechat/widget-config", s.wechatWidgetConfig)
	mux.HandleFunc("GET /api/v1/auth/wechat/callback", s.wechatCallback)
	mux.HandleFunc("GET /healthz", s.health)
	mux.HandleFunc("GET /readyz", s.ready)
	mux.HandleFunc("POST /api/v1/billing/authorize", s.authorize)
	mux.HandleFunc("POST /api/v1/billing/commit", s.commit)
	mux.HandleFunc("POST /api/v1/billing/cancel", s.cancel)
	mux.HandleFunc("POST /api/v1/billing/charge", s.charge)
	mux.HandleFunc("GET /api/v1/me", s.currentUser)
	mux.HandleFunc("PATCH /api/v1/me/profile", s.updateProfile)
	mux.HandleFunc("POST /api/v1/me/avatar", s.uploadAvatar)
	mux.HandleFunc("GET /api/v1/me/auth-bindings", s.authBindings)
	mux.HandleFunc("POST /api/v1/me/phone/code", s.sendBindPhoneCode)
	mux.HandleFunc("POST /api/v1/me/phone/bind", s.bindPhone)
	mux.HandleFunc("DELETE /api/v1/me/wechat-binding", s.unbindWeChat)
	mux.HandleFunc("POST /api/v1/me/password/code", s.sendChangePasswordCode)
	mux.HandleFunc("POST /api/v1/me/password/change", s.changePassword)
	mux.HandleFunc("GET /api/v1/me/wallet", s.walletSummary)
	mux.HandleFunc("GET /api/v1/me/wallet/ledger", s.walletLedger)
	mux.HandleFunc("GET /api/v1/me/recharge-orders", s.rechargeOrders)
	mux.HandleFunc("GET /api/v1/me/subscriptions", s.subscriptions)
	mux.HandleFunc("GET /api/v1/me/entitlements", s.entitlements)
	mux.HandleFunc("GET /api/v1/me/entitlement-ledger", s.entitlementLedger)
	mux.HandleFunc("GET /api/v1/me/usage-requests", s.usageRequests)
	mux.HandleFunc("GET /api/v1/users/avatar/{casdoor_user_id}", s.publicUserAvatar)
	mux.HandleFunc("GET /api/v1/branding/logo", s.brandingLogo)
	mux.HandleFunc("POST /api/v1/admin/branding/logo", s.adminUploadBrandingLogo)
	mux.HandleFunc("GET /api/v1/branding/favicon", s.brandingFavicon)
	mux.HandleFunc("POST /api/v1/admin/branding/favicon", s.adminUploadBrandingFavicon)
	mux.HandleFunc("GET /api/v1/admin/apps", s.adminListApps)
	mux.HandleFunc("POST /api/v1/admin/apps", s.adminCreateApp)
	mux.HandleFunc("POST /api/v1/admin/apps/{app_code}/api-keys", s.adminCreateAPIKey)
	mux.HandleFunc("GET /api/v1/admin/apps/{app_code}/pricing", s.adminListPricing)
	mux.HandleFunc("POST /api/v1/admin/apps/{app_code}/pricing", s.adminUpsertPricing)
	mux.HandleFunc("DELETE /api/v1/admin/pricing/token/{pricing_id}", s.adminDeleteTokenPricing)
	mux.HandleFunc("POST /api/v1/admin/apps/{app_code}/unit-pricing", s.adminUpsertUnitPricing)
	mux.HandleFunc("DELETE /api/v1/admin/pricing/unit/{pricing_id}", s.adminDeleteUnitPricing)
	mux.HandleFunc("GET /api/v1/admin/apps/{app_code}/plans", s.adminListPlans)
	mux.HandleFunc("POST /api/v1/admin/apps/{app_code}/plans", s.adminUpsertPlan)
	mux.HandleFunc("POST /api/v1/admin/apps/{app_code}/plans/{plan_code}/entitlements", s.adminUpsertPlanEntitlement)
	mux.HandleFunc("GET /api/v1/admin/ai/providers", s.adminListAIProviders)
	mux.HandleFunc("POST /api/v1/admin/ai/providers", s.adminUpsertAIProvider)
	mux.HandleFunc("GET /api/v1/admin/ai/models", s.adminListAIModels)
	mux.HandleFunc("POST /api/v1/admin/ai/models", s.adminUpsertAIModel)
	mux.HandleFunc("GET /api/v1/admin/apps/{app_code}/model-routes", s.adminListAppModelRoutes)
	mux.HandleFunc("POST /api/v1/admin/apps/{app_code}/model-routes", s.adminUpsertAppModelRoute)
	mux.HandleFunc("POST /api/v1/admin/entitlements/maintenance", s.adminRunEntitlementMaintenance)
	mux.HandleFunc("GET /api/v1/admin/metrics/summary", s.adminMetricsSummary)
	mux.HandleFunc("GET /api/v1/admin/usage-requests", s.adminUsageRequests)
	mux.HandleFunc("GET /api/v1/admin/recharge-orders", s.adminRechargeOrders)
	mux.HandleFunc("GET /api/v1/admin/payment-events", s.adminPaymentEvents)
	mux.HandleFunc("GET /api/v1/admin/users/search", s.adminSearchUsers)
	mux.HandleFunc("GET /api/v1/admin/users/{user_id}/asset", s.adminUserAsset)
	mux.HandleFunc("GET /api/v1/admin/users/{user_id}/wallet/ledger", s.adminUserWalletLedger)
	mux.HandleFunc("GET /api/v1/admin/users/{user_id}/subscriptions", s.adminUserSubscriptions)
	mux.HandleFunc("GET /api/v1/admin/users/{user_id}/entitlements", s.adminUserEntitlements)
	mux.HandleFunc("GET /api/v1/admin/users/{user_id}/entitlement-ledger", s.adminUserEntitlementLedger)
	mux.HandleFunc("GET /api/v1/admin/users/{user_id}/usage-requests", s.adminUserUsageRequests)
	mux.HandleFunc("GET /api/v1/admin/users/{user_id}/recharge-orders", s.adminUserRechargeOrders)
	mux.HandleFunc("POST /api/v1/admin/users/{user_id}/adjust-coins", s.adminAdjustCoins)
	mux.HandleFunc("POST /api/v1/admin/users/{user_id}/subscriptions", s.adminGrantSubscription)
	mux.HandleFunc("POST /api/v1/recharge/orders", s.createRechargeOrder)
	mux.HandleFunc("POST /api/v1/payments/wechat/notify", s.wechatPaymentNotify)
	mux.HandleFunc("POST /api/v1/payments/alipay/notify", s.alipayPaymentNotify)
	return mux
}

func (s *Server) health(w http.ResponseWriter, r *http.Request) {
	httpx.JSON(w, http.StatusOK, map[string]any{
		"service": "koffy-billing-api",
		"status":  "ok",
		"env":     s.cfg.AppEnv,
	})
}

func (s *Server) ready(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()

	dependencies := map[string]map[string]string{
		"mysql": {"status": "ok"},
	}
	statusCode := http.StatusOK
	status := "ready"

	if err := s.store.db.PingContext(ctx); err != nil {
		statusCode = http.StatusServiceUnavailable
		status = "not_ready"
		dependencies["mysql"] = map[string]string{
			"status": "error",
			"error":  err.Error(),
		}
	}

	httpx.JSON(w, statusCode, map[string]any{
		"service":      "koffy-billing-api",
		"status":       status,
		"env":          s.cfg.AppEnv,
		"dependencies": dependencies,
	})
}

func (s *Server) authorize(w http.ResponseWriter, r *http.Request) {
	if !s.authorizeInternal(w, r) {
		return
	}

	var req contracts.AuthorizeRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if req.AppID == "" || req.UserID == "" || req.IdempotencyKey == "" {
		httpx.Error(w, http.StatusBadRequest, "invalid_request", "app_id, user_id and idempotency_key are required")
		return
	}

	resp, err := s.store.Authorize(r.Context(), req)
	if err != nil {
		s.writeStoreError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, resp)
}

func (s *Server) commit(w http.ResponseWriter, r *http.Request) {
	if !s.authorizeInternal(w, r) {
		return
	}

	var req contracts.CommitRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if req.UsageRequestID == "" {
		httpx.Error(w, http.StatusBadRequest, "invalid_request", "usage_request_id is required")
		return
	}

	resp, err := s.store.Commit(r.Context(), req)
	if err != nil {
		s.writeStoreError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, resp)
}

func (s *Server) cancel(w http.ResponseWriter, r *http.Request) {
	if !s.authorizeInternal(w, r) {
		return
	}

	var req contracts.CancelRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if req.UsageRequestID == "" {
		httpx.Error(w, http.StatusBadRequest, "invalid_request", "usage_request_id is required")
		return
	}

	if err := s.store.Cancel(r.Context(), req); err != nil {
		s.writeStoreError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]string{"status": "cancelled"})
}

func (s *Server) charge(w http.ResponseWriter, r *http.Request) {
	if !s.authorizeInternal(w, r) {
		return
	}

	var req contracts.ChargeRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if req.AppID == "" || req.UserID == "" || req.IdempotencyKey == "" {
		httpx.Error(w, http.StatusBadRequest, "invalid_request", "app_id, user_id and idempotency_key are required")
		return
	}

	resp, err := s.store.Charge(r.Context(), req)
	if err != nil {
		s.writeStoreError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, resp)
}

func (s *Server) currentUser(w http.ResponseWriter, r *http.Request) {
	user, ok := s.currentUserProfile(w, r)
	if !ok {
		return
	}
	httpx.JSON(w, http.StatusOK, user)
}

func (s *Server) walletSummary(w http.ResponseWriter, r *http.Request) {
	user, ok := s.currentUserProfile(w, r)
	if !ok {
		return
	}
	wallet, err := s.store.WalletSummary(r.Context(), user.ID)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	httpx.JSON(w, http.StatusOK, wallet)
}

func (s *Server) walletLedger(w http.ResponseWriter, r *http.Request) {
	user, ok := s.currentUserProfile(w, r)
	if !ok {
		return
	}
	limit := 50
	if value := r.URL.Query().Get("limit"); value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil {
			httpx.Error(w, http.StatusBadRequest, "invalid_request", "limit must be an integer")
			return
		}
		limit = parsed
	}
	items, err := s.store.WalletLedger(r.Context(), user.ID, limit)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"items": items})
}

func (s *Server) rechargeOrders(w http.ResponseWriter, r *http.Request) {
	user, ok := s.currentUserProfile(w, r)
	if !ok {
		return
	}
	limit, ok := parseLimitQuery(w, r, 50, 100)
	if !ok {
		return
	}
	items, err := s.store.RechargeOrders(r.Context(), user.ID, limit)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"items": items})
}

func (s *Server) subscriptions(w http.ResponseWriter, r *http.Request) {
	user, ok := s.currentUserProfile(w, r)
	if !ok {
		return
	}
	items, err := s.store.Subscriptions(r.Context(), user.ID)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"items": items})
}

func (s *Server) entitlements(w http.ResponseWriter, r *http.Request) {
	user, ok := s.currentUserProfile(w, r)
	if !ok {
		return
	}
	items, err := s.store.Entitlements(r.Context(), user.ID)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"items": items})
}

func (s *Server) entitlementLedger(w http.ResponseWriter, r *http.Request) {
	user, ok := s.currentUserProfile(w, r)
	if !ok {
		return
	}
	limit, ok := parseLimitQuery(w, r, 50, 100)
	if !ok {
		return
	}
	items, err := s.store.EntitlementLedger(r.Context(), user.ID, limit)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"items": items})
}

func (s *Server) usageRequests(w http.ResponseWriter, r *http.Request) {
	user, ok := s.currentUserProfile(w, r)
	if !ok {
		return
	}
	limit, ok := parseLimitQuery(w, r, 50, 100)
	if !ok {
		return
	}
	items, err := s.store.UsageRequests(r.Context(), user.ID, limit)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"items": items})
}

func (s *Server) createRechargeOrder(w http.ResponseWriter, r *http.Request) {
	user, ok := s.currentUserProfile(w, r)
	if !ok {
		return
	}
	var req CreateRechargeOrderRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if req.AmountCents <= 0 {
		httpx.Error(w, http.StatusBadRequest, "invalid_request", "amount_cents must be positive")
		return
	}
	provider := paymentProviderForChannel(req.Channel)
	if provider == "" {
		httpx.Error(w, http.StatusBadRequest, "invalid_request", "unsupported payment channel")
		return
	}
	if provider == "wechat" && !s.cfg.WeChatPayEnabled {
		httpx.Error(w, http.StatusServiceUnavailable, "wechat_pay_disabled", "微信支付未启用")
		return
	}
	if provider == "alipay" && !s.cfg.AlipayPayEnabled {
		httpx.Error(w, http.StatusServiceUnavailable, "alipay_disabled", "支付宝支付未启用")
		return
	}
	if provider == "wechat" && s.cfg.AppEnv != "local" && req.Channel == "wechat_jsapi" {
		if req.WeChatPayCode == "" {
			httpx.Error(w, http.StatusBadRequest, "wechat_pay_openid_required", "需要先获取微信支付授权，请重新点击微信支付")
			return
		}
		openID, err := s.store.ConsumeWeChatPayOpenIDCode(r.Context(), user.ID, req.WeChatPayCode, time.Now())
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) || errors.Is(err, ErrVerificationExpired) || errors.Is(err, ErrRequestConflict) {
				httpx.Error(w, http.StatusBadRequest, "wechat_pay_openid_required", "需要先获取微信支付授权，请重新点击微信支付")
				return
			}
			httpx.Error(w, http.StatusInternalServerError, "internal_error", err.Error())
			return
		}
		req.OpenID = openID
	}
	resp, err := s.store.CreateRechargeOrder(r.Context(), user.ID, req, s.cfg)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	if s.cfg.AppEnv != "local" {
		switch provider {
		case "wechat":
			client, err := s.wechatClient(r.Context())
			if err != nil {
				httpx.Error(w, http.StatusInternalServerError, "wechat_pay_not_configured", err.Error())
				return
			}
			payment, err := client.Prepay(r.Context(), wechatpay.PrepayRequest{
				OrderNo:     resp.OrderNo,
				AmountCents: resp.AmountCents,
				Description: req.Description,
				Channel:     req.Channel,
				OpenID:      req.OpenID,
			})
			if err != nil {
				httpx.Error(w, http.StatusBadGateway, "wechat_pay_prepay_failed", err.Error())
				return
			}
			resp.Payment = payment
		case "alipay":
			client, err := s.alipayClient()
			if err != nil {
				httpx.Error(w, http.StatusInternalServerError, "alipay_not_configured", err.Error())
				return
			}
			payment, err := client.Prepay(alipaypay.PrepayRequest{
				OrderNo:     resp.OrderNo,
				AmountCents: resp.AmountCents,
				Description: req.Description,
				Channel:     req.Channel,
			})
			if err != nil {
				code, message := friendlyAlipayPrepayError(err)
				httpx.Error(w, http.StatusBadGateway, code, message)
				return
			}
			resp.Payment = payment
		}
	}
	httpx.JSON(w, http.StatusOK, resp)
}

func (s *Server) wechatPaymentNotify(w http.ResponseWriter, r *http.Request) {
	if s.cfg.AppEnv == "local" && r.Header.Get("X-WeChatPay-Test") == "true" {
		var req LocalWeChatNotifyRequest
		if !decodeJSON(w, r, &req) {
			return
		}
		if req.EventID == "" || req.OrderNo == "" || req.TransactionID == "" {
			httpx.Error(w, http.StatusBadRequest, "invalid_request", "event_id, order_no and transaction_id are required")
			return
		}
		resp, err := s.store.ProcessLocalWeChatNotify(r.Context(), req)
		if err != nil {
			s.writeStoreError(w, err)
			return
		}
		httpx.JSON(w, http.StatusOK, resp)
		return
	}
	if !s.cfg.WeChatPayEnabled {
		httpx.Error(w, http.StatusServiceUnavailable, "wechat_pay_disabled", "wechat pay is disabled")
		return
	}

	client, err := s.wechatClient(r.Context())
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "wechat_pay_not_configured", err.Error())
		return
	}
	notification, err := client.ParseNotification(r.Context(), r)
	if err != nil {
		httpx.Error(w, http.StatusUnauthorized, "invalid_wechat_pay_notify", err.Error())
		return
	}
	if notification.TradeState != "SUCCESS" {
		httpx.JSON(w, http.StatusOK, map[string]string{"code": "SUCCESS", "message": "ignored"})
		return
	}
	resp, err := s.store.ProcessWeChatPaymentSuccess(r.Context(), WeChatPaymentSuccess{
		EventID:       notification.EventID,
		EventType:     notification.EventType,
		OrderNo:       notification.OrderNo,
		TransactionID: notification.TransactionID,
		AmountCents:   notification.AmountCents,
		SuccessTime:   notification.SuccessTime,
	})
	if err != nil {
		s.writeStoreError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{
		"code":           "SUCCESS",
		"message":        "成功",
		"order_no":       resp.OrderNo,
		"already_paid":   resp.AlreadyPaid,
		"credited_coins": resp.CreditedCoins,
	})
}

func (s *Server) alipayPaymentNotify(w http.ResponseWriter, r *http.Request) {
	if s.cfg.AppEnv == "local" && r.Header.Get("X-Alipay-Test") == "true" {
		var req LocalAlipayNotifyRequest
		if !decodeJSON(w, r, &req) {
			return
		}
		if req.EventID == "" || req.OrderNo == "" || req.TransactionID == "" {
			httpx.Error(w, http.StatusBadRequest, "invalid_request", "event_id, order_no and transaction_id are required")
			return
		}
		resp, err := s.store.ProcessLocalAlipayNotify(r.Context(), req)
		if err != nil {
			s.writeStoreError(w, err)
			return
		}
		httpx.JSON(w, http.StatusOK, resp)
		return
	}
	if !s.cfg.AlipayPayEnabled {
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte("fail"))
		return
	}
	client, err := s.alipayClient()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("fail"))
		return
	}
	notification, err := client.ParseNotification(r.Context(), r)
	if err != nil {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte("fail"))
		return
	}
	if notification.TradeStatus != "TRADE_SUCCESS" && notification.TradeStatus != "TRADE_FINISHED" {
		client.ACKNotification(w)
		return
	}
	if _, err := s.store.ProcessAlipayPaymentSuccess(r.Context(), AlipayPaymentSuccess{
		EventID:       notification.EventID,
		EventType:     notification.EventType,
		OrderNo:       notification.OrderNo,
		TransactionID: notification.TransactionID,
		AmountCents:   notification.AmountCents,
		SuccessTime:   notification.SuccessTime,
	}); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("fail"))
		return
	}
	client.ACKNotification(w)
}

func (s *Server) adminListApps(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	items, err := s.store.AdminListApps(r.Context())
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"items": items})
}

func (s *Server) adminCreateApp(w http.ResponseWriter, r *http.Request) {
	admin, ok := s.requireAdmin(w, r)
	if !ok {
		return
	}
	var req CreateAppRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if req.AppCode == "" || req.Name == "" {
		httpx.Error(w, http.StatusBadRequest, "invalid_request", "app_code and name are required")
		return
	}
	app, err := s.store.AdminCreateApp(r.Context(), admin.ID, req)
	if err != nil {
		s.writeStoreError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, app)
}

func (s *Server) adminCreateAPIKey(w http.ResponseWriter, r *http.Request) {
	admin, ok := s.requireAdmin(w, r)
	if !ok {
		return
	}
	resp, err := s.store.AdminCreateAPIKey(r.Context(), admin.ID, r.PathValue("app_code"))
	if err != nil {
		s.writeStoreError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, resp)
}

func (s *Server) adminUpsertPricing(w http.ResponseWriter, r *http.Request) {
	admin, ok := s.requireAdmin(w, r)
	if !ok {
		return
	}
	var req UpsertPricingRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if err := validatePositivePricing(req); err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	if err := s.store.AdminUpsertPricing(r.Context(), admin.ID, r.PathValue("app_code"), req); err != nil {
		s.writeStoreError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) adminDeleteTokenPricing(w http.ResponseWriter, r *http.Request) {
	admin, ok := s.requireAdmin(w, r)
	if !ok {
		return
	}
	pricingID, err := parseID(r.PathValue("pricing_id"))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid_request", "pricing_id is invalid")
		return
	}
	if err := s.store.AdminDeleteTokenPricing(r.Context(), admin.ID, pricingID); err != nil {
		s.writeStoreError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) adminListPricing(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	tokenItems, unitItems, err := s.store.AdminListPricing(r.Context(), r.PathValue("app_code"))
	if err != nil {
		s.writeStoreError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{
		"token_pricing": tokenItems,
		"unit_pricing":  unitItems,
	})
}

func (s *Server) adminUpsertUnitPricing(w http.ResponseWriter, r *http.Request) {
	admin, ok := s.requireAdmin(w, r)
	if !ok {
		return
	}
	var req UpsertUnitPricingRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if err := validatePositiveUnitPricing(req); err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	if err := s.store.AdminUpsertUnitPricing(r.Context(), admin.ID, r.PathValue("app_code"), req); err != nil {
		s.writeStoreError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) adminDeleteUnitPricing(w http.ResponseWriter, r *http.Request) {
	admin, ok := s.requireAdmin(w, r)
	if !ok {
		return
	}
	pricingID, err := parseID(r.PathValue("pricing_id"))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid_request", "pricing_id is invalid")
		return
	}
	if err := s.store.AdminDeleteUnitPricing(r.Context(), admin.ID, pricingID); err != nil {
		s.writeStoreError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) adminListPlans(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	items, err := s.store.AdminListPlans(r.Context(), r.PathValue("app_code"))
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"items": items})
}

func (s *Server) adminUpsertPlan(w http.ResponseWriter, r *http.Request) {
	admin, ok := s.requireAdmin(w, r)
	if !ok {
		return
	}
	var req PlanRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if req.PlanCode == "" || req.Name == "" || req.Period == "" || req.PriceCents < 0 {
		httpx.Error(w, http.StatusBadRequest, "invalid_request", "plan_code, name, period and non-negative price_cents are required")
		return
	}
	item, err := s.store.AdminUpsertPlan(r.Context(), admin.ID, r.PathValue("app_code"), req)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	httpx.JSON(w, http.StatusOK, item)
}

func (s *Server) adminUpsertPlanEntitlement(w http.ResponseWriter, r *http.Request) {
	admin, ok := s.requireAdmin(w, r)
	if !ok {
		return
	}
	var req PlanEntitlementRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if req.EntitlementCode == "" || req.Name == "" || req.Unit == "" || req.MonthlyQuota < 0 {
		httpx.Error(w, http.StatusBadRequest, "invalid_request", "entitlement_code, name, unit and non-negative monthly_quota are required")
		return
	}
	item, err := s.store.AdminUpsertPlanEntitlement(r.Context(), admin.ID, r.PathValue("app_code"), r.PathValue("plan_code"), req)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	httpx.JSON(w, http.StatusOK, item)
}

func (s *Server) adminListAIProviders(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	items, err := s.store.AdminListAIProviders(r.Context())
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"items": items})
}

func (s *Server) adminUpsertAIProvider(w http.ResponseWriter, r *http.Request) {
	admin, ok := s.requireAdmin(w, r)
	if !ok {
		return
	}
	var req AIProviderRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if req.ProviderCode == "" || req.Name == "" {
		httpx.Error(w, http.StatusBadRequest, "invalid_request", "provider_code and name are required")
		return
	}
	item, err := s.store.AdminUpsertAIProvider(r.Context(), admin.ID, req)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	httpx.JSON(w, http.StatusOK, item)
}

func (s *Server) adminListAIModels(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	items, err := s.store.AdminListAIModels(r.Context())
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"items": items})
}

func (s *Server) adminUpsertAIModel(w http.ResponseWriter, r *http.Request) {
	admin, ok := s.requireAdmin(w, r)
	if !ok {
		return
	}
	var req AIModelRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if req.ProviderCode == "" || req.ModelAlias == "" || req.ProviderModel == "" || req.Capability == "" {
		httpx.Error(w, http.StatusBadRequest, "invalid_request", "provider_code, model_alias, provider_model and capability are required")
		return
	}
	item, err := s.store.AdminUpsertAIModel(r.Context(), admin.ID, req)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	httpx.JSON(w, http.StatusOK, item)
}

func (s *Server) adminListAppModelRoutes(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	items, err := s.store.AdminListAppModelRoutes(r.Context(), r.PathValue("app_code"))
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"items": items})
}

func (s *Server) adminUpsertAppModelRoute(w http.ResponseWriter, r *http.Request) {
	admin, ok := s.requireAdmin(w, r)
	if !ok {
		return
	}
	var req AppModelRouteRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if req.ModelAlias == "" {
		httpx.Error(w, http.StatusBadRequest, "invalid_request", "model_alias is required")
		return
	}
	item, err := s.store.AdminUpsertAppModelRoute(r.Context(), admin.ID, r.PathValue("app_code"), req)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	httpx.JSON(w, http.StatusOK, item)
}

func (s *Server) adminUserAsset(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	asset, err := s.store.AdminUserAsset(r.Context(), r.PathValue("user_id"))
	if err != nil {
		s.writeStoreError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, asset)
}

func (s *Server) adminSearchUsers(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	limit, ok := parseLimitQuery(w, r, 20, 50)
	if !ok {
		return
	}
	items, err := s.store.AdminSearchUsers(r.Context(), r.URL.Query().Get("q"), limit)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"items": items})
}

func (s *Server) adminUserWalletLedger(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	limit, ok := parseLimitQuery(w, r, 50, 100)
	if !ok {
		return
	}
	items, err := s.store.AdminUserWalletLedger(r.Context(), r.PathValue("user_id"), limit)
	if err != nil {
		s.writeStoreError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"items": items})
}

func (s *Server) adminUserSubscriptions(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	items, err := s.store.AdminUserSubscriptions(r.Context(), r.PathValue("user_id"))
	if err != nil {
		s.writeStoreError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"items": items})
}

func (s *Server) adminUserEntitlements(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	items, err := s.store.AdminUserEntitlements(r.Context(), r.PathValue("user_id"))
	if err != nil {
		s.writeStoreError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"items": items})
}

func (s *Server) adminUserEntitlementLedger(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	limit, ok := parseLimitQuery(w, r, 50, 100)
	if !ok {
		return
	}
	items, err := s.store.AdminUserEntitlementLedger(r.Context(), r.PathValue("user_id"), limit)
	if err != nil {
		s.writeStoreError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"items": items})
}

func (s *Server) adminUserUsageRequests(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	limit, ok := parseLimitQuery(w, r, 50, 100)
	if !ok {
		return
	}
	items, err := s.store.AdminUserUsageRequests(r.Context(), r.PathValue("user_id"), limit)
	if err != nil {
		s.writeStoreError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"items": items})
}

func (s *Server) adminUserRechargeOrders(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	limit, ok := parseLimitQuery(w, r, 50, 100)
	if !ok {
		return
	}
	items, err := s.store.AdminUserRechargeOrders(r.Context(), r.PathValue("user_id"), limit)
	if err != nil {
		s.writeStoreError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"items": items})
}

func (s *Server) adminAdjustCoins(w http.ResponseWriter, r *http.Request) {
	admin, ok := s.requireAdmin(w, r)
	if !ok {
		return
	}
	var req AdminAdjustCoinsRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	req.UserID = r.PathValue("user_id")
	if req.UserID == "" || req.AmountCoins == 0 || req.Remark == "" {
		httpx.Error(w, http.StatusBadRequest, "invalid_request", "user_id, non-zero amount_coins and remark are required")
		return
	}
	wallet, err := s.store.AdminAdjustCoins(r.Context(), admin.ID, req)
	if err != nil {
		s.writeStoreError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, wallet)
}

func (s *Server) adminGrantSubscription(w http.ResponseWriter, r *http.Request) {
	admin, ok := s.requireAdmin(w, r)
	if !ok {
		return
	}
	var req GrantSubscriptionRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	req.UserID = r.PathValue("user_id")
	if req.UserID == "" || req.AppCode == "" || req.PlanCode == "" {
		httpx.Error(w, http.StatusBadRequest, "invalid_request", "user_id, app_code and plan_code are required")
		return
	}
	item, err := s.store.AdminGrantSubscription(r.Context(), admin.ID, req)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	httpx.JSON(w, http.StatusOK, item)
}

func (s *Server) adminUsageRequests(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	limit, ok := parseLimitQuery(w, r, 100, 200)
	if !ok {
		return
	}
	items, err := s.store.AdminUsageRequests(r.Context(), AdminUsageRequestFilter{
		AppCode: r.URL.Query().Get("app_code"),
		UserID:  r.URL.Query().Get("user_id"),
		Limit:   limit,
	})
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"items": items})
}

func (s *Server) adminMetricsSummary(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	days := 7
	if value := r.URL.Query().Get("days"); value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil {
			httpx.Error(w, http.StatusBadRequest, "invalid_request", "days must be an integer")
			return
		}
		days = parsed
	}
	summary, err := s.store.AdminMetricsSummary(r.Context(), days)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	httpx.JSON(w, http.StatusOK, summary)
}

func (s *Server) adminRechargeOrders(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	limit, ok := parseLimitQuery(w, r, 100, 200)
	if !ok {
		return
	}
	items, err := s.store.AdminRechargeOrders(r.Context(), AdminRechargeOrderFilter{
		UserID: r.URL.Query().Get("user_id"),
		Status: r.URL.Query().Get("status"),
		Limit:  limit,
	})
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"items": items})
}

func (s *Server) adminPaymentEvents(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	limit, ok := parseLimitQuery(w, r, 100, 200)
	if !ok {
		return
	}
	items, err := s.store.AdminPaymentEvents(r.Context(), AdminPaymentEventFilter{
		OrderNo: r.URL.Query().Get("order_no"),
		Limit:   limit,
	})
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"items": items})
}

func (s *Server) currentUserProfile(w http.ResponseWriter, r *http.Request) (UserProfile, bool) {
	principal, err := s.auth.PrincipalFromRequest(r)
	if err != nil {
		writeAuthError(w, err)
		return UserProfile{}, false
	}
	user, err := s.store.SyncUser(r.Context(), principal)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "internal_error", err.Error())
		return UserProfile{}, false
	}
	return user, true
}

func (s *Server) requireAdmin(w http.ResponseWriter, r *http.Request) (UserProfile, bool) {
	user, ok := s.currentUserProfile(w, r)
	if !ok {
		return UserProfile{}, false
	}
	if !user.IsAdmin {
		httpx.Error(w, http.StatusForbidden, "forbidden", "admin permission is required")
		return UserProfile{}, false
	}
	return user, true
}

func (s *Server) authorizeInternal(w http.ResponseWriter, r *http.Request) bool {
	if s.cfg.BillingInternalAPIKey == "" {
		httpx.Error(w, http.StatusInternalServerError, "server_not_configured", "billing internal api key is empty")
		return false
	}
	if r.Header.Get("X-Internal-API-Key") != s.cfg.BillingInternalAPIKey {
		httpx.Error(w, http.StatusUnauthorized, "unauthorized", "invalid internal api key")
		return false
	}
	return true
}

func (s *Server) writeStoreError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrAppNotFound):
		httpx.Error(w, http.StatusNotFound, "app_not_found", "app is not active or does not exist")
	case errors.Is(err, ErrPricingNotFound):
		httpx.Error(w, http.StatusUnprocessableEntity, "pricing_not_found", "active pricing is required for the requested usage unit")
	case errors.Is(err, ErrInsufficientBalance):
		httpx.Error(w, http.StatusPaymentRequired, "insufficient_balance", "balance or entitlement is not enough")
	case errors.Is(err, ErrRequestConflict):
		httpx.Error(w, http.StatusConflict, "request_conflict", "usage request status does not allow this operation")
	case errors.Is(err, ErrUsageNotFound):
		httpx.Error(w, http.StatusNotFound, "usage_not_found", "usage request does not exist")
	case errors.Is(err, ErrRechargeOrderNotFound):
		httpx.Error(w, http.StatusNotFound, "recharge_order_not_found", "recharge order does not exist")
	case errors.Is(err, sql.ErrNoRows):
		httpx.Error(w, http.StatusNotFound, "user_not_found", "user does not exist")
	default:
		httpx.Error(w, http.StatusInternalServerError, "internal_error", err.Error())
	}
}

func (s *Server) wechatClient(ctx context.Context) (*wechatpay.Client, error) {
	s.wechatMu.Lock()
	defer s.wechatMu.Unlock()
	if s.wechat != nil {
		return s.wechat, nil
	}
	client, err := wechatpay.NewClient(ctx, s.cfg)
	if err != nil {
		return nil, err
	}
	s.wechat = client
	return client, nil
}

func (s *Server) alipayClient() (*alipaypay.Client, error) {
	s.alipayMu.Lock()
	defer s.alipayMu.Unlock()
	if s.alipay != nil {
		return s.alipay, nil
	}
	client, err := alipaypay.NewClient(s.cfg)
	if err != nil {
		return nil, err
	}
	s.alipay = client
	return client, nil
}

func paymentProviderForChannel(channel string) string {
	switch channel {
	case "", "wechat_native", "wechat_jsapi":
		return "wechat"
	case "alipay_page", "alipay_wap":
		return "alipay"
	default:
		return ""
	}
}

func friendlyAlipayPrepayError(err error) (string, string) {
	raw := err.Error()
	normalized := strings.ToLower(raw)
	switch {
	case strings.Contains(raw, "40003") || strings.Contains(raw, "应用未上线") || strings.Contains(normalized, "app not online"):
		return "alipay_app_not_online", "支付宝应用未上线，请等待应用审核通过并上线后再试"
	case strings.Contains(raw, "产品") && (strings.Contains(raw, "未开通") || strings.Contains(raw, "未签约")):
		return "alipay_product_not_enabled", "支付宝支付产品未开通或未签约，请在支付宝商家平台开通对应支付产品后再试"
	case strings.Contains(raw, "无权") || strings.Contains(raw, "无权限") || strings.Contains(normalized, "isv.insufficient-isv-permissions"):
		return "alipay_product_not_enabled", "当前支付宝应用没有调用该支付接口的权限，请确认已开通对应支付产品"
	case strings.Contains(raw, "签名") || strings.Contains(normalized, "sign"):
		return "alipay_sign_error", "支付宝支付签名校验失败，请检查应用私钥、支付宝公钥或证书配置"
	default:
		return "alipay_prepay_failed", raw
	}
}

func writeAuthError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, auth.ErrMissingToken):
		httpx.Error(w, http.StatusUnauthorized, "missing_token", "Authorization Bearer token is required")
	case errors.Is(err, auth.ErrInvalidToken):
		httpx.Error(w, http.StatusUnauthorized, "invalid_token", "Casdoor token is invalid")
	case errors.Is(err, auth.ErrEmptyUserClaim):
		httpx.Error(w, http.StatusUnauthorized, "invalid_token", "Casdoor token does not contain a user id")
	default:
		httpx.Error(w, http.StatusUnauthorized, "unauthorized", err.Error())
	}
}

func (s *Server) notImplemented(operation string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		httpx.Error(w, http.StatusNotImplemented, "not_implemented", operation+" is not implemented yet")
	}
}

func decodeJSON(w http.ResponseWriter, r *http.Request, dst any) bool {
	defer r.Body.Close()
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(dst); err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid_json", err.Error())
		return false
	}
	return true
}

func parseLimitQuery(w http.ResponseWriter, r *http.Request, fallback, maxValue int) (int, bool) {
	limit := fallback
	if value := r.URL.Query().Get("limit"); value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil {
			httpx.Error(w, http.StatusBadRequest, "invalid_request", "limit must be an integer")
			return 0, false
		}
		limit = parsed
	}
	if limit <= 0 || limit > maxValue {
		limit = fallback
	}
	return limit, true
}
