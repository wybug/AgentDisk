# Agent 集成指南

面向需要接入 AgentDisk 云盘的 AI Agent 开发者。本指南覆盖从安装到上线的完整流程，包含所有功能的代码示例。

## 架构概览

```
Agent (Python) ──SDK──> AgentDisk 后端 (9100)
                          ↑
                        JWT (网关签发)
                          ↑
用户浏览器 ──> 网关 (3100) ──proxy──> Agent AI 服务 (8090)
              注册 Agent
              签发 JWT
```

Agent 通过网关签发的 JWT 调用后端 API。JWT 中携带 `userId`、`agentId`、`agentGroupId` 用于身份识别和权限控制。

## 快速开始

### 安装 SDK

在你的 Agent 项目中添加依赖：

```bash
# 使用 uv（推荐）
uv add agentdisk

# 或使用 pip
pip install agentdisk
```

### 初始化客户端

SDK 支持两种认证方式：JWT Token（Agent 身份）和 API Key（公共目录只读）。

```python
from agentdisk import AgentDiskClient

# 方式一：JWT Token（网关签发，Agent 身份）
client = AgentDiskClient(
    base_url="http://localhost:9100",
    token="<gateway-issued-jwt>",
)

# 方式二：API Key（公共目录只读）
client = AgentDiskClient(
    base_url="http://localhost:9100",
    api_key="adk_xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
)

# 使用完毕后关闭
client.close()

# 或使用 context manager 自动管理生命周期
with AgentDiskClient(base_url="http://localhost:9100", token=jwt_token) as client:
    space = client.get_space()
```

## 认证流程

### Agent 注册

Agent 需先通过网关 REST API 注册：

```bash
curl -X POST http://localhost:3100/api/agents \
  -H "Content-Type: application/json" \
  -H "Cookie: gw_session=<session>" \
  -d '{"agentId":"my-agent","agentName":"我的助手","agentGroupId":"team-a"}'
```

| 参数 | 必填 | 说明 |
|------|------|------|
| `agentId` | 是 | Agent 唯一标识 |
| `agentName` | 是 | 显示名称 |
| `agentGroupId` | 否 | Agent 组，同组 Agent 共享文件读写权限 |

### JWT 获取

用户通过网关与 Agent 对话时，网关自动签发包含 Agent 身份的 JWT：

```
网关 /process 端点处理流程:
1. 验证用户已登录（gw_session）
2. 验证 agentId 已注册且属于当前用户
3. 签发 JWT (userId + agentId + agentGroupId)
4. 代理请求到 Agent 服务，携带 Authorization: Bearer <jwt>
```

JWT 结构：

```json
{
  "userId": "user001",
  "agentId": "my-agent",
  "agentGroupId": "team-a",
  "iat": 1747000000,
  "exp": 1747259200
}
```

Agent 服务收到请求后，从 `Authorization: Bearer <jwt>` 中提取 Token，用 SDK 调用后端 API。

## API 使用示例

### 空间查询

查看当前用户的存储空间使用情况：

```python
space = client.get_space()
print(f"已用 {space.used_quota / 1024 / 1024:.1f}MB / 总共 {space.total_quota / 1024 / 1024:.1f}MB")
```

### 文件夹操作

SDK 使用路径式 API，无需手动管理文件夹 ID：

```python
# 创建文件夹
folder = client.create_folder("项目文档")

# 创建子文件夹（自动创建父级）
sub = client.create_folder("项目文档/需求文档")

# 列出文件夹内容
children = client.list_folders("项目文档")

# 获取文件夹信息
info = client.get_folder("项目文档/需求文档")

# 重命名
client.rename_folder("项目文档", "项目文档v2")

# 删除
client.delete_folder("项目文档v2/需求文档")
```

### 文件上传与下载

支持本地文件上传和内存字节上传两种方式：

```python
# 上传本地文件
f = client.upload_file("reports/summary.md", "/path/to/local/summary.md")

# 上传时自动创建目录
f = client.upload_file(
    "reports/2024/annual.md",
    "/path/to/annual.md",
    auto_mkdir=True,
)

# 从内存上传（适用于 Agent 生成的内容）
f = client.upload_bytes(
    "reports/result.txt",
    b"这是 Agent 生成的文件内容",
    folder_id=0,
)

# 从内存上传到指定路径（自动创建目录）
f = client.upload_bytes(
    "output/analysis.json",
    b'{"key": "value"}',
    auto_mkdir=True,
)

# 获取文件详情（含 OSS 预签名 URL）
detail = client.get_file("reports/summary.md")
print(detail.url)  # 可直接访问的预签名 URL

# 列出文件夹中的文件
files = client.list_files("reports")

# 更新文件（自动创建版本快照）
updated = client.update_file("reports/summary.md", "/path/to/summary_v2.md")

# 从内存更新文件内容
updated = client.update_file_bytes(
    "reports/summary.md",
    b"# Updated Report\n\nNew content here.",
)

# 生成下载令牌（可分享给用户，无需登录即可下载）
token = client.create_share(
    "reports/summary.md",
    is_file=True,
    expire_hours=1,
)

# 删除文件（移入回收站，可恢复）
client.delete_file("reports/old-report.md")
```

### 版本管理

文件更新时自动创建版本快照，支持历史查看和回滚：

```python
# 列出文件版本历史
versions = client.list_versions("reports/summary.md")
for v in versions:
    print(f"v{v.version} - {v.file_size} bytes - {v.created_at}")

# 回滚到指定版本
client.rollback_version("reports/summary.md", version=1)
```

### 标签检索

通过标签对文件进行分类和检索：

```python
# 绑定标签
client.bind_tag("reports/summary.md", "重要")
client.bind_tag("reports/summary.md", "合同")

# 按标签搜索（支持多标签）
results = client.search_files(["重要", "合同"])
for f in results:
    print(f"{f.file_name} - {f.file_size} bytes")

# 解绑标签
client.unbind_tag("reports/summary.md", "合同")
```

### 权限控制

精细化控制 Agent 之间的文件访问权限：

```python
# 授予权限（路径式）
client.grant_permission(
    "reports/summary.md",
    agent_id="reviewer-agent",
    permission="read",
    is_file=True,
)

# 授予权限（文件夹级别）
client.grant_permission(
    "reports",
    agent_id="reviewer-agent",
    permission="read",
)

# 检查权限
has_access = client.check_permission(
    "reports/summary.md",
    permission="read",
    is_file=True,
    agent_id="reviewer-agent",
)
print(f"有权限: {has_access}")

# 列出所有权限
permissions = client.list_permissions()

# 撤销权限
client.revoke_permission(
    "reports/summary.md",
    agent_id="reviewer-agent",
    is_file=True,
)
```

### 外链分享

生成可分享的文件链接，支持提取码和访问限制：

```python
# 创建分享链接
share = client.create_share(
    "reports/summary.md",
    is_file=True,
    extract_code="1234",   # 可选提取码
    max_visit=100,         # 最大访问次数，-1 为不限
    expire_hours=24,       # 有效时长
)
print(f"分享码: {share.share_code}")

# 分享文件夹
share = client.create_share(
    "reports",
    is_file=False,
    expire_hours=72,
)

# 通过分享码获取信息（公开，无需认证）
info = client.get_share_by_code(share.share_code)

# 访问分享（需提取码时传入）
accessed = client.access_share(share.share_code, extract_code="1234")

# 列出我的分享
shares = client.list_shares()

# 撤销分享
client.revoke_share(share.id)
```

### 文件预览

获取文件的预览信息，支持 Markdown、代码、文本、图片等格式：

```python
preview = client.preview("reports/summary.md")
print(f"类型: {preview.file_type}")
print(f"预览地址: {preview.url}")
# url 为预签名 URL，可直接在浏览器打开预览
```

### 回收站

已删除的文件进入回收站，支持恢复和永久删除：

```python
# 查看回收站
items = client.list_recycle()
for item in items:
    print(f"{item.res_name} - 删除于 {item.created_at}")

# 恢复文件
client.restore(items[0].id)

# 永久删除（不可恢复）
client.delete_permanent(items[0].id)
```

### 公共目录

浏览系统公共目录和部门公共目录：

```python
# 列出可见的公共目录
dirs = client.list_public_directories()
for d in dirs:
    print(f"{d.name} - {d.scope} - {d.department or '全局'}")

# 获取公共目录详情
detail = client.get_public_directory(dirs[0].id)

# 列出公共目录中的文件
files = client.list_files(f"/public/{dirs[0].name}")
```

## 异步客户端

适用于 async Python（FastAPI、asyncio 等异步框架）：

```python
from agentdisk import AsyncAgentDiskClient

async with AsyncAgentDiskClient(
    base_url="http://localhost:9100",
    token=jwt_token,
) as client:
    # 所有方法前加 await 即可
    space = await client.get_space()
    folder = await client.create_folder("异步上传目录")
    f = await client.upload_bytes(
        "异步上传目录/data.json",
        b'{"key": "value"}',
        auto_mkdir=True,
    )

    # 并发操作
    import asyncio
    results = await asyncio.gather(
        client.list_files("异步上传目录"),
        client.list_recycle(),
        client.list_shares(),
    )
```

### 同步 vs 异步 API 对比

| 特性 | AgentDiskClient | AsyncAgentDiskClient |
|------|----------------|---------------------|
| 导入 | `from agentdisk import AgentDiskClient` | `from agentdisk import AsyncAgentDiskClient` |
| 初始化 | `AgentDiskClient(...)` | `AsyncAgentDiskClient(...)` |
| 调用方式 | `client.method()` | `await client.method()` |
| 上下文管理 | `with ... as client:` | `async with ... as client:` |
| 关闭 | `client.close()` | `await client.close()` |
| 适用框架 | Flask、Django、脚本 | FastAPI、asyncio、aiohttp |

## 错误处理

SDK 将后端错误映射为 Python 异常，形成完整的异常层次结构：

```python
from agentdisk import (
    AgentDiskError,         # 基础异常
    AuthError,              # 401 - Token 无效或过期
    PermissionDeniedError,  # 403 - 无权限
    NotFoundError,          # 404 - 资源不存在
    BadRequestError,        # 400 - 参数错误
    ServerError,            # 500 - 服务内部错误
)

try:
    client.get_file("nonexistent/report.md")
except NotFoundError as e:
    print(f"文件不存在: {e.message}")
except AuthError:
    print("Token 过期，需要重新获取")
except PermissionDeniedError as e:
    print(f"权限不足: {e.message}")
except AgentDiskError as e:
    print(f"错误 [{e.code}]: {e.message}")
```

### 异常属性

| 属性 | 类型 | 说明 |
|------|------|------|
| `e.code` | int | 后端业务错误码 |
| `e.message` | str | 错误描述信息 |
| `e.http_status` | int | HTTP 状态码 |

### 常见错误处理策略

```python
from agentdisk import AgentDiskClient, AuthError, PermissionDeniedError

with AgentDiskClient(base_url="...", token=token) as client:
    try:
        f = client.upload_file("docs/report.pdf", "/local/report.pdf", auto_mkdir=True)
    except AuthError:
        # Token 过期，需要通知网关重新签发
        raise RuntimeError("JWT 过期，请重新触发对话")
    except PermissionDeniedError:
        # 无权限，跳过或请求授权
        print("无上传权限，跳过")
    except Exception as e:
        # 其他错误，记录日志
        print(f"上传失败: {e}")
```

## Agent 自动权限规则

Agent 通过 JWT 中的 `agentId` 和 `agentGroupId` 标识身份，后端自动判断权限：

| 场景 | 权限 | 说明 |
|------|------|------|
| Agent 访问自己创建的文件 | 自动 read/write | `sourceAgent == agentId` |
| Agent 访问同组 Agent 创建的文件 | 自动 read/write | `sourceAgentGroup == agentGroupId` |
| Agent 访问用户手动上传的文件 | 需显式授权 | `isArtifact == false` |
| Agent 请求 delete/owner 权限 | 需显式授权 | 必须在权限表中配置 |
| 跨用户访问 | 始终拒绝 | `userID` 不匹配 |

## 完整 FastAPI 集成示例

以下是一个 Agent 服务集成 AgentDisk 的端到端示例，展示如何在 FastAPI 中接收网关请求并保存产物到云盘：

```python
"""Agent 服务示例：接收网关请求，保存产物到 AgentDisk。"""

import os
from fastapi import FastAPI, Request
from agentdisk import AgentDiskClient

app = FastAPI()
BASE_URL = os.environ.get("AGENTDISK_URL", "http://localhost:9100")


@app.post("/process")
async def process(request: Request):
    # 网关代理请求时自动注入 Authorization header
    auth_header = request.headers.get("Authorization", "")
    token = auth_header.replace("Bearer ", "")

    body = await request.json()
    user_input = body.get("input", [])

    # 使用网关签发的 JWT 初始化 SDK
    with AgentDiskClient(base_url=BASE_URL, token=token) as disk:
        # 1. 确保有工作目录
        folders = disk.list_folders("/")
        work_dir = next((f for f in folders if f.folder_name == "agent-work"), None)
        if not work_dir:
            work_dir = disk.create_folder("agent-work")

        # 2. 保存 Agent 产物
        result_file = disk.upload_bytes(
            "agent-work/result.md",
            b"# Agent Output\n\n处理完成。",
            auto_mkdir=True,
        )

        # 3. 给文件打标签
        disk.bind_tag("agent-work/result.md", "agent-output")

        # 4. 生成分享链接
        share = disk.create_share(
            "agent-work/result.md",
            is_file=True,
            expire_hours=24,
        )

    return {
        "output": f"文件已保存，分享码: {share.share_code}",
    }
```

## 环境变量

| 变量名 | 用途 | 示例 |
|--------|------|------|
| `AGENTDISK_URL` | 后端地址 | `http://localhost:9100` |
| `AGENTDISK_JWT_SECRET` | JWT 密钥（仅用于生成测试 Token） | -- |

## 测试验证

```bash
# 确保后端运行
bash scripts/dev.sh start

# 运行 SDK 测试
cd sdk
uv run pytest -v tests/
```
