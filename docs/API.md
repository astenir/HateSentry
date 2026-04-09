# HateSentry API 文档

## 基础信息

- **Base URL**: `http://localhost:8080/api/v1`
- **认证方式**: JWT Bearer Token
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

### 5. 重新生成 API Key

为当前用户生成新的 API Key。

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
  "error": "Rate limit exceeded"
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

- 每个用户每分钟最多 60 个请求
- 超过限制返回 `429 Too Many Requests`
- 限流基于 Redis 分布式实现

## 速率限制响应头

当启用速率限制时，响应可能包含以下头：

```
X-RateLimit-Limit: 60
X-RateLimit-Remaining: 55
X-RateLimit-Reset: 1609459200
```

## Webhook 支持（计划中）

支持通过 Webhook 推送异步检测结果。

配置方式将在后续版本中提供。
