package aigateway

import (
	"bufio"
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"koffy/internal/auth"
	"koffy/internal/config"
	"koffy/internal/contracts"
	"koffy/internal/httpx"
)

type Server struct {
	cfg     config.Config
	store   *Store
	billing *BillingClient
	auth    *auth.Authenticator
	client  *http.Client
	limiter *fixedWindowLimiter
}

func NewServer(cfg config.Config, db *sql.DB) *Server {
	return &Server{
		cfg:     cfg,
		store:   NewStore(db),
		billing: NewBillingClient(cfg.BillingAPIURL, cfg.BillingInternalAPIKey),
		auth:    auth.NewAuthenticator(cfg),
		client:  &http.Client{Timeout: 10 * time.Minute},
		limiter: newFixedWindowLimiter(),
	}
}

func (s *Server) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", s.health)
	mux.HandleFunc("GET /readyz", s.ready)
	mux.HandleFunc("POST /v1/chat/completions", s.chatCompletions)
	mux.HandleFunc("POST /v1/images/generations", s.imageGenerations)
	mux.HandleFunc("POST /api/v1/ai/requests", s.notImplemented("ai_request_proxy"))
	return mux
}

func (s *Server) health(w http.ResponseWriter, r *http.Request) {
	httpx.JSON(w, http.StatusOK, map[string]any{
		"service":          "koffy-gateway",
		"status":           "ok",
		"env":              s.cfg.AppEnv,
		"litellm_base_url": s.cfg.LiteLLMBaseURL,
		"billing_url":      s.cfg.BillingAPIURL,
	})
}

func (s *Server) ready(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()

	dependencies := map[string]map[string]string{
		"mysql":       {"status": "ok"},
		"billing_api": {"status": "ok"},
		"litellm":     {"status": "ok"},
	}
	statusCode := http.StatusOK
	status := "ready"

	if err := s.store.db.PingContext(ctx); err != nil {
		statusCode = http.StatusServiceUnavailable
		status = "not_ready"
		dependencies["mysql"] = map[string]string{"status": "error", "error": err.Error()}
	}
	if err := s.checkHTTPDependency(ctx, strings.TrimRight(s.cfg.BillingAPIURL, "/")+"/healthz", ""); err != nil {
		statusCode = http.StatusServiceUnavailable
		status = "not_ready"
		dependencies["billing_api"] = map[string]string{"status": "error", "error": err.Error()}
	}
	if err := s.checkHTTPDependency(ctx, strings.TrimRight(s.cfg.LiteLLMBaseURL, "/")+"/health/liveliness", s.cfg.LiteLLMMasterKey); err != nil {
		statusCode = http.StatusServiceUnavailable
		status = "not_ready"
		dependencies["litellm"] = map[string]string{"status": "error", "error": err.Error()}
	}

	httpx.JSON(w, statusCode, map[string]any{
		"service":      "koffy-gateway",
		"status":       status,
		"env":          s.cfg.AppEnv,
		"dependencies": dependencies,
	})
}

func (s *Server) checkHTTPDependency(ctx context.Context, targetURL string, bearerToken string) error {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, targetURL, nil)
	if err != nil {
		return err
	}
	if bearerToken != "" {
		request.Header.Set("Authorization", "Bearer "+bearerToken)
	}

	response, err := s.client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()

	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return fmt.Errorf("GET %s returned status %d", targetURL, response.StatusCode)
	}
	return nil
}

func (s *Server) chatCompletions(w http.ResponseWriter, r *http.Request) {
	appKey := strings.TrimSpace(r.Header.Get("X-App-Key"))
	idempotencyKey := strings.TrimSpace(r.Header.Get("Idempotency-Key"))
	if appKey == "" || idempotencyKey == "" {
		httpx.Error(w, http.StatusBadRequest, "invalid_request", "X-App-Key and Idempotency-Key are required")
		return
	}

	principal, err := s.auth.PrincipalFromRequest(r)
	if err != nil {
		writeAuthError(w, err)
		return
	}

	app, err := s.store.ResolveAppKey(r.Context(), appKey)
	if err != nil {
		if errors.Is(err, ErrInvalidAppKey) {
			httpx.Error(w, http.StatusUnauthorized, "invalid_app_key", "application key is invalid")
			return
		}
		httpx.Error(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	if !s.enforceRateLimit(w, app, principal.ID) {
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid_body", err.Error())
		return
	}
	defer r.Body.Close()

	model, stream, err := inspectChatRequest(body)
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid_json", err.Error())
		return
	}
	if err := s.store.EnsureModelAllowed(r.Context(), app.ID, model); err != nil {
		if errors.Is(err, ErrModelNotAllowed) {
			httpx.Error(w, http.StatusForbidden, "model_not_allowed", "model is not enabled for this application")
			return
		}
		httpx.Error(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	if stream {
		s.streamChatCompletions(w, r, app, principal.ID, idempotencyKey, model, body)
		return
	}

	auth, err := s.billing.Authorize(r.Context(), contracts.AuthorizeRequest{
		AppID:          app.AppCode,
		UserID:         principal.ID,
		IdempotencyKey: idempotencyKey,
		BillingMode:    contracts.BillingMode(app.BillingMode),
		Model:          model,
		EstimatedUsage: contracts.Usage{TotalTokens: s.cfg.DefaultPreauthTokens},
	})
	if err != nil {
		httpx.Error(w, http.StatusPaymentRequired, "billing_authorize_failed", err.Error())
		return
	}
	if auth.Status != "authorized" {
		httpx.Error(w, http.StatusConflict, "billing_request_conflict", "idempotency key has already been used with status "+auth.Status)
		return
	}

	upstreamBody, statusCode, contentType, err := s.callLiteLLM(r, body)
	if err != nil {
		ctx, cancel := billingFinalizerContext()
		defer cancel()
		_ = s.billing.Cancel(ctx, contracts.CancelRequest{
			UsageRequestID: auth.UsageRequestID,
			Reason:         "litellm_request_failed: " + err.Error(),
		})
		httpx.Error(w, http.StatusBadGateway, "model_request_failed", err.Error())
		return
	}
	if statusCode < 200 || statusCode >= 300 {
		ctx, cancel := billingFinalizerContext()
		defer cancel()
		_ = s.billing.Cancel(ctx, contracts.CancelRequest{
			UsageRequestID: auth.UsageRequestID,
			Reason:         fmt.Sprintf("litellm_status_%d", statusCode),
		})
		writeRaw(w, statusCode, contentType, upstreamBody)
		return
	}

	usage, providerModel, providerJobID, err := parseUsage(upstreamBody)
	if err != nil {
		ctx, cancel := billingFinalizerContext()
		defer cancel()
		_ = s.billing.Cancel(ctx, contracts.CancelRequest{
			UsageRequestID: auth.UsageRequestID,
			Reason:         "usage_missing: " + err.Error(),
		})
		httpx.Error(w, http.StatusBadGateway, "usage_missing", "model response does not contain billable usage")
		return
	}

	ctx, cancel := billingFinalizerContext()
	defer cancel()
	commit, err := s.billing.Commit(ctx, contracts.CommitRequest{
		UsageRequestID: auth.UsageRequestID,
		Provider:       "litellm",
		Model:          providerModel,
		ProviderJobID:  providerJobID,
		ActualUsage:    usage,
	})
	if err != nil {
		httpx.Error(w, http.StatusBadGateway, "billing_commit_failed", err.Error())
		return
	}

	w.Header().Set("X-Billing-Usage-Request-ID", auth.UsageRequestID)
	w.Header().Set("X-Billing-Charged-Coins", fmt.Sprintf("%d", commit.ChargedCoins))
	writeRaw(w, statusCode, contentType, upstreamBody)
}

func (s *Server) imageGenerations(w http.ResponseWriter, r *http.Request) {
	appKey := strings.TrimSpace(r.Header.Get("X-App-Key"))
	idempotencyKey := strings.TrimSpace(r.Header.Get("Idempotency-Key"))
	if appKey == "" || idempotencyKey == "" {
		httpx.Error(w, http.StatusBadRequest, "invalid_request", "X-App-Key and Idempotency-Key are required")
		return
	}

	principal, err := s.auth.PrincipalFromRequest(r)
	if err != nil {
		writeAuthError(w, err)
		return
	}

	app, err := s.store.ResolveAppKey(r.Context(), appKey)
	if err != nil {
		if errors.Is(err, ErrInvalidAppKey) {
			httpx.Error(w, http.StatusUnauthorized, "invalid_app_key", "application key is invalid")
			return
		}
		httpx.Error(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	if !s.enforceRateLimit(w, app, principal.ID) {
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid_body", err.Error())
		return
	}
	defer r.Body.Close()

	model, estimatedImages, err := inspectImageRequest(body)
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid_json", err.Error())
		return
	}
	if err := s.store.EnsureModelAllowed(r.Context(), app.ID, model); err != nil {
		if errors.Is(err, ErrModelNotAllowed) {
			httpx.Error(w, http.StatusForbidden, "model_not_allowed", "model is not enabled for this application")
			return
		}
		httpx.Error(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}

	auth, err := s.billing.Authorize(r.Context(), contracts.AuthorizeRequest{
		AppID:          app.AppCode,
		UserID:         principal.ID,
		IdempotencyKey: idempotencyKey,
		BillingMode:    contracts.BillingMode(app.BillingMode),
		Model:          model,
		EstimatedUsage: contracts.Usage{Images: estimatedImages},
	})
	if err != nil {
		httpx.Error(w, http.StatusPaymentRequired, "billing_authorize_failed", err.Error())
		return
	}
	if auth.Status != "authorized" {
		httpx.Error(w, http.StatusConflict, "billing_request_conflict", "idempotency key has already been used with status "+auth.Status)
		return
	}

	upstreamBody, statusCode, contentType, err := s.callLiteLLMPath(r, "/v1/images/generations", body)
	if err != nil {
		ctx, cancel := billingFinalizerContext()
		defer cancel()
		_ = s.billing.Cancel(ctx, contracts.CancelRequest{
			UsageRequestID: auth.UsageRequestID,
			Reason:         "litellm_image_request_failed: " + err.Error(),
		})
		httpx.Error(w, http.StatusBadGateway, "model_request_failed", err.Error())
		return
	}
	if statusCode < 200 || statusCode >= 300 {
		ctx, cancel := billingFinalizerContext()
		defer cancel()
		_ = s.billing.Cancel(ctx, contracts.CancelRequest{
			UsageRequestID: auth.UsageRequestID,
			Reason:         fmt.Sprintf("litellm_image_status_%d", statusCode),
		})
		writeRaw(w, statusCode, contentType, upstreamBody)
		return
	}

	usage, providerModel, providerJobID, err := parseImageUsage(upstreamBody, model)
	if err != nil {
		ctx, cancel := billingFinalizerContext()
		defer cancel()
		_ = s.billing.Cancel(ctx, contracts.CancelRequest{
			UsageRequestID: auth.UsageRequestID,
			Reason:         "image_usage_missing: " + err.Error(),
		})
		httpx.Error(w, http.StatusBadGateway, "usage_missing", "model response does not contain generated images")
		return
	}

	ctx, cancel := billingFinalizerContext()
	defer cancel()
	commit, err := s.billing.Commit(ctx, contracts.CommitRequest{
		UsageRequestID: auth.UsageRequestID,
		Provider:       "litellm",
		Model:          providerModel,
		ProviderJobID:  providerJobID,
		ActualUsage:    usage,
	})
	if err != nil {
		httpx.Error(w, http.StatusBadGateway, "billing_commit_failed", err.Error())
		return
	}

	w.Header().Set("X-Billing-Usage-Request-ID", auth.UsageRequestID)
	w.Header().Set("X-Billing-Charged-Coins", fmt.Sprintf("%d", commit.ChargedCoins))
	writeRaw(w, statusCode, contentType, upstreamBody)
}

func (s *Server) streamChatCompletions(w http.ResponseWriter, r *http.Request, app AppIdentity, userID, idempotencyKey, model string, body []byte) {
	streamBody, err := ensureStreamUsage(body)
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid_json", err.Error())
		return
	}

	auth, err := s.billing.Authorize(r.Context(), contracts.AuthorizeRequest{
		AppID:          app.AppCode,
		UserID:         userID,
		IdempotencyKey: idempotencyKey,
		BillingMode:    contracts.BillingMode(app.BillingMode),
		Model:          model,
		EstimatedUsage: contracts.Usage{TotalTokens: s.cfg.DefaultPreauthTokens},
	})
	if err != nil {
		httpx.Error(w, http.StatusPaymentRequired, "billing_authorize_failed", err.Error())
		return
	}
	if auth.Status != "authorized" {
		httpx.Error(w, http.StatusConflict, "billing_request_conflict", "idempotency key has already been used with status "+auth.Status)
		return
	}

	response, err := s.openLiteLLMStream(r, streamBody)
	if err != nil {
		ctx, cancel := billingFinalizerContext()
		defer cancel()
		_ = s.billing.Cancel(ctx, contracts.CancelRequest{
			UsageRequestID: auth.UsageRequestID,
			Reason:         "litellm_stream_failed: " + err.Error(),
		})
		httpx.Error(w, http.StatusBadGateway, "model_request_failed", err.Error())
		return
	}
	defer response.Body.Close()

	if response.StatusCode < 200 || response.StatusCode >= 300 {
		responseBody, _ := io.ReadAll(response.Body)
		ctx, cancel := billingFinalizerContext()
		defer cancel()
		_ = s.billing.Cancel(ctx, contracts.CancelRequest{
			UsageRequestID: auth.UsageRequestID,
			Reason:         fmt.Sprintf("litellm_status_%d", response.StatusCode),
		})
		writeRaw(w, response.StatusCode, response.Header.Get("Content-Type"), responseBody)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("X-Accel-Buffering", "no")
	w.Header().Set("Trailer", "X-Billing-Usage-Request-ID, X-Billing-Charged-Coins, X-Billing-Status")
	w.WriteHeader(http.StatusOK)

	usage, providerModel, providerJobID, copyErr := copySSEAndExtractUsage(w, response.Body)
	if copyErr != nil {
		ctx, cancel := billingFinalizerContext()
		defer cancel()
		_ = s.billing.Cancel(ctx, contracts.CancelRequest{
			UsageRequestID: auth.UsageRequestID,
			Reason:         "stream_copy_failed: " + copyErr.Error(),
		})
		w.Header().Set("X-Billing-Usage-Request-ID", auth.UsageRequestID)
		w.Header().Set("X-Billing-Status", "cancelled")
		return
	}
	if usage.TotalTokens <= 0 {
		ctx, cancel := billingFinalizerContext()
		defer cancel()
		_ = s.billing.Cancel(ctx, contracts.CancelRequest{
			UsageRequestID: auth.UsageRequestID,
			Reason:         "stream_usage_missing",
		})
		w.Header().Set("X-Billing-Usage-Request-ID", auth.UsageRequestID)
		w.Header().Set("X-Billing-Status", "cancelled")
		return
	}

	ctx, cancel := billingFinalizerContext()
	defer cancel()
	commit, err := s.billing.Commit(ctx, contracts.CommitRequest{
		UsageRequestID: auth.UsageRequestID,
		Provider:       "litellm",
		Model:          providerModel,
		ProviderJobID:  providerJobID,
		ActualUsage:    usage,
	})
	if err != nil {
		w.Header().Set("X-Billing-Usage-Request-ID", auth.UsageRequestID)
		w.Header().Set("X-Billing-Status", "commit_failed")
		return
	}

	w.Header().Set("X-Billing-Usage-Request-ID", auth.UsageRequestID)
	w.Header().Set("X-Billing-Charged-Coins", fmt.Sprintf("%d", commit.ChargedCoins))
	w.Header().Set("X-Billing-Status", commit.Status)
}

func (s *Server) callLiteLLM(r *http.Request, body []byte) ([]byte, int, string, error) {
	return s.callLiteLLMPath(r, "/v1/chat/completions", body)
}

func (s *Server) callLiteLLMPath(r *http.Request, path string, body []byte) ([]byte, int, string, error) {
	response, err := s.openLiteLLMStreamPath(r, path, body)
	if err != nil {
		return nil, 0, "", err
	}
	defer response.Body.Close()

	responseBody, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, 0, "", err
	}

	return responseBody, response.StatusCode, response.Header.Get("Content-Type"), nil
}

func (s *Server) openLiteLLMStream(r *http.Request, body []byte) (*http.Response, error) {
	return s.openLiteLLMStreamPath(r, "/v1/chat/completions", body)
}

func (s *Server) openLiteLLMStreamPath(r *http.Request, path string, body []byte) (*http.Response, error) {
	target := strings.TrimRight(s.cfg.LiteLLMBaseURL, "/") + path
	request, err := http.NewRequestWithContext(r.Context(), http.MethodPost, target, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	request.Header.Set("Content-Type", "application/json")
	if s.cfg.LiteLLMMasterKey != "" {
		request.Header.Set("Authorization", "Bearer "+s.cfg.LiteLLMMasterKey)
	}

	return s.client.Do(request)
}

func inspectChatRequest(body []byte) (string, bool, error) {
	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		return "", false, err
	}

	model, ok := payload["model"].(string)
	if !ok || strings.TrimSpace(model) == "" {
		return "", false, fmt.Errorf("model is required")
	}

	stream, _ := payload["stream"].(bool)
	return model, stream, nil
}

func inspectImageRequest(body []byte) (string, int64, error) {
	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		return "", 0, err
	}

	model, ok := payload["model"].(string)
	if !ok || strings.TrimSpace(model) == "" {
		return "", 0, fmt.Errorf("model is required")
	}

	images := int64(1)
	if value, ok := payload["n"]; ok {
		parsed, err := jsonNumberToInt64(value)
		if err != nil || parsed <= 0 {
			return "", 0, fmt.Errorf("n must be a positive integer")
		}
		images = parsed
	}
	return model, images, nil
}

func parseUsage(body []byte) (contracts.Usage, string, string, error) {
	var payload struct {
		ID    string `json:"id"`
		Model string `json:"model"`
		Usage struct {
			PromptTokens     int64 `json:"prompt_tokens"`
			CompletionTokens int64 `json:"completion_tokens"`
			TotalTokens      int64 `json:"total_tokens"`
		} `json:"usage"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return contracts.Usage{}, "", "", err
	}
	if payload.Usage.TotalTokens <= 0 {
		return contracts.Usage{}, "", "", fmt.Errorf("usage.total_tokens is missing")
	}

	return contracts.Usage{
		PromptTokens:     payload.Usage.PromptTokens,
		CompletionTokens: payload.Usage.CompletionTokens,
		TotalTokens:      payload.Usage.TotalTokens,
	}, payload.Model, payload.ID, nil
}

func parseImageUsage(body []byte, fallbackModel string) (contracts.Usage, string, string, error) {
	var payload struct {
		ID    string            `json:"id"`
		Model string            `json:"model"`
		Data  []json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return contracts.Usage{}, "", "", err
	}

	images := int64(len(payload.Data))
	if images <= 0 {
		return contracts.Usage{}, "", "", fmt.Errorf("response data is empty")
	}
	if payload.Model == "" {
		payload.Model = fallbackModel
	}

	return contracts.Usage{Images: images}, payload.Model, payload.ID, nil
}

func ensureStreamUsage(body []byte) ([]byte, error) {
	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, err
	}
	payload["stream"] = true

	options, ok := payload["stream_options"].(map[string]any)
	if !ok {
		options = map[string]any{}
	}
	options["include_usage"] = true
	payload["stream_options"] = options

	return json.Marshal(payload)
}

func jsonNumberToInt64(value any) (int64, error) {
	switch item := value.(type) {
	case float64:
		result := int64(item)
		if float64(result) != item {
			return 0, fmt.Errorf("number must be an integer")
		}
		return result, nil
	case json.Number:
		return item.Int64()
	default:
		return 0, fmt.Errorf("number must be an integer")
	}
}

func copySSEAndExtractUsage(w http.ResponseWriter, body io.Reader) (contracts.Usage, string, string, error) {
	var usage contracts.Usage
	var providerModel string
	var providerJobID string
	flusher, _ := w.(http.Flusher)
	scanner := bufio.NewScanner(body)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	for scanner.Scan() {
		line := scanner.Bytes()
		if _, err := w.Write(line); err != nil {
			return contracts.Usage{}, "", "", err
		}
		if _, err := w.Write([]byte("\n")); err != nil {
			return contracts.Usage{}, "", "", err
		}
		if flusher != nil {
			flusher.Flush()
		}

		chunkUsage, chunkModel, chunkID, ok := parseSSEUsageLine(line)
		if ok {
			usage = chunkUsage
			providerModel = chunkModel
			providerJobID = chunkID
		}
	}
	if err := scanner.Err(); err != nil {
		return contracts.Usage{}, "", "", err
	}
	return usage, providerModel, providerJobID, nil
}

func parseSSEUsageLine(line []byte) (contracts.Usage, string, string, bool) {
	text := strings.TrimSpace(string(line))
	if !strings.HasPrefix(text, "data:") {
		return contracts.Usage{}, "", "", false
	}
	data := strings.TrimSpace(strings.TrimPrefix(text, "data:"))
	if data == "" || data == "[DONE]" {
		return contracts.Usage{}, "", "", false
	}

	usage, model, jobID, err := parseUsage([]byte(data))
	if err != nil {
		return contracts.Usage{}, "", "", false
	}
	return usage, model, jobID, true
}

func (s *Server) enforceRateLimit(w http.ResponseWriter, app AppIdentity, userID string) bool {
	if ok, retry := s.limiter.Allow("app:"+app.AppCode, s.cfg.AIGatewayAppRPM); !ok {
		writeRateLimitError(w, retry, "app_rate_limited", "application rate limit exceeded")
		return false
	}
	if ok, retry := s.limiter.Allow("app_user:"+app.AppCode+":"+userID, s.cfg.AIGatewayUserRPM); !ok {
		writeRateLimitError(w, retry, "user_rate_limited", "user rate limit exceeded")
		return false
	}
	return true
}

func writeRateLimitError(w http.ResponseWriter, retryAfter time.Duration, code, message string) {
	seconds := int(retryAfter.Seconds())
	if seconds < 1 {
		seconds = 1
	}
	w.Header().Set("Retry-After", fmt.Sprintf("%d", seconds))
	httpx.Error(w, http.StatusTooManyRequests, code, message)
}

func writeRaw(w http.ResponseWriter, status int, contentType string, body []byte) {
	if contentType == "" {
		contentType = "application/json; charset=utf-8"
	}
	w.Header().Set("Content-Type", contentType)
	w.WriteHeader(status)
	_, _ = w.Write(body)
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

func billingFinalizerContext() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), 15*time.Second)
}

func (s *Server) notImplemented(operation string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		httpx.Error(w, http.StatusNotImplemented, "not_implemented", operation+" is not implemented yet")
	}
}
