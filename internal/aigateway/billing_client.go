package aigateway

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"koffy/internal/contracts"
)

type BillingClient struct {
	baseURL     string
	internalKey string
	httpClient  *http.Client
}

func NewBillingClient(baseURL, internalKey string) *BillingClient {
	return &BillingClient{
		baseURL:     strings.TrimRight(baseURL, "/"),
		internalKey: internalKey,
		httpClient:  &http.Client{Timeout: 30 * time.Second},
	}
}

func (c *BillingClient) Authorize(ctx context.Context, req contracts.AuthorizeRequest) (contracts.AuthorizeResponse, error) {
	var resp contracts.AuthorizeResponse
	err := c.post(ctx, "/api/v1/billing/authorize", req, &resp)
	return resp, err
}

func (c *BillingClient) Commit(ctx context.Context, req contracts.CommitRequest) (contracts.CommitResponse, error) {
	var resp contracts.CommitResponse
	err := c.post(ctx, "/api/v1/billing/commit", req, &resp)
	return resp, err
}

func (c *BillingClient) Cancel(ctx context.Context, req contracts.CancelRequest) error {
	var resp map[string]string
	return c.post(ctx, "/api/v1/billing/cancel", req, &resp)
}

func (c *BillingClient) post(ctx context.Context, path string, payload any, dst any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, bytes.NewReader(body))
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-Internal-API-Key", c.internalKey)

	response, err := c.httpClient.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()

	if response.StatusCode < 200 || response.StatusCode >= 300 {
		var errBody struct {
			Error struct {
				Code    string `json:"code"`
				Message string `json:"message"`
			} `json:"error"`
		}
		_ = json.NewDecoder(response.Body).Decode(&errBody)
		if errBody.Error.Code == "" {
			return fmt.Errorf("billing api returned status %d", response.StatusCode)
		}
		return fmt.Errorf("%s: %s", errBody.Error.Code, errBody.Error.Message)
	}

	return json.NewDecoder(response.Body).Decode(dst)
}
