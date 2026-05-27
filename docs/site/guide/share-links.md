# 外链分享

AgentDisk 提供安全的外链分享功能，允许你将文件或文件夹通过链接分享给他人，支持提取码保护、有效期限制和访问次数控制。

## 创建分享链接

### 通过 Web 界面

1. 在文件列表中选择要分享的文件或文件夹
2. 点击操作菜单中的 **分享** 按钮
3. 在弹出的分享设置中配置：
   - **提取码**（可选）：设置后访问者需要输入正确的提取码才能查看
   - **有效期**：设置分享链接的有效时长（默认 72 小时）
   - **最大访问次数**（可选）：限制链接的总访问次数
4. 点击确认，系统生成分享链接和分享码

### 通过 API

```bash
curl -X POST http://localhost:9100/v1/disk/shares \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "resourceId": 5,
    "resType": "file",
    "extractCode": "abc123",
    "maxVisit": 100,
    "expireHours": 48
  }'
```

**参数说明：**

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `resourceId` | uint64 | 是 | 要分享的文件或文件夹 ID |
| `resType` | string | 是 | 资源类型：`file` 或 `folder` |
| `extractCode` | string | 否 | 提取码，不设置则无需提取码即可访问 |
| `maxVisit` | int | 否 | 最大访问次数，`-1` 表示不限制（默认不限制） |
| `expireHours` | int | 否 | 有效时长（小时），默认 72 小时 |

返回示例：

```json
{
  "code": 0,
  "message": "created",
  "data": {
    "id": 1,
    "userId": "user001",
    "resourceId": 5,
    "resType": "file",
    "shareCode": "a1b2c3d4",
    "extractCode": "abc123",
    "maxVisit": 100,
    "visitCount": 0,
    "expireAt": "2026-01-17T10:30:00Z",
    "isActive": true,
    "createdAt": "2026-01-15T10:30:00Z"
  }
}
```

创建成功后，你可以将分享码 `a1b2c3d4` 和提取码 `abc123` 一起分享给目标用户。

## 分享访问流程

分享链接的访问流程分为以下步骤：

### 步骤 1：获取分享信息

访问者通过分享码获取分享基本信息（无需认证）：

```bash
curl http://localhost:9100/v1/disk/share/a1b2c3d4
```

返回分享的公开信息（不包含文件内容）：

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "shareCode": "a1b2c3d4",
    "resType": "file",
    "isActive": true,
    "expireAt": "2026-01-17T10:30:00Z"
  }
}
```

### 步骤 2：验证提取码并访问

如果分享设置了提取码，访问者需要提交提取码进行验证：

```bash
curl -X POST http://localhost:9100/v1/disk/share/access \
  -H "Content-Type: application/json" \
  -d '{
    "code": "a1b2c3d4",
    "extractCode": "abc123"
  }'
```

验证成功后返回分享的文件信息，同时访问计数加 1，系统记录访问日志。

### 步骤 3：下载分享文件

验证通过后，访问者可以获取下载令牌来下载文件：

```bash
curl -X POST http://localhost:9100/v1/disk/share/download \
  -H "Content-Type: application/json" \
  -d '{
    "code": "a1b2c3d4",
    "extractCode": "abc123",
    "resourceId": 5
  }'
```

返回临时下载令牌：

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "downloadToken": "eyJhbGciOiJIUzI1NiJ9...",
    "expiresIn": 300
  }
}
```

使用令牌下载：

```bash
curl -O "http://localhost:9100/v1/disk/files/download?t=<downloadToken>"
```

## 分享设置说明

### 有效期

- 默认有效期为 **72 小时**
- 可通过 `expireHours` 参数自定义，如 `expireHours: 24` 设置为 1 天
- 过期后分享链接自动失效，访问者无法再访问

### 提取码

- 提取码为可选设置
- 设置后，访问者必须输入正确的提取码才能查看和下载
- 建议为敏感文件设置提取码，增加一层保护

### 访问次数限制

- `maxVisit` 设置最大访问次数，如 `maxVisit: 100`
- 设为 `-1` 或不设置表示不限制
- 达到最大访问次数后，分享链接自动失效
- 访问计数包含所有成功的访问请求

## 管理分享

### 查看我的分享列表

```bash
curl http://localhost:9100/v1/disk/shares \
  -H "Authorization: Bearer $TOKEN"
```

返回当前用户创建的所有分享记录，包含已过期和已撤销的记录。

### 撤销分享

```bash
curl -X DELETE http://localhost:9100/v1/disk/shares \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "shareId": 1
  }'
```

撤销后：

- 分享记录的 `isActive` 标记为 `false`
- 访问者无法再通过分享码访问
- 撤销操作不可逆，如需重新分享请创建新的分享链接

### 自动失效

分享链接会在以下情况自动失效：

| 条件 | 行为 |
|------|------|
| 超过有效期 | `expireAt` 到期后自动失效 |
| 达到最大访问次数 | `visitCount >= maxVisit` 时自动失效 |
| 资源被删除 | 原始文件/文件夹被删除后，分享无法访问 |

## 访问日志追踪

每次分享访问都会记录详细的日志信息：

| 字段 | 说明 |
|------|------|
| `shareId` | 关联的分享记录 ID |
| `visitorIP` | 访问者 IP 地址 |
| `userAgent` | 访问者的浏览器/客户端信息 |
| `action` | 访问动作类型（查看、下载等） |
| `createdAt` | 访问时间 |

访问日志用于审计和安全追溯，帮助分享者了解文件的访问情况。

## 分享数据模型

| 字段 | 说明 |
|------|------|
| `id` | 分享记录 ID |
| `userId` | 创建者用户 ID |
| `resourceId` | 分享的资源 ID |
| `resType` | 资源类型（file / folder） |
| `shareCode` | 分享码（唯一标识） |
| `extractCode` | 提取码 |
| `maxVisit` | 最大访问次数（-1 表示不限制） |
| `visitCount` | 已访问次数 |
| `expireAt` | 过期时间 |
| `isActive` | 是否有效 |
| `createdAt` | 创建时间 |
