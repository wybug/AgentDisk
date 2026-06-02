# OAuth2 协议参考文档

本文档描述 AgentDisk 使用的 OAuth2 协议细节，供开发和测试参考。

---

## 1 授权流程

AgentDisk 采用 **Authorization Code + PKCE** 流程（RFC 6749 + RFC 7636），涉及三方：

- **浏览器**（用户代理）
- **AgentDisk 后端**（OAuth2 Client，port 9100）
- **网关**（OAuth2 Provider，port 3100）

```
浏览器                   AgentDisk 后端              网关
  |                           |                        |
  |  GET /auth/login          |                        |
  |-------------------------->|                        |
  |                           |  生成 state + PKCE     |
  |  302 → /oauth2/authorize  |                        |
  |<--------------------------|                        |
  |  (code_challenge, state)  |                        |
  |                                                    |
  |  GET /oauth2/authorize      ---------------------->|
  |                           用户未登录，跳转登录页      |
  |  302 → /login              <----------------------|
  |                           用户输入凭据登录            |
  |  POST /login (credentials) ---------------------->|
  |  302 → /authorize（授权页）<----------------------|
  |                           用户点击"允许"             |
  |  POST /oauth2/approve     ---------------------->|
  |  302 → redirect_uri?code  <----------------------|
  |                           (authorization code)     |
  |                                                    |
  |  GET /auth/callback?code  |                        |
  |-------------------------->|                        |
  |                           |  POST /oauth2/token    |
  |                           |  (code + code_verifier)|
  |                           |----------------------->|
  |                           |  access_token + userId |
  |                           |<-----------------------|
  |                           |                        |
  |                           |  GET /oauth2/userinfo  |
  |                           |  (Bearer access_token) |
  |                           |----------------------->|
  |                           |  {userId, userName}    |
  |                           |<-----------------------|
  |                           |                        |
  |  302 → frontend_url       |  设置 session cookie   |
  |<--------------------------|                        |
```

### 1.1 SSO 无感登录（prompt=none）

从网关页面跳转 AgentDisk 时，URL 带参数 `?from=gateway`：

1. AgentDisk 检测 `from=gateway` → 附加 `prompt=none` 参数到授权请求
2. 网关检测用户已有 session → 直接签发 code（无授权页）
3. 若用户未登录 → 返回 `error=login_required`，AgentDisk 降级为标准流程

---

## 2 端点说明

### 2.1 Authorization Endpoint

```
GET {issuer_url}/oauth2/authorize
```

| 参数 | 必填 | 说明 |
|------|------|------|
| `response_type` | 是 | 固定 `code` |
| `client_id` | 是 | 客户端标识（如 `agentdisk`） |
| `redirect_uri` | 是 | 回调地址，需与注册的 redirect_uri 匹配 |
| `state` | 是 | CSRF 防护随机值 |
| `code_challenge` | 是 | PKCE challenge（S256 方法） |
| `code_challenge_method` | 是 | 固定 `S256` |
| `scope` | 否 | 空格分隔的权限范围（如 `openid profile`） |
| `prompt` | 否 | `none` 表示静默授权 |

成功响应：`302 redirect_uri?code={authorization_code}&state={state}`

错误响应：`302 redirect_uri?error={error_code}&state={state}`

| 错误码 | 含义 |
|--------|------|
| `invalid_client` | 未知的 client_id |
| `invalid_request` | redirect_uri 不匹配 |
| `unsupported_response_type` | response_type 不是 `code` |
| `login_required` | prompt=none 时用户未登录 |

### 2.2 Token Endpoint

```
POST {issuer_url}/oauth2/token
Content-Type: application/x-www-form-urlencoded
```

| 参数 | 必填 | 说明 |
|------|------|------|
| `grant_type` | 是 | 固定 `authorization_code` |
| `code` | 是 | Authorization Endpoint 返回的 code |
| `redirect_uri` | 是 | 与授权请求一致 |
| `client_id` | 是 | 客户端标识 |
| `code_verifier` | 是 | PKCE verifier（与 code_challenge 对应） |

成功响应：

```json
{
  "access_token": "gw_xxx",
  "token_type": "Bearer",
  "expires_in": 86400,
  "refresh_token": "gw_r_xxx",
  "userId": "user_001"
}
```

### 2.3 UserInfo Endpoint

```
GET {issuer_url}/oauth2/userinfo
Authorization: Bearer {access_token}
```

成功响应：

```json
{
  "userId": "user_001",
  "userName": "张三"
}
```

---

## 3 PKCE 机制（RFC 7636）

PKCE（Proof Key for Code Exchange）防止授权码截获攻击。

### 3.1 流程

1. 客户端生成随机 `code_verifier`（43-128 字符，Base64url 编码）
2. 计算 `code_challenge = BASE64URL(SHA256(code_verifier))`
3. 授权请求携带 `code_challenge` + `code_challenge_method=S256`
4. Token 请求携带 `code_verifier`
5. 服务端验证 `SHA256(code_verifier) == code_challenge`

### 3.2 AgentDisk 实现

```
code_verifier  = BASE64URL(RANDOM(32 bytes))
code_challenge = BASE64URL(SHA256(code_verifier))
```

---

## 4 配置说明

### 4.1 数据库配置（唯一来源）

OAuth2 配置存储在 `disk_oauth2_config` 表，通过 Admin API (`/v1/disk/admin/oauth2`) 管理。

| 字段 | 说明 | 示例 |
|------|------|------|
| `client_id` | 客户端 ID | `agentdisk` |
| `client_secret` | 客户端密钥 | `agentdisk-secret` |
| `issuer_url` | OAuth2 提供方基础 URL | `http://localhost:3100` |
| `redirect_url` | 授权回调地址 | `http://localhost:9100/auth/callback` |
| `scopes` | 权限范围（逗号分隔） | `openid,profile` |
| `enabled` | 是否启用 | `true` |

端点 URL 由 `issuer_url` 自动派生：

| 端点 | URL |
|------|-----|
| Authorization | `{issuer_url}/oauth2/authorize` |
| Token | `{issuer_url}/oauth2/token` |
| UserInfo | `{issuer_url}/oauth2/userinfo` |

### 4.2 无配置时的行为

- `GET /auth/status` 返回 `{"oauth2": false}`
- `GET /auth/login` 返回 500 `"OAuth2 not configured"`
- 前端 401 拦截器检查 `/auth/status`，显示"OAuth2 未配置"提示页

---

## 5 安全考虑

| 措施 | 说明 |
|------|------|
| State 参数 | 每次 /auth/login 生成随机 state，callback 时校验，防 CSRF |
| PKCE | S256 方法，防授权码截获 |
| 授权码有效期 | 10 分钟，一次性使用 |
| Session Cookie | HttpOnly, SameSite-Lax |
| Access Token 有效期 | 24 小时 |
| 重定向 URI 白名单 | 网关验证 redirect_uri 必须在白名单中 |

---

## 6 RFC 参考

| RFC | 标题 | 说明 |
|-----|------|------|
| [RFC 6749](https://datatracker.ietf.org/doc/html/rfc6749) | The OAuth 2.0 Authorization Framework | OAuth2 核心框架 |
| [RFC 6750](https://datatracker.ietf.org/doc/html/rfc6750) | The OAuth 2.0 Authorization Framework: Bearer Token Usage | Bearer Token 用法 |
| [RFC 7636](https://datatracker.ietf.org/doc/html/rfc7636) | Proof Key for Code Exchange by OAuth Public Clients (PKCE) | PKCE 扩展 |
| [RFC 8414](https://datatracker.ietf.org/doc/html/rfc8414) | OAuth 2.0 Authorization Server Metadata | OIDC Discovery / Issuer URL |
