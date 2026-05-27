# 目录管理接口

提供目录（文件夹）的创建、查询、重命名、删除等操作。所有目录操作均按用户隔离。

## 创建目录

在指定父目录下创建新目录。

```
POST /v1/disk/folders
```

### 认证方式

需要 JWT Bearer Token 或 API Key 认证。

### 请求体

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `folderName` | `string` | 是 | 目录名称，最长 255 字符 |
| `parentId` | `uint64` | 否 | 父目录 ID，`0` 表示根目录，默认 `0` |

```json
{
  "folderName": "项目文档",
  "parentId": 0
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
    "parentId": 0,
    "folderName": "项目文档",
    "fullPath": "/项目文档",
    "sortOrder": 0,
    "isDeleted": false,
    "createdAt": "2025-06-15T10:00:00Z",
    "updatedAt": "2025-06-15T10:00:00Z"
  }
}
```

### curl 示例

```bash
curl -X POST http://localhost:9100/v1/disk/folders \
  -H "Authorization: Bearer eyJhbGciOiJIUzI1NiIs..." \
  -H "Content-Type: application/json" \
  -d '{"folderName": "项目文档", "parentId": 0}'
```

### 错误场景

| HTTP 状态码 | 场景说明 |
|------------|---------|
| 400 | `folderName` 为空 |
| 401 | Token 无效或已过期 |
| 500 | 目录创建失败（如重名等） |

---

## 获取目录信息

根据目录 ID 获取单个目录的详细信息。

```
GET /v1/disk/folders/:id
```

### 认证方式

需要 JWT Bearer Token 或 API Key 认证。

### 路径参数

| 参数 | 类型 | 说明 |
|------|------|------|
| `id` | `uint64` | 目录 ID |

### 响应示例

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "id": 1,
    "userId": "user_abc123",
    "parentId": 0,
    "folderName": "项目文档",
    "fullPath": "/项目文档",
    "sortOrder": 0,
    "isDeleted": false,
    "createdAt": "2025-06-15T10:00:00Z",
    "updatedAt": "2025-06-15T10:00:00Z"
  }
}
```

### curl 示例

```bash
curl -X GET http://localhost:9100/v1/disk/folders/1 \
  -H "Authorization: Bearer eyJhbGciOiJIUzI1NiIs..."
```

### 错误场景

| HTTP 状态码 | 场景说明 |
|------------|---------|
| 400 | `id` 格式无效 |
| 404 | 目录不存在或不属于当前用户 |

---

## 列出子目录

列出指定父目录下的所有直接子目录。

```
GET /v1/disk/folders?parentId=X
```

### 认证方式

需要 JWT Bearer Token 或 API Key 认证。

### 查询参数

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `parentId` | `uint64` | 否 | 父目录 ID，`0` 表示根目录，默认 `0` |

### 响应示例

```json
{
  "code": 0,
  "message": "success",
  "data": [
    {
      "id": 1,
      "userId": "user_abc123",
      "parentId": 0,
      "folderName": "项目文档",
      "fullPath": "/项目文档",
      "sortOrder": 0,
      "isDeleted": false,
      "createdAt": "2025-06-15T10:00:00Z",
      "updatedAt": "2025-06-15T10:00:00Z"
    },
    {
      "id": 2,
      "userId": "user_abc123",
      "parentId": 0,
      "folderName": "智能体产物",
      "fullPath": "/智能体产物",
      "sortOrder": 0,
      "isDeleted": false,
      "createdAt": "2025-06-15T10:05:00Z",
      "updatedAt": "2025-06-15T10:05:00Z"
    }
  ]
}
```

### curl 示例

```bash
# 列出根目录下的子目录
curl -X GET "http://localhost:9100/v1/disk/folders?parentId=0" \
  -H "Authorization: Bearer eyJhbGciOiJIUzI1NiIs..."

# 列出指定目录下的子目录
curl -X GET "http://localhost:9100/v1/disk/folders?parentId=1" \
  -H "Authorization: Bearer eyJhbGciOiJIUzI1NiIs..."
```

### 错误场景

| HTTP 状态码 | 场景说明 |
|------------|---------|
| 401 | Token 无效或已过期 |

---

## 获取目录祖先路径

获取从根目录到指定目录的完整祖先路径链，用于面包屑导航。

```
GET /v1/disk/folders/:id/ancestors
```

### 认证方式

需要 JWT Bearer Token 或 API Key 认证。

### 路径参数

| 参数 | 类型 | 说明 |
|------|------|------|
| `id` | `uint64` | 目录 ID |

### 响应示例

```json
{
  "code": 0,
  "message": "success",
  "data": [
    {
      "id": 1,
      "userId": "user_abc123",
      "parentId": 0,
      "folderName": "项目文档",
      "fullPath": "/项目文档",
      "sortOrder": 0,
      "isDeleted": false,
      "createdAt": "2025-06-15T10:00:00Z",
      "updatedAt": "2025-06-15T10:00:00Z"
    },
    {
      "id": 3,
      "userId": "user_abc123",
      "parentId": 1,
      "folderName": "技术方案",
      "fullPath": "/项目文档/技术方案",
      "sortOrder": 0,
      "isDeleted": false,
      "createdAt": "2025-06-15T10:10:00Z",
      "updatedAt": "2025-06-15T10:10:00Z"
    }
  ]
}
```

路径数组按层级从浅到深排列，第一个元素为根级目录，最后一个元素为当前目录的直接父级。

### curl 示例

```bash
curl -X GET http://localhost:9100/v1/disk/folders/5/ancestors \
  -H "Authorization: Bearer eyJhbGciOiJIUzI1NiIs..."
```

### 错误场景

| HTTP 状态码 | 场景说明 |
|------------|---------|
| 400 | `id` 格式无效 |
| 404 | 目录不存在或不属于当前用户 |

---

## 重命名目录

修改指定目录的名称。

```
PUT /v1/disk/folders/:id
```

### 认证方式

需要 JWT Bearer Token 或 API Key 认证。

### 路径参数

| 参数 | 类型 | 说明 |
|------|------|------|
| `id` | `uint64` | 目录 ID |

### 请求体

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `folderName` | `string` | 是 | 新的目录名称 |

```json
{
  "folderName": "技术方案_v2"
}
```

### 响应示例

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "id": 3,
    "userId": "user_abc123",
    "parentId": 1,
    "folderName": "技术方案_v2",
    "fullPath": "/项目文档/技术方案_v2",
    "sortOrder": 0,
    "isDeleted": false,
    "createdAt": "2025-06-15T10:10:00Z",
    "updatedAt": "2025-06-15T14:00:00Z"
  }
}
```

### curl 示例

```bash
curl -X PUT http://localhost:9100/v1/disk/folders/3 \
  -H "Authorization: Bearer eyJhbGciOiJIUzI1NiIs..." \
  -H "Content-Type: application/json" \
  -d '{"folderName": "技术方案_v2"}'
```

### 错误场景

| HTTP 状态码 | 场景说明 |
|------------|---------|
| 400 | `id` 无效或 `folderName` 为空 |
| 401 | Token 无效或已过期 |

---

## 删除目录

删除指定目录。该操作为**软删除**，删除后的目录会被移入回收站，可从回收站恢复。

```
DELETE /v1/disk/folders/:id
```

### 认证方式

需要 JWT Bearer Token 或 API Key 认证。

### 路径参数

| 参数 | 类型 | 说明 |
|------|------|------|
| `id` | `uint64` | 目录 ID |

### 请求体

无。

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
curl -X DELETE http://localhost:9100/v1/disk/folders/3 \
  -H "Authorization: Bearer eyJhbGciOiJIUzI1NiIs..."
```

::: warning 注意
删除目录后，该目录及其中所有文件将标记为已删除状态，并自动移入回收站。如需彻底删除，请在回收站中执行永久删除操作。
:::

### 错误场景

| HTTP 状态码 | 场景说明 |
|------------|---------|
| 400 | `id` 格式无效 |
| 401 | Token 无效或已过期 |
| 500 | 删除操作失败 |
