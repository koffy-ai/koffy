package billing

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"strings"
	"time"

	"koffy/internal/httpx"

	"github.com/casdoor/casdoor-go-sdk/casdoorsdk"
)

type sendAccountPhoneCodeRequest struct {
	CountryCode string `json:"country_code"`
	Phone       string `json:"phone"`
	HumanToken  string `json:"human_token"`
}

type resetPasswordRequest struct {
	CountryCode     string `json:"country_code"`
	Phone           string `json:"phone"`
	Code            string `json:"code"`
	Password        string `json:"password"`
	ConfirmPassword string `json:"confirm_password"`
}

type bindPhoneRequest struct {
	CountryCode     string `json:"country_code"`
	Phone           string `json:"phone"`
	Code            string `json:"code"`
	Password        string `json:"password"`
	ConfirmPassword string `json:"confirm_password"`
}

type changePasswordCodeRequest struct {
	HumanToken string `json:"human_token"`
}

type changePasswordRequest struct {
	Code            string `json:"code"`
	Password        string `json:"password"`
	ConfirmPassword string `json:"confirm_password"`
}

func (s *Server) sendResetPasswordCode(w http.ResponseWriter, r *http.Request) {
	var req sendAccountPhoneCodeRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	phone, ok := normalizeMainlandPhone(w, req.CountryCode, req.Phone)
	if !ok {
		return
	}
	if !s.casdoorPhoneExists(phone) {
		httpx.Error(w, http.StatusNotFound, "user_not_found", "该手机号尚未注册")
		return
	}
	if err := s.verifyHumanToken(r, req.HumanToken); err != nil {
		httpx.Error(w, http.StatusBadRequest, "human_verification_failed", err.Error())
		return
	}
	s.sendPurposePhoneCode(w, r, phone, phonePurposeResetPassword)
}

func (s *Server) resetPassword(w http.ResponseWriter, r *http.Request) {
	var req resetPasswordRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	phone, ok := normalizeMainlandPhone(w, req.CountryCode, req.Phone)
	if !ok {
		return
	}
	if !validatePasswordRequest(w, req.Code, req.Password, req.ConfirmPassword) {
		return
	}
	if !s.consumePurposePhoneCode(w, r, phone, phonePurposeResetPassword, req.Code) {
		return
	}
	user, err := s.casdoorUserByPhone(phone)
	if err != nil || user == nil || user.Name == "" {
		httpx.Error(w, http.StatusNotFound, "user_not_found", "该手机号尚未注册")
		return
	}
	if !s.setCasdoorPassword(w, user.Owner, user.Name, req.Password) {
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) sendBindPhoneCode(w http.ResponseWriter, r *http.Request) {
	current, ok := s.currentUserProfile(w, r)
	if !ok {
		return
	}
	if !s.ensureUserInConfiguredCasdoorOrganization(w, current, "当前账号不属于 Koffy 认证组织，不能在这里更换手机号") {
		return
	}
	var req sendAccountPhoneCodeRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	phone, ok := normalizeMainlandPhone(w, req.CountryCode, req.Phone)
	if !ok {
		return
	}
	if exists, err := s.localPhoneBoundToOther(r.Context(), phone, current.ID); err != nil {
		httpx.Error(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	} else if exists {
		httpx.Error(w, http.StatusConflict, "phone_already_bound", "该手机号已绑定其它账号")
		return
	}
	if s.casdoorPhoneBoundToOther(phone, current.CasdoorUserID) {
		httpx.Error(w, http.StatusConflict, "phone_already_bound", "该手机号已绑定其它账号")
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
	s.sendPurposePhoneCode(w, r, phone, phonePurposeBindPhone)
}

func (s *Server) bindPhone(w http.ResponseWriter, r *http.Request) {
	current, ok := s.currentUserProfile(w, r)
	if !ok {
		return
	}
	var req bindPhoneRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	phone, ok := normalizeMainlandPhone(w, req.CountryCode, req.Phone)
	if !ok {
		return
	}
	if !validatePasswordRequest(w, req.Code, req.Password, req.ConfirmPassword) {
		return
	}
	if exists, err := s.localPhoneBoundToOther(r.Context(), phone, current.ID); err != nil {
		httpx.Error(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	} else if exists {
		httpx.Error(w, http.StatusConflict, "phone_already_bound", "该手机号已绑定其它账号")
		return
	}
	if s.casdoorPhoneBoundToOther(phone, current.CasdoorUserID) {
		httpx.Error(w, http.StatusConflict, "phone_already_bound", "该手机号已绑定其它账号")
		return
	}
	if conflicts, err := s.phoneConflictsWithUsername(r.Context(), phone); err != nil {
		httpx.Error(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	} else if conflicts {
		httpx.Error(w, http.StatusConflict, "phone_conflicts_with_username", "该手机号无法作为登录手机号，请联系管理员")
		return
	}
	casdoorUser, ok := s.currentWritableCasdoorUser(w, current, "当前账号不属于 Koffy 认证组织，不能在这里更换手机号")
	if !ok {
		return
	}
	if !s.consumePurposePhoneCode(w, r, phone, phonePurposeBindPhone, req.Code) {
		return
	}
	casdoorUser.Phone = casdoorPhoneForMainland(phone)
	casdoorUser.CountryCode = casdoorMainlandCountryCode
	if casdoorUser.DisplayName == "" {
		casdoorUser.DisplayName = maskPhone(phone)
	}
	if ok, err := casdoorsdk.UpdateUserForColumns(casdoorUser, []string{"phone", "countryCode", "displayName"}); err != nil {
		httpx.Error(w, http.StatusBadGateway, "casdoor_user_update_failed", err.Error())
		return
	} else if !ok {
		httpx.Error(w, http.StatusBadGateway, "casdoor_user_update_failed", "认证服务未更新用户资料")
		return
	}
	verifiedCasdoorUser, err := casdoorsdk.GetUser(current.CasdoorUserID)
	if err != nil || verifiedCasdoorUser == nil || !samePhoneValue(verifiedCasdoorUser.Phone, phone) {
		httpx.Error(w, http.StatusConflict, "phone_already_bound", "该手机号已绑定其它账号")
		return
	}
	if !s.setCasdoorPassword(w, casdoorUser.Owner, casdoorUser.Name, req.Password) {
		return
	}
	if err := s.store.UpdateUserPhone(r.Context(), current.ID, phone, maskPhone(phone)); err != nil {
		if isDuplicatePhoneError(err) {
			httpx.Error(w, http.StatusConflict, "phone_already_bound", "该手机号已绑定其它账号")
			return
		}
		httpx.Error(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	updatedLocalUser, err := s.store.UserByCasdoorID(r.Context(), current.CasdoorUserID)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	token, expiry, err := s.auth.NewSessionToken(principalFromUserProfile(updatedLocalUser), 7*24*time.Hour)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "session_create_failed", err.Error())
		return
	}
	s.setSessionCookie(w, token, expiry)
	httpx.JSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) setCasdoorPassword(w http.ResponseWriter, owner string, name string, password string) bool {
	if ok, err := casdoorsdk.SetPassword(owner, name, "", password); err != nil {
		if isSamePasswordError(err) {
			return true
		}
		httpx.Error(w, http.StatusBadGateway, "casdoor_password_update_failed", userFacingCasdoorError(err))
		return false
	} else if !ok {
		httpx.Error(w, http.StatusBadGateway, "casdoor_password_update_failed", "认证服务未更新密码")
		return false
	}
	return true
}

func (s *Server) sendChangePasswordCode(w http.ResponseWriter, r *http.Request) {
	current, ok := s.currentUserProfile(w, r)
	if !ok {
		return
	}
	if !s.ensureUserInConfiguredCasdoorOrganization(w, current, "当前账号不属于 Koffy 认证组织，不能在这里修改密码") {
		return
	}
	if current.Phone == "" {
		httpx.Error(w, http.StatusBadRequest, "phone_not_bound", "请先绑定手机号")
		return
	}
	var req changePasswordCodeRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if err := s.verifyHumanToken(r, req.HumanToken); err != nil {
		httpx.Error(w, http.StatusBadRequest, "human_verification_failed", err.Error())
		return
	}
	s.sendPurposePhoneCode(w, r, current.Phone, phonePurposeChangePassword)
}

func (s *Server) changePassword(w http.ResponseWriter, r *http.Request) {
	current, ok := s.currentUserProfile(w, r)
	if !ok {
		return
	}
	if current.Phone == "" {
		httpx.Error(w, http.StatusBadRequest, "phone_not_bound", "请先绑定手机号")
		return
	}
	var req changePasswordRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if !validatePasswordRequest(w, req.Code, req.Password, req.ConfirmPassword) {
		return
	}
	if !s.ensureUserInConfiguredCasdoorOrganization(w, current, "当前账号不属于 Koffy 认证组织，不能在这里修改密码") {
		return
	}
	if !s.consumePurposePhoneCode(w, r, current.Phone, phonePurposeChangePassword, req.Code) {
		return
	}
	if !s.setCasdoorPassword(w, current.Owner, current.CasdoorUserID, req.Password) {
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) authBindings(w http.ResponseWriter, r *http.Request) {
	current, ok := s.currentUserProfile(w, r)
	if !ok {
		return
	}
	bindings, err := s.store.AuthBindings(r.Context(), current.ID)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	httpx.JSON(w, http.StatusOK, bindings)
}

func (s *Server) unbindWeChat(w http.ResponseWriter, r *http.Request) {
	current, ok := s.currentUserProfile(w, r)
	if !ok {
		return
	}
	if err := s.store.UnbindWeChatIdentity(r.Context(), current.ID); err != nil {
		switch {
		case errors.Is(err, sql.ErrNoRows):
			httpx.Error(w, http.StatusNotFound, "wechat_not_bound", "当前账号未绑定微信")
		case strings.Contains(err.Error(), "请先绑定手机号"):
			httpx.Error(w, http.StatusBadRequest, "phone_required", err.Error())
		default:
			httpx.Error(w, http.StatusInternalServerError, "internal_error", err.Error())
		}
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) sendPurposePhoneCode(w http.ResponseWriter, r *http.Request, phone string, purpose string) {
	code, err := numericCode(6)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	if _, err := s.store.CreatePhoneVerificationCode(r.Context(), phone, purpose, code, s.cfg.BillingInternalAPIKey, time.Now()); err != nil {
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

func (s *Server) consumePurposePhoneCode(w http.ResponseWriter, r *http.Request, phone string, purpose string, code string) bool {
	if strings.TrimSpace(code) == "" {
		httpx.Error(w, http.StatusBadRequest, "invalid_request", "验证码不能为空")
		return false
	}
	if err := s.store.ConsumePhoneVerificationCode(r.Context(), phone, purpose, strings.TrimSpace(code), s.cfg.BillingInternalAPIKey, time.Now()); err != nil {
		switch {
		case errors.Is(err, ErrVerificationExpired):
			httpx.Error(w, http.StatusBadRequest, "verification_expired", "验证码已过期")
		case errors.Is(err, ErrVerificationLocked):
			httpx.Error(w, http.StatusTooManyRequests, "verification_locked", "验证码错误次数过多，请重新获取")
		default:
			httpx.Error(w, http.StatusBadRequest, "verification_invalid", "验证码不正确")
		}
		return false
	}
	return true
}

func validatePasswordRequest(w http.ResponseWriter, code string, password string, confirmPassword string) bool {
	if strings.TrimSpace(code) == "" {
		httpx.Error(w, http.StatusBadRequest, "invalid_request", "验证码不能为空")
		return false
	}
	if message := validatePasswordComplexity(password); message != "" {
		httpx.Error(w, http.StatusBadRequest, "invalid_password", message)
		return false
	}
	if password != confirmPassword {
		httpx.Error(w, http.StatusBadRequest, "invalid_request", "两次输入的密码不一致")
		return false
	}
	return true
}

func (s *Server) casdoorPhoneExists(phone string) bool {
	user, err := s.casdoorUserByPhone(phone)
	return err == nil && user != nil && user.Name != ""
}

func (s *Server) casdoorPhoneBoundToOther(phone string, currentName string) bool {
	currentName = strings.TrimSpace(currentName)
	user, err := s.casdoorUserByPhone(phone)
	return err == nil && user != nil && user.Name != "" && user.Name != currentName
}

func (s *Server) casdoorUserByPhone(phone string) (*casdoorsdk.User, error) {
	var lastErr error
	for _, candidate := range phoneLookupCandidates(phone) {
		user, err := casdoorsdk.GetUserByPhone(candidate)
		if err == nil && user != nil && strings.TrimSpace(user.Name) != "" {
			return user, nil
		}
		if err != nil {
			lastErr = err
		}
	}
	return nil, lastErr
}

func (s *Server) casdoorUserNameExists(name string) bool {
	user, err := casdoorsdk.GetUser(name)
	return err == nil && user != nil && user.Name != ""
}

func (s *Server) localPhoneBoundToOther(ctx context.Context, phone string, currentUserID int64) (bool, error) {
	for _, candidate := range phoneLookupCandidates(phone) {
		existing, err := s.store.FindUserByPhone(ctx, candidate)
		if errors.Is(err, sql.ErrNoRows) {
			continue
		}
		if err != nil {
			return false, err
		}
		if existing.ID > 0 && existing.ID != currentUserID {
			return true, nil
		}
	}
	return false, nil
}

func phoneLookupCandidates(phone string) []string {
	normalized := strings.TrimSpace(phone)
	local := strings.TrimPrefix(normalized, "+86")
	if local == "" || local == normalized {
		return []string{normalized}
	}
	return []string{normalized, local}
}

func samePhoneValue(left string, right string) bool {
	left = strings.TrimSpace(left)
	right = strings.TrimSpace(right)
	if left == right {
		return true
	}
	leftLocal := strings.TrimPrefix(left, "+86")
	rightLocal := strings.TrimPrefix(right, "+86")
	return leftLocal != "" && leftLocal == rightLocal
}

func isDuplicatePhoneError(err error) bool {
	if err == nil {
		return false
	}
	lower := strings.ToLower(err.Error())
	return strings.Contains(lower, "duplicate") &&
		(strings.Contains(lower, "phone") || strings.Contains(lower, "uk_users_phone_unique"))
}

func isSamePasswordError(err error) bool {
	if err == nil {
		return false
	}
	lower := strings.ToLower(err.Error())
	return strings.Contains(lower, "new password must be different") ||
		strings.Contains(lower, "same password") ||
		strings.Contains(err.Error(), "新密码")
}

func userFacingCasdoorError(err error) string {
	if err == nil {
		return "认证服务操作失败"
	}
	lower := strings.ToLower(err.Error())
	switch {
	case strings.Contains(lower, "phone") && (strings.Contains(lower, "exist") || strings.Contains(lower, "duplicate")):
		return "该手机号已绑定其它账号"
	default:
		return "认证服务操作失败，请稍后再试"
	}
}
