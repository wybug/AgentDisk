# 认证集成

AgentDisk 采用 **HybridAuth** 四合一认证中间件，自动识别请求中的认证凭据，统一转换为 `userId` + 权限信息。本文档详细说明每种认证方式的工作原理和集成方法。

## 认证方式总览

| 认证方式 | 凭据位置 | 适用场景 | 身份类型 |
|---------|---------|---------|---------|
| JWT Bearer | `Authorization: Bearer <token>` | 内部服务调用、Agent 请求 | 用户 / Agent |
| OAuth2 Session | `agentdisk_session` Cookie | Web 前端用户 | 用户 |
| API Key | `X-API-Key` Header 或 `?apiKey=` | 脚本、外部系统 | 系统公共用户 |
| 下载令牌 | `?t=<downloadToken>` | 文件下载链接 | 匿名（绑定用户+文件） |

HybridAuth 按上述顺序依次尝试，首个成功即停止。所有认证失败的请求返回 `401 Unauthorized`。

## JWT 认证（内部服务调用）

JWT 是 Agent 和内部服务调用 AgentDisk API 的主要认证方式。Token 由网关签发，后端验证。

### JWT 签发

使用 `pkg/jwt` 包签发 Token：

```go
import "github.com/agentdisk/agent-disk/pkg/jwt"

// 不带 Agent 组
token, err := jwt.GenerateToken(secret, userId, agentId, expireHours)

// 带 Agent 组
token, err := jwt.GenerateTokenWithGroup(secret, userId, agentId, agentGroupId, expireHours)
```

### JWT Claims 结构

```json
{
  "userId": "user001",
  "agentId": "agent_001",
  "agentGroupId": "team-a",
  "department": "engineering",
  "iat": 1747000000,
  "exp": 1747259200
}
```

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `userId` | string | 是 | 用户唯一标识 |
| `agentId` | string | 否 | Agent 身份标识，为空时视为用户请求 |
| `agentGroupId` | string | 否 | Agent 组标识，同组 Agent 共享产物读写权限 |
| `department` | string | 否 | 部门标识，用于部门级公共目录权限 |
| `iat` | int64 | 是 | 签发时间 |
| `exp` | int64 | 是 | 过期时间 |

### 身份判定规则

- `agentId` 为空：请求视为**用户身份**，享有完全访问权限
- `agentId` 非空：请求视为 **Agent 身份**，受 Agent 权限规则约束

### 调用示例

```bash
curl -X GET http://localhost:9100/v1/disk/space \
  -H "Authorization: Bearer eyJhbGciOiJIUzI1NiJ9..."
```

### JWT 双体系隔离

AgentDisk 使用两套互相隔离的 JWT 体系：

| 体系 | Claims | 用途 |
|------|--------|------|
| 用户 JWT | `userId` + `agentId` + `agentGroupId` | 业务 API 调用 |
| Admin JWT | `username` + `role` + `isAdmin` | 管理后台 API |

两套 JWT 使用相同签名密钥（HS256），但通过字段验证互相拒绝：

- `ParseToken` 检查 `claims.UserID != ""`，拒绝 Admin JWT
- `ParseAdminToken` 检查 `claims.IsAdmin && claims.Username != ""`，拒绝用户 JWT

## OAuth2 Web 用户认证

Web 前端用户通过 OAuth2 协议完成登录认证，网关作为 OAuth2 Provider。

### 所需配置

| 配置项 | 值 |
|--------|-----|
| `client_id` | `agentdisk` |
| `redirect_uri` | `https://<agentdisk-host>/auth/callback` |
| OAuth2 Scopes | `openid`, `profile` |

### 方式 A：网关无感跳转（推荐）

用户已在网关 Web 登录，从网关页面跳转到 AgentDisk 时无需再次登录或授权。

```
1. 网关 Web 生成跳转链接: https://<agentdisk-host>/?from=gateway
2. AgentDisk 检测到 from=gateway → 使用 prompt=none 发起 OAuth2 授权
3. 网关检测到用户已有 Session → 自动批准（无授权页）→ 返回 code
4. AgentDisk 用 code 换取 access_token → 建立 Session → 用户无感进入
```

**网关侧要求**：

- `/oauth2/authorize` 端点支持 `prompt=none` 参数
- 对 `client_id=agentdisk` 自动批准，不显示授权确认页
- 用户未登录时返回 `error=login_required`（而非跳转登录页）

### 方式 B：标准 OAuth2 流程

用户直接在浏览器访问 AgentDisk 时的标准登录流程：

```
1. 用户访问 /auth/login
2. 自动跳转 Agent 网关授权页
3. 用户登录并授权后回调 /auth/callback?code=xxx
4. AgentDisk 自动完成 Token 交换，建立 Session
```

### Session 管理

| 属性 | 值 |
|------|-----|
| Cookie 名称 | `agentdisk_session` |
| 属性 | HttpOnly, Secure, SameSite-Lax |
| 有效期 | 与 OAuth2 Access Token 一致（默认 24 小时） |
| 续期 | 通过 refresh_token 自动续期 |

## Agent 注册与管理

Agent 通过网关 REST API 注册。注册后，网关在代理请求时将 `agentId` 写入 JWT，后端据此进行权限控制。

### 注册 Agent

```bash
POST /api/agents
Content-Type: application/json
Cookie: gw_session=<session>

{
  "agentId": "writer-01",
  "agentName": "写作助手",
  "agentGroupId": "team-a"
}
```

需要登录（`gw_session` cookie）。`agentGroupId` 选填，同组 Agent 共享文件读写权限。

### 查询已注册 Agent

```bash
GET /api/agents
Cookie: gw_session=<session>
```

返回当前用户注册的所有 Agent 列表。

### 注销 Agent

```bash
DELETE /api/agents/:agentId
Cookie: gw_session=<session>
```

只能注销属于当前用户的 Agent。

### 数据持久化

Agent 注册信息存储在网关本地 SQLite 数据库（`gateway/data/agents.db`），包含：

| 字段 | 说明 |
|------|------|
| `agent_id` | Agent 唯一标识 |
| `agent_name` | Agent 显示名称 |
| `user_id` | 归属用户 |
| `agent_group_id` | Agent 组（同组 Agent 自动共享产物读写） |

## Agent 自动权限规则

Agent 请求通过 JWT 中的 `agentId` 和 `agentGroupId` 字段标识身份。后端根据以下规则判断权限：

| 场景 | 权限 | 说明 |
|------|------|------|
| 用户请求（JWT 无 agentId） | 完全访问 | `file.UserID == userID` 即放行 |
| Agent 访问自己创建的文件 | 自动 read/write | `sourceAgent == agentId` |
| Agent 访问同组 Agent 创建的文件 | 自动 read/write | `sourceAgentGroup == agentGroupId` |
| Agent 访问用户手动上传的文件 | 需显式授权 | `isArtifact == false` |
| Agent 请求 delete/owner 权限 | 需显式授权 | 必须在 `disk_permission` 表中配置 |
| 跨用户访问 | 始终拒绝 | `userID` 不匹配 |

### 授权决策流程

```
请求进入 → HybridAuth 从 JWT 提取 userId + agentId + agentGroupId
  ├─ agentId 为空 → 用户身份 → 资源属于该用户即放行
  └─ agentId 非空 → Agent 身份 → 检查权限:
       1. 查询资源归属（ownerID, sourceAgent, sourceAgentGroup, isArtifact）
       2. 跨用户 → 拒绝
       3. 非 Agent 产物 (isArtifact=false) → 走显式权限表
       4. Agent 产物 + read/write:
            ├─ sourceAgent == agentId → 放行（自己创建）
            ├─ sourceAgentGroup == agentGroupId → 放行（同组）
            └─ 否则 → 走显式权限表
       5. delete/owner → 走显式权限表
```

## 路径授权

除精确资源 ID 授权外，AgentDisk 支持基于 Glob 通配符的**路径授权**，可对一组文件/文件夹批量授权。

### 两种授权模式

| 模式 | 适用场景 | 来源 |
|------|---------|------|
| 资源 ID 授权 | 对特定文件/文件夹授权 | 文件浏览器"授予权限"快捷操作 |
| 路径授权 | 对一组文件/文件夹批量授权 | 权限管理页面路径授权表单 |

### 通配符语法

| 模式 | 含义 | 示例 |
|------|------|------|
| `*` | 单级匹配（不含 `/`） | `/Documents/*` 匹配 `/Documents/file.pdf` |
| `**` | 多级匹配（含 `/`） | `/Documents/**` 匹配 `/Documents/a/b/file.pdf` |
| `*.ext` | 扩展名匹配 | `/**/*.txt` 匹配所有 txt 文件 |
| 精确路径 | 无通配符时精确匹配 | `/Documents/report.pdf` 仅匹配自身 |

### Agent 目标配置

路径授权支持两种目标（可同时配置）：

```json
{"agentId": "writer-01"}                       // 授权给单个 Agent
{"agentGroupId": "team-a"}                     // 授权给整个 Agent 组
{"agentId": "writer-01", "agentGroupId": "team-a"}  // 同时指定
```

### 授权 API 示例

```bash
# 路径授权：让 agent-01 可读所有文件
curl -X POST http://localhost:9100/v1/disk/permissions \
  -H "Authorization: Bearer <jwt>" \
  -H "Content-Type: application/json" \
  -d '{"agentId":"agent-01","resourcePath":"/**","permission":"read"}'

# 组路径授权：让 team-a 组可写 Documents 下所有文件
curl -X POST http://localhost:9100/v1/disk/permissions \
  -H "Authorization: Bearer <jwt>" \
  -H "Content-Type: application/json" \
  -d '{"agentGroupId":"team-a","resourcePath":"/Documents/**","permission":"write"}'

# 资源 ID 快捷授权：对特定文件授权
curl -X POST http://localhost:9100/v1/disk/permissions \
  -H "Authorization: Bearer <jwt>" \
  -H "Content-Type: application/json" \
  -d '{"agentId":"agent-01","resourceId":42,"resType":"file","permission":"read"}'
```

### 权限检查优先级

```
1. 精确资源 ID 授权（agentId 匹配）
2. 路径授权规则（agentId，逐一 glob 匹配）
3. Agent 组精确资源 ID 授权
4. Agent 组路径授权规则（逐一 glob 匹配）
5. 原有 artifact 自动规则（同组、同 Agent）
```

## 网关代理认证

网关为已登录用户签发 JWT，代理到 Agent 服务时携带，使 Agent 能以用户身份访问 AgentDisk API。

### 认证链路

```
浏览器 → 网关 /process (gw_session cookie + agentId)
       → 网关验证 Agent 归属 → 签发 JWT (userId + agentId + agentGroupId)
       → 代理到 Agent (Authorization: Bearer <jwt>)
       → Agent 拿 JWT 调用后端 /v1/disk/* 接口
```

### JWT 签发参数

| 参数 | 值 |
|------|-----|
| secret | 与后端 `config.yaml` 中 `jwt.secret` 一致 |
| payload（用户请求） | `{ userId: "<当前用户ID>" }` |
| payload（Agent 请求） | `{ userId: "<当前用户ID>", agentId: "<AgentID>", agentGroupId: "<组ID>" }` |
| expiresIn | 72h |

### 安全约束

- 未登录用户访问 `/process` 返回 401
- Agent 未注册或不属于当前用户返回 403
- JWT 绑定当前登录用户，无法伪造
- Agent 服务使用 JWT 调用后端时自动携带用户和 Agent 身份

## 下载令牌

下载令牌用于生成无需登录即可下载文件的临时直链。

### 生成流程

```
1. 用 JWT 调用 POST /v1/disk/files/:id/download-token
2. 获取 downloadToken（5 分钟有效）
3. 拼接 URL: https://<agentdisk-host>/v1/disk/files/download?t=<downloadToken>
4. 用户点击 URL 直接下载，无需登录
```

### 请求示例

```bash
# 1. 获取下载令牌
curl -X POST http://localhost:9100/v1/disk/files/123/download-token \
  -H "Authorization: Bearer <jwt>"

# 返回: {"code":200,"data":{"downloadToken":"xxx","expiresIn":300}}

# 2. 下载文件
curl -o file.pdf "http://localhost:9100/v1/disk/files/download?t=xxx"
```

### 令牌格式

```
Base64Url(payload).Base64Url(signature)

payload = {"uid": "userId", "fid": "fileId", "iat": timestamp, "exp": timestamp, "nonce": "random"}
signature = HMAC-SHA256(dlSecret, payloadBase64)
```

### 安全约束

- 令牌绑定 `userId + fileId`，无法跨用户使用
- 5 分钟后自动失效（可通过 `download_token.expire_seconds` 配置）
- 令牌不可伪造（HMAC-SHA256 签名）
- 含随机 `nonce`，防止重放攻击

## 错误码参考

| HTTP Code | Code | 含义 | 处理建议 |
|-----------|------|------|---------|
| 400 | BAD_REQUEST | 参数错误 | 检查请求参数 |
| 401 | UNAUTHORIZED | 未认证 | 检查 Token 或重新登录 |
| 403 | FORBIDDEN | 无权限 | 检查资源归属和权限；Agent 检查是否已注册、是否属于当前用户 |
| 404 | NOT_FOUND | 资源不存在 | 检查 ID 是否正确 |
| 500 | INTERNAL_ERROR | 服务内部错误 | 查看服务端日志排查 |

## 环境变量清单

| 变量名 | 用途 | 必填 |
|--------|------|------|
| `JWT_SECRET` | JWT 签名密钥 | 是 |
| `DL_TOKEN_SECRET` | 下载令牌签名密钥 | 是 |
| `OAUTH2_CLIENT_ID` | OAuth2 客户端 ID | 启用 OAuth2 时 |
| `OAUTH2_CLIENT_SECRET` | OAuth2 客户端密钥 | 启用 OAuth2 时 |
| `DB_PASSWORD` | 数据库密码 | 是 |
| `OSS_ACCESS_KEY` | OSS 访问密钥 | 是 |
| `OSS_SECRET_KEY` | OSS 秘密密钥 | 是 |

## 集成检查清单

- [ ] 获取 `JWT_SECRET` 环境变量
- [ ] 调用 API 前生成 JWT（使用 `pkg/jwt` 包）
- [ ] 所有请求携带 `Authorization: Bearer <token>` Header
- [ ] 处理 401/403 错误（重新生成 Token / 跳过无权限资源）
- [ ] 如需生成下载直链，调用 `POST /v1/disk/files/:id/download-token`
- [ ] 通过网关 API 注册 Agent（`POST /api/agents`）
- [ ] Agent 请求携带 `agentId` 参数
- [ ] 运行 `bash scripts/test_auth.sh` 验证集成正确性
