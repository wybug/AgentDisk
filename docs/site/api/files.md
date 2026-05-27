# 文件管理接口

提供文件上传、下载、查询、更新、删除等全生命周期管理能力。文件实体存储在 OSS（对象存储服务）中，接口返回元数据及预签名 URL。

## 上传文件

上传文件到指定目录。支持通过 `multipart/form-data` 方式上传。

```
POST /v1/disk/files/upload
```

### 认证方式

需要 JWT Bearer Token 或 API Key 认证。

### 请求体（multipart/form-data）

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `file` | `file` | 是 | 要上传的文件 |
| `folderId` | `uint64` | 否 | 目标目录 ID，`0` 表示根目录，默认 `0` |
| `agentId` | `string` | 否 | 智能体 ID（若通过表单传递，优先使用 JWT 中的值） |

::: tip
如果 JWT 中包含 `agentId` 声明，系统将自动使用 JWT 中的值，无需在表单中重复传递。如果 JWT 中也包含 `agentGroupId`，将自动关联到文件的 `sourceAgentGroup` 字段。
:::

### 响应示例

**HTTP 201 Created**

```json
{
  "code": 0,
  "message": "created",
  "data": {
    "id": 10,
    "userId": "user_abc123",
    "folderId": 1,
    "fileName": "report.pdf",
    "fileSize": 2048576,
    "fileType": "pdf",
    "ossKey": "user_abc123/1/report_v1_1718452800.pdf",
    "md5": "d41d8cd98f00b204e9800998ecf8427e",
    "version": 1,
    "isDeleted": false,
    "sourceAgent": "agent_writer",
    "sourceAgentGroup": "group_content",
    "isArtifact": true,
    "tags": "",
    "createdAt": "2025-06-15T10:00:00Z",
    "updatedAt": "2025-06-15T10:00:00Z"
  }
}
```

### 响应字段说明

| 字段 | 类型 | 说明 |
|------|------|------|
| `id` | `uint64` | 文件 ID |
| `userId` | `string` | 所属用户 ID |
| `folderId` | `uint64` | 所在目录 ID |
| `fileName` | `string` | 文件名 |
| `fileSize` | `int64` | 文件大小（字节） |
| `fileType` | `string` | 文件扩展名（不含点号） |
| `ossKey` | `string` | OSS 存储路径 |
| `md5` | `string` | 文件 MD5 哈希 |
| `version` | `int` | 当前版本号，首次上传为 `1` |
| `isDeleted` | `bool` | 是否已删除 |
| `sourceAgent` | `string` | 来源智能体 ID |
| `sourceAgentGroup` | `string` | 来源智能体组 ID |
| `isArtifact` | `bool` | 是否为智能体产物 |
| `tags` | `string` | 标签列表 |

### curl 示例

```bash
# 上传文件到根目录
curl -X POST http://localhost:9100/v1/disk/files/upload \
  -H "Authorization: Bearer eyJhbGciOiJIUzI1NiIs..." \
  -F "file=@/path/to/report.pdf" \
  -F "folderId=0"

# 上传文件到指定目录并关联智能体
curl -X POST http://localhost:9100/v1/disk/files/upload \
  -H "Authorization: Bearer eyJhbGciOiJIUzI1NiIs..." \
  -F "file=@/path/to/data.csv" \
  -F "folderId=5" \
  -F "agentId=agent_analyzer"
```

### 错误场景

| HTTP 状态码 | 场景说明 |
|------------|---------|
| 400 | 缺少 `file` 字段 |
| 401 | Token 无效或已过期 |
| 500 | 上传失败（存储服务异常等） |

---

## 获取文件信息

根据文件 ID 获取文件元数据及预签名访问 URL。如果请求者通过智能体访问，会自动进行权限校验。

```
GET /v1/disk/files/:id
```

### 认证方式

需要 JWT Bearer Token 或 API Key 认证。

### 路径参数

| 参数 | 类型 | 说明 |
|------|------|------|
| `id` | `uint64` | 文件 ID |

### 响应示例

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "file": {
      "id": 10,
      "userId": "user_abc123",
      "folderId": 1,
      "fileName": "report.pdf",
      "fileSize": 2048576,
      "fileType": "pdf",
      "ossKey": "user_abc123/1/report_v1_1718452800.pdf",
      "md5": "d41d8cd98f00b204e9800998ecf8427e",
      "version": 1,
      "isDeleted": false,
      "sourceAgent": "",
      "sourceAgentGroup": "",
      "isArtifact": false,
      "tags": "",
      "createdAt": "2025-06-15T10:00:00Z",
      "updatedAt": "2025-06-15T10:00:00Z"
    },
    "url": "https://oss.example.com/bucket/user_abc123/1/report_v1_1718452800.pdf?X-Amz-Algorithm=...&X-Amz-Expires=3600"
  }
}
```

::: tip
返回的 `url` 是 OSS 预签名 URL，具有时效性（通常 1 小时），过期后需要重新获取。该 URL 可直接用于文件访问或下载。
:::

### curl 示例

```bash
curl -X GET http://localhost:9100/v1/disk/files/10 \
  -H "Authorization: Bearer eyJhbGciOiJIUzI1NiIs..."
```

### 错误场景

| HTTP 状态码 | 场景说明 |
|------------|---------|
| 400 | `id` 格式无效 |
| 403 | 智能体无该文件的读取权限 |
| 500 | 文件查询失败 |

---

## 列出目录下的文件

获取指定目录下的所有文件列表。

```
GET /v1/disk/files?folderId=X
```

### 认证方式

需要 JWT Bearer Token 或 API Key 认证。

### 查询参数

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `folderId` | `uint64` | 是 | 目录 ID |

### 响应示例

```json
{
  "code": 0,
  "message": "success",
  "data": [
    {
      "id": 10,
      "userId": "user_abc123",
      "folderId": 1,
      "fileName": "report.pdf",
      "fileSize": 2048576,
      "fileType": "pdf",
      "ossKey": "user_abc123/1/report_v1_1718452800.pdf",
      "md5": "d41d8cd98f00b204e9800998ecf8427e",
      "version": 1,
      "isDeleted": false,
      "sourceAgent": "",
      "sourceAgentGroup": "",
      "isArtifact": false,
      "tags": "报告,季度",
      "createdAt": "2025-06-15T10:00:00Z",
      "updatedAt": "2025-06-15T10:00:00Z"
    },
    {
      "id": 11,
      "userId": "user_abc123",
      "folderId": 1,
      "fileName": "data.csv",
      "fileSize": 512000,
      "fileType": "csv",
      "ossKey": "user_abc123/1/data_v1_1718452900.csv",
      "md5": "098f6bcd4621d373cade4e832627b4f6",
      "version": 1,
      "isDeleted": false,
      "sourceAgent": "agent_analyzer",
      "sourceAgentGroup": "group_data",
      "isArtifact": true,
      "tags": "",
      "createdAt": "2025-06-15T10:10:00Z",
      "updatedAt": "2025-06-15T10:10:00Z"
    }
  ]
}
```

### curl 示例

```bash
curl -X GET "http://localhost:9100/v1/disk/files?folderId=1" \
  -H "Authorization: Bearer eyJhbGciOiJIUzI1NiIs..."
```

### 错误场景

| HTTP 状态码 | 场景说明 |
|------------|---------|
| 400 | `folderId` 参数缺失或无效 |
| 401 | Token 无效或已过期 |

---

## 更新文件

上传文件的新版本。系统会自动创建版本快照，版本号自增。

```
PUT /v1/disk/files/:id
```

### 认证方式

需要 JWT Bearer Token 或 API Key 认证。智能体访问时需具备 `write` 权限。

### 路径参数

| 参数 | 类型 | 说明 |
|------|------|------|
| `id` | `uint64` | 文件 ID |

### 请求体（multipart/form-data）

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `file` | `file` | 是 | 新版本的文件内容 |

### 响应示例

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "id": 10,
    "userId": "user_abc123",
    "folderId": 1,
    "fileName": "report.pdf",
    "fileSize": 2199999,
    "fileType": "pdf",
    "ossKey": "user_abc123/1/report_v2_1718453600.pdf",
    "md5": "a1b2c3d4e5f6a7b8c9d0e1f2a3b4c5d6",
    "version": 2,
    "isDeleted": false,
    "sourceAgent": "",
    "sourceAgentGroup": "",
    "isArtifact": false,
    "tags": "",
    "createdAt": "2025-06-15T10:00:00Z",
    "updatedAt": "2025-06-15T14:00:00Z"
  }
}
```

### curl 示例

```bash
curl -X PUT http://localhost:9100/v1/disk/files/10 \
  -H "Authorization: Bearer eyJhbGciOiJIUzI1NiIs..." \
  -F "file=@/path/to/report_v2.pdf"
```

### 错误场景

| HTTP 状态码 | 场景说明 |
|------------|---------|
| 400 | `id` 无效或缺少 `file` 字段 |
| 403 | 智能体无该文件的写入权限 |
| 500 | 更新失败 |

---

## 删除文件

删除指定文件。该操作为**软删除**，文件会被移入回收站，可从回收站恢复。

```
DELETE /v1/disk/files/:id
```

### 认证方式

需要 JWT Bearer Token 或 API Key 认证。智能体访问时需具备 `delete` 权限。

### 路径参数

| 参数 | 类型 | 说明 |
|------|------|------|
| `id` | `uint64` | 文件 ID |

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
curl -X DELETE http://localhost:9100/v1/disk/files/10 \
  -H "Authorization: Bearer eyJhbGciOiJIUzI1NiIs..."
```

::: warning 注意
删除文件后，文件标记为已删除并自动移入回收站。OSS 中的文件内容不会立即删除，通过回收站永久删除时才会清除。
:::

### 错误场景

| HTTP 状态码 | 场景说明 |
|------------|---------|
| 400 | `id` 格式无效 |
| 403 | 智能体无该文件的删除权限 |
| 500 | 删除操作失败 |

---

## 生成下载令牌

为指定文件生成一个临时下载令牌。该令牌可用于公开下载接口，无需 Authorization 头。

```
POST /v1/disk/files/:id/download-token
```

### 认证方式

需要 JWT Bearer Token 或 API Key 认证。

### 路径参数

| 参数 | 类型 | 说明 |
|------|------|------|
| `id` | `uint64` | 文件 ID |

### 请求体

无。

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

### 响应字段说明

| 字段 | 类型 | 说明 |
|------|------|------|
| `downloadToken` | `string` | 下载令牌 |
| `expiresIn` | `int` | 令牌有效时长（秒），默认 300 秒（5 分钟） |

### curl 示例

```bash
curl -X POST http://localhost:9100/v1/disk/files/10/download-token \
  -H "Authorization: Bearer eyJhbGciOiJIUzI1NiIs..."
```

### 错误场景

| HTTP 状态码 | 场景说明 |
|------------|---------|
| 400 | `id` 格式无效 |
| 403 | 文件不存在或无权限 |
| 500 | 令牌生成失败 |

---

## 通过令牌下载文件

使用下载令牌获取文件的下载信息。此接口为**公开接口**，无需认证头。

```
GET /v1/disk/files/download?t=<download-token>
```

### 认证方式

无需认证。通过下载令牌（`t` 参数）进行鉴权。

### 查询参数

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `t` | `string` | 是 | 下载令牌（由生成下载令牌接口获取） |

### 响应示例

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "file": {
      "id": 10,
      "userId": "user_abc123",
      "folderId": 1,
      "fileName": "report.pdf",
      "fileSize": 2048576,
      "fileType": "pdf",
      "ossKey": "user_abc123/1/report_v1_1718452800.pdf",
      "md5": "d41d8cd98f00b204e9800998ecf8427e",
      "version": 1,
      "isDeleted": false,
      "sourceAgent": "",
      "sourceAgentGroup": "",
      "isArtifact": false,
      "tags": "",
      "createdAt": "2025-06-15T10:00:00Z",
      "updatedAt": "2025-06-15T10:00:00Z"
    },
    "downloadUrl": "https://oss.example.com/bucket/user_abc123/1/report_v1_1718452800.pdf?X-Amz-Algorithm=...&X-Amz-Expires=3600"
  }
}
```

### curl 示例

```bash
# 生成令牌后，使用令牌下载
curl -X GET "http://localhost:9100/v1/disk/files/download?t=dl_eyJhbGciOiJIUzI1NiIs..."

# 直接下载文件内容
curl -L -o report.pdf "http://localhost:9100/v1/disk/files/download?t=dl_eyJhbGciOiJIUzI1NiIs..."
```

### 错误场景

| HTTP 状态码 | 场景说明 |
|------------|---------|
| 400 | 令牌缺失或文件 ID 无效 |
| 401 | 令牌无效或已过期 |
| 403 | 文件不存在或已被删除 |
