# API 概览

AgentDisk 提供 RESTful 风格的 HTTP API，所有接口遵循统一的请求/响应规范，支持多种认证方式，便于与各类智能体框架、客户端应用集成。

## 基础信息

| 项目 | 说明 |
|------|------|
| 基础 URL | `http://localhost:9100` |
| API 前缀 | `/v1/disk/` |
| 协议 | HTTP/HTTPS |
| 字符编码 | UTF-8 |
| 时间格式 | RFC 3339（如 `2025-01-15T10:30:00Z`） |
| 容量单位 | 字节（Bytes） |

## 统一响应格式

所有接口均返回 JSON 格式响应体，结构如下：

```json
{
  "code": 0,
  "message": "success",
  "data": {}
}
```

| 字段 | 类型 | 说明 |
|------|------|------|
| `code` | `int` | 业务状态码，`0` 表示成功，非 `0` 表示错误 |
| `message` | `string` | 响应描述信息 |
| `data` | `any` | 响应数据，错误时可能不存在 |

### 成功响应

**HTTP 200 OK** —— 查询、更新、删除成功时返回：

```json
{
  "code": 0,
  "message": "success",
  "data": { ... }
}
```

**HTTP 201 Created** —— 资源创建成功时返回：

```json
{
  "code": 0,
  "message": "created",
  "data": { ... }
}
```

## 认证方式

AgentDisk 支持四种认证方式，通过 `HybridAuth` 中间件自动识别：

### 1. JWT Bearer Token（推荐）

适用于已登录用户通过 OAuth2 获取 JWT 后的请求。在请求头中携带 Token：

```
Authorization: Bearer <jwt-token>
```

JWT Payload 中包含以下声明：

| 字段 | 类型 | 说明 |
|------|------|------|
| `userId` | `string` | 用户唯一标识 |
| `department` | `string` | 所属部门 |
| `agentId` | `string` | 关联智能体 ID（可选） |
| `agentGroupId` | `string` | 关联智能体组 ID（可选） |

### 2. OAuth2 会话认证

通过标准 OAuth2 授权码流程登录，服务端自动管理会话：

- 发起登录：`GET /auth/login`
- 回调处理：`GET /auth/callback`
- 退出登录：`POST /auth/logout`

### 3. API Key 认证

适用于服务间调用和智能体集成场景。在请求头中携带 API Key：

```
X-API-Key: <api-key>
```

API Key 由管理后台创建，支持配置作用域（scope）和部门（department）。

### 4. 下载令牌（Download Token）

仅用于文件下载接口，是一种短期有效的临时令牌：

```
GET /v1/disk/files/download?t=<download-token>
```

- 默认有效期 300 秒（5 分钟）
- 由服务端生成，绑定特定用户和文件
- 下载接口为公开路由，无需 Authorization 头

## 错误码

| HTTP 状态码 | 业务码 | 说明 | 典型场景 |
|------------|--------|------|---------|
| 400 | 400 | 请求参数错误 | 缺少必填字段、参数格式不正确 |
| 401 | 401 | 未认证 | Token 缺失、过期或无效 |
| 403 | 403 | 权限不足 | 无权访问该资源、越权操作 |
| 404 | 404 | 资源不存在 | 文件/目录/分享未找到 |
| 500 | 500 | 服务器内部错误 | 服务端异常（详细信息不对外暴露） |

错误响应示例：

```json
{
  "code": 401,
  "message": "invalid or expired token"
}
```

::: warning 安全说明
服务端内部错误（500）不会向客户端暴露堆栈信息或内部错误详情，响应统一为 `"internal error"`。
:::

## 通用请求头

| 请求头 | 必填 | 说明 |
|--------|------|------|
| `Authorization` | 是* | JWT Bearer Token，格式：`Bearer <token>` |
| `X-API-Key` | 是* | API Key 认证时使用 |
| `Content-Type` | 是 | 请求体格式：`application/json` 或 `multipart/form-data` |

::: tip
`Authorization` 和 `X-API-Key` 二选一即可，系统自动识别认证方式。公开接口（如下载、外链访问）无需认证头。
:::

## 接口分类

| 分类 | 路径前缀 | 说明 |
|------|---------|------|
| 空间管理 | `/v1/disk/space` | 查询用户磁盘配额 |
| 目录管理 | `/v1/disk/folders` | 目录的增删改查 |
| 文件管理 | `/v1/disk/files` | 文件上传、下载、更新、删除 |
| 分享管理 | `/v1/disk/shares` | 外链分享创建与管理 |
| 权限管理 | `/v1/disk/permissions` | 智能体权限授予与校验 |
| 标签管理 | `/v1/disk/tags` | 标签绑定与检索 |
| 版本管理 | `/v1/disk/versions` | 文件版本列表与回溯 |
| 回收站 | `/v1/disk/recycle` | 回收站列表、恢复、永久删除 |
| 文件预览 | `/v1/disk/preview` | 在线预览文件内容 |
| 管理接口 | `/v1/disk/admin` | 管理员登录、用户管理、配置管理 |

## 频率限制

当前版本暂不限制请求频率。建议遵循以下最佳实践：

- 避免短时间内大量重复请求同一接口
- 批量操作建议合理间隔（100ms - 500ms）
- 大文件上传建议使用分块方式
- 下载令牌有效期内勿重复申请

## 数据隔离

所有数据操作强制按 `userId` 隔离：

- 查询时自动过滤当前用户的数据
- 无法访问其他用户的文件和目录
- 管理员接口使用独立的认证体系，与用户数据隔离

## curl 示例模板

以下为通用的 curl 请求模板，各接口文档中的示例基于此格式：

```bash
# JWT 认证请求
curl -X GET http://localhost:9100/v1/disk/space \
  -H "Authorization: Bearer <your-jwt-token>"

# API Key 认证请求
curl -X GET http://localhost:9100/v1/disk/space \
  -H "X-API-Key: <your-api-key>"

# POST JSON 请求
curl -X POST http://localhost:9100/v1/disk/folders \
  -H "Authorization: Bearer <your-jwt-token>" \
  -H "Content-Type: application/json" \
  -d '{"folderName": "新目录", "parentId": 0}'

# 文件上传请求
curl -X POST http://localhost:9100/v1/disk/files/upload \
  -H "Authorization: Bearer <your-jwt-token>" \
  -F "file=@/path/to/file.txt" \
  -F "folderId=0"
```
