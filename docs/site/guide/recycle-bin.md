# 回收站

AgentDisk 的回收站功能提供文件的安全删除与恢复机制。所有删除操作均为软删除，文件会先移入回收站，支持随时恢复或彻底删除。

## 软删除机制

当你删除文件或文件夹时，系统不会立即永久删除数据，而是执行以下操作：

1. 将原始记录标记为已删除（`isDeleted: true`）
2. 在回收站表中创建一条记录，保存原始路径信息
3. OSS 中的实际文件暂时保留
4. 记录过期时间，超过保留期后可被清理

这种机制确保误删文件可以快速恢复，避免数据丢失风险。

### 回收站数据模型

每条回收站记录包含以下信息：

| 字段 | 说明 |
|------|------|
| `id` | 回收站记录 ID |
| `userId` | 所属用户 |
| `resourceId` | 原始资源 ID |
| `resType` | 资源类型：`file` 或 `folder` |
| `resName` | 资源名称 |
| `originalPath` | 删除前的原始路径 |
| `deletedBy` | 执行删除操作者 |
| `expireAt` | 过期时间 |
| `createdAt` | 删除时间 |

## 查看回收站

### 通过 Web 界面

1. 在左侧边栏中点击 **回收站** 入口
2. 可以看到所有已删除的文件和文件夹
3. 列表显示文件名、原始路径、删除时间等信息

### 通过 API

```bash
# 列出回收站中的所有项目
curl http://localhost:9100/v1/disk/recycle \
  -H "Authorization: Bearer $TOKEN"
```

返回示例：

```json
{
  "code": 0,
  "message": "success",
  "data": [
    {
      "id": 1,
      "userId": "user001",
      "resourceId": 5,
      "resType": "file",
      "resName": "旧版报告.pdf",
      "originalPath": "/项目文档/旧版报告.pdf",
      "deletedBy": "user001",
      "expireAt": "2026-02-15T00:00:00Z",
      "createdAt": "2026-01-15T10:30:00Z"
    },
    {
      "id": 2,
      "userId": "user001",
      "resourceId": 3,
      "resType": "folder",
      "resName": "临时文件夹",
      "originalPath": "/临时文件夹",
      "deletedBy": "user001",
      "expireAt": "2026-02-15T00:00:00Z",
      "createdAt": "2026-01-15T11:00:00Z"
    }
  ]
}
```

## 恢复文件或文件夹

### 通过 Web 界面

1. 进入回收站页面
2. 找到要恢复的项目
3. 点击 **恢复** 按钮
4. 文件将恢复到原始位置

### 通过 API

```bash
curl -X POST http://localhost:9100/v1/disk/recycle/restore \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "resourceId": 5,
    "resType": "file"
  }'
```

恢复成功后：

- 原始文件/文件夹记录的 `isDeleted` 标记恢复为 `false`
- 回收站中的记录被清除
- 文件重新出现在原来的目录中

::: tip
如果原始目录已被删除，系统会将文件恢复到根目录。
:::

## 彻底删除

### 通过 Web 界面

1. 进入回收站页面
2. 选择要彻底删除的项目
3. 点击 **永久删除** 按钮
4. 确认操作

::: danger
永久删除操作不可恢复！删除后 OSS 中的实际文件也会被清除。
:::

### 通过 API

```bash
curl -X DELETE http://localhost:9100/v1/disk/recycle \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "resourceId": 5,
    "resType": "file"
  }'
```

彻底删除会执行以下操作：

1. 从数据库中永久删除文件/文件夹元数据记录
2. 从 OSS 中删除实际存储的文件
3. 清除回收站记录
4. 释放用户空间配额

## 审计日志

回收站的所有操作均会留有记录：

- **删除记录**：包含 `deletedBy`（操作者）、`createdAt`（操作时间）、`originalPath`（原始路径）
- **恢复记录**：恢复操作会清除回收站条目，但原始数据中的更新时间会变化
- **永久删除**：彻底删除后数据库记录清除，建议配合外部审计系统保留操作日志

## 删除文件触发回收站

### 删除文件

```bash
curl -X DELETE http://localhost:9100/v1/disk/files/5 \
  -H "Authorization: Bearer $TOKEN"
```

### 删除文件夹

```bash
curl -X DELETE http://localhost:9100/v1/disk/folders/1 \
  -H "Authorization: Bearer $TOKEN"
```

删除文件夹时，该文件夹及其下所有文件都会被移入回收站，每个资源各生成一条回收站记录。
