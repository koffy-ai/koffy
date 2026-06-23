package billing

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"koffy/internal/auth"
	"koffy/internal/httpx"

	"github.com/casdoor/casdoor-go-sdk/casdoorsdk"
)

const oauthStateCookieName = "billing_oauth_state"
const oauthReturnToCookieName = "billing_oauth_return_to"

type loginAccountRequest struct {
	Account  string `json:"account"`
	Password string `json:"password"`
	ReturnTo string `json:"return_to"`
}

type sessionExchangeRequest struct {
	Code string `json:"code"`
}

type loginAccountResponse struct {
	Status     string `json:"status"`
	RedirectTo string `json:"redirect_to"`
}

type casdoorPasswordLoginResponse struct {
	Status string `json:"status"`
	Msg    string `json:"msg"`
	Data   string `json:"data"`
}

type loginCandidate struct {
	organization string
	username     string
}

func (s *Server) login(w http.ResponseWriter, r *http.Request) {
	returnTo := s.safeReturnTo(r.URL.Query().Get("return_to"))
	http.Redirect(w, r, "/login?return_to="+url.QueryEscape(returnTo), http.StatusFound)
}

func (s *Server) register(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, "/register", http.StatusFound)
}

func (s *Server) startOAuth(w http.ResponseWriter, r *http.Request, action string) {
	if s.cfg.CasdoorEndpoint == "" || s.cfg.CasdoorClientID == "" {
		httpx.Error(w, http.StatusInternalServerError, "casdoor_not_configured", "casdoor endpoint and client id are required")
		return
	}
	redirectURI := strings.TrimRight(s.cfg.PublicWebURL, "/") + "/auth/callback"
	if s.cfg.AppEnv == "local" {
		if err := s.store.EnsureLocalCasdoorOAuth(
			r.Context(),
			s.cfg.CasdoorOrganizationName,
			s.cfg.CasdoorApplicationName,
			s.cfg.CasdoorClientID,
			s.cfg.CasdoorClientSecret,
			redirectURI,
		); err != nil {
			httpx.Error(w, http.StatusInternalServerError, "local_casdoor_bootstrap_failed", err.Error())
			return
		}
	}
	state, err := randomState()
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     oauthStateCookieName,
		Value:    state,
		Path:     "/",
		MaxAge:   300,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   s.secureCookies(),
	})
	http.SetCookie(w, &http.Cookie{
		Name:     oauthReturnToCookieName,
		Value:    s.safeReturnTo(r.URL.Query().Get("return_to")),
		Path:     "/",
		MaxAge:   300,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   s.secureCookies(),
	})

	publicCasdoorURL := s.cfg.PublicCasdoorURL
	if publicCasdoorURL == "" {
		publicCasdoorURL = s.cfg.CasdoorEndpoint
	}
	authPath := "/login/oauth/authorize"
	if action == "signup" {
		authPath = "/signup/oauth/authorize"
	}
	authURL, err := url.Parse(strings.TrimRight(publicCasdoorURL, "/") + authPath)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "invalid_casdoor_endpoint", err.Error())
		return
	}
	query := authURL.Query()
	query.Set("client_id", s.cfg.CasdoorClientID)
	query.Set("response_type", "code")
	query.Set("redirect_uri", redirectURI)
	query.Set("scope", "read")
	query.Set("state", state)
	authURL.RawQuery = query.Encode()

	http.Redirect(w, r, authURL.String(), http.StatusFound)
}

func (s *Server) authCallback(w http.ResponseWriter, r *http.Request) {
	code := strings.TrimSpace(r.URL.Query().Get("code"))
	state := strings.TrimSpace(r.URL.Query().Get("state"))
	if code == "" || state == "" {
		httpx.Error(w, http.StatusBadRequest, "invalid_request", "code and state are required")
		return
	}
	stateCookie, err := r.Cookie(oauthStateCookieName)
	if err != nil || stateCookie.Value != state {
		httpx.Error(w, http.StatusBadRequest, "invalid_state", "oauth state is invalid or expired")
		return
	}

	token, err := casdoorsdk.GetOAuthToken(code, state)
	if err != nil {
		httpx.Error(w, http.StatusBadGateway, "casdoor_token_exchange_failed", err.Error())
		return
	}
	if token.AccessToken == "" {
		httpx.Error(w, http.StatusBadGateway, "casdoor_token_exchange_failed", "access token is empty")
		return
	}

	s.setSessionCookie(w, token.AccessToken, token.Expiry)
	clearCookie(w, oauthStateCookieName, s.secureCookies())
	returnTo := "/center"
	if cookie, err := r.Cookie(oauthReturnToCookieName); err == nil {
		returnTo = s.safeReturnTo(cookie.Value)
	}
	clearCookie(w, oauthReturnToCookieName, s.secureCookies())
	http.Redirect(w, r, returnTo, http.StatusFound)
}

func (s *Server) loginAccount(w http.ResponseWriter, r *http.Request) {
	var req loginAccountRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if strings.TrimSpace(req.Account) == "" || req.Password == "" {
		httpx.Error(w, http.StatusBadRequest, "invalid_request", "账号和密码不能为空")
		return
	}
	if s.cfg.CasdoorEndpoint == "" || s.cfg.CasdoorClientID == "" || s.cfg.CasdoorClientSecret == "" {
		httpx.Error(w, http.StatusInternalServerError, "casdoor_not_configured", "认证服务未配置完整")
		return
	}

	redirectURI := strings.TrimRight(s.cfg.PublicWebURL, "/") + "/auth/callback"
	if s.cfg.AppEnv == "local" {
		if err := s.store.EnsureLocalCasdoorOAuth(
			r.Context(),
			s.cfg.CasdoorOrganizationName,
			s.cfg.CasdoorApplicationName,
			s.cfg.CasdoorClientID,
			s.cfg.CasdoorClientSecret,
			redirectURI,
		); err != nil {
			httpx.Error(w, http.StatusInternalServerError, "local_casdoor_bootstrap_failed", err.Error())
			return
		}
	}

	state, err := randomState()
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	code, err := s.casdoorPasswordLogin(r, req, redirectURI, state)
	if err != nil {
		httpx.Error(w, http.StatusUnauthorized, "invalid_credentials", "账号或密码不正确")
		return
	}
	token, err := casdoorsdk.GetOAuthToken(code, state)
	if err != nil {
		httpx.Error(w, http.StatusBadGateway, "casdoor_token_exchange_failed", err.Error())
		return
	}
	if token.AccessToken == "" {
		httpx.Error(w, http.StatusBadGateway, "casdoor_token_exchange_failed", "access token is empty")
		return
	}
	s.setSessionCookie(w, token.AccessToken, token.Expiry)
	redirectTo := s.safeReturnTo(req.ReturnTo)
	if user, err := s.syncUserFromAccessToken(r, token.AccessToken); err == nil {
		redirectTo = s.withLoginCode(r, redirectTo, user)
	}
	httpx.JSON(w, http.StatusOK, loginAccountResponse{Status: "ok", RedirectTo: redirectTo})
}

func (s *Server) casdoorPasswordLogin(r *http.Request, req loginAccountRequest, redirectURI string, state string) (string, error) {
	candidates, err := s.loginCandidates(r.Context(), req.Account)
	if err != nil {
		return "", err
	}
	var lastErr error
	for _, candidate := range candidates {
		code, err := s.tryCasdoorPasswordLogin(r, candidate, req.Password, redirectURI, state)
		if err == nil {
			return code, nil
		}
		lastErr = err
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("no login candidates")
	}
	return "", lastErr
}

func (s *Server) loginCandidates(ctx context.Context, account string) ([]loginCandidate, error) {
	account = strings.TrimSpace(account)
	if account == "" {
		return nil, fmt.Errorf("account is required")
	}
	if normalized, _, ok := normalizeMainlandPhoneValue("+86", account); ok {
		if user, err := s.casdoorUserByPhone(normalized); err == nil && user != nil && strings.TrimSpace(user.Name) != "" {
			owner := strings.TrimSpace(user.Owner)
			if owner == "" {
				owner = s.cfg.CasdoorOrganizationName
			}
			return []loginCandidate{{organization: owner, username: user.Name}}, nil
		}
		if user, err := s.store.FindUserByPhone(ctx, normalized); err == nil && strings.TrimSpace(user.CasdoorUserID) != "" {
			owner := strings.TrimSpace(user.Owner)
			if owner == "" {
				owner = s.cfg.CasdoorOrganizationName
			}
			return []loginCandidate{{organization: owner, username: user.CasdoorUserID}}, nil
		}
		return nil, fmt.Errorf("phone user not found")
	}
	if regexpPhoneInput.MatchString(account) {
		return nil, fmt.Errorf("invalid phone")
	}
	candidates := []loginCandidate{{organization: s.cfg.CasdoorOrganizationName, username: account}}
	if !strings.EqualFold(s.cfg.CasdoorOrganizationName, "built-in") {
		candidates = append(candidates, loginCandidate{organization: "built-in", username: account})
	}
	return candidates, nil
}

func (s *Server) tryCasdoorPasswordLogin(r *http.Request, candidate loginCandidate, password string, redirectURI string, state string) (string, error) {
	loginURL, err := url.Parse(strings.TrimRight(s.cfg.CasdoorEndpoint, "/") + "/api/login")
	if err != nil {
		return "", err
	}
	query := loginURL.Query()
	query.Set("clientId", s.cfg.CasdoorClientID)
	query.Set("responseType", "code")
	query.Set("redirectUri", redirectURI)
	query.Set("type", "code")
	query.Set("scope", "read")
	query.Set("state", state)
	query.Set("nonce", "")
	query.Set("code_challenge_method", "")
	query.Set("code_challenge", "")
	loginURL.RawQuery = query.Encode()

	body, err := json.Marshal(map[string]string{
		"username":     candidate.username,
		"password":     password,
		"organization": candidate.organization,
		"application":  s.cfg.CasdoorApplicationName,
		"signinMethod": "Password",
		"type":         "code",
	})
	if err != nil {
		return "", err
	}
	httpReq, err := http.NewRequestWithContext(r.Context(), http.MethodPost, loginURL.String(), bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept-Language", r.Header.Get("Accept-Language"))
	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("casdoor login failed: %s", resp.Status)
	}
	var loginResp casdoorPasswordLoginResponse
	if err := json.Unmarshal(respBody, &loginResp); err != nil {
		return "", err
	}
	if loginResp.Status != "ok" || strings.TrimSpace(loginResp.Data) == "" {
		if loginResp.Msg != "" {
			return "", fmt.Errorf("%s", loginResp.Msg)
		}
		return "", fmt.Errorf("casdoor login failed")
	}
	return loginResp.Data, nil
}

func (s *Server) setSessionCookie(w http.ResponseWriter, accessToken string, expiry time.Time) {
	maxAge := int(time.Until(expiry).Seconds())
	if maxAge <= 0 {
		maxAge = 24 * 60 * 60
	}
	http.SetCookie(w, &http.Cookie{
		Name:     auth.SessionCookieName,
		Value:    accessToken,
		Path:     "/",
		MaxAge:   maxAge,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   s.secureCookies(),
	})
}

func (s *Server) exchangeSession(w http.ResponseWriter, r *http.Request) {
	var req sessionExchangeRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	code := strings.TrimSpace(req.Code)
	if code == "" {
		httpx.Error(w, http.StatusBadRequest, "invalid_request", "登录码不能为空")
		return
	}
	user, err := s.store.ConsumeLoginCode(r.Context(), code, time.Now())
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid_login_code", "登录码无效或已过期")
		return
	}
	principal := principalFromUserProfile(user)
	token, expiry, err := s.auth.NewSessionToken(principal, 7*24*time.Hour)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "session_create_failed", err.Error())
		return
	}
	s.setSessionCookie(w, token, expiry)
	httpx.JSON(w, http.StatusOK, map[string]any{
		"access_token": token,
		"token_type":   "Bearer",
		"expires_at":   expiry,
		"user":         user,
	})
}

func (s *Server) logout(w http.ResponseWriter, r *http.Request) {
	clearCookie(w, auth.SessionCookieName, s.secureCookies())
	clearCookie(w, oauthStateCookieName, s.secureCookies())
	clearCookie(w, oauthReturnToCookieName, s.secureCookies())
	httpx.JSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) syncUserFromAccessToken(r *http.Request, accessToken string) (UserProfile, error) {
	claims, err := casdoorsdk.ParseJwtToken(accessToken)
	if err != nil {
		return UserProfile{}, err
	}
	userID := strings.TrimSpace(claims.Name)
	if userID == "" {
		userID = strings.TrimSpace(claims.Id)
	}
	return s.store.SyncUser(r.Context(), auth.Principal{
		ID:          userID,
		Name:        claims.Name,
		DisplayName: claims.DisplayName,
		Email:       claims.Email,
		Phone:       claims.Phone,
		Owner:       claims.Owner,
		IsAdmin:     claims.IsAdmin,
		Source:      auth.PrincipalSourceCasdoorJWT,
	})
}

func (s *Server) withLoginCode(r *http.Request, returnTo string, user UserProfile) string {
	if !s.isExternalReturnTo(returnTo) {
		return returnTo
	}
	code, err := s.store.CreateLoginCode(r.Context(), user.ID, returnTo, time.Now())
	if err != nil {
		return "/center"
	}
	parsed, err := url.Parse(returnTo)
	if err != nil {
		return "/center"
	}
	query := parsed.Query()
	query.Set("koffy_login_code", code)
	parsed.RawQuery = query.Encode()
	return parsed.String()
}

func (s *Server) withLoginExchangeCode(r *http.Request, returnTo string, user UserProfile) string {
	if s.isExternalReturnTo(returnTo) {
		return s.withLoginCode(r, returnTo, user)
	}
	code, err := s.store.CreateLoginCode(r.Context(), user.ID, returnTo, time.Now())
	loginTarget := "/login?return_to=" + url.QueryEscape(returnTo)
	if err != nil {
		return withWeChatError(loginTarget, "session_create_failed", "登录状态创建失败，请稍后重试")
	}
	parsed := &url.URL{Path: "/login"}
	query := parsed.Query()
	query.Set("return_to", returnTo)
	query.Set("koffy_login_code", code)
	parsed.RawQuery = query.Encode()
	return parsed.String()
}

func principalFromUserProfile(user UserProfile) auth.Principal {
	return auth.Principal{
		ID:          user.CasdoorUserID,
		Name:        user.Name,
		DisplayName: user.DisplayName,
		Email:       user.Email,
		Phone:       user.Phone,
		Owner:       user.Owner,
		IsAdmin:     user.IsAdmin,
	}
}

func (s *Server) secureCookies() bool {
	return strings.HasPrefix(strings.ToLower(s.cfg.PublicWebURL), "https://")
}

func clearCookie(w http.ResponseWriter, name string, secure bool) {
	http.SetCookie(w, &http.Cookie{
		Name:     name,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   secure,
	})
}

func randomState() (string, error) {
	var raw [16]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", fmt.Errorf("generate oauth state: %w", err)
	}
	return hex.EncodeToString(raw[:]), nil
}

func (s *Server) safeReturnTo(value string) string {
	value = strings.TrimSpace(value)
	if value == "" || strings.ContainsAny(value, "\r\n") {
		return "/center"
	}
	if strings.HasPrefix(value, "/") && !strings.HasPrefix(value, "//") {
		return value
	}
	parsed, err := url.Parse(value)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return "/center"
	}
	if parsed.Scheme != "https" && !(s.cfg.AppEnv == "local" && parsed.Scheme == "http") {
		return "/center"
	}
	for _, origin := range s.allowedReturnOrigins() {
		if strings.EqualFold(origin, parsed.Scheme+"://"+parsed.Host) {
			return value
		}
	}
	return "/center"
}

func (s *Server) allowedReturnOrigins() []string {
	origins := make([]string, 0, len(s.cfg.AuthAllowedReturnOrigins)+1)
	if publicURL, err := url.Parse(strings.TrimRight(s.cfg.PublicWebURL, "/")); err == nil && publicURL.Scheme != "" && publicURL.Host != "" {
		origins = append(origins, publicURL.Scheme+"://"+publicURL.Host)
	}
	origins = append(origins, s.cfg.AuthAllowedReturnOrigins...)
	return origins
}

func (s *Server) isExternalReturnTo(value string) bool {
	parsed, err := url.Parse(value)
	return err == nil && parsed.Scheme != "" && parsed.Host != ""
}
