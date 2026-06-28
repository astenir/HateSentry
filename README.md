# HateSentry

![Go Version](https://img.shields.io/badge/Go-1.21+-00ADD8?style=flat&logo=go)
![License](https://img.shields.io/badge/License-MIT-blue.svg)
![Docker](https://img.shields.io/badge/Docker-Ready-2496ED?style=flat&logo=docker)

HateSentry 是一个基于 Go 的文本内容审核网关。当前重点是接收文本审核请求，调用 OpenAI 或 Ollama 等 AI provider 生成风险建议，再由服务端策略转换为 `allow`、`review`、`block` 三类业务决策，并保存审核记录用于审计和后续人工复核。

项目中仍保留旧版 detection、RabbitMQ、Redis、Prometheus 等后端基础设施代码，但近期产品方向优先保证文本审核 MVP 可运行、可测试、可接入。图片、批量、完整异步队列和管理界面属于后续路线，不应视为当前已完整可用能力。

## ✨ 特性

### 核心功能
- **文本审核接口**：`POST /api/v1/moderation/check`，当前版本只面向文本内容。
- **服务端策略决策**：AI provider 只给出风险分数、标签和原因，最终 `allow` / `review` / `block` 由服务端策略生成。
- **策略配置**：支持通过 `config/config.yaml` 或环境变量配置策略版本、复核阈值和阻断阈值。
- **策略版本查询**：管理员可查询当前已加载的审核策略版本和阈值，用于外部客户端策略分配。
- **审核记录查询**：`GET /api/v1/moderation/results/:request_id` 可查询当前用户拥有的审核结果。
- **管理端审核历史**：`GET /api/v1/admin/moderation/results` 可按 `decision`、`client_id`、`external_id` 查询最近审核记录。
- **人工复核队列**：`review` 决策会自动创建复核记录，管理员可查询待复核内容并执行通过、拒绝或标记误判。
- **外部客户端接入**：管理员可创建客户端 API Key；外部系统可使用 `X-API-Key` 调用文本审核接口。
- **客户端名称更新**：管理员可修改外部客户端展示名称，不影响该客户端 API Key、Webhook 或策略配置。
- **客户端状态管理**：管理员可停用或重新启用外部客户端；停用后该客户端 API Key 不再通过认证。
- **客户端密钥轮换**：管理员可重新生成外部客户端 API Key；旧 API Key 会立即失效，新密钥只返回一次。
- **客户端策略更新**：管理员可为外部客户端切换已配置的审核策略版本，或重置为默认策略。
- **客户端 Webhook 更新**：管理员可更新或清除外部客户端回调地址，并重新生成签名 secret。
- **Webhook 自动重试**：失败的最终决策回调会按持久化 delivery 记录后台重试，避免短暂下游故障只能靠人工补偿。
- **外部 ID 幂等**：同一客户端重复提交相同 `external_id` 时返回既有审核结果，避免重复创建活跃审核记录。
- **客户端限流**：API Key 调用文本审核接口时按客户端 ID 做 Redis 限流，默认每分钟 60 次。
- **审计字段保存**：保存内容来源、外部 ID、提交者 ID、provider、model、标签、风险分数、决策、原因和策略版本。

### 技术架构
- **Web 框架**：Gin HTTP 路由和中间件。
- **认证方式**：文本审核接口支持 JWT Bearer Token 或外部客户端 API Key；管理和复核接口仍需要 JWT 管理员身份。
- **AI provider**：已有 OpenAI 和 Ollama provider 适配。
- **持久化**：MySQL + GORM，审核请求和审核结果会写入数据库。
- **基础设施**：仓库包含 Redis 缓存、RabbitMQ 队列和 Prometheus 指标相关代码；当前已有 HTTP、文本审核、人工复核和 Webhook 投递/重试指标，其他能力仍需按实际路径逐步补齐和验证。

## 🚀 快速开始

### 使用 Docker Compose（推荐）

1. **克隆项目**
   ```bash
   git clone https://github.com/yourusername/hatesentry.git
   cd hatesentry
   ```

2. **配置环境变量**
   ```bash
   cp .env.example .env
   # 编辑 .env 文件，配置 OpenAI API Key 或使用 Ollama
   # 如需初始化第一个管理员，先设置 ADMIN_BOOTSTRAP_TOKEN
   ```

3. **启动服务**
   ```bash
   # 启动所有服务（包括监控栈）
   make docker-up

   # 或者仅启动基础服务
   docker-compose up -d
   ```

4. **启动监控栈（可选）**
   ```bash
   docker-compose -f docker-compose.monitoring.yml up -d
   ```

5. **验证服务状态**
   ```bash
   # 健康检查
   make health

   # 查看日志
   make docker-logs
   ```

6. **访问服务**
   - API: http://localhost:8080
   - RabbitMQ UI: http://localhost:15672 (guest/guest)
   - Prometheus: http://localhost:9090 (监控栈)
   - Grafana: http://localhost:3000 (监控栈, admin/admin)
   - 健康检查: http://localhost:8080/api/v1/health

### 本地开发（热重载）

1. **安装依赖**
   ```bash
   make deps
   make install-deps
   ```

2. **启动依赖服务**
   ```bash
   docker-compose up -d mysql redis rabbitmq
   ```

3. **运行应用（热重载）**
   ```bash
   make dev
   ```

### 生产部署

1. **构建应用**
   ```bash
   make build
   ```

2. **配置环境变量**
   - 设置生产密钥（JWT Secret、数据库密码等）
   - 初始化第一个管理员前设置一次性 `ADMIN_BOOTSTRAP_TOKEN`，完成后移除或清空
   - 配置 AI API Keys
   - 设置日志级别为 production

3. **启动服务**
   ```bash
   docker-compose up -d
   ```

4. **监控和日志**
   - 集成 Prometheus + Grafana
   - 配置日志转发到 ELK 或其他日志系统
   - 设置告警规则

## 📖 API 文档

完整的 API 文档请参考 [docs/API.md](docs/API.md)

### 认证接口（JWT）

#### 用户注册
```http
POST /api/v1/auth/register
Content-Type: application/json

{
  "username": "string",
  "email": "string",
  "password": "string",
  "admin_bootstrap_token": "string"
}
```

空数据库中的第一个注册用户会在 `admin_bootstrap_token` 与服务端配置的 `ADMIN_BOOTSTRAP_TOKEN` 匹配时成为 `admin`，用于初始化外部客户端、API Key、审核策略和人工复核管理；缺少或错误 token 的空库注册请求会被拒绝。后续注册用户默认是 `user`，不需要也不会使用该 token。如果已有用户数据，`make create-user` 不会提升已有或新建用户权限，需要使用已有管理员账号操作管理端接口。

#### 用户登录
```http
POST /api/v1/auth/login
Content-Type: application/json

{
  "email": "string",
  "password": "string"
}
```

#### 刷新 Token
```http
POST /api/v1/auth/refresh
Authorization: Bearer <token>
```

#### 获取用户信息
```http
GET /api/v1/auth/profile
Authorization: Bearer <token>
```

### 内容审核接口

#### 文本审核
```http
POST /api/v1/moderation/check
Authorization: Bearer <token>
Content-Type: application/json

{
  "content": "用户提交的文本内容",
  "source": "comment",
  "external_id": "comment_123",
  "actor_id": "user_456"
}
```

#### 获取文本审核结果
```http
GET /api/v1/moderation/results/:request_id
Authorization: Bearer <token>
```

#### 管理端查询最近审核历史
```http
GET /api/v1/admin/moderation/results?decision=review&client_id=11&external_id=comment_123&limit=50
Authorization: Bearer <admin-token>
```

#### 创建外部客户端
```http
POST /api/v1/admin/clients
Authorization: Bearer <admin-token>
Content-Type: application/json

{
  "name": "blog-comments",
  "webhook_url": "https://example.com/moderation/webhook",
  "policy_version": "default-v1"
}
```

响应中的 `api_key` 只返回一次，服务端只保存哈希值。配置 `webhook_url` 时，响应还会返回一次 `webhook_secret`，用于验证后续最终决策回调的 HMAC 签名。

#### 更新外部客户端名称
```http
POST /api/v1/admin/clients/:id/name
Authorization: Bearer <admin-token>
Content-Type: application/json

{
  "name": "blog-comments-v2"
}
```

更新名称不会改变客户端状态、API Key、Webhook 或策略配置，响应不会返回 API Key 哈希或 Webhook secret。

#### 查询单个外部客户端
```http
GET /api/v1/admin/clients/:id
Authorization: Bearer <admin-token>
```

响应返回客户端当前状态、API Key 前缀、Webhook URL 和策略版本，不返回完整 API Key、API Key 哈希或 Webhook secret。

#### 查询可用审核策略
```http
GET /api/v1/admin/moderation/policies
Authorization: Bearer <admin-token>
```

响应会返回当前已加载的策略版本、复核阈值、阻断阈值和默认策略标记。该接口只读，不创建或修改策略；客户端 `policy_version` 仍通过下面的客户端策略接口分配。

#### 更新外部客户端策略
```http
POST /api/v1/admin/clients/:id/policy
Authorization: Bearer <admin-token>
Content-Type: application/json

{
  "policy_version": "strict-v1"
}
```

`policy_version` 必须匹配已配置的策略版本；传空字符串会重置为默认策略。响应不会返回 API Key 哈希或 Webhook secret。

#### 更新外部客户端 Webhook
```http
POST /api/v1/admin/clients/:id/webhook
Authorization: Bearer <admin-token>
Content-Type: application/json

{
  "webhook_url": "https://example.com/moderation/webhook"
}
```

非空 `webhook_url` 会保存新的 HTTPS 回调地址并返回一次新的 `webhook_secret`；旧 secret 立即失效。传空字符串会清除 Webhook URL 和 secret，后续不再发送回调。响应不会返回 API Key 哈希。

#### 轮换外部客户端 API Key
```http
POST /api/v1/admin/clients/:id/api-key/rotate
Authorization: Bearer <admin-token>
```

响应中的新 `api_key` 只返回一次，旧 API Key 会立即失效。轮换不会自动启用已停用的客户端，也不会修改 Webhook 或策略配置。若管理员并发触发多次轮换，最后完成的轮换返回的 API Key 才是当前有效密钥。

#### 使用 API Key 提交文本审核
```http
POST /api/v1/moderation/check
X-API-Key: <client-api-key>
Content-Type: application/json

{
  "content": "用户提交的文本内容",
  "source": "comment",
  "external_id": "comment_123",
  "actor_id": "user_456"
}
```

#### 查询复核队列
```http
GET /api/v1/reviews?status=pending
Authorization: Bearer <admin-token>
```

#### 查看单条复核记录
```http
GET /api/v1/reviews/:id
Authorization: Bearer <admin-token>
```

#### 查看复核与审核统计
```http
GET /api/v1/reviews/stats
Authorization: Bearer <admin-token>
```

#### 处理复核记录
```http
POST /api/v1/reviews/:id/approve
Authorization: Bearer <admin-token>
Content-Type: application/json

{
  "notes": "内容可发布"
}
```

```http
POST /api/v1/reviews/:id/reject
Authorization: Bearer <admin-token>
Content-Type: application/json

{
  "notes": "内容需要拦截"
}
```

```http
POST /api/v1/reviews/:id/mark-mistake
Authorization: Bearer <admin-token>
Content-Type: application/json

{
  "final_decision": "allow",
  "notes": "策略过于保守"
}
```

旧版 `/api/v1/detection/*` 路由仍存在于代码中，用于兼容已有检测原型；新的产品能力应优先使用 `/api/v1/moderation/*`。

### 审核响应示例

#### 成功响应
```json
{
  "request_id": "550e8400-e29b-41d4-a716-446655440000",
  "decision": "review",
  "risk_score": 0.6,
  "labels": ["harassment"],
  "reason": "Brief explanation suitable for operators",
  "policy_version": "default-v1"
}
```

#### 错误响应
```json
{
  "error": "VALIDATION_ERROR",
  "code": "VALIDATION_ERROR",
  "message": "content is required",
  "severity": "medium",
  "timestamp": ""
}
```

#### 健康检查响应
```json
{
  "status": "healthy",
  "version": "1.0.0",
  "checks": {
    "database": "healthy",
    "redis": "healthy",
    "rabbitmq": "healthy"
  },
  "timestamp": "2026-04-09T18:00:00Z"
}
```

## 🔧 配置说明

### 配置文件 (config/config.yaml)

```yaml
# 服务器配置
server:
  host: "0.0.0.0"
  port: 8080
  mode: "debug"
  read_timeout: 60s
  write_timeout: 60s

# 数据库配置
database:
  host: "localhost"
  port: 3306
  username: "root"
  password: "password"
  database: "hatesentry"
  charset: "utf8mb4"
  parse_time: true
  loc: "Local"
  max_idle_conns: 10
  max_open_conns: 100
  conn_max_lifetime: 3600s

# Redis 配置
redis:
  host: "localhost"
  port: 6379
  password: ""
  db: 0
  pool_size: 100
  min_idle_conns: 10

# RabbitMQ 配置
rabbitmq:
  host: "localhost"
  port: 5672
  username: "guest"
  password: "guest"
  vhost: "/"
  queue: "detection_tasks"
  exchange: "hatesentry"
  routing_key: "detection"

# 认证初始化配置
auth:
  admin_bootstrap_token: ""

# JWT 配置
jwt:
  secret: "your-secret-key-change-in-production"
  expire_hours: 24
  issuer: "hatesentry"

# AI 模型配置
ai:
  provider: "openai"  # openai 或 ollama
  openai:
    api_key: "your-api-key"
    base_url: "https://api.openai.com/v1"
    model: "gpt-4o-mini"
    max_tokens: 1000
    temperature: 0.3
  ollama:
    base_url: "http://localhost:11434"
    model: "llama3"
    max_tokens: 1000
    temperature: 0.3

# 检测配置
detection:
  enable_image_analysis: true
  enable_text_analysis: true
  confidence_threshold: 0.7
  async_threshold: 5
  max_concurrent_requests: 100
  result_cache_ttl: 3600s

# 内容审核策略
moderation:
  policy:
    version: "default-v1"
    review_threshold: 0.4
    block_threshold: 0.75
  policies:
    - version: "strict-v1"
      review_threshold: 0.2
      block_threshold: 0.5
  client_rate_limit:
    limit: 60
    window: 1m
  webhook_retry:
    enabled: true
    interval: 1m
    batch_size: 10
    max_attempts: 3

# 日志配置
logging:
  level: "info"  # debug, info, warn, error
  format: "json"  # json, console
  output: "stdout"
```

### 环境变量

优先级高于配置文件，用于敏感信息和部署配置：

```bash
# 数据库
DB_HOST=localhost
DB_PORT=3306
DB_USERNAME=root
DB_PASSWORD=password
DB_DATABASE=hatesentry

# Redis
REDIS_HOST=localhost
REDIS_PORT=6379
REDIS_PASSWORD=

# RabbitMQ
RABBITMQ_HOST=localhost
RABBITMQ_PORT=5672
RABBITMQ_USERNAME=guest
RABBITMQ_PASSWORD=guest

# 认证初始化
ADMIN_BOOTSTRAP_TOKEN=

# JWT
JWT_SECRET=your-secret-key-change-in-production

# AI 服务
OPENAI_API_KEY=your-openai-api-key
OLLAMA_BASE_URL=http://localhost:11434

# 内容审核策略
MODERATION_POLICY_VERSION=default-v1
MODERATION_REVIEW_THRESHOLD=0.4
MODERATION_BLOCK_THRESHOLD=0.75
MODERATION_CLIENT_RATE_LIMIT=60
MODERATION_CLIENT_RATE_WINDOW=1m
MODERATION_WEBHOOK_RETRY_ENABLED=true
MODERATION_WEBHOOK_RETRY_INTERVAL=1m
MODERATION_WEBHOOK_RETRY_BATCH_SIZE=10
MODERATION_WEBHOOK_RETRY_MAX_ATTEMPTS=3

# 日志
LOG_LEVEL=info
LOG_FORMAT=json
```

## 🏗️ 项目结构

```
hatesentry/
├── config/                      # 配置文件
│   ├── config.yaml              # 主配置
│   ├── monitoring/              # 监控配置
│   └── logging/                 # 日志配置
├── docs/                        # 文档
│   ├── API.md                   # API 文档
│   ├── MONITORING.md            # 监控文档
│   └── OPERATIONS.md            # 运维文档
├── internal/                    # 内部代码
│   ├── ai/                      # AI 检测模块
│   │   ├── detection_service.go # 检测服务核心
│   │   ├── openai_provider.go    # OpenAI 提供者
│   │   ├── ollama_provider.go   # Ollama 提供者
│   │   ├── prompt.go            # 提示词生成
│   │   └── types.go             # 类型定义
│   ├── app/                     # 应用核心
│   │   └── app.go               # 应用初始化和运行
│   ├── auth/                    # 认证模块
│   │   ├── jwt.go               # JWT 令牌管理
│   │   ├── middleware.go        # 认证中间件
│   │   └── password.go          # 密码哈希
│   ├── cache/                   # 缓存模块
│   │   ├── redis.go             # Redis 客户端
│   │   ├── detection_cache.go   # 检测结果缓存
│   │   ├── rate_limiter.go      # 限流器
│   │   └── multilevel.go        # 多级缓存
│   ├── config/                  # 配置加载
│   │   └── config.go            # 配置结构定义
│   ├── database/                # 数据库模块
│   │   ├── database.go          # 数据库连接
│   │   └── optimizations.go     # 数据库优化
│   ├── errors/                  # 错误处理
│   │   ├── app_errors.go        # 错误类型定义
│   │   ├── handler.go           # HTTP 错误处理
│   │   └── validator.go         # 请求验证
│   ├── handlers/                # HTTP 处理器
│   │   ├── auth.go              # 认证处理器
│   │   ├── detection.go         # 检测处理器
│   │   ├── batch_detection.go   # 批量检测处理器
│   │   ├── feedback.go          # 反馈处理器
│   │   └── health.go            # 健康检查
│   ├── logging/                 # 日志模块
│   │   ├── logger.go            # 日志配置
│   │   ├── middleware.go        # 日志中间件
│   │   ├── structured.go        # 结构化日志
│   │   └── forwarding.go        # 日志转发
│   ├── models/                  # 数据模型
│   │   ├── models.go            # 核心模型
│   │   └── feedback.go          # 反馈模型
│   ├── observability/           # 可观测性
│   │   ├── health.go            # 健康检查
│   │   ├── metrics.go           # Prometheus 指标
│   │   └── middleware.go        # 指标中间件
│   ├── queue/                   # 消息队列
│   │   ├── rabbitmq.go          # RabbitMQ 连接
│   │   └── consumer.go          # 消费者接口
│   └── router/                  # 路由配置
│       └── router.go            # 路由定义
├── scripts/                     # 脚本
│   ├── start.sh                 # 启动脚本
│   └── stop.sh                  # 停止脚本
├── ARCHITECTURE.md              # 架构文档
├── Dockerfile                   # Docker 配置
├── docker-compose.yml           # 基础服务
├── docker-compose.monitoring.yml # 监控服务
├── go.mod                       # Go 模块定义
├── go.sum                       # 依赖校验和
├── main.go                      # 应用入口
├── Makefile                     # 构建脚本
├── README.md                    # 项目说明
└── .env.example                 # 环境变量示例
```

## 🧪 开发

### 安装开发工具
```bash
make install-deps
```

这将安装以下工具：
- `golangci-lint`: Go 代码检查工具
- `swag`: Swagger API 文档生成器
- `air`: 热重载开发工具

### 代码格式化
```bash
make fmt
```

### 代码检查
```bash
make lint
```

### 运行测试
```bash
make test
```

默认测试不包含带 `integration` build tag 的 MySQL 集成测试。

### 运行集成测试
```bash
docker compose up -d mysql
HATESENTRY_TEST_DSN='root:password@tcp(127.0.0.1:3306)/hatesentry?charset=utf8mb4&parseTime=True&loc=Local' make test-integration
```

集成测试会通过 `go test -tags=integration ./...` 运行，需要可连接的 MySQL 测试库。部分测试会创建并删除临时数据库，测试账号需要 `CREATE DATABASE` 和 `DROP DATABASE` 权限；本地 Docker 示例中的 `root` 账号已满足该要求。

### 验证 Docker Compose 运行状态
```bash
make verify-compose
```

该命令会构建并启动 Compose 服务，然后等待 `GET /api/v1/health` 返回健康状态。健康响应需要确认 API 服务、MySQL、Redis 和 RabbitMQ 都可达。

### 测试覆盖率
```bash
make test-coverage
```

生成 `coverage.txt` 和 `coverage.html` 文件。

### 本地开发（热重载）
```bash
make dev
```

使用 air 实现代码修改后自动重新编译和运行。

### 生成 API 文档
```bash
make docs
```

使用 Swagger/OpenAPI 生成 API 文档。

### 健康检查
```bash
make health
```

检查服务健康状态。

### 创建测试用户
```bash
make create-user
```

使用当前 shell 中的 `ADMIN_BOOTSTRAP_TOKEN` 在空数据库中创建初始管理员用户用于开发测试；该值必须与运行中 API 服务加载的 `ADMIN_BOOTSTRAP_TOKEN` 相同。如果数据库已有用户，则该命令创建的是普通用户。示例：

```bash
export ADMIN_BOOTSTRAP_TOKEN="<same-token-configured-on-api-service>"
make create-user
```

## ✅ 当前状态

### 已验证能力

- JWT 注册、登录和受保护路由。
- 同步文本审核接口：`POST /api/v1/moderation/check`。
- 审核结果查询接口：`GET /api/v1/moderation/results/:request_id`。
- 管理端最近审核历史查询：`GET /api/v1/admin/moderation/results`，支持按 `decision`、`client_id`、`external_id` 过滤，默认返回 50 条，最多 100 条。
- 管理端审核策略查询：`GET /api/v1/admin/moderation/policies`，返回当前已加载的策略版本、复核阈值、阻断阈值和默认策略标记。
- 人工复核队列：`GET /api/v1/reviews?status=pending`。
- 单条复核记录查询：`GET /api/v1/reviews/:id`。
- 复核与审核统计：`GET /api/v1/reviews/stats`。
- 复核处理接口：`POST /api/v1/reviews/:id/approve`、`reject`、`mark-mistake`。
- 外部客户端管理：`POST /api/v1/admin/clients`、`GET /api/v1/admin/clients`、`GET /api/v1/admin/clients/:id`。
- 外部客户端名称更新：`POST /api/v1/admin/clients/:id/name`。
- 外部客户端状态管理：`POST /api/v1/admin/clients/:id/deactivate`、`POST /api/v1/admin/clients/:id/activate`。
- 外部客户端策略更新：`POST /api/v1/admin/clients/:id/policy`。
- 外部客户端 Webhook 更新：`POST /api/v1/admin/clients/:id/webhook`。
- 外部客户端 API Key 轮换：`POST /api/v1/admin/clients/:id/api-key/rotate`。
- API Key 文本审核接入：`X-API-Key` + `POST /api/v1/moderation/check`。
- 外部客户端策略分配：客户端 `policy_version` 会选择 `moderation.policy` 或 `moderation.policies` 中的已配置策略版本。
- 同一客户端的 `external_id` 幂等查询。
- API Key 客户端限流：`POST /api/v1/moderation/check` 默认按客户端 ID 每分钟 60 次。
- 基础 Webhook 最终决策回调：向客户端 HTTPS `webhook_url` 同步单次发送 `allow` / `block` 或人工复核后的最终决策，并使用 HMAC-SHA256 签名。
- Webhook 最新投递状态、尝试次数持久化、后台自动重试、管理端按 `status`、`client_id`、`request_id` 查询、单条查询和失败手动重试：`GET /api/v1/admin/webhook-deliveries`、`GET /api/v1/admin/webhook-deliveries/:id`、`POST /api/v1/admin/webhook-deliveries/:id/retry`。
- Prometheus 会记录文本审核结果、人工复核完成延迟、Webhook 投递结果/耗时和后台重试批次结果；这些指标只使用固定枚举标签，不包含请求 ID、客户端 ID、URL 或错误文本。
- 服务端策略决策：`allow`、`review`、`block`。
- 可配置策略阈值和策略版本。
- 审核请求与结果持久化。
- `review` 决策会自动创建复核记录，人工最终决策与 AI 建议、策略决策分开保存。
- OpenAI 与 Ollama provider 的文本审核适配。
- 配置环境变量覆盖、路由注册、策略解析、provider 输出解析和服务层行为已有单元测试。

### 已有但仍需继续验证或补齐

- 旧版 `/api/v1/detection/*` 路由仍存在，但不是新的产品主线。
- Redis 缓存、RabbitMQ 队列和 Prometheus 监控相关代码已存在，但完整异步审核工作流、批量审核状态查询、真实图片审核和高并发承诺还没有作为 MVP 完成。
- Webhook 已支持同步首次投递、失败记录、后台自动重试和失败手动重试，但仍只保存最新 delivery 状态，不是完整投递历史表；管理界面仍在路线图中。

## 🔒 安全与限制

- 当前审核 API 支持 JWT Bearer Token 和外部客户端 `X-API-Key`；客户端 API Key 只在创建或轮换时返回明文，数据库仅保存哈希值。
- 审核策略查询接口只读，只暴露当前配置的策略版本和阈值；不会修改配置或返回 provider 原始输出。
- 更新外部客户端名称只影响展示名称；不会改变客户端启用状态、API Key、Webhook 或策略配置，也不会返回 secret。
- 停用外部客户端会让其当前有效的 API Key 无法继续认证；重新启用后当前有效的 API Key 可继续使用。
- 轮换外部客户端 API Key 会让旧 key 立即失效；轮换不会改变客户端启用状态、Webhook 配置或策略版本；并发轮换时最后完成的轮换结果生效。
- 更新外部客户端策略版本只影响后续审核请求；不会修改客户端启用状态、API Key 或 Webhook 配置。空策略版本表示使用默认策略。
- 更新外部客户端 Webhook 会为非空 URL 生成新的 `webhook_secret`，旧 secret 立即失效；清空 URL 会同时清空 secret 并停止后续回调。
- Webhook 回调用创建客户端或最近一次 Webhook 更新返回的 `webhook_secret` 计算 HMAC-SHA256 签名；客户端列表不会返回 secret。`webhook_url` 仅支持 HTTPS，且会拒绝 localhost、内网、链路本地、组播和元数据服务 IP；发送时还会检查域名解析结果。
- Webhook 后台自动重试默认每分钟扫描失败或过期 `retrying` delivery，每批最多 10 条，最多尝试 3 次；可通过 `MODERATION_WEBHOOK_RETRY_*` 环境变量调整或关闭。
- 密码不会在响应中返回，JWT Secret 可通过环境变量覆盖。
- 文本审核入口会校验必填内容、内容长度和元数据长度。
- 审核结果查询按当前登录用户和 `request_id` 过滤，避免跨用户读取。
- 管理端审核历史接口需要 JWT 管理员身份，返回最近审核记录，不返回 provider 原始输出。
- 复核队列和复核处理需要管理员角色，人工处理人会记录为 `reviewer_id`。
- API Key 客户端重复提交相同 `external_id` 时会复用既有结果；未提供 `external_id` 时每次调用都会创建新记录。
- API Key 客户端限流依赖 Redis；限流检查成功执行时会返回 `X-RateLimit-Limit`、`X-RateLimit-Remaining` 和 `X-RateLimit-Reset`，超限时额外返回 `Retry-After`。
- 当前版本不会在 API 响应中返回 provider 原始输出；原始输出仅存入审核结果记录用于后续审计。

## 📝 许可证

MIT License - 详见 [LICENSE](LICENSE) 文件

## 🤝 贡献

欢迎贡献代码、报告 Bug 或提出新功能建议！

### 贡献流程

1. Fork 本仓库
2. 创建特性分支 (`git checkout -b feature/AmazingFeature`)
3. 提交更改 (`git commit -m 'Add some AmazingFeature'`)
4. 推送到分支 (`git push origin feature/AmazingFeature`)
5. 开启 Pull Request

### 代码规范

- 遵循 Go 官方代码风格
- 运行 `make fmt` 格式化代码
- 运行 `make lint` 检查代码
- 添加必要的单元测试
- 更新相关文档

## 📧 联系

如有问题或建议，欢迎通过以下方式联系：

- 提交 [Issue](https://github.com/yourusername/hatesentry/issues)
- 发送邮件: support@example.com
- 加入讨论组

## 📚 相关文档

- [ARCHITECTURE.md](ARCHITECTURE.md) - 架构设计文档
- [docs/API.md](docs/API.md) - API 接口文档
- [docs/MONITORING.md](docs/MONITORING.md) - 监控系统文档
- [docs/OPERATIONS.md](docs/OPERATIONS.md) - 运维部署文档

## 🛠️ 技术栈

### 后端框架
- **Go 1.21+**: 编程语言
- **Gin**: 高性能 HTTP 框架
- **GORM 2.0**: ORM 框架

### 数据库
- **MySQL 8.0+**: 关系型数据库
- **Redis 7+**: 缓存和会话存储

### 消息队列
- **RabbitMQ 3.12+**: 消息队列服务

### 监控
- **Prometheus**: 指标收集
- **Grafana**: 可视化监控
- **Zap**: 结构化日志

### AI 服务
- **OpenAI API**: 文本审核 provider
- **Ollama**: 本地文本审核 provider

### 部署
- **Docker**: 容器化
- **Docker Compose**: 编排工具

## 🗺️ 发展路线

### 已完成 ✅
- [x] JWT 认证和授权
- [x] 同步文本审核接口
- [x] 服务端 `allow` / `review` / `block` 策略决策
- [x] 审核结果持久化和查询
- [x] 管理端最近审核历史查询
- [x] 策略阈值配置
- [x] 管理端审核策略查询
- [x] 人工复核队列和复核处理接口
- [x] 复核与审核统计接口
- [x] API Key 外部客户端认证
- [x] 外部客户端名称更新
- [x] 外部客户端启用和停用
- [x] 外部客户端 API Key 轮换
- [x] 外部客户端策略版本更新
- [x] 外部客户端 Webhook URL 和 secret 更新
- [x] 外部客户端 `external_id` 幂等
- [x] 基础同步单次 Webhook 回调和 HMAC 签名
- [x] Webhook 最新投递状态、尝试次数持久化和失败手动重试
- [x] Webhook 后台自动重试
- [x] 统一错误处理框架
- [x] 健康检查和 Prometheus 指标入口
- [x] 文本审核结果和人工复核延迟 Prometheus 指标
- [x] Webhook 投递和后台重试 Prometheus 指标
- [x] 结构化日志系统
- [x] Docker 部署支持
- [x] Docker Compose 端到端健康检查验证

### 进行中 🚧
- [ ] 更完整的操作指标、失败分类和延迟观测
- [ ] README、API 文档和运维文档持续按实现校准

### 计划中 📋
- [ ] 完整异步审核队列
- [ ] 批量审核状态和结果接口
- [ ] 真实图片审核（下载、校验、provider 图片 API）
- [ ] 管理后台 UI
- [ ] 数据导出功能
- [ ] 指标仪表盘和告警建议

---

**HateSentry** - 面向小型应用的文本内容审核网关
