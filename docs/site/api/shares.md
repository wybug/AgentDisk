# 分享管理接口

提供外链分享的创建、查询、撤销，以及公开的分享访问和下载功能。分享接口分为两部分：**需要认证的管理接口**和**无需认证的公开访问接口**。

## 创建分享

为指定资源（文件或目录）创建外链分享。

```
POST /v1/disk/shares
```

### 认证方式

需要 JWT Bearer Token 或 API Key 认证。

### 请求体

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `resourceId` | `uint64` | 是 | 资源 ID（文件或目录的 ID） |
| `resType` | `string` | 是 | 资源类型：`file` 或 `folder` |
| `extractCode` | `string` | 否 | 提取码，最长 8 字符，不设置则无需提取码 |
| `maxVisit` | `int` | 否 | 最大访问次数，`-1` 表示不限制，默认 `-1` |
| `expireHours` | `int` | 否 | 有效时长（小时），默认 `72`（3 天） |

```json
{
  "resourceId": 10,
  "resType": "file",
  "extractCode": "abc123",
  "maxVisit": 100,
  "expireHours": 48
}
```

### 响应示例

**HTTP 201 Created**

```json
{
  "code": 0,
  "message": "created",
  "data": {
    "id": 1,
    "userId": "user_abc123",
    "resourceId": 10,
    "resType": "file",
    "shareCode": "a1b2c3d4",
    "extractCode": "abc123",
    "maxVisit": 100,
    "visitCount": 0,
    "expireAt": "2025-06-17T10:00:00Z",
    "isActive": true,
    "createdAt": "2025-06-15T10:00:00Z"
  }
}
```

### 响应字段说明

| 字段 | 类型 | 说明 |
|------|------|------|
| `id` | `uint64` | 分享记录 ID |
| `shareCode` | `string` | 分享码，用于构建分享链接 |
| `extractCode` | `string` | 提取码（敏感信息，访问时需验证） |
| `maxVisit` | `int` | 最大访问次数，`-1` 表示不限 |
| `visitCount` | `int` | 已访问次数 |
| `expireAt` | `string` | 过期时间（RFC 3339） |
| `isActive` | `bool` | 是否有效（手动撤销后为 `false`） |

### curl 示例

```bash
curl -X POST http://localhost:9100/v1/disk/shares \
  -H "Authorization: Bearer eyJhbGciOiJIUzI1NiIs..." \
  -H "Content-Type: application/json" \
  -d '{
    "resourceId": 10,
    "resType": "file",
    "extractCode": "abc123",
    "maxVisit": 100,
    "expireHours": 48
  }'
```

### 错误场景

| HTTP 状态码 | 场景说明 |
|------------|---------|
| 400 | 必填字段缺失或参数格式错误 |
| 401 | Token 无效或已过期 |

---

## 列出我的分享

获取当前用户创建的所有分享列表。

```
GET /v1/disk/shares
```

### 认证方式

需要 JWT Bearer Token 或 API Key 认证。

### 请求参数

无。

### 响应示例

```json
{
  "code": 0,
  "message": "success",
  "data": [
    {
      "id": 1,
      "userId": "user_abc123",
      "resourceId": 10,
      "resType": "file",
      "shareCode": "a1b2c3d4",
      "extractCode": "abc123",
      "maxVisit": 100,
      "visitCount": 23,
      "expireAt": "2025-06-17T10:00:00Z",
      "isActive": true,
      "createdAt": "2025-06-15T10:00:00Z"
    },
    {
      "id": 2,
      "userId": "user_abc123",
      "resourceId": 5,
      "resType": "folder",
      "shareCode": "e5f6g7h8",
      "extractCode": "",
      "maxVisit": -1,
      "visitCount": 5,
      "expireAt": "2025-06-18T08:00:00Z",
      "isActive": true,
      "createdAt": "2025-06-15T12:00:00Z"
    }
  ]
}
```

### curl 示例

```bash
curl -X GET http://localhost:9100/v1/disk/shares \
  -H "Authorization: Bearer eyJhbGciOiJIUzI1NiIs..."
```

### 错误场景

| HTTP 状态码 | 场景说明 |
|------------|---------|
| 401 | Token 无效或已过期 |
| 500 | 查询失败 |

---

## 撤销分享

手动撤销一个分享链接，撤销后该分享立即失效。

```
DELETE /v1/disk/shares
```

### 认证方式

需要 JWT Bearer Token 或 API Key 认证。

### 请求体

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `shareId` | `uint64` | 是 | 要撤销的分享 ID |

```json
{
  "shareId": 1
}
```

### 响应示例

```json
{
  "code": 0,
  "message": "success",
  "data": null
}
```

### curl 示例

```bash
curl -X DELETE http://localhost:9100/v1/disk/shares \
  -H "Authorization: Bearer eyJhbGciOiJIUzI1NiIs..." \
  -H "Content-Type: application/json" \
  -d '{"shareId": 1}'
```

### 错误场景

| HTTP 状态码 | 场景说明 |
|------------|---------|
| 400 | `shareId` 缺失或无效 |
| 401 | Token 无效或已过期 |
| 500 | 撤销失败 |

---

## 获取分享信息（公开）

根据分享码获取分享的基本信息。此接口为**公开接口**，无需认证。

```
GET /v1/disk/share/:code
```

### 认证方式

无需认证。

### 路径参数

| 参数 | 类型 | 说明 |
|------|------|------|
| `code` | `string` | 分享码 |

### 响应示例

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "id": 1,
    "userId": "user_abc123",
    "resourceId": 10,
    "resType": "file",
    "shareCode": "a1b2c3d4",
    "extractCode": "abc123",
    "maxVisit": 100,
    "visitCount": 23,
    "expireAt": "2025-06-17T10:00:00Z",
    "isActive": true,
    "createdAt": "2025-06-15T10:00:00Z"
  }
}
```

### curl 示例

```bash
curl -X GET http://localhost:9100/v1/disk/share/a1b2c3d4
```

### 错误场景

| HTTP 状态码 | 场景说明 |
|------------|---------|
| 404 | 分享码不存在或已失效 |

---

## 访问分享（公开）

通过分享码和提取码访问分享内容。系统会记录访问日志（IP、User-Agent 等）。此接口为**公开接口**，无需认证。

```
POST /v1/disk/share/access
```

### 认证方式

无需认证。

### 请求体

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `code` | `string` | 是 | 分享码 |
| `extractCode` | `string` | 否 | 提取码（如果分享设置了提取码则必填） |

```json
{
  "code": "a1b2c3d4",
  "extractCode": "abc123"
}
```

### 响应示例

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "id": 1,
    "userId": "user_abc123",
    "resourceId": 10,
    "resType": "file",
    "shareCode": "a1b2c3d4",
    "extractCode": "",
    "maxVisit": 100,
    "visitCount": 24,
    "expireAt": "2025-06-17T10:00:00Z",
    "isActive": true,
    "createdAt": "2025-06-15T10:00:00Z"
  }
}
```

::: tip
访问成功后，`visitCount` 会自动递增。当 `visitCount` 达到 `maxVisit` 时（`maxVisit != -1`），该分享将自动失效。
:::

### curl 示例

```bash
curl -X POST http://localhost:9100/v1/disk/share/access \
  -H "Content-Type: application/json" \
  -d '{"code": "a1b2c3d4", "extractCode": "abc123"}'
```

### 错误场景

| HTTP 状态码 | 场景说明 |
|------------|---------|
| 400 | 分享码缺失 |
| 403 | 提取码错误、分享已过期或已达最大访问次数 |

---

## 分享文件下载（公开）

为分享的文件生成下载令牌。此接口为**公开接口**，无需认证。

```
POST /v1/disk/share/download
```

### 认证方式

无需认证。

### 请求体

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `code` | `string` | 是 | 分享码 |
| `extractCode` | `string` | 否 | 提取码（如果分享设置了提取码则必填） |
| `resourceId` | `uint64` | 是 | 资源 ID（文件 ID） |

```json
{
  "code": "a1b2c3d4",
  "extractCode": "abc123",
  "resourceId": 10
}
```

### 响应示例

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "downloadToken": "dl_eyJhbGciOiJIUzI1NiIs...",
    "expiresIn": 300
  }
}
```

获取下载令牌后，使用[文件下载接口](./files.md#通过令牌下载文件)完成下载。

### curl 示例

```bash
# 第一步：获取下载令牌
curl -X POST http://localhost:9100/v1/disk/share/download \
  -H "Content-Type: application/json" \
  -d '{"code": "a1b2c3d4", "extractCode": "abc123", "resourceId": 10}'

# 第二步：使用令牌下载文件
curl -L -o report.pdf "http://localhost:9100/v1/disk/files/download?t=dl_eyJhbGciOiJIUzI1NiIs..."
```

### 错误场景

| HTTP 状态码 | 场景说明 |
|------------|---------|
| 400 | 分享码或 `resourceId` 缺失 |
| 403 | 提取码错误或 `resourceId` 与分享不匹配 |
| 404 | 分享不存在 |
| 500 | 令牌生成失败 |
