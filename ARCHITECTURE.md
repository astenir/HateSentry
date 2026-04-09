# HateSentry 架构文档

## 系统架构概览

```
┌─────────────────────────────────────────────────────────────┐
│                         Client Layer                         │
│  (Web, Mobile, CLI, API Consumers)                          │
└──────────────────────┬──────────────────────────────────────┘
                       │
                       ▼
┌─────────────────────────────────────────────────────────────┐
│                      API Gateway                            │
│              Gin HTTP Server + JWT Auth                     │
├─────────────────────────────────────────────────────────────┤
│  - Request Validation  - Rate Limiting  - CORS              │
│  - Authentication      - Error Handling                    │
│  - Logging Middleware  - Metrics Collection                  │
└──────────┬──────────────────────────────────────┬───────────┘
           │                                      │
           ▼                                      ▼
┌─────────────────────┐              ┌────────────────────────┐
│   Synchronous       │              │    Asynchronous        │
│   Detection         │              │    Detection           │
└──────────┬──────────┘              └────────────┬───────────┘
           │                                       │
           ▼                                       ▼
┌─────────────────────┐              ┌────────────────────────┐
│  AI Detection       │              │     RabbitMQ           │
│  Service            │              └────────────┬───────────┘
├─────────────────────┤                           │
│  - OpenAI Provider  │                           ▼
│  - Ollama Provider  │              ┌────────────────────────┐
│  - Chain of Thought │              │   Detection Worker    │
│  - Streaming        │              │   (Consumer)          │
└──────────┬──────────┘              ├────────────────────────┤
           │                         │  - Task Processing     │
           │                         │  - Result Storage     │
           │                         │  - Cache Update       │
           │                         └────────────────────────┘
           │                                       │
           └───────────────────┬───────────────────┘
                               │
                               ▼
              ┌──────────────────────────────────────┐
              │         Data Layer                   │
              ├──────────────────────────────────────┤
              │  MySQL (Persistent Storage)          │
              │  Redis (Cache & Session)            │
              └──────────────────────────────────────┘
                               │
                               ▼
              ┌──────────────────────────────────────┐
              │      Observability Layer             │
              ├──────────────────────────────────────┤
              │  - Structured Logging (Zap)        │
              │  - Prometheus Metrics              │
              │  - Health Checks                  │
              │  - Log Forwarding (ES/Logstash)   │
              └──────────────────────────────────────┘
```

## 核心组件

### 1. API Gateway (Gin)

**职责**:
- HTTP 请求处理和路由
- JWT 认证和授权
- 请求验证和限流
- 统一错误处理
- 响应格式化
- 请求/响应日志记录
- Prometheus 指标收集

**关键组件**:
- `internal/router/router.go`: 路由配置和中间件
- `internal/handlers/`: 请求处理器
  - `auth.go`: 认证相关处理
  - `detection.go`: 检测请求处理
  - `batch_detection.go`: 批量检测处理
  - `feedback.go`: 反馈处理
  - `health.go`: 健康检查
- `internal/auth/middleware.go`: JWT 认证中间件
- `internal/auth/jwt.go`: JWT 令牌管理
- `internal/auth/password.go`: 密码哈希
- `internal/errors/`: 统一错误处理框架
  - `app_errors.go`: 错误类型定义
  - `handler.go`: HTTP 错误响应处理
  - `validator.go`: 请求验证

**错误处理**:
- 统一的错误类型系统
- 结构化的错误响应
- 错误详情追踪
- HTTP 状态码自动映射

### 2. Detection Service

**职责**:
- 协调 AI 检测流程
- 管理 AI 提供者（OpenAI/Ollama）
- 提供同步、异步、流式检测接口
- 检测结果验证和处理

**关键组件**:
- `internal/ai/detection_service.go`: 检测服务核心
- `internal/ai/openai_provider.go`: OpenAI API 提供者
- `internal/ai/ollama_provider.go`: Ollama 本地提供者
- `internal/ai/prompt.go`: 提示词生成和管理
- `internal/ai/types.go`: 数据类型定义

**检测模式**:
1. **同步检测**: 直接返回结果
2. **异步检测**: 通过队列处理，返回任务ID
3. **流式检测**: Server-Sent Events (SSE) 实时推送
4. **批量检测**: 一次处理多个请求

### 3. Message Queue (RabbitMQ)

**职责**:
- 异步任务调度
- 请求队列管理
- 工作节点协调
- 任务优先级控制
- 任务重试机制

**关键组件**:
- `internal/queue/rabbitmq.go`: RabbitMQ 连接和队列管理
- `internal/queue/consumer.go`: 任务消费者接口
- `internal/app/app.go`: 任务处理器实现

**队列特性**:
- 持久化消息
- 消息确认机制
- 死信队列支持
- 连接重试逻辑

### 4. Cache Layer (Redis)

**职责**:
- 检测结果缓存
- 用户会话管理
- 速率限制
- 实时数据存储

**关键组件**:
- `internal/cache/redis.go`: Redis 客户端封装
- `internal/cache/detection_cache.go`: 检测结果缓存
- `internal/cache/rate_limiter.go`: 基于令牌桶的限流器
- `internal/cache/multilevel.go`: 多级缓存策略（L1内存 + L2 Redis）

**缓存策略**:
- **L1 缓存**: 应用内存缓存（sync.Map）
- **L2 缓存**: Redis 分布式缓存
- **L3 缓存**: 数据库持久化
- **缓存失效**: TTL 过期 + 主动失效

**智能缓存**:
- 内容相似度匹配
- 自动缓存预热
- LRU 淘汰策略

### 5. Database Layer (MySQL)

**职责**:
- 用户数据持久化
- 检测记录存储
- 统计数据分析
- 审计日志
- 数据库优化

**关键组件**:
- `internal/database/database.go`: 数据库连接和初始化
- `internal/database/optimizations.go`: 数据库优化
- `internal/models/models.go`: 数据模型定义
- `internal/models/feedback.go`: 反馈模型

**数据模型**:
- `User`: 用户信息（用户名、邮箱、角色、API密钥）
- `DetectionRequest`: 检测请求（内容、图片、状态）
- `DetectionResult`: 检测结果（分类、置信度、解释）
- `DetectionStats`: 检测统计（日统计、平均指标）
- `AuditLog`: 审计日志（操作记录）

**数据库优化**:
- 复合索引优化
- 查询性能分析
- 定期数据清理
- 分区表支持

## 数据流

### 同步检测流程

```
1. Client Request
   ↓
2. API Gateway (Auth & Validation)
   - JWT Token 验证
   - 请求参数验证
   - 速率限制检查
   ↓
3. Check Multi-Level Cache (L1 → L2)
   - L1: 应用内存缓存
   - L2: Redis 分布式缓存
   ↓
4. Cache Hit? → Return Cached Result
   ↓
5. Call Detection Service
   - 选择 AI 提供者
   - 构建提示词
   ↓
6. AI Provider (OpenAI/Ollama)
   - 发送检测请求
   - 接收响应
   ↓
7. Process & Parse Response
   - JSON 解析
   - 结果验证
   ↓
8. Store in Database (MySQL)
   - 检测结果持久化
   - 更新统计信息
   ↓
9. Cache Result (L1 & L2)
   - 写入内存缓存
   - 写入 Redis 缓存
   ↓
10. Return Response to Client
   - 统一 JSON 格式
   - 包含详细解释
```

### 异步检测流程

```
1. Client Request
   ↓
2. API Gateway (Auth & Validation)
   ↓
3. Create Request Record (MySQL)
   - 生成唯一 RequestID
   - 状态设为 pending
   ↓
4. Publish Task to RabbitMQ
   - 序列化任务数据
   - 发送到检测队列
   ↓
5. Return Request ID to Client
   - 202 Accepted
   - 包含查询结果的 URL
   ↓
6. Worker Consumes Task
   - 从队列获取任务
   - 更新状态为 processing
   ↓
7. Call Detection Service
   - 执行检测逻辑
   ↓
8. AI Provider Processing
   - 调用 AI API
   - 处理响应
   ↓
9. Store Result (MySQL)
   - 保存检测结果
   - 更新请求状态
   ↓
10. Cache Result (Redis)
   - 写入缓存
   ↓
11. Update Request Status
   - 状态设为 completed
   - 记录审计日志
```

### 流式检测流程 (SSE)

```
1. Client Request (stream=true)
   ↓
2. API Gateway (Auth & Validation)
   ↓
3. Establish SSE Connection
   - 设置响应头
   - 保持连接
   ↓
4. Start Detection Service
   - 初始化流式检测
   ↓
5. Stream Events from AI Provider
   - "start" 事件: 检测开始
   - "progress" 事件: 进度更新
   - "error" 事件: 错误信息
   ↓
6. Push Events to Client via SSE
   - 实时推送
   - JSON 格式事件
   ↓
7. Final Event (Result)
   - "result" 事件: 最终结果
   - 包含完整检测信息
   ↓
8. Store & Cache Result
   - 持久化到数据库
   - 写入缓存
   ↓
9. Close SSE Connection
   - 发送完成信号
   - 关闭连接
```

### 批量检测流程

```
1. Client Request (Batch)
   - 包含多个检测项
   ↓
2. API Gateway (Auth & Validation)
   - 验证批量请求
   - 检查配额
   ↓
3. Create Batch Request Record
   - 生成批量任务ID
   - 记录所有子请求
   ↓
4. Process in Parallel
   - 并发处理子请求
   - 复用检测逻辑
   ↓
5. Collect Results
   - 汇总所有结果
   - 统计成功率
   ↓
6. Store Batch Result
   - 保存批量结果
   - 更新统计信息
   ↓
7. Return Batch Response
   - 包含所有子结果
   - 批量状态摘要
```

## AI 架构

### 提示词工程

**系统提示词**:
- 定义仇恨言论检测标准和类别
- 指定输出格式（JSON）
- 提供判断准则和上下文
- 包含推理要求

**提示词策略**:
1. **基础提示词 (Basic)**: 简单直接的任务描述
   - 适用于简单文本检测
   - 快速响应

2. **链式推理提示词 (Chain of Thought)**: 分步思考过程
   - 提供推理步骤
   - 提高准确性
   - 用于复杂场景

3. **少样本提示词 (Few-Shot)**: 提供示例学习
   - 包含正负样本
   - 引导模型学习模式

4. **多模态提示词 (Multimodal)**: 文本和图片联合分析
   - 图像描述 + 文本分析
   - 综合判断

**提示词构建器**:
```go
type PromptBuilder struct {
    SystemPrompt string
    Strategy    PromptStrategy
}

type PromptStrategy string

const (
    StrategyBasic         PromptStrategy = "basic"
    StrategyChainOfThought PromptStrategy = "chain_of_thought"
    StrategyFewShot      PromptStrategy = "few_shot"
    StrategyMultimodal   PromptStrategy = "multimodal"
)
```

### 模型支持

**OpenAI**:
- **GPT-4 Vision Preview**: 多模态检测（文本+图片）
- **GPT-4 Turbo**: 高精度文本检测
- **GPT-3.5 Turbo**: 快速检测（成本优化）

**Ollama**:
- **LLaVA**: 视觉语言模型（本地多模态）
- **Llama 2**: 文本检测（本地部署）
- **Mistral**: 平衡性能和成本

**提供者接口**:
```go
type AIProvider interface {
    DetectHateSpeech(ctx context.Context, req *DetectionRequest) (*DetectionResponse, error)
    DetectHateSpeechWithImage(ctx context.Context, req *DetectionRequest, imageData []byte) (*DetectionResponse, error)
    DetectHateSpeechWithStreaming(ctx context.Context, req *DetectionRequest, callback func(*StreamDetectionEvent)) (*DetectionResponse, error)
    GetModel() string
}
```

### 可解释性

**检测结果包含**:
- `is_hate_speech`: 是否为仇恨言论（布尔值）
- `confidence`: 置信度分数（0.0-1.0）
- `categories`: 分类标签（数组）
  - 种族歧视
  - 性别歧视
  - 宗教仇恨
  - 暴力煽动
  - 其他仇恨言论
- `explanation`: 详细解释（自然语言）
- `model`: 使用的模型名称
- `processing_time`: 处理时间（毫秒）
- `prompt_used`: 使用的提示词（可选）
- `raw_response`: AI 原始响应（可选）

**流式事件类型**:
- `start`: 检测开始
- `progress`: 进度更新（包含内容片段）
- `result`: 最终结果
- `error`: 错误信息

## 安全架构

### 认证与授权

**JWT 认证流程**:
1. 用户登录验证
2. 生成 JWT Token（包含用户信息、角色、过期时间）
3. 客户端存储 Token（Bearer Token）
4. 请求时在 Authorization 头携带 Token
5. 服务端验证 Token 签名和有效期

**JWT Token 结构**:
```json
{
  "user_id": 123,
  "username": "john_doe",
  "role": "user",
  "exp": 1712884800,
  "iat": 1712798400
}
```

**角色权限**:
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

## 性能优化

### 缓存策略

**多层缓存架构**:
1. **L1 (内存)**: 应用内存缓存（sync.Map）
   - 超快速访问
   - 容量受限
   - 进程级缓存
2. **L2 (Redis)**: 分布式缓存
   - 跨实例共享
   - 持久化支持
   - 淘汰策略（LRU）
3. **L3 (数据库)**: 持久化存储
   - 完整数据
   - 事务支持
   - 索引优化

**缓存失效策略**:
- **基于时间失效 (TTL)**:
  - 检测结果: 1小时
  - 用户统计: 5分钟
  - 会话数据: 24小时
- **基于事件失效**:
  - 用户更新时清除相关缓存
  - 检测结果更新时清除缓存
  - 手动失效接口

**缓存预热**:
- 系统启动时加载热门数据
- 定时刷新统计数据
- 用户登录时加载常用数据

**智能缓存**:
- 内容相似度匹配
- 部分命中时降级返回
- 自动缓存相似结果（降低置信度）

### 连接池管理

**MySQL 连接池**:
- 最大连接数：100（可配置）
- 空闲连接数：10（可配置）
- 连接最大生命周期：1小时（可配置）
- 连接最大空闲时间：30分钟（可配置）

**Redis 连接池**:
- 池大小：100（可配置）
- 最小空闲连接：10（可配置）
- 连接超时：5秒
- 读写超时：3秒

**连接健康检查**:
- 定期 ping 检测
- 自动重连机制
- 连接泄露检测

### 异步处理

**RabbitMQ 任务队列**:
- 持久化消息队列
- 消息确认机制（ACK）
- 死信队列（DLQ）支持
- 消息 TTL 控制

**任务重试策略**:
- 指数退避重试
- 最大重试次数：3次
- 失败任务进入死信队列

**工作节点扩展**:
- 支持多个消费者并行处理
- 动态扩缩容（计划中）
- 任务优先级支持（计划中）

### 数据库优化

**索引优化**:
- 复合索引：`(user_id, created_at DESC)`
- 状态索引：`status`
- 处理状态索引：`(processed, created_at DESC)`
- 请求ID索引：`request_id`（唯一）
- 仇恨言论索引：`(is_hate_speech, created_at DESC)`

**查询优化**:
- 使用索引覆盖查询
- 避免 SELECT *
- 合理使用 JOIN
- 分页查询优化

**分区表**:
- 检测结果表按月分区
- 提升大数据量查询性能
- 简化数据清理

**慢查询监控**:
- 记录慢查询日志
- 自动分析优化建议
- 定期执行 ANALYZE

### 请求批处理

**批量检测 API**:
- 支持一次检测多个内容
- 减少网络开销
- 提高吞吐量
- 批量结果汇总

**批处理优化**:
- 并发处理子请求
- 批量数据库写入
- 批量缓存更新

## 扩展性设计

### 水平扩展

**API 层**:
- 无状态设计
- 支持负载均衡（Nginx、HAProxy）
- 会话数据存储在 Redis
- 多实例部署

**Worker 层**:
- 多实例部署
- RabbitMQ 任务分发
- 支持动态扩缩容
- 任务负载均衡

**数据层**:
- Redis 主从复制
- Redis 哨兵模式（高可用）
- MySQL 主从复制
- 读写分离（计划中）

### 垂直扩展

**资源优化**:
- CPU 多核利用
- 内存使用优化
- 磁盘 I/O 优化
- 网络带宽优化

**性能调优**:
- Go 运行时参数调整
- GC 参数优化
- 并发控制
- 资源限制

### 插件化

**AI 提供者**:
- 统一的提供者接口
- 易于添加新提供者
- 配置化切换
- 多提供者并行调用（计划中）

**检测策略**:
- 可配置的检测规则
- 支持自定义策略
- 规则热更新（计划中）
- A/B 测试支持（计划中）

**存储后端**:
- 抽象的存储接口
- 支持多种数据库
- 存储层切换
- 混合存储（计划中）

## 监控与日志

### 日志系统

**日志级别**:
- **Debug**: 详细调试信息（开发环境）
- **Info**: 一般信息（生产环境默认）
- **Warn**: 警告信息
- **Error**: 错误信息
- **Fatal**: 致命错误（程序退出）

**日志格式**:
- **JSON 格式**（生产环境）
  - 结构化日志
  - 易于解析
  - 支持日志聚合
- **控制台格式**（开发环境）
  - 彩色输出
  - 易于阅读

**日志上下文**:
- Request ID: 关联请求的所有日志
- User ID: 用户标识
- Trace ID: 分布式追踪
- Component: 组件名称

**日志转发**:
- **Elasticsearch**: 集中存储和搜索
- **Logstash**: 日志聚合和转换
- **聚合转发器**: 同时转发到多个目标

### 监控指标

**系统指标 (Prometheus)**:
- CPU 使用率
- 内存使用率
- 磁盘 I/O
- 网络流量
- Goroutine 数量
- 文件描述符数量

**HTTP 指标**:
- `http_requests_total`: 请求总数（按方法、端点、状态码）
- `http_request_duration_seconds`: 请求延迟（直方图）
- `http_request_size_bytes`: 请求大小
- `http_response_size_bytes`: 响应大小

**业务指标**:
- `detection_requests_total`: 检测请求总数
- `detection_duration_seconds`: 检测耗时
- `detection_confidence`: 检测置信度
- `hate_speech_detected_total`: 仇恨言论检测数
- `cache_hits_total`: 缓存命中数
- `cache_misses_total`: 缓存未命中数

**数据库指标**:
- `db_connections_active`: 活跃连接数
- `db_queries_total`: 查询总数
- `db_query_duration_seconds`: 查询耗时

**队列指标**:
- `queue_messages_published`: 发布消息数
- `queue_messages_consumed`: 消费消息数
- `queue_messages_failed`: 失败消息数
- `queue_consumer_lag`: 消费延迟

### 健康检查

**健康检查端点**:
- `/health`: 完整健康检查
- `/readiness`: 就绪检查
- `/liveness`: 存活检查

**检查项**:
- 数据库连接状态
- Redis 连接状态
- RabbitMQ 连接状态
- 磁盘空间
- 内存使用

**健康状态**:
- `healthy`: 所有检查通过
- `degraded`: 部分检查失败
- `unhealthy`: 关键检查失败

### 告警规则

**告警指标**:
- 错误率 > 5%
- 请求延迟 P99 > 1s
- 队列长度 > 1000
- 数据库连接数 > 90%
- 磁盘使用率 > 85%

**告警通道**:
- Email
- Slack/Teams
- PagerDuty
- Webhook

## 错误处理

### 统一错误框架

**错误类型**:
- `BadRequest`: 400 - 请求参数错误
- `Unauthorized`: 401 - 未认证
- `Forbidden`: 403 - 无权限
- `NotFound`: 404 - 资源不存在
- `Conflict`: 409 - 资源冲突
- `UnprocessableEntity`: 422 - 请求无法处理
- `RateLimitExceeded`: 429 - 超出速率限制
- `InternalServerError`: 500 - 服务器内部错误
- `ServiceUnavailable`: 503 - 服务不可用

**分类错误**:
- `ValidationError`: 验证错误
- `AuthenticationError`: 认证错误
- `AuthorizationError`: 授权错误
- `DatabaseError`: 数据库错误
- `CacheError`: 缓存错误
- `ExternalServiceError`: 外部服务错误
- `ConfigurationError`: 配置错误
- `Internal`: 内部错误

**错误响应格式**:
```json
{
  "error": {
    "code": "VALIDATION_ERROR",
    "message": "Invalid request parameters",
    "details": "content field is required",
    "request_id": "req_123456",
    "timestamp": "2026-04-09T18:00:00Z"
  }
}
```

**错误追踪**:
- 包含原始错误信息
- 错误堆栈记录（错误级别）
- 请求 ID 关联
- 结构化日志记录

## 部署架构

### Docker 部署

**容器化组件**:
- `hatesentry-api`: API 服务
- `hatesentry-worker`: Worker 服务
- `mysql:8.0`: 数据库服务
- `redis:7-alpine`: 缓存服务
- `rabbitmq:3-management`: 消息队列服务
- `prometheus`: 监控服务
- `grafana`: 可视化服务

**Docker Compose 配置**:
- `docker-compose.yml`: 基础服务
- `docker-compose.monitoring.yml`: 监控栈

**环境变量**:
- 数据库连接信息
- Redis 连接信息
- RabbitMQ 连接信息
- JWT Secret
- AI API Keys
- 日志级别

**优势**:
- 环境一致性
- 快速部署
- 易于扩展
- 版本控制

### 生产部署

**部署策略**:
- 蓝绿部署
- 滚动更新
- 金丝雀发布

**配置管理**:
- 环境变量
- 配置文件（YAML）
- 配置中心（计划中）

**服务发现**:
- 服务注册（计划中）
- 健康检查
- 负载均衡

## 未来规划

### 短期目标（1-2个月）
- [ ] 完善单元测试覆盖（目标 80%+）
- [ ] 添加集成测试
- [ ] 实现 Webhook 支持
- [ ] 完善监控和告警
- [ ] 添加 API 文档（Swagger/OpenAPI）
- [ ] 性能基准测试

### 中期目标（3-6个月）
- [ ] 支持多语言检测
- [ ] 实现检测规则配置化
- [ ] 添加管理后台 UI
- [ ] 支持自定义模型导入
- [ ] 实现读写分离
- [ ] 添加数据导出功能

### 长期目标（6-12个月）
- [ ] 支持 Edge 部署
- [ ] 实现联邦学习
- [ ] 开源社区生态
- [ ] 企业级功能（SSO、审计、合规）
- [ ] 多租户支持
- [ ] 实时流处理（Kafka）

## 技术债务

### 已知问题
- [ ] 需要添加更多单元测试
- [ ] 部分错误处理需要更细化
- [ ] 文档需要持续更新
- [ ] 缺少性能基准测试
- [ ] 日志格式需要统一

### 改进计划
- [ ] 优化数据库查询性能
- [ ] 增加更多中间件（请求追踪、限流增强）
- [ ] 改进错误追踪和调试
- [ ] 实现配置热更新
- [ ] 添加灰度发布支持
- [ ] 优化缓存策略

### 代码质量
- [ ] 代码审查流程
- [ ] CI/CD 流水线
- [ ] 代码覆盖率报告
- [ ] 静态代码分析
- [ ] 安全扫描

## 附录

### 技术栈

**后端框架**:
- Go 1.21+
- Gin Web Framework
- GORM ORM

**数据库**:
- MySQL 8.0+
- Redis 7+

**消息队列**:
- RabbitMQ 3.12+

**监控**:
- Prometheus
- Grafana
- Zap Logger

**AI 服务**:
- OpenAI API
- Ollama (本地)

**部署**:
- Docker
- Docker Compose

### 项目结构

```
hatesentry/
├── config/                    # 配置文件
│   ├── config.yaml            # 主配置
│   └── monitoring/            # 监控配置
├── docs/                      # 文档
│   ├── API.md                # API 文档
│   ├── LOGGING.md            # 日志文档
│   ├── MONITORING.md         # 监控文档
│   └── ERROR_HANDLING_MIGRATION.md  # 错误处理迁移文档
├── internal/                  # 内部代码
│   ├── ai/                  # AI 检测模块
│   │   ├── detection_service.go
│   │   ├── openai_provider.go
│   │   ├── ollama_provider.go
│   │   ├── prompt.go
│   │   └── types.go
│   ├── app/                 # 应用核心
│   │   └── app.go
│   ├── auth/                # 认证模块
│   │   ├── jwt.go
│   │   ├── middleware.go
│   │   └── password.go
│   ├── cache/               # 缓存模块
│   │   ├── redis.go
│   │   ├── detection_cache.go
│   │   ├── rate_limiter.go
│   │   └── multilevel.go
│   ├── config/              # 配置加载
│   │   └── config.go
│   ├── database/            # 数据库模块
│   │   ├── database.go
│   │   └── optimizations.go
│   ├── errors/              # 错误处理
│   │   ├── app_errors.go
│   │   ├── handler.go
│   │   └── validator.go
│   ├── handlers/            # HTTP 处理器
│   │   ├── auth.go
│   │   ├── detection.go
│   │   ├── batch_detection.go
│   │   ├── feedback.go
│   │   └── health.go
│   ├── logging/             # 日志模块
│   │   ├── logger.go
│   │   ├── middleware.go
│   │   ├── structured.go
│   │   └── forwarding.go
│   ├── middleware/          # 中间件
│   │   └── ...
│   ├── models/              # 数据模型
│   │   ├── models.go
│   │   └── feedback.go
│   ├── observability/       # 可观测性
│   │   ├── health.go
│   │   ├── metrics.go
│   │   └── middleware.go
│   ├── queue/               # 消息队列
│   │   ├── rabbitmq.go
│   │   └── consumer.go
│   └── router/              # 路由配置
│       └── router.go
├── scripts/                  # 脚本
│   ├── start.sh
│   └── stop.sh
├── ARCHITECTURE.md           # 架构文档（本文件）
├── Dockerfile                # Docker 配置
├── docker-compose.yml        # 基础服务
├── docker-compose.monitoring.yml  # 监控服务
├── go.mod                   # Go 模块定义
├── go.sum                   # 依赖校验和
├── main.go                  # 应用入口
├── Makefile                 # 构建脚本
├── README.md                # 项目说明
└── .gitignore               # Git 忽略规则
```

### 相关文档

- [README.md](./README.md) - 项目简介和快速开始
- [API.md](./docs/API.md) - API 接口文档
- [LOGGING.md](./docs/LOGGING.md) - 日志系统文档
- [MONITORING.md](./docs/MONITORING.md) - 监控系统文档
- [ERROR_HANDLING_MIGRATION.md](./ERROR_HANDLING_MIGRATION.md) - 错误处理迁移文档

---

**文档版本**: 2.0
**最后更新**: 2026-04-09
**维护者**: HateSentry Team
