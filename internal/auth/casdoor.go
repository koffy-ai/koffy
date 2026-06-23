package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"koffy/internal/config"

	"github.com/casdoor/casdoor-go-sdk/casdoorsdk"
)

var (
	ErrMissingToken   = errors.New("missing bearer token")
	ErrInvalidToken   = errors.New("invalid bearer token")
	ErrEmptyUserClaim = errors.New("empty user claim")
)

const SessionCookieName = "billing_session"
const koffySessionPrefix = "koffy1"

const (
	PrincipalSourceCasdoorJWT   = "casdoor_jwt"
	PrincipalSourceKoffySession = "koffy_session"
	PrincipalSourceLocalHeader  = "local_header"
	PrincipalSourceLocalJWT     = "local_jwt"
)

type Principal struct {
	ID          string
	Name        string
	DisplayName string
	Email       string
	Phone       string
	Owner       string
	IsAdmin     bool
	Source      string
}

type Authenticator struct {
	cfg config.Config
}

func NewAuthenticator(cfg config.Config) *Authenticator {
	casdoorsdk.InitConfig(
		cfg.CasdoorEndpoint,
		cfg.CasdoorClientID,
		cfg.CasdoorClientSecret,
		cfg.CasdoorCertificate,
		cfg.CasdoorOrganizationName,
		cfg.CasdoorApplicationName,
	)
	return &Authenticator{cfg: cfg}
}

func (a *Authenticator) PrincipalFromRequest(r *http.Request) (Principal, error) {
	if a.cfg.AppEnv == "local" {
		if userID := strings.TrimSpace(r.Header.Get("X-User-ID")); userID != "" {
			return Principal{
				ID:      userID,
				Name:    userID,
				IsAdmin: strings.EqualFold(strings.TrimSpace(r.Header.Get("X-Admin")), "true"),
				Source:  PrincipalSourceLocalHeader,
			}, nil
		}
	}

	token, err := tokenFromRequest(r)
	if err != nil {
		return Principal{}, err
	}

	claims, err := casdoorsdk.ParseJwtToken(token)
	if err != nil {
		if principal, sessionErr := a.principalFromKoffySession(token); sessionErr == nil {
			return principal, nil
		}
		if a.cfg.AppEnv != "local" {
			return Principal{}, ErrInvalidToken
		}
		principal, localErr := localPrincipalFromJWT(token)
		if localErr == nil {
			principal.Source = PrincipalSourceLocalJWT
		}
		return principal, localErr
	}

	userID := strings.TrimSpace(claims.Name)
	if userID == "" {
		userID = strings.TrimSpace(claims.Id)
	}
	if userID == "" {
		return Principal{}, ErrEmptyUserClaim
	}

	return Principal{
		ID:          userID,
		Name:        claims.Name,
		DisplayName: claims.DisplayName,
		Email:       claims.Email,
		Phone:       claims.Phone,
		Owner:       claims.Owner,
		IsAdmin:     claims.IsAdmin,
		Source:      PrincipalSourceCasdoorJWT,
	}, nil
}

func (a *Authenticator) NewSessionToken(principal Principal, ttl time.Duration) (string, time.Time, error) {
	if strings.TrimSpace(a.cfg.BillingInternalAPIKey) == "" {
		return "", time.Time{}, fmt.Errorf("billing internal api key is required")
	}
	expiry := time.Now().Add(ttl)
	payload := struct {
		ID          string `json:"id"`
		Name        string `json:"name"`
		DisplayName string `json:"display_name"`
		Email       string `json:"email"`
		Phone       string `json:"phone"`
		Owner       string `json:"owner"`
		IsAdmin     bool   `json:"is_admin"`
		Exp         int64  `json:"exp"`
	}{
		ID:          principal.ID,
		Name:        principal.Name,
		DisplayName: principal.DisplayName,
		Email:       principal.Email,
		Phone:       principal.Phone,
		Owner:       principal.Owner,
		IsAdmin:     principal.IsAdmin,
		Exp:         expiry.Unix(),
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return "", time.Time{}, err
	}
	encoded := base64.RawURLEncoding.EncodeToString(raw)
	signature := a.signSession(encoded)
	return koffySessionPrefix + "." + encoded + "." + signature, expiry, nil
}

func (a *Authenticator) principalFromKoffySession(token string) (Principal, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 || parts[0] != koffySessionPrefix {
		return Principal{}, ErrInvalidToken
	}
	if strings.TrimSpace(a.cfg.BillingInternalAPIKey) == "" {
		return Principal{}, ErrInvalidToken
	}
	expected := a.signSession(parts[1])
	if !hmac.Equal([]byte(expected), []byte(parts[2])) {
		return Principal{}, ErrInvalidToken
	}
	raw, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return Principal{}, ErrInvalidToken
	}
	var claims struct {
		ID          string `json:"id"`
		Name        string `json:"name"`
		DisplayName string `json:"display_name"`
		Email       string `json:"email"`
		Phone       string `json:"phone"`
		Owner       string `json:"owner"`
		IsAdmin     bool   `json:"is_admin"`
		Exp         int64  `json:"exp"`
	}
	if err := json.Unmarshal(raw, &claims); err != nil {
		return Principal{}, ErrInvalidToken
	}
	if claims.Exp <= time.Now().Unix() {
		return Principal{}, ErrInvalidToken
	}
	if strings.TrimSpace(claims.ID) == "" {
		return Principal{}, ErrEmptyUserClaim
	}
	return Principal{
		ID:          claims.ID,
		Name:        firstNonEmpty(claims.Name, claims.ID),
		DisplayName: claims.DisplayName,
		Email:       claims.Email,
		Phone:       claims.Phone,
		Owner:       claims.Owner,
		IsAdmin:     claims.IsAdmin,
		Source:      PrincipalSourceKoffySession,
	}, nil
}

func (a *Authenticator) signSession(encodedPayload string) string {
	mac := hmac.New(sha256.New, []byte(a.cfg.BillingInternalAPIKey))
	mac.Write([]byte(koffySessionPrefix))
	mac.Write([]byte("."))
	mac.Write([]byte(encodedPayload))
	mac.Write([]byte("."))
	mac.Write([]byte(strconv.Itoa(len(encodedPayload))))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func localPrincipalFromJWT(token string) (Principal, error) {
	parts := strings.Split(token, ".")
	if len(parts) < 2 {
		return Principal{}, ErrInvalidToken
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return Principal{}, ErrInvalidToken
	}
	var claims struct {
		ID          string `json:"id"`
		Sub         string `json:"sub"`
		Name        string `json:"name"`
		DisplayName string `json:"displayName"`
		Email       string `json:"email"`
		Phone       string `json:"phone"`
		Owner       string `json:"owner"`
		IsAdmin     bool   `json:"isAdmin"`
	}
	if err := json.Unmarshal(payload, &claims); err != nil {
		return Principal{}, fmt.Errorf("%w: %v", ErrInvalidToken, err)
	}
	userID := strings.TrimSpace(claims.Name)
	if userID == "" {
		userID = strings.TrimSpace(claims.ID)
	}
	if userID == "" {
		userID = strings.TrimSpace(claims.Sub)
	}
	if userID == "" {
		return Principal{}, ErrEmptyUserClaim
	}
	return Principal{
		ID:          userID,
		Name:        claims.Name,
		DisplayName: claims.DisplayName,
		Email:       claims.Email,
		Phone:       claims.Phone,
		Owner:       claims.Owner,
		IsAdmin:     claims.IsAdmin,
	}, nil
}

func tokenFromRequest(r *http.Request) (string, error) {
	if token, err := bearerToken(r.Header.Get("Authorization")); err == nil {
		return token, nil
	}
	cookie, err := r.Cookie(SessionCookieName)
	if err != nil {
		return "", ErrMissingToken
	}
	token := strings.TrimSpace(cookie.Value)
	if token == "" {
		return "", ErrMissingToken
	}
	return token, nil
}

func bearerToken(header string) (string, error) {
	parts := strings.Fields(header)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return "", ErrMissingToken
	}
	token := strings.TrimSpace(parts[1])
	if token == "" {
		return "", ErrMissingToken
	}
	return token, nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
