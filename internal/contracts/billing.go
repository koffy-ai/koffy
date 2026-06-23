package contracts

type BillingMode string

const (
	BillingModeEntitlement BillingMode = "entitlement"
	BillingModeCoins       BillingMode = "coins"
	BillingModeHybrid      BillingMode = "hybrid"
)

type AuthorizeRequest struct {
	AppID          string      `json:"app_id"`
	UserID         string      `json:"user_id"`
	IdempotencyKey string      `json:"idempotency_key"`
	BillingMode    BillingMode `json:"billing_mode"`
	Model          string      `json:"model,omitempty"`
	EstimatedUsage Usage       `json:"estimated_usage"`
}

type AuthorizeResponse struct {
	UsageRequestID string `json:"usage_request_id"`
	Status         string `json:"status"`
	ReservedCoins  int64  `json:"reserved_coins"`
	ReservedUnits  int64  `json:"reserved_units"`
}

type CommitRequest struct {
	UsageRequestID string `json:"usage_request_id"`
	ActualUsage    Usage  `json:"actual_usage"`
	Provider       string `json:"provider,omitempty"`
	Model          string `json:"model,omitempty"`
	ProviderJobID  string `json:"provider_job_id,omitempty"`
}

type CommitResponse struct {
	UsageRequestID string `json:"usage_request_id"`
	Status         string `json:"status"`
	ChargedCoins   int64  `json:"charged_coins"`
	ChargedUnits   int64  `json:"charged_units"`
}

type CancelRequest struct {
	UsageRequestID string `json:"usage_request_id"`
	Reason         string `json:"reason,omitempty"`
}

type ChargeRequest struct {
	AppID          string `json:"app_id"`
	UserID         string `json:"user_id"`
	IdempotencyKey string `json:"idempotency_key"`
	Model          string `json:"model,omitempty"`
	ActualUsage    Usage  `json:"actual_usage"`
}

type Usage struct {
	PromptTokens     int64 `json:"prompt_tokens,omitempty"`
	CompletionTokens int64 `json:"completion_tokens,omitempty"`
	TotalTokens      int64 `json:"total_tokens,omitempty"`
	ImageTokens      int64 `json:"image_tokens,omitempty"`
	Images           int64 `json:"images,omitempty"`
	AudioTokens      int64 `json:"audio_tokens,omitempty"`
	VideoSeconds     int64 `json:"video_seconds,omitempty"`
	BusinessUnits    int64 `json:"business_units,omitempty"`
}

func CeilDiv(numerator, denominator int64) int64 {
	if denominator <= 0 {
		return 0
	}
	if numerator <= 0 {
		return 0
	}
	return (numerator + denominator - 1) / denominator
}
