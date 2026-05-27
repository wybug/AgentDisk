# 版本管理接口

提供文件版本历史查询和版本回溯功能。每次通过更新接口上传文件新版本时，系统会自动创建版本快照，保留历史版本的 OSS 存储路径和元数据。

## 查询文件版本列表

获取指定文件的所有历史版本。

```
GET /v1/disk/versions?fileId=X
```

### 认证方式

需要 JWT Bearer Token 或 API Key 认证。

### 查询参数

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `fileId` | `uint64` | 是 | 文件 ID |

### 响应示例

```json
{
  "code": 0,
  "message": "success",
  "data": [
    {
      "id": 1,
      "fileId": 10,
      "userId": "user_abc123",
      "version": 1,
      "ossKey": "user_abc123/1/report_v1_1718452800.pdf",
      "fileSize": 2048576,
      "md5": "d41d8cd98f00b204e9800998ecf8427e",
      "snapshotBy": "user",
      "createdAt": "2025-06-15T10:00:00Z"
    },
    {
      "id": 2,
      "fileId": 10,
      "userId": "user_abc123",
      "version": 2,
      "ossKey": "user_abc123/1/report_v2_1718453600.pdf",
      "fileSize": 2199999,
      "md5": "a1b2c3d4e5f6a7b8c9d0e1f2a3b4c5d6",
      "snapshotBy": "agent_writer",
      "createdAt": "2025-06-15T14:00:00Z"
    },
    {
      "id": 3,
      "fileId": 10,
      "userId": "user_abc123",
      "version": 3,
      "ossKey": "user_abc123/1/report_v3_1718460000.pdf",
      "fileSize": 2300000,
      "md5": "f1e2d3c4b5a6978869504132a1b2c3d4",
      "snapshotBy": "user",
      "createdAt": "2025-06-15T16:00:00Z"
    }
  ]
}
```

### 响应字段说明

| 字段 | 类型 | 说明 |
|------|------|------|
| `id` | `uint64` | 版本记录 ID |
| `fileId` | `uint64` | 所属文件 ID |
| `version` | `int` | 版本号，从 `1` 开始递增 |
| `ossKey` | `string` | 该版本文件在 OSS 中的存储路径 |
| `fileSize` | `int64` | 该版本文件大小（字节） |
| `md5` | `string` | 该版本文件的 MD5 哈希 |
| `snapshotBy` | `string` | 创建该版本的操作者（用户 ID 或智能体 ID） |
| `createdAt` | `string` | 版本创建时间（RFC 3339） |

### curl 示例

```bash
curl -X GET "http://localhost:9100/v1/disk/versions?fileId=10" \
  -H "Authorization: Bearer eyJhbGciOiJIUzI1NiIs..."
```

### 错误场景

| HTTP 状态码 | 场景说明 |
|------------|---------|
| 400 | `fileId` 参数缺失或无效 |
| 401 | Token 无效或已过期 |
| 500 | 查询失败 |

---

## 回溯到指定版本

将文件回溯到指定的历史版本。系统会将文件当前的 OSS 路径替换为目标版本的路径，并自动创建新的版本快照。

```
POST /v1/disk/versions/rollback
```

### 认证方式

需要 JWT Bearer Token 或 API Key 认证。

### 请求体

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `fileId` | `uint64` | 是 | 文件 ID |
| `version` | `int` | 是 | 目标版本号 |

```json
{
  "fileId": 10,
  "version": 2
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

::: warning 注意
回溯操作不会删除历史版本记录。回溯后文件内容恢复为目标版本的状态，版本号不会回退，而是基于当前最大版本号继续递增。
:::

### curl 示例

```bash
curl -X POST http://localhost:9100/v1/disk/versions/rollback \
  -H "Authorization: Bearer eyJhbGciOiJIUzI1NiIs..." \
  -H "Content-Type: application/json" \
  -d '{"fileId": 10, "version": 2}'
```

### 错误场景

| HTTP 状态码 | 场景说明 |
|------------|---------|
| 400 | `fileId` 或 `version` 缺失 |
| 401 | Token 无效或已过期 |
| 500 | 回溯失败（如目标版本不存在） |
