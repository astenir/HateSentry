# HateSentry API 文档

## 基础信息

- **Base URL**: `http://localhost:8080/api/v1`
- **认证方式**: JWT Bearer Token；`POST /moderation/check` 也支持外部客户端 API Key
- **Content-Type**: `application/json`

## 认证

### 1. 用户注册

创建新用户账号。

**端点**: `POST /auth/register`

**请求体**:
```json
{
  "username": "johndoe",
  "email": "john@example.com",
  "password": "password123"
}
```

**字段说明**:
- `username`: 用户名（3-50字符）
- `email`: 邮箱地址（有效邮箱格式）
- `password`: 密码（至少8字符）

**响应** (201 Created):
```json
{
  "token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
  "user": {
    "id": 1,
    "username": "johndoe",
    "email": "john@example.com",
    "role": "user"
  }
}
```

### 2. 用户登录

使用邮箱和密码登录。

**端点**: `POST /auth/login`

**请求体**:
```json
{
  "email": "john@example.com",
  "password": "password123"
}
```

**响应** (200 OK):
```json
{
  "token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
  "user": {
    "id": 1,
    "username": "johndoe",
    "email": "john@example.com",
    "role": "user"
  }
}
```

### 3. 刷新 Token

使用现有 token 获取新的 token。

**端点**: `POST /auth/refresh`

**请求头**:
```
Authorization: Bearer <token>
```

**响应** (200 OK):
```json
{
  "token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."
}
```

### 4. 获取用户信息

获取当前登录用户的信息。

**端点**: `GET /auth/profile`

**请求头**:
```
Authorization: Bearer <token>
```

**响应** (200 OK):
```json
{
  "id": 1,
  "username": "johndoe",
  "email": "john@example.com",
  "role": "user"
}
```

### 5. 重新生成用户 API Key（旧接口）

为当前用户生成新的旧版 API Key。外部系统接入文本审核时，应优先使用下面的“外部客户端”接口创建客户端 API Key。

**端点**: `POST /auth/api-key/regenerate`

**请求头**:
```
Authorization: Bearer <token>
```

**响应** (200 OK):
```json
{
  "api_key": "hs_550e8400-e29b-41d4-a716-446655440000"
}
```

## 外部客户端

外部客户端用于小型应用、评论系统或论坛等系统接入文本审核 API。客户端由管理员创建，创建响应会返回一次明文 `api_key`。如果配置了 `webhook_url`，创建响应还会返回一次 `webhook_secret`，用于验证后续回调签名。服务端只保存 API Key 哈希；Webhook secret 不会出现在查询客户端列表的响应中。

### 1. 创建客户端

**端点**: `POST /admin/clients`

**请求头**:
```
Authorization: Bearer <admin-token>
Content-Type: application/json
```

**请求体**:
```json
{
  "name": "blog-comments",
  "webhook_url": "https://example.com/moderation/webhook",
  "policy_version": "default-v1"
}
```

**字段说明**:
- `name`: 必填，客户端名称。
- `webhook_url`: 可选，接收审核最终决策回调的 HTTPS 地址；不允许 localhost、内网、链路本地、组播或元数据服务 IP。发送回调时也会检查域名解析结果，避免域名指向内部地址。
- `policy_version`: 可选，预留给后续客户端策略分配；当前审核仍使用服务端全局策略配置。

**响应** (201 Created):
```json
{
  "id": 11,
  "name": "blog-comments",
  "status": "active",
  "api_key": "hs_live_xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
  "api_key_prefix": "hs_live_xxxx",
  "webhook_secret": "whsec_xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
  "webhook_url": "https://example.com/moderation/webhook",
  "policy_version": "default-v1",
  "created_at": "2026-06-28T12:00:00Z"
}
```

### 2. 查询客户端

**端点**: `GET /admin/clients`

**请求头**:
```
Authorization: Bearer <admin-token>
```

**响应** (200 OK):
```json
{
  "items": [
    {
      "id": 11,
      "name": "blog-comments",
      "status": "active",
      "api_key_prefix": "hs_live_xxxx",
      "webhook_url": "https://example.com/moderation/webhook",
      "policy_version": "default-v1",
      "created_at": "2026-06-28T12:00:00Z",
      "updated_at": "2026-06-28T12:00:00Z"
    }
  ]
}
```

## 内容审核

### 1. 文本审核

提交一段文本，服务会调用当前配置的 AI provider 生成风险建议，再由服务端默认策略生成最终业务决策。

当前接口为同步处理，支持 JWT 认证或外部客户端 API Key 认证。API Key 客户端配置了 `webhook_url` 时，`allow` / `block` 会同步尝试发送最终决策回调；`review` 会先进入人工复核队列，待管理员复核后再发送最终决策回调。回调投递结果会记录为审计状态，失败投递支持管理员手动重试；当前版本不包含异步自动重试队列。

**端点**: `POST /moderation/check`

**JWT 请求头**:
```
Authorization: Bearer <token>
Content-Type: application/json
```

**API Key 请求头**:
```
X-API-Key: <client-api-key>
Content-Type: application/json
```

**请求体**:
```json
{
  "content": "user submitted text",
  "source": "comment",
  "external_id": "comment_123",
  "actor_id": "user_456"
}
```

**字段说明**:
- `content`: 必填，要审核的文本内容。当前版本只支持文本审核。
- `source`: 可选，内容来源，例如 `comment`、`forum_post`、`support_ticket`。为空时按 `api` 记录。
- `external_id`: 可选，外部系统中的内容 ID。
- `actor_id`: 可选，外部系统中的内容提交者 ID。

使用 API Key 调用时，如果同一客户端重复提交相同 `external_id`，接口会返回既有审核结果，并通过数据库唯一键避免创建重复审核记录。未提供 `external_id` 时，每次调用都会创建新审核记录。

API Key 调用会按客户端 ID 做请求级限流。默认配置为每个客户端每分钟 60 次 `POST /moderation/check`，可通过 `config/config.yaml` 的 `moderation.client_rate_limit` 或环境变量 `MODERATION_CLIENT_RATE_LIMIT`、`MODERATION_CLIENT_RATE_WINDOW` 调整。JWT 操作员调用当前不走这条客户端限流规则。

**响应** (200 OK):
```json
{
  "request_id": "550e8400-e29b-41d4-a716-446655440000",
  "decision": "review",
  "risk_score": 0.6,
  "labels": ["harassment", "identity_attack"],
  "reason": "Brief explanation suitable for operators",
  "policy_version": "default-v1"
}
```

**决策说明**:
- `allow`: 内容可以自动通过。
- `review`: 内容需要人工复核。
- `block`: 内容应自动拒绝或隐藏。

默认配置下的策略版本为 `default-v1`：
- `risk_score < 0.4`: `allow`
- `0.4 <= risk_score < 0.75`: `review`
- `risk_score >= 0.75`: `block`

可通过 `config/config.yaml` 的 `moderation.policy` 配置项，或环境变量 `MODERATION_POLICY_VERSION`、`MODERATION_REVIEW_THRESHOLD`、`MODERATION_BLOCK_THRESHOLD` 调整策略版本和阈值。

接口响应不会返回 provider 原始输出；原始输出仅作为审核记录存储，便于后续审计。

### 2. 获取文本审核结果

根据 `request_id` 查询当前用户拥有的文本审核记录。接口返回审核结果和基础审计元数据，但不会返回 provider 原始输出。

**端点**: `GET /moderation/results/:request_id`

**请求头**:
```
Authorization: Bearer <token>
```

**响应** (200 OK):
```json
{
  "request_id": "550e8400-e29b-41d4-a716-446655440000",
  "client_id": 11,
  "content": "user submitted text",
  "source": "comment",
  "external_id": "comment_123",
  "actor_id": "user_456",
  "status": "completed",
  "provider": "openai",
  "model": "gpt-4",
  "decision": "review",
  "risk_score": 0.6,
  "labels": ["harassment", "identity_attack"],
  "reason": "Brief explanation suitable for operators",
  "policy_version": "default-v1",
  "created_at": "2026-06-28T10:30:00Z"
}
```

### 3. 管理端查询最近审核历史

管理员可以查询最近的审核记录，用于运营审计和排查。该接口只返回有界列表，不是完整导出接口；响应不会返回 provider 原始输出。

**端点**: `GET /admin/moderation/results?decision=review&client_id=11&external_id=comment_123&limit=50`

**请求头**:
```
Authorization: Bearer <admin-token>
```

**查询参数**:
- `decision`: 可选，支持 `allow`、`review`、`block`。
- `client_id`: 可选，外部客户端 ID。
- `external_id`: 可选，外部系统内容 ID，最长 128 字符。
- `limit`: 可选，默认 50，最大 100。

**响应** (200 OK):
```json
{
  "items": [
    {
      "request_id": "550e8400-e29b-41d4-a716-446655440000",
      "client_id": 11,
      "content": "user submitted text",
      "source": "comment",
      "external_id": "comment_123",
      "actor_id": "user_456",
      "status": "completed",
      "provider": "openai",
      "model": "gpt-4",
      "policy_decision": "review",
      "review_status": "approved",
      "final_decision": "allow",
      "risk_score": 0.6,
      "labels": ["harassment"],
      "reason": "Brief explanation suitable for operators",
      "policy_version": "default-v1",
      "created_at": "2026-06-28T10:30:00Z"
    }
  ]
}
```

## 人工复核

当文本审核的服务端策略决策为 `review` 时，系统会自动创建一条复核记录。复核记录保存人工最终决策，不会覆盖原始 AI 建议或服务端策略决策。

当前复核接口使用 JWT 认证，并要求当前登录用户具备 `admin` 角色。复核记录保留原提交用户 ID，人工处理人会记录为 `reviewer_id`。

### 1. 查询复核队列

**端点**: `GET /reviews?status=pending`

**请求头**:
```
Authorization: Bearer <admin-token>
```

**查询参数**:
- `status`: 可选，支持 `pending`、`approved`、`rejected`、`mistake`。为空时默认查询 `pending`。

**响应** (200 OK):
```json
{
  "items": [
    {
      "id": 3,
      "request_id": "550e8400-e29b-41d4-a716-446655440000",
      "user_id": 7,
      "content": "user submitted text",
      "source": "comment",
      "external_id": "comment_123",
      "actor_id": "user_456",
      "status": "pending",
      "policy_decision": "review",
      "risk_score": 0.6,
      "labels": ["harassment"],
      "reason": "Brief explanation suitable for operators",
      "policy_version": "default-v1",
      "created_at": "2026-06-28T11:00:00Z"
    }
  ]
}
```

### 2. 通过复核

将待复核内容标记为可发布，人工最终决策为 `allow`。

**端点**: `POST /reviews/:id/approve`

**请求头**:
```
Authorization: Bearer <admin-token>
Content-Type: application/json
```

**请求体**:
```json
{
  "notes": "内容可发布"
}
```

**响应** (200 OK):
```json
{
  "id": 3,
  "request_id": "550e8400-e29b-41d4-a716-446655440000",
  "user_id": 7,
  "content": "user submitted text",
  "source": "comment",
  "status": "approved",
  "policy_decision": "review",
  "final_decision": "allow",
  "risk_score": 0.6,
  "labels": ["harassment"],
  "reason": "Brief explanation suitable for operators",
  "policy_version": "default-v1",
  "reviewer_id": 42,
  "review_notes": "内容可发布",
  "reviewed_at": "2026-06-28T11:05:00Z",
  "created_at": "2026-06-28T11:00:00Z"
}
```

### 3. 拒绝复核

将待复核内容标记为应拦截，人工最终决策为 `block`。

**端点**: `POST /reviews/:id/reject`

**请求头**:
```
Authorization: Bearer <admin-token>
Content-Type: application/json
```

**请求体**:
```json
{
  "notes": "内容需要拦截"
}
```

响应字段与通过复核一致，其中 `status` 为 `rejected`，`final_decision` 为 `block`。

### 4. 标记误判

将待复核内容标记为策略或 provider 处理误判，同时记录人工最终决策。`final_decision` 必须是 `allow` 或 `block`。

**端点**: `POST /reviews/:id/mark-mistake`

**请求头**:
```
Authorization: Bearer <admin-token>
Content-Type: application/json
```

**请求体**:
```json
{
  "final_decision": "allow",
  "notes": "策略过于保守"
}
```

响应字段与通过复核一致，其中 `status` 为 `mistake`。

### 5. 复核与审核统计

返回当前审核工作流的最小运营统计。`allowed` / `blocked` 会把自动策略终态和人工复核终态合并统计；待复核内容只计入 `pending_review`，不会提前计入 `allowed` 或 `blocked`。

**端点**: `GET /reviews/stats`

**请求头**:
```
Authorization: Bearer <admin-token>
```

**响应** (200 OK):
```json
{
  "total_moderated": 120,
  "allowed": 80,
  "blocked": 30,
  "pending_review": 10,
  "reviewed": 25,
  "mistakes": 2,
  "mistake_rate": 0.08
}
```

## 检测

### 1. 检测仇恨言论

检测文本、图片或混合内容是否包含仇恨言论。

**端点**: `POST /detection/detect`

**请求头**:
```
Authorization: Bearer <token>
```

**请求体**:
```json
{
  "content": "要检测的文本内容",
  "image_url": "https://example.com/image.jpg",
  "async": false,
  "stream": false
}
```

**字段说明**:
- `content`: 文本内容（可选，除非只检测图片）
- `image_url`: 图片 URL（可选）
- `async`: 是否异步处理（默认 false）
- `stream`: 是否流式返回（默认 false）

**注意**: `content` 和 `image_url` 至少需要提供一个。

#### 1.1 同步检测响应

**响应** (200 OK):
```json
{
  "id": 1,
  "request_id": "550e8400-e29b-41d4-a716-446655440000",
  "is_hate_speech": true,
  "confidence": 0.92,
  "categories": ["Racial/Ethnic discrimination"],
  "explanation": "This content contains explicit racial hate speech with derogatory language targeting a specific ethnic group.",
  "model": "gpt-4-vision-preview",
  "processing_time": 1234,
  "prompt_used": "...",
  "raw_response": "...",
  "created_at": "2024-01-01T00:00:00Z"
}
```

**字段说明**:
- `id`: 结果 ID
- `request_id`: 请求 ID
- `is_hate_speech`: 是否为仇恨言论
- `confidence`: 置信度（0.0-1.0）
- `categories`: 仇恨言论类别列表
- `explanation`: 详细解释
- `model`: 使用的模型
- `processing_time`: 处理时间（毫秒）

#### 1.2 异步检测响应

**响应** (202 Accepted):
```json
{
  "request_id": "550e8400-e29b-41d4-a716-446655440000",
  "status": "queued",
  "message": "Detection task queued for processing"
}
```

#### 1.3 流式检测响应

**响应类型**: Server-Sent Events (SSE)

**事件类型**:
1. `start`: 开始检测
```json
{
  "type": "start",
  "data": {
    "request_id": "550e8400-e29b-41d4-a716-446655440000"
  },
  "progress": 0.0
}
```

2. `progress`: 检测进度
```json
{
  "type": "progress",
  "data": {
    "content": "分析中..."
  },
  "progress": 0.5
}
```

3. `result`: 检测结果
```json
{
  "type": "result",
  "data": {
    "id": 1,
    "request_id": "550e8400-e29b-41d4-a716-446655440000",
    "is_hate_speech": false,
    "confidence": 0.95,
    "categories": [],
    "explanation": "This content does not contain hate speech.",
    ...
  },
  "progress": 1.0
}
```

4. `error`: 错误信息
```json
{
  "type": "error",
  "data": {
    "error": "Detection failed: ..."
  }
}
```

### 2. 获取检测结果

根据 request_id 获取检测结果。

**端点**: `GET /detection/result/:request_id`

**请求头**:
```
Authorization: Bearer <token>
```

**路径参数**:
- `request_id`: 请求 ID

**响应** (200 OK):
```json
{
  "id": 1,
  "request_id": "550e8400-e29b-41d4-a716-446655440000",
  "is_hate_speech": true,
  "confidence": 0.92,
  "categories": ["Racial/Ethnic discrimination"],
  "explanation": "...",
  "model": "gpt-4-vision-preview",
  "processing_time": 1234,
  "created_at": "2024-01-01T00:00:00Z"
}
```

**响应** (404 Not Found):
```json
{
  "error": "Result not found"
}
```

### 3. 获取检测历史

获取当前用户的检测历史记录。

**端点**: `GET /detection/history`

**请求头**:
```
Authorization: Bearer <token>
```

**查询参数**:
- `page`: 页码（默认 1）
- `limit`: 每页数量（默认 10）

**响应** (200 OK):
```json
{
  "data": [
    {
      "id": 1,
      "request_id": "550e8400-e29b-41d4-a716-446655440000",
      "user_id": 1,
      "content": "检测的文本内容",
      "image_url": "https://example.com/image.jpg",
      "content_type": "mixed",
      "processed": true,
      "status": "completed",
      "created_at": "2024-01-01T00:00:00Z",
      "detection_result": {
        "id": 1,
        "request_id": "550e8400-e29b-41d4-a716-446655440000",
        "is_hate_speech": false,
        "confidence": 0.95,
        "categories": [],
        ...
      }
    }
  ],
  "total": 100,
  "page": 1,
  "limit": 10
}
```

## 健康检查

### 获取服务健康状态

检查所有服务的健康状态。

**端点**: `GET /health`

**响应** (200 OK):
```json
{
  "status": "healthy",
  "timestamp": "2024-01-01T00:00:00Z",
  "services": {
    "database": "healthy",
    "redis": "healthy",
    "rabbitmq": "healthy"
  }
}
```

**响应** (503 Service Unavailable):
```json
{
  "status": "unhealthy",
  "timestamp": "2024-01-01T00:00:00Z",
  "services": {
    "database": "healthy",
    "redis": "unhealthy: connection refused",
    "rabbitmq": "healthy"
  }
}
```

## 错误响应

所有错误响应都遵循以下格式：

```json
{
  "error": "错误描述信息"
}
```

### 常见 HTTP 状态码

- `200 OK`: 请求成功
- `201 Created`: 资源创建成功
- `202 Accepted`: 请求已接受，异步处理中
- `400 Bad Request`: 请求参数错误
- `401 Unauthorized`: 未授权或 token 无效
- `403 Forbidden`: 权限不足
- `404 Not Found`: 资源不存在
- `429 Too Many Requests`: 请求频率超限
- `500 Internal Server Error`: 服务器内部错误
- `503 Service Unavailable`: 服务不可用

### 错误示例

**认证错误** (401):
```json
{
  "error": "Unauthorized"
}
```

**参数错误** (400):
```json
{
  "error": "content is required"
}
```

**限流错误** (429):
```json
{
  "error": "RATE_LIMIT_EXCEEDED",
  "code": "RATE_LIMIT_EXCEEDED",
  "message": "Client rate limit exceeded",
  "severity": "low",
  "timestamp": ""
}
```

**服务器错误** (500):
```json
{
  "error": "Internal server error"
}
```

## 使用示例

### cURL 示例

#### 用户注册
```bash
curl -X POST http://localhost:8080/api/v1/auth/register \
  -H "Content-Type: application/json" \
  -d '{
    "username": "testuser",
    "email": "test@example.com",
    "password": "password123"
  }'
```

#### 用户登录
```bash
curl -X POST http://localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{
    "email": "test@example.com",
    "password": "password123"
  }'
```

#### 文本检测
```bash
curl -X POST http://localhost:8080/api/v1/detection/detect \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer YOUR_TOKEN" \
  -d '{
    "content": "这是要检测的文本内容"
  }'
```

#### 异步检测
```bash
curl -X POST http://localhost:8080/api/v1/detection/detect \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer YOUR_TOKEN" \
  -d '{
    "content": "这是要检测的文本内容",
    "async": true
  }'
```

#### 获取检测结果
```bash
curl -X GET http://localhost:8080/api/v1/detection/result/REQUEST_ID \
  -H "Authorization: Bearer YOUR_TOKEN"
```

### JavaScript 示例

```javascript
// 用户登录
const login = async () => {
  const response = await fetch('http://localhost:8080/api/v1/auth/login', {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
    },
    body: JSON.stringify({
      email: 'test@example.com',
      password: 'password123',
    }),
  });
  const data = await response.json();
  return data.token;
};

// 检测仇恨言论
const detect = async (token, content) => {
  const response = await fetch('http://localhost:8080/api/v1/detection/detect', {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      'Authorization': `Bearer ${token}`,
    },
    body: JSON.stringify({ content }),
  });
  return await response.json();
};

// 使用
const token = await login();
const result = await detect(token, '要检测的文本内容');
console.log(result);
```

### Python 示例

```python
import requests

# 用户登录
def login():
    response = requests.post('http://localhost:8080/api/v1/auth/login', json={
        'email': 'test@example.com',
        'password': 'password123'
    })
    return response.json()['token']

# 检测仇恨言论
def detect(token, content):
    response = requests.post(
        'http://localhost:8080/api/v1/detection/detect',
        headers={
            'Content-Type': 'application/json',
            'Authorization': f'Bearer {token}'
        },
        json={'content': content}
    )
    return response.json()

# 使用
token = login()
result = detect(token, '要检测的文本内容')
print(result)
```

## 限流规则

- 外部客户端通过 API Key 调用 `POST /moderation/check` 时，按客户端 ID 限流。
- 默认限制为每个客户端每分钟 60 次，可通过 `moderation.client_rate_limit.limit` 和 `moderation.client_rate_limit.window` 配置。
- 对应环境变量为 `MODERATION_CLIENT_RATE_LIMIT` 和 `MODERATION_CLIENT_RATE_WINDOW`，例如 `60` 和 `1m`。
- `limit = 0` 或 `window = 0` 时，该客户端限流中间件不启用；负数无效。正数 `window` 必须不小于 `1s`。
- 限流基于 Redis 实现；当前版本不返回 `X-RateLimit-*` 响应头。
- 超过限制返回 `429 Too Many Requests` 和 `RATE_LIMIT_EXCEEDED` 错误码。

## Webhook 支持

客户端配置 `webhook_url` 后，服务会向该地址推送审核最终决策：

- `allow` / `block`: 文本审核完成后立即发送。
- `review`: 不立即发送最终决策；管理员复核通过、拒绝或标记误判后发送。
- 当前版本为同步单次尝试；最新投递状态、尝试次数和最后一次错误会记录到 `webhook_deliveries`，失败不会阻断审核记录保存或人工复核结果保存。
- 失败投递可由管理员手动重试；暂未实现异步自动重试队列。

**请求头**:
```http
Content-Type: application/json
X-HateSentry-Event: moderation.final_decision
X-HateSentry-Delivery: <uuid>
X-HateSentry-Timestamp: <unix-timestamp>
X-HateSentry-Signature: sha256=<hex-hmac>
```

签名计算方式：

```text
HMAC_SHA256(webhook_secret, "<timestamp>.<raw-json-body>")
```

**请求体**:
```json
{
  "event": "moderation.final_decision",
  "request_id": "550e8400-e29b-41d4-a716-446655440000",
  "client_id": 11,
  "external_id": "comment_123",
  "actor_id": "user_456",
  "source": "comment",
  "decision": "block",
  "review_status": "rejected",
  "risk_score": 0.82,
  "labels": ["harassment", "identity_attack"],
  "reason": "Policy threshold exceeded.",
  "policy_version": "default-v1",
  "created_at": "2026-06-28T12:00:00Z"
}
```

`review_status` 只会在人工复核产生最终决策时出现。

### 查询 Webhook 投递记录

管理员可以查询最近的 Webhook 投递记录，用于定位失败投递并取得手动重试所需的内部 `id`。

**端点**: `GET /admin/webhook-deliveries?status=failed&limit=50`

**查询参数**:

- `status`: 可选，支持 `succeeded`、`failed`、`retrying`。
- `limit`: 可选，默认 `50`，最大 `100`。

**请求头**:
```http
Authorization: Bearer <admin-token>
```

**响应** (200 OK):
```json
{
  "items": [
    {
      "id": 5,
      "delivery_id": "550e8400-e29b-41d4-a716-446655440000",
      "request_id": "550e8400-e29b-41d4-a716-446655440000",
      "client_id": 11,
      "event": "moderation.final_decision",
      "status": "failed",
      "attempt_count": 1,
      "last_attempt_at": "2026-06-28T12:00:00Z",
      "http_status": 500,
      "error_message": "webhook returned status 500",
      "created_at": "2026-06-28T12:00:00Z",
      "updated_at": "2026-06-28T12:00:00Z"
    }
  ]
}
```

### 手动重试 Webhook 投递

管理员可以对失败的 Webhook 投递记录发起一次手动重试。路径中的 `:id` 是投递记录的内部数字 ID，可通过投递记录查询接口获取。

**端点**: `POST /admin/webhook-deliveries/:id/retry`

**请求头**:
```http
Authorization: Bearer <admin-token>
```

**响应** (200 OK):
```json
{
  "id": 5,
  "delivery_id": "550e8400-e29b-41d4-a716-446655440000",
  "request_id": "550e8400-e29b-41d4-a716-446655440000",
  "client_id": 11,
  "event": "moderation.final_decision",
  "status": "succeeded",
  "attempt_count": 2,
  "last_attempt_at": "2026-06-28T12:05:00Z",
  "created_at": "2026-06-28T12:00:00Z",
  "updated_at": "2026-06-28T12:05:00Z"
}
```

只有 `status = "failed"` 的投递记录可以重试；刚进入 `retrying` 的记录会返回冲突错误，超过内部重试租约的陈旧 `retrying` 记录可以被重新认领。重试仍使用原始最终决策载荷和同一个 `X-HateSentry-Delivery` 标识。当前表保存最新状态和尝试次数，不保存每一次历史尝试的完整明细。
