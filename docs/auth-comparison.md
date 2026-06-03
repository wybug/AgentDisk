# SDK 认证方式对比

AgentDisk SDK 提供两种认证方式，对应不同的客户端和使用场景。

## 客户端类型

| 客户端 | 认证方式 | 用途 |
|--------|----------|------|
| `AgentDiskClient(token=...)` | JWT Token | 私有资源完整管理 + 公共目录只读 |
| `AgentDiskClient(api_key=...)` | API Key | 公共目录文件 CRUD（无私有资源访问） |
| `AgentDiskAdminClient(api_key=...)` | API Key | 公共目录用户授权管理 |

## 路径访问

公共目录通过 `display_name` 直接作为路径段访问，无需固定前缀：

```python
from agentdisk import AgentDiskClient, AgentDiskAdminClient

# --- JWT Token 客户端 ---
client = AgentDiskClient(base_url="http://localhost:9100", token="<jwt>")

# 私有资源
client.list_files("/")
client.upload_file("docs/report.txt", "/local/report.txt")
client.create_folder("projects")

# 公共目录（display_name 即路径，需被授权）
client.list_files("shared-project")                   # 列表
client.download_file("shared-project/data.csv")       # 下载
client.create_share("shared-project/data.csv", is_file=True)  # 分享
client.list_folders("shared-project/subfolder")       # 浏览子目录

# --- API Key 客户端 ---
api_client = AgentDiskClient(base_url="http://localhost:9100", api_key="<key>")

# 公共目录文件管理
api_client.upload_bytes("shared-project/new.csv", b"data", content_type="text/csv")
api_client.create_folder("shared-project/2024")
api_client.delete_file("shared-project/old.csv")
api_client.list_files("shared-project")

# --- 管理客户端 ---
admin = AgentDiskAdminClient(base_url="http://localhost:9100", api_key="<key>")

# 授权管理
admin.grant_access(public_dir_id=1, user_id="user-123")
admin.list_granted_users(public_dir_id=1)
admin.revoke_access(public_dir_id=1, user_id="user-123")
```

## 权限矩阵

### 按操作

| 操作 | JWT Token | API Key |
|------|-----------|---------|
| 私有文件/目录 CRUD | ✅ 完整 | ❌ 被拒绝 |
| 私有分享/权限/标签/版本 | ✅ 完整 | ❌ 被拒绝 |
| 公共目录列表/浏览 | ✅（需授权） | ✅ |
| 公共目录文件下载 | ✅（需授权） | ✅ |
| 公共目录文件分享 | ✅（需授权） | ❌ |
| 公共目录文件上传 | ❌ | ✅ |
| 公共目录创建子目录 | ❌ | ✅ |
| 公共目录删除文件 | ❌ | ✅ |
| 公共目录用户授权管理 | ❌ | ✅ |

### 按后端端点

| 端点 | JWT Token | API Key |
|------|-----------|---------|
| `/files/*`, `/folders/*` | ✅ | ❌ |
| `/permissions/*`, `/versions/*` | ✅ | ❌ |
| `/recycle/*`, `/tags/*` | ✅ | ❌ |
| `/shares/*` | ✅ | ❌ |
| `/space` | ✅ | ❌ |
| `/preview/*` | ✅ | ❌ |
| `/public-directories` (GET 浏览/列表) | ✅（需授权） | ✅ |
| `/public-directories/:id/files` (GET) | ✅（需授权） | ✅ |
| `/public-directories/:id/download-token` | ✅（需授权） | ✅ |
| `/public-directories/:id/files/upload` | ❌ | ✅ |
| `/public-directories/:id/folders` (POST) | ❌ | ✅ |
| `/public-directories/:id/files/:fileId` (DELETE) | ❌ | ✅ |
| `/public-directories/:id/grants` | ❌ | ✅ |

## 授权流程

```
管理员创建公共目录 ─→ 管理员创建 API Key ─→ API Key 管理目录文件
                         │
                         └──→ 管理员授权用户 ─→ JWT 用户只读访问
                                                  │
                                                  ├── 列表/下载/分享 ✅
                                                  └── 上传/删除/创建目录 ❌
```

## API Key 创建与管理

通过管理员后台创建 API Key：

```bash
# 管理员登录获取 Token
curl -X POST http://localhost:9100/v1/disk/admin/login \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"admin123"}'

# 创建 API Key
curl -X POST http://localhost:9100/v1/disk/admin/api-keys \
  -H "Authorization: Bearer <admin-token>" \
  -H "Content-Type: application/json" \
  -d '{"name":"my-api-key"}'

# 创建公共目录
curl -X POST http://localhost:9100/v1/disk/admin/public-directories \
  -H "Authorization: Bearer <admin-token>" \
  -H "Content-Type: application/json" \
  -d '{"displayName":"shared-project","scope":"global"}'
```

SDK 发送 API Key 时使用 `X-API-Key` 请求头。
