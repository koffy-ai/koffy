# API 契约

## 通用约定

业务应用只调用 Koffy Gateway。Koffy Billing API 的计费接口主要供 Koffy Gateway 和内部服务调用。

请求头：

```http
Authorization: Bearer <koffy_access_token>
X-App-Key: <application_api_key>
Idempotency-Key: <unique_request_key>
```

用户中心和管理后台也支持由 `/auth/login` 写入的 HttpOnly Cookie 会话。API 调用优先使用 Bearer token；没有 Bearer 时会读取 `billing_session` Cookie。

仅在 `APP_ENV=local` 时可使用：

```http
X-User-ID: demo-user
```

生产环境中 `X-User-ID` 会由 Koffy Gateway 根据 Koffy token 或兼容的 Casdoor token 解析得到，业务应用不应自行传入可信用户 ID。当 `APP_ENV != local` 时，Koffy Gateway 必须校验 Bearer token。

Koffy Gateway 默认启用进程内限流：

- `AI_GATEWAY_APP_RATE_LIMIT_PER_MINUTE`：单应用每分钟请求上限，默认 `600`。
- `AI_GATEWAY_USER_RATE_LIMIT_PER_MINUTE`：单应用下单用户每分钟请求上限，默认 `120`。

任一值设为 `0` 可关闭对应维度限流。触发限流时返回 `429`，并带 `Retry-After` 响应头。

错误格式：

```json
{
  "error": {
    "code": "insufficient_balance",
    "message": "wallet balance is not enough"
  }
}
```

## 健康检查

```http
GET /healthz
GET /readyz
```

`/healthz` 是轻量存活检查，只表示进程已启动。`/readyz` 会检查关键依赖，适合 Docker healthcheck、Nginx 上线前验证和故障排查：

- Koffy Billing API：检查 MySQL。
- Koffy Gateway：检查 MySQL、Koffy Billing API 和 LiteLLM。

## Koffy Gateway

### Chat Completions

```http
POST /v1/chat/completions
```

兼容 OpenAI Chat Completions 请求结构。Koffy Gateway 会：

1. 校验应用 API Key。
2. 校验 Casdoor 用户 token，本地开发可临时使用 `X-User-ID`。
3. 调用 Koffy Billing API 预授权。
4. 转发请求到 LiteLLM。
5. 读取 `usage.total_tokens`。
6. 调用 Koffy Billing API 确认扣费。

支持非流式和流式请求。流式请求会由 Koffy Gateway 自动补充：

```json
{
  "stream_options": {
    "include_usage": true
  }
}
```

LiteLLM 会在 `[DONE]` 前返回一个包含 `usage` 的最终 chunk，Koffy Gateway 据此完成扣费。如果流式响应结束时没有 usage，Koffy Gateway 会取消本次预授权。

本地示例：

```bash
curl -X POST http://localhost:8081/v1/chat/completions \
  -H 'Content-Type: application/json' \
  -H 'X-App-Key: local-dev-app-key' \
  -H 'X-User-ID: demo-user' \
  -H 'Idempotency-Key: ai-request-001' \
  -d '{
    "model": "openai-chat-default",
    "messages": [
      {"role": "user", "content": "hello"}
    ]
  }'
```

成功响应会透传模型响应，并附加：

```http
X-Billing-Usage-Request-ID: <usage_request_id>
X-Billing-Charged-Coins: <charged_coins>
```

流式成功响应会通过 HTTP trailer 返回：

```http
X-Billing-Usage-Request-ID: <usage_request_id>
X-Billing-Charged-Coins: <charged_coins>
X-Billing-Status: committed
```

如果模型供应商调用失败，Koffy Gateway 会取消 Koffy Billing API 预授权。

### Image Generations

```http
POST /v1/images/generations
```

兼容 OpenAI 图像生成类接口。Koffy Gateway 会读取请求中的 `model` 和 `n`，预授权 `n` 张图片权益；`n` 省略时按 1 张预授权。模型供应商成功返回后，Koffy Gateway 会以响应 `data` 数组的实际长度作为 `actual_usage.images` 提交 Koffy Billing API，套餐内扣图片权益，超出后按 `unit=images` 的单位定价扣点数。

示例：

```bash
curl -X POST http://localhost:8081/v1/images/generations \
  -H 'Content-Type: application/json' \
  -H 'X-App-Key: local-dev-app-key' \
  -H 'X-User-ID: demo-user' \
  -H 'Idempotency-Key: img-001' \
  -d '{"model":"openai-image-default","prompt":"a product photo","n":1,"size":"1024x1024"}'
```

## Koffy Billing API

### 预授权

```http
POST /api/v1/billing/authorize
```

```json
{
  "app_id": "image-app",
  "user_id": "casdoor-user-id",
  "idempotency_key": "req-001",
  "billing_mode": "hybrid",
  "model": "openai-chat-default",
  "estimated_usage": {
    "total_tokens": 2000,
    "images": 1,
    "video_seconds": 0
  }
}
```

响应：

```json
{
  "usage_request_id": "10001",
  "status": "authorized",
  "reserved_coins": 2,
  "reserved_units": 1
}
```

`reserved_units` / `charged_units` 是套餐权益消耗的汇总值；详细单位可通过用户权益余额和权益流水查看。

### 确认扣费

```http
POST /api/v1/billing/commit
```

```json
{
  "usage_request_id": "10001",
  "provider": "openai",
  "model": "gpt-4o-mini",
  "provider_job_id": "chatcmpl_xxx",
  "actual_usage": {
    "prompt_tokens": 1200,
    "completion_tokens": 301,
    "total_tokens": 1501,
    "images": 1
  }
}
```

响应：

```json
{
  "usage_request_id": "10001",
  "status": "committed",
  "charged_coins": 2,
  "charged_units": 1
}
```

### 取消预授权

```http
POST /api/v1/billing/cancel
```

```json
{
  "usage_request_id": "10001",
  "reason": "model_request_failed"
}
```

### 直接扣费

```http
POST /api/v1/billing/charge
```

用于无需预授权的短事务场景。内部仍按幂等键记录请求。

## 用户中心

```http
GET /api/v1/me
PATCH /api/v1/me/profile
POST /api/v1/me/avatar
GET /api/v1/me/auth-bindings
POST /api/v1/me/phone/code
POST /api/v1/me/phone/bind
POST /api/v1/me/password/code
POST /api/v1/me/password/change
GET /api/v1/me/wallet
GET /api/v1/me/wallet/ledger
GET /api/v1/me/recharge-orders
GET /api/v1/me/subscriptions
GET /api/v1/me/entitlements
GET /api/v1/me/entitlement-ledger
GET /api/v1/me/usage-requests
POST /api/v1/recharge/orders
GET /api/v1/branding/logo
GET /api/v1/branding/favicon
```

用户中心接口使用 Koffy/Casdoor Bearer token，也支持由 Koffy 登录页写入的 `billing_session` Cookie。本地开发时可用 `X-User-ID`。业务应用可用登录码交换得到的 Koffy access token，在自己的后端调用这些接口，用于应用内轻量展示账户资料、余额、套餐权益和最近使用记录。

### Casdoor 登录

```http
GET /auth/login
GET /auth/register
GET /auth/callback
POST /auth/logout
GET /api/v1/auth/config
POST /api/v1/auth/login
POST /api/v1/auth/session-exchange
POST /api/v1/auth/forgot-password/code
POST /api/v1/auth/forgot-password/reset
GET /api/v1/auth/wechat/start
GET /api/v1/auth/wechat/callback
```

`/auth/login` 会跳转到 Koffy 登录页 `/login`，`/auth/register` 会转到 `/register`。后端通过内部地址调用 Casdoor，浏览器不需要访问 Casdoor 登录页。密码登录和微信登录最终都建立 Koffy 会话；API 客户端也可以继续使用兼容的 Casdoor Bearer token。

登录回跳支持站内路径和白名单外部域名。业务应用应跳转到：

```text
https://koffy.example.com/login?return_to=https%3A%2F%2Fapp.example.com%2Fcallback
```

外部 `return_to` 必须在 `AUTH_ALLOWED_RETURN_ORIGINS` 中配置。

密码登录：

```http
POST /api/v1/auth/login
```

```json
{
  "account": "<phone-or-username>",
  "password": "Password123",
  "return_to": "https://app.example.com/auth/callback"
}
```

`account` 支持普通用户手机号，也支持管理员账号名。登录成功后写入 `billing_session` Cookie，并返回：

```json
{
  "status": "ok",
  "redirect_to": "https://app.example.com/auth/callback?koffy_login_code=..."
}
```

如果 `return_to` 是站内路径，`redirect_to` 不会带登录码。如果 `return_to` 是白名单外部域名，Koffy 会附加一次性 `koffy_login_code`。

业务应用后端用登录码换取 Koffy access token：

```http
POST /api/v1/auth/session-exchange
```

```json
{
  "code": "一次性koffy_login_code"
}
```

响应：

```json
{
  "access_token": "koffy1....",
  "token_type": "Bearer",
  "expires_at": "2026-06-13T12:00:00+08:00",
  "user": {
    "id": 1,
    "casdoor_user_id": "user_example_id",
    "display_name": "138****8000"
  }
}
```

业务应用后端应消费该 code、建立自己的业务登录态，并在调用 Koffy Gateway 时把 `access_token` 放入 `Authorization: Bearer <token>`。登录码有效期 2 分钟且只能使用一次。

直接使用 Casdoor OAuth 时，应用回调地址为：

```text
{PUBLIC_WEB_URL}/auth/callback
```

`/auth/callback` 会交换 authorization code，并建立 HttpOnly Cookie 会话。生产环境需要在 Casdoor 应用中加入上述回调地址。

### 手机号注册

注册页面由 Koffy Web 提供，用户不会跳转到认证服务域名。后端通过内部地址调用认证服务创建用户。

发送验证码：

```http
POST /api/v1/auth/phone-code
```

```json
{
  "country_code": "+86",
  "phone": "<mainland-phone>",
  "human_token": "<captcha-token>"
}
```

当 `CAPTCHA_ENABLED=false` 时，发送短信验证码前不再要求 `human_token`；短信验证码本身仍会生成、发送并在后续接口中校验。当 `CAPTCHA_PROVIDER=tencent` 时，`human_token` 是前端腾讯云验证码成功回调后生成的 JSON 字符串，包含 `ticket`、`randstr` 和 `captcha_app_id`。业务应用如果自建登录页，也需要先调用 `/api/v1/auth/config` 获取 provider 与 site key，再按 provider 获取人机验证 token。

注册：

```http
POST /api/v1/auth/register
```

```json
{
  "country_code": "+86",
  "phone": "<mainland-phone>",
  "code": "123456",
  "password": "password123",
  "confirm_password": "password123"
}
```

当前只支持中国大陆手机号。Koffy API 和本地数据库内部会统一规范成 `+86` 格式；写入 Casdoor 用户资料时遵循 Casdoor 字段约定，`phone` 保存 11 位本地手机号，`countryCode` 保存 `CN`。本地环境会在响应中返回 `debug_code` 便于测试。生产环境短信由 Casdoor 中配置的 SMS Provider 发送；如果要固定使用某个腾讯云 SMS Provider，在 Koffy `.env` 中设置 `REGISTRATION_SMS_PROVIDER` 为 Casdoor 里的 Provider 名称。验证码发送成功后，前端会禁用发送按钮 60 秒，并在倒计时结束后要求重新完成人机验证。

### 忘记密码

发送重置验证码：

```http
POST /api/v1/auth/forgot-password/code
```

```json
{
  "country_code": "+86",
  "phone": "<mainland-phone>",
  "human_token": "<captcha-token>"
}
```

当 `CAPTCHA_ENABLED=false` 时，发送短信验证码前不再要求 `human_token`；短信验证码本身仍会生成、发送并在重置密码时校验。

重置密码：

```http
POST /api/v1/auth/forgot-password/reset
```

```json
{
  "country_code": "+86",
  "phone": "<mainland-phone>",
  "code": "123456",
  "password": "Password123",
  "confirm_password": "Password123"
}
```

密码强度：至少 8 位，且至少包含一个大写字母、一个小写字母和一个数字。

### 微信登录

```http
GET /api/v1/auth/wechat/start?action=login&return_to=/center
GET /api/v1/auth/wechat/callback
```

`mode` 可选：

- `mode=official`：微信内服务号 OAuth。
- `mode=website`：微信外开放平台网站应用 OAuth。
- 不传 `mode`：后端根据 User-Agent 自动选择。微信内使用服务号，微信外使用网站应用。

微信开放平台网站应用授权回调域填写 `koffy.example.com`，实际回调接口为 `{PUBLIC_WEB_URL}/api/v1/auth/wechat/callback`。

### 账号绑定与改密

```http
GET /api/v1/me/auth-bindings
POST /api/v1/me/phone/code
POST /api/v1/me/phone/bind
POST /api/v1/me/password/code
POST /api/v1/me/password/change
GET /api/v1/auth/wechat/start?action=bind&return_to=/center/security
```

绑定手机号发送验证码：

```json
{
  "country_code": "+86",
  "phone": "<mainland-phone>",
  "human_token": "<captcha-token>"
}
```

当 `CAPTCHA_ENABLED=false` 时，发送短信验证码前不再要求 `human_token`；短信验证码本身仍会生成、发送并在绑定手机号时校验。

绑定手机号并设置密码：

```json
{
  "country_code": "+86",
  "phone": "<mainland-phone>",
  "code": "123456",
  "password": "Password123",
  "confirm_password": "Password123"
}
```

修改密码先调用 `/api/v1/me/password/code` 发送验证码，再调用 `/api/v1/me/password/change`：

```json
{
  "code": "123456",
  "password": "Password123",
  "confirm_password": "Password123"
}
```

### 当前用户

```http
GET /api/v1/me
```

返回本地同步后的用户资料，包括 Casdoor 用户 ID、展示名、邮箱、手机号和 `is_admin`。

### 钱包余额

```http
GET /api/v1/me/wallet
```

响应：

```json
{
  "balance_coins": 9998,
  "reserved_coins": 0,
  "available_coins": 9998
}
```

### 钱包流水

```http
GET /api/v1/me/wallet/ledger?limit=50
```

流水会记录预授权 `reserve`、释放 `release`、实际扣费 `debit` 和充值/后台增加 `credit`。

### 使用和权益流水

```http
GET /api/v1/me/usage-requests?limit=50
GET /api/v1/me/entitlement-ledger?limit=50
```

`usage-requests` 返回用户在各应用中的预授权、确认扣费、取消记录；`entitlement-ledger` 返回套餐权益的预留、消费、释放和重置流水。

用户中心前端会把这两类接口结果合并为面向用户的 `使用记录` 卡片列表，用中文状态和业务文案展示，不直接暴露内部动作名、方向字段或英文状态。管理后台可继续分别展示完整明细用于排查。

### 充值订单

```http
GET /api/v1/me/recharge-orders?limit=50
```

返回当前用户的充值订单、金额、点数数量、支付状态和微信交易号。用户中心充值页只展示已支付成功的充值记录，并隐藏订单号、待支付/失败状态等运营排查信息；完整订单信息由管理后台展示。

### 订阅和权益

```http
GET /api/v1/me/subscriptions
GET /api/v1/me/entitlements
```

当前返回用户已订阅套餐和各自然月权益余额。未购买套餐时返回空列表。

### 品牌 Logo

```http
GET /api/v1/branding/logo?area=center
GET /api/v1/branding/logo?area=admin
```

返回当前用户中心和管理后台使用的 `image/png` Logo。未在管理后台上传自定义 Logo 时，返回系统内置默认 Logo。建议前端直接使用该接口作为图片地址。

### 品牌 favicon

```http
GET /api/v1/branding/favicon?area=center
GET /api/v1/branding/favicon?area=admin
```

返回用户中心或管理后台 favicon。自定义图标为 `128*128` PNG；未上传时返回系统内置的中性 SVG。两个区域可独立配置。

### 创建充值订单

```http
POST /api/v1/recharge/orders
```

```json
{
  "amount_cents": 100,
  "channel": "wechat_native",
  "description": "点数充值",
  "openid": ""
}
```

当前固定比例为 `1 元 = 100 点数`，因此 `amount_cents=100` 会生成 `100` 点数。

`channel` 支持：

- `wechat_native`：返回二维码支付链接 `code_url`。
- `wechat_jsapi`：返回 JSAPI 调起支付参数，必须传 `openid`。

当 `WECHAT_PAY_ENABLED=false` 时，该接口返回 `503 wechat_pay_disabled`，不会创建充值订单；其它非充值功能不受影响。

本地响应：

```json
{
  "order_no": "kf202605271417231b68f2b0443b5ba8",
  "provider": "wechat",
  "amount_cents": 100,
  "coins": 100,
  "status": "pending",
  "payment": {
    "channel": "wechat_native",
    "mode": "local_test",
    "notify_url": "https://koffy.example.com/api/v1/payments/wechat/notify"
  }
}
```

生产环境会使用微信支付 API v3 官方 Go SDK 统一下单。Native 响应示例：

```json
{
  "order_no": "kf202605271417231b68f2b0443b5ba8",
  "provider": "wechat",
  "amount_cents": 100,
  "coins": 100,
  "status": "pending",
  "payment": {
    "channel": "wechat_native",
    "code_url": "weixin://wxpay/bizpayurl?pr=xxx",
    "notify_url": "https://koffy.example.com/api/v1/payments/wechat/notify"
  }
}
```

JSAPI 响应中的 `payment` 会包含 `prepay_id`、`appId`、`timeStamp`、`nonceStr`、`package`、`signType` 和 `paySign`。

## 管理后台

管理后台接口使用 Casdoor Bearer token，并要求当前用户 `IsAdmin=true`。本地开发时可使用：

```http
X-User-ID: local-admin
X-Admin: true
```

普通用户访问管理接口会返回 `403 forbidden`。

### 更换 Logo

```http
POST /api/v1/admin/branding/logo?area=center
Content-Type: multipart/form-data
```

表单字段：

| 字段 | 说明 |
| --- | --- |
| `logo` | 上传的图片文件 |

如果上传文件刚好是 `702*180` 像素 PNG 且小于 `200KB`，后端会原样保存；否则会转换为 `702*180` 像素、文件大小小于 `200KB` 的 PNG 后保存。建议使用透明背景 PNG。

响应：

```json
{
  "status": "ok",
  "area": "center",
  "logo_url": "/api/v1/branding/logo?area=center",
  "size_bytes": 74413,
  "width": 702,
  "height": 180
}
```

`area` 支持 `center` 和 `admin`，分别对应用户中心和管理后台。

### 更换 favicon

```http
POST /api/v1/admin/branding/favicon?area=center
Content-Type: multipart/form-data
```

表单字段为 `favicon`。支持 PNG、JPEG 和 GIF，文件不得超过 `2MB`；后端居中裁剪并转换为 `128*128`、小于 `100KB` 的安全 PNG。`area` 同样支持 `center` 和 `admin`。

```json
{
  "status": "ok",
  "area": "center",
  "favicon_url": "/api/v1/branding/favicon?area=center",
  "size_bytes": 4218,
  "width": 128,
  "height": 128
}
```

### 运营概览

```http
GET /api/v1/admin/metrics/summary?days=7
```

按应用汇总最近 N 天的调用量、成功/取消数量、扣点数、权益消耗、token、图片、视频秒和业务单位用量；同时按状态汇总充值订单。

### 调用记录

```http
GET /api/v1/admin/usage-requests?app_code=demo-app&user_id=demo-user&limit=100
```

`app_code` 和 `user_id` 均可省略。返回各应用调用的预授权金额、实际扣费、权益消耗、状态和错误信息，用于后台监控应用接入和使用情况。

### 充值和支付事件

```http
GET /api/v1/admin/recharge-orders?user_id=demo-user&status=paid&limit=100
GET /api/v1/admin/payment-events?order_no=kf202605271417231b68f2b0443b5ba8&limit=100
```

用于管理后台查看充值订单状态、微信交易号、支付事件是否已处理。查询参数均可省略。

### 应用列表

```http
GET /api/v1/admin/apps
```

响应：

```json
{
  "items": [
    {
      "id": 1,
      "app_code": "demo-app",
      "name": "Demo App",
      "status": "active",
      "billing_mode": "hybrid",
      "description": "local demo application"
    }
  ]
}
```

### 创建或更新应用

```http
POST /api/v1/admin/apps
```

```json
{
  "app_code": "image-app",
  "name": "Image App",
  "billing_mode": "hybrid",
  "description": "image generation application"
}
```

`billing_mode` 可用于标记应用支持的扣费方式，当前核心实现支持：

- `hybrid`：套餐权益优先，超出后使用点数。
- `wallet`：仅点数。
- `entitlement`：仅套餐权益。

### 创建应用 API Key

```http
POST /api/v1/admin/apps/{app_code}/api-keys
```

响应：

```json
{
  "app_code": "image-app",
  "key": "bgw_xxx",
  "prefix": "bgw_xxx"
}
```

明文 `key` 只在创建时返回一次，数据库只保存 SHA-256 哈希。业务应用调用 Koffy Gateway 时把该 key 放在 `X-App-Key`。

### 配置 Token 定价

查看当前定价：

```http
GET /api/v1/admin/apps/{app_code}/pricing
```

返回：

```json
{
  "token_pricing": [],
  "unit_pricing": []
}
```

```http
POST /api/v1/admin/apps/{app_code}/pricing
```

```json
{
  "model_alias": "*",
  "token_amount": 1000,
  "coin_amount": 1
}
```

扣费规则为：

```text
charged_coins = ceil(total_tokens * coin_amount / token_amount)
```

`model_alias="*"` 表示默认规则。未来可为不同模型设置独立规则。

同一应用、同一 `model_alias` 只会保留一条 active Token 定价；再次保存会自动停用旧 active 记录并生成新记录，避免出现相互矛盾的扣费规则。

删除 Token 定价：

```http
DELETE /api/v1/admin/pricing/token/{pricing_id}
```

### 配置非 Token 单位定价

```http
POST /api/v1/admin/apps/{app_code}/unit-pricing
```

```json
{
  "model_alias": "*",
  "unit": "images",
  "unit_amount": 1,
  "coin_amount": 20
}
```

`unit` 可为 `images`、`video_seconds`、`business_units`。扣费规则为：

```text
charged_coins = ceil(usage_amount * coin_amount / unit_amount)
```

例如 `unit=images, unit_amount=1, coin_amount=20` 表示每 1 张图扣 20 点数；`unit=video_seconds, unit_amount=60, coin_amount=300` 表示每 60 秒视频扣 300 点数，不足 60 秒时向上取整。

同一应用、同一 `model_alias`、同一 `unit` 只会保留一条 active 单位定价；再次保存会自动停用旧 active 记录并生成新记录。

删除单位定价：

```http
DELETE /api/v1/admin/pricing/unit/{pricing_id}
```

### 套餐配置

```http
GET /api/v1/admin/apps/{app_code}/plans
POST /api/v1/admin/apps/{app_code}/plans
POST /api/v1/admin/apps/{app_code}/plans/{plan_code}/entitlements
```

创建或更新套餐：

```json
{
  "plan_code": "starter",
  "name": "Starter Monthly",
  "period": "monthly",
  "price_cents": 990,
  "status": "active"
}
```

配置套餐权益：

```json
{
  "entitlement_code": "monthly_tokens",
  "name": "Monthly Tokens",
  "unit": "tokens",
  "monthly_quota": 200000
}
```

当前计费链路支持 `unit=tokens`、`unit=images`、`unit=video_seconds`、`unit=business_units` 的套餐权益。应用为 `hybrid` 时，会按实际请求中的单位分别优先消耗当月权益：例如 `actual_usage.images=1` 扣 1 张图额度，`actual_usage.video_seconds=30` 扣 30 秒视频额度。权益超出后会按对应的 token 定价或非 token 单位定价扣点数；应用为 `entitlement` 时，任一单位权益不足都会拒绝请求。

### 给用户开通套餐

```http
POST /api/v1/admin/users/{casdoor_user_id}/subscriptions
```

```json
{
  "app_code": "demo-app",
  "plan_code": "starter",
  "months": 1
}
```

接口会为用户创建或延长同一应用同一套餐的有效订阅，并生成当前自然月权益余额。若重复开通同一套餐，会延长现有订阅，避免重复叠加多份当月权益。

### 套餐权益维护

```http
POST /api/v1/admin/entitlements/maintenance
```

管理员手动触发套餐维护任务。服务也会按 `ENTITLEMENT_MAINTENANCE_INTERVAL_MINUTES` 定时自动执行同一逻辑：

- 将已到期的 active 订阅标记为 `expired`。
- 为仍在有效期内的 active 订阅生成当前自然月权益余额。
- 新生成的自然月权益会写入 `entitlement_ledger`，方向为 `reset`。

响应示例：

```json
{
  "expired_subscriptions": 1,
  "created_balances": 4,
  "updated_balances": 0
}
```

### AI 供应商配置

```http
GET /api/v1/admin/ai/providers
POST /api/v1/admin/ai/providers
```

```json
{
  "provider_code": "openai",
  "name": "OpenAI via LiteLLM",
  "status": "active",
  "base_url": "https://api.openai.com"
}
```

### AI 模型配置

```http
GET /api/v1/admin/ai/models
POST /api/v1/admin/ai/models
```

```json
{
  "provider_code": "openai",
  "model_alias": "openai-chat-default",
  "provider_model": "gpt-4o-mini",
  "capability": "chat",
  "status": "active"
}
```

`model_alias` 是业务应用调用 Koffy Gateway 时传入的模型名，也是计费规则匹配的模型名。`provider_model` 是供应商侧真实模型名，当前 LiteLLM 仍按其配置解析别名。

### 应用模型路由

```http
GET /api/v1/admin/apps/{app_code}/model-routes
POST /api/v1/admin/apps/{app_code}/model-routes
```

```json
{
  "model_alias": "openai-chat-default",
  "status": "active"
}
```

Koffy Gateway 的校验规则：

- 应用没有配置任何 active 模型路由时，允许调用任意模型别名。生产应用应显式配置路由以限制可用模型。
- 一旦应用配置了 active 模型路由，则只能调用已启用的模型别名。
- 未启用模型会返回 `403 model_not_allowed`，不会进入模型供应商，也不会扣费。

### 查询用户资产

```http
GET /api/v1/admin/users/{casdoor_user_id}/asset
```

响应包括用户资料和钱包余额：

```json
{
  "user": {
    "casdoor_user_id": "demo-user",
    "name": "demo-user",
    "is_admin": false
  },
  "wallet": {
    "balance_coins": 10048,
    "reserved_coins": 0,
    "available_coins": 10048
  }
}
```

管理员查看某个用户的完整资产明细：

```http
GET /api/v1/admin/users/{casdoor_user_id}/wallet/ledger?limit=50
GET /api/v1/admin/users/{casdoor_user_id}/subscriptions
GET /api/v1/admin/users/{casdoor_user_id}/entitlements
GET /api/v1/admin/users/{casdoor_user_id}/entitlement-ledger?limit=50
GET /api/v1/admin/users/{casdoor_user_id}/usage-requests?limit=50
GET /api/v1/admin/users/{casdoor_user_id}/recharge-orders?limit=50
```

这些接口与用户中心接口返回结构一致，但数据对象是路径中的 `{casdoor_user_id}`，用于管理后台查看任意用户的钱包流水、套餐权益、AI 调用记录和充值订单。

### 后台手工调账

```http
POST /api/v1/admin/users/{casdoor_user_id}/adjust-coins
```

```json
{
  "amount_coins": 50,
  "remark": "manual top-up"
}
```

`amount_coins` 为正数表示增加点数，为负数表示扣减点数。接口会写入 `wallet_ledger`，并记录 `audit_logs`。

## 微信支付回调

```http
POST /api/v1/payments/wechat/notify
```

生产处理要求：

- 验签。
- 解密微信支付 API v3 通知资源。
- 仅 `trade_state=SUCCESS` 时入账。
- 校验通知金额与本地充值订单金额一致。
- `payment_events.provider + event_id` 唯一。
- `recharge_orders.order_no` 唯一。
- 重复回调直接返回成功，不重复入账。

当前生产代码使用微信支付 API v3 官方 Go SDK：

- 初始化支付客户端时使用商户号、AppID、商户证书序列号、商户私钥和 APIv3 key。
- 创建订单时调用 Native 或 JSAPI 统一下单。
- 回调时使用 SDK `notify.Handler` 验签并解密为交易对象。

当前代码仅允许本地测试模式使用明文通知：

```http
X-WeChatPay-Test: true
```

```json
{
  "event_id": "evt-local-recharge-001",
  "event_type": "TRANSACTION.SUCCESS",
  "order_no": "kf202605271417231b68f2b0443b5ba8",
  "transaction_id": "wx-local-tx-001",
  "success_time": "2026-05-27T14:17:30+08:00"
}
```

接口会在同一事务中记录支付事件、锁定充值订单、更新钱包余额，并写入 `wallet_ledger`。非本地环境在微信支付 SDK 验签/解密接入前会拒绝处理回调，避免未验签充值。
