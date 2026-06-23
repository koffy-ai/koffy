package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

type Config struct {
	AppEnv   string
	LogLevel string

	PublicWebURL     string
	PublicGatewayURL string
	PublicCasdoorURL string

	MySQLHost     string
	MySQLPort     string
	MySQLDatabase string
	MySQLUser     string
	MySQLPassword string

	RedisAddr     string
	RedisPassword string
	RedisDB       int

	CasdoorEndpoint          string
	CasdoorClientID          string
	CasdoorClientSecret      string
	CasdoorCertificate       string
	CasdoorOrganizationName  string
	CasdoorApplicationName   string
	RegistrationSMSProvider  string
	AuthAllowedReturnOrigins []string

	CaptchaEnabled   bool
	CaptchaProvider  string
	CaptchaSiteKey   string
	CaptchaSecret    string
	CaptchaVerifyURL string

	TencentCloudSecretID       string
	TencentCloudSecretKey      string
	TencentCaptchaAppID        string
	TencentCaptchaAppSecretKey string

	WeChatOfficialAppID     string
	WeChatOfficialAppSecret string
	WeChatWebsiteAppID      string
	WeChatWebsiteAppSecret  string

	BillingAPIAddr        string
	BillingAPIURL         string
	BillingInternalAPIKey string
	CoinExchangeRateCNY   int64

	AIGatewayAddr        string
	LiteLLMBaseURL       string
	LiteLLMMasterKey     string
	DefaultPreauthTokens int64
	AIGatewayAppRPM      int
	AIGatewayUserRPM     int

	EntitlementMaintenanceIntervalMinutes int

	OpenAIAPIKey string

	WeChatPayEnabled         bool
	WeChatPayMchID           string
	WeChatPayAppID           string
	WeChatPayAPIV3Key        string
	WeChatPayMchCertSerialNo string
	WeChatPayPrivateKeyPath  string
	WeChatPayPublicKeyID     string
	WeChatPayPublicKeyPath   string
	WeChatPayNotifyURL       string
}

func Load() Config {
	appEnv := env("APP_ENV", "local")
	return Config{
		AppEnv:   appEnv,
		LogLevel: env("LOG_LEVEL", "info"),

		PublicWebURL:     env("PUBLIC_WEB_URL", "http://localhost:3000"),
		PublicGatewayURL: env("PUBLIC_GATEWAY_URL", "http://localhost:8081"),
		PublicCasdoorURL: env("PUBLIC_CASDOOR_URL", "http://localhost:8000"),

		MySQLHost:     env("MYSQL_HOST", "mysql"),
		MySQLPort:     env("MYSQL_PORT", "3306"),
		MySQLDatabase: env("MYSQL_DATABASE", "koffy"),
		MySQLUser:     env("MYSQL_USER", "koffy_user"),
		MySQLPassword: env("MYSQL_PASSWORD", ""),

		RedisAddr:     env("REDIS_ADDR", "redis:6379"),
		RedisPassword: env("REDIS_PASSWORD", ""),
		RedisDB:       envInt("REDIS_DB", 0),

		CasdoorEndpoint:          env("CASDOOR_ENDPOINT", "http://localhost:8000"),
		CasdoorClientID:          env("CASDOOR_CLIENT_ID", ""),
		CasdoorClientSecret:      env("CASDOOR_CLIENT_SECRET", ""),
		CasdoorCertificate:       envMultiline("CASDOOR_CERTIFICATE", ""),
		CasdoorOrganizationName:  env("CASDOOR_ORGANIZATION_NAME", "koffy"),
		CasdoorApplicationName:   env("CASDOOR_APPLICATION_NAME", "app-built-in"),
		RegistrationSMSProvider:  env("REGISTRATION_SMS_PROVIDER", ""),
		AuthAllowedReturnOrigins: envList("AUTH_ALLOWED_RETURN_ORIGINS", ""),

		CaptchaEnabled:   envBool("CAPTCHA_ENABLED", true),
		CaptchaProvider:  env("CAPTCHA_PROVIDER", "none"),
		CaptchaSiteKey:   env("CAPTCHA_SITE_KEY", ""),
		CaptchaSecret:    env("CAPTCHA_SECRET", ""),
		CaptchaVerifyURL: env("CAPTCHA_VERIFY_URL", ""),

		TencentCloudSecretID:       env("TENCENT_CLOUD_SECRET_ID", ""),
		TencentCloudSecretKey:      env("TENCENT_CLOUD_SECRET_KEY", ""),
		TencentCaptchaAppID:        env("TENCENT_CAPTCHA_APP_ID", ""),
		TencentCaptchaAppSecretKey: env("TENCENT_CAPTCHA_APP_SECRET_KEY", ""),

		WeChatOfficialAppID:     env("WECHAT_OFFICIAL_APP_ID", ""),
		WeChatOfficialAppSecret: env("WECHAT_OFFICIAL_APP_SECRET", ""),
		WeChatWebsiteAppID:      env("WECHAT_WEBSITE_APP_ID", ""),
		WeChatWebsiteAppSecret:  env("WECHAT_WEBSITE_APP_SECRET", ""),

		BillingAPIAddr:        env("BILLING_API_ADDR", ":8080"),
		BillingAPIURL:         env("BILLING_API_URL", "http://localhost:8080"),
		BillingInternalAPIKey: env("BILLING_INTERNAL_API_KEY", ""),
		CoinExchangeRateCNY:   envInt64("COIN_EXCHANGE_RATE_CNY", 100),

		AIGatewayAddr:        env("AI_GATEWAY_ADDR", ":8081"),
		LiteLLMBaseURL:       env("LITELLM_BASE_URL", "http://litellm:4000"),
		LiteLLMMasterKey:     env("LITELLM_MASTER_KEY", ""),
		DefaultPreauthTokens: envInt64("AI_GATEWAY_DEFAULT_PREAUTH_TOKENS", 2000),
		AIGatewayAppRPM:      envInt("AI_GATEWAY_APP_RATE_LIMIT_PER_MINUTE", 600),
		AIGatewayUserRPM:     envInt("AI_GATEWAY_USER_RATE_LIMIT_PER_MINUTE", 120),

		EntitlementMaintenanceIntervalMinutes: envInt("ENTITLEMENT_MAINTENANCE_INTERVAL_MINUTES", 60),

		OpenAIAPIKey: env("OPENAI_API_KEY", ""),

		WeChatPayEnabled:         envBool("WECHAT_PAY_ENABLED", strings.EqualFold(appEnv, "local")),
		WeChatPayMchID:           env("WECHAT_PAY_MCH_ID", ""),
		WeChatPayAppID:           env("WECHAT_PAY_APP_ID", ""),
		WeChatPayAPIV3Key:        env("WECHAT_PAY_API_V3_KEY", ""),
		WeChatPayMchCertSerialNo: env("WECHAT_PAY_MCH_CERT_SERIAL_NO", ""),
		WeChatPayPrivateKeyPath:  env("WECHAT_PAY_PRIVATE_KEY_PATH", ""),
		WeChatPayPublicKeyID:     env("WECHAT_PAY_PUBLIC_KEY_ID", ""),
		WeChatPayPublicKeyPath:   env("WECHAT_PAY_PUBLIC_KEY_PATH", ""),
		WeChatPayNotifyURL:       env("WECHAT_PAY_NOTIFY_URL", ""),
	}
}

func (c Config) Validate(service string) error {
	if !strings.EqualFold(c.AppEnv, "production") {
		return nil
	}

	required := map[string]string{
		"MYSQL_HOST":                               c.MySQLHost,
		"MYSQL_DATABASE":                           c.MySQLDatabase,
		"MYSQL_USER":                               c.MySQLUser,
		"MYSQL_PASSWORD":                           c.MySQLPassword,
		"CASDOOR_ENDPOINT":                         c.CasdoorEndpoint,
		"CASDOOR_CLIENT_ID":                        c.CasdoorClientID,
		"CASDOOR_CLIENT_SECRET":                    c.CasdoorClientSecret,
		"CASDOOR_CERTIFICATE":                      c.CasdoorCertificate,
		"CASDOOR_ORGANIZATION_NAME":                c.CasdoorOrganizationName,
		"CASDOOR_APPLICATION_NAME":                 c.CasdoorApplicationName,
		"PUBLIC_WEB_URL":                           c.PublicWebURL,
		"AUTH_ALLOWED_RETURN_ORIGINS":              strings.Join(c.AuthAllowedReturnOrigins, ","),
		"BILLING_INTERNAL_API_KEY":                 c.BillingInternalAPIKey,
		"ENTITLEMENT_MAINTENANCE_INTERVAL_MINUTES": strconv.Itoa(c.EntitlementMaintenanceIntervalMinutes),
	}
	if c.CaptchaEnabled {
		required["CAPTCHA_PROVIDER"] = c.CaptchaProvider
		switch strings.ToLower(strings.TrimSpace(c.CaptchaProvider)) {
		case "none", "":
			required["CAPTCHA_PROVIDER"] = ""
		case "tencent":
			required["TENCENT_CLOUD_SECRET_ID"] = c.TencentCloudSecretID
			required["TENCENT_CLOUD_SECRET_KEY"] = c.TencentCloudSecretKey
			required["TENCENT_CAPTCHA_APP_ID"] = c.TencentCaptchaAppID
			required["TENCENT_CAPTCHA_APP_SECRET_KEY"] = c.TencentCaptchaAppSecretKey
		default:
			required["CAPTCHA_SITE_KEY"] = c.CaptchaSiteKey
			required["CAPTCHA_SECRET"] = c.CaptchaSecret
		}
	}
	switch service {
	case "koffy-billing-api":
		if c.WeChatPayEnabled {
			required["WECHAT_PAY_MCH_ID"] = c.WeChatPayMchID
			required["WECHAT_PAY_APP_ID"] = c.WeChatPayAppID
			required["WECHAT_PAY_API_V3_KEY"] = c.WeChatPayAPIV3Key
			required["WECHAT_PAY_MCH_CERT_SERIAL_NO"] = c.WeChatPayMchCertSerialNo
			required["WECHAT_PAY_PRIVATE_KEY_PATH"] = c.WeChatPayPrivateKeyPath
			required["WECHAT_PAY_NOTIFY_URL"] = c.WeChatPayNotifyURL
			if c.WeChatPayPublicKeyID != "" || c.WeChatPayPublicKeyPath != "" {
				required["WECHAT_PAY_PUBLIC_KEY_ID"] = c.WeChatPayPublicKeyID
				required["WECHAT_PAY_PUBLIC_KEY_PATH"] = c.WeChatPayPublicKeyPath
			}
		}
	case "koffy-gateway":
		required["PUBLIC_GATEWAY_URL"] = c.PublicGatewayURL
		required["BILLING_API_URL"] = c.BillingAPIURL
		required["LITELLM_BASE_URL"] = c.LiteLLMBaseURL
		required["LITELLM_MASTER_KEY"] = c.LiteLLMMasterKey
	}

	missing := make([]string, 0)
	for key, value := range required {
		if invalidProductionValue(value) {
			missing = append(missing, key)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("missing required production environment variables: %s", strings.Join(missing, ", "))
	}
	return nil
}

func invalidProductionValue(value string) bool {
	value = strings.TrimSpace(value)
	lower := strings.ToLower(value)
	return value == "" ||
		strings.Contains(lower, "change-me") ||
		strings.Contains(lower, "replace-me") ||
		strings.Contains(lower, "localhost") ||
		strings.Contains(value, "...")
}

func env(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func envMultiline(key, fallback string) string {
	return strings.ReplaceAll(env(key, fallback), `\n`, "\n")
}

func envBool(key string, fallback bool) bool {
	value := strings.TrimSpace(strings.ToLower(env(key, "")))
	if value == "" {
		return fallback
	}
	return value == "1" || value == "true" || value == "yes" || value == "on"
}

func envInt(key string, fallback int) int {
	value, err := strconv.Atoi(env(key, ""))
	if err != nil {
		return fallback
	}
	return value
}

func envInt64(key string, fallback int64) int64 {
	value, err := strconv.ParseInt(env(key, ""), 10, 64)
	if err != nil {
		return fallback
	}
	return value
}

func envList(key string, fallback string) []string {
	raw := env(key, fallback)
	items := strings.Split(raw, ",")
	values := make([]string, 0, len(items))
	for _, item := range items {
		item = strings.TrimSpace(item)
		if item != "" {
			values = append(values, item)
		}
	}
	return values
}
