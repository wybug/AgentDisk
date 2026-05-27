# 公共目录 + Admin 管理 + API Key 认证架构设计

## 1. 概述

AgentDisk 在原有用户个人文件隔离基础上，新增公共目录、部门目录、Admin 独立认证和 API Key 认证体系。设计原则：

- **最小侵入**：现有用户个人文件逻辑完全不变
- **双 JWT 体系**：用户 JWT（OAuth2/网关颁发）和 Admin JWT（独立用户名密码登录）互不干扰
- **路径隔离**：公共目录使用 `/public/` 和 `/department/{dept}/` 保留前缀
- **SDK 一致性**：Python SDK 操作接口不变，认证方式透明决定命名空间

## 2. 路径体系

```
/                          → 用户个人空间根目录
/public/{dir-name}/        → 全局公共目录
/department/{dept}/{dir}/  → 部门公共目录
```

- `public` 和 `department` 为保留路径前缀，用户不能在个人空间创建同名顶级目录
- 每个公共/部门目录对应一个系统用户 `__system_public__` 的 `disk_folder` 记录
- 公共目录的 `fixed_path` 固定不变，便于前端和 SDK 定位

## 3. 双 JWT 认证体系

### 用户 JWT（Claims）
```json
{
  "userId": "user001",
  "agentId": "agent_001",
  "agentGroupId": "group-a",
  "department": "engineering",
  "exp": 1234567890
}
```

### Admin JWT（AdminClaims）
```json
{
  "username": "admin",
  "role": "super_admin",
  "isAdmin": true,
  "exp": 1234567890
}
```

两种 JWT 使用相同签名密钥（HS256），但通过字段验证互相拒绝：
- `ParseToken` 检查 `claims.UserID != ""`，拒绝 Admin JWT
- `ParseAdminToken` 检查 `claims.IsAdmin && claims.Username != ""`，拒绝用户 JWT

## 4. 四重认证中间件（HybridAuth）

API 请求按以下顺序尝试认证：

1. **JWT Bearer Token** — 内部服务调用，`Authorization: Bearer <jwt>`
2. **OAuth2 Session Cookie** — Web 用户，`agentdisk_session` cookie
3. **Download Token** — 下载链接，`?t=<download_token>`
4. **API Key** — SDK/脚本，`X-API-Key: adk_xxx` 或 `?apiKey=adk_xxx`

API Key 认证后：
- `userId` 设为 `__system_public__`
- `department` 设为 API Key 的 department 字段
- `apiKeyScope` 设为 `public_read`
- 只能访问公共目录，不能访问用户个人目录

## 5. Admin 独立认证

- 独立登录路径 `POST /v1/disk/admin/login`，用户名/密码（bcrypt）
- 初始管理员通过 CLI 脚本创建：`go run scripts/add_admin/main.go -username admin -password <pass>`
- Admin 管理路由使用 `AdminAuth` + `AdminOnly` 中间件保护
- Admin JWT 存储在 `localStorage`（前端），与用户 JWT 隔离

## 6. API Key 设计

- 前缀 `adk_` + 64 位 hex（32 随机字节），总计 68 字符
- 仅创建时返回完整 Key，数据库存储 SHA-256 哈希
- `scope` 固定为 `public_read`，只能读公共/部门目录
- `department` 字段：空=全局+所有部门，有值=全局+匹配部门
- 支持过期时间和手动吊销

## 7. 公共目录权限模型

| 角色 | 读 | 写 | 管理 |
|------|----|----|------|
| 所有认证用户 | 全局公共目录 ✓ | ✗ | ✗ |
| 匹配部门用户 | 部门公共目录 ✓ | ✗ | ✗ |
| API Key | 匹配范围的公共目录 ✓ | ✗ | ✗ |
| Admin | ✓ | ✓ | ✓ |

## 8. 数据库新增表

- `disk_admin_user` — 管理员账户（独立认证）
- `disk_api_key` — API 密钥（SHA-256 哈希）
- `disk_public_directory` — 公共目录映射（folder_id → scope/department）
- `disk_oauth2_config` — OAuth2 动态配置

## 9. Python SDK 兼容性

```python
# JWT 认证 → 用户个人空间
client = AgentDiskClient(base_url="...", token="<jwt>")

# API Key 认证 → 公共目录空间
client = AgentDiskClient(base_url="...", api_key="adk_xxx")
```

- 同一套 `list_folders()`, `list_files()`, `get_file()` 方法
- 后端根据认证类型透明切换命名空间
- `_resolver.py` 无需修改

## 10. 安全约束

- 公共目录只读（仅 Admin 可写）
- `public` 和 `department` 为保留路径，用户创建顶级目录时被拒绝
- API Key 不能访问用户个人目录
- OAuth2 配置支持动态管理，`client_secret` 脱敏返回
- Admin 密码 bcrypt 哈希存储
- 所有凭证通过环境变量注入，禁止硬编码
