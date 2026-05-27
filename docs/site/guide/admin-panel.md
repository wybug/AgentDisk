# 管理后台

AgentDisk 提供独立的管理后台，用于系统管理、用户管理、OAuth2 配置、API Key 管理和公共目录管理。管理后台与用户端分离，使用独立的管理员认证体系。

## 访问管理后台

### 登录入口

在浏览器中访问 `http://localhost:9101/admin` 进入管理后台登录页面。

### 首次初始化

首次使用时，系统中没有管理员账户。访问管理后台登录页面时，系统会自动检测初始化状态，并跳转到管理员创建页面：

1. 打开 `http://localhost:9101/admin/login`
2. 系统检测到未初始化，自动跳转到管理员创建页面（`/admin/setup`）
3. 填写用户名、密码、确认密码和显示名称（可选）
4. 点击"创建管理员"，系统自动创建管理员并登录
5. 创建成功后自动跳转到管理后台首页

::: tip
初始化状态通过 `GET /v1/disk/admin/init-status` 接口检测。已初始化后访问 `/admin/setup` 会自动跳转回登录页。
:::

### CLI 创建管理员

也可以通过命令行创建管理员：

```bash
curl -X POST http://localhost:9100/v1/disk/admin/bootstrap \
  -H "Content-Type: application/json" \
  -d '{
    "username": "admin",
    "password": "your-strong-password",
    "displayName": "系统管理员"
  }'
```

::: warning
Bootstrap 接口仅在系统中不存在任何管理员时可用。一旦创建了第一个管理员，该接口将返回 403 错误。
:::

返回示例：

```json
{
  "code": 0,
  "message": "created",
  "data": {
    "username": "admin",
    "role": "admin",
    "message": "first admin created successfully"
  }
}
```

### 管理员登录

```bash
curl -X POST http://localhost:9100/v1/disk/admin/login \
  -H "Content-Type: application/json" \
  -d '{
    "username": "admin",
    "password": "your-strong-password"
  }'
```

返回管理员 JWT Token：

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "token": "eyJhbGciOiJIUzI1NiJ9...",
    "username": "admin",
    "role": "admin"
  }
}
```

后续所有管理接口都需要在请求头中携带此 Token：

```bash
export ADMIN_TOKEN="eyJhbGciOiJIUzI1NiJ9..."
```

## 管理后台认证

管理后台使用独立的 JWT 认证体系，与用户端的 JWT Token 相互隔离：

- **管理员 Token**：通过 `POST /v1/disk/admin/login` 获取
- **认证方式**：`Authorization: Bearer <admin_token>`
- **中间件**：`AdminAuth` + `AdminOnly` 双重验证
- **Token 有效期**：与 `config.yaml` 中 `jwt.expire_hours` 一致（默认 72 小时）

## 管理员用户管理

### 查看管理员列表

```bash
curl http://localhost:9100/v1/disk/admin/users \
  -H "Authorization: Bearer $ADMIN_TOKEN"
```

返回示例：

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
    }
  ]
}
```

### 创建管理员

```bash
curl -X POST http://localhost:9100/v1/disk/admin/users \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "username": "ops-admin",
    "password": "secure-password-123",
    "role": "admin",
    "displayName": "运维管理员"
  }'
```

**参数说明：**

| 参数 | 必填 | 说明 |
|------|------|------|
| `username` | 是 | 管理员用户名（唯一） |
| `password` | 是 | 密码（最少 6 个字符） |
| `role` | 否 | 角色，默认 `admin` |
| `displayName` | 否 | 显示名称 |

### 修改管理员密码

```bash
curl -X PUT http://localhost:9100/v1/disk/admin/users/ops-admin/password \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "password": "new-secure-password"
  }'
```

### 删除管理员

```bash
curl -X DELETE http://localhost:9100/v1/disk/admin/users/ops-admin \
  -H "Authorization: Bearer $ADMIN_TOKEN"
```

## 管理面板 Dashboard

Dashboard 接口提供系统概览信息：

```bash
curl http://localhost:9100/v1/disk/admin/dashboard \
  -H "Authorization: Bearer $ADMIN_TOKEN"
```

返回示例：

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

## OAuth2 动态配置

管理员可以在运行时动态修改 OAuth2 配置，无需重启服务。

### 查看当前 OAuth2 配置

```bash
curl http://localhost:9100/v1/disk/admin/oauth2 \
  -H "Authorization: Bearer $ADMIN_TOKEN"
```

### 更新 OAuth2 配置

```bash
curl -X PUT http://localhost:9100/v1/disk/admin/oauth2 \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "enabled": true,
    "clientId": "your-client-id",
    "clientSecret": "your-client-secret",
    "authUrl": "https://your-idp.com/oauth2/authorize",
    "tokenUrl": "https://your-idp.com/oauth2/token",
    "userInfoUrl": "https://your-idp.com/oauth2/userinfo",
    "redirectUrl": "https://your-domain.com/auth/callback",
    "frontendUrl": "https://your-domain.com",
    "scopes": "openid,profile,email"
  }'
```

**参数说明：**

| 参数 | 说明 |
|------|------|
| `enabled` | 是否启用 OAuth2 |
| `clientId` | OAuth2 客户端 ID |
| `clientSecret` | OAuth2 客户端密钥 |
| `authUrl` | 授权端点 URL |
| `tokenUrl` | 令牌端点 URL |
| `userInfoUrl` | 用户信息端点 URL |
| `redirectUrl` | 回调地址 |
| `frontendUrl` | 前端首页地址 |
| `scopes` | 授权范围（逗号分隔） |

### 测试 OAuth2 连接

```bash
curl -X POST http://localhost:9100/v1/disk/admin/oauth2/test \
  -H "Authorization: Bearer $ADMIN_TOKEN"
```

返回：

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

## API Key 管理

API Key 用于外部系统集成，允许通过 `X-API-Key` 请求头或 `apiKey` 查询参数进行认证。

### 创建 API Key

```bash
curl -X POST http://localhost:9100/v1/disk/admin/api-keys \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "数据分析服务",
    "department": "engineering"
  }'
```

返回：

```json
{
  "code": 0,
  "message": "created",
  "data": {
    "id": 1,
    "key": "ak_a1b2c3d4e5f6g7h8i9j0k1l2m3n4o5p6",
    "keyPrefix": "ak_a1b2",
    "keyName": "数据分析服务",
    "scope": "public_read",
    "department": "engineering",
    "createdAt": "2026-01-15T10:00:00Z"
  }
}
```

::: danger
创建时返回的 `key` 是完整的 API Key，**仅显示一次**。请立即保存到安全位置，后续无法再次查看完整密钥。
:::

### 查看 API Key 列表

```bash
curl http://localhost:9100/v1/disk/admin/api-keys \
  -H "Authorization: Bearer $ADMIN_TOKEN"
```

列表中只显示 Key 的前缀（`keyPrefix`），不显示完整密钥：

```json
{
  "code": 0,
  "message": "success",
  "data": [
    {
      "id": 1,
      "keyName": "数据分析服务",
      "keyPrefix": "ak_a1b2",
      "scope": "public_read",
      "department": "engineering",
      "isRevoked": false,
      "lastUsedAt": "2026-01-15T12:00:00Z",
      "createdBy": "admin",
      "createdAt": "2026-01-15T10:00:00Z"
    }
  ]
}
```

### 撤销 API Key

```bash
curl -X DELETE http://localhost:9100/v1/disk/admin/api-keys/1 \
  -H "Authorization: Bearer $ADMIN_TOKEN"
```

撤销后：

- API Key 立即失效，无法再用于认证
- 撤销操作不可逆，如需恢复请创建新的 API Key

### API Key 属性

| 属性 | 说明 |
|------|------|
| `keyName` | 密钥名称，用于标识用途 |
| `keyPrefix` | 密钥前缀（前 4 位），用于列表展示 |
| `scope` | 权限范围，默认 `public_read` |
| `department` | 所属部门，影响公共目录的可见范围 |
| `isRevoked` | 是否已撤销 |
| `lastUsedAt` | 最后使用时间 |
| `expiresAt` | 过期时间（可选） |

### 使用 API Key

```bash
# 通过请求头传递
curl http://localhost:9100/v1/disk/public-directories \
  -H "X-API-Key: ak_a1b2c3d4e5f6g7h8i9j0k1l2m3n4o5p6"

# 通过查询参数传递
curl "http://localhost:9100/v1/disk/public-directories?apiKey=ak_a1b2c3d4e5f6g7h8i9j0k1l2m3n4o5p6"
```

## 公共目录管理

管理员可以在管理后台创建、编辑和删除公共目录映射。详细说明请参考 [公共目录](/guide/public-directories) 章节。

### 管理接口速览

```bash
# 创建公共目录
POST /v1/disk/admin/public-directories

# 查看所有公共目录
GET /v1/disk/admin/public-directories

# 更新公共目录
PUT /v1/disk/admin/public-directories/:id

# 删除公共目录
DELETE /v1/disk/admin/public-directories/:id
```

## 管理后台接口总览

| 接口 | 方法 | 说明 |
|------|------|------|
| `/v1/disk/admin/init-status` | GET | 检查系统是否已初始化 |
| `/v1/disk/admin/bootstrap` | POST | 初始化超级管理员（仅首次） |
| `/v1/disk/admin/login` | POST | 管理员登录 |
| `/v1/disk/admin/dashboard` | GET | 系统概览 |
| `/v1/disk/admin/users` | GET | 管理员列表 |
| `/v1/disk/admin/users` | POST | 创建管理员 |
| `/v1/disk/admin/users/:username/password` | PUT | 修改密码 |
| `/v1/disk/admin/users/:username` | DELETE | 删除管理员 |
| `/v1/disk/admin/api-keys` | GET | API Key 列表 |
| `/v1/disk/admin/api-keys` | POST | 创建 API Key |
| `/v1/disk/admin/api-keys/:id` | DELETE | 撤销 API Key |
| `/v1/disk/admin/public-directories` | GET | 公共目录列表 |
| `/v1/disk/admin/public-directories` | POST | 创建公共目录 |
| `/v1/disk/admin/public-directories/:id` | PUT | 更新公共目录 |
| `/v1/disk/admin/public-directories/:id` | DELETE | 删除公共目录 |
| `/v1/disk/admin/oauth2` | GET | 获取 OAuth2 配置 |
| `/v1/disk/admin/oauth2` | PUT | 更新 OAuth2 配置 |
| `/v1/disk/admin/oauth2/test` | POST | 测试 OAuth2 连接 |
