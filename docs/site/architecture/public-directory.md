# 公共目录架构

AgentDisk 在原有用户个人文件隔离基础上，新增公共目录和部门目录体系。本文档描述公共目录的设计目标、数据模型、管理方式和访问规则。

## 设计目标

- **最小侵入**：现有用户个人文件逻辑完全不变，公共目录作为独立命名空间存在
- **只读共享**：公共目录对所有认证用户只读开放，仅管理员可写入
- **部门隔离**：支持部门级公共目录，用户仅可见所属部门的目录
- **API Key 适配**：API Key 认证后只能访问公共目录，与个人空间隔离
- **SDK 一致性**：Python SDK 操作接口不变，认证方式透明决定命名空间

## 路径体系

```
/                            → 用户个人空间根目录
/public/{dir-name}/          → 全局公共目录（所有认证用户可见）
/department/{dept}/{dir}/    → 部门公共目录（匹配部门用户可见）
```

### 保留路径

`public` 和 `department` 为保留路径前缀。用户不能在个人空间创建同名顶级目录，后端会拒绝此类操作。

### 示例

```
/reports/q3-summary.pdf                    # 用户的个人文件
/public/company-policy/employee-handbook.pdf  # 全局公共文件
/department/engineering/architecture-docs/   # 工程部门公共文件
/department/marketing/campaign-assets/       # 市场部门公共文件
```

## 数据模型

### disk_public_directory 表

| 字段 | 类型 | 说明 |
|------|------|------|
| id | int | 主键 |
| folder_id | int | 关联的 disk_folder.id |
| scope | string | 目录范围：`global`（全局）或 `department`（部门） |
| department | string | 部门标识，scope 为 global 时为空 |
| name | string | 公共目录名称 |
| description | string | 目录描述 |
| created_at | datetime | 创建时间 |
| updated_at | datetime | 更新时间 |

### 目录映射机制

每个公共/部门目录对应：

1. 一个系统用户 `__system_public__` 的 `disk_folder` 记录
2. 一条 `disk_public_directory` 映射记录
3. 公共目录的 `fixed_path` 固定不变，便于前端和 SDK 定位

```
disk_folder (userId=__system_public__, folderName="company-policy")
    ↕ 映射
disk_public_directory (scope=global, name="company-policy")
```

### 目录范围

| scope 值 | department 值 | 可见范围 |
|----------|--------------|---------|
| `global` | 空 | 所有认证用户 + 所有 API Key |
| `department` | `engineering` | engineering 部门用户 + 匹配部门的 API Key |
| `department` | `marketing` | marketing 部门用户 + 匹配部门的 API Key |

## 双 JWT 认证体系

公共目录的访问依赖 AgentDisk 的双 JWT 认证体系：

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

### 隔离机制

两套 JWT 使用相同签名密钥（HS256），但通过字段验证互相拒绝：

- `ParseToken` 检查 `claims.UserID != ""`，拒绝 Admin JWT
- `ParseAdminToken` 检查 `claims.IsAdmin && claims.Username != ""`，拒绝用户 JWT

## HybridAuth 与公共目录

HybridAuth 四重认证中间件处理公共目录的访问控制：

| 认证方式 | userId | department | 可访问范围 |
|---------|--------|-----------|-----------|
| JWT Bearer | 真实 userId | JWT 中的 department | 个人空间 + 匹配的公共目录 |
| OAuth2 Session | 真实 userId | OAuth2 返回的 department | 个人空间 + 匹配的公共目录 |
| API Key | `__system_public__` | Key 的 department | 仅公共目录（匹配 scope） |
| 下载令牌 | 绑定的 userId | N/A | 特定文件 |

### API Key 认证后的身份映射

API Key 认证后，系统自动设置以下身份信息：

```python
userId = "__system_public__"
department = api_key.department  # Key 绑定的部门
apiKeyScope = "public_read"
```

这使得 API Key 只能访问公共目录，不能访问用户个人目录。

## 管理界面

### Admin 管理面板

管理员通过 Admin 面板管理公共目录，功能包括：

- **创建公共目录**：设置名称、范围（全局/部门）、部门、描述
- **编辑公共目录**：修改描述等信息
- **删除公共目录**：移除公共目录映射（可选删除底层文件）
- **查看文件列表**：浏览公共目录中的文件
- **上传文件**：向公共目录上传文件

### Admin 认证

- 独立登录路径：`POST /v1/disk/admin/login`
- 初始管理员通过 CLI 创建：`go run scripts/add_admin/main.go -username admin -password <pass>`
- Admin JWT 存储在浏览器 `localStorage`，与用户 JWT 隔离

## 用户侧公共目录浏览

### 前端展示

普通用户登录后可在文件管理器中看到「公共目录」区域：

1. **全局公共目录**：所有登录用户可见
2. **部门公共目录**：仅匹配用户 `department` 字段的目录可见
3. **只读操作**：浏览、搜索、下载、预览、生成分享链接
4. **禁止操作**：上传、删除、重命名、移动（仅 Admin 可操作）

### API 访问

```bash
# 列出可见的公共目录（根据认证身份过滤）
GET /v1/disk/public-directories

# 列出公共目录中的文件
GET /v1/disk/folders/:folderId/files

# 获取公共文件详情（含预签名 URL）
GET /v1/disk/files/:id
```

### SDK 访问

```python
from agentdisk import AgentDiskClient

# JWT 认证 -- 可访问个人空间 + 公共目录
client = AgentDiskClient(base_url="...", token=jwt_token)
dirs = client.list_public_directories()
files = client.list_files("/public/company-policy")

# API Key 认证 -- 只能访问公共目录
client = AgentDiskClient(base_url="...", api_key="adk_xxx")
dirs = client.list_public_directories()
files = client.list_files("/public/company-policy")
```

## 权限模型

| 角色 | 读全局公共 | 读部门公共 | 写公共目录 | 管理公共目录 |
|------|-----------|-----------|-----------|-------------|
| 所有认证用户 | 允许 | 仅匹配部门 | 禁止 | 禁止 |
| API Key | 允许 | 仅匹配部门 | 禁止 | 禁止 |
| Admin | 允许 | 允许 | 允许 | 允许 |

### 权限检查流程

```
请求公共目录资源 → HybridAuth 提取身份
  │
  ├─ Admin JWT → 完全访问（读/写/管理）
  │
  ├─ 用户 JWT / OAuth2 Session
  │    ├─ scope=global → 放行
  │    └─ scope=department → 检查 user.department 是否匹配
  │
  └─ API Key
       ├─ scope=global → 放行
       └─ scope=department → 检查 key.department 是否匹配
```

## SDK 兼容性

Python SDK 对公共目录和个人空间使用完全一致的接口，后端根据认证类型透明切换命名空间：

```python
# JWT 认证 → 个人空间 + 公共目录
client = AgentDiskClient(base_url="...", token=jwt_token)

# API Key 认证 → 仅公共目录
client = AgentDiskClient(base_url="...", api_key="adk_xxx")

# 以下方法在两种认证下均可使用
client.list_folders("/")
client.list_files("/public/shared-docs")
client.get_file("/public/shared-docs/readme.md")
```

SDK 的 `_resolver.py` 模块无需修改，路径解析逻辑对公共目录和个人空间一视同仁。

## 安全约束

- 公共目录只读（仅 Admin 可写），防止未授权修改共享资源
- `public` 和 `department` 为保留路径，用户创建顶级目录时会被拒绝
- API Key 不能访问用户个人目录，避免权限越界
- OAuth2 配置支持动态管理，`client_secret` 脱敏返回
- Admin 密码使用 bcrypt 哈希存储
- 所有凭证通过环境变量注入，禁止硬编码
- 公共目录文件的下载链接使用 OSS 预签名 URL，设置过期时间
