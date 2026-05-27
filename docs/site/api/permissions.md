# 权限管理接口

提供智能体权限的授予、校验、撤销和列表查询功能。支持按智能体 ID、智能体组 ID、资源 ID 或资源路径（Glob 通配符）进行授权，权限类型包括 `read`、`write`、`delete`。

## 授予权限

为智能体或智能体组授予资源访问权限。

```
POST /v1/disk/permissions
```

### 认证方式

需要 JWT Bearer Token 或 API Key 认证。

### 请求体

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `agentId` | `string` | 否* | 智能体 ID（与 `agentGroupId` 至少填一个） |
| `agentGroupId` | `string` | 否* | 智能体组 ID（与 `agentId` 至少填一个） |
| `resourceId` | `uint64` | 否* | 资源 ID（与 `resourcePath` 至少填一个） |
| `resType` | `string` | 否* | 资源类型：`file` 或 `folder`（使用 `resourceId` 时必填） |
| `resourcePath` | `string` | 否* | 资源路径，支持 Glob 通配符，必须以 `/` 开头（与 `resourceId` 至少填一个） |
| `permission` | `string` | 是 | 权限类型：`read`、`write`、`delete` |

::: tip
`agentId` 和 `agentGroupId` 可以同时填写。`resourceId` 和 `resourcePath` 也可以同时指定。当使用 `resourceId` 时，必须同时指定 `resType`。
:::

```json
{
  "agentId": "agent_writer",
  "resourceId": 10,
  "resType": "file",
  "permission": "read"
}
```

使用资源路径授权的示例：

```json
{
  "agentId": "agent_writer",
  "resourcePath": "/项目文档/**",
  "permission": "write"
}
```

为智能体组授权的示例：

```json
{
  "agentGroupId": "group_content",
  "resourceId": 5,
  "resType": "folder",
  "permission": "read"
}
```

### 响应示例

**HTTP 201 Created**

```json
{
  "code": 0,
  "message": "created",
  "data": null
}
```

### curl 示例

```bash
# 为智能体授予文件读取权限
curl -X POST http://localhost:9100/v1/disk/permissions \
  -H "Authorization: Bearer eyJhbGciOiJIUzI1NiIs..." \
  -H "Content-Type: application/json" \
  -d '{"agentId": "agent_writer", "resourceId": 10, "resType": "file", "permission": "read"}'

# 使用路径通配符授权
curl -X POST http://localhost:9100/v1/disk/permissions \
  -H "Authorization: Bearer eyJhbGciOiJIUzI1NiIs..." \
  -H "Content-Type: application/json" \
  -d '{"agentId": "agent_writer", "resourcePath": "/项目文档/**", "permission": "write"}'

# 为智能体组授权
curl -X POST http://localhost:9100/v1/disk/permissions \
  -H "Authorization: Bearer eyJhbGciOiJIUzI1NiIs..." \
  -H "Content-Type: application/json" \
  -d '{"agentGroupId": "group_content", "resourceId": 5, "resType": "folder", "permission": "read"}'
```

### 错误场景

| HTTP 状态码 | 场景说明 |
|------------|---------|
| 400 | 必填参数缺失：未提供 `agentId` 或 `agentGroupId`；未提供 `resourceId` 或 `resourcePath`；使用 `resourceId` 时未提供 `resType`；`resourcePath` 未以 `/` 开头 |
| 401 | Token 无效或已过期 |
| 500 | 授权失败 |

---

## 校验权限

检查智能体或智能体组是否拥有指定资源的特定权限。

```
GET /v1/disk/permissions/check
```

### 认证方式

需要 JWT Bearer Token 或 API Key 认证。

### 查询参数

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `agentId` | `string` | 否 | 智能体 ID（与 `agentGroupId` 至少填一个） |
| `agentGroupId` | `string` | 否 | 智能体组 ID（与 `agentId` 至少填一个） |
| `resourceId` | `uint64` | 是 | 资源 ID |
| `resType` | `string` | 否 | 资源类型：`file` 或 `folder` |
| `permission` | `string` | 是 | 待校验的权限：`read`、`write`、`delete` |

### 响应示例

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "allowed": true
  }
}
```

### curl 示例

```bash
# 校验智能体权限
curl -X GET "http://localhost:9100/v1/disk/permissions/check?agentId=agent_writer&resourceId=10&resType=file&permission=read" \
  -H "Authorization: Bearer eyJhbGciOiJIUzI1NiIs..."

# 校验智能体组权限
curl -X GET "http://localhost:9100/v1/disk/permissions/check?agentGroupId=group_content&resourceId=5&resType=folder&permission=write" \
  -H "Authorization: Bearer eyJhbGciOiJIUzI1NiIs..."
```

::: tip
校验逻辑为先检查智能体个人权限，若不通过则继续检查其所属智能体组的权限。两者任一通过即返回 `allowed: true`。
:::

### 错误场景

| HTTP 状态码 | 场景说明 |
|------------|---------|
| 400 | `resourceId` 无效 |
| 401 | Token 无效或已过期 |

---

## 撤销权限

撤销智能体或智能体组对指定资源的访问权限。

```
DELETE /v1/disk/permissions
```

### 认证方式

需要 JWT Bearer Token 或 API Key 认证。

### 请求体

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `agentId` | `string` | 否* | 智能体 ID（与 `agentGroupId` 至少填一个） |
| `agentGroupId` | `string` | 否* | 智能体组 ID（与 `agentId` 至少填一个） |
| `resourceId` | `uint64` | 否* | 资源 ID（与 `resourcePath` 至少填一个） |
| `resType` | `string` | 否 | 资源类型 |
| `resourcePath` | `string` | 否* | 资源路径（与 `resourceId` 至少填一个） |

```json
{
  "agentId": "agent_writer",
  "resourceId": 10,
  "resType": "file"
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
curl -X DELETE http://localhost:9100/v1/disk/permissions \
  -H "Authorization: Bearer eyJhbGciOiJIUzI1NiIs..." \
  -H "Content-Type: application/json" \
  -d '{"agentId": "agent_writer", "resourceId": 10, "resType": "file"}'
```

### 错误场景

| HTTP 状态码 | 场景说明 |
|------------|---------|
| 400 | 必填参数缺失 |
| 401 | Token 无效或已过期 |
| 500 | 撤销失败 |

---

## 列出权限

获取当前用户授予的所有权限列表。

```
GET /v1/disk/permissions
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
      "agentId": "agent_writer",
      "agentGroupId": "",
      "resourceId": 10,
      "resType": "file",
      "resourcePath": "",
      "permission": "read",
      "createdAt": "2025-06-15T10:00:00Z",
      "updatedAt": "2025-06-15T10:00:00Z"
    },
    {
      "id": 2,
      "userId": "user_abc123",
      "agentId": "",
      "agentGroupId": "group_content",
      "resourceId": 0,
      "resType": "",
      "resourcePath": "/项目文档/**",
      "permission": "write",
      "createdAt": "2025-06-15T11:00:00Z",
      "updatedAt": "2025-06-15T11:00:00Z"
    }
  ]
}
```

### curl 示例

```bash
curl -X GET http://localhost:9100/v1/disk/permissions \
  -H "Authorization: Bearer eyJhbGciOiJIUzI1NiIs..."
```

### 错误场景

| HTTP 状态码 | 场景说明 |
|------------|---------|
| 401 | Token 无效或已过期 |
| 500 | 查询失败 |
