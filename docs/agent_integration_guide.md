# Agent 集成 AgentDisk 指南

面向需要接入 AgentDisk 云盘的 AI Agent 开发者。

---

## 1. 架构概览

```
Agent (Python) ──SDK──> AgentDisk 后端 (9100)
                          ↑
                        JWT (网关签发)
                          ↑
用户浏览器 ──> 网关 (3100) ──proxy──> Agent AI 服务 (8090)
              注册 Agent
              签发 JWT
```

Agent 通过网关签发的 JWT 调用后端 API，JWT 中携带 `userId`、`agentId`、`agentGroupId` 用于身份识别和权限控制。

---

## 2. 快速开始

### 2.1 安装 SDK

```bash
cd sdk
uv sync
```

或在你的 Agent 项目中：

```bash
uv add agentdisk
```

### 2.2 初始化客户端

```python
from agentdisk import AgentDiskClient

# token 从网关的 /process 请求中获取（网关自动签发）
client = AgentDiskClient(
    base_url="http://localhost:9100",
    token="<gateway-issued-jwt>",
)

# 使用完毕后关闭
client.close()

# 或使用 context manager
with AgentDiskClient(base_url="http://localhost:9100", token=jwt_token) as client:
    space = client.space.get()
```

---

## 3. 认证流程

### 3.1 Agent 注册

Agent 需先通过网关 REST API 注册：

```bash
curl -X POST http://localhost:3100/api/agents \
  -H "Content-Type: application/json" \
  -H "Cookie: gw_session=<session>" \
  -d '{"agentId":"my-agent","agentName":"我的助手","agentGroupId":"team-a"}'
```

- `agentId`: Agent 唯一标识
- `agentName`: 显示名称
- `agentGroupId`: 可选，同组 Agent 共享文件读写权限

### 3.2 JWT 获取

用户通过网关与 Agent 对话时，网关自动签发包含 Agent 身份的 JWT：

```python
# 网关 /process 端点会：
# 1. 验证用户已登录
# 2. 验证 agentId 已注册且属于当前用户
# 3. 签发 JWT (userId + agentId + agentGroupId)
# 4. 代理请求到 Agent 服务
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

Agent 服务收到请求后，从 `Authorization: Bearer <jwt>` 中提取 token，用 SDK 调用后端 API。

---

## 4. API 使用示例

### 4.1 空间查询

```python
space = client.space.get()
print(f"已用 {space.used_quota / 1024 / 1024:.1f}MB / 总共 {space.total_quota / 1024 / 1024:.1f}MB")
```

### 4.2 文件夹操作

```python
# 创建文件夹
folder = client.folders.create("项目文档", parent_id=0)

# 创建子文件夹
sub = client.folders.create("需求文档", parent_id=folder.id)

# 列出文件夹内容
children = client.folders.list(folder.id)

# 获取文件夹路径
path = client.folders.ancestors(sub.id)
# [根目录, 项目文档]

# 重命名
client.folders.rename(folder.id, "项目文档v2")

# 删除
client.folders.delete(sub.id)
```

### 4.3 文件上传与下载

```python
# 上传文件（路径）
f = client.files.upload("/path/to/report.pdf", folder_id=folder.id)

# 上传文件（内存字节）
f = client.files.upload_bytes(
    "summary.txt",
    b"这是文件内容",
    folder_id=folder.id,
)

# 获取文件信息（含预签名 URL）
detail = client.files.get(f.id)
print(detail.url)  # 可直接访问的 OSS 预签名 URL

# 列出文件夹中的文件
files = client.files.list(folder.id)

# 更新文件（自动创建版本快照）
updated = client.files.update(f.id, "/path/to/report_v2.pdf")

# 生成下载令牌（可分享给用户）
token = client.files.create_download_token(f.id)
download_url = f"http://your-host:9100/v1/disk/files/download?t={token.download_token}"

# 通过令牌下载
dl = client.files.download_by_token(token.download_token)
print(dl.download_url)

# 删除文件（移入回收站）
client.files.delete(f.id)
```

### 4.4 版本管理

```python
# 列出文件版本历史
versions = client.versions.list(f.id)
for v in versions:
    print(f"v{v.version} - {v.file_size} bytes - {v.created_at}")

# 回滚到指定版本
client.versions.rollback(f.id, version=1)
```

### 4.5 标签检索

```python
# 绑定标签
client.tags.bind(f.id, "重要")
client.tags.bind(f.id, "合同")

# 按标签搜索
results = client.tags.search(["重要", "合同"])

# 解绑标签
client.tags.unbind(f.id, "合同")
```

### 4.6 权限控制

```python
# 给其他 Agent 授权
client.permissions.grant(
    agent_id="reviewer-agent",
    resource_id=f.id,
    res_type="file",
    permission="read",
)

# 检查权限
has_access = client.permissions.check(
    resource_id=f.id,
    res_type="file",
    permission="read",
    agent_id="reviewer-agent",
)

# 回收权限
client.permissions.revoke(
    agent_id="reviewer-agent",
    resource_id=f.id,
    res_type="file",
)
```

### 4.7 外链分享

```python
# 创建分享链接
share = client.shares.create(
    resource_id=f.id,
    res_type="file",
    extract_code="1234",   # 可选提取码
    max_visit=100,         # 最大访问次数，-1 为不限
    expire_hours=24,       # 有效时长
)
print(f"分享码: {share.share_code}")

# 通过分享码获取信息（公开，无需认证）
info = client.shares.get_by_code(share.share_code)

# 访问分享（公开）
accessed = client.shares.access(share.share_code, extract_code="1234")

# 撤销分享
client.shares.revoke(share.id)
```

### 4.8 文件预览

```python
preview = client.preview.file(f.id)
print(f"类型: {preview.file_type}")
# url 为预签名 URL，可直接在浏览器打开预览
print(f"预览地址: {preview.url}")
```

### 4.9 回收站

```python
# 查看回收站
items = client.recycle.list()
for item in items:
    print(f"{item.res_name} - 删除于 {item.created_at}")

# 恢复
client.recycle.restore(items[0].id)

# 永久删除
client.recycle.delete_permanent(items[0].id)
```

---

## 5. 异步客户端

适用于 async Python（FastAPI、asyncio 等）：

```python
from agentdisk import AsyncAgentDiskClient

async with AsyncAgentDiskClient(
    base_url="http://localhost:9100",
    token=jwt_token,
) as client:
    space = await client.space.get()
    folder = await client.folders.create("异步上传目录")
    f = await client.files.upload_bytes("data.json", b'{"key": "value"}', folder_id=folder.id)
```

---

## 6. 错误处理

SDK 将后端错误映射为 Python 异常：

```python
from agentdisk import (
    AgentDiskError,
    AuthError,           # 401 - Token 无效或过期
    PermissionDeniedError, # 403 - 无权限
    NotFoundError,       # 404 - 资源不存在
    BadRequestError,     # 400 - 参数错误
    ServerError,         # 500 - 服务内部错误
)

try:
    client.files.get(99999)
except NotFoundError as e:
    print(f"文件不存在: {e.message}")
except AuthError:
    print("Token 过期，需要重新获取")
except AgentDiskError as e:
    print(f"错误 [{e.code}]: {e.message}")
```

---

## 7. Agent 自动权限规则

Agent 通过 JWT 中的 `agentId` 和 `agentGroupId` 标识身份，后端自动判断权限：

| 场景 | 权限 | 说明 |
|------|------|------|
| Agent 访问自己创建的文件 | 自动 read/write | `sourceAgent == agentId` |
| Agent 访问同组 Agent 创建的文件 | 自动 read/write | `sourceAgentGroup == agentGroupId` |
| Agent 访问用户手动上传的文件 | 需显式授权 | `isArtifact == false` |
| Agent 请求 delete/owner 权限 | 需显式授权 | 必须在权限表中配置 |
| 跨用户访问 | 始终拒绝 | `userID` 不匹配 |

---

## 8. 完整集成示例

以下是一个 Agent 服务集成 AgentDisk 的端到端示例：

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
        folders = disk.folders.list(0)
        work_dir = next((f for f in folders if f.folder_name == "agent-work"), None)
        if not work_dir:
            work_dir = disk.folders.create("agent-work")

        # 2. 保存 Agent 产物
        result_file = disk.files.upload_bytes(
            "result.md",
            b"# Agent Output\n\n处理完成。",
            folder_id=work_dir.id,
        )

        # 3. 生成分享链接
        share = disk.shares.create(
            resource_id=result_file.id,
            res_type="file",
            expire_hours=24,
        )

    return {
        "output": f"文件已保存，分享码: {share.share_code}",
    }
```

---

## 9. 环境变量

| 变量名 | 用途 | 示例 |
|--------|------|------|
| `AGENTDISK_URL` | 后端地址 | `http://localhost:9100` |
| `AGENTDISK_JWT_SECRET` | JWT 密钥（仅用于生成测试 token） | — |

---

## 10. 测试

```bash
# 确保后端运行
bash scripts/dev.sh start

# 运行 SDK 测试
cd sdk
uv run pytest -v tests/
```
