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

管理员可以在运行时动态修改 OAuth2 配置，无需重启服务。OAuth2 配置存储在数据库中，通过 Admin API 管理。

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
    "issuerUrl": "https://your-idp.com",
    "redirectUrl": "https://your-domain.com/auth/callback",
    "scopes": "openid,profile"
  }'
```

**参数说明：**

| 参数 | 说明 |
|------|------|
| `enabled` | 是否启用 OAuth2 |
| `clientId` | OAuth2 客户端 ID |
| `clientSecret` | OAuth2 客户端密钥 |
| `issuerUrl` | OAuth2 提供方基础地址（如 `https://your-idp.com`），端点 URL 自动派生 |
| `redirectUrl` | 回调地址 |
| `scopes` | 授权范围（逗号分隔），默认 `openid,profile` |

端点 URL 由 `issuerUrl` 自动派生：

| 端点 | URL |
|------|-----|
| Authorization | `{issuerUrl}/oauth2/authorize` |
| Token | `{issuerUrl}/oauth2/token` |
| UserInfo | `{issuerUrl}/oauth2/userinfo` |

### 无配置时的行为

- `GET /auth/status` 返回 `{"oauth2": false}`
- 前端显示"OAuth2 登录未配置"提示页

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

## MFA 多因素认证

管理员可启用 WebAuthn 通行密钥作为第二因素，提升管理后台的登录安全性。

::: info 前提条件
需在 `config.yaml` 中启用 WebAuthn：

```yaml
webauthn:
  enabled: true
  rp_display_name: "AgentDisk Admin"
  rp_id: "localhost"
  rp_origins: "http://localhost:9101"
  timeout: 60000
```

`rp_id` 和 `rp_origins` 需与实际部署域名一致，否则浏览器会拒绝 WebAuthn 操作。
:::

### 启用 MFA

1. 以管理员身份登录管理后台
2. 导航到「MFA 设置」页面（`/admin/mfa`）
3. 点击「注册通行密钥」，在浏览器弹窗中完成指纹/面容/安全密钥验证
4. 为通行密钥命名并确认
5. 注册成功后，MFA 开关变为可用，点击开关启用 MFA

### MFA 登录流程

启用 MFA 后，管理员登录流程变为两步：

1. 输入用户名和密码
2. 系统返回 `mfaRequired: true`，前端跳转到通行密钥验证页
3. 浏览器弹出 WebAuthn 验证弹窗（指纹/面容/安全密钥）
4. 验证通过后获得管理员 JWT Token

### 管理通行密钥

```bash
# 查看已注册的通行密钥
curl http://localhost:9100/v1/disk/admin/mfa/credentials \
  -H "Authorization: Bearer $ADMIN_TOKEN"

# 重命名通行密钥
curl -X PUT http://localhost:9100/v1/disk/admin/mfa/credentials/1 \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"name": "办公电脑"}'

# 删除通行密钥
curl -X DELETE http://localhost:9100/v1/disk/admin/mfa/credentials/1 \
  -H "Authorization: Bearer $ADMIN_TOKEN"

# 查看 MFA 状态
curl http://localhost:9100/v1/disk/admin/mfa/status \
  -H "Authorization: Bearer $ADMIN_TOKEN"

# 开启/关闭 MFA
curl -X PUT http://localhost:9100/v1/disk/admin/mfa/enabled \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"enabled": true}'
```

::: warning
- 删除最后一个通行密钥后，MFA 将自动关闭
- 无通行密钥时无法开启 MFA
- 详细的 API 参数说明请参考 [管理接口 - MFA](/api/admin#mfa-多因素认证管理)
:::

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
| `/v1/disk/admin/mfa/registration/begin` | POST | 开始注册通行密钥 |
| `/v1/disk/admin/mfa/registration/finish` | POST | 完成注册通行密钥 |
| `/v1/disk/admin/mfa/credentials` | GET | 列出通行密钥 |
| `/v1/disk/admin/mfa/credentials/:id` | PUT | 重命名通行密钥 |
| `/v1/disk/admin/mfa/credentials/:id` | DELETE | 删除通行密钥 |
| `/v1/disk/admin/mfa/status` | GET | 获取 MFA 状态 |
| `/v1/disk/admin/mfa/enabled` | PUT | 开启/关闭 MFA |
| `/v1/disk/admin/mfa/login/begin` | POST | 开始 MFA 登录验证 |
| `/v1/disk/admin/mfa/login/finish` | POST | 完成 MFA 登录验证 |
