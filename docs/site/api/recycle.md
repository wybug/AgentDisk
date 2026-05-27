# 回收站接口

回收站用于存放被软删除的文件和目录。支持查看已删除项目、恢复到原位置、以及永久删除。

## 列出回收站项目

获取当前用户回收站中的所有项目。

```
GET /v1/disk/recycle
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
      "resName": "report.pdf",
      "originalPath": "/项目文档/report.pdf",
      "deletedBy": "user",
      "expireAt": "2025-07-15T10:00:00Z",
      "createdAt": "2025-06-15T10:00:00Z"
    },
    {
      "id": 2,
      "userId": "user_abc123",
      "resourceId": 3,
      "resType": "folder",
      "resName": "旧文档",
      "originalPath": "/旧文档",
      "deletedBy": "agent_cleaner",
      "expireAt": "2025-07-15T12:00:00Z",
      "createdAt": "2025-06-15T12:00:00Z"
    }
  ]
}
```

### 响应字段说明

| 字段 | 类型 | 说明 |
|------|------|------|
| `id` | `uint64` | 回收站记录 ID（注意：不是原资源 ID） |
| `resourceId` | `uint64` | 原资源 ID |
| `resType` | `string` | 资源类型：`file` 或 `folder` |
| `resName` | `string` | 原资源名称 |
| `originalPath` | `string` | 删除前的完整路径 |
| `deletedBy` | `string` | 执行删除的操作者（用户 ID 或智能体 ID） |
| `expireAt` | `string` | 过期时间，到期后系统可能自动永久删除（RFC 3339） |
| `createdAt` | `string` | 进入回收站的时间（RFC 3339） |

### curl 示例

```bash
curl -X GET http://localhost:9100/v1/disk/recycle \
  -H "Authorization: Bearer eyJhbGciOiJIUzI1NiIs..."
```

### 错误场景

| HTTP 状态码 | 场景说明 |
|------------|---------|
| 401 | Token 无效或已过期 |
| 500 | 查询失败 |

---

## 恢复项目

将回收站中的项目恢复到原位置。恢复后文件或目录将回到删除前的状态。

```
POST /v1/disk/recycle/restore
```

### 认证方式

需要 JWT Bearer Token 或 API Key 认证。

### 请求体

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `recycleId` | `uint64` | 是 | 回收站记录 ID（`id` 字段，非 `resourceId`） |

```json
{
  "recycleId": 1
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
curl -X POST http://localhost:9100/v1/disk/recycle/restore \
  -H "Authorization: Bearer eyJhbGciOiJIUzI1NiIs..." \
  -H "Content-Type: application/json" \
  -d '{"recycleId": 1}'
```

::: tip
恢复操作会同时将资源的 `isDeleted` 状态恢复为 `false`，并从回收站中移除该记录。
:::

### 错误场景

| HTTP 状态码 | 场景说明 |
|------------|---------|
| 400 | `recycleId` 缺失或无效 |
| 401 | Token 无效或已过期 |
| 500 | 恢复失败（如原父目录已被删除） |

---

## 永久删除

彻底删除回收站中的项目，不可恢复。系统会同时删除 OSS 中的文件内容和数据库中的记录。

```
DELETE /v1/disk/recycle
```

### 认证方式

需要 JWT Bearer Token 或 API Key 认证。

### 请求体

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `recycleId` | `uint64` | 是 | 回收站记录 ID |

```json
{
  "recycleId": 1
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

::: danger 危险操作
永久删除不可恢复！该操作会同时清除数据库记录和 OSS 存储中的文件内容。请确认不再需要该文件后再执行此操作。
:::

### curl 示例

```bash
curl -X DELETE http://localhost:9100/v1/disk/recycle \
  -H "Authorization: Bearer eyJhbGciOiJIUzI1NiIs..." \
  -H "Content-Type: application/json" \
  -d '{"recycleId": 1}'
```

### 错误场景

| HTTP 状态码 | 场景说明 |
|------------|---------|
| 400 | `recycleId` 缺失或无效 |
| 401 | Token 无效或已过期 |
| 500 | 永久删除失败 |
