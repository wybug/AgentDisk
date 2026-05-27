# 文件管理器

AgentDisk 提供了功能完整的 Web 文件管理器，支持目录管理、文件上传下载、在线预览等操作。本章介绍 Web 界面中各项功能的使用方法。

## 界面概览

登录 AgentDisk 后，你将看到文件管理器的主界面，主要包含以下区域：

- **顶部导航栏**：显示当前登录用户、空间使用情况
- **左侧边栏**：目录树导航、公共目录入口、回收站入口
- **主内容区**：当前目录的文件和子文件夹列表
- **操作工具栏**：上传、新建文件夹、搜索等操作按钮

## 空间配额

每个用户拥有独立的云盘空间，默认配额为 **10 GB**。

查看空间使用情况：

- 在 Web 界面顶部导航栏可以看到已用/总量
- 通过 API 查看：`GET /v1/disk/space`

```bash
curl http://localhost:9100/v1/disk/space \
  -H "Authorization: Bearer $TOKEN"
```

返回示例：

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "totalQuota": 10737418240,
    "usedQuota": 5242880,
    "rootFolder": "/"
  }
}
```

## 创建文件夹

### 通过 Web 界面

1. 进入目标目录（或停留在根目录）
2. 点击工具栏中的 **新建文件夹** 按钮
3. 输入文件夹名称
4. 确认创建

支持创建多级子文件夹，在目录树中可以展开查看层级关系。

### 通过 API

```bash
# 在根目录创建文件夹
curl -X POST http://localhost:9100/v1/disk/folders \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "folderName": "项目文档",
    "parentId": 0
  }'
```

在子目录中创建文件夹：

```bash
# 在 folderId=1 的文件夹下创建子文件夹
curl -X POST http://localhost:9100/v1/disk/folders \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "folderName": "技术方案",
    "parentId": 1
  }'
```

### 获取目录祖先路径

查看某个文件夹的完整路径层级：

```bash
curl http://localhost:9100/v1/disk/folders/3/ancestors \
  -H "Authorization: Bearer $TOKEN"
```

## 上传文件

### 通过 Web 界面

AgentDisk 支持单文件和多文件上传：

1. 进入目标文件夹
2. 点击 **上传** 按钮
3. 选择一个或多个文件
4. 等待上传完成

上传进度会实时显示，上传完成后文件自动出现在当前目录列表中。

### 通过 API

```bash
# 上传单个文件到根目录（folderId=0）
curl -X POST http://localhost:9100/v1/disk/files/upload \
  -H "Authorization: Bearer $TOKEN" \
  -F "file=@./report.pdf" \
  -F "folderId=0"

# 上传到指定文件夹
curl -X POST http://localhost:9100/v1/disk/files/upload \
  -H "Authorization: Bearer $TOKEN" \
  -F "file=@./data.csv" \
  -F "folderId=2"
```

上传成功返回：

```json
{
  "code": 0,
  "message": "created",
  "data": {
    "id": 5,
    "fileName": "report.pdf",
    "fileSize": 1048576,
    "fileType": "pdf",
    "folderId": 0,
    "version": 1,
    "sourceAgent": "",
    "isArtifact": false,
    "createdAt": "2026-01-15T10:30:00Z"
  }
}
```

### 重复上传同名文件

当向同一目录上传同名文件时，系统会自动创建新版本，版本号递增。历史版本可通过 [版本回溯](/guide/file-versions) 功能查看和恢复。

## 下载文件

### 通过 Web 界面

1. 在文件列表中找到目标文件
2. 点击文件操作菜单中的 **下载** 按钮
3. 文件将自动下载到本地

### 通过 API

文件下载采用两步机制，确保安全性：

**步骤 1：获取下载令牌**

```bash
curl -X POST http://localhost:9100/v1/disk/files/5/download-token \
  -H "Authorization: Bearer $TOKEN"
```

返回：

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

**步骤 2：使用令牌下载**

```bash
# 令牌有效期默认 5 分钟，过期需重新获取
curl -O http://localhost:9100/v1/disk/files/download?t=<downloadToken>
```

::: tip
下载令牌机制确保 OSS 存储始终为私有，文件访问通过后端签名控制，不会暴露 OSS 直链。
:::

## 在线预览

AgentDisk 支持多种文件格式的在线预览，无需下载即可查看内容。

### 支持的预览格式

| 文件类型 | 支持的扩展名 | 预览方式 |
|----------|-------------|----------|
| Markdown | `.md` | 渲染为格式化页面 |
| 代码文件 | `.go`, `.py`, `.js`, `.ts`, `.java`, `.json`, `.yaml` 等 | 语法高亮显示 |
| 文本文件 | `.txt`, `.log`, `.csv` 等 | 纯文本展示 |
| 图片文件 | `.png`, `.jpg`, `.jpeg`, `.gif`, `.svg`, `.webp` | 直接展示 |
| HTML 文件 | `.html`, `.htm` | 沙箱渲染 |

### 通过 Web 界面预览

1. 在文件列表中点击文件名
2. 系统自动打开预览窗口
3. 根据文件类型展示对应的预览内容

### HTML 沙箱预览

HTML 文件使用独立的沙箱预览机制，通过 iframe 隔离运行：

- 自动过滤潜在危险的脚本和标签
- 在安全沙箱中渲染 HTML 内容
- 防止 XSS 攻击和恶意代码执行

### 通过 API 预览

```bash
# 预览普通文件
curl http://localhost:9100/v1/disk/preview/5 \
  -H "Authorization: Bearer $TOKEN"

# 预览 HTML 文件（沙箱渲染）
curl http://localhost:9100/v1/disk/preview/5/html \
  -H "Authorization: Bearer $TOKEN"
```

## 重命名

### 重命名文件夹

```bash
curl -X PUT http://localhost:9100/v1/disk/folders/1 \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "folderName": "新名称"
  }'
```

### 更新文件（替换内容）

```bash
# 更新文件内容（会创建新版本）
curl -X PUT http://localhost:9100/v1/disk/files/5 \
  -H "Authorization: Bearer $TOKEN" \
  -F "file=@./updated-report.pdf" \
  -F "folderId=0"
```

## 删除文件和文件夹

### 通过 Web 界面

1. 在文件列表中选择要删除的文件或文件夹
2. 点击 **删除** 按钮
3. 确认删除操作

::: tip
删除操作会将文件移入 **回收站**，不会立即永久删除。你可以在回收站中恢复或彻底删除。
:::

### 通过 API

```bash
# 删除文件
curl -X DELETE http://localhost:9100/v1/disk/files/5 \
  -H "Authorization: Bearer $TOKEN"

# 删除文件夹（文件夹下的文件也会移入回收站）
curl -X DELETE http://localhost:9100/v1/disk/folders/1 \
  -H "Authorization: Bearer $TOKEN"
```

删除操作为软删除，数据进入回收站。详细操作请参考 [回收站](/guide/recycle-bin) 章节。

## 列出和浏览目录

### 获取文件夹详情

```bash
curl http://localhost:9100/v1/disk/folders/1 \
  -H "Authorization: Bearer $TOKEN"
```

### 列出子文件夹

```bash
# 列出根目录下的所有文件夹
curl "http://localhost:9100/v1/disk/folders?parentId=0" \
  -H "Authorization: Bearer $TOKEN"

# 列出指定文件夹下的子文件夹
curl "http://localhost:9100/v1/disk/folders?parentId=1" \
  -H "Authorization: Bearer $TOKEN"
```

### 列出文件

```bash
# 列出指定文件夹下的文件
curl "http://localhost:9100/v1/disk/files?folderId=0" \
  -H "Authorization: Bearer $TOKEN"
```

## 文件元数据

每个文件包含以下元数据信息：

| 字段 | 说明 |
|------|------|
| `id` | 文件唯一标识 |
| `fileName` | 文件名 |
| `fileSize` | 文件大小（字节） |
| `fileType` | 文件扩展名 |
| `folderId` | 所属文件夹 ID |
| `version` | 当前版本号 |
| `md5` | 文件 MD5 校验值 |
| `sourceAgent` | 来源智能体名称 |
| `sourceAgentGroup` | 来源智能体组 |
| `isArtifact` | 是否为智能体产物 |
| `tags` | 标签（逗号分隔） |
| `createdAt` | 创建时间 |
| `updatedAt` | 更新时间 |

## 智能体产物自动入盘

当文件由 AI 智能体生成时，系统会自动记录来源信息：

- `sourceAgent` - 产生该文件的智能体 ID
- `sourceAgentGroup` - 智能体所属的组 ID
- `isArtifact` - 标记为智能体产物

这些信息可通过 JWT Token 中的 `agentId` 和 `agentGroupId` 自动填充，无需手动设置。
