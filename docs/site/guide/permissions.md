# 权限管理

AgentDisk 提供细粒度的权限控制体系，支持针对智能体（Agent）和智能体组（Agent Group）的资源授权，确保多智能体场景下的文件安全隔离与协作。

## 权限类型

系统支持以下四种权限类型：

| 权限 | 值 | 说明 |
|------|-----|------|
| Owner | `owner` | 资源所有者，拥有全部权限，包括删除和授权 |
| Read | `read` | 只读权限，可以查看文件内容和元数据 |
| Write | `write` | 写入权限，可以修改文件内容和上传新版本 |
| Delete | `delete` | 删除权限，可以将文件移入回收站 |

## 授权方式

AgentDisk 支持两种资源授权维度：

### 1. 资源 ID 授权

通过指定具体的 `resourceId` 和 `resType` 授权访问特定资源：

```bash
# 授予智能体对特定文件的读取权限
curl -X POST http://localhost:9100/v1/disk/permissions \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "agentId": "agent-001",
    "resourceId": 5,
    "resType": "file",
    "permission": "read"
  }'
```

### 2. 路径授权（Glob 模式）

通过 `resourcePath` 使用 Glob 模式匹配一组资源，实现批量授权：

```bash
# 授予智能体对 /reports/ 目录下所有文件的读取权限
curl -X POST http://localhost:9100/v1/disk/permissions \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "agentId": "agent-001",
    "resourcePath": "/reports/**",
    "permission": "read"
  }'
```

**支持的 Glob 模式：**

| 模式 | 说明 | 示例 |
|------|------|------|
| `*` | 匹配单层路径中任意名称 | `/docs/*` 匹配 `/docs/report.md` |
| `**` | 匹配多层路径 | `/docs/**` 匹配 `/docs/a/b/c.md` |
| `*.ext` | 匹配特定扩展名 | `/data/*.csv` 匹配 `/data/test.csv` |
| 组合使用 | 结合多种模式 | `/reports/**/*.pdf` 匹配 `/reports/2026/q1/report.pdf` |

::: tip
`resourcePath` 必须以 `/` 开头。资源 ID 授权和路径授权可以并存，系统会综合判定权限。
:::

## 智能体目标配置

权限授权需要指定授权目标，支持以下两种：

- **agentId** - 授予特定智能体权限
- **agentGroupId** - 授予智能体组权限（组内所有智能体继承该权限）

两者至少需要指定一个，也可以同时指定：

```bash
# 授予智能体组对目录的写入权限
curl -X POST http://localhost:9100/v1/disk/permissions \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "agentGroupId": "group-data-analysis",
    "resourcePath": "/shared/datasets/**",
    "permission": "read"
  }'
```

## 授权操作

### 授予权限

```bash
# 示例 1：按资源 ID 授予文件读取权限
curl -X POST http://localhost:9100/v1/disk/permissions \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "agentId": "agent-001",
    "resourceId": 5,
    "resType": "file",
    "permission": "read"
  }'
```

```bash
# 示例 2：按路径授予目录写入权限
curl -X POST http://localhost:9100/v1/disk/permissions \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "agentId": "agent-002",
    "resourcePath": "/workspace/project-a/**",
    "permission": "write"
  }'
```

```bash
# 示例 3：授予智能体组所有权限
curl -X POST http://localhost:9100/v1/disk/permissions \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "agentGroupId": "group-admin",
    "resourcePath": "/admin/**",
    "permission": "owner"
  }'
```

### 检查权限

```bash
# 检查智能体是否有特定资源的读取权限
curl "http://localhost:9100/v1/disk/permissions/check?agentId=agent-001&resourceId=5&resType=file&permission=read" \
  -H "Authorization: Bearer $TOKEN"
```

返回：

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "allowed": true
  }
}
```

也可以检查智能体组的权限：

```bash
curl "http://localhost:9100/v1/disk/permissions/check?agentGroupId=group-data-analysis&resourceId=5&resType=file&permission=read" \
  -H "Authorization: Bearer $TOKEN"
```

### 撤销权限

```bash
curl -X DELETE http://localhost:9100/v1/disk/permissions \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "agentId": "agent-001",
    "resourceId": 5,
    "resType": "file"
  }'
```

也可以撤销路径授权：

```bash
curl -X DELETE http://localhost:9100/v1/disk/permissions \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "agentId": "agent-001",
    "resourcePath": "/reports/**"
  }'
```

### 查看权限列表

```bash
# 列出当前用户授权的所有权限记录
curl http://localhost:9100/v1/disk/permissions \
  -H "Authorization: Bearer $TOKEN"
```

## 智能体产物自动权限

当智能体通过 JWT Token 认证并上传文件时，系统会自动处理权限：

- JWT Token 中包含 `agentId` 和 `agentGroupId` 声明
- 上传的文件自动标记 `sourceAgent` 和 `sourceAgentGroup`
- `isArtifact` 标记为 `true`，表示该文件为智能体产物
- 文件创建者（Token 中的 `userId`）自动获得 `owner` 权限

这意味着智能体产物的权限管理是自动化的，无需手动为每个产物设置权限。

## 权限优先级

当存在多条权限规则时，系统按以下优先级判定：

1. **资源 ID 精确匹配** > **路径 Glob 匹配**
2. **智能体个体权限** > **智能体组权限**
3. **高权限包含低权限**：`owner` > `delete` > `write` > `read`

判定逻辑：

```
请求访问资源 R，需要权限 P：
├── 检查 agentId + resourceId 精确匹配 → 找到则返回
├── 检查 agentId + resourcePath Glob 匹配 → 找到则返回
├── 检查 agentGroupId + resourceId 精确匹配 → 找到则返回
├── 检查 agentGroupId + resourcePath Glob 匹配 → 找到则返回
└── 未找到匹配 → 拒绝访问
```

## 权限数据模型

| 字段 | 说明 |
|------|------|
| `id` | 权限记录 ID |
| `userId` | 授权者用户 ID |
| `agentId` | 被授权的智能体 ID |
| `agentGroupId` | 被授权的智能体组 ID |
| `resourceId` | 资源 ID（精确授权时使用） |
| `resType` | 资源类型：`file` 或 `folder` |
| `resourcePath` | 资源路径（Glob 授权时使用） |
| `permission` | 权限类型：`owner` / `read` / `write` / `delete` |
| `createdAt` | 授权时间 |
| `updatedAt` | 更新时间 |

## 最佳实践

### 最小权限原则

只授予完成任务所需的最小权限：

- 智能体只需要读取文件 → 授予 `read`
- 智能体需要生成报告 → 授予 `write` 到指定目录
- 避免广泛使用 `owner` 权限

### 使用智能体组管理权限

当多个智能体需要相同的权限时，使用智能体组统一管理：

```bash
# 创建组权限，组内所有智能体继承
curl -X POST http://localhost:9100/v1/disk/permissions \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "agentGroupId": "analysis-team",
    "resourcePath": "/data/analysis/**",
    "permission": "read"
  }'
```

### 使用路径授权批量管理

利用 Glob 模式一次性授权整个目录树，避免逐个资源授权：

```bash
# 授予对整个工作区的读写权限
curl -X POST http://localhost:9100/v1/disk/permissions \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "agentId": "agent-code-review",
    "resourcePath": "/workspace/reviews/**",
    "permission": "write"
  }'
```

### 定期审查权限

定期检查权限列表，清理不再需要的授权：

```bash
curl http://localhost:9100/v1/disk/permissions \
  -H "Authorization: Bearer $TOKEN"
```
