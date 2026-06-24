package billing

import (
	"errors"
	"testing"
	"time"
)

func TestNewRechargeOrderNoWechatPayCompatible(t *testing.T) {
	orderNo, err := newRechargeOrderNo()
	if err != nil {
		t.Fatalf("newRechargeOrderNo() error = %v", err)
	}
	if len(orderNo) > 32 {
		t.Fatalf("order number length = %d, want <= 32: %s", len(orderNo), orderNo)
	}
	for _, char := range orderNo {
		if char >= '0' && char <= '9' {
			continue
		}
		if char >= 'a' && char <= 'z' {
			continue
		}
		if char >= 'A' && char <= 'Z' {
			continue
		}
		t.Fatalf("order number contains unsupported character %q: %s", char, orderNo)
	}
}

func TestPaymentProviderForChannel(t *testing.T) {
	cases := map[string]string{
		"":              "wechat",
		"wechat_native": "wechat",
		"wechat_jsapi":  "wechat",
		"alipay_page":   "alipay",
		"alipay_wap":    "alipay",
		"bank_card":     "",
	}
	for channel, want := range cases {
		if got := paymentProviderForChannel(channel); got != want {
			t.Fatalf("paymentProviderForChannel(%q) = %q, want %q", channel, got, want)
		}
	}
}

func TestFriendlyAlipayPrepayError(t *testing.T) {
	code, message := friendlyAlipayPrepayError(errors.New("40003 - 应用未上线"))
	if code != "alipay_app_not_online" {
		t.Fatalf("code = %q, want alipay_app_not_online", code)
	}
	if message == "40003 - 应用未上线" || message == "" {
		t.Fatalf("message was not localized: %q", message)
	}
}

func TestParsePaymentSuccessTimeUsesAlipayLocalTime(t *testing.T) {
	parsed, ok := parsePaymentSuccessTime("2026-06-24 20:52:00")
	if !ok {
		t.Fatal("parsePaymentSuccessTime() did not parse Alipay local time")
	}
	want := time.Date(2026, 6, 24, 20, 52, 0, 0, paymentNotifyLocation())
	if !parsed.Equal(want) {
		t.Fatalf("parsed = %s, want %s", parsed, want)
	}
	if parsed.UTC().Hour() != 12 {
		t.Fatalf("parsed was not interpreted as Asia/Shanghai time: %s", parsed.UTC())
	}
}
