# 管理接口

管理接口提供管理员认证、用户管理、API Key 管理、公共目录管理和 OAuth2 配置管理功能。所有管理接口需要管理员权限（`AdminAuth` + `AdminOnly` 中间件），登录和初始化引导接口除外。

::: warning 权限要求
除 `POST /v1/disk/admin/login` 和 `POST /v1/disk/admin/bootstrap` 外，所有管理接口均需要在请求头中携带管理员 JWT Token：
```
Authorization: Bearer <admin-jwt-token>
```
管理员 JWT 与普通用户 JWT 使用相同的签名密钥，但 Payload 中包含 `adminUser` 和 `adminRole` 声明。
:::

## 管理员登录

管理员账号密码登录，获取管理员 JWT Token。

```
POST /v1/disk/admin/login
```

### 认证方式

公开接口，无需认证。

### 请求体

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `username` | `string` | 是 | 管理员用户名 |
| `password` | `string` | 是 | 密码 |

```json
{
  "username": "admin",
  "password": "secure_password"
}
```

### 响应示例

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "token": "eyJhbGciOiJIUzI1NiIs...",
    "username": "admin",
    "role": "admin"
  }
}
```

### curl 示例

```bash
curl -X POST http://localhost:9100/v1/disk/admin/login \
  -H "Content-Type: application/json" \
  -d '{"username": "admin", "password": "secure_password"}'
```

### 错误场景

| HTTP 状态码 | 场景说明 |
|------------|---------|
| 400 | 用户名或密码缺失 |
| 401 | 用户名或密码错误 |

---

## 初始化超级管理员

在系统首次部署、尚无管理员时创建第一个超级管理员。仅在管理员数量为 0 时可用。

```
POST /v1/disk/admin/bootstrap
```

### 认证方式

公开接口，仅在系统无管理员时可用。

### 请求体

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `username` | `string` | 是 | 管理员用户名 |
| `password` | `string` | 是 | 密码，最少 6 个字符 |
| `displayName` | `string` | 否 | 显示名称 |

```json
{
  "username": "superadmin",
  "password": "admin123456",
  "displayName": "超级管理员"
}
```

### 响应示例

**HTTP 201 Created**

```json
{
  "code": 0,
  "message": "created",
  "data": {
    "username": "superadmin",
    "role": "admin",
    "message": "first admin created successfully"
  }
}
```

### curl 示例

```bash
curl -X POST http://localhost:9100/v1/disk/admin/bootstrap \
  -H "Content-Type: application/json" \
  -d '{"username": "superadmin", "password": "admin123456", "displayName": "超级管理员"}'
```

### 错误场景

| HTTP 状态码 | 场景说明 |
|------------|---------|
| 400 | 用户名或密码缺失，或密码少于 6 字符 |
| 403 | 系统中已存在管理员，不允许再次引导 |

---

## 管理面板仪表盘

获取管理面板概览信息。

```
GET /v1/disk/admin/dashboard
```

### 认证方式

需要管理员 JWT Token。

### 响应示例

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "adminUser": "admin",
    "adminRole": "admin",
    "adminCount": 2
  }
}
```

### curl 示例

```bash
curl -X GET http://localhost:9100/v1/disk/admin/dashboard \
  -H "Authorization: Bearer eyJhbGciOiJIUzI1NiIs..."
```

---

## 管理员用户管理

### 列出管理员用户

```
GET /v1/disk/admin/users
```

#### 认证方式

需要管理员 JWT Token。

#### 响应示例

```json
{
  "code": 0,
  "message": "success",
  "data": [
    {
      "username": "admin",
      "role": "admin",
      "displayName": "系统管理员",
      "isActive": true,
      "createdBy": "bootstrap"
    },
    {
      "username": "ops_admin",
      "role": "admin",
      "displayName": "运维管理员",
      "isActive": true,
      "createdBy": "admin"
    }
  ]
}
```

#### curl 示例

```bash
curl -X GET http://localhost:9100/v1/disk/admin/users \
  -H "Authorization: Bearer eyJhbGciOiJIUzI1NiIs..."
```

---

### 创建管理员用户

```
POST /v1/disk/admin/users
```

#### 认证方式

需要管理员 JWT Token。

#### 请求体

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `username` | `string` | 是 | 用户名 |
| `password` | `string` | 是 | 密码，最少 6 个字符 |
| `role` | `string` | 否 | 角色，默认 `admin` |
| `displayName` | `string` | 否 | 显示名称 |

```json
{
  "username": "ops_admin",
  "password": "ops123456",
  "role": "admin",
  "displayName": "运维管理员"
}
```

#### 响应示例

**HTTP 201 Created**

```json
{
  "code": 0,
  "message": "created",
  "data": {
    "username": "ops_admin",
    "role": "admin",
    "displayName": "运维管理员"
  }
}
```

#### curl 示例

```bash
curl -X POST http://localhost:9100/v1/disk/admin/users \
  -H "Authorization: Bearer eyJhbGciOiJIUzI1NiIs..." \
  -H "Content-Type: application/json" \
  -d '{"username": "ops_admin", "password": "ops123456", "role": "admin", "displayName": "运维管理员"}'
```

---

### 修改管理员密码

```
PUT /v1/disk/admin/users/:username/password
```

#### 认证方式

需要管理员 JWT Token。

#### 路径参数

| 参数 | 类型 | 说明 |
|------|------|------|
| `username` | `string` | 用户名 |

#### 请求体

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `password` | `string` | 是 | 新密码，最少 6 个字符 |

```json
{
  "password": "new_secure_password"
}
```

#### 响应示例

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "message": "password updated"
  }
}
```

#### curl 示例

```bash
curl -X PUT http://localhost:9100/v1/disk/admin/users/ops_admin/password \
  -H "Authorization: Bearer eyJhbGciOiJIUzI1NiIs..." \
  -H "Content-Type: application/json" \
  -d '{"password": "new_secure_password"}'
```

---

### 删除管理员用户

```
DELETE /v1/disk/admin/users/:username
```

#### 认证方式

需要管理员 JWT Token。

#### 路径参数

| 参数 | 类型 | 说明 |
|------|------|------|
| `username` | `string` | 用户名 |

#### 响应示例

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "message": "admin user deleted"
  }
}
```

#### curl 示例

```bash
curl -X DELETE http://localhost:9100/v1/disk/admin/users/ops_admin \
  -H "Authorization: Bearer eyJhbGciOiJIUzI1NiIs..."
```

---

## API Key 管理

### 创建 API Key

创建一个新的 API Key，用于服务间调用或智能体集成。

```
POST /v1/disk/admin/api-keys
```

#### 认证方式

需要管理员 JWT Token。

#### 请求体

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `name` | `string` | 是 | API Key 名称，用于标识 |
| `department` | `string` | 否 | 所属部门 |

```json
{
  "name": "agent_sdk_key",
  "department": "engineering"
}
```

#### 响应示例

**HTTP 201 Created**

```json
{
  "code": 0,
  "message": "created",
  "data": {
    "id": 1,
    "key": "adk_live_a1b2c3d4e5f6g7h8i9j0k1l2m3n4o5p6",
    "keyPrefix": "adk_live",
    "keyName": "agent_sdk_key",
    "scope": "public_read",
    "department": "engineering",
    "createdAt": "2025-06-15T10:00:00Z"
  }
}
```

::: danger 重要
`key` 字段仅在创建时返回一次，后续无法再次查看。请务必妥善保存。
:::

#### curl 示例

```bash
curl -X POST http://localhost:9100/v1/disk/admin/api-keys \
  -H "Authorization: Bearer eyJhbGciOiJIUzI1NiIs..." \
  -H "Content-Type: application/json" \
  -d '{"name": "agent_sdk_key", "department": "engineering"}'
```

---

### 列出 API Key

获取所有 API Key 列表。

```
GET /v1/disk/admin/api-keys
```

#### 认证方式

需要管理员 JWT Token。

#### 响应示例

```json
{
  "code": 0,
  "message": "success",
  "data": [
    {
      "id": 1,
      "keyName": "agent_sdk_key",
      "keyPrefix": "adk_live",
      "scope": "public_read",
      "department": "engineering",
      "isRevoked": false,
      "lastUsedAt": "2025-06-15T14:30:00Z",
      "expiresAt": null,
      "createdBy": "admin",
      "createdAt": "2025-06-15T10:00:00Z"
    }
  ]
}
```

::: tip
列表接口不返回完整的 Key 值，仅返回前缀（`keyPrefix`）用于识别。
:::

#### curl 示例

```bash
curl -X GET http://localhost:9100/v1/disk/admin/api-keys \
  -H "Authorization: Bearer eyJhbGciOiJIUzI1NiIs..."
```

---

### 撤销 API Key

撤销一个 API Key，撤销后该 Key 立即失效。

```
DELETE /v1/disk/admin/api-keys/:id
```

#### 认证方式

需要管理员 JWT Token。

#### 路径参数

| 参数 | 类型 | 说明 |
|------|------|------|
| `id` | `uint64` | API Key 记录 ID |

#### 响应示例

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "message": "API key revoked"
  }
}
```

#### curl 示例

```bash
curl -X DELETE http://localhost:9100/v1/disk/admin/api-keys/1 \
  -H "Authorization: Bearer eyJhbGciOiJIUzI1NiIs..."
```

---

## 公共目录管理

### 创建公共目录

将一个目录配置为公共目录，使所有认证用户可见。

```
POST /v1/disk/admin/public-directories
```

#### 认证方式

需要管理员 JWT Token。

#### 请求体

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `displayName` | `string` | 是 | 公共目录显示名称 |
| `scope` | `string` | 是 | 可见范围，如 `public`、`department` |
| `department` | `string` | 否 | 所属部门（当 `scope` 为 `department` 时使用） |

```json
{
  "displayName": "公司公告",
  "scope": "public",
  "department": ""
}
```

#### 响应示例

**HTTP 201 Created**

```json
{
  "code": 0,
  "message": "created",
  "data": {
    "id": 1,
    "folderId": 100,
    "scope": "public",
    "department": "",
    "displayName": "公司公告",
    "fixedPath": "",
    "isActive": true,
    "createdBy": "admin"
  }
}
```

#### curl 示例

```bash
curl -X POST http://localhost:9100/v1/disk/admin/public-directories \
  -H "Authorization: Bearer eyJhbGciOiJIUzI1NiIs..." \
  -H "Content-Type: application/json" \
  -d '{"displayName": "公司公告", "scope": "public"}'
```

---

### 列出公共目录（管理）

获取所有公共目录的完整列表（包括未激活的）。

```
GET /v1/disk/admin/public-directories
```

#### 认证方式

需要管理员 JWT Token。

#### 响应示例

```json
{
  "code": 0,
  "message": "success",
  "data": [
    {
      "id": 1,
      "folderId": 100,
      "scope": "public",
      "department": "",
      "displayName": "公司公告",
      "fixedPath": "/公共资源/公司公告",
      "isActive": true,
      "createdBy": "admin",
      "createdAt": "2025-06-15T10:00:00Z",
      "updatedAt": "2025-06-15T10:00:00Z"
    }
  ]
}
```

#### curl 示例

```bash
curl -X GET http://localhost:9100/v1/disk/admin/public-directories \
  -H "Authorization: Bearer eyJhbGciOiJIUzI1NiIs..."
```

---

### 更新公共目录

更新公共目录的显示名称或启用/禁用状态。

```
PUT /v1/disk/admin/public-directories/:id
```

#### 认证方式

需要管理员 JWT Token。

#### 路径参数

| 参数 | 类型 | 说明 |
|------|------|------|
| `id` | `uint64` | 公共目录记录 ID |

#### 请求体

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `displayName` | `string` | 否 | 新的显示名称 |
| `isActive` | `bool` | 否 | 是否启用，默认 `true` |

```json
{
  "displayName": "公司公告（更新）",
  "isActive": true
}
```

#### 响应示例

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "id": 1,
    "folderId": 100,
    "scope": "public",
    "department": "",
    "displayName": "公司公告（更新）",
    "fixedPath": "/公共资源/公司公告",
    "isActive": true,
    "createdBy": "admin",
    "createdAt": "2025-06-15T10:00:00Z",
    "updatedAt": "2025-06-15T14:00:00Z"
  }
}
```

#### curl 示例

```bash
curl -X PUT http://localhost:9100/v1/disk/admin/public-directories/1 \
  -H "Authorization: Bearer eyJhbGciOiJIUzI1NiIs..." \
  -H "Content-Type: application/json" \
  -d '{"displayName": "公司公告（更新）", "isActive": true}'
```

---

### 删除公共目录

删除公共目录配置。

```
DELETE /v1/disk/admin/public-directories/:id
```

#### 认证方式

需要管理员 JWT Token。

#### 路径参数

| 参数 | 类型 | 说明 |
|------|------|------|
| `id` | `uint64` | 公共目录记录 ID |

#### 响应示例

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "message": "public directory deleted"
  }
}
```

#### curl 示例

```bash
curl -X DELETE http://localhost:9100/v1/disk/admin/public-directories/1 \
  -H "Authorization: Bearer eyJhbGciOiJIUzI1NiIs..."
```

---

## OAuth2 配置管理

### 获取 OAuth2 配置

获取当前的 OAuth2 认证配置信息。

```
GET /v1/disk/admin/oauth2
```

#### 认证方式

需要管理员 JWT Token。

#### 响应示例

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "id": 1,
    "name": "企业 OAuth2",
    "enabled": true,
    "clientId": "my-client-id",
    "authUrl": "https://auth.example.com/oauth2/authorize",
    "tokenUrl": "https://auth.example.com/oauth2/token",
    "userInfoUrl": "https://auth.example.com/oauth2/userinfo",
    "redirectUrl": "http://localhost:9100/auth/callback",
    "frontendUrl": "http://localhost:3000",
    "scopes": "openid profile email",
    "updatedBy": "admin",
    "createdAt": "2025-06-15T10:00:00Z",
    "updatedAt": "2025-06-15T10:00:00Z"
  }
}
```

::: tip
返回数据中不包含 `clientSecret`，出于安全考虑该字段仅存储不对外暴露。
:::

#### curl 示例

```bash
curl -X GET http://localhost:9100/v1/disk/admin/oauth2 \
  -H "Authorization: Bearer eyJhbGciOiJIUzI1NiIs..."
```

---

### 更新 OAuth2 配置

创建或更新 OAuth2 认证配置。

```
PUT /v1/disk/admin/oauth2
```

#### 认证方式

需要管理员 JWT Token。

#### 请求体

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `clientId` | `string` | 否 | OAuth2 客户端 ID |
| `clientSecret` | `string` | 否 | OAuth2 客户端密钥 |
| `authUrl` | `string` | 否 | 授权端点 URL |
| `tokenUrl` | `string` | 否 | 令牌端点 URL |
| `userInfoUrl` | `string` | 否 | 用户信息端点 URL |
| `redirectUrl` | `string` | 否 | 回调 URL |
| `frontendUrl` | `string` | 否 | 前端 URL |
| `scopes` | `string` | 否 | OAuth2 权限范围，多个以空格分隔 |
| `enabled` | `bool` | 否 | 是否启用 OAuth2 认证 |

```json
{
  "clientId": "my-client-id",
  "clientSecret": "my-client-secret",
  "authUrl": "https://auth.example.com/oauth2/authorize",
  "tokenUrl": "https://auth.example.com/oauth2/token",
  "userInfoUrl": "https://auth.example.com/oauth2/userinfo",
  "redirectUrl": "http://localhost:9100/auth/callback",
  "frontendUrl": "http://localhost:3000",
  "scopes": "openid profile email",
  "enabled": true
}
```

#### 响应示例

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "message": "OAuth2 config updated"
  }
}
```

#### curl 示例

```bash
curl -X PUT http://localhost:9100/v1/disk/admin/oauth2 \
  -H "Authorization: Bearer eyJhbGciOiJIUzI1NiIs..." \
  -H "Content-Type: application/json" \
  -d '{
    "clientId": "my-client-id",
    "clientSecret": "my-client-secret",
    "authUrl": "https://auth.example.com/oauth2/authorize",
    "tokenUrl": "https://auth.example.com/oauth2/token",
    "userInfoUrl": "https://auth.example.com/oauth2/userinfo",
    "redirectUrl": "http://localhost:9100/auth/callback",
    "frontendUrl": "http://localhost:3000",
    "scopes": "openid profile email",
    "enabled": true
  }'
```

---

### 测试 OAuth2 配置

验证当前 OAuth2 配置是否可以正常构建 OAuth2 客户端。

```
POST /v1/disk/admin/oauth2/test
```

#### 认证方式

需要管理员 JWT Token。

#### 请求体

无。

#### 响应示例

配置有效时：

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "status": "ok",
    "message": "OAuth2 client can be built from config"
  }
}
```

配置无效时：

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "status": "error",
    "message": "missing client_id"
  }
}
```

::: tip
该接口仅验证配置参数是否可以用于构建 OAuth2 客户端，不会实际发起 OAuth2 授权请求。
:::

#### curl 示例

```bash
curl -X POST http://localhost:9100/v1/disk/admin/oauth2/test \
  -H "Authorization: Bearer eyJhbGciOiJIUzI1NiIs..."
```
