# AgentDisk 认证集成指南

## 适用对象

所有需要调用 AgentDisk API 的 Agent（T1-T6）及外部平台。

---

## 1. 内部服务调用（JWT）

### 你需要什么

- JWT Secret（与环境变量 `JWT_SECRET` 一致）
- userId 和 agentId

### 如何获取 Token

```go
import "github.com/agentdisk/agent-disk/pkg/jwt"

token, err := jwt.GenerateToken(secret, userId, agentId, expireHours)
```

### 如何调用 API

所有 `/v1/disk/*` 接口需在 Header 中携带：

```
Authorization: Bearer <token>
```

### 请求示例

```bash
curl -X GET http://agentdisk:8080/v1/disk/space \
  -H "Authorization: Bearer eyJhbG..."
```

### 错误码

- `401`: Token 缺失/无效/过期
- `403`: 无权限访问该资源

---

## 2. Web 用户访问（OAuth2）

### 你需要什么

- Agent 网关的 OAuth2 端点配置
- `client_id`: agentdisk
- `redirect_uri`: `https://<agentdisk-host>/auth/callback`

### 方式 A: 从网关 Web 无感跳转（推荐）

用户已在网关 Web 登录，从网关页面跳转到 AgentDisk 时**无需再次登录或授权**。

1. 网关 Web 生成跳转链接: `https://<agentdisk-host>/?from=gateway`
2. AgentDisk 检测到 `from=gateway` → 使用 `prompt=none` 发起 OAuth2 授权
3. 网关检测到用户已有 Session → 自动批准（无授权页）→ 返回 code
4. AgentDisk 用 code 换取 access_token → 建立 Session → 用户无感进入

**网关侧要求：**

- `/oauth2/authorize` 端点支持 `prompt=none` 参数
- 对 `client_id=agentdisk` 自动批准，不显示授权确认页
- 用户未登录时返回 `error=login_required`（而非跳转登录页）

### 方式 B: 独立访问 AgentDisk（标准 OAuth2）

用户直接在浏览器访问 AgentDisk。

1. 用户访问 `/auth/login`
2. 自动跳转 Agent 网关授权页
3. 用户登录并授权后回调 `/auth/callback?code=xxx`
4. AgentDisk 自动完成 Token 交换，建立 Session

### Session 管理

- Cookie: `agentdisk_session`，HttpOnly, Secure, SameSite=Lax
- 有效期: 与 OAuth2 Access Token 一致（默认 24 小时）
- 续期: 通过 refresh_token 自动续期

---

## 3. 网关代理认证（Agent 对话）

网关为已登录用户签发 JWT，代理到 Agent 服务时携带，使 Agent 能以用户身份访问 AgentDisk API。

### 认证链路

```
浏览器 → 网关 /process (gw_session cookie)
       → 网关从 session 取 userId → 签发 JWT
       → 代理到 Agent (Authorization: Bearer <jwt>)
       → Agent 拿 JWT 调用后端 /v1/disk/* 接口
```

### JWT 签发参数

| 参数 | 值 |
|------|-----|
| secret | 与后端 `config.yaml` 中 `jwt.secret` 一致 |
| payload | `{ userId: "<当前用户ID>" }` |
| expiresIn | 72h |

### 安全约束

- 未登录用户访问 `/process` 返回 401
- JWT 绑定当前登录用户，无法伪造
- Agent 服务使用 JWT 调用后端时自动携带用户身份

---

## 4. 生成下载直链

### 你需要什么

- JWT（用于认证）
- 文件 ID（fileId）

### 步骤

1. 用 JWT 调用 `POST /v1/disk/files/:id/download-token`
2. 获取 `downloadToken`（5 分钟有效）
3. 拼接 URL: `https://<agentdisk-host>/v1/disk/files/download?t=<downloadToken>`
4. 用户点击 URL 直接下载，无需登录

### 请求示例

```bash
# 1. 获取下载令牌
curl -X POST http://agentdisk:8080/v1/disk/files/123/download-token \
  -H "Authorization: Bearer <jwt>"

# 返回: {"code":200,"data":{"downloadToken":"xxx","expiresIn":300}}

# 2. 下载文件
curl -o file.pdf "http://agentdisk:8080/v1/disk/files/download?t=xxx"
```

### 安全约束

- 下载令牌绑定 userId + fileId，无法跨用户使用
- 5 分钟后自动失效（可通过 `download_token.expire_seconds` 配置）
- 令牌不可伪造（HMAC-SHA256 签名）

---

## 5. 公共常量与约定

### JWT Claims 结构

```json
{
  "userId": "string, 必填",
  "agentId": "string, 选填",
  "iat": 1747000000,
  "exp": 1747259200
}
```

### 下载令牌格式

```
Base64Url(payload).Base64Url(signature)

payload = {"uid": "userId", "fid": "fileId", "iat": timestamp, "exp": timestamp, "nonce": "random"}
signature = HMAC-SHA256(dlSecret, payloadBase64)
```

### OAuth2 Scopes

- `openid`: 基础身份
- `profile`: 用户信息

---

## 6. 错误码速查

| HTTP Code | Code | 含义 | 处理建议 |
|-----------|------|------|---------|
| 400 | BAD_REQUEST | 参数错误 | 检查请求参数 |
| 401 | UNAUTHORIZED | 未认证 | 检查 Token 或重新登录 |
| 403 | FORBIDDEN | 无权限 | 检查资源归属和权限 |
| 404 | NOT_FOUND | 资源不存在 | 检查 ID 是否正确 |
| 500 | INTERNAL_ERROR | 服务内部错误 | 联系 T0 |

---

## 7. 环境变量清单

| 变量名 | 用途 | 必填 |
|--------|------|------|
| `JWT_SECRET` | JWT 签名密钥 | 是 |
| `DL_TOKEN_SECRET` | 下载令牌签名密钥 | 是 |
| `OAUTH2_CLIENT_ID` | OAuth2 客户端 ID | 启用 OAuth2 时 |
| `OAUTH2_CLIENT_SECRET` | OAuth2 客户端密钥 | 启用 OAuth2 时 |
| `DB_PASSWORD` | 数据库密码 | 是 |
| `OSS_ACCESS_KEY` | OSS 访问密钥 | 是 |
| `OSS_SECRET_KEY` | OSS 秘密密钥 | 是 |

---

## 8. 各 Agent 集成检查清单

- [ ] 获取 `JWT_SECRET` 环境变量
- [ ] 调用 API 前生成 JWT（使用 `pkg/jwt` 包）
- [ ] 所有请求携带 `Authorization: Bearer <token>` Header
- [ ] 处理 401/403 错误（重新生成 Token / 跳过无权限资源）
- [ ] 如需生成下载直链，调用 `POST /v1/disk/files/:id/download-token`
- [ ] 运行 `bash scripts/test_auth.sh` 验证集成正确性
