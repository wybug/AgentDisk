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

- 语言：Golang
- 框架：Gin
- 数据库：MySQL 8.0 / SQLite
- 存储：MinIO SDK（全OSS兼容）
- 鉴权：JWT（Bearer Token）
- 缓存：Redis（可选关闭）
- 部署：Docker / 单二进制文件

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

### 3. 开发模式

```bash
# 启动依赖服务
docker compose -f docker/docker-compose.yaml up -d mysql minio

# 本地运行
make run
```

## 📁 项目结构

```
agent-disk/
├── CLAUDE.md              # 智能体团队开发约束
├── config.yaml            # 配置模板
├── main.go                # 入口文件
├── config/                # 配置加载
├── internal/
│   ├── model/             # 数据模型（8张表）
│   ├── middleware/         # 中间件（JWT/CORS/Logger）
│   ├── handler/           # API处理器
│   ├── service/           # 业务逻辑层
│   ├── repository/        # 数据访问层
│   └── router/            # 路由注册
├── pkg/
│   ├── oss/               # OSS客户端封装
│   ├── response/          # 统一响应体
│   └── jwt/               # JWT工具
├── sql/                   # 数据库建表脚本
├── docker/                # Docker部署文件
├── docs/                  # OpenAPI文档
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
