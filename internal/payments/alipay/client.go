package alipay

import (
	"context"
	"fmt"
	"math"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"koffy/internal/config"

	alipay "github.com/smartwalle/alipay/v3"
)

type Client struct {
	cfg  config.Config
	core *alipay.Client
}

type PrepayRequest struct {
	OrderNo     string
	AmountCents int64
	Description string
	Channel     string
}

type Notification struct {
	EventID       string
	EventType     string
	OrderNo       string
	TransactionID string
	TradeStatus   string
	AmountCents   int64
	SuccessTime   string
}

func NewClient(cfg config.Config) (*Client, error) {
	if cfg.AlipayAppID == "" || cfg.AlipayPrivateKeyPath == "" || cfg.AlipayNotifyURL == "" {
		return nil, fmt.Errorf("alipay config is incomplete")
	}
	privateKey, err := os.ReadFile(cfg.AlipayPrivateKeyPath)
	if err != nil {
		return nil, fmt.Errorf("load alipay private key: %w", err)
	}
	client, err := alipay.New(cfg.AlipayAppID, strings.TrimSpace(string(privateKey)), cfg.AlipayProduction)
	if err != nil {
		return nil, fmt.Errorf("new alipay client: %w", err)
	}
	if err := loadVerificationMaterial(client, cfg); err != nil {
		return nil, err
	}
	return &Client{cfg: cfg, core: client}, nil
}

func (c *Client) Prepay(req PrepayRequest) (map[string]any, error) {
	description := req.Description
	if description == "" {
		description = "点数充值"
	}
	switch req.Channel {
	case "", "alipay_page":
		return c.pagePay(req, description)
	case "alipay_wap":
		return c.wapPay(req, description)
	default:
		return nil, fmt.Errorf("unsupported alipay channel: %s", req.Channel)
	}
}

func (c *Client) pagePay(req PrepayRequest, description string) (map[string]any, error) {
	payURL, err := c.core.TradePagePay(alipay.TradePagePay{
		Trade: alipay.Trade{
			Subject:        description,
			OutTradeNo:     req.OrderNo,
			TotalAmount:    formatCNY(req.AmountCents),
			ProductCode:    "FAST_INSTANT_TRADE_PAY",
			NotifyURL:      c.cfg.AlipayNotifyURL,
			ReturnURL:      c.cfg.AlipayReturnURL,
			TimeoutExpress: "30m",
		},
		IntegrationType: "PCWEB",
	})
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"channel":    "alipay_page",
		"pay_url":    payURL.String(),
		"notify_url": c.cfg.AlipayNotifyURL,
		"return_url": c.cfg.AlipayReturnURL,
	}, nil
}

func (c *Client) wapPay(req PrepayRequest, description string) (map[string]any, error) {
	payURL, err := c.core.TradeWapPay(alipay.TradeWapPay{
		Trade: alipay.Trade{
			Subject:        description,
			OutTradeNo:     req.OrderNo,
			TotalAmount:    formatCNY(req.AmountCents),
			ProductCode:    "QUICK_WAP_WAY",
			NotifyURL:      c.cfg.AlipayNotifyURL,
			ReturnURL:      c.cfg.AlipayReturnURL,
			TimeoutExpress: "30m",
		},
		QuitURL: c.cfg.AlipayReturnURL,
	})
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"channel":    "alipay_wap",
		"pay_url":    payURL.String(),
		"notify_url": c.cfg.AlipayNotifyURL,
		"return_url": c.cfg.AlipayReturnURL,
	}, nil
}

func (c *Client) ParseNotification(ctx context.Context, r *http.Request) (Notification, error) {
	if err := r.ParseForm(); err != nil {
		return Notification{}, err
	}
	notification, err := c.core.DecodeNotification(ctx, r.Form)
	if err != nil {
		return Notification{}, err
	}
	amountCents, err := parseCNYToCents(notification.TotalAmount)
	if err != nil {
		return Notification{}, err
	}
	eventID := notification.NotifyId
	if eventID == "" {
		eventID = strings.Join([]string{notification.TradeNo, notification.OutTradeNo, string(notification.TradeStatus)}, ":")
	}
	return Notification{
		EventID:       eventID,
		EventType:     notification.NotifyType,
		OrderNo:       notification.OutTradeNo,
		TransactionID: notification.TradeNo,
		TradeStatus:   string(notification.TradeStatus),
		AmountCents:   amountCents,
		SuccessTime:   notification.GmtPayment,
	}, nil
}

func (c *Client) ACKNotification(w http.ResponseWriter) {
	c.core.ACKNotification(w)
}

func loadVerificationMaterial(client *alipay.Client, cfg config.Config) error {
	if cfg.AlipayAppCertPath != "" || cfg.AlipayCertPath != "" || cfg.AlipayRootCertPath != "" {
		if cfg.AlipayAppCertPath == "" || cfg.AlipayCertPath == "" || cfg.AlipayRootCertPath == "" {
			return fmt.Errorf("alipay certificate config is incomplete")
		}
		if err := client.LoadAppCertPublicKeyFromFile(cfg.AlipayAppCertPath); err != nil {
			return fmt.Errorf("load alipay app cert: %w", err)
		}
		if err := client.LoadAlipayCertPublicKeyFromFile(cfg.AlipayCertPath); err != nil {
			return fmt.Errorf("load alipay cert: %w", err)
		}
		if err := client.LoadAliPayRootCertFromFile(cfg.AlipayRootCertPath); err != nil {
			return fmt.Errorf("load alipay root cert: %w", err)
		}
		return nil
	}
	if cfg.AlipayPublicKey != "" {
		if err := client.LoadAliPayPublicKey(strings.TrimSpace(cfg.AlipayPublicKey)); err != nil {
			return fmt.Errorf("load alipay public key: %w", err)
		}
		return nil
	}
	if cfg.AlipayPublicKeyPath != "" {
		publicKey, err := os.ReadFile(cfg.AlipayPublicKeyPath)
		if err != nil {
			return fmt.Errorf("load alipay public key: %w", err)
		}
		if err := client.LoadAliPayPublicKey(strings.TrimSpace(string(publicKey))); err != nil {
			return fmt.Errorf("load alipay public key: %w", err)
		}
		return nil
	}
	return fmt.Errorf("alipay public key or certificate config is required")
}

func formatCNY(amountCents int64) string {
	return fmt.Sprintf("%d.%02d", amountCents/100, amountCents%100)
}

func parseCNYToCents(value string) (int64, error) {
	if strings.TrimSpace(value) == "" {
		return 0, nil
	}
	amount, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
	if err != nil {
		return 0, err
	}
	return int64(math.Round(amount * 100)), nil
}

func ParseAlipayTime(value string) (time.Time, bool) {
	if value == "" {
		return time.Time{}, false
	}
	if parsed, err := time.ParseInLocation("2006-01-02 15:04:05", value, alipayLocation()); err == nil {
		return parsed, true
	}
	if parsed, err := time.Parse(time.RFC3339, value); err == nil {
		return parsed, true
	}
	return time.Time{}, false
}

func alipayLocation() *time.Location {
	location, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		return time.FixedZone("CST", 8*60*60)
	}
	return location
}
