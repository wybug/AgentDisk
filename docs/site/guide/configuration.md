# 配置说明

AgentDisk 通过 `config.yaml` 配置文件和环境变量两种方式管理配置。环境变量优先级高于配置文件，适合在生产环境中注入敏感信息。

## 配置加载机制

配置加载顺序（优先级从高到低）：

1. **系统环境变量** - 直接设置的环境变量
2. **`.env` 文件** - 项目根目录下的 `.env` 文件（可选）
3. **`config.yaml`** - 主配置文件

敏感字段（密码、密钥）始终通过环境变量覆盖，配置文件中无需填写明文。

## config.yaml 完整参考

以下是 `config.yaml` 的完整配置项说明：

```yaml
# ==================== 服务配置 ====================
server:
  port: "9100"           # 监听端口
  mode: release          # 运行模式：debug / release / test

# ==================== 数据库配置 ====================
database:
  host: 127.0.0.1        # 数据库主机地址
  port: 3306             # 数据库端口
  name: agentdisk        # 数据库名称
  user: root             # 数据库用户名
  # password: 通过环境变量 DB_PASSWORD 注入
  max_idle_conns: 10     # 最大空闲连接数
  max_open_conns: 100    # 最大打开连接数
  log_level: warn        # 日志级别：silent / error / warn / info

# ==================== OSS 对象存储配置 ====================
oss:
  endpoint: "127.0.0.1:9000"  # OSS 服务地址（不含协议前缀）
  # access_key: 通过环境变量 OSS_ACCESS_KEY 注入
  # secret_key: 通过环境变量 OSS_SECRET_KEY 注入
  bucket: "agentdisk"         # 存储桶名称
  use_ssl: false              # 是否使用 HTTPS 连接 OSS
  region: "default"           # 区域标识

# ==================== Redis 配置 ====================
redis:
  enabled: false              # 是否启用 Redis
  addr: "127.0.0.1:6379"     # Redis 地址
  # password: 通过环境变量 REDIS_PASSWORD 注入
  db: 0                       # Redis 数据库编号

# ==================== JWT 配置 ====================
jwt:
  secret: "dev-jwt-secret"    # JWT 签名密钥（生产环境必须更换）
  expire_hours: 72            # Token 有效期（小时）

# ==================== 日志配置 ====================
log:
  level: info                 # 日志级别：debug / info / warn / error
  output: stdout              # 日志输出：stdout / file
  file_path: logs/agentdisk.log  # 日志文件路径（output 为 file 时生效）

# ==================== OAuth2 配置 ====================
oauth2:
  enabled: true                              # 是否启用 OAuth2 Web 登录
  client_id: "agentdisk"                     # OAuth2 客户端 ID
  # client_secret: 通过环境变量 OAUTH2_CLIENT_SECRET 注入
  auth_url: "http://localhost:3100/oauth2/authorize"      # 授权端点
  token_url: "http://localhost:3100/oauth2/token"         # 令牌端点
  userinfo_url: "http://localhost:3100/oauth2/userinfo"   # 用户信息端点
  redirect_url: "http://localhost:9100/auth/callback"     # 回调地址
  frontend_url: "http://localhost:9101"                   # 前端地址
  scopes:                                    # OAuth2 授权范围
    - openid
    - profile

# ==================== 下载令牌配置 ====================
download_token:
  secret: "dev-dl-token-secret"   # 下载令牌签名密钥
  expire_seconds: 300             # 下载令牌有效期（秒），默认 300 秒（5 分钟）
```

## 配置项详解

### server - 服务配置

| 字段 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `port` | string | `"9100"` | HTTP 服务监听端口 |
| `mode` | string | `"release"` | Gin 框架运行模式。`debug` 输出详细日志，`release` 为生产模式，`test` 为测试模式 |

- 生产环境建议使用 `mode: release`，减少日志输出，提升性能
- 开发调试时使用 `mode: debug`，可以看到请求路由和参数详情

### database - 数据库配置

| 字段 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `host` | string | `"127.0.0.1"` | 数据库主机地址 |
| `port` | int | `3306` | 数据库端口 |
| `name` | string | `"agentdisk"` | 数据库名称 |
| `user` | string | `"root"` | 数据库用户名 |
| `password` | string | - | 通过 `DB_PASSWORD` 环境变量注入 |
| `max_idle_conns` | int | `10` | 连接池最大空闲连接数 |
| `max_open_conns` | int | `100` | 连接池最大打开连接数 |
| `log_level` | string | `"warn"` | GORM 日志级别 |

**连接池调优建议：**

- 低并发场景：`max_idle_conns: 10`、`max_open_conns: 50`
- 中等并发场景：`max_idle_conns: 20`、`max_open_conns: 100`
- 高并发场景：`max_idle_conns: 50`、`max_open_conns: 200`

### oss - 对象存储配置

| 字段 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `endpoint` | string | - | OSS 服务地址（不含 `http://` 或 `https://` 前缀） |
| `access_key` | string | - | Access Key，通过 `OSS_ACCESS_KEY` 环境变量注入 |
| `secret_key` | string | - | Secret Key，通过 `OSS_SECRET_KEY` 环境变量注入 |
| `bucket` | string | `"agentdisk"` | 存储桶名称 |
| `use_ssl` | bool | `false` | 是否使用 HTTPS 连接 OSS |
| `region` | string | `"default"` | 区域标识 |

**不同 OSS 提供商的配置示例：**

::: details MinIO（本地部署）
```yaml
oss:
  endpoint: "127.0.0.1:9000"
  bucket: "agentdisk"
  use_ssl: false
  region: "default"
```
:::

::: details 阿里云 OSS
```yaml
oss:
  endpoint: "oss-cn-hangzhou.aliyuncs.com"
  bucket: "your-bucket-name"
  use_ssl: true
  region: "oss-cn-hangzhou"
```
:::

::: details 腾讯云 COS
```yaml
oss:
  endpoint: "cos.ap-shanghai.myqcloud.com"
  bucket: "your-bucket-1250000000"
  use_ssl: true
  region: "ap-shanghai"
```
:::

### redis - Redis 配置

| 字段 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `enabled` | bool | `false` | 是否启用 Redis |
| `addr` | string | `"127.0.0.1:6379"` | Redis 服务地址 |
| `password` | string | - | Redis 密码，通过 `REDIS_PASSWORD` 环境变量注入 |
| `db` | int | `0` | Redis 数据库编号 |

当前版本 Redis 为可选组件，不启用不影响核心功能。

### jwt - JWT 配置

| 字段 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `secret` | string | - | JWT Token 签名密钥 |
| `expire_hours` | int | `72` | Token 有效期（小时），默认 72 小时（3 天） |

::: warning
`jwt.secret` 是系统安全的核心密钥，生产环境必须使用强随机字符串（建议 32 位以上）。可通过 `openssl rand -hex 32` 生成。
:::

JWT Token 中包含以下声明：

| 字段 | 说明 |
|------|------|
| `userId` | 用户唯一标识 |
| `agentId` | 智能体 ID（可选） |
| `agentGroupId` | 智能体组 ID（可选） |
| `department` | 部门标识（可选） |

### oauth2 - OAuth2 配置

| 字段 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `enabled` | bool | `true` | 是否启用 OAuth2 登录 |
| `client_id` | string | `"agentdisk"` | OAuth2 客户端 ID |
| `client_secret` | string | - | 客户端密钥，通过 `OAUTH2_CLIENT_SECRET` 环境变量注入 |
| `auth_url` | string | - | OAuth2 授权端点 |
| `token_url` | string | - | OAuth2 令牌端点 |
| `userinfo_url` | string | - | 用户信息端点 |
| `redirect_url` | string | - | OAuth2 回调地址（指向后端） |
| `frontend_url` | string | - | 前端首页地址 |
| `scopes` | []string | `["openid", "profile"]` | OAuth2 授权范围 |

OAuth2 认证流程：

1. 用户访问前端页面，未登录时跳转至 `auth_url`
2. 用户在 OAuth2 Provider 完成认证
3. Provider 回调 `redirect_url`，携带授权码
4. 后端使用授权码换取 Token，获取用户信息
5. 后端创建会话 Cookie，重定向回 `frontend_url`

### download_token - 下载令牌配置

| 字段 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `secret` | string | - | 下载令牌签名密钥 |
| `expire_seconds` | int | `300` | 令牌有效期（秒），默认 5 分钟 |

下载令牌用于临时授权文件下载，生成流程：

1. 客户端调用 `POST /v1/disk/files/:id/download-token` 获取临时令牌
2. 使用令牌访问 `GET /v1/disk/files/download?t=<token>` 下载文件
3. 令牌过期后需重新获取

### log - 日志配置

| 字段 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `level` | string | `"info"` | 日志级别 |
| `output` | string | `"stdout"` | 日志输出目标 |
| `file_path` | string | `"logs/agentdisk.log"` | 日志文件路径 |

日志级别说明：

| 级别 | 用途 |
|------|------|
| `debug` | 详细的调试信息，仅开发环境使用 |
| `info` | 一般运行信息，适合预发布环境 |
| `warn` | 警告信息，推荐生产环境使用 |
| `error` | 仅错误信息，排查问题时配合监控使用 |

## 环境变量速查

| 环境变量 | 对应配置 | 说明 |
|----------|---------|------|
| `DB_PASSWORD` | `database.password` | 数据库密码 |
| `OSS_ACCESS_KEY` | `oss.access_key` | OSS Access Key |
| `OSS_SECRET_KEY` | `oss.secret_key` | OSS Secret Key |
| `JWT_SECRET` | `jwt.secret` | JWT 签名密钥 |
| `REDIS_PASSWORD` | `redis.password` | Redis 密码 |
| `OAUTH2_CLIENT_ID` | `oauth2.client_id` | OAuth2 客户端 ID |
| `OAUTH2_CLIENT_SECRET` | `oauth2.client_secret` | OAuth2 客户端密钥 |
| `DL_TOKEN_SECRET` | `download_token.secret` | 下载令牌密钥 |
