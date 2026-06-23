USE koffy;

INSERT INTO apps (app_code, name, billing_mode, description)
VALUES ('demo-app', 'Demo AI App', 'hybrid', 'Local development demo app')
ON DUPLICATE KEY UPDATE
  name = VALUES(name),
  billing_mode = VALUES(billing_mode),
  description = VALUES(description);

INSERT INTO app_api_keys (app_id, key_prefix, key_hash, status)
SELECT id, 'local', SHA2('local-dev-app-key', 256), 'active'
FROM apps
WHERE app_code = 'demo-app'
ON DUPLICATE KEY UPDATE status = VALUES(status);

INSERT INTO app_token_pricing (app_id, model_alias, token_amount, coin_amount, status)
SELECT a.id, '*', 1000, 1, 'active'
FROM apps a
WHERE a.app_code = 'demo-app'
  AND NOT EXISTS (
    SELECT 1 FROM app_token_pricing p
    WHERE p.app_id = a.id AND p.model_alias = '*' AND p.status = 'active'
  );

INSERT INTO app_unit_pricing (app_id, model_alias, unit, unit_amount, coin_amount, status)
SELECT a.id, '*', 'images', 1, 20, 'active'
FROM apps a
WHERE a.app_code = 'demo-app'
  AND NOT EXISTS (
    SELECT 1 FROM app_unit_pricing p
    WHERE p.app_id = a.id AND p.model_alias = '*' AND p.unit = 'images' AND p.status = 'active'
  );

INSERT INTO app_unit_pricing (app_id, model_alias, unit, unit_amount, coin_amount, status)
SELECT a.id, '*', 'video_seconds', 60, 300, 'active'
FROM apps a
WHERE a.app_code = 'demo-app'
  AND NOT EXISTS (
    SELECT 1 FROM app_unit_pricing p
    WHERE p.app_id = a.id AND p.model_alias = '*' AND p.unit = 'video_seconds' AND p.status = 'active'
  );

INSERT INTO ai_providers (provider_code, name, status, base_url)
VALUES ('openai', 'OpenAI via LiteLLM', 'active', 'https://api.openai.com')
ON DUPLICATE KEY UPDATE name = VALUES(name), status = VALUES(status), base_url = VALUES(base_url);

INSERT INTO ai_models (provider_id, model_alias, provider_model, capability, status)
SELECT id, 'openai-chat-default', 'gpt-4o-mini', 'chat', 'active'
FROM ai_providers
WHERE provider_code = 'openai'
ON DUPLICATE KEY UPDATE provider_model = VALUES(provider_model), capability = VALUES(capability), status = VALUES(status);

INSERT INTO ai_models (provider_id, model_alias, provider_model, capability, status)
SELECT id, 'openai-image-default', 'gpt-image-1', 'image', 'active'
FROM ai_providers
WHERE provider_code = 'openai'
ON DUPLICATE KEY UPDATE provider_model = VALUES(provider_model), capability = VALUES(capability), status = VALUES(status);

INSERT INTO app_model_routes (app_id, model_id, status)
SELECT a.id, m.id, 'active'
FROM apps a
JOIN ai_models m ON m.model_alias = 'openai-chat-default'
WHERE a.app_code = 'demo-app'
ON DUPLICATE KEY UPDATE status = VALUES(status);

INSERT INTO app_model_routes (app_id, model_id, status)
SELECT a.id, m.id, 'active'
FROM apps a
JOIN ai_models m ON m.model_alias = 'openai-image-default'
WHERE a.app_code = 'demo-app'
ON DUPLICATE KEY UPDATE status = VALUES(status);

INSERT INTO users (casdoor_user_id, casdoor_owner, name, display_name)
VALUES ('demo-user', 'built-in', 'demo-user', 'Demo User')
ON DUPLICATE KEY UPDATE display_name = VALUES(display_name);

INSERT INTO wallets (user_id, balance_coins, reserved_coins)
SELECT id, 10000, 0
FROM users
WHERE casdoor_user_id = 'demo-user'
ON DUPLICATE KEY UPDATE balance_coins = VALUES(balance_coins);
