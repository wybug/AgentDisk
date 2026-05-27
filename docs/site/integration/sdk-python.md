# Python SDK

AgentDisk Python SDK 提供路径式 API，支持同步和异步两种客户端。SDK 封装了所有 AgentDisk 后端 API，使用 httpx 作为 HTTP 客户端，自动处理认证、缓存和错误转换。

## 安装

```bash
# 使用 uv（推荐）
uv add agentdisk

# 使用 pip
pip install agentdisk

# 从源码安装（开发环境）
cd sdk
uv sync
```

**依赖要求**：Python 3.9+

## 同步客户端：AgentDiskClient

### 初始化

```python
from agentdisk import AgentDiskClient

# JWT Token 认证（Agent 身份，完整功能）
client = AgentDiskClient(
    base_url="http://localhost:9100",
    token="<jwt-from-gateway>",
)

# API Key 认证（公共目录只读）
client = AgentDiskClient(
    base_url="http://localhost:9100",
    api_key="adk_xxxx...xxxx",
)

# 自定义超时时间
client = AgentDiskClient(
    base_url="http://localhost:9100",
    token="<jwt>",
    timeout=60.0,       # HTTP 请求超时（秒），默认 30
    cache_ttl=120.0,    # 路径缓存有效期（秒），默认 60
)
```

> **注意**：`token` 和 `api_key` 至少提供一个，否则初始化时会抛出 `ValueError`。

### Context Manager 模式

推荐使用 `with` 语句自动管理客户端生命周期：

```python
with AgentDiskClient(base_url="http://localhost:9100", token=jwt_token) as client:
    space = client.get_space()
    # 退出 with 块时自动关闭 HTTP 连接
```

手动管理方式：

```python
client = AgentDiskClient(base_url="http://localhost:9100", token=jwt_token)
try:
    space = client.get_space()
finally:
    client.close()
```

### 空间操作

```python
# 查询存储空间
space = client.get_space()
print(f"已用: {space.used_quota / 1024 / 1024:.1f} MB")
print(f"总额: {space.total_quota / 1024 / 1024:.1f} MB")
```

### 文件夹操作

SDK 使用路径式 API，自动解析路径到文件夹 ID：

```python
# 创建文件夹（路径不存在会报错，除非 exist_ok=True）
folder = client.create_folder("docs/reports")
folder = client.create_folder("docs/reports", exist_ok=True)

# 列出文件夹内容
subfolders = client.list_folders("docs")
subfolders = client.list_folders("/")  # 根目录

# 获取文件夹信息
folder = client.get_folder("docs/reports")

# 重命名文件夹
client.rename_folder("docs/reports", "reports-2024")

# 删除文件夹（移入回收站）
client.delete_folder("docs/old-folder")
```

### 文件操作

```python
# 上传本地文件
f = client.upload_file("docs/report.pdf", "/local/path/report.pdf")

# 上传并自动创建目录
f = client.upload_file(
    "docs/2024/annual/report.pdf",
    "/local/report.pdf",
    auto_mkdir=True,
)

# 从内存上传字节
f = client.upload_bytes(
    "docs/summary.txt",
    b"文件内容",
    auto_mkdir=True,
)

# 从内存上传并指定 Content-Type
f = client.upload_bytes(
    "docs/data.json",
    b'{"key": "value"}',
    content_type="application/json",
    auto_mkdir=True,
)

# 获取文件详情（含预签名下载 URL）
detail = client.get_file("docs/report.pdf")
print(detail.url)

# 列出文件夹中的文件
files = client.list_files("docs")

# 更新文件（自动创建版本快照）
f = client.update_file("docs/report.pdf", "/local/report_v2.pdf")

# 从内存更新文件
f = client.update_file_bytes(
    "docs/report.pdf",
    b"updated content",
    content_type="text/plain",
)

# 删除文件（移入回收站）
client.delete_file("docs/old-file.txt")
```

### 版本操作

```python
# 列出文件版本历史
versions = client.list_versions("docs/report.pdf")
for v in versions:
    print(f"v{v.version} | {v.file_size} bytes | {v.created_at}")

# 回滚到指定版本
client.rollback_version("docs/report.pdf", version=1)
```

### 标签操作

```python
# 绑定标签
client.bind_tag("docs/report.pdf", "重要")
client.bind_tag("docs/report.pdf", "合同")

# 按标签搜索文件
results = client.search_files(["重要", "合同"])

# 解绑标签
client.unbind_tag("docs/report.pdf", "合同")
```

### 权限操作

```python
# 授予权限（文件）
client.grant_permission(
    "docs/report.pdf",
    agent_id="reviewer-agent",
    permission="read",
    is_file=True,
)

# 授予权限（文件夹）
client.grant_permission(
    "docs",
    agent_id="reviewer-agent",
    permission="write",
)

# 检查权限
has_perm = client.check_permission(
    "docs/report.pdf",
    permission="read",
    is_file=True,
    agent_id="reviewer-agent",
)

# 列出所有权限
perms = client.list_permissions()

# 撤销权限
client.revoke_permission(
    "docs/report.pdf",
    agent_id="reviewer-agent",
    is_file=True,
)
```

### 分享操作

```python
# 创建文件分享链接
share = client.create_share(
    "docs/report.pdf",
    is_file=True,
    extract_code="1234",   # 可选提取码
    max_visit=100,         # 最大访问次数，-1 不限
    expire_hours=24,       # 有效时长
)
print(f"分享码: {share.share_code}")

# 创建文件夹分享
share = client.create_share("docs", is_file=False, expire_hours=72)

# 通过分享码获取信息（无需认证）
info = client.get_share_by_code(share.share_code)

# 访问分享
accessed = client.access_share(share.share_code, extract_code="1234")

# 列出我的分享
shares = client.list_shares()

# 撤销分享
client.revoke_share(share.id)
```

### 预览操作

```python
# 获取文件预览信息
preview = client.preview("docs/report.pdf")
print(f"文件类型: {preview.file_type}")
print(f"预览 URL: {preview.url}")
```

### 回收站操作

```python
# 查看回收站
items = client.list_recycle()

# 恢复文件
client.restore(items[0].id)

# 永久删除
client.delete_permanent(items[0].id)
```

### 公共目录操作

```python
# 列出可见的公共目录
dirs = client.list_public_directories()

# 获取公共目录详情
detail = client.get_public_directory(dirs[0].id)
```

### 缓存管理

SDK 内部维护路径到 ID 的缓存，加快重复路径解析速度：

```python
# 使指定路径缓存失效
client.invalidate_cache("docs/reports")

# 清除所有缓存
client.clear_cache()
```

## 异步客户端：AsyncAgentDiskClient

异步客户端提供与同步客户端完全一致的 API，所有方法均为 `async`。

### 初始化

```python
from agentdisk import AsyncAgentDiskClient

# 使用 async with 自动管理生命周期
async with AsyncAgentDiskClient(
    base_url="http://localhost:9100",
    token=jwt_token,
) as client:
    space = await client.get_space()
```

### 完整异步示例

```python
import asyncio
from agentdisk import AsyncAgentDiskClient

async def main():
    async with AsyncAgentDiskClient(
        base_url="http://localhost:9100",
        token=jwt_token,
    ) as client:
        # 创建目录
        await client.create_folder("async-demo", exist_ok=True)

        # 并发上传多个文件
        import asyncio
        await asyncio.gather(
            client.upload_bytes("async-demo/file1.txt", b"content 1"),
            client.upload_bytes("async-demo/file2.txt", b"content 2"),
            client.upload_bytes("async-demo/file3.txt", b"content 3"),
        )

        # 列出文件
        files = await client.list_files("async-demo")
        for f in files:
            print(f.file_name)

asyncio.run(main())
```

### 异步 API 速查

所有方法与同步客户端一致，仅需加 `await`：

| 同步 | 异步 |
|------|------|
| `client.get_space()` | `await client.get_space()` |
| `client.create_folder(path)` | `await client.create_folder(path)` |
| `client.list_folders(path)` | `await client.list_folders(path)` |
| `client.upload_file(path, local)` | `await client.upload_file(path, local)` |
| `client.upload_bytes(path, data)` | `await client.upload_bytes(path, data)` |
| `client.get_file(path)` | `await client.get_file(path)` |
| `client.update_file(path, local)` | `await client.update_file(path, local)` |
| `client.delete_file(path)` | `await client.delete_file(path)` |
| `client.create_share(path)` | `await client.create_share(path)` |
| `client.bind_tag(path, tag)` | `await client.bind_tag(path, tag)` |
| `client.grant_permission(...)` | `await client.grant_permission(...)` |
| `client.preview(path)` | `await client.preview(path)` |
| `client.list_versions(path)` | `await client.list_versions(path)` |
| `client.list_recycle()` | `await client.list_recycle()` |

## 错误处理

SDK 将后端 HTTP 错误映射为 Python 异常层次：

```
AgentDiskError               # 基础异常 (code, message, http_status)
  ├── AuthError              # 401 Unauthorized
  ├── PermissionDeniedError  # 403 Forbidden
  ├── NotFoundError          # 404 Not Found
  ├── BadRequestError        # 400 Bad Request
  └── ServerError            # 500 Internal Server Error
```

### 异常属性

| 属性 | 类型 | 说明 |
|------|------|------|
| `code` | int | 后端业务错误码 |
| `message` | str | 错误描述 |
| `http_status` | int | HTTP 状态码 |

### 使用示例

```python
from agentdisk import (
    AgentDiskClient,
    AgentDiskError,
    AuthError,
    PermissionDeniedError,
    NotFoundError,
    BadRequestError,
    ServerError,
)

with AgentDiskClient(base_url="...", token=jwt) as client:
    try:
        f = client.get_file("docs/report.pdf")
    except NotFoundError:
        print("文件不存在")
    except AuthError:
        print("Token 过期")
    except PermissionDeniedError as e:
        print(f"无权限: {e.message}")
    except BadRequestError as e:
        print(f"参数错误: {e.message}")
    except ServerError as e:
        print(f"服务错误: {e.message}")
    except AgentDiskError as e:
        print(f"未知错误 [{e.code}]: {e.message}")
```

## 配置选项

### ClientConfig

```python
from agentdisk import ClientConfig

config = ClientConfig(
    base_url="http://localhost:9100",
    token="<jwt>",
    timeout=30.0,
)
```

| 参数 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `base_url` | str | 必填 | AgentDisk 后端地址 |
| `token` | str | "" | JWT Token（与 api_key 二选一） |
| `api_key` | str | "" | API Key（与 token 二选一） |
| `timeout` | float | 30.0 | HTTP 请求超时（秒） |
| `cache_ttl` | float | 60.0 | 路径缓存有效期（秒） |

### 环境变量

| 变量名 | 用途 | 示例 |
|--------|------|------|
| `AGENTDISK_URL` | 后端地址 | `http://localhost:9100` |
| `AGENTDISK_JWT_SECRET` | JWT 密钥（仅用于测试） | -- |
