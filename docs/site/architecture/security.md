# 安全设计

AgentDisk 安全体系贯穿认证、授权、数据隔离、传输加密、审计日志等层面。本文档详细说明各项安全机制的设计和实现。

## 安全架构总览

```
┌─────────────────────────────────────────────────────┐
│                    安全防护层                         │
│                                                     │
│  ┌─────────────────────────────────────────────┐   │
│  │  HybridAuth 中间件                            │   │
│  │  JWT Bearer → OAuth2 Session → API Key → Token │   │
│  └──────────────────┬──────────────────────────┘   │
│                     │                               │
│  ┌──────────────────▼──────────────────────────┐   │
│  │  数据隔离层                                   │   │
│  │  userId 强制绑定 · OSS 私有桶 · 部门隔离       │   │
│  └──────────────────┬──────────────────────────┘   │
│                     │                               │
│  ┌──────────────────▼──────────────────────────┐   │
│  │  存储安全层                                   │   │
│  │  预签名 URL · 哈希存储 · 加密传输              │   │
│  └──────────────────┬──────────────────────────┘   │
│                     │                               │
│  ┌──────────────────▼──────────────────────────┐   │
│  │  审计日志层                                   │   │
│  │  操作日志 · 访问记录 · 90 天保留               │   │
│  └─────────────────────────────────────────────┘   │
│                                                     │
└─────────────────────────────────────────────────────┘
```

## HybridAuth 四合一认证中间件

HybridAuth 是 AgentDisk 的核心认证中间件，按优先级依次尝试四种认证方式，首个成功即停止：

```
请求进入 HybridAuth
  │
  ├─ 1. 检查 Authorization: Bearer <token>
  │     └─ 有效 JWT → 提取 userId / agentId / agentGroupId
  │
  ├─ 2. 检查 agentdisk_session Cookie
  │     └─ 有效 OAuth2 Session → 从 Session Store 提取 userId
  │
  ├─ 3. 检查 ?t=<download_token> 查询参数
  │     └─ 有效下载令牌 → 提取绑定的 userId + fileId
  │
  └─ 4. 检查 X-API-Key Header 或 ?apiKey 查询参数
        └─ 有效 API Key → userId=__system_public__, department=Key.department

全部失败 → 返回 401 Unauthorized
```

### 认证方式对比

| 认证方式 | 适用场景 | 身份提取 | 权限范围 |
|---------|---------|---------|---------|
| JWT Bearer | Agent、内部服务 | userId + agentId + agentGroupId | 完整功能 |
| OAuth2 Session | Web 前端 | userId（从 Session Store） | 完整功能 |
| 下载令牌 | 文件下载 | userId + fileId（绑定） | 仅指定文件下载 |
| API Key | 脚本、外部系统 | __system_public__ + department | 公共目录只读 |

## 数据隔离

### userId 强制绑定

所有业务数据表均包含 `userId` 字段，所有查询**必须**携带 `userId` 条件：

```sql
-- 所有文件查询强制带 userId
SELECT * FROM disk_file WHERE user_id = ? AND id = ?;

-- 目录查询强制带 userId
SELECT * FROM disk_folder WHERE user_id = ? AND parent_id = ?;

-- 权限查询强制带 userId
SELECT * FROM disk_permission WHERE user_id = ? AND agent_id = ?;
```

### 隔离保证

- **ORM 层面**：所有 GORM 查询自动注入 `userId` 条件
- **禁止无过滤查询**：不允许出现不包含 `userId` 的数据查询（公共目录除外）
- **跨用户拒绝**：任何跨用户访问尝试均返回 `403 Forbidden`

### 公共目录隔离

公共目录使用系统用户 `__system_public__` 作为 userId，与个人空间完全隔离：

```
个人空间:  userId = "user001"  → 仅 user001 可访问
公共目录:  userId = "__system_public__"  → 按权限规则控制访问
```

## OSS 私有桶策略

所有文件存储在 MinIO/OSS 的**私有桶**中，永不对公网开放。

### 访问方式

唯一访问方式是通过后端生成的**预签名 URL**（Presigned URL）：

```
1. 用户/Agent 请求 GET /v1/disk/files/:id
2. 后端验证身份和权限
3. 后端生成 OSS 预签名 URL（有效期 5-15 分钟）
4. 返回给客户端用于下载或预览
```

### 安全保证

- **禁止公开读写**：OSS 桶策略为 `private`，不允许匿名访问
- **禁止裸链暴露**：API 响应中只返回预签名 URL，不暴露 OSS 直链
- **时效控制**：预签名 URL 设置过期时间，超时自动失效
- **签名绑定**：预签名 URL 包含签名信息，无法篡改

### OSS 路径规则

```
oss://bucket/{userId}/files/{fileId}/{fileName}
oss://bucket/__system_public__/files/{fileId}/{fileName}
```

每个用户的文件存储在独立的 OSS 路径前缀下，物理隔离。

## JWT Claims 与验证

### 用户 JWT

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

### Admin JWT

```json
{
  "username": "admin",
  "role": "super_admin",
  "isAdmin": true,
  "iat": 1747000000,
  "exp": 1747259200
}
```

### 验证规则

- **签名算法**：HS256（HMAC-SHA256）
- **密钥**：通过 `JWT_SECRET` 环境变量配置
- **过期检查**：自动验证 `exp` 字段，过期 Token 拒绝
- **双体系隔离**：
  - `ParseToken` 检查 `claims.UserID != ""`，拒绝 Admin JWT
  - `ParseAdminToken` 检查 `claims.IsAdmin && claims.Username != ""`，拒绝用户 JWT
- **不可伪造**：无密钥无法生成有效 JWT

### Token 有效期

| Token 类型 | 有效期 | 说明 |
|-----------|--------|------|
| 用户 JWT（网关签发） | 72 小时 | 网关签发给 Agent 服务 |
| Admin JWT | 可配置 | 管理后台登录 |
| OAuth2 Access Token | 24 小时 | Web 前端 Session |
| 下载令牌 | 5 分钟 | 临时下载直链 |

## API Key 哈希存储

API Key 使用 SHA-256 哈希存储，确保即使数据库泄露也无法还原原始 Key。

### 存储流程

```
1. 生成: adk_<64位hex>  (adk_a1b2c3d4e5f6...)
2. 哈希: SHA-256("adk_a1b2c3d4e5f6...") → "e3b0c44298fc1c14..."
3. 存储: 将哈希值写入 disk_api_key.key_hash
4. 返回: 仅创建时返回完整 Key，后续仅显示脱敏格式 (adk_****)
```

### 验证流程

```
1. 请求携带 X-API-Key: adk_a1b2c3d4e5f6...
2. 后端计算: SHA-256("adk_a1b2c3d4e5f6...")
3. 数据库查询: WHERE key_hash = ?
4. 匹配 → 认证成功; 不匹配 → 401 Unauthorized
```

## Admin 密码哈希

管理员密码使用 bcrypt 算法哈希存储，成本因子为 10。

### 存储流程

```
1. 用户设置密码: "admin123"
2. bcrypt 哈希: $2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy
3. 存储: 写入 disk_admin_user.password_hash
```

### 安全特性

- **自适应成本**：bcrypt 计算成本可随硬件升级提高，抵抗暴力破解
- **内置盐值**：每个密码使用独立随机盐，防止彩虹表攻击
- **不可逆**：无法从哈希值反推原始密码

## 审计日志

### 日志范围

所有高危操作均记录审计日志：

| 操作类型 | 日志内容 | 保留期限 |
|---------|---------|---------|
| 文件上传/删除 | 操作人、文件信息、时间 | 90 天 |
| 权限变更 | 授权人、被授权 Agent、权限类型 | 90 天 |
| 分享创建/撤销 | 操作人、分享信息 | 90 天 |
| Admin 操作 | 管理员、操作类型、目标资源 | 90 天 |
| API Key 创建/吊销 | 管理员、Key 信息 | 90 天 |
| 登录/认证 | 用户、IP、时间、结果 | 90 天 |

### 日志格式

```json
{
  "timestamp": "2026-05-27T10:00:00Z",
  "userId": "user001",
  "action": "file.delete",
  "resource": {"type": "file", "id": 123, "name": "report.pdf"},
  "ip": "192.168.1.100",
  "userAgent": "Mozilla/5.0..."
}
```

## 下载令牌安全

下载令牌使用 HMAC-SHA256 签名，具有以下安全特性：

### 令牌结构

```
Base64Url(payload).Base64Url(signature)

payload = {
  "uid": "user001",       // 绑定用户
  "fid": "123",           // 绑定文件
  "iat": 1747000000,      // 签发时间
  "exp": 1747000300,      // 过期时间（5 分钟后）
  "nonce": "a1b2c3d4"     // 随机数，防重放
}
signature = HMAC-SHA256(DL_TOKEN_SECRET, payloadBase64)
```

### 安全保证

| 特性 | 说明 |
|------|------|
| 绑定用户 | `uid` 字段绑定签发者，无法跨用户使用 |
| 绑定文件 | `fid` 字段绑定特定文件，无法用于下载其他文件 |
| 时效控制 | 5 分钟后自动失效（`exp` 字段） |
| 签名保护 | HMAC-SHA256 签名，无法伪造或篡改 |
| 随机防重放 | `nonce` 随机数，每次签发唯一 |
| 一次性校验 | 可选启用一次性校验，使用后立即失效 |

## CORS 与 CSRF 防护

### CORS 配置

```go
// 允许的源（生产环境应限制为前端域名）
AllowOrigins: []string{
  "https://agentdisk.example.com",
  "http://localhost:9101",  // 开发环境
}

// 允许的方法
AllowMethods: []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"}

// 允许的请求头
AllowHeaders: []string{
  "Authorization",
  "X-API-Key",
  "Content-Type",
}

// 凭证
AllowCredentials: true

// 预检缓存
MaxAge: 12 * time.Hour
```

### CSRF 防护

- **SameSite Cookie**：`agentdisk_session` 设置 `SameSite=Lax`，阻止跨站请求携带 Cookie
- **HttpOnly**：Session Cookie 设置 `HttpOnly`，JavaScript 无法读取
- **Secure**：生产环境设置 `Secure`，仅通过 HTTPS 传输
- **State 参数**：OAuth2 授权流程使用 `state` 参数防止 CSRF 攻击

## 敏感数据处理

### 禁止暴露的信息

| 信息类型 | 处理方式 |
|---------|---------|
| OSS 直链 | 仅返回预签名 URL，不暴露 OSS 内部地址 |
| JWT 密钥 | 环境变量注入，代码中禁止硬编码 |
| 数据库密码 | 环境变量注入，日志中禁止明文打印 |
| API Key 明文 | 数据库仅存储哈希值，API 响应脱敏 |
| Admin 密码 | bcrypt 哈希存储，日志中禁止记录 |
| OAuth2 Client Secret | 管理界面脱敏显示，API 响应脱敏 |
| 内部错误堆栈 | 生产环境返回通用错误信息，不暴露堆栈 |

### 响应脱敏规则

```json
// API Key 列表响应 -- Key 脱敏
{
  "key": "adk_****"
}

// OAuth2 配置响应 -- Secret 脱敏
{
  "clientSecret": "****"
}

// 错误响应 -- 不暴露内部信息
{
  "code": 500,
  "message": "服务内部错误",
  "data": null
}
```

### 环境变量管理

所有凭证通过环境变量注入，禁止在代码中硬编码：

| 变量 | 用途 | 安全要求 |
|------|------|---------|
| `JWT_SECRET` | JWT 签名密钥 | 32+ 字符随机字符串 |
| `DL_TOKEN_SECRET` | 下载令牌签名密钥 | 32+ 字符随机字符串 |
| `OAUTH2_CLIENT_SECRET` | OAuth2 客户端密钥 | 从 OAuth2 Provider 获取 |
| `DB_PASSWORD` | 数据库密码 | 强密码 |
| `OSS_ACCESS_KEY` | OSS 访问密钥 | 从 OSS 服务获取 |
| `OSS_SECRET_KEY` | OSS 秘密密钥 | 从 OSS 服务获取 |

### 日志安全

- 日志中禁止记录完整 Token、密钥、密码
- 文件路径使用相对路径，不暴露服务器物理路径
- 错误日志记录错误类型和摘要，不记录请求体中的敏感字段
- 审计日志记录操作类型，不记录操作内容中的敏感数据
