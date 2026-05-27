# 架构概览

AgentDisk 是专为多智能体系统设计的企业级云盘中间件。本文档介绍系统的整体架构、核心组件、数据流和技术栈选择。

## 系统架构

```
┌─────────────────────────────────────────────────────────────────────┐
│                           用户层                                     │
│                                                                     │
│  ┌──────────────┐    ┌──────────────┐    ┌──────────────────────┐   │
│  │  Web 浏览器   │    │  Agent 服务   │    │  外部系统 / 脚本      │   │
│  │  (用户界面)   │    │  (Python)    │    │  (REST API)          │   │
│  └──────┬───────┘    └──────┬───────┘    └──────────┬───────────┘   │
│         │                   │                       │               │
└─────────┼───────────────────┼───────────────────────┼───────────────┘
          │                   │                       │
          │ OAuth2            │ JWT Bearer            │ API Key
          │                   │                       │
┌─────────┼───────────────────┼───────────────────────┼───────────────┐
│         │          接入层    │                       │               │
│         │                   │                       │               │
│  ┌──────▼───────┐    ┌──────▼───────┐              │               │
│  │  React 前端   │    │   Gateway    │              │               │
│  │  (Ant Design) │    │  (Node.js)  │              │               │
│  │  :9101       │    │  :3100      │              │               │
│  └──────┬───────┘    └──────┬───────┘              │               │
│         │                   │                      │               │
└─────────┼───────────────────┼──────────────────────┼───────────────┘
          │                   │                      │
          │                   │                      │
┌─────────┼───────────────────┼──────────────────────┼───────────────┐
│         │          服务层    │                      │               │
│         │                   │                      │               │
│  ┌──────▼───────────────────▼──────────────────────▼───────────┐   │
│  │                  AgentDisk Backend                           │   │
│  │                  (Go + Gin, :9100)                           │   │
│  │                                                              │   │
│  │  ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────────┐   │   │
│  │  │ HybridAuth│ │ 文件服务  │ │ 权限服务  │ │ 分享/预览服务 │   │   │
│  │  │  中间件    │ │          │ │          │ │              │   │   │
│  │  └──────────┘ └──────────┘ └──────────┘ └──────────────┘   │   │
│  │  ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────────┐   │   │
│  │  │ 版本服务  │ │ 标签服务  │ │ 回收站   │ │ Admin 服务   │   │   │
│  │  └──────────┘ └──────────┘ └──────────┘ └──────────────┘   │   │
│  └──────────────────────┬──────────────────────────────────────┘   │
│                         │                                          │
└─────────────────────────┼──────────────────────────────────────────┘
                          │
┌─────────────────────────┼──────────────────────────────────────────┐
│                 存储层  │                                          │
│                         │                                          │
│  ┌──────────────────────┼────────────────────────────────────┐     │
│  │                      │                                    │     │
│  │  ┌───────────┐ ┌─────▼─────┐ ┌───────────┐ ┌──────────┐ │     │
│  │  │ MySQL /   │ │ MinIO /   │ │  Redis     │ │ SQLite   │ │     │
│  │  │ SQLite    │ │ OSS       │ │ (缓存/会话)│ │ (网关)   │ │     │
│  │  │ (元数据)  │ │ (文件存储) │ │           │ │          │ │     │
│  │  └───────────┘ └───────────┘ └───────────┘ └──────────┘ │     │
│  │                                                          │     │
│  └──────────────────────────────────────────────────────────┘     │
│                                                                    │
└────────────────────────────────────────────────────────────────────┘
```

## 核心组件

### Backend（Go + Gin）

后端服务是 AgentDisk 的核心，提供所有业务 API。

| 特性 | 说明 |
|------|------|
| 语言 | Go |
| 框架 | Gin |
| 端口 | 9100 |
| API 风格 | RESTful，OpenAI 风格响应格式 |
| 认证中间件 | HybridAuth（JWT / OAuth2 / API Key / 下载令牌） |
| 存储 | MySQL 8.0 或 SQLite（元数据）、MinIO/OSS（文件）、Redis（缓存） |

核心模块：

- **HybridAuth 中间件**：四合一认证，自动识别请求凭据类型
- **文件服务**：上传、下载、更新、删除，自动版本快照
- **目录服务**：目录树管理，路径解析，祖级查询
- **权限服务**：RBAC 模型，Agent 自动权限，路径授权（Glob 通配符）
- **分享服务**：外链生成，提取码，访问限制，过期管控
- **预览服务**：Markdown、代码、文本、图片沙箱渲染
- **版本服务**：自动快照，版本列表，一键回滚
- **标签服务**：绑定、解绑、多条件筛选
- **回收站服务**：逻辑删除，恢复，永久销毁
- **Admin 服务**：管理员认证，API Key 管理，公共目录管理，OAuth2 配置

### Gateway（Node.js）

网关负责用户认证、Agent 注册和请求代理。

| 特性 | 说明 |
|------|------|
| 语言 | TypeScript / Node.js |
| 端口 | 3100 |
| 功能 | 用户登录、OAuth2 Provider、Agent 注册、JWT 签发、请求代理 |

核心功能：

- **用户认证**：OAuth2 授权端点，支持 `prompt=none` 无感跳转
- **Agent 注册**：`POST/GET/DELETE /api/agents`，Agent 信息持久化到本地 SQLite
- **JWT 签发**：验证用户登录状态后签发 JWT（userId + agentId + agentGroupId）
- **请求代理**：将用户对话请求代理到 Agent AI 服务，携带签发的 JWT

### Frontend（React + Ant Design）

前端提供图形化的文件管理界面。

| 特性 | 说明 |
|------|------|
| 框架 | React |
| UI 库 | Ant Design |
| 端口 | 9101 |
| 认证 | OAuth2 Session Cookie |

功能模块：

- **文件管理器**：目录树导航，拖拽上传，文件预览
- **公共目录**：浏览系统公共文件和部门文件
- **管理后台**：OAuth2 配置，API Key 管理，公共目录管理
- **分享管理**：创建/查看/撤销外链分享
- **权限管理**：资源授权，路径授权
- **回收站**：查看、恢复、永久删除

### Python SDK

提供同步和异步两种客户端，路径式 API 设计。

| 特性 | 说明 |
|------|------|
| 包名 | agentdisk |
| Python 版本 | 3.9+ |
| HTTP 客户端 | httpx |
| 客户端 | AgentDiskClient（同步）、AsyncAgentDiskClient（异步） |

特性：

- **路径式 API**：`client.upload_file("docs/report.pdf", "/local/file")`，无需管理 ID
- **自动缓存**：路径到 ID 的映射缓存，可配置 TTL
- **双模式**：同步 + 异步，覆盖所有主流 Python 框架
- **异常体系**：HTTP 错误自动映射为 Python 异常
- **双认证**：支持 JWT Token 和 API Key

## 数据流

### 用户文件操作

```
用户浏览器 → React 前端 → AgentDisk Backend API
                            │
                            ├→ MySQL/SQLite: 保存文件元数据
                            ├→ MinIO/OSS:    存储文件内容
                            └→ Redis:        缓存会话信息
```

### Agent 文件操作

```
用户对话 → Gateway → Agent AI 服务 → AgentDisk Backend API
          │             │                │
          ├→ 验证用户    ├→ 使用 JWT       ├→ 验证 JWT
          ├→ 签发 JWT    └→ 调用 SDK       ├→ 检查权限
          └→ 代理请求                      ├→ 操作元数据
                                           └→ 操作文件存储
```

### 下载流程

```
Agent/用户 → POST /files/:id/download-token → 返回临时令牌
                                                     │
用户浏览器 → GET /files/download?t=<token> → 验证令牌 → 生成 OSS 预签名 URL → 重定向下载
```

## 数据库设计

### 核心业务表（8 张）

| 表名 | 说明 | 主要字段 |
|------|------|---------|
| `disk_user_disk` | 用户磁盘空间 | userId, usedQuota, totalQuota |
| `disk_folder` | 目录 | userId, folderName, parentId, fixedPath |
| `disk_file` | 文件 | userId, fileName, folderId, ossKey, fileSize, sourceAgent, isArtifact |
| `disk_file_version` | 文件版本 | fileId, version, ossKey, fileSize |
| `disk_permission` | 权限 | agentId, agentGroupId, resourceId, resType, permission, resourcePath |
| `disk_share` | 分享链接 | resourceId, resType, shareCode, extractCode, maxVisit, expireAt |
| `disk_tag` | 标签绑定 | fileId, tagName |
| `disk_recycle_bin` | 回收站 | resId, resType, resName, userId |

### 管理扩展表（4 张）

| 表名 | 说明 | 主要字段 |
|------|------|---------|
| `disk_admin_user` | 管理员账户 | username, passwordHash (bcrypt), role |
| `disk_api_key` | API 密钥 | name, keyHash (SHA-256), scope, department, expiresAt |
| `disk_public_directory` | 公共目录 | folderId, scope, department, name, description |
| `disk_oauth2_config` | OAuth2 配置 | provider, clientId, clientSecret, authURL, tokenURL |

## 技术栈选择

### 后端（Go）

| 选择 | 原因 |
|------|------|
| Go | 高性能并发，编译为单二进制，部署简单 |
| Gin | 轻量级 HTTP 框架，中间件生态丰富 |
| GORM | Go 主流 ORM，支持 MySQL 和 SQLite |
| jwt-go | JWT 签发和验证 |

### 前端（React）

| 选择 | 原因 |
|------|------|
| React | 组件化开发，生态成熟 |
| Ant Design | 企业级 UI 组件库，开箱即用 |
| Vite | 快速构建和热更新 |

### 网关（Node.js）

| 选择 | 原因 |
|------|------|
| Node.js + TypeScript | 与前端共享类型定义，异步 I/O 适合代理场景 |
| Express/Fastify | 轻量级 HTTP 服务 |
| better-sqlite3 | 嵌入式 SQLite，无需额外数据库服务 |

### 存储

| 选择 | 原因 |
|------|------|
| MySQL 8.0 | 企业级关系数据库，支持复杂查询 |
| SQLite | 开发环境轻量替代，零配置 |
| MinIO | 兼容 S3 API 的对象存储，可自部署 |
| Redis | 高性能缓存，会话管理，分布式锁 |

### SDK

| 选择 | 原因 |
|------|------|
| Python 3.9+ | AI Agent 生态主流语言 |
| httpx | 支持同步和异步 HTTP 客户端 |
| uv | 现代 Python 包管理器，快速依赖解析 |
