package billing

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"regexp"
	"strings"
	"time"

	"koffy/internal/httpx"

	"github.com/casdoor/casdoor-go-sdk/casdoorsdk"
)

type sendPhoneCodeRequest struct {
	CountryCode string `json:"country_code"`
	Phone       string `json:"phone"`
	HumanToken  string `json:"human_token"`
}

type sendPhoneCodeResponse struct {
	Status    string `json:"status"`
	DebugCode string `json:"debug_code,omitempty"`
}

type registerAccountRequest struct {
	CountryCode     string `json:"country_code"`
	Phone           string `json:"phone"`
	Code            string `json:"code"`
	Password        string `json:"password"`
	ConfirmPassword string `json:"confirm_password"`
}

var mainlandPhonePattern = regexp.MustCompile(`^1[3-9]\d{9}$`)
var regexpPhoneInput = regexp.MustCompile(`^[+\d\s-]+$`)

const casdoorMainlandCountryCode = "CN"

func (s *Server) sendPhoneCode(w http.ResponseWriter, r *http.Request) {
	var req sendPhoneCodeRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	phone, ok := normalizeMainlandPhone(w, req.CountryCode, req.Phone)
	if !ok {
		return
	}
	if exists, err := s.phoneAlreadyRegistered(r.Context(), phone); err != nil {
		httpx.Error(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	} else if exists {
		httpx.Error(w, http.StatusConflict, "phone_already_registered", "该手机号已注册")
		return
	}
	if conflicts, err := s.phoneConflictsWithUsername(r.Context(), phone); err != nil {
		httpx.Error(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	} else if conflicts {
		httpx.Error(w, http.StatusConflict, "phone_conflicts_with_username", "该手机号无法作为登录手机号，请联系管理员")
		return
	}
	if err := s.verifyHumanToken(r, req.HumanToken); err != nil {
		httpx.Error(w, http.StatusBadRequest, "human_verification_failed", err.Error())
		return
	}

	code, err := numericCode(6)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	if _, err := s.store.CreatePhoneVerificationCode(r.Context(), phone, phonePurposeRegister, code, s.cfg.BillingInternalAPIKey, time.Now()); err != nil {
		switch {
		case errors.Is(err, ErrVerificationTooSoon):
			httpx.Error(w, http.StatusTooManyRequests, "verification_too_soon", "请稍后再获取验证码")
		default:
			httpx.Error(w, http.StatusInternalServerError, "internal_error", err.Error())
		}
		return
	}

	resp := sendPhoneCodeResponse{Status: "ok"}
	if s.cfg.AppEnv == "local" {
		resp.DebugCode = code
		httpx.JSON(w, http.StatusOK, resp)
		return
	}

	if err := s.sendVerificationSMS(r.Context(), phone, code); err != nil {
		writeSMSSendError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, resp)
}

func (s *Server) registerAccount(w http.ResponseWriter, r *http.Request) {
	var req registerAccountRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	phone, ok := normalizeMainlandPhone(w, req.CountryCode, req.Phone)
	if !ok {
		return
	}
	if strings.TrimSpace(req.Code) == "" {
		httpx.Error(w, http.StatusBadRequest, "invalid_request", "验证码不能为空")
		return
	}
	if message := validatePasswordComplexity(req.Password); message != "" {
		httpx.Error(w, http.StatusBadRequest, "invalid_password", message)
		return
	}
	if req.Password != req.ConfirmPassword {
		httpx.Error(w, http.StatusBadRequest, "invalid_request", "两次输入的密码不一致")
		return
	}
	if exists, err := s.phoneAlreadyRegistered(r.Context(), phone); err != nil {
		httpx.Error(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	} else if exists {
		httpx.Error(w, http.StatusConflict, "phone_already_registered", "该手机号已注册")
		return
	}
	if conflicts, err := s.phoneConflictsWithUsername(r.Context(), phone); err != nil {
		httpx.Error(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	} else if conflicts {
		httpx.Error(w, http.StatusConflict, "phone_conflicts_with_username", "该手机号无法作为登录手机号，请联系管理员")
		return
	}
	if err := s.store.ConsumePhoneVerificationCode(r.Context(), phone, phonePurposeRegister, strings.TrimSpace(req.Code), s.cfg.BillingInternalAPIKey, time.Now()); err != nil {
		switch {
		case errors.Is(err, ErrVerificationExpired):
			httpx.Error(w, http.StatusBadRequest, "verification_expired", "验证码已过期")
		case errors.Is(err, ErrVerificationLocked):
			httpx.Error(w, http.StatusTooManyRequests, "verification_locked", "验证码错误次数过多，请重新获取")
		default:
			httpx.Error(w, http.StatusBadRequest, "verification_invalid", "验证码不正确")
		}
		return
	}

	userName, displayName, err := s.newPhoneRegistrationUserName(r.Context())
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "username_generate_failed", err.Error())
		return
	}
	okCreated, err := casdoorsdk.AddUser(&casdoorsdk.User{
		Owner:       s.cfg.CasdoorOrganizationName,
		Name:        userName,
		DisplayName: displayName,
		Phone:       casdoorPhoneForMainland(phone),
		CountryCode: casdoorMainlandCountryCode,
		Avatar:      s.defaultAvatarURL(),
		Password:    req.Password,
	})
	if err != nil {
		if strings.Contains(err.Error(), "built-in") {
			httpx.Error(w, http.StatusConflict, "registration_org_not_allowed", "当前认证组织不允许前台注册，请为普通用户创建独立组织后再启用注册")
			return
		}
		httpx.Error(w, http.StatusBadGateway, "casdoor_register_failed", err.Error())
		return
	}
	if !okCreated {
		httpx.Error(w, http.StatusConflict, "user_already_exists", "该手机号已注册")
		return
	}
	if err := s.store.EnsureRegisteredUser(r.Context(), userName, s.cfg.CasdoorOrganizationName, phone, displayName); err != nil {
		httpx.Error(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func normalizeMainlandPhone(w http.ResponseWriter, countryCode string, phone string) (string, bool) {
	normalized, message, ok := normalizeMainlandPhoneValue(countryCode, phone)
	if !ok {
		httpx.Error(w, http.StatusBadRequest, "invalid_phone", message)
		return "", false
	}
	return normalized, true
}

func normalizeMainlandPhoneValue(countryCode string, phone string) (string, string, bool) {
	code := strings.TrimSpace(countryCode)
	number := strings.TrimSpace(phone)
	number = strings.ReplaceAll(number, " ", "")
	number = strings.ReplaceAll(number, "-", "")
	if strings.HasPrefix(number, "+86") {
		number = strings.TrimPrefix(number, "+86")
	}
	if strings.HasPrefix(number, "86") && len(number) == 13 {
		number = strings.TrimPrefix(number, "86")
	}
	if code == "" {
		code = "+86"
	}
	if code != "+86" && code != "86" {
		return "", "当前仅支持中国大陆手机号", false
	}
	if !mainlandPhonePattern.MatchString(number) {
		return "", "请输入有效的中国大陆手机号", false
	}
	return "+86" + number, "", true
}

func casdoorPhoneForMainland(phone string) string {
	normalized, _, ok := normalizeMainlandPhoneValue("+86", phone)
	if !ok {
		return strings.TrimSpace(phone)
	}
	return strings.TrimPrefix(normalized, "+86")
}

func internalPhoneFromCasdoorPhone(phone string) string {
	normalized, _, ok := normalizeMainlandPhoneValue("+86", phone)
	if !ok {
		return strings.TrimSpace(phone)
	}
	return normalized
}

func numericCode(length int) (string, error) {
	var builder strings.Builder
	for i := 0; i < length; i++ {
		n, err := rand.Int(rand.Reader, big.NewInt(10))
		if err != nil {
			return "", err
		}
		builder.WriteByte(byte('0' + n.Int64()))
	}
	return builder.String(), nil
}

func validatePasswordComplexity(password string) string {
	if len(password) < 8 {
		return "密码长度必须至少为 8 个字符"
	}
	hasUpper := false
	hasLower := false
	hasDigit := false
	for _, char := range password {
		switch {
		case char >= 'A' && char <= 'Z':
			hasUpper = true
		case char >= 'a' && char <= 'z':
			hasLower = true
		case char >= '0' && char <= '9':
			hasDigit = true
		}
	}
	if !hasUpper || !hasLower || !hasDigit {
		return "密码必须包含至少一个大写字母、一个小写字母和一个数字"
	}
	return ""
}

func randomHex(size int) string {
	raw := make([]byte, size)
	if _, err := rand.Read(raw); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(raw)
}

func casdoorNameFromPhone(phone string) string {
	return "u_" + strings.NewReplacer("+", "", "-", "", " ", "").Replace(phone)
}

func (s *Server) phoneAlreadyRegistered(ctx context.Context, phone string) (bool, error) {
	for _, candidate := range phoneLookupCandidates(phone) {
		if existing, err := s.store.FindUserByPhone(ctx, candidate); err == nil && existing.ID > 0 {
			return true, nil
		} else if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return false, err
		}
	}
	return s.casdoorPhoneExists(phone), nil
}

func (s *Server) phoneConflictsWithUsername(ctx context.Context, phone string) (bool, error) {
	for _, candidate := range phoneUsernameConflictCandidates(phone) {
		exists, err := s.store.UserNameExists(ctx, candidate)
		if err != nil {
			return false, err
		}
		if exists || s.casdoorUserNameExists(candidate) {
			return true, nil
		}
	}
	return false, nil
}

func phoneUsernameConflictCandidates(phone string) []string {
	normalized := strings.TrimSpace(phone)
	local := strings.TrimPrefix(normalized, "+86")
	if local == normalized || local == "" {
		return []string{normalized}
	}
	return []string{normalized, local}
}

func (s *Server) newPhoneRegistrationUserName(ctx context.Context) (string, string, error) {
	for i := 0; i < 32; i++ {
		suffix, err := randomLowerAlpha(5)
		if err != nil {
			return "", "", err
		}
		name := "user_" + suffix
		displayName := "用户" + suffix
		exists, err := s.store.UserNameExists(ctx, name)
		if err != nil {
			return "", "", err
		}
		if !exists && !s.casdoorUserNameExists(name) {
			return name, displayName, nil
		}
	}
	return "", "", fmt.Errorf("unable to generate unique username")
}

func randomLowerAlpha(length int) (string, error) {
	var builder strings.Builder
	for i := 0; i < length; i++ {
		n, err := rand.Int(rand.Reader, big.NewInt(26))
		if err != nil {
			return "", err
		}
		builder.WriteByte(byte('a' + n.Int64()))
	}
	return builder.String(), nil
}

func maskPhone(phone string) string {
	number := strings.TrimPrefix(phone, "+86")
	if len(number) == 11 {
		return number[:3] + "****" + number[7:]
	}
	return phone
}

func (s *Server) defaultAvatarURL() string {
	publicWebURL := strings.TrimRight(s.cfg.PublicWebURL, "/")
	if publicWebURL == "" {
		return ""
	}
	return publicWebURL + "/default-avatar.svg"
}
