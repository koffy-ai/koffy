package billing

import (
	"context"
	"net/http"
	"strings"

	"koffy/internal/httpx"

	"github.com/casdoor/casdoor-go-sdk/casdoorsdk"
)

func (s *Server) sendVerificationSMS(ctx context.Context, phone string, code string) error {
	_ = ctx
	receiver := smsReceiverPhone(phone)
	if strings.TrimSpace(s.cfg.RegistrationSMSProvider) != "" {
		return casdoorsdk.SendSmsByProvider(code, s.cfg.RegistrationSMSProvider, receiver)
	}
	content := "您的验证码是 " + code + "，10 分钟内有效。"
	return casdoorsdk.SendSms(content, receiver)
}

func writeSMSSendError(w http.ResponseWriter, err error) {
	status, code, message := smsErrorResponse(err)
	httpx.Error(w, status, code, message)
}

func smsReceiverPhone(phone string) string {
	normalized, _, ok := normalizeMainlandPhoneValue("+86", phone)
	if ok {
		return normalized
	}
	return strings.TrimSpace(phone)
}

func smsErrorResponse(err error) (int, string, string) {
	raw := ""
	if err != nil {
		raw = err.Error()
	}
	lower := strings.ToLower(raw)
	switch {
	case strings.Contains(raw, "PhoneNumberThirtySecondLimit") ||
		strings.Contains(lower, "thirtysecond") ||
		strings.Contains(lower, "30 second") ||
		strings.Contains(raw, "30秒"):
		return http.StatusTooManyRequests, "sms_too_frequent", "发送太频繁，请 30 秒后再试"
	case strings.Contains(raw, "PhoneNumberOneHourLimit") ||
		strings.Contains(lower, "onehour") ||
		strings.Contains(lower, "one hour") ||
		strings.Contains(raw, "1小时"):
		return http.StatusTooManyRequests, "sms_hour_limit", "该手机号 1 小时内验证码发送次数已达上限，请稍后再试"
	case strings.Contains(raw, "PhoneNumberDailyLimit") ||
		strings.Contains(lower, "dailylimit") ||
		strings.Contains(lower, "daily limit") ||
		strings.Contains(raw, "自然日") ||
		strings.Contains(raw, "日上限"):
		return http.StatusTooManyRequests, "sms_day_limit", "该手机号今日验证码发送次数已达上限，请明天再试"
	case strings.Contains(lower, "limit") ||
		strings.Contains(lower, "exceed") ||
		strings.Contains(raw, "超过") ||
		strings.Contains(raw, "频率"):
		return http.StatusTooManyRequests, "sms_rate_limited", "验证码发送次数已达上限，请稍后再试"
	case strings.Contains(lower, "invalid phone receivers"):
		return http.StatusBadGateway, "sms_invalid_receiver", "手机号格式不符合短信服务要求，请联系管理员"
	case strings.Contains(raw, "未找到提供商") ||
		strings.Contains(lower, "provider"):
		return http.StatusBadGateway, "sms_provider_not_found", "短信服务配置有误，请联系管理员"
	default:
		return http.StatusBadGateway, "sms_send_failed", "验证码发送失败，请稍后再试"
	}
}
