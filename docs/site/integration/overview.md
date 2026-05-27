# 集成概览

AgentDisk 提供三种集成方式，覆盖不同使用场景。无论是 Web 前端应用、AI Agent 后端服务，还是脚本自动化，都能快速接入。

## 架构总览

```
                        AgentDisk 系统集成架构

  ┌──────────────┐      ┌──────────────┐      ┌──────────────────┐
  │  Web 浏览器   │─────>│   Gateway    │─────>│  Agent AI 服务    │
  │  (用户界面)   │<─────│  (3100)      │<─────│  (8090)          │
  └──────────────┘      └──────┬───────┘      └────────┬─────────┘
         │                     │                        │
         │  OAuth2 Session     │ JWT 签发               │ JWT Bearer
         │                     │                        │
         │              ┌──────▼───────┐      ┌────────▼─────────┐
         │              │  React 前端   │      │  Python SDK      │
         │              │  (9101)      │      │  (agentdisk)     │
         │              └──────┬───────┘      └────────┬─────────┘
         │                     │                        │
         └─────────────────────┼────────────────────────┘
                               │
                    ┌──────────▼──────────┐
                    │  AgentDisk Backend   │
                    │  (Go + Gin, :9100)   │
                    └──────────┬──────────┘
                               │
              ┌────────────────┼────────────────┐
              │                │                │
     ┌────────▼──────┐ ┌──────▼──────┐ ┌───────▼──────┐
     │  MySQL/SQLite  │ │  MinIO/OSS  │ │    Redis      │
     │  (元数据)      │ │ (文件存储)   │ │ (缓存/会话)   │
     └───────────────┘ └─────────────┘ └──────────────┘
```

## 三种集成路径

### 1. Web UI（React 前端）

适用于需要图形化文件管理界面的场景。AgentDisk 提供基于 React + Ant Design 的完整前端应用，通过 OAuth2 会话认证访问后端 API。

- **目标用户**：终端用户、管理员
- **认证方式**：OAuth2 Session Cookie（`agentdisk_session`）
- **端口**：前端 `:9101`，后端 `:9100`
- **功能覆盖**：完整的文件管理、公共目录浏览、管理后台

### 2. Python SDK（agentdisk）

适用于 AI Agent 后端服务集成。提供同步和异步两种客户端，路径式 API 设计，开箱即用。

- **目标用户**：Agent 开发者、后端工程师
- **认证方式**：JWT Bearer Token（网关签发）或 API Key
- **安装方式**：`uv add agentdisk` 或 `pip install agentdisk`
- **功能覆盖**：全部文件管理 API，含版本、标签、权限、分享、预览

### 3. REST API（直接 HTTP 调用）

适用于非 Python 环境（Go、Java、Node.js 等）或脚本自动化场景。所有接口遵循 OpenAI 风格的 RESTful 规范。

- **目标用户**：全栈开发者、运维工程师
- **认证方式**：JWT Bearer、API Key、下载令牌
- **接口风格**：RESTful，统一响应格式 `{"code": int, "message": string, "data": any}`
- **功能覆盖**：与 SDK 完全一致

## 集成方式对比

| 特性 | Web UI | Python SDK | REST API |
|------|--------|-----------|----------|
| **适用场景** | 用户文件管理、管理后台 | Agent 产物存储、自动化 | 非 Python 环境、脚本 |
| **认证方式** | OAuth2 Session | JWT / API Key | JWT / API Key / 下载令牌 |
| **安装成本** | 无（浏览器直接访问） | `uv add agentdisk` | 无依赖 |
| **学习曲线** | 低（图形界面） | 低（链式 API） | 中（需阅读 API 文档） |
| **语言要求** | 无 | Python 3.9+ | 任意（HTTP 客户端） |
| **异步支持** | N/A | 支持（AsyncAgentDiskClient） | 取决于 HTTP 客户端 |
| **路径式 API** | N/A | 支持 | 不支持（基于 ID） |
| **错误处理** | 图形化提示 | Python 异常体系 | HTTP 状态码 + 错误响应体 |

## 如何选择

### 你是 Agent 开发者

推荐使用 **Python SDK**。SDK 提供路径式 API，无需手动管理文件夹 ID，自动缓存解析结果，支持同步和异步两种模式。配合网关签发的 JWT，几行代码即可完成文件存取。

```python
from agentdisk import AgentDiskClient

with AgentDiskClient(base_url="http://localhost:9100", token=jwt_token) as client:
    client.upload_bytes("reports/summary.md", b"# Report\nDone!", auto_mkdir=True)
```

### 你是前端开发者

直接使用 **Web UI**。前端已内置完整的文件管理功能，支持拖拽上传、目录树导航、外链分享等。如需自定义集成，可通过 OAuth2 获取会话后直接调用 REST API。

### 你是非 Python 后端开发者

使用 **REST API**。所有接口使用标准 HTTP 方法，统一 JSON 响应格式，任何支持 HTTP 的语言均可接入。通过 JWT 或 API Key 认证后，按 OpenAPI 文档调用即可。

```bash
# 获取空间信息
curl http://localhost:9100/v1/disk/space \
  -H "Authorization: Bearer <jwt>"
```

## 集成流程总览

```
1. 确定集成方式（Web UI / SDK / REST API）
         │
2. 选择认证方式
         ├── JWT Bearer（内部服务 / Agent）
         ├── OAuth2 Session（Web 用户）
         ├── API Key（脚本 / 外部系统）
         └── 下载令牌（文件下载）
         │
3. 配置环境
         ├── 部署 AgentDisk 后端（:9100）
         ├── 部署 Gateway（:3100，可选）
         └── 部署前端（:9101，可选）
         │
4. 开始集成
         ├── 阅读「认证集成」了解认证细节
         ├── 阅读「Agent 集成」了解完整 Agent 接入流程
         ├── 阅读「Python SDK」了解 SDK 使用方法
         └── 阅读「API Key 认证」了解 API Key 管理
```

## 下一步

- [认证集成](/integration/auth) -- 了解 JWT、OAuth2、API Key 等认证方式的详细配置
- [Agent 集成](/integration/agent) -- 完整的 Agent 接入指南，含代码示例
- [Python SDK](/integration/sdk-python) -- SDK 安装、初始化和全部 API 用法
- [API Key 认证](/integration/api-keys) -- API Key 创建、使用和安全最佳实践
