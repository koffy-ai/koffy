package billing

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"koffy/internal/httpx"

	captcha "github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/captcha/v20190722"
	"github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common"
	"github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common/profile"
)

type captchaConfigResponse struct {
	Provider string `json:"provider"`
	SiteKey  string `json:"site_key"`
	Enabled  bool   `json:"enabled"`
}

type tencentCaptchaToken struct {
	Ticket       string `json:"ticket"`
	Randstr      string `json:"randstr"`
	CaptchaAppID string `json:"captcha_app_id"`
}

func (s *Server) authConfig(w http.ResponseWriter, r *http.Request) {
	provider := strings.ToLower(strings.TrimSpace(s.cfg.CaptchaProvider))
	httpx.JSON(w, http.StatusOK, captchaConfigResponse{
		Provider: provider,
		SiteKey:  s.captchaSiteKey(provider),
		Enabled:  s.captchaEnabled(),
	})
}

func (s *Server) verifyHumanToken(r *http.Request, token string) error {
	provider := strings.ToLower(strings.TrimSpace(s.cfg.CaptchaProvider))
	if !s.cfg.CaptchaEnabled {
		return nil
	}
	if provider == "" || provider == "none" {
		if s.cfg.AppEnv == "production" {
			return fmt.Errorf("人机验证未配置")
		}
		return nil
	}
	token = strings.TrimSpace(token)
	if token == "" {
		return fmt.Errorf("请先完成人机验证")
	}
	if s.cfg.AppEnv == "local" && token == "dev-ok" {
		return nil
	}
	if provider == "tencent" {
		return s.verifyTencentCaptchaToken(r, token)
	}
	if strings.TrimSpace(s.cfg.CaptchaSecret) == "" {
		return fmt.Errorf("人机验证密钥未配置")
	}

	verifyURL := strings.TrimSpace(s.cfg.CaptchaVerifyURL)
	if verifyURL == "" {
		switch provider {
		case "turnstile", "cloudflare":
			verifyURL = "https://challenges.cloudflare.com/turnstile/v0/siteverify"
		case "hcaptcha":
			verifyURL = "https://hcaptcha.com/siteverify"
		default:
			return fmt.Errorf("不支持的人机验证服务")
		}
	}

	values := url.Values{}
	values.Set("secret", s.cfg.CaptchaSecret)
	values.Set("response", token)
	if ip := clientIP(r); ip != "" {
		values.Set("remoteip", ip)
	}
	req, err := http.NewRequestWithContext(r.Context(), http.MethodPost, verifyURL, strings.NewReader(values.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	var parsed struct {
		Success bool `json:"success"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return err
	}
	if !parsed.Success {
		return fmt.Errorf("人机验证失败，请重试")
	}
	return nil
}

func (s *Server) captchaEnabled() bool {
	provider := strings.ToLower(strings.TrimSpace(s.cfg.CaptchaProvider))
	return s.cfg.CaptchaEnabled && provider != "" && provider != "none"
}

func (s *Server) captchaSiteKey(provider string) string {
	if provider == "tencent" {
		return strings.TrimSpace(s.cfg.TencentCaptchaAppID)
	}
	return strings.TrimSpace(s.cfg.CaptchaSiteKey)
}

func (s *Server) verifyTencentCaptchaToken(r *http.Request, token string) error {
	if strings.TrimSpace(s.cfg.TencentCloudSecretID) == "" || strings.TrimSpace(s.cfg.TencentCloudSecretKey) == "" ||
		strings.TrimSpace(s.cfg.TencentCaptchaAppID) == "" || strings.TrimSpace(s.cfg.TencentCaptchaAppSecretKey) == "" {
		return fmt.Errorf("腾讯云验证码未配置完整")
	}
	var parsedToken tencentCaptchaToken
	if err := json.Unmarshal([]byte(token), &parsedToken); err != nil {
		return fmt.Errorf("腾讯云验证码参数无效")
	}
	parsedToken.Ticket = strings.TrimSpace(parsedToken.Ticket)
	parsedToken.Randstr = strings.TrimSpace(parsedToken.Randstr)
	parsedToken.CaptchaAppID = strings.TrimSpace(parsedToken.CaptchaAppID)
	if parsedToken.Ticket == "" || parsedToken.Randstr == "" {
		return fmt.Errorf("请先完成腾讯云验证码")
	}
	if parsedToken.CaptchaAppID != "" && parsedToken.CaptchaAppID != s.cfg.TencentCaptchaAppID {
		return fmt.Errorf("腾讯云验证码应用不匹配")
	}
	appID, err := strconv.ParseUint(s.cfg.TencentCaptchaAppID, 10, 64)
	if err != nil || appID == 0 {
		return fmt.Errorf("腾讯云验证码 CaptchaAppId 无效")
	}

	credential := common.NewCredential(s.cfg.TencentCloudSecretID, s.cfg.TencentCloudSecretKey)
	clientProfile := profile.NewClientProfile()
	client, err := captcha.NewClient(credential, "", clientProfile)
	if err != nil {
		return err
	}
	req := captcha.NewDescribeCaptchaResultRequest()
	captchaType := uint64(9)
	needGetCaptchaTime := int64(1)
	appSecretKey := strings.TrimSpace(s.cfg.TencentCaptchaAppSecretKey)
	req.CaptchaType = &captchaType
	req.CaptchaAppId = &appID
	req.AppSecretKey = &appSecretKey
	req.Ticket = &parsedToken.Ticket
	req.Randstr = &parsedToken.Randstr
	userIP := clientIP(r)
	req.UserIp = &userIP
	req.NeedGetCaptchaTime = &needGetCaptchaTime

	resp, err := client.DescribeCaptchaResultWithContext(r.Context(), req)
	if err != nil {
		return fmt.Errorf("腾讯云验证码校验失败：%w", err)
	}
	if resp == nil || resp.Response == nil || resp.Response.CaptchaCode == nil {
		return fmt.Errorf("腾讯云验证码返回为空")
	}
	code := *resp.Response.CaptchaCode
	msg := ""
	if resp.Response.CaptchaMsg != nil {
		msg = *resp.Response.CaptchaMsg
	}
	if code == 1 {
		return nil
	}
	if msg == "" {
		msg = fmt.Sprintf("CaptchaCode=%d", code)
	}
	return fmt.Errorf("腾讯云验证码未通过：%s", humanCaptchaMessage(msg))
}

func humanCaptchaMessage(message string) string {
	lower := strings.ToLower(strings.TrimSpace(message))
	switch {
	case strings.Contains(lower, "ticket reused") ||
		strings.Contains(lower, "reused") ||
		strings.Contains(lower, "expired") ||
		strings.Contains(message, "过期"):
		return "安全验证已失效，请重新完成滑块验证"
	case strings.Contains(lower, "limit") || strings.Contains(message, "频率"):
		return "安全验证请求过于频繁，请稍后再试"
	default:
		return "安全验证未通过，请重新完成滑块验证"
	}
}

func clientIP(r *http.Request) string {
	if forwarded := strings.TrimSpace(r.Header.Get("X-Forwarded-For")); forwarded != "" {
		parts := strings.Split(forwarded, ",")
		return strings.TrimSpace(parts[0])
	}
	host := r.RemoteAddr
	if index := strings.LastIndex(host, ":"); index > 0 {
		return host[:index]
	}
	return host
}
