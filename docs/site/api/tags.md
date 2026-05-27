# 标签管理接口

提供文件标签的绑定、解绑和按标签检索功能。标签可以帮助用户对文件进行分类和组织。

## 绑定标签

为指定文件绑定一个标签。同一文件可绑定多个标签。

```
POST /v1/disk/tags/bind
```

### 认证方式

需要 JWT Bearer Token 或 API Key 认证。

### 请求体

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `fileId` | `uint64` | 是 | 文件 ID |
| `tagName` | `string` | 是 | 标签名称，最长 64 字符 |

```json
{
  "fileId": 10,
  "tagName": "季度报告"
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
curl -X POST http://localhost:9100/v1/disk/tags/bind \
  -H "Authorization: Bearer eyJhbGciOiJIUzI1NiIs..." \
  -H "Content-Type: application/json" \
  -d '{"fileId": 10, "tagName": "季度报告"}'
```

### 错误场景

| HTTP 状态码 | 场景说明 |
|------------|---------|
| 400 | `fileId` 或 `tagName` 缺失 |
| 401 | Token 无效或已过期 |
| 500 | 绑定失败（如标签已存在） |

---

## 解绑标签

移除文件的指定标签。

```
POST /v1/disk/tags/unbind
```

### 认证方式

需要 JWT Bearer Token 或 API Key 认证。

### 请求体

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `fileId` | `uint64` | 是 | 文件 ID |
| `tagName` | `string` | 是 | 要移除的标签名称 |

```json
{
  "fileId": 10,
  "tagName": "季度报告"
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
curl -X POST http://localhost:9100/v1/disk/tags/unbind \
  -H "Authorization: Bearer eyJhbGciOiJIUzI1NiIs..." \
  -H "Content-Type: application/json" \
  -d '{"fileId": 10, "tagName": "季度报告"}'
```

### 错误场景

| HTTP 状态码 | 场景说明 |
|------------|---------|
| 400 | `fileId` 或 `tagName` 缺失 |
| 401 | Token 无效或已过期 |
| 500 | 解绑失败 |

---

## 按标签检索

根据一个或多个标签搜索匹配的文件。多个标签之间为"与"关系（即文件必须同时拥有所有指定标签）。

```
GET /v1/disk/tags/search?tags=X,Y
```

### 认证方式

需要 JWT Bearer Token 或 API Key 认证。

### 查询参数

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `tags` | `string` | 是 | 标签列表，多个标签以英文逗号分隔 |

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
      "fileName": "report_q1.pdf",
      "fileSize": 2048576,
      "fileType": "pdf",
      "ossKey": "user_abc123/1/report_q1_v1_1718452800.pdf",
      "md5": "d41d8cd98f00b204e9800998ecf8427e",
      "version": 1,
      "isDeleted": false,
      "sourceAgent": "",
      "sourceAgentGroup": "",
      "isArtifact": false,
      "tags": "季度报告,2025",
      "createdAt": "2025-06-15T10:00:00Z",
      "updatedAt": "2025-06-15T10:00:00Z"
    },
    {
      "id": 15,
      "userId": "user_abc123",
      "folderId": 1,
      "fileName": "report_q2.pdf",
      "fileSize": 1890000,
      "fileType": "pdf",
      "ossKey": "user_abc123/1/report_q2_v1_1718453000.pdf",
      "md5": "a1b2c3d4e5f6a7b8c9d0e1f2a3b4c5d6",
      "version": 1,
      "isDeleted": false,
      "sourceAgent": "",
      "sourceAgentGroup": "",
      "isArtifact": false,
      "tags": "季度报告,2025",
      "createdAt": "2025-06-15T12:00:00Z",
      "updatedAt": "2025-06-15T12:00:00Z"
    }
  ]
}
```

### curl 示例

```bash
# 单标签搜索
curl -X GET "http://localhost:9100/v1/disk/tags/search?tags=季度报告" \
  -H "Authorization: Bearer eyJhbGciOiJIUzI1NiIs..."

# 多标签搜索（与关系）
curl -X GET "http://localhost:9100/v1/disk/tags/search?tags=季度报告,2025" \
  -H "Authorization: Bearer eyJhbGciOiJIUzI1NiIs..."
```

::: tip
多标签搜索采用交集匹配，返回同时拥有所有指定标签的文件。标签名区分大小写。
:::

### 错误场景

| HTTP 状态码 | 场景说明 |
|------------|---------|
| 400 | `tags` 参数缺失 |
| 401 | Token 无效或已过期 |
| 500 | 搜索失败 |
