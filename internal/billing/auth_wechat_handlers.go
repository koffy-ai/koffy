package billing

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"koffy/internal/auth"
	"koffy/internal/httpx"

	"github.com/casdoor/casdoor-go-sdk/casdoorsdk"
)

type wechatOAuthTokenResponse struct {
	AccessToken  string `json:"access_token"`
	ExpiresIn    int    `json:"expires_in"`
	RefreshToken string `json:"refresh_token"`
	OpenID       string `json:"openid"`
	Scope        string `json:"scope"`
	UnionID      string `json:"unionid"`
	ErrCode      int    `json:"errcode"`
	ErrMsg       string `json:"errmsg"`
}

type wechatUserInfoResponse struct {
	OpenID     string   `json:"openid"`
	Nickname   string   `json:"nickname"`
	Sex        int      `json:"sex"`
	Province   string   `json:"province"`
	City       string   `json:"city"`
	Country    string   `json:"country"`
	HeadImgURL string   `json:"headimgurl"`
	Privilege  []string `json:"privilege"`
	UnionID    string   `json:"unionid"`
	ErrCode    int      `json:"errcode"`
	ErrMsg     string   `json:"errmsg"`
}

type wechatWidgetConfigResponse struct {
	AppID       string `json:"app_id"`
	AppType     string `json:"app_type"`
	Scope       string `json:"scope"`
	RedirectURI string `json:"redirect_uri"`
	State       string `json:"state"`
	StyleLite   string `json:"stylelite"`
	AuthURL     string `json:"auth_url"`
}

func (s *Server) startWeChatAuth(w http.ResponseWriter, r *http.Request) {
	appType := s.wechatAppType(r)
	action := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("action")))
	if action == "" {
		action = "login"
	}
	if action != "login" && action != "bind" && action != "pay" {
		httpx.Error(w, http.StatusBadRequest, "invalid_request", "不支持的微信授权动作")
		return
	}

	appID, redirectURI, state, scope, err := s.createWeChatAuthState(r, appType, action)
	if err != nil {
		s.writeWeChatConfigError(w, err)
		return
	}

	http.Redirect(w, r, s.buildWeChatAuthURL(appType, appID, redirectURI, state, scope), http.StatusFound)
}

func (s *Server) wechatWidgetConfig(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	action := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("action")))
	if action == "" {
		action = "login"
	}
	if action != "login" && action != "bind" && action != "pay" {
		httpx.Error(w, http.StatusBadRequest, "invalid_request", "不支持的微信授权动作")
		return
	}

	appType := "website"
	if strings.Contains(strings.ToLower(r.UserAgent()), "micromessenger") {
		appType = "official"
	}
	appID, redirectURI, state, scope, err := s.createWeChatAuthState(r, appType, action)
	if err != nil {
		s.writeWeChatConfigError(w, err)
		return
	}
	authURL := s.buildWeChatAuthURL(appType, appID, redirectURI, state, scope)
	httpx.JSON(w, http.StatusOK, wechatWidgetConfigResponse{
		AppID:       appID,
		AppType:     appType,
		Scope:       scope,
		RedirectURI: redirectURI,
		State:       state,
		StyleLite:   "1",
		AuthURL:     authURL,
	})
}

func (s *Server) buildWeChatAuthURL(appType string, appID string, redirectURI string, state string, scope string) string {
	authURL := &url.URL{
		Scheme: "https",
		Host:   "open.weixin.qq.com",
	}
	if appType == "official" {
		authURL.Path = "/connect/oauth2/authorize"
	} else {
		authURL.Path = "/connect/qrconnect"
	}
	query := url.Values{}
	query.Set("appid", appID)
	query.Set("redirect_uri", redirectURI)
	query.Set("response_type", "code")
	query.Set("scope", scope)
	query.Set("state", state)
	authURL.RawQuery = query.Encode()
	authURL.Fragment = "wechat_redirect"
	return authURL.String()
}

func (s *Server) createWeChatAuthState(r *http.Request, appType string, action string) (string, string, string, string, error) {
	var userID sql.NullInt64
	if action == "bind" || action == "pay" {
		principal, err := s.auth.PrincipalFromRequest(r)
		if err != nil {
			return "", "", "", "", err
		}
		current, err := s.store.SyncUser(r.Context(), principal)
		if err != nil {
			return "", "", "", "", err
		}
		userID = sql.NullInt64{Int64: current.ID, Valid: true}
	}
	appID, _, err := s.wechatAppCredentials(appType)
	if err != nil {
		return "", "", "", "", err
	}
	state, err := randomState()
	if err != nil {
		return "", "", "", "", err
	}
	returnTo := s.safeReturnTo(r.URL.Query().Get("return_to"))
	if err := s.store.CreateAuthState(r.Context(), authState{
		State:     state,
		Provider:  "wechat",
		AppType:   appType,
		Action:    action,
		ReturnTo:  returnTo,
		UserID:    userID,
		ExpiresAt: time.Now().Add(10 * time.Minute),
	}); err != nil {
		return "", "", "", "", err
	}

	redirectURI := strings.TrimRight(s.cfg.PublicWebURL, "/") + "/api/v1/auth/wechat/callback"
	scope := "snsapi_login"
	if appType == "official" {
		scope = "snsapi_userinfo"
	}
	if appType == "official" && action == "pay" {
		scope = "snsapi_base"
	}
	return appID, redirectURI, state, scope, nil
}

func (s *Server) writeWeChatConfigError(w http.ResponseWriter, err error) {
	if errors.Is(err, auth.ErrMissingToken) || errors.Is(err, auth.ErrInvalidToken) || errors.Is(err, auth.ErrEmptyUserClaim) {
		httpx.Error(w, http.StatusUnauthorized, "unauthorized", "请先登录")
		return
	}
	if strings.Contains(err.Error(), "未配置") {
		httpx.Error(w, http.StatusInternalServerError, "wechat_not_configured", err.Error())
		return
	}
	httpx.Error(w, http.StatusInternalServerError, "internal_error", err.Error())
}

func (s *Server) wechatCallback(w http.ResponseWriter, r *http.Request) {
	code := strings.TrimSpace(r.URL.Query().Get("code"))
	stateValue := strings.TrimSpace(r.URL.Query().Get("state"))
	if code == "" || stateValue == "" {
		s.writeWeChatCallbackRedirect(w, withWeChatError("/login", "invalid_request", "微信授权参数不完整，请重新登录"))
		return
	}
	state, err := s.store.ConsumeAuthState(r.Context(), stateValue, time.Now())
	if err != nil {
		if target, ok := s.recoverConsumedWeChatStateTarget(r, stateValue); ok {
			s.writeWeChatCallbackRedirect(w, target)
			return
		}
		s.writeWeChatCallbackRedirect(w, withWeChatError("/login", "invalid_state", "微信授权已过期，请重新发起授权"))
		return
	}
	identity, err := s.fetchWeChatIdentity(r, state.AppType, code, state.Action != "pay")
	if err != nil {
		s.writeWeChatCallbackRedirect(w, s.wechatFailureTarget(state, "wechat_oauth_failed", "微信授权失败，请稍后重试"))
		return
	}

	if state.Action == "pay" {
		if !state.UserID.Valid {
			s.writeWeChatCallbackRedirect(w, s.wechatFailureTarget(state, "invalid_state", "微信支付状态无效，请重新发起支付"))
			return
		}
		payCode, err := s.store.CreateWeChatPayOpenIDCode(r.Context(), state.UserID.Int64, identity.OpenID, time.Now())
		if err != nil {
			s.writeWeChatCallbackRedirect(w, s.wechatFailureTarget(state, "internal_error", "微信支付授权失败，请稍后重试"))
			return
		}
		s.writeWeChatCallbackRedirect(w, withQueryValue(state.ReturnTo, "wechat_pay_code", payCode))
		return
	}

	if state.Action == "bind" {
		if !state.UserID.Valid {
			s.writeWeChatCallbackRedirect(w, s.wechatFailureTarget(state, "invalid_state", "微信绑定状态无效，请重新绑定"))
			return
		}
		if existing, err := s.store.FindUserByWeChatIdentity(r.Context(), identity); err == nil && existing.ID != state.UserID.Int64 {
			s.writeWeChatCallbackRedirect(w, s.wechatFailureTarget(state, "wechat_already_bound", "抱歉，该微信号已绑定其它账号，您可以直接使用该微信号登录另一个账号，或者使用另一个微信号绑定当前账号。"))
			return
		}
		if err := s.store.BindWeChatIdentity(r.Context(), state.UserID.Int64, identity); err != nil {
			s.writeWeChatCallbackRedirect(w, s.wechatFailureTarget(state, "internal_error", "微信绑定失败，请稍后重试"))
			return
		}
		if user, displayChanged, avatarChanged, err := s.store.ApplyWeChatProfileDefaults(r.Context(), state.UserID.Int64, identity); err == nil {
			_ = s.syncCasdoorProfileColumns(user, displayChanged, avatarChanged)
		}
		s.writeWeChatCallbackRedirect(w, state.ReturnTo)
		return
	}

	user, err := s.store.FindUserByWeChatIdentity(r.Context(), identity)
	if err != nil {
		if err != sql.ErrNoRows {
			s.writeWeChatCallbackRedirect(w, s.wechatFailureTarget(state, "internal_error", "微信登录失败，请稍后重试"))
			return
		}
		user, err = s.createWeChatUser(r, identity)
		if err != nil {
			s.writeWeChatCallbackRedirect(w, s.wechatFailureTarget(state, "wechat_user_create_failed", "微信账号创建失败，请稍后重试"))
			return
		}
	} else if updated, displayChanged, avatarChanged, applyErr := s.store.ApplyWeChatProfileDefaults(r.Context(), user.ID, identity); applyErr == nil {
		user = updated
		_ = s.syncCasdoorProfileColumns(user, displayChanged, avatarChanged)
	}

	s.writeWeChatCallbackRedirect(w, s.withLoginExchangeCode(r, state.ReturnTo, user))
}

func (s *Server) syncCasdoorProfileColumns(user UserProfile, displayChanged bool, avatarChanged bool) error {
	if !displayChanged && !avatarChanged {
		return nil
	}
	casdoorUser, err := casdoorsdk.GetUser(user.CasdoorUserID)
	if err != nil || casdoorUser == nil {
		return err
	}
	columns := make([]string, 0, 2)
	if displayChanged {
		casdoorUser.DisplayName = user.DisplayName
		columns = append(columns, "displayName")
	}
	if avatarChanged {
		casdoorUser.Avatar = user.AvatarURL
		columns = append(columns, "avatar")
	}
	_, err = casdoorsdk.UpdateUserForColumns(casdoorUser, columns)
	return err
}

func (s *Server) createWeChatUser(r *http.Request, identity wechatIdentity) (UserProfile, error) {
	userName := "wx_" + shortHash(firstNonEmpty(identity.UnionID, identity.OpenID))
	displayName := strings.TrimSpace(identity.Nickname)
	if displayName == "" {
		displayName = "微信用户"
	}
	displayName, err := s.store.UniqueDisplayName(r.Context(), displayName, 0)
	if err != nil {
		return UserProfile{}, err
	}
	randomPassword := "Wx" + randomHex(18) + "9"
	ok, err := casdoorsdk.AddUser(&casdoorsdk.User{
		Owner:       s.cfg.CasdoorOrganizationName,
		Name:        userName,
		DisplayName: displayName,
		Avatar:      firstNonEmpty(identity.AvatarURL, s.defaultAvatarURL()),
		Password:    randomPassword,
	})
	if err != nil && !strings.Contains(err.Error(), "already exists") {
		return UserProfile{}, err
	}
	if err == nil && !ok {
		return UserProfile{}, fmt.Errorf("认证服务未创建微信用户")
	}
	return s.store.EnsureWeChatUser(r.Context(), auth.Principal{
		ID:          userName,
		Name:        userName,
		DisplayName: displayName,
		Owner:       s.cfg.CasdoorOrganizationName,
	}, identity)
}

func (s *Server) fetchWeChatIdentity(r *http.Request, appType string, code string, fetchProfile bool) (wechatIdentity, error) {
	appID, appSecret, err := s.wechatAppCredentials(appType)
	if err != nil {
		return wechatIdentity{}, err
	}
	tokenURL := "https://api.weixin.qq.com/sns/oauth2/access_token"
	query := url.Values{}
	query.Set("appid", appID)
	query.Set("secret", appSecret)
	query.Set("code", code)
	query.Set("grant_type", "authorization_code")
	var tokenResp wechatOAuthTokenResponse
	if err := getJSON(r, tokenURL+"?"+query.Encode(), &tokenResp); err != nil {
		return wechatIdentity{}, err
	}
	if tokenResp.ErrCode != 0 {
		return wechatIdentity{}, fmt.Errorf("微信授权失败：%s", tokenResp.ErrMsg)
	}
	if tokenResp.OpenID == "" || tokenResp.AccessToken == "" {
		return wechatIdentity{}, fmt.Errorf("微信授权结果缺少 openid")
	}
	if !fetchProfile {
		return wechatIdentity{
			AppType: appType,
			OpenID:  tokenResp.OpenID,
			UnionID: tokenResp.UnionID,
		}, nil
	}

	infoURL := "https://api.weixin.qq.com/sns/userinfo"
	infoQuery := url.Values{}
	infoQuery.Set("access_token", tokenResp.AccessToken)
	infoQuery.Set("openid", tokenResp.OpenID)
	infoQuery.Set("lang", "zh_CN")
	var info wechatUserInfoResponse
	if err := getJSON(r, infoURL+"?"+infoQuery.Encode(), &info); err != nil {
		return wechatIdentity{}, err
	}
	if info.ErrCode != 0 {
		return wechatIdentity{}, fmt.Errorf("微信用户资料获取失败：%s", info.ErrMsg)
	}
	unionID := firstNonEmpty(info.UnionID, tokenResp.UnionID)
	return wechatIdentity{
		AppType:   appType,
		OpenID:    firstNonEmpty(info.OpenID, tokenResp.OpenID),
		UnionID:   unionID,
		Nickname:  info.Nickname,
		AvatarURL: info.HeadImgURL,
	}, nil
}

func (s *Server) wechatAppType(r *http.Request) string {
	mode := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("mode")))
	switch mode {
	case "official", "wechat":
		return "official"
	case "website", "web":
		return "website"
	}
	ua := strings.ToLower(r.UserAgent())
	if strings.Contains(ua, "micromessenger") {
		return "official"
	}
	return "website"
}

func (s *Server) wechatAppCredentials(appType string) (string, string, error) {
	if appType == "official" {
		if s.cfg.WeChatOfficialAppID == "" || s.cfg.WeChatOfficialAppSecret == "" {
			return "", "", fmt.Errorf("服务号微信登录未配置")
		}
		return s.cfg.WeChatOfficialAppID, s.cfg.WeChatOfficialAppSecret, nil
	}
	if s.cfg.WeChatWebsiteAppID == "" || s.cfg.WeChatWebsiteAppSecret == "" {
		return "", "", fmt.Errorf("开放平台网站应用微信登录未配置")
	}
	return s.cfg.WeChatWebsiteAppID, s.cfg.WeChatWebsiteAppSecret, nil
}

func (s *Server) wechatFailureTarget(state authState, code string, message string) string {
	target := state.ReturnTo
	if state.Action != "bind" && state.Action != "pay" {
		target = "/login?return_to=" + url.QueryEscape(state.ReturnTo)
	}
	return withWeChatError(target, code, message)
}

func withWeChatError(target string, code string, message string) string {
	parsed, err := url.Parse(target)
	if err != nil {
		parsed = &url.URL{Path: "/login"}
	}
	query := parsed.Query()
	query.Set("wechat_error_code", code)
	query.Set("wechat_error", message)
	parsed.RawQuery = query.Encode()
	return parsed.String()
}

func (s *Server) recoverConsumedWeChatStateTarget(r *http.Request, stateValue string) (string, bool) {
	state, err := s.store.AuthStateByState(r.Context(), stateValue)
	if err != nil {
		return "", false
	}
	if _, err := s.auth.PrincipalFromRequest(r); err != nil {
		return "", false
	}
	if state.ReturnTo == "" {
		return "/center", true
	}
	return state.ReturnTo, true
}

func (s *Server) writeWeChatCallbackRedirect(w http.ResponseWriter, target string) {
	if strings.TrimSpace(target) == "" {
		target = "/center"
	}
	jsTarget, err := json.Marshal(target)
	if err != nil {
		jsTarget = []byte(`"/center"`)
		target = "/center"
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusOK)
	_, _ = fmt.Fprintf(w, `<!doctype html>
<html lang="zh-CN">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>微信登录</title>
</head>
<body>
  <script>
    (function () {
      var target = %s;
      try {
        if (window.top && window.top !== window) {
          window.top.location.replace(target);
          return;
        }
      } catch (e) {}
      window.location.replace(target);
    })();
  </script>
  <p>微信授权完成，正在跳转...</p>
  <p><a href="%s" target="_top" rel="noreferrer">点击继续</a></p>
</body>
</html>`, jsTarget, html.EscapeString(target))
}

func withQueryValue(target string, key string, value string) string {
	parsed, err := url.Parse(target)
	if err != nil {
		parsed = &url.URL{Path: "/center/recharge/"}
	}
	query := parsed.Query()
	query.Set(key, value)
	parsed.RawQuery = query.Encode()
	return parsed.String()
}

func getJSON(r *http.Request, target string, dst any) error {
	req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, target, nil)
	if err != nil {
		return err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("HTTP %s: %s", resp.Status, string(body))
	}
	return json.Unmarshal(body, dst)
}

func shortHash(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])[:24]
}
