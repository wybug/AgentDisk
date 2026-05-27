# 公共目录

公共目录是 AgentDisk 提供的团队级资源共享机制，允许管理员将指定目录发布为公共目录，所有登录用户都可以浏览其中的文件内容。

## 什么是公共目录

公共目录是一种由管理员创建和管理的共享资源映射。它将一个私有文件夹发布为公共可见的目录，使所有已认证用户（包括通过 API Key 访问的外部系统）都能查看其中的文件。

公共目录的核心特性：

- **管理员创建**：只有管理员可以在管理后台创建公共目录映射
- **全员可见**：所有已认证用户可以在侧边栏看到公共目录
- **作用域控制**：支持按部门（department）限制可见范围
- **统一浏览**：通过专用的浏览接口访问公共文件

## 典型使用场景

- **团队共享资料**：发布团队常用的模板、文档、规范
- **公共素材库**：提供图片、字体等公共资源
- **知识库**：共享技术文档、API 说明、操作手册
- **数据集分发**：发布训练数据集、测试数据

## 管理员创建公共目录

### 通过管理后台

1. 以管理员身份登录管理后台（`http://localhost:9101/admin`）
2. 进入 **公共目录管理** 页面
3. 点击 **创建公共目录**
4. 填写以下信息：
   - **显示名称**：公共目录在前端显示的名称
   - **作用域（scope）**：可见范围，`public` 表示全员可见，`department` 表示按部门可见
   - **部门（department）**：当作用域为 `department` 时，指定可见的部门名称
5. 确认创建

创建完成后，系统会自动生成对应的文件夹和路径映射。

### 通过 API

```bash
# 创建全员可见的公共目录
curl -X POST http://localhost:9100/v1/disk/admin/public-directories \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "displayName": "团队模板库",
    "scope": "public",
    "department": ""
  }'
```

创建按部门可见的公共目录：

```bash
curl -X POST http://localhost:9100/v1/disk/admin/public-directories \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "displayName": "研发部文档",
    "scope": "department",
    "department": "engineering"
  }'
```

返回示例：

```json
{
  "code": 0,
  "message": "created",
  "data": {
    "id": 1,
    "folderId": 10,
    "scope": "public",
    "department": "",
    "displayName": "团队模板库",
    "fixedPath": "/public/团队模板库",
    "isActive": true,
    "createdBy": "admin"
  }
}
```

## 用户浏览公共目录

### 通过 Web 界面

1. 登录 AgentDisk 文件管理器
2. 在左侧边栏中找到 **公共目录** 区域
3. 点击公共目录名称展开
4. 浏览目录中的文件和子文件夹
5. 可以下载或预览其中的文件

### 通过 API

**列出可见的公共目录**

```bash
# 列出当前用户可见的所有公共目录
curl http://localhost:9100/v1/disk/public-directories \
  -H "Authorization: Bearer $TOKEN"
```

系统会根据用户的部门信息自动过滤可见的公共目录：

- `scope: "public"` 的目录对所有用户可见
- `scope: "department"` 的目录仅对匹配部门的用户可见

**查看公共目录详情**

```bash
curl http://localhost:9100/v1/disk/public-directories/1 \
  -H "Authorization: Bearer $TOKEN"
```

**浏览公共目录下的子文件夹**

```bash
curl http://localhost:9100/v1/disk/public-directories/1/folders \
  -H "Authorization: Bearer $TOKEN"
```

## 管理公共目录

### 更新公共目录

```bash
# 修改显示名称或启用/禁用
curl -X PUT http://localhost:9100/v1/disk/admin/public-directories/1 \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "displayName": "更新后的名称",
    "isActive": true
  }'
```

### 禁用公共目录

将 `isActive` 设为 `false` 可以临时禁用公共目录，禁用后普通用户将看不到该目录：

```bash
curl -X PUT http://localhost:9100/v1/disk/admin/public-directories/1 \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "isActive": false
  }'
```

### 删除公共目录

```bash
curl -X DELETE http://localhost:9100/v1/disk/admin/public-directories/1 \
  -H "Authorization: Bearer $ADMIN_TOKEN"
```

::: warning
删除公共目录映射不会删除对应的文件夹和文件，仅解除公共可见的映射关系。
:::

### 查看所有公共目录（管理视图）

```bash
curl http://localhost:9100/v1/disk/admin/public-directories \
  -H "Authorization: Bearer $ADMIN_TOKEN"
```

## 公共目录数据模型

| 字段 | 说明 |
|------|------|
| `id` | 公共目录映射 ID |
| `folderId` | 关联的文件夹 ID |
| `scope` | 可见范围：`public`（全员）或 `department`（部门） |
| `department` | 部门标识（scope 为 department 时生效） |
| `displayName` | 前端显示名称 |
| `fixedPath` | 固定路径标识 |
| `isActive` | 是否启用 |
| `createdBy` | 创建者 |

## 使用 API Key 访问公共目录

外部系统可以通过 API Key 访问公共目录：

```bash
# 使用 API Key 浏览公共目录
curl http://localhost:9100/v1/disk/public-directories \
  -H "X-API-Key: ak_your_api_key_here"
```

API Key 关联的 `department` 字段会用于公共目录的部门过滤，确保只能看到匹配的公共目录。
