# 应用接入操作流程

本文档面向后续业务应用开发者和运营管理员。业务应用只接入 Koffy Gateway；Koffy Gateway 会完成应用鉴权、用户鉴权、模型转发、用量回传和计费扣减。除非是用户中心或管理后台场景，业务应用不要直接调用 Koffy Billing API。

## 1. 接入前准备

需要先确认以下信息已经可用：

- 用户登录体系：Koffy Center 已部署，用户可通过手机号或微信登录。
- 用户中心/管理后台：`https://koffy.example.com`。
- Koffy Gateway：`https://gateway.koffy.example.com`，这是公网 Nginx 入口，内部会反代到 `koffy-gateway:8081`。
- 认证服务管理入口：`https://auth.koffy.example.com`，仅管理员维护时使用，业务应用不要跳转到该地址。
- 管理员账号：Casdoor 中 `IsAdmin=true` 的账号，用于进入管理后台。
- 模型供应商密钥：例如 OpenAI、兼容 OpenAI 协议的模型服务，或 LiteLLM 支持的其他供应商。

生产环境中，业务应用后端必须保存 App Key；不要把 App Key 暴露给浏览器、小程序前端或移动端安装包。

## 2. 管理后台配置应用

1. 使用管理员账号登录 `https://koffy.example.com/admin`。
2. 在“应用”中创建业务应用：
   - `app_code`：稳定唯一标识，例如 `image-studio`。
   - `name`：后台显示名称。
   - `billing_mode`：选择 `coins`、`entitlement` 或 `hybrid`。
   - `hybrid` 表示用户有套餐时优先消耗套餐权益，超出后再扣点数。
3. 为应用生成 App Key。App Key 只在创建时完整展示一次，应立即保存到业务应用后端的环境变量。
4. 配置应用可调用的模型路由。只有已启用路由的模型别名才允许被该应用调用。
5. 按业务需要配置扣费规则：
   - Token 扣费：例如 `1000 tokens = 1 点数`。
   - 图片扣费：例如 `1 张图 = 20 点数`。
   - 视频扣费：例如 `1 秒视频 = 5 点数`。
   - 其他业务单位：可使用 `business_units` 表示应用自定义次数或额度。

扣点数时没有小数，实际消耗会向上取整。例如 `1501 tokens` 按 `1000 tokens = 1 点数` 计费时扣 `2` 点。

## 3. 配置套餐权益

套餐权益按自然月重置，不累计到下月。

1. 在应用下创建套餐：
   - `plan_code`：套餐唯一标识，例如 `pro-monthly`。
   - `period`：当前支持月/年等周期字段，实际权益按自然月重置。
   - `price_cents`：套餐价格，单位为分。
   - `status`：上线套餐设为 `active`。
2. 为套餐添加权益：
   - 文本类：`unit=tokens`，例如每月 `200000` tokens。
   - 图片类：`unit=images`，例如每月 `200` 张图。
   - 视频类：`unit=video_seconds`，例如每月 `600` 秒视频。
   - 自定义类：`unit=business_units`。
3. 如需人工给用户开通套餐，可在“用户资产”中为用户添加订阅。
4. 定时维护任务会自动为有效订阅生成当月权益余额。管理员也可以手动触发权益维护。

## 4. 配置 AI 模型

1. 在管理后台添加 AI Provider，例如 OpenAI 或兼容 OpenAI 协议的 LiteLLM provider。
2. 添加模型别名：
   - `model_alias`：业务应用调用时使用的稳定名称，例如 `openai-chat-default`。
   - `provider_model`：供应商真实模型名。
   - `capability`：例如 `chat` 或 `image`。
   - `status=active`。
3. 在应用模型路由中把该 `model_alias` 授权给业务应用。
4. 在应用扣费规则中为该模型别名配置 token 或单位价格。可以用 `*` 作为兜底规则。

## 5. 业务应用登录接入

业务应用不要自己实现手机号/微信登录，也不要把用户直接带到认证服务域名。统一跳转到 Koffy Center：

```text
https://koffy.example.com/login?return_to=https%3A%2F%2Fapp.example.com%2Fauth%2Fcallback
```

登录成功后，Koffy 会先校验 `return_to` 是否在 `AUTH_ALLOWED_RETURN_ORIGINS` 白名单中，再跳回：

```text
https://app.example.com/auth/callback?koffy_login_code=一次性登录码
```

业务应用后端收到 `koffy_login_code` 后，调用 Koffy Center 换取 access token：

```bash
curl -X POST https://koffy.example.com/api/v1/auth/session-exchange \
  -H 'Content-Type: application/json' \
  -d '{"code":"一次性登录码"}'
```

响应中的 `access_token` 是 Koffy 签发的 Bearer token。业务应用应在自己的服务端建立登录态，例如写入业务应用自己的 HttpOnly Cookie；不要把长期 token 留在 URL 中。`koffy_login_code` 有效期 2 分钟且只能使用一次。

如果用户在微信中打开 Koffy 登录页，Koffy 会使用服务号 OAuth；如果用户在微信外打开，Koffy 会使用微信开放平台网站应用 OAuth。业务应用不需要自己区分这两种微信授权方式。

## 6. 应用内账户资产展示

业务应用建议在自己的界面内展示高频、轻量的账户资产信息，例如头像昵称、点数余额、当前应用套餐和剩余额度。完整账户管理仍跳转到 Koffy 用户中心。

推荐分工：

- 业务应用内展示：头像、昵称、点数余额、当前应用套餐、当前应用权益剩余、最近几条调用/扣费记录。
- Koffy 用户中心展示：账号安全、绑定手机/微信、修改密码、已订购套餐、成功充值记录、完整点数流水，以及合并后的使用记录；完整充值订单状态、订单号和运营排查信息由管理后台查看。

这些展示接口使用登录码交换得到的 `access_token`：

```http
Authorization: Bearer <koffy_access_token>
```

如果业务应用采用后端会话模式，建议由业务应用后端调用 Koffy Center 的用户侧接口，再把必要字段返回给自己的前端。不要把长期 token 写入 URL，也不要把 App Key 暴露给浏览器。

推荐业务应用后端封装一个自己的轻量聚合接口，例如：

```http
GET /api/account/summary
```

该接口由业务应用后端读取自己的登录态，取出保存的 Koffy access token，然后在服务端调用 Koffy Center，最后只返回当前页面需要的字段。这样业务前端不需要理解 Koffy 的 token 生命周期，也能避免每个页面同时请求多条 Koffy 接口。

一个常见返回结构可以是：

```json
{
  "user": {
    "display_name": "Example User",
    "avatar_url": "https://koffy.example.com/api/v1/users/avatar/...",
    "phone": "138****0000"
  },
  "wallet": {
    "available_coins": 9980
  },
  "current_app": {
    "app_code": "image-studio",
    "subscriptions": [],
    "entitlements": [
      {
        "unit": "images",
        "quota": 200,
        "used": 12,
        "available": 188
      }
    ]
  },
  "links": {
    "center": "https://koffy.example.com/center",
    "recharge": "https://koffy.example.com/center/recharge",
    "security": "https://koffy.example.com/center/security"
  }
}
```

业务应用可以缓存这个聚合结果几十秒，用于页面顶部余额、套餐角标和权益余量展示。真正发起 AI 调用时仍以 Koffy Gateway 的实时预授权结果为准，不要依赖前端缓存自行判断是否扣费。

### 6.1 用户资料

用于右上角用户区、头像、昵称展示：

```bash
curl https://koffy.example.com/api/v1/me \
  -H 'Authorization: Bearer <koffy_access_token>'
```

常用字段：

- `display_name`：用户昵称。
- `avatar_url`：头像地址。
- `phone`：已绑定手机号。
- `is_admin`：是否管理员。

### 6.2 点数余额

用于应用内余额展示、余额不足前置提示：

```bash
curl https://koffy.example.com/api/v1/me/wallet \
  -H 'Authorization: Bearer <koffy_access_token>'
```

常用字段：

- `balance_coins`：总余额。
- `reserved_coins`：已预留但尚未最终结算的点数。
- `available_coins`：可用余额，应用内一般展示这个字段。

### 6.3 当前套餐与权益

用于应用内显示用户是否有套餐、套餐内额度还剩多少：

```bash
curl https://koffy.example.com/api/v1/me/subscriptions \
  -H 'Authorization: Bearer <koffy_access_token>'

curl https://koffy.example.com/api/v1/me/entitlements \
  -H 'Authorization: Bearer <koffy_access_token>'
```

`subscriptions` 返回用户已订购套餐；`entitlements` 返回自然月权益余额。业务应用展示时应按自己的 `app_code` 过滤，只显示当前应用相关套餐和权益。

常用权益字段：

- `app_code` / `app_name`：所属应用。
- `entitlement_code`：权益标识。
- `unit`：权益单位，例如 `tokens`、`images`、`video_seconds`、`business_units`。
- `quota`：当月总额度。
- `used`：已消耗额度。
- `reserved`：已预留额度。
- `available`：当前可用额度。

### 6.4 最近使用记录

用于应用内展示最近几条生成/调用记录，或在任务详情页展示本次扣费结果：

```bash
curl 'https://koffy.example.com/api/v1/me/usage-requests?limit=20' \
  -H 'Authorization: Bearer <koffy_access_token>'
```

返回内容包含应用、模型、预估用量、实际用量、扣费状态、套餐权益消耗和点数扣费。业务应用展示时同样应按自己的 `app_code` 过滤。

Koffy Gateway 调用成功后还会在响应头中返回：

```http
X-Billing-Usage-Request-ID: <usage_request_id>
X-Billing-Charged-Coins: <charged_coins>
```

业务应用可以把 `usage_request_id` 记录到自己的任务表，方便后续关联 Koffy 的扣费记录。

### 6.5 完整流水与充值记录

完整流水更适合跳转 Koffy 用户中心查看；如果业务应用确实需要展示，也可以调用：

```bash
curl 'https://koffy.example.com/api/v1/me/wallet/ledger?limit=50' \
  -H 'Authorization: Bearer <koffy_access_token>'

curl 'https://koffy.example.com/api/v1/me/entitlement-ledger?limit=50' \
  -H 'Authorization: Bearer <koffy_access_token>'

curl 'https://koffy.example.com/api/v1/me/recharge-orders?limit=50' \
  -H 'Authorization: Bearer <koffy_access_token>'
```

应用内一般不建议做完整账户中心，只保留“查看更多”入口跳转到 Koffy：

```text
https://koffy.example.com/center
https://koffy.example.com/center/recharge
https://koffy.example.com/center/security
```

### 6.6 充值入口

余额不足时推荐优先跳转 Koffy 充值页：

```text
https://koffy.example.com/center/recharge/?return_to=https%3A%2F%2Fapp.example.com%2F...
```

如果业务应用希望在自己的页面内创建充值订单，可调用：

```bash
curl -X POST https://koffy.example.com/api/v1/recharge/orders \
  -H 'Content-Type: application/json' \
  -H 'Authorization: Bearer <koffy_access_token>' \
  -d '{
    "amount_cents": 1000,
    "channel": "wechat_native",
    "description": "compute coin recharge"
  }'
```

订单创建后，业务应用需要按返回参数展示微信支付二维码，并轮询或刷新充值订单状态。微信内的用户侧充值页会使用 JSAPI 支付；微信外或业务应用自行创建 `wechat_native` 订单时展示二维码。推荐直接跳转 Koffy 充值页，减少每个应用重复处理支付状态和异常恢复逻辑。

### 6.7 推荐展示组合

业务应用首页右上角：

- `GET /api/v1/me`
- `GET /api/v1/me/wallet`

业务应用套餐/额度面板：

- `GET /api/v1/me/subscriptions`
- `GET /api/v1/me/entitlements`

任务详情或生成历史：

- Koffy Gateway 响应头 `X-Billing-Usage-Request-ID`
- `GET /api/v1/me/usage-requests?limit=20`

完整账户管理：

- 跳转 `https://koffy.example.com/center`

账号安全、绑定手机、绑定微信、修改密码：

- 跳转 `https://koffy.example.com/center/security`

充值：

- 优先跳转 `https://koffy.example.com/center/recharge`
- 需要深度嵌入支付时再调用 `POST /api/v1/recharge/orders`

### 6.8 展示边界

应用内展示接口只用于“显示资料和资产状态”，不要用它们直接完成扣费或发放权益。

- AI 调用、token/图片/视频计费、套餐权益预留和扣减：只通过 Koffy Gateway 完成。
- 修改头像、昵称、绑定微信、绑定手机号、修改密码：可以在 Koffy 用户中心完成；业务应用只建议提供跳转入口。
- 充值：推荐跳转 Koffy 充值页；只有确实需要深度嵌入支付流程时，业务应用再调用创建充值订单接口。
- 当前应用套餐展示：`subscriptions` 和 `entitlements` 当前返回用户所有应用的数据，业务应用应按自己的 `app_code` 过滤。
- 没有套餐时：展示点数余额和“去充值”入口即可，不应把空套餐理解为异常。
- 没有定价或未授权模型时：Koffy Gateway 会拒绝调用，业务应用应提示“当前服务暂不可用，请稍后再试或联系管理员”。

## 7. 业务应用调用 Koffy Gateway

业务应用服务端向 Koffy Gateway 发起请求，并携带三类头：

```http
Authorization: Bearer <koffy_access_token>
X-App-Key: <application_api_key>
Idempotency-Key: <unique_request_key>
```

- `Authorization` 来自登录码交换得到的 Koffy access token；兼容已有 Casdoor access token。
- `X-App-Key` 来自管理后台生成的应用密钥。
- `Idempotency-Key` 是业务应用为“本次用户操作”生成的唯一键。网络重试时复用同一个键；用户发起新的生成任务时必须使用新键。

Chat Completions 示例：

```bash
curl -X POST https://gateway.koffy.example.com/v1/chat/completions \
  -H 'Content-Type: application/json' \
  -H 'Authorization: Bearer <koffy_access_token>' \
  -H 'X-App-Key: <application_api_key>' \
  -H 'Idempotency-Key: image-studio-chat-20260528-000001' \
  -d '{
    "model": "openai-chat-default",
    "messages": [
      {"role": "user", "content": "写一段产品介绍"}
    ]
  }'
```

流式调用仍使用同一接口：

```bash
curl -N -X POST https://gateway.koffy.example.com/v1/chat/completions \
  -H 'Content-Type: application/json' \
  -H 'Authorization: Bearer <koffy_access_token>' \
  -H 'X-App-Key: <application_api_key>' \
  -H 'Idempotency-Key: image-studio-stream-20260528-000001' \
  -d '{
    "model": "openai-chat-default",
    "stream": true,
    "messages": [
      {"role": "user", "content": "给我三个标题"}
    ]
  }'
```

图片生成示例：

```bash
curl -X POST https://gateway.koffy.example.com/v1/images/generations \
  -H 'Content-Type: application/json' \
  -H 'Authorization: Bearer <koffy_access_token>' \
  -H 'X-App-Key: <application_api_key>' \
  -H 'Idempotency-Key: image-studio-image-20260528-000001' \
  -d '{
    "model": "image-default",
    "prompt": "一张用于文章封面的科技风产品图",
    "n": 1,
    "size": "1024x1024"
  }'
```

Koffy Gateway 会在调用模型前向 Koffy Billing API 预授权，调用完成后按实际 usage 结算。业务应用不需要自行计算用户余额或直接扣费。

## 8. 幂等与重复请求规则

`Idempotency-Key` 用来解决用户多次点击、前端超时重试、网络抖动导致的重复请求。

- 同一次用户操作的重试必须复用同一个 `Idempotency-Key`。
- 不同用户操作必须使用不同 `Idempotency-Key`。
- 不要使用固定值、用户 ID、日期等低粒度值作为幂等键。
- 建议格式：`<app_code>-<business_action>-<order_or_task_id>`。

如果相同幂等键已经完成扣费，网关会返回同一处理结果或拒绝冲突请求，避免重复扣费。

## 9. 用户侧流程

用户访问 `https://koffy.example.com/center` 可查看：

- 用户基础信息。
- 点数余额。
- 点数钱包流水。
- 已订购套餐。
- 使用记录，合并展示套餐权益消耗、额度刷新和点数扣费等用户可理解的信息。

用户访问“充值”页选择人民币金额后，系统按 `1 元 = 100 点数` 创建微信支付订单。微信内使用 JSAPI 调起支付；微信外使用 Native 二维码弹窗。支付回调验签成功后自动入账，用户侧充值记录只展示已支付成功的记录，完整订单状态和订单号在管理后台查看。

## 10. 常见错误处理

业务应用应按 HTTP 状态和错误码给用户明确提示：

- `401 missing_token` / `invalid_token`：用户未登录或 token 失效，引导重新登录。
- `403 invalid_app_key`：应用密钥错误或已停用，检查后台 App Key。
- `403 model_not_allowed`：应用未授权该模型，检查模型路由。
- `402 billing_authorize_failed` 或余额不足类错误：引导用户充值或升级套餐。
- `409 idempotency_conflict`：同一幂等键被用于不同请求，业务端应生成新的任务键。
- `429 rate_limited`：触发限流，按 `Retry-After` 后重试。
- `502 provider_failed`：上游模型服务失败，可提示稍后重试。

## 11. 接入验收清单

上线前建议逐项验证：

- 管理后台能看到应用、App Key、模型路由、扣费规则。
- 用户能通过手机号或微信登录用户中心。
- 业务应用能通过 `koffy_login_code` 换取 token，并建立自己的登录态。
- 用户能看到余额、套餐和权益余额。
- 业务应用后端调用 Koffy Gateway 时只使用服务端保存的 App Key。
- 同一次请求重复发送不会重复扣费。
- 套餐内调用优先扣权益，权益用完后按单位价格扣点数。
- 图片、视频、token 三类计费单位均按配置生效。
- 余额不足时，业务应用能引导用户充值。
- 管理后台能看到用量记录、充值订单和用户资产变化。

## 12. 相关文档

- [API 契约](api.md)
- [部署说明](deployment.md)
