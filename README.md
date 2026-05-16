# AgentDisk

**专为多智能体设计、OpenAPI 标准化、基于 OSS 的企业级智能体云盘中间件**

**Apache 2.0 开源协议，永久免费商用**

## 🎯 项目介绍

AgentDisk 是一款面向 AI 智能体生态的专用云盘，解决多智能体文件共享、产物持久化、权限隔离、资产沉淀问题。目前全网无同类开源项目，填补多智能体云盘技术空白。

本项目剥离业务耦合，纯中立中间件，任何 AI Agent 平台均可快速集成。

## ✨ 核心能力

- **多智能体Team架构**：支持多智能体协作、资源隔离、权限分发
- **OpenAI 风格 OpenAPI**：Bearer Token、统一返回、AI生态无缝接入
- **全量OSS兼容**：MinIO、阿里云OSS、腾讯云COS、华为OBS
- **细粒度RBAC权限**：用户、智能体双维度权限管控
- **高级文件能力**：版本回溯、回收站、标签检索、在线预览
- **安全外链分享**：时效、提取码、访问次数、手动作废
- **生产级安全规范**：审计日志、数据脱敏、防越权、防注入

## 🛠 技术栈

### 后端

- 语言：Golang
- 框架：Gin
- 数据库：MySQL 8.0 / SQLite
- 存储：MinIO SDK（全OSS兼容）
- 鉴权：JWT（Bearer Token） + OAuth2（Web 登录）
- 缓存：Redis（可选关闭）
- 部署：Docker / 单二进制文件

### 测试网关

- 运行时：Node.js + TypeScript
- 框架：Express
- 功能：OAuth2 Provider（授权/令牌/用户信息端点）、PKCE 支持、测试用户管理

### Web 前端

- 框架：React 18 + Vite + TypeScript
- UI 库：Ant Design 5
- 状态管理：Zustand + TanStack React Query
- 预览渲染：react-markdown、react-syntax-highlighter
- 路由：React Router v6

## 🚀 快速部署

### 1. 二进制启动

```bash
make build
./bin/agentdisk --config config.yaml
```

### 2. Docker 部署

```bash
make docker-up
```

### 3. 本地开发（前后端联调）

需要同时启动三个服务：

#### 步骤 1：启动后端

确保 MySQL 和 MinIO 已启动，然后配置 OAuth2（使用测试网关）：

```yaml
# config.yaml 中 oauth2 部分
oauth2:
  enabled: true
  client_id: "agentdisk"
  client_secret: "agentdisk-secret"
  auth_url: "http://localhost:3000/oauth2/authorize"
  token_url: "http://localhost:3000/oauth2/token"
  userinfo_url: "http://localhost:3000/oauth2/userinfo"
  redirect_url: "http://localhost:5173/auth/callback"
  scopes:
    - openid
    - profile
```

启动后端：

```bash
make run   # 端口 9100
```

#### 步骤 2：启动测试网关

```bash
cd gateway
npm install
npm run dev   # 端口 3000
```

网关预置测试账号：

| 用户 ID | 用户名 | 密码 |
|---------|--------|------|
| user001 | 张三 | test123 |
| user002 | 李四 | test123 |
| user003 | 王五 | test123 |

#### 步骤 3：启动 Web 前端

```bash
cd web
npm install
npm run dev   # 端口 5173
```

前端通过 Vite 代理将 `/v1`、`/auth`、`/health` 请求转发到后端 9100 端口，确保 Cookie 同源。

#### 访问

打开浏览器访问 http://localhost:5173，将自动跳转到网关登录页。

## 📁 项目结构

```
agent-disk/
├── CLAUDE.md              # 智能体团队开发约束
├── config.yaml            # 配置模板
├── main.go                # 入口文件
├── config/                # 配置加载
├── internal/
│   ├── model/             # 数据模型（8张表）
│   ├── middleware/         # 中间件（JWT/CORS/Logger/HybridAuth）
│   ├── handler/           # API处理器
│   ├── service/           # 业务逻辑层
│   ├── repository/        # 数据访问层
│   └── router/            # 路由注册
├── pkg/
│   ├── oss/               # OSS客户端封装
│   ├── response/          # 统一响应体
│   ├── jwt/               # JWT工具
│   ├── oauth2client/      # OAuth2客户端
│   └── download_token/    # 下载令牌工具
├── sql/                   # 数据库建表脚本
├── docker/                # Docker部署文件
├── docs/                  # OpenAPI文档 + 认证集成指南
├── gateway/               # 测试网关（Node.js OAuth2 Provider）
│   ├── src/
│   │   ├── index.ts       # Express 入口
│   │   ├── oauth2/        # OAuth2 端点实现
│   │   ├── store/         # 内存数据存储
│   │   └── views/         # 内嵌 HTML 页面
│   └── package.json
├── web/                   # Web 前端（React）
│   ├── src/
│   │   ├── api/           # API 客户端层
│   │   ├── components/    # UI 组件
│   │   ├── pages/         # 页面
│   │   ├── router/        # 路由 + 登录守卫
│   │   ├── store/         # 状态管理
│   │   └── utils/         # 工具函数
│   └── package.json
└── test/                  # 测试
```

## 🔗 第三方平台集成方式

1. **HTTP API接入**：5分钟快速对接，无需侵入源码。
2. **SDK接入**：提供 Go / Python SDK，深度嵌入智能体框架。
3. **私有化部署**：无厂商绑定、无平台锁定。

## 📦 版本规划

### 社区开源版（永久开源）

- 用户空间、目录管理、文件CRUD
- 基础权限、回收站、标签检索
- 在线预览、基础外链
- 完整OpenAPI文档

### 企业增强版（闭源增值）

- 数据加密存储、超大文件分片
- 分布式检索、运维监控面板
- 高级审计、专属技术支持

## 📄 开源协议

本项目采用 **Apache 2.0** 开源协议，商用免费，唯一要求：保留开源声明。

## ⭐ Star

如果本项目对你有用，请点亮 Star，助力 AI 智能体开源生态。
