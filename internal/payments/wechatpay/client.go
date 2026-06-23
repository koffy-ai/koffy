package wechatpay

import (
	"context"
	"fmt"
	"net/http"

	"koffy/internal/config"

	"github.com/wechatpay-apiv3/wechatpay-go/core"
	"github.com/wechatpay-apiv3/wechatpay-go/core/auth/verifiers"
	"github.com/wechatpay-apiv3/wechatpay-go/core/downloader"
	"github.com/wechatpay-apiv3/wechatpay-go/core/notify"
	"github.com/wechatpay-apiv3/wechatpay-go/core/option"
	"github.com/wechatpay-apiv3/wechatpay-go/services/payments"
	"github.com/wechatpay-apiv3/wechatpay-go/services/payments/jsapi"
	"github.com/wechatpay-apiv3/wechatpay-go/services/payments/native"
	"github.com/wechatpay-apiv3/wechatpay-go/utils"
)

type Client struct {
	cfg     config.Config
	core    *core.Client
	handler *notify.Handler
}

type PrepayRequest struct {
	OrderNo     string
	AmountCents int64
	Description string
	Channel     string
	OpenID      string
}

type Notification struct {
	EventID       string
	EventType     string
	OrderNo       string
	TransactionID string
	TradeState    string
	AmountCents   int64
	SuccessTime   string
}

func NewClient(ctx context.Context, cfg config.Config) (*Client, error) {
	if cfg.WeChatPayMchID == "" || cfg.WeChatPayAppID == "" || cfg.WeChatPayAPIV3Key == "" ||
		cfg.WeChatPayMchCertSerialNo == "" || cfg.WeChatPayPrivateKeyPath == "" || cfg.WeChatPayNotifyURL == "" {
		return nil, fmt.Errorf("wechat pay config is incomplete")
	}
	if (cfg.WeChatPayPublicKeyID == "") != (cfg.WeChatPayPublicKeyPath == "") {
		return nil, fmt.Errorf("wechat pay public key config is incomplete")
	}

	privateKey, err := utils.LoadPrivateKeyWithPath(cfg.WeChatPayPrivateKeyPath)
	if err != nil {
		return nil, fmt.Errorf("load wechat pay private key: %w", err)
	}

	if cfg.WeChatPayPublicKeyID != "" {
		publicKey, err := utils.LoadPublicKeyWithPath(cfg.WeChatPayPublicKeyPath)
		if err != nil {
			return nil, fmt.Errorf("load wechat pay public key: %w", err)
		}
		client, err := core.NewClient(ctx, option.WithWechatPayPublicKeyAuthCipher(
			cfg.WeChatPayMchID,
			cfg.WeChatPayMchCertSerialNo,
			privateKey,
			cfg.WeChatPayPublicKeyID,
			publicKey,
		))
		if err != nil {
			return nil, fmt.Errorf("new wechat pay client: %w", err)
		}
		return &Client{
			cfg:  cfg,
			core: client,
			handler: notify.NewNotifyHandler(
				cfg.WeChatPayAPIV3Key,
				verifiers.NewSHA256WithRSAPubkeyVerifier(cfg.WeChatPayPublicKeyID, *publicKey),
			),
		}, nil
	}

	client, err := core.NewClient(ctx, option.WithWechatPayAutoAuthCipher(
		cfg.WeChatPayMchID,
		cfg.WeChatPayMchCertSerialNo,
		privateKey,
		cfg.WeChatPayAPIV3Key,
	))
	if err != nil {
		return nil, fmt.Errorf("new wechat pay client: %w", err)
	}
	certVisitor := downloader.MgrInstance().GetCertificateVisitor(cfg.WeChatPayMchID)
	handler, err := notify.NewRSANotifyHandler(
		cfg.WeChatPayAPIV3Key,
		verifiers.NewSHA256WithRSAVerifier(certVisitor),
	)
	if err != nil {
		return nil, fmt.Errorf("new wechat pay notify handler: %w", err)
	}

	return &Client{cfg: cfg, core: client, handler: handler}, nil
}

func (c *Client) Prepay(ctx context.Context, req PrepayRequest) (map[string]any, error) {
	description := req.Description
	if description == "" {
		description = "compute coin recharge"
	}

	switch req.Channel {
	case "wechat_jsapi":
		if req.OpenID == "" {
			return nil, fmt.Errorf("openid is required for wechat_jsapi")
		}
		svc := jsapi.JsapiApiService{Client: c.core}
		resp, _, err := svc.PrepayWithRequestPayment(ctx, jsapi.PrepayRequest{
			Appid:       core.String(c.cfg.WeChatPayAppID),
			Mchid:       core.String(c.cfg.WeChatPayMchID),
			Description: core.String(description),
			OutTradeNo:  core.String(req.OrderNo),
			NotifyUrl:   core.String(c.cfg.WeChatPayNotifyURL),
			Amount: &jsapi.Amount{
				Total:    core.Int64(req.AmountCents),
				Currency: core.String("CNY"),
			},
			Payer: &jsapi.Payer{Openid: core.String(req.OpenID)},
		})
		if err != nil {
			return nil, err
		}
		return map[string]any{
			"channel":    "wechat_jsapi",
			"prepay_id":  stringValue(resp.PrepayId),
			"appId":      stringValue(resp.Appid),
			"timeStamp":  stringValue(resp.TimeStamp),
			"nonceStr":   stringValue(resp.NonceStr),
			"package":    stringValue(resp.Package),
			"signType":   stringValue(resp.SignType),
			"paySign":    stringValue(resp.PaySign),
			"notify_url": c.cfg.WeChatPayNotifyURL,
		}, nil
	default:
		svc := native.NativeApiService{Client: c.core}
		resp, _, err := svc.Prepay(ctx, native.PrepayRequest{
			Appid:       core.String(c.cfg.WeChatPayAppID),
			Mchid:       core.String(c.cfg.WeChatPayMchID),
			Description: core.String(description),
			OutTradeNo:  core.String(req.OrderNo),
			NotifyUrl:   core.String(c.cfg.WeChatPayNotifyURL),
			Amount: &native.Amount{
				Total:    core.Int64(req.AmountCents),
				Currency: core.String("CNY"),
			},
		})
		if err != nil {
			return nil, err
		}
		return map[string]any{
			"channel":    "wechat_native",
			"code_url":   stringValue(resp.CodeUrl),
			"notify_url": c.cfg.WeChatPayNotifyURL,
		}, nil
	}
}

func (c *Client) ParseNotification(ctx context.Context, r *http.Request) (Notification, error) {
	transaction := new(payments.Transaction)
	notifyReq, err := c.handler.ParseNotifyRequest(ctx, r, transaction)
	if err != nil {
		return Notification{}, err
	}

	var amountCents int64
	if transaction.Amount != nil && transaction.Amount.Total != nil {
		amountCents = *transaction.Amount.Total
	}

	return Notification{
		EventID:       notifyReq.ID,
		EventType:     notifyReq.EventType,
		OrderNo:       stringValue(transaction.OutTradeNo),
		TransactionID: stringValue(transaction.TransactionId),
		TradeState:    stringValue(transaction.TradeState),
		AmountCents:   amountCents,
		SuccessTime:   stringValue(transaction.SuccessTime),
	}, nil
}

func stringValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}
