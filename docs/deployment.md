# Deployment

This guide covers the supported local and production layouts for Koffy v0.1.0.

## Requirements

- Docker Engine 24+ and Docker Compose v2
- MySQL 8.4
- Redis 7+ or 8+
- Domain names and TLS certificates for production
- Optional provider accounts for models, SMS, Tencent CAPTCHA, WeChat login, and WeChat Pay

## Casdoor setup

Koffy does not seed or manage Casdoor's administrative configuration. Complete these steps before testing login:

1. Deploy Casdoor and create an organization, for example `koffy`.
2. Create an application, for example `koffy-local` or `koffy-production`.
3. Add Koffy's callback URL: `http://localhost:3000/auth/callback` locally or `https://koffy.example.com/auth/callback` in production.
4. Enable the password grant used by Koffy's phone/password flow.
5. Copy the application client ID, client secret, certificate, organization name, and application name into Koffy's environment file.
6. For SMS verification, create a supported SMS provider in Casdoor and set `REGISTRATION_SMS_PROVIDER` to that provider's name.

Koffy stores phone numbers in Casdoor's national-number field and stores the country/region separately. Koffy's own database stores the normalized E.164 form for uniqueness and lookup.

## Local environment

`docker-compose.local.yml` starts MySQL, Redis, an empty Casdoor instance, LiteLLM, and all Koffy services. You still perform the Casdoor setup above.

```bash
cp .env.example .env
docker compose -f docker-compose.local.yml up -d mysql redis casdoor litellm
# Configure Casdoor at http://localhost:8000 and update .env.
docker compose -f docker-compose.local.yml up --build -d
```

MySQL applies both SQL files only when its data directory is empty. To preserve local data, do not remove the `mysql_data` volume. To test a fresh schema, use a separate temporary Compose project or explicitly remove only a disposable test volume.

## Production topology

`docker-compose.prod.example.yml` assumes a blank Docker host and starts MySQL, Redis, Casdoor, LiteLLM, all Koffy services, and Nginx. MySQL and Redis use named volumes; runtime configuration, certificates, and payment keys remain in ignored host-side files.

Recommended public endpoints:

| Public endpoint | Upstream | Purpose |
| --- | --- | --- |
| `https://koffy.example.com` | `koffy-web:3000` | UI |
| `https://koffy.example.com/api/` | `koffy-billing-api:8080` | Same-origin API |
| `https://koffy.example.com/auth/` | `koffy-billing-api:8080` | Authentication callbacks |
| `https://gateway.koffy.example.com` | `koffy-gateway:8081` | Business application traffic |
| `https://auth.koffy.example.com` | `casdoor:8000` | Casdoor |

Do not expose `koffy-billing-api:8080`, MySQL, Redis, or LiteLLM directly to the internet.

## Production steps

1. Copy and complete the production environment:

   ```bash
   cp production.env.example production.env
   chmod 600 production.env
   ```

2. Copy the tracked configuration templates to ignored runtime files:

   ```bash
   cp deployments/nginx/koffy.example.com.conf.example deployments/nginx/koffy.conf
   cp deployments/litellm/config.example.yaml deployments/litellm/config.yaml
   ```

   Edit the copied files for your domains, certificate filenames, and model routes. Git ignores these runtime files, so source updates do not replace them.

3. Put TLS certificates under `./certs` and optional WeChat Pay key files under `./secrets`. Both directories are ignored by Git.
4. Build or pull immutable Koffy image tags. Avoid `latest` for controlled releases.
5. Start the complete stack:

   ```bash
   docker compose --env-file production.env -f docker-compose.prod.example.yml up -d
   ```

   On an empty `mysql_data` volume, MySQL creates the `koffy` and `casdoor` databases and applies `migrations/001_init.sql`. Production never applies `002_seed_local.sql`.

6. Nginx starts without waiting for Koffy application health, so the new Casdoor service remains reachable during initial setup. Open `PUBLIC_CASDOOR_URL`, complete the Casdoor setup, update `production.env` with its application credentials, and recreate the Koffy application containers:

   ```bash
   docker compose --env-file production.env -f docker-compose.prod.example.yml up -d --force-recreate koffy-billing-api koffy-gateway koffy-web
   ```
7. Verify readiness:

   ```bash
   docker inspect koffy-billing-api --format '{{json .State.Health}}'
   docker inspect koffy-gateway --format '{{json .State.Health}}'
   docker inspect koffy-web --format '{{json .State.Health}}'
   ```

## Configuration notes

- `AUTH_ALLOWED_RETURN_ORIGINS` is a comma-separated allowlist for external login redirects. Do not use wildcards.
- `BILLING_INTERNAL_API_KEY` authenticates Gateway-to-Billing calls and should contain at least 32 random bytes.
- `LITELLM_MASTER_KEY` must match LiteLLM's configured master key.
- `CAPTCHA_ENABLED=false` disables only the human challenge. SMS codes continue to work when a Casdoor SMS provider is configured.
- `WECHAT_PAY_ENABLED=false` is the safe default. Enabling it requires every merchant field, mounted key files, and a public HTTPS notify URL.
- `.env` and `production.env` are ignored by Git. Never put secret values in Compose files or container images.

## Persistent customization and upgrades

Image upgrades do not overwrite deployment-specific settings:

| Data | Persistence location |
| --- | --- |
| Koffy and Casdoor records, uploaded logos, favicons, and avatars | MySQL `mysql_data` volume |
| Sessions and rate-limit state | Redis `redis_data` volume |
| Domains and reverse-proxy rules | `deployments/nginx/koffy.conf` |
| Model routes and provider mapping | `deployments/litellm/config.yaml` |
| TLS certificates | `certs/` |
| WeChat Pay keys | `secrets/` |
| Production environment and credentials | `production.env` |

The repository tracks only neutral defaults and `.example` templates. Update containers with `docker compose pull` and `docker compose up -d`; do not run `docker compose down -v` unless you intentionally want to delete databases and uploaded branding.

## Optional WeChat login

- `WECHAT_OFFICIAL_APP_ID` and `WECHAT_OFFICIAL_APP_SECRET` handle OAuth inside WeChat.
- `WECHAT_WEBSITE_APP_ID` and `WECHAT_WEBSITE_APP_SECRET` handle desktop QR login.
- Configure the website application's authorization callback domain to the host of `PUBLIC_WEB_URL`.
- The callback endpoint is `{PUBLIC_WEB_URL}/api/v1/auth/wechat/callback`.

## Optional WeChat Pay

Mount the merchant private key and WeChat Pay public key under `./secrets` on the host. Set the corresponding container paths in the environment file. `WECHAT_PAY_PUBLIC_KEY_ID` is the public key ID issued by the merchant platform, not a certificate serial number.

The notify URL must be publicly reachable over HTTPS and must not contain query parameters:

```text
https://koffy.example.com/api/v1/payments/wechat/notify
```

## Backup and upgrades

Back up the `mysql_data` volume (both `koffy` and `casdoor` databases), ignored runtime configs, certificates, and secrets. Test restoration before relying on backups. v0.1.0 is the initial public schema baseline; future releases that change the schema must add forward migration files and document the supported upgrade path in `CHANGELOG.md`.

## Troubleshooting

- **Billing API is unhealthy:** verify MySQL, Redis, Casdoor network reachability, and required production variables.
- **Login returns to the login page:** verify the Casdoor callback URL, public URLs, reverse-proxy headers, and that Web and `/api/` share an origin.
- **SMS is consumed but not received:** check the SMS provider delivery record; accepted requests may still be queued or rejected downstream.
- **Gateway releases a reservation:** inspect LiteLLM/provider authentication and model routing. Provider failures intentionally cancel the reservation.
