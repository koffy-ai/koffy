USE koffy;

CREATE TABLE IF NOT EXISTS users (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  casdoor_user_id VARCHAR(128) NOT NULL,
  casdoor_owner VARCHAR(128) NOT NULL,
  name VARCHAR(128) NOT NULL,
  display_name VARCHAR(255) NOT NULL DEFAULT '',
  display_name_custom BOOLEAN NOT NULL DEFAULT FALSE,
  email VARCHAR(255) NOT NULL DEFAULT '',
  phone VARCHAR(64) NOT NULL DEFAULT '',
  phone_unique VARCHAR(64) GENERATED ALWAYS AS (NULLIF(phone, '')) STORED,
  avatar_url VARCHAR(1024) NOT NULL DEFAULT '',
  avatar_custom BOOLEAN NOT NULL DEFAULT FALSE,
  is_admin BOOLEAN NOT NULL DEFAULT FALSE,
  last_login_at DATETIME(3) NULL,
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  PRIMARY KEY (id),
  UNIQUE KEY uk_users_casdoor_user_id (casdoor_user_id),
  UNIQUE KEY uk_users_name (name),
  UNIQUE KEY uk_users_phone_unique (phone_unique),
  KEY idx_users_email (email)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS user_avatar_assets (
  user_id BIGINT UNSIGNED NOT NULL,
  content_type VARCHAR(128) NOT NULL,
  data LONGBLOB NOT NULL,
  size_bytes INT NOT NULL,
  width INT NOT NULL,
  height INT NOT NULL,
  updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  PRIMARY KEY (user_id),
  CONSTRAINT fk_user_avatar_assets_user FOREIGN KEY (user_id) REFERENCES users(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS wallets (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  user_id BIGINT UNSIGNED NOT NULL,
  balance_coins BIGINT NOT NULL DEFAULT 0,
  reserved_coins BIGINT NOT NULL DEFAULT 0,
  version BIGINT UNSIGNED NOT NULL DEFAULT 0,
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  PRIMARY KEY (id),
  UNIQUE KEY uk_wallets_user_id (user_id),
  CONSTRAINT fk_wallets_user FOREIGN KEY (user_id) REFERENCES users(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS wallet_ledger (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  user_id BIGINT UNSIGNED NOT NULL,
  wallet_id BIGINT UNSIGNED NOT NULL,
  direction ENUM('credit', 'debit', 'reserve', 'release') NOT NULL,
  reason ENUM('recharge', 'usage', 'admin_adjustment', 'reservation', 'reservation_release') NOT NULL,
  amount_coins BIGINT NOT NULL,
  balance_after BIGINT NOT NULL,
  usage_request_id BIGINT UNSIGNED NULL,
  recharge_order_id BIGINT UNSIGNED NULL,
  admin_user_id BIGINT UNSIGNED NULL,
  remark VARCHAR(512) NOT NULL DEFAULT '',
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  PRIMARY KEY (id),
  KEY idx_wallet_ledger_user_created (user_id, created_at),
  KEY idx_wallet_ledger_usage_request (usage_request_id),
  CONSTRAINT fk_wallet_ledger_user FOREIGN KEY (user_id) REFERENCES users(id),
  CONSTRAINT fk_wallet_ledger_wallet FOREIGN KEY (wallet_id) REFERENCES wallets(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS apps (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  app_code VARCHAR(64) NOT NULL,
  name VARCHAR(128) NOT NULL,
  status ENUM('active', 'disabled') NOT NULL DEFAULT 'active',
  billing_mode ENUM('entitlement', 'coins', 'hybrid') NOT NULL DEFAULT 'hybrid',
  description VARCHAR(512) NOT NULL DEFAULT '',
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  PRIMARY KEY (id),
  UNIQUE KEY uk_apps_app_code (app_code)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS app_api_keys (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  app_id BIGINT UNSIGNED NOT NULL,
  key_prefix VARCHAR(32) NOT NULL,
  key_hash CHAR(64) NOT NULL,
  status ENUM('active', 'disabled') NOT NULL DEFAULT 'active',
  last_used_at DATETIME(3) NULL,
  expires_at DATETIME(3) NULL,
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  PRIMARY KEY (id),
  UNIQUE KEY uk_app_api_keys_hash (key_hash),
  KEY idx_app_api_keys_app (app_id),
  CONSTRAINT fk_app_api_keys_app FOREIGN KEY (app_id) REFERENCES apps(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS app_token_pricing (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  app_id BIGINT UNSIGNED NOT NULL,
  model_alias VARCHAR(128) NOT NULL DEFAULT '*',
  token_amount BIGINT NOT NULL,
  coin_amount BIGINT NOT NULL,
  status ENUM('active', 'disabled') NOT NULL DEFAULT 'active',
  effective_from DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  PRIMARY KEY (id),
  UNIQUE KEY uk_app_token_pricing (app_id, model_alias, effective_from),
  CONSTRAINT fk_app_token_pricing_app FOREIGN KEY (app_id) REFERENCES apps(id),
  CHECK (token_amount > 0),
  CHECK (coin_amount > 0)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS app_unit_pricing (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  app_id BIGINT UNSIGNED NOT NULL,
  model_alias VARCHAR(128) NOT NULL DEFAULT '*',
  unit ENUM('images', 'video_seconds', 'business_units') NOT NULL,
  unit_amount BIGINT NOT NULL,
  coin_amount BIGINT NOT NULL,
  status ENUM('active', 'disabled') NOT NULL DEFAULT 'active',
  effective_from DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  PRIMARY KEY (id),
  UNIQUE KEY uk_app_unit_pricing (app_id, model_alias, unit, effective_from),
  KEY idx_app_unit_pricing_lookup (app_id, unit, model_alias, status, effective_from),
  CONSTRAINT fk_app_unit_pricing_app FOREIGN KEY (app_id) REFERENCES apps(id),
  CHECK (unit_amount > 0),
  CHECK (coin_amount > 0)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS plans (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  app_id BIGINT UNSIGNED NOT NULL,
  plan_code VARCHAR(64) NOT NULL,
  name VARCHAR(128) NOT NULL,
  period ENUM('monthly', 'yearly') NOT NULL,
  price_cents BIGINT NOT NULL,
  status ENUM('active', 'disabled') NOT NULL DEFAULT 'active',
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  PRIMARY KEY (id),
  UNIQUE KEY uk_plans_app_plan_code (app_id, plan_code),
  CONSTRAINT fk_plans_app FOREIGN KEY (app_id) REFERENCES apps(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS plan_entitlements (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  plan_id BIGINT UNSIGNED NOT NULL,
  entitlement_code VARCHAR(64) NOT NULL,
  name VARCHAR(128) NOT NULL,
  unit ENUM('tokens', 'images', 'video_seconds', 'business_units') NOT NULL,
  monthly_quota BIGINT NOT NULL,
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  PRIMARY KEY (id),
  UNIQUE KEY uk_plan_entitlements_code (plan_id, entitlement_code),
  CONSTRAINT fk_plan_entitlements_plan FOREIGN KEY (plan_id) REFERENCES plans(id),
  CHECK (monthly_quota >= 0)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS user_subscriptions (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  user_id BIGINT UNSIGNED NOT NULL,
  app_id BIGINT UNSIGNED NOT NULL,
  plan_id BIGINT UNSIGNED NOT NULL,
  status ENUM('active', 'expired', 'cancelled') NOT NULL DEFAULT 'active',
  starts_at DATETIME(3) NOT NULL,
  ends_at DATETIME(3) NOT NULL,
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  PRIMARY KEY (id),
  KEY idx_user_subscriptions_user_app (user_id, app_id, status),
  CONSTRAINT fk_user_subscriptions_user FOREIGN KEY (user_id) REFERENCES users(id),
  CONSTRAINT fk_user_subscriptions_app FOREIGN KEY (app_id) REFERENCES apps(id),
  CONSTRAINT fk_user_subscriptions_plan FOREIGN KEY (plan_id) REFERENCES plans(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS entitlement_balances (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  user_id BIGINT UNSIGNED NOT NULL,
  app_id BIGINT UNSIGNED NOT NULL,
  subscription_id BIGINT UNSIGNED NOT NULL,
  entitlement_code VARCHAR(64) NOT NULL,
  unit ENUM('tokens', 'images', 'video_seconds', 'business_units') NOT NULL,
  period_month CHAR(7) NOT NULL,
  quota BIGINT NOT NULL,
  used BIGINT NOT NULL DEFAULT 0,
  reserved BIGINT NOT NULL DEFAULT 0,
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  PRIMARY KEY (id),
  UNIQUE KEY uk_entitlement_balance_period (subscription_id, entitlement_code, period_month),
  KEY idx_entitlement_balances_user_app (user_id, app_id, period_month),
  CONSTRAINT fk_entitlement_balances_user FOREIGN KEY (user_id) REFERENCES users(id),
  CONSTRAINT fk_entitlement_balances_app FOREIGN KEY (app_id) REFERENCES apps(id),
  CONSTRAINT fk_entitlement_balances_subscription FOREIGN KEY (subscription_id) REFERENCES user_subscriptions(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS entitlement_ledger (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  entitlement_balance_id BIGINT UNSIGNED NOT NULL,
  user_id BIGINT UNSIGNED NOT NULL,
  app_id BIGINT UNSIGNED NOT NULL,
  direction ENUM('consume', 'reserve', 'release', 'reset') NOT NULL,
  amount BIGINT NOT NULL,
  used_after BIGINT NOT NULL,
  reserved_after BIGINT NOT NULL,
  usage_request_id BIGINT UNSIGNED NULL,
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  PRIMARY KEY (id),
  KEY idx_entitlement_ledger_user_created (user_id, created_at),
  CONSTRAINT fk_entitlement_ledger_balance FOREIGN KEY (entitlement_balance_id) REFERENCES entitlement_balances(id),
  CONSTRAINT fk_entitlement_ledger_user FOREIGN KEY (user_id) REFERENCES users(id),
  CONSTRAINT fk_entitlement_ledger_app FOREIGN KEY (app_id) REFERENCES apps(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS usage_requests (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  app_id BIGINT UNSIGNED NOT NULL,
  user_id BIGINT UNSIGNED NOT NULL,
  idempotency_key VARCHAR(128) NOT NULL,
  status ENUM('authorized', 'committed', 'cancelled', 'failed') NOT NULL,
  billing_mode ENUM('entitlement', 'coins', 'hybrid') NOT NULL,
  model_alias VARCHAR(128) NOT NULL DEFAULT '',
  provider VARCHAR(64) NOT NULL DEFAULT '',
  provider_model VARCHAR(128) NOT NULL DEFAULT '',
  provider_job_id VARCHAR(128) NOT NULL DEFAULT '',
  estimated_total_tokens BIGINT NOT NULL DEFAULT 0,
  estimated_images BIGINT NOT NULL DEFAULT 0,
  estimated_video_seconds BIGINT NOT NULL DEFAULT 0,
  estimated_business_units BIGINT NOT NULL DEFAULT 0,
  actual_prompt_tokens BIGINT NOT NULL DEFAULT 0,
  actual_completion_tokens BIGINT NOT NULL DEFAULT 0,
  actual_total_tokens BIGINT NOT NULL DEFAULT 0,
  actual_images BIGINT NOT NULL DEFAULT 0,
  actual_video_seconds BIGINT NOT NULL DEFAULT 0,
  actual_business_units BIGINT NOT NULL DEFAULT 0,
  reserved_coins BIGINT NOT NULL DEFAULT 0,
  charged_coins BIGINT NOT NULL DEFAULT 0,
  reserved_units BIGINT NOT NULL DEFAULT 0,
  charged_units BIGINT NOT NULL DEFAULT 0,
  error_code VARCHAR(64) NOT NULL DEFAULT '',
  error_message VARCHAR(512) NOT NULL DEFAULT '',
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  authorized_at DATETIME(3) NULL,
  committed_at DATETIME(3) NULL,
  cancelled_at DATETIME(3) NULL,
  PRIMARY KEY (id),
  UNIQUE KEY uk_usage_requests_idempotency (app_id, user_id, idempotency_key),
  KEY idx_usage_requests_user_created (user_id, created_at),
  KEY idx_usage_requests_app_created (app_id, created_at),
  CONSTRAINT fk_usage_requests_app FOREIGN KEY (app_id) REFERENCES apps(id),
  CONSTRAINT fk_usage_requests_user FOREIGN KEY (user_id) REFERENCES users(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS usage_reservations (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  usage_request_id BIGINT UNSIGNED NOT NULL,
  reservation_type ENUM('coins', 'entitlement') NOT NULL,
  wallet_id BIGINT UNSIGNED NULL,
  entitlement_balance_id BIGINT UNSIGNED NULL,
  amount BIGINT NOT NULL,
  consumed_amount BIGINT NOT NULL DEFAULT 0,
  released_amount BIGINT NOT NULL DEFAULT 0,
  status ENUM('reserved', 'consumed', 'released') NOT NULL DEFAULT 'reserved',
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  PRIMARY KEY (id),
  KEY idx_usage_reservations_usage (usage_request_id),
  KEY idx_usage_reservations_wallet (wallet_id),
  KEY idx_usage_reservations_entitlement (entitlement_balance_id),
  CONSTRAINT fk_usage_reservations_usage FOREIGN KEY (usage_request_id) REFERENCES usage_requests(id),
  CONSTRAINT fk_usage_reservations_wallet FOREIGN KEY (wallet_id) REFERENCES wallets(id),
  CONSTRAINT fk_usage_reservations_entitlement FOREIGN KEY (entitlement_balance_id) REFERENCES entitlement_balances(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS recharge_orders (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  order_no VARCHAR(64) NOT NULL,
  user_id BIGINT UNSIGNED NOT NULL,
  provider ENUM('wechat') NOT NULL DEFAULT 'wechat',
  amount_cents BIGINT NOT NULL,
  coins BIGINT NOT NULL,
  status ENUM('pending', 'paid', 'closed', 'failed') NOT NULL DEFAULT 'pending',
  provider_trade_no VARCHAR(128) NOT NULL DEFAULT '',
  paid_at DATETIME(3) NULL,
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  PRIMARY KEY (id),
  UNIQUE KEY uk_recharge_orders_order_no (order_no),
  KEY idx_recharge_orders_user_created (user_id, created_at),
  CONSTRAINT fk_recharge_orders_user FOREIGN KEY (user_id) REFERENCES users(id),
  CHECK (amount_cents > 0),
  CHECK (coins > 0)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS payment_events (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  provider ENUM('wechat') NOT NULL DEFAULT 'wechat',
  event_id VARCHAR(128) NOT NULL,
  event_type VARCHAR(128) NOT NULL,
  order_no VARCHAR(64) NOT NULL DEFAULT '',
  provider_trade_no VARCHAR(128) NOT NULL DEFAULT '',
  payload_json JSON NOT NULL,
  processed_at DATETIME(3) NULL,
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  PRIMARY KEY (id),
  UNIQUE KEY uk_payment_events_provider_event (provider, event_id),
  KEY idx_payment_events_order_no (order_no)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS ai_providers (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  provider_code VARCHAR(64) NOT NULL,
  name VARCHAR(128) NOT NULL,
  status ENUM('active', 'disabled') NOT NULL DEFAULT 'active',
  base_url VARCHAR(512) NOT NULL DEFAULT '',
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  PRIMARY KEY (id),
  UNIQUE KEY uk_ai_providers_code (provider_code)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS ai_models (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  provider_id BIGINT UNSIGNED NOT NULL,
  model_alias VARCHAR(128) NOT NULL,
  provider_model VARCHAR(128) NOT NULL,
  capability ENUM('chat', 'image', 'audio', 'video', 'embedding') NOT NULL,
  status ENUM('active', 'disabled') NOT NULL DEFAULT 'active',
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  PRIMARY KEY (id),
  UNIQUE KEY uk_ai_models_alias (model_alias),
  KEY idx_ai_models_provider (provider_id),
  CONSTRAINT fk_ai_models_provider FOREIGN KEY (provider_id) REFERENCES ai_providers(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS app_model_routes (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  app_id BIGINT UNSIGNED NOT NULL,
  model_id BIGINT UNSIGNED NOT NULL,
  status ENUM('active', 'disabled') NOT NULL DEFAULT 'active',
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  PRIMARY KEY (id),
  UNIQUE KEY uk_app_model_routes (app_id, model_id),
  CONSTRAINT fk_app_model_routes_app FOREIGN KEY (app_id) REFERENCES apps(id),
  CONSTRAINT fk_app_model_routes_model FOREIGN KEY (model_id) REFERENCES ai_models(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS audit_logs (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  actor_user_id BIGINT UNSIGNED NULL,
  action VARCHAR(128) NOT NULL,
  target_type VARCHAR(64) NOT NULL,
  target_id VARCHAR(128) NOT NULL,
  ip VARCHAR(64) NOT NULL DEFAULT '',
  user_agent VARCHAR(512) NOT NULL DEFAULT '',
  detail_json JSON NULL,
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  PRIMARY KEY (id),
  KEY idx_audit_logs_actor_created (actor_user_id, created_at),
  KEY idx_audit_logs_target (target_type, target_id),
  CONSTRAINT fk_audit_logs_actor FOREIGN KEY (actor_user_id) REFERENCES users(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS branding_assets (
  asset_key VARCHAR(64) NOT NULL,
  content_type VARCHAR(64) NOT NULL,
  data MEDIUMBLOB NOT NULL,
  size_bytes INT UNSIGNED NOT NULL,
  width INT UNSIGNED NOT NULL,
  height INT UNSIGNED NOT NULL,
  original_filename VARCHAR(255) NULL,
  updated_by_user_id BIGINT UNSIGNED NULL,
  created_at TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  updated_at TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  PRIMARY KEY (asset_key),
  CONSTRAINT fk_branding_assets_updated_by FOREIGN KEY (updated_by_user_id) REFERENCES users(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS phone_verification_codes (
  id BIGINT NOT NULL AUTO_INCREMENT,
  phone VARCHAR(32) NOT NULL,
  purpose VARCHAR(32) NOT NULL,
  code_hash CHAR(64) NOT NULL,
  salt VARCHAR(64) NOT NULL,
  attempts INT NOT NULL DEFAULT 0,
  expires_at DATETIME(3) NOT NULL,
  consumed_at DATETIME(3) NULL,
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  PRIMARY KEY (id),
  KEY idx_phone_verification_lookup (phone, purpose, consumed_at, expires_at),
  KEY idx_phone_verification_created_at (created_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS auth_identities (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  user_id BIGINT UNSIGNED NOT NULL,
  provider ENUM('wechat') NOT NULL,
  app_type ENUM('official', 'website') NOT NULL,
  openid VARCHAR(128) NOT NULL,
  unionid VARCHAR(128) NOT NULL DEFAULT '',
  nickname VARCHAR(255) NOT NULL DEFAULT '',
  avatar_url VARCHAR(1024) NOT NULL DEFAULT '',
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  PRIMARY KEY (id),
  UNIQUE KEY uk_auth_identity_provider_openid (provider, app_type, openid),
  KEY idx_auth_identity_unionid (provider, unionid),
  KEY idx_auth_identity_user (user_id),
  CONSTRAINT fk_auth_identities_user FOREIGN KEY (user_id) REFERENCES users(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS auth_states (
  state CHAR(32) NOT NULL,
  provider ENUM('wechat') NOT NULL,
  app_type ENUM('official', 'website') NOT NULL,
  action ENUM('login', 'bind', 'pay') NOT NULL,
  return_to VARCHAR(1024) NOT NULL DEFAULT '/center',
  user_id BIGINT UNSIGNED NULL,
  expires_at DATETIME(3) NOT NULL,
  consumed_at DATETIME(3) NULL,
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  PRIMARY KEY (state),
  KEY idx_auth_states_expiry (expires_at),
  CONSTRAINT fk_auth_states_user FOREIGN KEY (user_id) REFERENCES users(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS auth_login_codes (
  code CHAR(32) NOT NULL,
  user_id BIGINT UNSIGNED NOT NULL,
  return_to VARCHAR(1024) NOT NULL,
  expires_at DATETIME(3) NOT NULL,
  consumed_at DATETIME(3) NULL,
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  PRIMARY KEY (code),
  KEY idx_auth_login_codes_expiry (expires_at),
  KEY idx_auth_login_codes_user (user_id),
  CONSTRAINT fk_auth_login_codes_user FOREIGN KEY (user_id) REFERENCES users(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS wechat_pay_openids (
  code CHAR(32) NOT NULL,
  user_id BIGINT UNSIGNED NOT NULL,
  openid VARCHAR(128) NOT NULL,
  expires_at DATETIME(3) NOT NULL,
  consumed_at DATETIME(3) NULL,
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  PRIMARY KEY (code),
  KEY idx_wechat_pay_openids_user_created (user_id, created_at),
  KEY idx_wechat_pay_openids_expiry (expires_at),
  CONSTRAINT fk_wechat_pay_openids_user FOREIGN KEY (user_id) REFERENCES users(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
