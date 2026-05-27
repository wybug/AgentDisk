# API Key 认证

API Key 是 AgentDisk 提供的轻量级认证方式，适用于脚本自动化、外部系统集成和公共目录只读访问场景。API Key 通过管理后台创建，无需 OAuth2 登录流程。

## 概述

| 特性 | 说明 |
|------|------|
| Key 格式 | `adk_` 前缀 + 64 位 hex（32 随机字节），共 68 字符 |
| 存储方式 | 数据库仅存储 SHA-256 哈希值，明文仅在创建时返回一次 |
| 权限范围 | `public_read` -- 只能读取公共目录和部门目录 |
| 认证位置 | `X-API-Key` 请求头 或 `?apiKey` 查询参数 |
| 有效期 | 可设置过期时间，也可手动吊销 |

## 创建 API Key

通过管理后台创建 API Key：

### 方式一：管理后台界面

1. 访问管理后台（`/admin`），使用管理员账号登录
2. 进入「API Key 管理」页面
3. 点击「创建 API Key」
4. 填写配置：
   - **名称**：Key 的用途描述（如 "监控脚本"）
   - **部门**：绑定部门（留空表示全局 + 所有部门）
   - **过期时间**：可选，留空表示永不过期
5. 创建成功后，**立即复制并保存 Key**，此为唯一一次查看完整 Key 的机会

### 方式二：管理 API

```bash
curl -X POST http://localhost:9100/v1/disk/admin/api-keys \
  -H "Authorization: Bearer <admin-jwt>" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "监控脚本",
    "department": "engineering",
    "scope": "public_read",
    "expiresAt": "2026-12-31T23:59:59Z"
  }'

# 返回示例：
# {
#   "code": 200,
#   "data": {
#     "id": 1,
#     "name": "监控脚本",
#     "key": "adk_a1b2c3d4...x64chars",
#     "scope": "public_read",
#     "department": "engineering",
#     "expiresAt": "2026-12-31T23:59:59Z",
#     "createdAt": "2026-05-27T10:00:00Z"
#   }
# }
```

> **重要**：`key` 字段仅在创建响应中返回。后续查询仅返回脱敏后的前缀（`adk_****`）。请务必妥善保存。

## Key 格式与安全

### 格式

```
adk_<64位十六进制字符串>
```

示例：`adk_a1b2c3d4e5f6a7b8c9d0e1f2a3b4c5d6e7f8a9b0c1d2e3f4a5b6c7d8e9f0a1b2`

### 安全机制

- **仅创建时返回完整 Key**：数据库存储 SHA-256 哈希值，无法反推原始 Key
- **随机生成**：使用加密安全随机数生成器（32 字节随机源）
- **前缀标识**：`adk_` 前缀便于识别 Key 类型
- **不可逆**：即使数据库泄露，也无法还原 API Key 明文

### 哈希存储

```
原始 Key: adk_a1b2c3d4...
存储值:   SHA-256("adk_a1b2c3d4...") → "e3b0c44298fc1c149afbf4c8996fb924..."
```

验证时，后端对传入的 Key 计算 SHA-256 哈希，与数据库中存储的哈希值比对。

## Scope 与权限

### 当前支持的 Scope

| Scope | 权限 | 说明 |
|-------|------|------|
| `public_read` | 读取公共/部门目录 | 只能通过 API Key 访问公共资源 |

使用 API Key 认证时：

- `userId` 设为 `__system_public__`（系统公共用户）
- `department` 设为 API Key 的 department 字段
- `apiKeyScope` 设为 `public_read`
- 只能访问公共目录，不能访问用户个人目录

### 部门绑定规则

| department 值 | 可访问范围 |
|--------------|-----------|
| 空（不填） | 全局公共目录 + 所有部门公共目录 |
| `engineering` | 全局公共目录 + engineering 部门目录 |
| `marketing` | 全局公共目录 + marketing 部门目录 |

## 使用 API Key

### 方式一：X-API-Key 请求头（推荐）

```bash
curl http://localhost:9100/v1/disk/public-directories \
  -H "X-API-Key: adk_a1b2c3d4...x64chars"
```

### 方式二：apiKey 查询参数

适用于无法设置请求头的场景（如浏览器直接访问）：

```bash
curl "http://localhost:9100/v1/disk/public-directories?apiKey=adk_a1b2c3d4...x64chars"
```

### SDK 中使用

Python SDK 原生支持 API Key 认证：

```python
from agentdisk import AgentDiskClient

# API Key 认证
client = AgentDiskClient(
    base_url="http://localhost:9100",
    api_key="adk_a1b2c3d4...x64chars",
)

# 列出公共目录
dirs = client.list_public_directories()

# 列出公共目录中的文件
files = client.list_files("/public/shared-docs")
```

### 异步客户端

```python
from agentdisk import AsyncAgentDiskClient

async with AsyncAgentDiskClient(
    base_url="http://localhost:9100",
    api_key="adk_a1b2c3d4...x64chars",
) as client:
    dirs = await client.list_public_directories()
    files = await client.list_files("/public/shared-docs")
```

## Key 管理

### 查询 API Key 列表

```bash
curl http://localhost:9100/v1/disk/admin/api-keys \
  -H "Authorization: Bearer <admin-jwt>"
```

返回列表中 Key 为脱敏格式（`adk_****`），不包含完整 Key。

### 吊销 API Key

```bash
curl -X DELETE http://localhost:9100/v1/disk/admin/api-keys/1 \
  -H "Authorization: Bearer <admin-jwt>"
```

吊销后，使用该 Key 的所有请求将返回 `401 Unauthorized`。

### Key 轮换

API Key 不支持自动轮换。轮换流程：

```
1. 创建新的 API Key
2. 更新所有使用旧 Key 的系统配置
3. 验证新 Key 正常工作
4. 吊销旧 Key
```

## 与 JWT 认证的对比

| 特性 | JWT Bearer | API Key |
|------|-----------|---------|
| 获取方式 | 网关签发 / 手动生成 | 管理后台创建 |
| 有效期 | 短期（默认 72 小时） | 长期或永久 |
| 身份 | 用户 / Agent（userId + agentId） | 系统公共用户（__system_public__） |
| 权限范围 | 完整功能（个人空间 + 公共目录） | 仅公共目录只读 |
| 适用场景 | Agent 服务间调用 | 脚本、外部系统、监控 |
| 存储安全 | Token 短期有效 | SHA-256 哈希存储 |

## 安全最佳实践

### Key 存储

- 不要将 API Key 硬编码在源代码中
- 通过环境变量注入：`AGENTDISK_API_KEY=adk_xxxx`
- 使用密钥管理服务（如 Vault、AWS Secrets Manager）存储 Key
- 禁止将 Key 提交到版本控制系统

### Key 使用

- 优先使用 `X-API-Key` 请求头，避免 Key 出现在 URL 中
- 使用 `apiKey` 查询参数时，注意浏览器历史记录和服务器日志可能记录 URL
- 为不同用途创建不同的 Key，便于审计和吊销
- 设置合理的过期时间，定期轮换

### Key 保护

- 创建后立即保存 Key，关闭页面后无法再次查看
- 疑似泄露时立即吊销，创建新 Key 替换
- 定期审查 API Key 列表，清理不再使用的 Key
- 监控 API Key 使用日志，发现异常访问及时处理

### 网络安全

- 生产环境务必使用 HTTPS，防止 Key 在传输中被截获
- 限制 API Key 的 IP 访问范围（如通过网关或反向代理配置）
- 启用审计日志，记录所有通过 API Key 的访问
