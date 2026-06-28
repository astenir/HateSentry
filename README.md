# HateSentry

![Go Version](https://img.shields.io/badge/Go-1.21+-00ADD8?style=flat&logo=go)
![License](https://img.shields.io/badge/License-MIT-blue.svg)
![Docker](https://img.shields.io/badge/Docker-Ready-2496ED?style=flat&logo=docker)

HateSentry 是一个基于 Go 的内容安全检测服务，面向仇恨言论识别场景，集成 AI 模型调用、JWT 认证、异步任务队列、多级缓存、结构化日志和 Prometheus 监控等后端能力。

## ✨ 特性

### 核心功能
- 🤖 **AI 驱动检测**：集成 GPT-4 Vision、LLaVA 等大语言模型进行实时内容分析
- 🔄 **多模态支持**：支持文本、图片和混合内容的智能检测
- 🧠 **智能推理**：实现链式推理、少样本学习和自动提示生成策略
- 📊 **可解释性**：提供详细的检测结果、置信度评分和自然语言解释
- ⚡ **实时处理**：支持 Server-Sent Events (SSE) 流式响应，实时推送检测进度
- 📦 **批量检测**：支持一次性处理多个检测请求，提高吞吐量

### 技术架构
- 🌐 **Web 框架**：Gin 高性能 HTTP 框架，支持中间件和路由分组
- 🤖 **模型支持**：兼容 OpenAI API 和 Ollama 本地部署，支持模型热切换
- 📤 **输出方式**：同步/异步/流式检测，灵活适应不同场景
- 🔐 **认证授权**：JWT Token 认证和基于角色的权限控制 (RBAC)
- 🎯 **统一错误处理**：结构化错误类型、自动 HTTP 状态码映射、详细错误追踪
- 📈 **可观测性**：Prometheus 指标、结构化日志、健康检查、告警规则

### 性能优化
- 🗄️ **持久化存储**：MySQL 8.0 + GORM 2.0，连接池优化，索引和查询优化
- 💾 **三级缓存**：L1 内存 + L2 Redis + L3 数据库，智能缓存预热和失效策略
- 📨 **异步处理**：RabbitMQ 任务队列，支持任务优先级和重试机制
- 🚀 **高并发**：支持高并发突发请求处理，水平扩展能力
- ⚖️ **负载均衡**：令牌桶算法限流，防止 DDoS 和资源滥用

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

### 认证接口

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

#### 生成 API Key
```http
POST /api/v1/auth/apikey
Authorization: Bearer <token>
```

#### 删除 API Key
```http
DELETE /api/v1/auth/apikey
Authorization: Bearer <token>
```

### 检测接口

#### 文本检测
```http
POST /api/v1/detection/detect
Authorization: Bearer <token>
Content-Type: application/json

{
  "content": "要检测的文本内容",
  "async": false,
  "stream": false
}
```

#### 图片检测
```http
POST /api/v1/detection/detect
Authorization: Bearer <token>
Content-Type: application/json

{
  "image_url": "https://example.com/image.jpg",
  "async": false,
  "stream": false
}
```

#### 混合内容检测
```http
POST /api/v1/detection/detect
Authorization: Bearer <token>
Content-Type: application/json

{
  "content": "文本描述",
  "image_url": "https://example.com/image.jpg",
  "async": false,
  "stream": false
}
```

#### 流式检测 (SSE)
```http
POST /api/v1/detection/detect
Authorization: Bearer <token>
Content-Type: application/json

{
  "content": "要检测的文本内容",
  "stream": true
}

# 响应格式 (Server-Sent Events)
event: start
data: {"type":"start","timestamp":"2026-04-09T18:00:00Z"}

event: progress
data: {"type":"progress","content":"Analyzing...","progress":0.5}

event: result
data: {"type":"result","is_hate_speech":true,"confidence":0.92,...}
```

#### 异步检测
```http
POST /api/v1/detection/detect
Authorization: Bearer <token>
Content-Type: application/json

{
  "content": "要检测的文本内容",
  "async": true
}

Response:
{
  "request_id": "uuid",
  "status": "queued",
  "message": "Detection task queued for processing"
}
```

#### 获取检测结果
```http
GET /api/v1/detection/result/:request_id
Authorization: Bearer <token>
```

#### 获取检测历史
```http
GET /api/v1/detection/history?page=1&limit=10
Authorization: Bearer <token>
```

#### 批量检测
```http
POST /api/v1/detection/batch
Authorization: Bearer <token>
Content-Type: application/json

{
  "requests": [
    {"content": "文本内容1"},
    {"content": "文本内容2"}
  ]
}
```

#### 提交反馈
```http
POST /api/v1/detection/feedback
Authorization: Bearer <token>
Content-Type: application/json

{
  "detection_id": "uuid",
  "is_correct": true,
  "user_comment": "检测结果准确"
}
```

### 响应示例

#### 成功响应
```json
{
  "id": 1,
  "request_id": "550e8400-e29b-41d4-a716-446655440000",
  "is_hate_speech": true,
  "confidence": 0.92,
  "categories": ["种族歧视"],
  "explanation": "该内容包含明显的种族仇恨言论...",
  "model": "gpt-4-vision-preview",
  "processing_time": 1234,
  "created_at": "2026-04-09T18:00:00Z"
}
```

#### 错误响应
```json
{
  "error": {
    "code": "VALIDATION_ERROR",
    "message": "请求参数无效",
    "details": "content 字段是必需的",
    "request_id": "req_123456",
    "timestamp": "2026-04-09T18:00:00Z"
  }
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
  mode: "release"  # debug, release, test
  read_timeout: 30
  write_timeout: 30
  shutdown_timeout: 10

# 数据库配置
database:
  host: "localhost"
  port: 3306
  username: "root"
  password: "password"
  database: "hatesentry"
  max_open_conns: 100
  max_idle_conns: 10
  conn_max_lifetime: 3600
  conn_max_idle_time: 1800

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
  queue_name: "detection_queue"
  exchange_name: "detection_exchange"

# JWT 配置
jwt:
  secret: "your-secret-key-change-in-production"
  expires_in: 86400  # 24小时（秒）

# AI 模型配置
ai:
  provider: "openai"  # openai 或 ollama
  openai:
    api_key: "your-api-key"
    model: "gpt-4-vision-preview"
    base_url: "https://api.openai.com/v1"
  ollama:
    base_url: "http://localhost:11434"
    model: "llava:13b"

# 检测配置
detection:
  enable_image_analysis: true
  enable_text_analysis: true
  confidence_threshold: 0.7
  async_threshold: 5
  batch_size_limit: 100

# 缓存配置
cache:
  ttl_detection: 3600      # 检测结果缓存1小时
  ttl_stats: 300           # 统计数据缓存5分钟
  ttl_session: 86400       # 会话缓存24小时

# 限流配置
rate_limit:
  enabled: true
  requests_per_minute: 60
  burst: 10

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

## 📊 性能优化

### 三级缓存架构

**L1 缓存（内存）**:
- 使用 `sync.Map` 实现
- 超快速访问（微秒级）
- 容量有限，进程级别

**L2 缓存（Redis）**:
- 分布式共享缓存
- 支持持久化
- LRU 淘汰策略

**L3 缓存（数据库）**:
- 完整数据持久化
- 事务支持
- 索引优化查询

**缓存配置**:
- 检测结果: 1小时
- 统计数据: 5分钟
- 会话数据: 24小时

### 异步处理

**触发条件**:
- 并发请求数 > 5
- 大批量检测请求
- 长时间任务

**RabbitMQ 特性**:
- 持久化消息队列
- 消息确认机制（ACK）
- 死信队列（DLQ）支持
- 指数退避重试策略

### 连接池优化

**MySQL 连接池**:
- 最大连接数: 100
- 空闲连接数: 10
- 连接最大生命周期: 1小时
- 连接最大空闲时间: 30分钟

**Redis 连接池**:
- 池大小: 100
- 最小空闲连接: 10
- 连接超时: 5秒
- 读写超时: 3秒

### 数据库优化

**索引策略**:
- 复合索引: `(user_id, created_at DESC)`
- 状态索引: `status`
- 请求ID索引: `request_id` (唯一)
- 仇恨言论索引: `(is_hate_speech, created_at DESC)`

**查询优化**:
- 使用索引覆盖查询
- 避免 SELECT *
- 分页查询优化
- 定期执行 ANALYZE

### 限流保护

**限流策略**:
- 令牌桶算法
- 用户级别限流（基于 IP 和用户ID）
- 默认限制: 60 请求/分钟
- 突发请求: 10 请求

**防护目标**:
- 防止 DDoS 攻击
- 防止资源滥用
- 保护系统稳定性

## 🔒 安全

### 认证与授权

**JWT 认证**:
- Token 有效期: 24小时（可配置）
- 包含用户信息、角色、过期时间
- 支持 Token 刷新机制
- 安全的密钥存储

**角色权限 (RBAC)**:
- `user`: 普通用户权限
  - 执行检测
  - 查看历史记录
  - 管理个人资料
- `admin`: 管理员权限
  - 访问管理接口
  - 查看所有用户数据
  - 系统配置管理

**API Key 管理**:
- 为每个用户生成唯一 API Key
- 支持重新生成
- 用于 API 调用认证

### 数据安全

**密码安全**:
- bcrypt 加密存储（cost factor 10）
- 不在日志中记录密码
- 不在响应中返回密码

**敏感数据保护**:
- JWT Secret 环境变量配置
- API Key 唯一哈希存储
- 数据库连接信息加密

### 网络安全

**CORS**:
- 支持跨域请求
- 可配置允许的域名、方法、头

**速率限制**:
- 用户级别限流（基于 IP 和用户ID）
- 令牌桶算法
- 防止滥用和 DDoS

**输入验证**:
- 严格的请求参数验证
- 内容长度限制
- 防止注入攻击（SQL、XSS等）
- 图片格式和大小限制

**安全头**:
- X-Content-Type-Options: nosniff
- X-Frame-Options: DENY
- X-XSS-Protection: 1; mode=block

### 统一错误处理

**错误类型**:
- `ValidationError`: 请求参数验证错误
- `AuthenticationError`: 认证失败
- `AuthorizationError`: 权限不足
- `DatabaseError`: 数据库错误
- `ExternalServiceError`: 外部服务错误
- `ConfigurationError`: 配置错误

**错误响应**:
- 结构化的 JSON 格式
- 包含错误代码、消息、详情
- 请求 ID 关联
- 时间戳记录

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
- **OpenAI API**: GPT-4 Vision, GPT-4 Turbo
- **Ollama**: 本地 LLaVA, Llama 2 等模型

### 部署
- **Docker**: 容器化
- **Docker Compose**: 编排工具

## 🗺️ 发展路线

### 已完成 ✅
- [x] 基础检测功能（文本、图片、混合）
- [x] 同步、异步、流式检测模式
- [x] 批量检测 API
- [x] JWT 认证和授权
- [x] 多级缓存架构
- [x] RabbitMQ 异步处理
- [x] 统一错误处理框架
- [x] Prometheus 指标和健康检查
- [x] 结构化日志系统
- [x] 限流和安全保护
- [x] Docker 部署支持

### 进行中 🚧
- [ ] 完善单元测试覆盖（目标 80%+）
- [ ] 添加集成测试
- [ ] 实现 Webhook 支持
- [ ] 完善 API 文档（Swagger/OpenAPI）

### 计划中 📋
- [ ] 支持多语言检测
- [ ] 检测规则配置化
- [ ] 管理后台 UI
- [ ] 自定义模型导入
- [ ] 数据导出功能
- [ ] 实时流处理（Kafka）
- [ ] Edge 部署支持
- [ ] 企业级功能（SSO、审计、合规）
- [ ] 多租户支持

---

**HateSentry** - 保护数字世界的安全与和谐 🛡️
