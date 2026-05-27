# 快速开始

本节帮助你从零开始快速运行 AgentDisk，完成第一次文件上传并通过 API 验证。

## 前置条件

在开始之前，请确保本地已安装以下软件：

| 依赖 | 最低版本 | 说明 |
|------|----------|------|
| Go | 1.22+ | 后端编译与运行 |
| Node.js | 18+ | 测试网关与前端开发 |
| MySQL | 8.0+ | 数据库（也可使用 SQLite） |
| MinIO | 最新版 | 对象存储服务（兼容 S3 协议） |
| Redis | 7+ | 可选，部分功能依赖 |
| Git | 2.x | 代码拉取 |

### 安装依赖服务

如果你使用 macOS，可以通过 Homebrew 快速安装：

```bash
# 安装 Redis
brew install redis

# 安装 MinIO
brew install minio/stable/minio

# 安装 MySQL（如需本地数据库）
brew install mysql
```

## 克隆项目

```bash
git clone https://github.com/wybug/agent-disk.git
cd agent-disk
```

## 配置环境

### 1. 创建环境变量文件

在项目根目录创建 `.env` 文件，填入敏感配置：

```bash
# 数据库密码
DB_PASSWORD=your_db_password

# OSS 密钥
OSS_ACCESS_KEY=minioadmin
OSS_SECRET_KEY=minioadmin

# JWT 签名密钥（生产环境务必更换为强随机字符串）
JWT_SECRET=your-strong-jwt-secret

# 下载令牌签名密钥
DL_TOKEN_SECRET=your-download-token-secret
```

::: warning
`.env` 文件包含敏感凭证，**切勿提交到版本控制系统**。项目 `.gitignore` 已包含 `.env` 条目。
:::

### 2. 编辑配置文件

编辑项目根目录的 `config.yaml`，根据你的环境调整以下配置：

```yaml
server:
  port: "9100"
  mode: debug

database:
  host: 127.0.0.1
  port: 3306
  name: agentdisk
  user: root
  max_idle_conns: 10
  max_open_conns: 100

oss:
  endpoint: "127.0.0.1:9000"
  bucket: "agentdisk"
  use_ssl: false
  region: "default"

oauth2:
  enabled: true
  client_id: "agentdisk"
  client_secret: "agentdisk-secret"
  auth_url: "http://localhost:3100/oauth2/authorize"
  token_url: "http://localhost:3100/oauth2/token"
  userinfo_url: "http://localhost:3100/oauth2/userinfo"
  redirect_url: "http://localhost:9100/auth/callback"
  frontend_url: "http://localhost:9101"
  scopes:
    - openid
    - profile
```

数据库和 OSS 的连接信息请根据实际情况修改。详细的配置说明请参考 [配置说明](/guide/configuration)。

### 3. 准备数据库与存储

```bash
# 创建 MySQL 数据库
mysql -u root -p -e "CREATE DATABASE IF NOT EXISTS agentdisk CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;"

# 创建 MinIO Bucket（如果 MinIO 已启动）
mc alias set local http://127.0.0.1:9000 minioadmin minioadmin
mc mb local/agentdisk --ignore-existing
```

::: tip
AgentDisk 启动时会自动执行数据库迁移（AutoMigrate），无需手动建表。
:::

## 启动服务

### 一键启动（推荐）

项目提供了一键启动脚本，会自动启动后端 API、测试网关和 Web 前端三个服务：

```bash
bash scripts/dev.sh start
```

启动成功后，你会看到如下输出：

```
╔══════════════════════════════════════════╗
║     所有服务已启动                        ║
╠══════════════════════════════════════════╣
║  后端 API:   http://localhost:9100       ║
║  测试网关:   http://localhost:3100       ║
║  Web 前端:   http://localhost:9101       ║
╠══════════════════════════════════════════╣
║  日志目录: .dev-logs/
║  停止命令: bash scripts/dev.sh stop
╚══════════════════════════════════════════╝
```

### 其他管理命令

```bash
# 停止所有服务
bash scripts/dev.sh stop

# 重启所有服务
bash scripts/dev.sh restart

# 查看服务状态
bash scripts/dev.sh status

# 查看日志（全部服务）
bash scripts/dev.sh logs

# 查看指定服务日志
bash scripts/dev.sh logs backend
bash scripts/dev.sh logs gateway
bash scripts/dev.sh logs web
```

### 测试网关预设账号

测试网关预置了以下测试账号，可直接用于登录：

| 用户 ID | 用户名 | 密码 |
|---------|--------|------|
| user001 | 张三 | test123 |
| user002 | 李四 | test123 |
| user003 | 王五 | test123 |

## 访问 Web 界面

1. 打开浏览器，访问 `http://localhost:9101`
2. 页面将自动跳转到测试网关的 OAuth2 登录页面
3. 输入测试账号（如 `张三` / `test123`），点击登录
4. 登录成功后自动跳转回 AgentDisk 文件管理器

## 第一次文件上传

### 通过 Web 界面上传

1. 登录后，你将看到文件管理器主界面
2. 在左侧目录树中，点击根目录或新建一个文件夹
3. 点击 **上传** 按钮，选择本地文件
4. 上传完成后，文件将出现在当前目录的文件列表中
5. 点击文件可以预览（支持 Markdown、代码文件、文本、图片等格式）

### 通过 API 上传

你也可以通过 API 直接上传文件：

```bash
# 1. 获取 JWT Token（通过 OAuth2 登录后获取，或使用 API Key）
TOKEN="your-jwt-token-here"

# 2. 上传文件
curl -X POST http://localhost:9100/v1/disk/files/upload \
  -H "Authorization: Bearer $TOKEN" \
  -F "file=@./README.md" \
  -F "folderId=0"

# 返回示例：
# {
#   "code": 0,
#   "message": "created",
#   "data": {
#     "id": 1,
#     "fileName": "README.md",
#     "fileSize": 1024,
#     "fileType": "md",
#     "folderId": 0,
#     "version": 1,
#     "createdAt": "2026-01-01T00:00:00Z"
#   }
# }
```

## 快速 API 测试

以下是一些常用的 API 测试命令，帮助你快速验证服务是否正常运行。

### 健康检查

```bash
curl http://localhost:9100/health
```

返回：

```json
{"code": 0, "message": "success", "data": {"status": "ok"}}
```

### 创建文件夹

```bash
curl -X POST http://localhost:9100/v1/disk/folders \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "folderName": "项目文档",
    "parentId": 0
  }'
```

### 列出文件夹内容

```bash
# 列出根目录下的文件夹
curl http://localhost:9100/v1/disk/folders?parentId=0 \
  -H "Authorization: Bearer $TOKEN"
```

### 查看空间使用情况

```bash
curl http://localhost:9100/v1/disk/space \
  -H "Authorization: Bearer $TOKEN"
```

返回：

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "totalQuota": 10737418240,
    "usedQuota": 1024,
    "rootFolder": "/"
  }
}
```

### 创建外链分享

```bash
curl -X POST http://localhost:9100/v1/disk/shares \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "resourceId": 1,
    "resType": "file",
    "extractCode": "abc123",
    "maxVisit": 100,
    "expireHours": 24
  }'
```

## 下一步

- [安装部署](/guide/installation) - 了解生产环境部署方案
- [配置说明](/guide/configuration) - 深入了解所有配置项
- [文件管理器](/guide/file-explorer) - Web 界面使用指南
- [API 参考](/api/overview) - 完整的 API 接口文档
