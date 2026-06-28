# HateSentry

![Go Version](https://img.shields.io/badge/Go-1.21+-00ADD8?style=flat&logo=go)
![License](https://img.shields.io/badge/License-MIT-blue.svg)
![Docker](https://img.shields.io/badge/Docker-Ready-2496ED?style=flat&logo=docker)

HateSentry 是一个基于 Go 的文本内容审核网关。当前重点是接收文本审核请求，调用 OpenAI 或 Ollama 等 AI provider 生成风险建议，再由服务端策略转换为 `allow`、`review`、`block` 三类业务决策，并保存审核记录用于审计和后续人工复核。

项目中仍保留旧版 detection、RabbitMQ、Redis、Prometheus 等后端基础设施代码，但近期产品方向优先保证文本审核 MVP 可运行、可测试、可接入。图片、批量、完整异步队列、API Key 客户端接入、Webhook 和管理界面属于后续路线，不应视为当前已完整可用能力。

## ✨ 特性

### 核心功能
- **文本审核接口**：`POST /api/v1/moderation/check`，当前版本只面向文本内容。
- **服务端策略决策**：AI provider 只给出风险分数、标签和原因，最终 `allow` / `review` / `block` 由服务端策略生成。
- **策略配置**：支持通过 `config/config.yaml` 或环境变量配置策略版本、复核阈值和阻断阈值。
- **审核记录查询**：`GET /api/v1/moderation/results/:request_id` 可查询当前用户拥有的审核结果。
- **审计字段保存**：保存内容来源、外部 ID、提交者 ID、provider、model、标签、风险分数、决策、原因和策略版本。

### 技术架构
- **Web 框架**：Gin HTTP 路由和中间件。
- **认证方式**：当前公开审核接口使用 JWT Bearer Token。
- **AI provider**：已有 OpenAI 和 Ollama provider 适配。
- **持久化**：MySQL + GORM，审核请求和审核结果会写入数据库。
- **基础设施**：仓库包含 Redis 缓存、RabbitMQ 队列和 Prometheus 指标相关代码；这些能力仍需按实际路径逐步补齐和验证。

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
  "password": "string"
}
```

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

# JWT
JWT_SECRET=your-secret-key-change-in-production

# AI 服务
OPENAI_API_KEY=your-openai-api-key
OLLAMA_BASE_URL=http://localhost:11434

# 内容审核策略
MODERATION_POLICY_VERSION=default-v1
MODERATION_REVIEW_THRESHOLD=0.4
MODERATION_BLOCK_THRESHOLD=0.75

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

创建一个测试用户用于开发测试。

## ✅ 当前状态

### 已验证能力

- JWT 注册、登录和受保护路由。
- 同步文本审核接口：`POST /api/v1/moderation/check`。
- 审核结果查询接口：`GET /api/v1/moderation/results/:request_id`。
- 服务端策略决策：`allow`、`review`、`block`。
- 可配置策略阈值和策略版本。
- 审核请求与结果持久化。
- OpenAI 与 Ollama provider 的文本审核适配。
- 配置环境变量覆盖、路由注册、策略解析、provider 输出解析和服务层行为已有单元测试。

### 已有但仍需继续验证或补齐

- 旧版 `/api/v1/detection/*` 路由仍存在，但不是新的产品主线。
- Redis 缓存、RabbitMQ 队列和 Prometheus 监控相关代码已存在，但完整异步审核工作流、批量审核状态查询、真实图片审核和高并发承诺还没有作为 MVP 完成。
- API Key 外部客户端接入、Webhook 回调、人工复核队列和管理界面仍在路线图中。

## 🔒 安全与限制

- 当前审核 API 使用 JWT Bearer Token；API Key 客户端认证尚未完成。
- 密码不会在响应中返回，JWT Secret 可通过环境变量覆盖。
- 文本审核入口会校验必填内容、内容长度和元数据长度。
- 审核结果查询按当前登录用户和 `request_id` 过滤，避免跨用户读取。
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
- [x] 策略阈值配置
- [x] 统一错误处理框架
- [x] 健康检查和 Prometheus 指标入口
- [x] 结构化日志系统
- [x] Docker 部署支持

### 进行中 🚧
- [ ] 人工复核队列
- [ ] API Key 外部客户端认证
- [ ] 实现 Webhook 支持
- [ ] README、API 文档和运维文档持续按实现校准
- [ ] Docker Compose 端到端运行验证

### 计划中 📋
- [ ] 完整异步审核队列
- [ ] 批量审核状态和结果接口
- [ ] 真实图片审核（下载、校验、provider 图片 API）
- [ ] 管理后台 UI
- [ ] 数据导出功能
- [ ] 更完整的指标和审核延迟观测

---

**HateSentry** - 面向小型应用的文本内容审核网关
