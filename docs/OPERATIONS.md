# HateSentry 运维文档

**文档版本:** 1.0.0
**最后更新:** 2026-04-09
**适用范围:** HateSentry 系统运维和故障排查

---

## 📋 目录

1. [错误处理概述](#错误处理概述)
2. [错误码规范](#错误码规范)
3. [常见错误场景](#常见错误场景)
4. [错误排查指南](#错误排查指南)
5. [监控和告警](#监控和告警)
6. [故障恢复](#故障恢复)
7. [性能优化](#性能优化)

---

## 错误处理概述

### 错误响应格式

所有错误响应都遵循统一格式：

```json
{
  "error": "ERROR_CODE",
  "code": "ERROR_CODE",
  "message": "Human-readable error message",
  "details": "Additional error details (optional)",
  "severity": "low|medium|high|critical",
  "trace_id": "unique-trace-id",
  "timestamp": "2026-04-09T15:00:00Z"
}
```

### 错误严重级别

| 级别 | 说明 | HTTP 状态码示例 | 告警级别 |
|--------|------|-----------------|-----------|
| **low** | 一般性错误，不影响核心功能 | 400, 404 | INFO |
| **medium** | 影响部分功能，可继续使用 | 401, 403, 409 | WARNING |
| **high** | 影响重要功能，需要关注 | N/A | ERROR |
| **critical** | 系统核心功能失效 | 500, 502, 503 | CRITICAL |

---

## 错误码规范

### 错误码分类规则

错误码采用 4 位数字编码系统：

- **1000-1999**: 通用错误
- **2000-2999**: 认证和授权错误
- **3000-3999**: 数据库错误
- **4000-4999**: 检测服务错误
- **5000-5999**: 缓存错误
- **6000-6999**: 消息队列错误
- **7000-7999**: 外部服务错误

### 完整错误码对照表

#### 通用错误 (1000-1999)

| 错误码 | HTTP 状态 | 严重度 | 说明 | 常见原因 |
|--------|-----------|---------|------|---------|
| `INTERNAL_ERROR` | 500 | critical | 内部服务器错误 | 未捕获的异常 |
| `BAD_REQUEST` | 400 | medium | 请求参数错误 | 参数缺失或格式错误 |
| `UNAUTHORIZED` | 401 | high | 未授权访问 | 缺少或无效的认证 |
| `FORBIDDEN` | 403 | high | 禁止访问 | 权限不足 |
| `NOT_FOUND` | 404 | low | 资源未找到 | 请求的资源不存在 |
| `CONFLICT` | 409 | medium | 资源冲突 | 资源已存在 |
| `VALIDATION_ERROR` | 400 | medium | 数据验证失败 | 输入数据不符合规则 |
| `RATE_LIMIT_EXCEEDED` | 429 | medium | 超过速率限制 | 请求过于频繁 |

#### 认证授权错误 (2000-2999)

| 错误码 | HTTP 状态 | 严重度 | 说明 | 常见原因 |
|--------|-----------|---------|------|---------|
| `INVALID_TOKEN` | 401 | high | 无效的令牌 | Token 格式错误或被篡改 |
| `EXPIRED_TOKEN` | 401 | high | 令牌已过期 | Token 超过有效期 |
| `INVALID_CREDENTIALS` | 401 | high | 无效的凭证 | 用户名或密码错误 |
| `ACCOUNT_LOCKED` | 403 | high | 账户已锁定 | 多次登录失败 |
| `ACCOUNT_INACTIVE` | 403 | high | 账户未激活 | 账户状态为非活跃 |

#### 数据库错误 (3000-3999)

| 错误码 | HTTP 状态 | 严重度 | 说明 | 常见原因 |
|--------|-----------|---------|------|---------|
| `DATABASE_ERROR` | 500 | critical | 数据库操作失败 | SQL 错误、约束违反 |
| `RECORD_NOT_FOUND` | 404 | low | 记录未找到 | 查询条件不匹配 |
| `DUPLICATE_RECORD` | 409 | medium | 记录重复 | 唯一键冲突 |
| `DATABASE_CONNECTION_ERROR` | 500 | critical | 数据库连接失败 | 连接池耗尽、网络问题 |

#### 检测服务错误 (4000-4999)

| 错误码 | HTTP 状态 | 严重度 | 说明 | 常见原因 |
|--------|-----------|---------|------|---------|
| `DETECTION_FAILED` | 500 | critical | 检测失败 | AI 服务异常 |
| `CONTENT_REQUIRED` | 400 | medium | 需要提供内容 | 缺少必需的文本或图片 |
| `IMAGE_REQUIRED` | 400 | medium | 需要提供图片 | 图片分析需要图片数据 |
| `INVALID_CONTENT` | 400 | medium | 无效的内容格式 | 内容格式不支持 |
| `AI_PROVIDER_ERROR` | 500 | critical | AI 提供者错误 | OpenAI/第三方服务异常 |
| `MODEL_UNAVAILABLE` | 503 | critical | 模型不可用 | 模型加载失败 |

#### 缓存错误 (5000-5999)

| 错误码 | HTTP 状态 | 严重度 | 说明 | 常见原因 |
|--------|-----------|---------|------|---------|
| `CACHE_ERROR` | 500 | medium | 缓存操作失败 | Redis 连接问题 |
| `CACHE_MISS` | 404 | low | 缓存未命中 | 正常的缓存行为 |
| `REDIS_UNAVAILABLE` | 503 | high | Redis 不可用 | Redis 服务宕机 |

#### 消息队列错误 (6000-6999)

| 错误码 | HTTP 状态 | 严重度 | 说明 | 常见原因 |
|--------|-----------|---------|------|---------|
| `QUEUE_ERROR` | 500 | critical | 队列操作失败 | RabbitMQ 连接问题 |
| `QUEUE_FULL` | 503 | high | 队列已满 | 消息堆积过多 |
| `PUBLISH_FAILED` | 500 | high | 消息发布失败 | 网络或配置问题 |

#### 外部服务错误 (7000-7999)

| 错误码 | HTTP 状态 | 严重度 | 说明 | 常见原因 |
|--------|-----------|---------|------|---------|
| `EXTERNAL_SERVICE_ERROR` | 502 | high | 外部服务错误 | 第三方 API 故障 |
| `TIMEOUT` | 504 | high | 请求超时 | 网络延迟或服务响应慢 |
| `SERVICE_UNAVAILABLE` | 503 | critical | 服务不可用 | 外部服务宕机 |

---

## 常见错误场景

### 1. 认证相关错误

#### 场景 1.1: 用户登录失败

**错误响应:**
```json
{
  "error": "INVALID_CREDENTIALS",
  "code": "INVALID_CREDENTIALS",
  "message": "Invalid credentials",
  "severity": "high",
  "trace_id": "abc-123",
  "timestamp": "2026-04-09T15:00:00Z"
}
```

**可能原因:**
1. 用户名或密码错误
2. 账户被锁定
3. 账户未激活

**排查步骤:**
1. 检查数据库中的用户记录
2. 验证密码哈希是否匹配
3. 检查账户状态字段

**解决方案:**
- 用户确认凭证信息
- 重置用户密码
- 激活账户（如需要）

#### 场景 1.2: Token 刷新失败

**错误响应:**
```json
{
  "error": "EXPIRED_TOKEN",
  "code": "EXPIRED_TOKEN",
  "message": "Token has expired and is outside refresh window",
  "severity": "high",
  "trace_id": "abc-123",
  "timestamp": "2026-04-09T15:00:00Z"
}
```

**可能原因:**
1. Token 过期时间过长
2. 用户长时间未活动
3. 系统时间不同步

**排查步骤:**
1. 检查 Token 的 issued_at 和 expires_at
2. 验证服务器时间是否同步
3. 检查 Token 刷新窗口配置

**解决方案:**
- 用户重新登录
- 调整 Token 有效期配置
- 同步服务器时间

### 2. 检测服务错误

#### 场景 2.1: AI 检测失败

**错误响应:**
```json
{
  "error": "AI_PROVIDER_ERROR",
  "code": "AI_PROVIDER_ERROR",
  "message": "Failed to process detection request",
  "details": "OpenAI API timeout after 30s",
  "severity": "critical",
  "trace_id": "abc-123",
  "timestamp": "2026-04-09T15:00:00Z"
}
```

**可能原因:**
1. OpenAI API 服务不可用
2. API 密钥无效或过期
3. 请求超时
4. 配额耗尽

**排查步骤:**
1. 检查 OpenAI 服务状态页面
2. 验证 API 密钥配置
3. 查看 API 使用情况和配额
4. 检查网络连接

**解决方案:**
- 更新 API 密钥
- 增加超时配置
- 切换到备用 AI 提供者
- 购买额外的 API 配额

#### 场景 2.2: 内容验证失败

**错误响应:**
```json
{
  "error": "CONTENT_REQUIRED",
  "code": "CONTENT_REQUIRED",
  "message": "Content or image URL must be provided",
  "severity": "medium",
  "trace_id": "abc-123",
  "timestamp": "2026-04-09T15:00:00Z"
}
```

**可能原因:**
1. 请求中未提供内容
2. 内容格式不正确
3. 图片 URL 无效

**排查步骤:**
1. 检查请求 JSON 结构
2. 验证字段名称是否正确
3. 测试图片 URL 是否可访问

**解决方案:**
- 提供有效的文本内容或图片 URL
- 检查 API 文档确认请求格式

### 3. 数据库错误

#### 场景 3.1: 数据库连接失败

**错误响应:**
```json
{
  "error": "DATABASE_CONNECTION_ERROR",
  "code": "DATABASE_CONNECTION_ERROR",
  "message": "Failed to connect to database",
  "details": "connection refused",
  "severity": "critical",
  "trace_id": "abc-123",
  "timestamp": "2026-04-09T15:00:00Z"
}
```

**可能原因:**
1. 数据库服务未启动
2. 网络连接问题
3. 认证凭证错误
4. 连接池配置错误

**排查步骤:**
1. 检查数据库服务状态
2. 验证连接配置（主机、端口、用户名、密码）
3. 测试网络连通性
4. 检查防火墙规则

**解决方案:**
- 启动数据库服务
- 修正连接配置
- 调整防火墙规则
- 优化连接池配置

#### 场景 3.2: 记录重复错误

**错误响应:**
```json
{
  "error": "DUPLICATE_RECORD",
  "code": "DUPLICATE_RECORD",
  "message": "Username or email already exists",
  "severity": "medium",
  "trace_id": "abc-123",
  "timestamp": "2026-04-09T15:00:00Z"
}
```

**可能原因:**
1. 用户已存在
2. 唯一键冲突

**排查步骤:**
1. 查询数据库确认记录是否存在
2. 检查唯一约束定义

**解决方案:**
- 使用不同的用户名或邮箱
- 更新现有记录而非创建新记录

### 4. 缓存错误

#### 场景 4.1: Redis 连接失败

**错误响应:**
```json
{
  "error": "REDIS_UNAVAILABLE",
  "code": "REDIS_UNAVAILABLE",
  "message": "Redis cache is unavailable",
  "severity": "high",
  "trace_id": "abc-123",
  "timestamp": "2026-04-09T15:00:00Z"
}
```

**可能原因:**
1. Redis 服务未启动
2. 网络连接问题
3. Redis 内存不足

**排查步骤:**
1. 检查 Redis 服务状态
2. 使用 redis-cli 测试连接
3. 查看 Redis 日志

**解决方案:**
- 启动 Redis 服务
- 检查网络配置
- 增加 Redis 内存配置

### 5. 消息队列错误

#### 场景 5.1: RabbitMQ 发布失败

**错误响应:**
```json
{
  "error": "PUBLISH_FAILED",
  "code": "PUBLISH_FAILED",
  "message": "Failed to publish message to queue",
  "details": "channel closed",
  "severity": "high",
  "trace_id": "abc-123",
  "timestamp": "2026-04-09T15:00:00Z"
}
```

**可能原因:**
1. RabbitMQ 服务不可用
2. 队列不存在
3. 权限问题
4. 连接断开

**排查步骤:**
1. 检查 RabbitMQ 管理界面
2. 验证队列配置
3. 检查连接状态

**解决方案:**
- 重启 RabbitMQ 服务
- 重新声明队列
- 检查用户权限

---

## 错误排查指南

### 快速诊断流程

```
┌─────────────────────────────────────────────────────────┐
│                  收集错误信息                      │
│  - 错误码和消息                                 │
│  - Trace ID                                     │
│  - 时间戳                                       │
│  - 完整的错误堆栈                                │
└─────────────────────────────────────────────────────────┘
                        │
                        ▼
┌─────────────────────────────────────────────────────────┐
│               确定错误严重级别                     │
│  - Critical: 立即处理，影响所有用户               │
│  - High: 优先处理，影响部分功能                   │
│  - Medium: 计划处理，不影响核心功能                │
│  - Low: 记录日志，后续优化                        │
└─────────────────────────────────────────────────────────┘
                        │
                        ▼
┌─────────────────────────────────────────────────────────┐
│              查询错误码对照表                       │
│  - 确定错误类型和类别                            │
│  - 了解可能的原因和影响范围                        │
└─────────────────────────────────────────────────────────┘
                        │
                        ▼
┌─────────────────────────────────────────────────────────┐
│              检查系统状态                         │
│  - 数据库连接状态                                 │
│  - Redis 连接状态                                 │
│  - RabbitMQ 连接状态                              │
│  - 外部 API 服务状态                              │
│  - 服务器资源使用情况                              │
└─────────────────────────────────────────────────────────┘
                        │
                        ▼
┌─────────────────────────────────────────────────────────┐
│              执行针对性排查                         │
│  - 查看相关日志                                   │
│  - 检查监控指标                                   │
│  - 分析错误堆栈                                   │
│  - 复现问题（如果可能）                            │
└─────────────────────────────────────────────────────────┘
                        │
                        ▼
┌─────────────────────────────────────────────────────────┐
│              实施解决方案                           │
│  - 应用已知修复                                    │
│  - 重启相关服务                                    │
│  - 回滚最近变更                                    │
│  - 临时降级处理                                    │
└─────────────────────────────────────────────────────────┘
```

### 日志查询指南

#### 使用 Trace ID 追踪请求

```bash
# 搜索特定 trace_id 的所有日志
grep "trace_id=abc-123" /var/log/hatesentry/app.log

# 使用 jq 格式化 JSON 日志
grep "trace_id=abc-123" /var/log/hatesentry/app.log | jq '.'

# 按时间范围过滤
grep "trace_id=abc-123" /var/log/hatesentry/app.log | grep "2026-04-09"
```

#### 查找特定错误类型

```bash
# 查找所有 critical 级别错误
grep "severity=critical" /var/log/hatesentry/app.log

# 查找特定错误码
grep "ERROR_CODE" /var/log/hatesentry/app.log

# 查找最近的错误
tail -f /var/log/hatesentry/app.log | grep "error"
```

### 系统状态检查

#### 检查数据库状态

```bash
# 检查 MySQL 服务状态
systemctl status mysql

# 测试数据库连接
mysql -u hatesentry -p -h localhost -e "SELECT 1"

# 查看连接数
mysql -u hatesentry -p -e "SHOW STATUS LIKE 'Threads_connected'"

# 检查慢查询
mysql -u hatesentry -p -e "SHOW VARIABLES LIKE 'slow_query_log'"
```

#### 检查 Redis 状态

```bash
# 检查 Redis 服务状态
systemctl status redis

# 测试 Redis 连接
redis-cli ping

# 查看 Redis 信息
redis-cli info

# 检查内存使用
redis-cli info memory
```

#### 检查 RabbitMQ 状态

```bash
# 检查 RabbitMQ 服务状态
systemctl status rabbitmq-server

# 查看队列状态
rabbitmqctl list_queues

# 查看连接状态
rabbitmqctl list_connections

# 查看消费者状态
rabbitmqctl list_consumers
```

---

## 监控和告警

### 关键监控指标

#### 1. 系统级指标

| 指标 | 阈值 | 告警级别 | 说明 |
|--------|--------|-----------|------|
| CPU 使用率 | > 80% | WARNING | 持续高 CPU 使用 |
| 内存使用率 | > 85% | WARNING | 可能内存泄漏 |
| 磁盘使用率 | > 90% | CRITICAL | 可能导致服务不可用 |
| 网络流量 | > 10Gbps | WARNING | 可能 DDoS 攻击 |
| 磁盘 I/O | > 90% | WARNING | 磁盘瓶颈 |

#### 2. 应用级指标

| 指标 | 阈值 | 告警级别 | 说明 |
|--------|--------|-----------|------|
| 请求成功率 | < 99% | WARNING | 服务质量下降 |
| 请求延迟 (P95) | > 1s | WARNING | 响应慢 |
| 错误率 (5xx) | > 5% | CRITICAL | 大量错误 |
| 数据库连接数 | > 80% 最大值 | WARNING | 连接池压力 |
| 缓存命中率 | < 50% | INFO | 缓存效率低 |
| 队列堆积数 | > 1000 | WARNING | 消息处理慢 |

#### 3. 业务级指标

| 指标 | 阈值 | 告警级别 | 说明 |
|--------|--------|-----------|------|
| 检测请求量 | 异常下降 | WARNING | 服务不可用 |
| AI 调用失败率 | > 10% | WARNING | AI 服务问题 |
| 用户登录失败率 | > 20% | CRITICAL | 可能有攻击 |

### 告警配置示例

#### Prometheus 告警规则

```yaml
groups:
  - name: hatesentry_alerts
    interval: 30s
    rules:
      # 高错误率告警
      - alert: HighErrorRate
        expr: |
          rate(http_requests_total{status=~"5.."}[5m]) / rate(http_requests_total[5m]) > 0.05
        for: 5m
        labels:
          severity: critical
        annotations:
          summary: "High error rate detected"
          description: "Error rate is {{ $value | humanizePercentage }}"

      # 数据库连接池告警
      - alert: DatabaseConnectionPoolExhausted
        expr: |
          db_connections_active / db_connections_max > 0.8
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "Database connection pool nearly exhausted"
          description: "Usage: {{ $value | humanizePercentage }}"

      # AI 服务不可用告警
      - alert: AIServiceUnavailable
        expr: |
          up{job="ai-provider"} == 0
        for: 2m
        labels:
          severity: critical
        annotations:
          summary: "AI service is down"
          description: "AI provider has been down for more than 2 minutes"
```

---

## 故障恢复

### 紧急响应流程

#### 阶段 1: 识别和确认 (0-5 分钟)

1. **接收告警**
   - 确认告警类型和严重级别
   - 查看监控面板确认问题
   - 检查错误日志

2. **初步评估**
   - 确定影响范围（用户数、功能）
   - 判断是否需要立即处理
   - 通知相关团队

#### 阶段 2: 紧急修复 (5-30 分钟)

3. **快速修复**
   - 应用已知修复方案
   - 重启失败服务
   - 切换到备用系统（如果有）

4. **临时措施**
   - 启用降级模式
   - 限流保护
   - 临时扩容

#### 阶段 3: 恢复验证 (30-60 分钟)

5. **功能验证**
   - 测试核心功能
   - 验证错误率下降
   - 确认用户可正常使用

6. **监控观察**
   - 持续监控关键指标
   - 检查日志是否正常
   - 确认没有新问题

#### 阶段 4: 根因分析 (1-24 小时)

7. **深入调查**
   - 分析完整错误日志
   - 复现问题（如果可能）
   - 检查相关代码变更

8. **永久修复**
   - 实施长期解决方案
   - 更新文档和流程
   - 添加或修改监控

### 常见故障恢复场景

#### 场景 1: 数据库宕机

**应急措施:**
```bash
# 1. 尝试重启数据库
systemctl restart mysql

# 2. 检查错误日志
tail -100 /var/log/mysql/error.log

# 3. 启用只读模式（如果有主从）
# 配置应用使用从库进行读取
```

**验证步骤:**
```bash
# 测试连接
mysql -u hatesentry -p -e "SELECT 1"

# 检查服务状态
systemctl status mysql
```

#### 场景 2: Redis 故障

**应急措施:**
```bash
# 1. 重启 Redis
systemctl restart redis

# 2. 检查内存配置
redis-cli config get maxmemory

# 3. 清空缓存（如果需要）
redis-cli FLUSHALL
```

**应用降级:**
- 禁用缓存功能
- 直接查询数据库
- 监控数据库负载

#### 场景 3: AI 服务不可用

**应急措施:**
```bash
# 1. 检查配置
cat /etc/hatesentry/config.yaml | grep ai_provider

# 2. 切换到备用提供者
# 修改配置文件并重启服务
```

**应用降级:**
- 返回默认结果
- 使用本地模型（如果有）
- 暂停新请求，排队处理

#### 场景 4: 消息队列堆积

**应急措施:**
```bash
# 1. 查看队列状态
rabbitmqctl list_queues

# 2. 增加消费者数量
# 启动额外的 worker 实例

# 3. 清空队列（紧急情况）
rabbitmqctl purge_queue hatesentry_detection
```

---

## 性能优化

### 数据库优化

#### 1. 慢查询优化

```sql
-- 启用慢查询日志
SET GLOBAL slow_query_log = 'ON';
SET GLOBAL long_query_time = 1;

-- 查看慢查询
SELECT * FROM mysql.slow_log;

-- 优化建议
-- 1. 添加适当的索引
CREATE INDEX idx_user_id ON detection_requests(user_id);
CREATE INDEX idx_created_at ON detection_requests(created_at);

-- 2. 优化查询语句
-- 避免 SELECT *
-- 使用 LIMIT 限制结果
-- 使用 JOIN 替代子查询
```

#### 2. 连接池优化

```go
// 推荐配置
MaxOpenConns:    100,  // 根据应用负载调整
MaxIdleConns:    20,   // 通常是 MaxOpenConns 的 20%
ConnMaxLifetime:  30 * time.Minute,
```

### 缓存优化

#### 1. Redis 配置优化

```bash
# redis.conf
maxmemory 2gb
maxmemory-policy allkeys-lru
save 900 1
save 300 10
save 60 10000
```

#### 2. 缓存策略优化

```go
// 合理设置 TTL
// 热数据: 1 小时
// 温数据: 4 小时
// 冷数据: 24 小时

// 使用多级缓存
// L1: 内存缓存 (5 分钟)
// L2: Redis 缓存 (1 小时)
```

### 应用优化

#### 1. 并发控制

```go
// 使用 worker pool 限制并发
maxWorkers := 10
semaphore := make(chan struct{}, maxWorkers)

for _, task := range tasks {
    semaphore <- struct{}{}
    go func(t Task) {
        defer func() { <-semaphore }()
        process(t)
    }(task)
}
```

#### 2. 限流保护

```go
// 使用令牌桶算法
// 或滑动窗口算法
// 防止突发流量压垮系统
```

---

## 附录

### A. 错误码速查表

| 类别 | 错误码前缀 | 示例 |
|------|-------------|------|
| 通用 | 1xxx | INTERNAL_ERROR, BAD_REQUEST |
| 认证 | 2xxx | INVALID_TOKEN, INVALID_CREDENTIALS |
| 数据库 | 3xxx | DATABASE_ERROR, RECORD_NOT_FOUND |
| 检测 | 4xxx | DETECTION_FAILED, AI_PROVIDER_ERROR |
| 缓存 | 5xxx | CACHE_ERROR, REDIS_UNAVAILABLE |
| 队列 | 6xxx | QUEUE_ERROR, PUBLISH_FAILED |
| 外部 | 7xxx | EXTERNAL_SERVICE_ERROR, TIMEOUT |

### B. 联系方式

- **技术支持**: support@hatesentry.com
- **紧急热线**: +86-400-XXX-XXXX (24/7)
- **文档中心**: https://docs.hatesentry.com
- **状态页面**: https://status.hatesentry.com

### C. 相关文档

- [API 文档](./API.md)
- [日志文档](./LOGGING.md)
- [部署文档](./DEPLOYMENT.md)
- [架构文档](../ARCHITECTURE.md)

---

**文档维护者:** HateSentry 运维团队
**最后审核:** 2026-04-09
**下次审核:** 2026-07-09
