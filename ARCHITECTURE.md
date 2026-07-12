# HateSentry 架构文档

## 当前产品边界

HateSentry 当前应被理解为一个文本内容审核网关，而不是完整的多模态检测平台。主线工作流是：

1. 外部应用或操作员提交文本审核请求。
2. AI provider 返回风险分数、标签和原因。
3. 服务端策略把 provider 建议转换为 `allow`、`review`、`block` 三类业务决策。
4. 审核请求、provider 建议、服务端决策和人工复核结果分开持久化。
5. 外部客户端可以通过 API Key 接入，并在配置 Webhook 后接收最终决策回调。

旧版 `/api/v1/detection/*`、RabbitMQ 队列、批量检测、图片提示词和监控代码仍存在于仓库中，但不属于当前 MVP 的稳定主线。它们需要在后续阶段重新验证或重构后，才能作为当前能力对外说明。

人工复核控制台位于 `web/`，生产构建由同一个 Go 服务在 `/console/` 提供。控制台与 `/api/v1/*` 保持同源，不新增独立认证、跨域 cookie 或第二套后端代理边界。Docker 镜像固定使用 `/app` 工作目录并把产物复制到 `/app/web/dist`；非容器运行时，进程工作目录下必须存在 `web/dist`。

## 高层结构

```text
External App / Operator
        |
        v
Gin Router + Auth
        |
        +-- JWT admin/operator routes
        |      - auth/profile/refresh
        |      - moderation result/history query
        |      - review queue and finalization
        |      - client/API key/webhook/policy management
        |
        +-- API Key moderation route
               - POST /api/v1/moderation/check
               - per-client Redis rate limit
               - external_id idempotency

Moderation Service
        |
        +-- Analyzer adapter
        |      - OpenAI text moderation prompt path
        |      - Ollama text moderation prompt path
        |
        +-- Policy engine
        |      - default policy
        |      - configured extra policy versions
        |      - allow/review/block threshold decision
        |
        +-- Persistence
        |      - moderation_requests
        |      - moderation_results
        |      - review_cases
        |      - client_applications
        |      - webhook_deliveries
        |
        +-- Webhook dispatch
               - signed HTTPS callback
               - delivery status record
               - manual retry for failed deliveries
```

## 主要模块

### HTTP 路由

入口在 `internal/router/router.go`。

当前主线接口分为四类：

- 公共认证与健康检查：`/api/v1/auth/*`、`/api/v1/health`。
- 文本审核：`POST /api/v1/moderation/check`，支持 JWT 或外部客户端 API Key。
- 操作员审核与复核：`GET /api/v1/moderation/results/:request_id`、`GET /api/v1/admin/moderation/results`、`/api/v1/reviews/*`。
- 外部接入管理：`/api/v1/admin/clients/*`、`/api/v1/admin/moderation/policies`、`/api/v1/admin/webhook-deliveries/*`。

旧版 detection 路由仍注册在 `/api/v1/detection/*`，用于保留早期原型兼容性。新的业务集成应优先使用 moderation 路由。

### 认证与授权

JWT 相关代码在 `internal/auth/jwt.go` 和 `internal/auth/middleware.go`。

API Key 相关代码在 `internal/auth/api_key.go`，客户端记录在 `internal/clients/*`。

当前边界：

- 管理端、复核队列、客户端管理和 Webhook 投递查询需要 JWT，并要求管理员角色。
- `POST /api/v1/moderation/check` 支持 JWT，也支持 `X-API-Key` 或 `Authorization: ApiKey <key>`。
- 客户端 API Key 只在创建或轮换时返回明文，数据库中保存哈希和短前缀。
- 停用客户端会让对应 API Key 认证失败，但不会删除历史审核记录。

### 文本审核服务

核心代码在 `internal/moderation/service.go`。

一次文本审核的主要步骤：

1. 校验并规范化 `content`、`source`、`external_id`、`actor_id`。
2. 如果是 API Key 客户端且提供了 `external_id`，先按客户端和外部 ID 查询既有结果。
3. 根据客户端配置选择策略版本；未配置时使用默认策略。
4. 调用 analyzer 获取 provider 建议。
5. 由服务端策略计算最终 `decision`。
6. 原子写入审核请求、审核结果，并在 `review` 决策时创建复核记录。
7. 对 `allow` / `block` 终态同步尝试发送 Webhook；`review` 待人工处理后再发送最终决策。

服务响应不会返回 provider 原始输出。原始输出只持久化在审核结果中，供后续审计和排查使用。

### AI Provider

当前 analyzer 复用 `internal/ai` 下的 provider 适配。

实现重点是文本审核：

- OpenAI provider。
- Ollama provider。
- 文本审核 prompt。
- provider JSON 输出解析和归一化。

图片 URL 或混合内容相关旧代码仍在仓库中，但当前主线 HTTP 审核接口不执行真实图片下载、文件类型校验、大小限制或 provider 图片输入提交。

### 策略决策

策略代码在 `internal/moderation/policy.go`。

当前决策只接受三种业务结果：

- `allow`
- `review`
- `block`

默认阈值：

- `risk_score < 0.4`: `allow`
- `0.4 <= risk_score < 0.75`: `review`
- `risk_score >= 0.75`: `block`

配置来源：

- `moderation.policy` 定义默认策略。
- `moderation.policies` 定义额外可分配策略版本。
- 环境变量 `MODERATION_POLICY_VERSION`、`MODERATION_REVIEW_THRESHOLD`、`MODERATION_BLOCK_THRESHOLD` 只覆盖默认策略。

外部客户端可以配置 `policy_version`。非空版本必须匹配已加载策略，否则客户端创建、策略更新或审核请求会失败。

### 持久化模型

主要模型在 `internal/models/models.go`。

当前 moderation 主线使用：

- `ClientApplication`: 外部客户端、API Key 哈希、Webhook URL、Webhook secret、策略版本和启用状态。
- `ModerationRequest`: 原始文本、来源、外部 ID、actor ID、客户端 ID 和幂等键。
- `ModerationResult`: provider/model、raw output、风险分、标签、服务端策略决策、原因和策略版本。
- `ReviewCase`: 人工复核状态、人工最终决策、处理人、备注和处理时间。
- `WebhookDelivery`: 最终决策回调的 delivery ID、状态、尝试次数、HTTP 状态、错误信息和 payload。

旧版 detection、stats、audit 和 feedback 模型仍存在，但不代表新的 moderation MVP 已经完整覆盖这些旧路径。

### 人工复核

复核工作流由 `internal/moderation/service.go` 和 `internal/handlers/moderation.go` 提供。

策略返回 `review` 时会自动创建 `ReviewCase`。管理员可以：

- 查询待复核或已处理记录。
- 查看单条复核记录。
- 通过复核，最终决策为 `allow`。
- 拒绝复核，最终决策为 `block`。
- 标记误判，并显式提交 `allow` 或 `block`。

人工最终决策保存在 `ReviewCase` 中，不覆盖 `ModerationResult.Decision`。这样可以同时保留 AI/provider 建议、服务端策略决策和人工最终判断。

### 外部客户端与 Webhook

客户端管理代码在 `internal/clients/*`。

Webhook 代码在 `internal/webhooks/*`，投递记录由 moderation repository 持久化。

当前能力：

- 管理员创建、查询、启停客户端。
- 管理员更新客户端名称、策略版本和 Webhook URL。
- 管理员轮换客户端 API Key。
- 配置 Webhook URL 时生成一次性 `webhook_secret`。
- 回调用 HMAC-SHA256 签名，并附带事件、delivery ID、时间戳和签名头。
- URL 校验要求 HTTPS，并拒绝 localhost、内网、链路本地、组播和云元数据地址。
- Webhook 首次投递同步执行；失败状态会持久化，管理员可以手动重试，后台 worker 也会按配置扫描并重试失败或超时停留在 `retrying` 的 delivery。

当前只保存每个 delivery 的最新状态和累计尝试次数，尚未实现逐次投递历史表或基于独立消息队列的投递系统。

### 缓存、限流和队列

Redis 当前在主线 moderation 中主要用于 API Key 客户端限流。限流键按客户端 ID 计算，成功检查会返回 `X-RateLimit-*` 响应头，超限返回 `Retry-After`。

旧版 detection cache、多级缓存和 RabbitMQ consumer 仍在仓库中。它们属于旧 detection 原型或后续阶段基础设施，不能视为 moderation MVP 的完整异步审核能力。

### 可观测性

项目包含 Prometheus metrics 中间件和 `/metrics` 端点，也保留了监控 compose 配置。

当前已记录 HTTP 请求、文本审核结果、人工复核完成延迟、Webhook 投递结果与耗时，以及后台重试批次结果。这些指标只使用固定枚举标签，不包含请求 ID、客户端 ID、URL 或错误文本。更完整的失败分类、运营仪表盘和告警规则仍需后续补齐并验证。

## 数据流

### JWT 文本审核

```text
Operator
  -> POST /api/v1/moderation/check
  -> JWT middleware
  -> ModerationHandler.Check
  -> ModerationService.Check
  -> Analyzer.AnalyzeText
  -> Policy.Decide
  -> Save ModerationRequest + ModerationResult + optional ReviewCase
  -> JSON response
```

JWT 调用当前不走客户端 API Key 限流，也不会触发客户端 Webhook，因为没有 `ClientID`。

### API Key 文本审核

```text
External App
  -> POST /api/v1/moderation/check
  -> API Key middleware
  -> Redis client rate limit
  -> external_id idempotency lookup
  -> client policy selection
  -> Analyzer.AnalyzeText
  -> Policy.Decide
  -> Save audit records
  -> If allow/block: signed Webhook attempt
  -> JSON response
```

当策略返回 `review` 时，接口会返回 `review`，并创建复核记录。最终 Webhook 等管理员复核后发送。

### 人工复核终态

```text
Admin
  -> POST /api/v1/reviews/:id/approve|reject|mark-mistake
  -> JWT admin role check
  -> lock pending ReviewCase
  -> save reviewer, status, final_decision, notes, reviewed_at
  -> if client webhook exists: signed Webhook attempt
  -> JSON response
```

重复处理已经终结的复核记录会返回冲突错误。

### Webhook 手动重试

```text
Admin
  -> POST /api/v1/admin/webhook-deliveries/:id/retry
  -> JWT admin role check
  -> atomically claim failed delivery
  -> load active client and saved payload
  -> dispatch signed HTTPS callback
  -> update delivery status, HTTP status, error message, attempt count
  -> JSON response
```

只有失败或过期 retrying 状态的投递可被领取重试。

## 配置

主配置文件为 `config/config.yaml`，环境变量覆盖在 `internal/config/config.go` 中显式处理。

部署相关覆盖包括：

- `DB_HOST`、`DB_PORT`、`DB_USERNAME`、`DB_PASSWORD`、`DB_DATABASE`
- `REDIS_HOST`、`REDIS_PORT`、`REDIS_PASSWORD`、`REDIS_DB`
- `RABBITMQ_HOST`、`RABBITMQ_PORT`、`RABBITMQ_USERNAME`、`RABBITMQ_PASSWORD`
- `JWT_SECRET`
- `OPENAI_API_KEY`、`OPENAI_BASE_URL`、`OPENAI_MODEL`
- `OLLAMA_BASE_URL`、`OLLAMA_MODEL`
- `MODERATION_POLICY_VERSION`、`MODERATION_REVIEW_THRESHOLD`、`MODERATION_BLOCK_THRESHOLD`
- `MODERATION_CLIENT_RATE_LIMIT`、`MODERATION_CLIENT_RATE_WINDOW`
- `LOG_LEVEL`、`LOG_FORMAT`、`LOG_OUTPUT`

Docker Compose 提供的数据库、Redis、RabbitMQ、JWT 和 AI provider 环境变量应在 config loader 中有对应覆盖。新增环境变量时必须同时更新配置加载和测试。

## 当前验证边界

默认验证命令：

```bash
go test ./...
go build ./...
make web-test
make web-build
```

真实浏览器复核闭环使用：

```bash
npm --prefix web exec playwright install chromium
make smoke-console-local
```

集成测试使用 `integration` build tag，并需要 MySQL：

```bash
HATESENTRY_TEST_DSN='root:password@tcp(127.0.0.1:3306)/hatesentry?charset=utf8mb4&parseTime=True&loc=Local' make test-integration
```

当前已有测试覆盖包括：

- 配置环境变量覆盖。
- 路由注册和访问保护。
- provider 输出解析。
- 策略阈值和策略版本。
- moderation service 行为。
- repository 部分持久化行为。
- API Key 认证和客户端管理。
- Webhook 签名、URL 校验、投递状态和重试路径。

## 后续阶段

以下能力不应在当前架构中作为已完成主线描述：

- 旧 detection 异步队列修复。
- 批量 moderation 状态和结果持久化。
- 真实图片下载、校验和 provider 图片输入。
- 独立 Webhook 投递队列和完整逐次尝试历史。
- 更完整的失败分类、运营仪表盘和告警规则。
- 客户端、策略、审核历史和 Webhook 管理界面。
