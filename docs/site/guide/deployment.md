# 生产部署指南

本文档面向运维人员，提供 AgentDisk **生产环境**的完整部署流程。

**范围**：后端 API + Web 前端 + 文档站点 + MySQL + MinIO + Redis（可选），不含测试网关。

**前置阅读**：请先阅读 [安装部署](/guide/installation) 了解基础概念和开发环境搭建。

## 部署架构

所有服务通过 Docker Compose 编排，一条命令启动全部：

```
                         ┌─────────────────────────────────────┐
                         │     Nginx 容器 (80/443)             │
                         │                                     │
                         │  /           → 前端静态文件          │
                         │  /docs/     → 文档静态文件          │
                         │  /v1/、/auth/ → 反代至 Backend      │
                         └──┬──────────────────┬───────────────┘
                            │ 静态文件(内置)    │ 反向代理
                            │                  │
                     ┌──────┴──────┐    ┌──────┴──────────┐
                     │ Web 前端     │    │  Backend :8080  │
                     │ 文档站点     │    └──┬─────┬─────┬──┘
                     │ (构建时打包) │       │     │     │
                     └─────────────┘    ┌──┴──┐┌┴───┐┌┴────┐
                                        │MySQL││MinIO││Redis│
                                        │:3306││:9000││:6379│
                                        └─────┘└─────┘└─────┘
                                            内部网络（不暴露宿主机）
```

### 端口规划

| 服务 | 容器端口 | 宿主机端口 | 公网访问 | 说明 |
|------|----------|-----------|----------|------|
| Nginx | 80 | 80 / 443 | 是 | 唯一对外入口 |
| 后端 API | 8080 | 无 | 否 | 仅 Nginx 容器可访问 |
| MySQL | 3306 | 无 | 否 | 仅后端可访问 |
| MinIO API | 9000 | 无 | 否 | 仅后端可访问 |
| MinIO Console | 9001 | 无 | 否 | 仅运维内网访问 |
| Redis | 6379 | 无 | 否 | 仅后端可访问 |

> 所有后端服务运行在 Docker 内部网络 `agentdisk-net` 中，仅 Nginx 容器对外暴露端口。

## 部署前准备清单

### 硬件配置

| 场景 | CPU | 内存 | 磁盘（SSD） |
|------|-----|------|-------------|
| 最低配置 | 2 核 | 4 GB | 50 GB |
| 推荐配置 | 4 核 | 8 GB | 200 GB |
| 大规模 | 8 核+ | 16 GB+ | 500 GB+ |

### 软件依赖

| 软件 | 版本 | 用途 |
|------|------|------|
| Docker | 20.10+ | 容器运行时 |
| Docker Compose | v2+ | 服务编排 |

### 网络要求

- 已注册域名并配置 DNS A 记录指向服务器 IP
- 防火墙仅开放 22（SSH）、80（HTTP）、443（HTTPS）端口

### 生成密钥

```bash
# 生成所有生产密钥
export JWT_SECRET=$(openssl rand -hex 32)
export DB_PASSWORD=$(openssl rand -hex 16)
export OSS_ACCESS_KEY=$(openssl rand -hex 12)
export OSS_SECRET_KEY=$(openssl rand -hex 16)
export DL_TOKEN_SECRET=$(openssl rand -hex 32)
export REDIS_PASSWORD=$(openssl rand -hex 16)

# 查看并安全保存
env | grep -E 'SECRET|PASSWORD|KEY'
```

### 检查清单

- [ ] 服务器满足最低硬件要求
- [ ] 域名 DNS 已配置 A 记录
- [ ] Docker 及 Docker Compose 已安装
- [ ] 所有密钥已生成并安全保存
- [ ] `.env` 文件已创建并填入生产值
- [ ] 防火墙仅开放 22/80/443
- [ ] `config.yaml` 已按生产环境调整

## Docker Compose 生产部署

### 文件结构

```
docker/
├── Dockerfile              # 后端构建
├── Dockerfile.nginx        # 前端+文档+Nginx 多阶段构建
├── nginx.conf              # Nginx 配置（反代+静态文件）
├── docker-compose.yaml     # 开发环境
└── docker-compose.prod.yaml # 生产环境
```

### Dockerfile.nginx

前端和文档站点通过多阶段构建打包进 Nginx 镜像，无需手动构建或拷贝静态文件：

```dockerfile
# Stage 1: 构建前端
FROM node:20-alpine AS frontend
WORKDIR /app
COPY web/package.json web/package-lock.json ./
RUN npm ci
COPY web/ .
RUN npm run build

# Stage 2: 构建文档
FROM node:20-alpine AS docs
WORKDIR /app
COPY docs/site/package.json docs/site/package-lock.json ./
RUN npm ci
COPY docs/site/ .
RUN npm run docs:build

# Stage 3: Nginx
FROM nginx:1.27-alpine
COPY docker/nginx.conf /etc/nginx/conf.d/default.conf
COPY --from=frontend /app/dist /usr/share/nginx/html
COPY --from=docs /app/.vitepress/dist /usr/share/nginx/docs
EXPOSE 80
```

### docker-compose.prod.yaml

创建 `docker/docker-compose.prod.yaml`：

```yaml
version: "3.8"

services:
  nginx:
    build:
      context: ..
      dockerfile: docker/Dockerfile.nginx
    ports:
      - "80:80"
      - "443:443"
    depends_on:
      - agentdisk
    volumes:
      - ./ssl:/etc/nginx/ssl:ro
    restart: unless-stopped
    networks:
      - agentdisk-net

  agentdisk:
    build:
      context: ..
      dockerfile: docker/Dockerfile
    depends_on:
      mysql:
        condition: service_healthy
      minio:
        condition: service_healthy
    environment:
      - DB_HOST=mysql
      - DB_PASSWORD=${DB_PASSWORD}
      - OSS_ENDPOINT=minio:9000
      - OSS_ACCESS_KEY=${OSS_ACCESS_KEY}
      - OSS_SECRET_KEY=${OSS_SECRET_KEY}
      - JWT_SECRET=${JWT_SECRET}
      - DL_TOKEN_SECRET=${DL_TOKEN_SECRET}
    volumes:
      - ../config.yaml:/app/config.yaml:ro
      - app_logs:/app/logs
    restart: unless-stopped
    deploy:
      resources:
        limits:
          memory: 1G
          cpus: "2"
    networks:
      - agentdisk-net

  mysql:
    image: mysql:8.0
    environment:
      MYSQL_ROOT_PASSWORD: ${DB_PASSWORD}
      MYSQL_DATABASE: agentdisk
    volumes:
      - mysql_data:/var/lib/mysql
      - ../sql/schema.sql:/docker-entrypoint-initdb.d/schema.sql:ro
    command: >
      --character-set-server=utf8mb4
      --collation-server=utf8mb4_unicode_ci
      --innodb-buffer-pool-size=512M
      --max-connections=200
      --slow-query-log=1
      --long-query-time=2
    healthcheck:
      test: ["CMD", "mysqladmin", "ping", "-h", "localhost"]
      interval: 10s
      timeout: 5s
      retries: 10
    restart: unless-stopped
    networks:
      - agentdisk-net

  minio:
    image: minio/minio:latest
    command: server /data --console-address ":9001"
    ports:
      - "127.0.0.1:9001:9001"
    environment:
      MINIO_ROOT_USER: ${OSS_ACCESS_KEY}
      MINIO_ROOT_PASSWORD: ${OSS_SECRET_KEY}
    volumes:
      - minio_data:/data
    healthcheck:
      test: ["CMD", "curl", "-f", "http://localhost:9000/minio/health/live"]
      interval: 10s
      timeout: 5s
      retries: 10
    restart: unless-stopped
    networks:
      - agentdisk-net

  redis:
    image: redis:7-alpine
    command: >
      redis-server
      --requirepass ${REDIS_PASSWORD}
      --appendonly yes
      --maxmemory 256mb
      --maxmemory-policy allkeys-lru
    volumes:
      - redis_data:/data
    restart: unless-stopped
    networks:
      - agentdisk-net

volumes:
  mysql_data:
  minio_data:
  redis_data:
  app_logs:

networks:
  agentdisk-net:
    driver: bridge
```

> 关键设计说明：
> - Nginx 是唯一对外暴露端口（80/443）的服务
> - 后端 API、MySQL、MinIO、Redis 均不暴露宿主机端口，仅在内部网络通信
> - 前端和文档在构建时打包进 Nginx 镜像，无需手动管理静态文件
> - MinIO Console 保留 `127.0.0.1:9001` 映射，方便运维通过 SSH 隧道访问

### 启动服务

```bash
# 构建并启动（首次）
docker compose -f docker/docker-compose.prod.yaml up -d --build

# 查看状态
docker compose -f docker/docker-compose.prod.yaml ps

# 查看日志
docker compose -f docker/docker-compose.prod.yaml logs -f

# 验证
curl http://localhost/health
```

## Nginx 配置说明

`docker/nginx.conf` 已内置在 Nginx 容器中，主要配置：

| 路径 | 目标 | 说明 |
|------|------|------|
| `/v1/`、`/auth/` | `http://agentdisk:8080` | API 反向代理 |
| `/health` | `http://agentdisk:8080` | 健康检查 |
| `/docs/` | `/usr/share/nginx/docs/` | 文档站点静态文件 |
| `/` | `/usr/share/nginx/html/` | 前端 SPA（兜底） |

`location` 顺序很关键：`/v1/`、`/auth/`、`/health`、`/docs/` 必须在 `/` 之前，否则会被前端兜底规则拦截。

如需自定义 Nginx 配置（如添加 TLS），修改 `docker/nginx.conf` 后重建容器即可。

## HTTPS 与证书配置

### 挂载证书方式

将证书文件放入 `docker/ssl/` 目录，Nginx 容器通过 volume 自动加载：

```bash
mkdir -p docker/ssl
cp agentdisk.crt docker/ssl/
cp agentdisk.key docker/ssl/
chmod 600 docker/ssl/agentdisk.key
```

修改 `docker/nginx.conf`，将 `listen 80` 改为 HTTPS 配置：

```nginx
server {
    listen 443 ssl http2;
    server_name agentdisk.example.com;

    ssl_certificate     /etc/nginx/ssl/agentdisk.crt;
    ssl_certificate_key /etc/nginx/ssl/agentdisk.key;
    ssl_protocols       TLSv1.2 TLSv1.3;
    ssl_ciphers         ECDHE-ECDSA-AES128-GCM-SHA256:ECDHE-RSA-AES128-GCM-SHA256:ECDHE-ECDSA-AES256-GCM-SHA384:ECDHE-RSA-AES256-GCM-SHA384;
    ssl_prefer_server_ciphers on;
    ssl_session_cache   shared:SSL:10m;
    ssl_session_timeout 10m;

    add_header Strict-Transport-Security "max-age=31536000; includeSubDomains" always;

    # ... 其余 location 配置不变
}

# HTTP 重定向至 HTTPS
server {
    listen 80;
    server_name agentdisk.example.com;
    return 301 https://$host$request_uri;
}
```

重建 Nginx 容器使配置生效：

```bash
docker compose -f docker/docker-compose.prod.yaml up -d --build nginx
```

### Let's Encrypt（推荐）

```bash
# 1. 先用 HTTP 模式启动服务
docker compose -f docker/docker-compose.prod.yaml up -d

# 2. 在宿主机安装 certbot 获取证书
apt install -y certbot
certbot certonly --standalone -d agentdisk.example.com \
  --pre-hook "docker compose -f docker/docker-compose.prod.yaml stop nginx" \
  --post-hook "docker compose -f docker/docker-compose.prod.yaml start nginx"

# 3. 将证书链接到 docker/ssl/
cp /etc/letsencrypt/live/agentdisk.example.com/fullchain.pem docker/ssl/agentdisk.crt
cp /etc/letsencrypt/live/agentdisk.example.com/privkey.pem docker/ssl/agentdisk.key

# 4. 修改 nginx.conf 启用 HTTPS（如上）

# 5. 重建并启动
docker compose -f docker/docker-compose.prod.yaml up -d --build nginx

# 6. 设置自动续期
echo "0 3 * * * certbot renew --quiet --deploy-hook 'cp /etc/letsencrypt/live/agentdisk.example.com/*.pem /path/to/agent-disk/docker/ssl/ && docker compose -f /path/to/agent-disk/docker/docker-compose.prod.yaml restart nginx'" | crontab -
```

## 生产配置调优

### 后端配置

编辑 `config.yaml` 生产环境关键项：

```yaml
server:
  port: 8080            # 容器内端口，无需修改
  mode: release            # 必须为 release

database:
  max_idle_conns: 20       # 空闲连接数
  max_open_conns: 100      # 最大连接数
  log_level: warn          # 生产环境降低日志级别

log:
  level: warn              # debug → info → warn → error
  output: file             # 输出到文件
  file_path: /app/logs/agentdisk.log

jwt:
  expire_hours: 24         # 生产环境缩短 Token 有效期

download_token:
  expire_seconds: 300      # 下载令牌有效期
```

### MySQL 调优

在 Docker Compose 的 `mysql.command` 中已包含基础调参。如使用独立 MySQL 实例，建议在 `my.cnf` 中添加：

```ini
[mysqld]
innodb_buffer_pool_size = 1G          # 可用内存的 60-70%
innodb_log_file_size    = 256M
max_connections         = 200
innodb_flush_log_at_trx_commit = 1
sync_binlog             = 1
slow_query_log          = 1
long_query_time         = 2
character-set-server    = utf8mb4
collation-server        = utf8mb4_unicode_ci
```

创建专用数据库用户（避免使用 root）：

```sql
CREATE USER 'agentdisk'@'%' IDENTIFIED BY 'your-strong-password';
GRANT ALL PRIVILEGES ON agentdisk.* TO 'agentdisk'@'%';
FLUSH PRIVILEGES;
```

### MinIO 调优

- **磁盘容量**：建议预留原始数据量的 3 倍空间（含版本快照和冗余）
- **多磁盘部署**：生产环境建议至少 4 块磁盘启用纠删码
- **Bucket 策略**：确保 `agentdisk` Bucket 为 **私有读写**

```bash
# 通过 SSH 隧道访问 MinIO Console
ssh -L 9001:127.0.0.1:9001 user@server
# 本地浏览器打开 http://localhost:9001

# 或使用 mc 命令行验证
docker compose -f docker/docker-compose.prod.yaml exec minio mc alias set local http://localhost:9000 $OSS_ACCESS_KEY $OSS_SECRET_KEY
docker compose -f docker/docker-compose.prod.yaml exec minio mc anonymous get local/agentdisk
```

### Redis 调优（如启用）

Docker Compose 中已配置核心参数：

| 参数 | 值 | 说明 |
|------|----|------|
| `requirepass` | 随机强密码 | 认证保护 |
| `appendonly` | yes | AOF 持久化 |
| `maxmemory` | 256mb | 内存上限，按需调整 |
| `maxmemory-policy` | allkeys-lru | 淘汰策略 |

在 `config.yaml` 中启用 Redis：

```yaml
redis:
  enabled: true
  addr: "redis:6379"       # 使用 Docker 服务名
  password: ""             # 通过 REDIS_PASSWORD 环境变量注入
  db: 0
```

## 安全加固

### 防火墙

```bash
# 仅开放必要端口
ufw default deny incoming
ufw default allow outgoing
ufw allow 22/tcp
ufw allow 80/tcp
ufw allow 443/tcp
ufw enable

# 验证
ufw status
```

由于所有后端服务在 Docker 内部网络中运行，防火墙只需管控 22/80/443。

### 密钥轮换

定期轮换密钥（建议每 90 天）：

```bash
# 1. 生成新密钥
export NEW_JWT_SECRET=$(openssl rand -hex 32)

# 2. 更新 .env 文件
sed -i "s/^JWT_SECRET=.*/JWT_SECRET=$NEW_JWT_SECRET/" .env

# 3. 重启后端
docker compose -f docker/docker-compose.prod.yaml restart agentdisk

# 4. 验证
curl http://localhost/health
```

> 注意：轮换 JWT Secret 后，所有已颁发的 Token 将失效，用户需重新登录。

### 数据库安全

```sql
-- 禁用远程 root 登录
DELETE FROM mysql.user WHERE User='root' AND Host NOT IN ('localhost', '127.0.0.1', '::1');

-- 删除匿名用户
DELETE FROM mysql.user WHERE User='';

-- 删除测试库
DROP DATABASE IF EXISTS test;

FLUSH PRIVILEGES;
```

### SSH 加固

```bash
# /etc/ssh/sshd_config
PermitRootLogin no
PasswordAuthentication no
PubkeyAuthentication yes
```

```bash
systemctl restart sshd
```

### Docker 安全

- `.env` 文件权限设为 `600`：`chmod 600 .env`
- Docker socket 权限控制：避免将 `/var/run/docker.sock` 挂载入容器
- Nginx 容器中的静态文件为只读，无需额外权限控制

## 备份与恢复

### 备份策略

| 项目 | 频率 | 保留策略 | 工具 |
|------|------|----------|------|
| MySQL | 每日 03:00 | 7 日 + 4 周 + 12 月 | mysqldump |
| MinIO | 每日 04:00 | 7 日 + 4 周 | mc mirror |
| 配置文件 | 每次变更 | 最新 5 份 | tar |

> 前端和文档静态文件已内置于 Nginx 镜像中，可通过 `docker compose build` 重建，无需单独备份。

### MySQL 备份

```bash
#!/bin/bash
# /opt/agentdisk/scripts/backup-mysql.sh
BACKUP_DIR="/backup/mysql"
mkdir -p "$BACKUP_DIR"

docker compose -f docker/docker-compose.prod.yaml exec -T mysql \
  mysqldump -u root -p"${DB_PASSWORD}" \
  --single-transaction --routines --triggers \
  agentdisk | gzip > "${BACKUP_DIR}/agentdisk_$(date +%Y%m%d_%H%M%S).sql.gz"

# 清理 30 天前的备份
find "$BACKUP_DIR" -name "*.sql.gz" -mtime +30 -delete
```

添加 cron 定时任务：

```bash
chmod +x /opt/agentdisk/scripts/backup-mysql.sh
(crontab -l 2>/dev/null; echo "0 3 * * * /opt/agentdisk/scripts/backup-mysql.sh") | crontab -
```

### MySQL 恢复

```bash
gunzip < /backup/mysql/agentdisk_20260529_030000.sql.gz \
  | docker compose -f docker/docker-compose.prod.yaml exec -T mysql \
    mysql -u root -p"${DB_PASSWORD}" agentdisk
```

### MinIO 备份

```bash
#!/bin/bash
# /opt/agentdisk/scripts/backup-minio.sh
docker compose -f docker/docker-compose.prod.yaml exec minio mc alias set local \
  http://localhost:9000 "${OSS_ACCESS_KEY}" "${OSS_SECRET_KEY}"
docker compose -f docker/docker-compose.prod.yaml exec minio \
  mc mirror local/agentdisk "/backup/minio/agentdisk_$(date +%Y%m%d)"

# 清理 14 天前的备份
find /backup/minio -maxdepth 1 -type d -mtime +14 -exec rm -rf {} +
```

### MinIO 恢复

```bash
docker compose -f docker/docker-compose.prod.yaml exec minio mc alias set local \
  http://localhost:9000 "${OSS_ACCESS_KEY}" "${OSS_SECRET_KEY}"
docker compose -f docker/docker-compose.prod.yaml exec minio \
  mc mirror /backup/minio/agentdisk_20260529 local/agentdisk
```

### 配置文件备份

```bash
tar czf /backup/config/agentdisk-config_$(date +%Y%m%d).tar.gz \
  config.yaml .env \
  docker/docker-compose.prod.yaml \
  docker/nginx.conf \
  docker/Dockerfile.nginx
```

### 完整灾难恢复流程

1. 准备新服务器，安装 Docker 和 Docker Compose
2. 恢复代码仓库：`git clone` 或恢复代码备份
3. 恢复配置文件：`tar xzf agentdisk-config_YYYYMMDD.tar.gz`
4. 启动基础设施：`docker compose -f docker/docker-compose.prod.yaml up -d mysql minio redis`
5. 等待 MySQL 和 MinIO 健康检查通过
6. 恢复 MySQL 数据：`gunzip < backup.sql.gz | docker compose exec -T mysql mysql ...`
7. 恢复 MinIO 数据：`docker compose exec minio mc mirror /backup/... local/agentdisk`
8. 构建并启动全部服务：`docker compose -f docker/docker-compose.prod.yaml up -d --build`
9. 验证：`curl https://agentdisk.example.com/health`
10. 切换 DNS 指向新服务器

## 升级与回滚

### 升级流程

```bash
# 1. 备份数据库
/opt/agentdisk/scripts/backup-mysql.sh

# 2. 拉取新版本代码
git pull origin main

# 3. 重新构建并启动全部服务
docker compose -f docker/docker-compose.prod.yaml up -d --build

# 4. 验证
curl http://localhost/health
```

前端、文档和后端的变更会在 `--build` 时自动重建，无需手动操作。

### 仅升级后端

```bash
# 停止旧后端（MySQL/MinIO/Redis/Nginx 保持运行）
docker compose -f docker/docker-compose.prod.yaml stop agentdisk

# 重建并启动后端
docker compose -f docker/docker-compose.prod.yaml up -d --build agentdisk

# 验证
curl http://localhost/health
```

### 仅升级前端或文档

```bash
# 重建 Nginx 容器（包含前端+文档）
docker compose -f docker/docker-compose.prod.yaml up -d --build nginx
```

### 数据库迁移

后端启动时 GORM AutoMigrate 会自动处理表结构变更。升级前务必备份数据库，以防迁移出现异常。

### 回滚流程

```bash
# 1. 切回旧版本代码
git checkout v1.2.3

# 2. 如数据库已迁移，恢复备份
gunzip < /backup/mysql/agentdisk_PRE_UPGRADE.sql.gz \
  | docker compose -f docker/docker-compose.prod.yaml exec -T mysql \
    mysql -u root -p"${DB_PASSWORD}" agentdisk

# 3. 重新构建并启动
docker compose -f docker/docker-compose.prod.yaml up -d --build

# 4. 验证
curl http://localhost/health
```

### 版本固定建议

生产环境应使用具体版本号，而非 `latest`：

```yaml
services:
  mysql:
    image: mysql:8.0.36
  minio:
    image: minio/minio:RELEASE.2024-01-01T00-00-00Z
  redis:
    image: redis:7.2-alpine
```

后端和 Nginx 镜像通过代码版本控制，`git checkout` 切换版本后 `--build` 重建。

## 监控与告警

### 健康检查

后端提供健康检查端点：

```bash
curl http://localhost/health
# 返回 {"code":0,"message":"success","data":{"status":"ok"}}
```

Docker Compose 中可添加后端健康检查：

```yaml
agentdisk:
  # ... 其他配置
  healthcheck:
    test: ["CMD", "wget", "-q", "--spider", "http://localhost:8080/health"]
    interval: 30s
    timeout: 5s
    retries: 3
    start_period: 15s
```

### 日志管理

**查看服务日志**：

```bash
# 查看所有服务日志
docker compose -f docker/docker-compose.prod.yaml logs -f

# 查看特定服务
docker compose -f docker/docker-compose.prod.yaml logs -f agentdisk
docker compose -f docker/docker-compose.prod.yaml logs -f nginx
```

**Docker 日志轮转**：创建 `/etc/docker/daemon.json`：

```json
{
  "log-driver": "json-file",
  "log-opts": {
    "max-size": "50m",
    "max-file": "5"
  }
}
```

**后端文件日志**：在 `config.yaml` 中配置：

```yaml
log:
  level: warn
  output: file
  file_path: /app/logs/agentdisk.log
```

日志文件存储在 `app_logs` volume 中，可通过 `docker volume inspect` 查看路径。

### 关键监控指标

| 指标 | 来源 | 告警阈值 |
|------|------|----------|
| 后端可用性 | `/health` 端点 | 连续 3 次失败 |
| API 错误率 | Nginx access log | > 5% |
| API 响应时间 | Nginx access log | P99 > 3s |
| MySQL 连接数 | `SHOW STATUS LIKE 'Threads_connected'` | > 80% max_connections |
| MySQL 慢查询 | slow query log | 单条 > 10s |
| MinIO 磁盘使用 | `mc admin info` | > 85% |
| Redis 内存 | `INFO memory` | > 80% maxmemory |
| 系统磁盘 | `df -h` | > 85% |

### 告警建议

- **紧急**：后端宕机、MySQL 不可用、MinIO 不可用、磁盘 > 90%
- **警告**：错误率升高、响应变慢、磁盘 > 70%、证书即将过期
- **通知渠道**：邮件、Webhook、即时通讯

## 常见运维操作

```bash
# 查看所有服务状态
docker compose -f docker/docker-compose.prod.yaml ps

# 重启单个服务
docker compose -f docker/docker-compose.prod.yaml restart agentdisk

# 查看资源使用
docker stats

# 数据库维护
docker compose -f docker/docker-compose.prod.yaml exec mysql \
  mysql -u root -p -e "OPTIMIZE TABLE agentdisk.disk_file"

# 进入 Nginx 容器排查
docker compose -f docker/docker-compose.prod.yaml exec nginx sh

# 查看 Nginx 配置是否正确
docker compose -f docker/docker-compose.prod.yaml exec nginx nginx -t

# 重建并重启（代码更新后）
docker compose -f docker/docker-compose.prod.yaml up -d --build
```

## 故障排查

### 后端无法启动

```
检查项：
1. 查看后端日志：docker compose logs agentdisk
2. MySQL 是否就绪：docker compose exec mysql mysqladmin ping -h localhost
3. MinIO 是否就绪：docker compose exec minio curl -f http://localhost:9000/minio/health/live
4. 密钥是否设置：docker compose exec agentdisk env | grep JWT_SECRET
5. config.yaml 语法：docker compose exec agentdisk cat config.yaml
```

### 前端白屏

```
检查项：
1. Nginx 容器是否运行：docker compose ps nginx
2. 静态文件是否正确：docker compose exec nginx ls /usr/share/nginx/html/index.html
3. Nginx 配置语法：docker compose exec nginx nginx -t
4. 浏览器控制台是否有 API 请求错误（CORS/404）
5. 查看 Nginx 日志：docker compose logs nginx
```

### 数据库连接失败

```
检查项：
1. MySQL 容器状态：docker compose ps mysql
2. 网络连通性：docker compose exec agentdisk ping mysql
3. 密码是否一致：对比 .env 中 DB_PASSWORD 与 MySQL MYSQL_ROOT_PASSWORD
4. 手动连接测试：docker compose exec mysql mysql -u root -p
```

### 文件上传失败

```
检查项：
1. Nginx client_max_body_size（默认 500M）
2. MinIO 磁盘空间：docker compose exec minio mc admin info local
3. OSS 凭据是否正确：检查 OSS_ACCESS_KEY/OSS_SECRET_KEY
4. 后端日志中的错误信息：docker compose logs agentdisk
```

### SSL 证书问题

```
检查项：
1. 证书文件是否存在：ls docker/ssl/
2. 证书过期时间：openssl x509 -in docker/ssl/agentdisk.crt -noout -dates
3. 证书链完整性：curl -v https://agentdisk.example.com 2>&1 | grep "verify"
4. Nginx 配置语法：docker compose exec nginx nginx -t
```
