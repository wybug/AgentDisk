# 生产部署指南

本文档面向运维人员，提供 AgentDisk **生产环境**的完整部署流程。

**范围**：后端 API + Web 前端 + 文档站点 + MySQL + MinIO + Redis（可选），不含测试网关。

**前置阅读**：请先阅读 [安装部署](/guide/installation) 了解基础概念和开发环境搭建。

## 部署架构

```
                    ┌─────────────────────────────────────────┐
                    │           Nginx (80/443)                │
  浏览器/Agent ───► │  ├── /           → 前端静态文件          │
                    │  ├── /docs/      → 文档静态文件          │
                    │  ├── /v1/        → 后端 API :9100       │
                    │  ├── /auth/      → 后端 API :9100       │
                    │  └── /health     → 后端 API :9100       │
                    └────────────┬────────────────────────────┘
                                 │
              ┌──────────────────┼──────────────────┐
              │                  │                  │
        ┌─────┴─────┐    ┌──────┴──────┐    ┌──────┴──────┐
        │  Backend   │    │   MySQL     │    │   MinIO     │
        │  :9100     │    │   :3306     │    │   :9000     │
        └───────────┘    └─────────────┘    └─────────────┘
              │
        ┌─────┴─────┐
        │  Redis    │  （可选）
        │  :6379    │
        └───────────┘
```

### 端口规划

| 服务 | 端口 | 公网访问 | 说明 |
|------|------|----------|------|
| Nginx | 80 / 443 | 是 | 唯一对外入口 |
| 后端 API | 9100 | 否 | 仅 Nginx 可访问 |
| MySQL | 3306 | 否 | 仅后端可访问 |
| MinIO API | 9000 | 否 | 仅后端可访问 |
| MinIO Console | 9001 | 否 | 仅运维内网访问 |
| Redis | 6379 | 否 | 仅后端可访问 |

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
| Nginx | 1.24+ | 反向代理 |
| certbot | 最新 | TLS 证书（可选） |

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
- [ ] Nginx 已安装
- [ ] 所有密钥已生成并安全保存
- [ ] `.env` 文件已创建并填入生产值
- [ ] 防火墙仅开放 22/80/443
- [ ] `config.yaml` 已按生产环境调整

## Docker Compose 生产部署

创建 `docker/docker-compose.prod.yaml`：

```yaml
version: "3.8"

services:
  agentdisk:
    image: agentdisk:latest
    build:
      context: ..
      dockerfile: docker/Dockerfile
    ports:
      - "127.0.0.1:9100:8080"
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
    ports:
      - "127.0.0.1:3306:3306"
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
      - "127.0.0.1:9000:9000"
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
    ports:
      - "127.0.0.1:6379:6379"
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
> - 所有端口绑定 `127.0.0.1`，仅本机 Nginx 可访问，不暴露公网
> - 后端容器内运行在 8080（Dockerfile 默认），映射到宿主机 9100
> - Redis 在生产环境默认启用，包含密码认证和持久化
> - 资源限制防止单服务占用过多资源

### 启动服务

```bash
# 构建（首次）
docker compose -f docker/docker-compose.prod.yaml build

# 启动
docker compose -f docker/docker-compose.prod.yaml up -d

# 查看状态
docker compose -f docker/docker-compose.prod.yaml ps

# 查看日志
docker compose -f docker/docker-compose.prod.yaml logs -f agentdisk

# 验证健康状态
curl http://127.0.0.1:9100/health
```

## 前端生产构建

```bash
cd web
npm ci
npm run build
# 产物输出至 web/dist/
```

将构建产物部署到 Nginx 静态目录：

```bash
mkdir -p /opt/agentdisk/web
cp -r web/dist/* /opt/agentdisk/web/
```

> SPA 路由要求：Nginx 必须配置 `try_files $uri $uri/ /index.html` 以支持前端路由。

## 文档站点生产构建

```bash
cd docs/site
npm ci
npm run docs:build
# 产物输出至 docs/site/.vitepress/dist/
```

```bash
mkdir -p /opt/agentdisk/docs
cp -r docs/site/.vitepress/dist/* /opt/agentdisk/docs/
```

## Nginx 反向代理配置

创建 `/etc/nginx/conf.d/agentdisk.conf`：

```nginx
server {
    listen 80;
    server_name agentdisk.example.com;
    return 301 https://$host$request_uri;
}

server {
    listen 443 ssl http2;
    server_name agentdisk.example.com;

    # ── TLS ────────────────────────────────────────
    ssl_certificate     /etc/nginx/ssl/agentdisk.crt;
    ssl_certificate_key /etc/nginx/ssl/agentdisk.key;
    ssl_protocols       TLSv1.2 TLSv1.3;
    ssl_ciphers         ECDHE-ECDSA-AES128-GCM-SHA256:ECDHE-RSA-AES128-GCM-SHA256:ECDHE-ECDSA-AES256-GCM-SHA384:ECDHE-RSA-AES256-GCM-SHA384;
    ssl_prefer_server_ciphers on;
    ssl_session_cache   shared:SSL:10m;
    ssl_session_timeout 10m;

    # ── 安全头 ─────────────────────────────────────
    add_header X-Frame-Options       SAMEORIGIN always;
    add_header X-Content-Type-Options nosniff always;
    add_header X-XSS-Protection      "1; mode=block" always;
    add_header Strict-Transport-Security "max-age=31536000; includeSubDomains" always;

    # ── 全局设置 ───────────────────────────────────
    client_max_body_size 500M;
    proxy_connect_timeout 60s;
    proxy_read_timeout    300s;

    # ── Gzip ───────────────────────────────────────
    gzip on;
    gzip_types text/plain text/css application/json application/javascript text/xml application/xml text/javascript;
    gzip_min_length 1024;

    # ── 后端 API ───────────────────────────────────
    location /v1/ {
        proxy_pass http://127.0.0.1:9100;
        proxy_set_header Host              $host;
        proxy_set_header X-Real-IP         $remote_addr;
        proxy_set_header X-Forwarded-For   $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }

    # ── OAuth2 回调 ────────────────────────────────
    location /auth/ {
        proxy_pass http://127.0.0.1:9100;
        proxy_set_header Host              $host;
        proxy_set_header X-Real-IP         $remote_addr;
        proxy_set_header X-Forwarded-For   $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }

    # ── 健康检查 ───────────────────────────────────
    location /health {
        proxy_pass http://127.0.0.1:9100;
        access_log off;
    }

    # ── 文档站点 ───────────────────────────────────
    location /docs/ {
        alias /opt/agentdisk/docs/;
        try_files $uri $uri/ $uri/index.html =404;

        location ~* \.(js|css|png|jpg|svg|ico|woff2?)$ {
            expires 30d;
            add_header Cache-Control "public, immutable";
        }
    }

    # ── 前端（必须放最后，作为兜底） ───────────────
    location / {
        root /opt/agentdisk/web;
        try_files $uri $uri/ /index.html;

        location ~* \.(js|css|png|jpg|svg|ico|woff2?)$ {
            expires 30d;
            add_header Cache-Control "public, immutable";
        }
    }
}
```

> `location` 顺序很关键：`/v1/`、`/auth/`、`/health`、`/docs/` 必须在 `/` 之前，否则会被前端兜底规则拦截。

```bash
# 验证配置
nginx -t

# 重载配置
nginx -s reload
```

## HTTPS 与证书配置

### Let's Encrypt（推荐）

```bash
# 安装 certbot
apt install -y certbot python3-certbot-nginx

# 自动获取并配置证书
certbot --nginx -d agentdisk.example.com

# 测试自动续期
certbot renew --dry-run
```

certbot 会自动修改 Nginx 配置并设置续期 cron。证书默认位于 `/etc/letsencrypt/live/agentdisk.example.com/`。

### 自定义证书

将证书文件放置到指定路径，并在 Nginx 配置中引用：

```bash
mkdir -p /etc/nginx/ssl
cp agentdisk.crt /etc/nginx/ssl/
cp agentdisk.key /etc/nginx/ssl/
chmod 600 /etc/nginx/ssl/agentdisk.key
```

### 证书自动续期

Let's Encrypt 证书有效期 90 天，certbot 已自动配置续期定时任务。可手动验证：

```bash
# 查看续期定时器
systemctl list-timers | grep certbot

# 手动续期
certbot renew
nginx -s reload
```

## 生产配置调优

### 后端配置

编辑 `config.yaml` 生产环境关键项：

```yaml
server:
  port: 9100
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
# 验证 Bucket 策略为 private
mc alias set local http://127.0.0.1:9000 $OSS_ACCESS_KEY $OSS_SECRET_KEY
mc anonymous get local/agentdisk
# 应输出: Access permission for 'local/agentdisk' is 'none'
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
  addr: "127.0.0.1:6379"
  password: ""            # 通过 REDIS_PASSWORD 环境变量注入
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

确保 9100、3306、9000、6379 端口**不对外开放**。

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
curl https://agentdisk.example.com/health
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

- 容器以非 root 用户运行（Dockerfile 中已通过 Alpine 默认用户实现）
- `.env` 文件权限设为 `600`：`chmod 600 .env`
- Docker socket 权限控制：避免将 `/var/run/docker.sock` 挂载入容器

## 备份与恢复

### 备份策略

| 项目 | 频率 | 保留策略 | 工具 |
|------|------|----------|------|
| MySQL | 每日 03:00 | 7 日 + 4 周 + 12 月 | mysqldump |
| MinIO | 每日 04:00 | 7 日 + 4 周 | mc mirror |
| 配置文件 | 每次变更 | 最新 5 份 | tar |

### MySQL 备份

```bash
#!/bin/bash
# /opt/agentdisk/scripts/backup-mysql.sh
BACKUP_DIR="/backup/mysql"
mkdir -p "$BACKUP_DIR"

mysqldump -h 127.0.0.1 -u root -p"${DB_PASSWORD}" \
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
# 解压并恢复
gunzip < /backup/mysql/agentdisk_20260529_030000.sql.gz \
  | mysql -h 127.0.0.1 -u root -p agentdisk
```

### MinIO 备份

```bash
#!/bin/bash
# /opt/agentdisk/scripts/backup-minio.sh
mc alias set local http://127.0.0.1:9000 "${OSS_ACCESS_KEY}" "${OSS_SECRET_KEY}"
mc mirror local/agentdisk "/backup/minio/agentdisk_$(date +%Y%m%d)"

# 清理 14 天前的备份
find /backup/minio -maxdepth 1 -type d -mtime +14 -exec rm -rf {} +
```

### MinIO 恢复

```bash
mc alias set local http://127.0.0.1:9000 "${OSS_ACCESS_KEY}" "${OSS_SECRET_KEY}"
mc mirror /backup/minio/agentdisk_20260529 local/agentdisk
```

### 配置文件备份

```bash
tar czf /backup/config/agentdisk-config_$(date +%Y%m%d).tar.gz \
  /opt/agentdisk/config.yaml \
  .env \
  /etc/nginx/conf.d/agentdisk.conf \
  docker/docker-compose.prod.yaml
```

### 完整灾难恢复流程

1. 准备新服务器，安装 Docker、Docker Compose、Nginx
2. 恢复配置文件：`tar xzf agentdisk-config_YYYYMMDD.tar.gz -C /`
3. 启动基础设施：`docker compose -f docker/docker-compose.prod.yaml up -d mysql minio redis`
4. 等待 MySQL 和 MinIO 健康检查通过
5. 恢复 MySQL 数据：`gunzip < backup.sql.gz | mysql ...`
6. 恢复 MinIO 数据：`mc mirror /backup/minio/... local/agentdisk`
7. 启动后端：`docker compose -f docker/docker-compose.prod.yaml up -d agentdisk`
8. 恢复前端和文档静态文件
9. 验证：`curl https://agentdisk.example.com/health`
10. 切换 DNS 指向新服务器

## 升级与回滚

### 升级流程

```bash
# 1. 备份数据库
/opt/agentdisk/scripts/backup-mysql.sh

# 2. 拉取新版本代码
git pull origin main

# 3. 重新构建后端镜像
docker compose -f docker/docker-compose.prod.yaml build agentdisk

# 4. 停止旧后端（MySQL/MinIO/Redis 保持运行）
docker compose -f docker/docker-compose.prod.yaml stop agentdisk

# 5. 启动新版本
docker compose -f docker/docker-compose.prod.yaml up -d agentdisk

# 6. 验证
curl http://127.0.0.1:9100/health
```

### 前端和文档升级

```bash
# 构建前端
cd web && npm ci && npm run build
cp -r web/dist/* /opt/agentdisk/web/

# 构建文档
cd ../docs/site && npm ci && npm run docs:build
cp -r docs/site/.vitepress/dist/* /opt/agentdisk/docs/

# 重载 Nginx
nginx -s reload
```

### 数据库迁移

后端启动时 GORM AutoMigrate 会自动处理表结构变更。升级前务必备份数据库，以防迁移出现异常。

### 回滚流程

```bash
# 1. 停止新版本后端
docker compose -f docker/docker-compose.prod.yaml stop agentdisk

# 2. 如数据库已迁移，恢复备份
gunzip < /backup/mysql/agentdisk_PRE_UPGRADE.sql.gz \
  | mysql -h 127.0.0.1 -u root -p agentdisk

# 3. 使用旧版本镜像
docker compose -f docker/docker-compose.prod.yaml up -d agentdisk

# 4. 恢复旧前端文件
cp -r /backup/web/* /opt/agentdisk/web/
nginx -s reload

# 5. 验证
curl http://127.0.0.1:9100/health
```

### 版本固定建议

生产环境应使用具体版本号，而非 `latest`：

```yaml
services:
  agentdisk:
    image: agentdisk:v1.2.3    # 使用具体版本号
  mysql:
    image: mysql:8.0.36        # 固定 MySQL 小版本
  minio:
    image: minio/minio:RELEASE.2024-01-01T00-00-00Z
  redis:
    image: redis:7.2-alpine
```

## 监控与告警

### 健康检查

后端提供健康检查端点：

```bash
curl http://127.0.0.1:9100/health
# 返回 {"code":0,"message":"success","data":{"status":"ok"}}
```

Docker Compose 中可添加健康检查：

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

**后端日志**：在 `config.yaml` 中配置文件输出：

```yaml
log:
  level: warn
  output: file
  file_path: /app/logs/agentdisk.log
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

**文件日志轮转**：创建 `/etc/logrotate.d/agentdisk`：

```
/opt/agentdisk/logs/*.log {
    daily
    rotate 30
    compress
    delaycompress
    missingok
    notifempty
    copytruncate
}
```

**Nginx 日志**：默认位于 `/var/log/nginx/access.log` 和 `/var/log/nginx/error.log`，通过系统 logrotate 自动管理。

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
# 查看后端日志
docker compose -f docker/docker-compose.prod.yaml logs -f agentdisk

# 重启单个服务
docker compose -f docker/docker-compose.prod.yaml restart agentdisk

# 修改配置后重启
docker compose -f docker/docker-compose.prod.yaml restart agentdisk

# 查看资源使用
docker stats

# 数据库维护
docker compose -f docker/docker-compose.prod.yaml exec mysql \
  mysql -u root -p -e "OPTIMIZE TABLE agentdisk.disk_file"

# 清理 Docker 资源（谨慎）
docker system prune -f
```

## 故障排查

### 后端无法启动

```
检查项：
1. config.yaml 语法是否正确：cat config.yaml
2. MySQL 是否就绪：docker compose exec mysql mysqladmin ping -h localhost
3. MinIO 是否就绪：curl http://127.0.0.1:9000/minio/health/live
4. JWT_SECRET 是否设置：docker compose exec agentdisk env | grep JWT_SECRET
5. 查看后端日志：docker compose logs agentdisk
```

### 前端白屏

```
检查项：
1. Nginx 静态文件路径是否正确：ls /opt/agentdisk/web/index.html
2. Nginx try_files 配置是否包含 /index.html 回退
3. 浏览器控制台是否有 API 请求错误（CORS/404）
4. Nginx 配置语法：nginx -t
```

### 数据库连接失败

```
检查项：
1. MySQL 容器状态：docker compose ps mysql
2. 网络连通性：docker compose exec agentdisk ping mysql
3. 密码是否一致：对比 .env 中 DB_PASSWORD 与 MySQL MYSQL_ROOT_PASSWORD
4. 手动连接测试：mysql -h 127.0.0.1 -u root -p
```

### 文件上传失败

```
检查项：
1. Nginx client_max_body_size 是否足够大
2. MinIO 磁盘空间：mc admin info local
3. OSS 凭据是否正确：检查 OSS_ACCESS_KEY/OSS_SECRET_KEY
4. 后端日志中的错误信息
```

### SSL 证书问题

```
检查项：
1. 证书过期时间：openssl x509 -in /etc/nginx/ssl/agentdisk.crt -noout -dates
2. 证书链完整性：curl -v https://agentdisk.example.com 2>&1 | grep "verify"
3. certbot 续期状态：certbot certificates
4. Nginx SSL 配置语法：nginx -t
```
