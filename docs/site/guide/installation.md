# 安装部署

本文档介绍 AgentDisk 的系统要求以及多种部署方式，包括 Docker 部署、手动部署和生产环境建议。

## 系统要求

### 硬件要求

| 组件 | 最低配置 | 推荐配置 |
|------|----------|----------|
| CPU | 1 核 | 2 核+ |
| 内存 | 512 MB | 2 GB+ |
| 磁盘 | 1 GB（程序 + 日志） | 10 GB+（含 OSS 数据） |

### 软件要求

| 软件 | 版本要求 | 说明 |
|------|----------|------|
| Go | 1.22+ | 编译后端（仅源码部署需要） |
| Node.js | 18+ | 测试网关与前端（仅源码部署需要） |
| MySQL | 8.0+ | 主数据库（也可使用 SQLite） |
| MinIO | 最新版 | 对象存储，兼容 S3 协议 |
| Redis | 7+（可选） | 缓存，部分高级功能依赖 |

## Docker 部署（推荐）

Docker 部署是最简单的方式，项目提供了完整的 `docker-compose.yaml`。

### 1. 准备配置文件

```bash
cd agent-disk
cp config.yaml config.yaml.bak
```

根据实际环境修改 `config.yaml`，确保数据库和 OSS 的连接地址正确。在 Docker 环境中，服务间通过容器名通信，因此地址应使用服务名：

```yaml
database:
  host: mysql      # Docker 服务名
  port: 3306

oss:
  endpoint: "minio:9000"  # Docker 服务名
```

### 2. 配置环境变量

创建 `.env` 文件：

```bash
# MySQL 密码
DB_PASSWORD=your-strong-password

# OSS 密钥
OSS_ACCESS_KEY=minioadmin
OSS_SECRET_KEY=minioadmin

# JWT 密钥（必须设置，启动时会校验）
JWT_SECRET=your-strong-jwt-secret-at-least-32-chars
```

### 3. 启动服务

```bash
# 启动全部服务（后端 + MySQL + MinIO）
docker-compose -f docker/docker-compose.yaml up -d

# 如需启用 Redis
docker-compose -f docker/docker-compose.yaml --profile redis up -d
```

启动后服务端口映射：

| 服务 | 容器内端口 | 宿主机端口 | 说明 |
|------|-----------|-----------|------|
| AgentDisk API | 8080 | 8080 | 后端 API 服务 |
| MySQL | 3306 | 3306 | 数据库 |
| MinIO API | 9000 | 9000 | 对象存储 API |
| MinIO Console | 9001 | 9001 | MinIO 管理界面 |
| Redis | 6379 | 6379 | 缓存（可选） |

### 4. 验证服务

```bash
# 检查健康状态
curl http://localhost:8080/health

# 查看容器状态
docker-compose -f docker/docker-compose.yaml ps

# 查看后端日志
docker-compose -f docker/docker-compose.yaml logs -f agentdisk
```

### Docker Compose 服务说明

```yaml
services:
  agentdisk:
    build:
      context: ..
      dockerfile: docker/Dockerfile
    ports:
      - "8080:8080"
    depends_on:
      mysql:
        condition: service_healthy
      minio:
        condition: service_healthy
    environment:
      - DB_HOST=mysql
      - DB_PASSWORD=${DB_PASSWORD:-agentdisk123}
      - OSS_ENDPOINT=minio:9000
      - OSS_ACCESS_KEY=${OSS_ACCESS_KEY:-minioadmin}
      - OSS_SECRET_KEY=${OSS_SECRET_KEY:-minioadmin}
      - JWT_SECRET=${JWT_SECRET}
    volumes:
      - ../config.yaml:/app/config.yaml

  mysql:
    image: mysql:8.0
    environment:
      MYSQL_ROOT_PASSWORD: ${DB_PASSWORD:-agentdisk123}
      MYSQL_DATABASE: agentdisk
    volumes:
      - mysql_data:/var/lib/mysql
      - ../sql/schema.sql:/docker-entrypoint-initdb.d/schema.sql

  minio:
    image: minio/minio:latest
    command: server /data --console-address ":9001"
    environment:
      MINIO_ROOT_USER: ${OSS_ACCESS_KEY:-minioadmin}
      MINIO_ROOT_PASSWORD: ${OSS_SECRET_KEY:-minioadmin}
    volumes:
      - minio_data:/data
```

## 手动部署

### 1. 编译后端

```bash
# 克隆项目
git clone https://github.com/wybug/agent-disk.git
cd agent-disk

# 编译
make build

# 编译产物位于 bin/agentdisk
ls -la bin/agentdisk
```

你也可以直接使用 `go build`：

```bash
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o agentdisk .
```

### 2. 准备运行环境

```bash
# 创建运行目录
mkdir -p /opt/agentdisk
cp bin/agentdisk /opt/agentdisk/
cp config.yaml /opt/agentdisk/
```

### 3. 配置环境变量

```bash
# 创建 systemd 环境文件
cat > /etc/agentdisk.env << 'EOF'
DB_PASSWORD=your-db-password
OSS_ACCESS_KEY=your-access-key
OSS_SECRET_KEY=your-secret-key
JWT_SECRET=your-jwt-secret
DL_TOKEN_SECRET=your-download-token-secret
EOF

chmod 600 /etc/agentdisk.env
```

### 4. 创建 systemd 服务

```bash
cat > /etc/systemd/system/agentdisk.service << 'EOF'
[Unit]
Description=AgentDisk API Server
After=network.target mysql.service

[Service]
Type=simple
User=agentdisk
Group=agentdisk
WorkingDirectory=/opt/agentdisk
EnvironmentFile=/etc/agentdisk.env
ExecStart=/opt/agentdisk/agentdisk --config config.yaml
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
EOF

# 创建运行用户
useradd -r -s /bin/false agentdisk
chown -R agentdisk:agentdisk /opt/agentdisk

# 启动服务
systemctl daemon-reload
systemctl enable agentdisk
systemctl start agentdisk

# 查看状态
systemctl status agentdisk
```

### 5. 部署前端（可选）

如果需要部署 Web 前端，可以编译为静态文件后使用 Nginx 反向代理：

```bash
cd web
npm install
npm run build
# 产物在 web/dist/ 目录
```

Nginx 配置示例：

```nginx
server {
    listen 9101;
    server_name _;

    # 前端静态文件
    location / {
        root /opt/agentdisk/web/dist;
        try_files $uri $uri/ /index.html;
    }

    # API 反向代理
    location /v1/ {
        proxy_pass http://127.0.0.1:9100;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }

    # OAuth2 回调代理
    location /auth/ {
        proxy_pass http://127.0.0.1:9100;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
    }

    # 健康检查
    location /health {
        proxy_pass http://127.0.0.1:9100;
    }
}
```

## 环境变量参考

AgentDisk 通过环境变量覆盖敏感配置，优先级：**环境变量 > .env 文件 > config.yaml**。

| 环境变量 | 对应配置项 | 必需 | 说明 |
|----------|-----------|------|------|
| `DB_PASSWORD` | `database.password` | 是 | 数据库密码 |
| `OSS_ACCESS_KEY` | `oss.access_key` | 是 | OSS Access Key |
| `OSS_SECRET_KEY` | `oss.secret_key` | 是 | OSS Secret Key |
| `JWT_SECRET` | `jwt.secret` | 是 | JWT 签名密钥 |
| `REDIS_PASSWORD` | `redis.password` | 否 | Redis 密码 |
| `OAUTH2_CLIENT_ID` | `oauth2.client_id` | 否 | OAuth2 客户端 ID |
| `OAUTH2_CLIENT_SECRET` | `oauth2.client_secret` | 否 | OAuth2 客户端密钥 |
| `DL_TOKEN_SECRET` | `download_token.secret` | 否 | 下载令牌签名密钥 |

## 数据库初始化

AgentDisk 启动时会自动执行数据库迁移（GORM AutoMigrate），自动创建所需的全部数据表：

| 表名 | 说明 |
|------|------|
| `disk_folder` | 目录结构 |
| `disk_file` | 文件元数据 |
| `disk_file_version` | 版本快照 |
| `disk_permission` | 权限记录 |
| `disk_recycle_bin` | 回收站 |
| `disk_tag` | 标签字典 |
| `disk_tag_relation` | 标签-文件关联 |
| `disk_share` | 外链分享 |
| `disk_share_access_log` | 分享访问日志 |
| `user_disk` | 用户空间配额 |
| `disk_admin_user` | 管理员账户 |
| `disk_api_key` | API 密钥 |
| `disk_public_directory` | 公共目录映射 |
| `disk_oauth2_config` | OAuth2 动态配置 |

你也可以手动执行 SQL 脚本：

```bash
mysql -u root -p agentdisk < sql/schema.sql
```

## MinIO / OSS 配置

### 本地 MinIO

```bash
# 启动 MinIO
minio server /data --console-address ":9001"

# 创建 Bucket
mc alias set local http://127.0.0.1:9000 minioadmin minioadmin
mc mb local/agentdisk
```

### 云厂商 OSS

AgentDisk 兼容所有 S3 协议的对象存储，修改 `config.yaml` 中的 `oss` 配置即可：

| 云厂商 | endpoint 示例 | 说明 |
|--------|--------------|------|
| 阿里云 OSS | `oss-cn-hangzhou.aliyuncs.com` | 需设置 `use_ssl: true` |
| 腾讯云 COS | `cos.ap-shanghai.myqcloud.com` | 需设置 `use_ssl: true` |
| 华为云 OBS | `obs.cn-north-4.myhuaweicloud.com` | 需设置 `use_ssl: true` |
| AWS S3 | `s3.amazonaws.com` | 需设置 `use_ssl: true` |

::: warning
生产环境务必启用 `use_ssl: true`，确保数据传输安全。OSS Bucket 应设置为 **私有读写**，AgentDisk 通过签名 URL 控制访问。
:::

## Redis 配置

Redis 为可选组件，当前版本可不启用：

```yaml
redis:
  enabled: false
  addr: "127.0.0.1:6379"
  db: 0
```

如需启用：

```bash
# 启动 Redis
redis-server

# 配置密码时
redis-server --requirepass your-redis-password
```

在 `config.yaml` 中设置 `redis.enabled: true` 并配置对应地址和密码。

## 生产部署建议

### 安全

- **更换所有默认密钥**：JWT Secret、下载令牌 Secret、数据库密码必须使用强随机字符串
- **启用 HTTPS**：在反向代理层（Nginx/负载均衡器）终止 TLS
- **网络隔离**：后端 API 不直接暴露公网，通过反向代理访问
- **OSS 私有访问**：Bucket 必须设置为私有，通过签名 URL 访问文件
- **定期备份数据库**：建议每日全量备份 + 增量备份

### 性能

- **数据库连接池**：根据并发量调整 `max_open_conns`（默认 100）
- **数据库索引**：AgentDisk 已自动创建必要索引，大数据量场景可额外优化
- **OSS 分区**：大量文件时考虑分 Bucket 或分前缀存储
- **资源限制**：使用 Docker 时设置合理的 CPU 和内存限制

### 监控

- 健康检查端点：`GET /health`
- 日志级别：生产环境建议设置 `log.level: warn`
- 日志输出：可配置为文件输出 `log.output: file`

### 数据备份

```bash
# MySQL 备份
mysqldump -u root -p agentdisk > backup_$(date +%Y%m%d).sql

# MinIO 备份
mc mirror local/agentdisk /backup/minio/agentdisk
```
