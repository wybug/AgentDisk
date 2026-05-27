# 标签搜索

标签搜索是 AgentDisk 提供的文件分类和检索功能，通过为文件绑定标签实现灵活的文件组织和快速查找。

## 功能概述

标签系统允许你：

- 为任意文件绑定一个或多个标签
- 通过多个标签组合搜索文件
- 随时解绑不需要的标签
- 利用标签建立文件分类体系

标签由系统自动管理，绑定操作时会自动创建标签字典条目。同一标签可以关联多个文件，同一文件也可以有多个标签。

## 绑定标签

### 通过 Web 界面

1. 在文件列表中选择目标文件
2. 点击文件操作菜单中的 **标签** 按钮
3. 输入标签名称（支持创建新标签或选择已有标签）
4. 确认绑定

### 通过 API

```bash
curl -X POST http://localhost:9100/v1/disk/tags/bind \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "fileId": 5,
    "tagName": "重要"
  }'
```

为同一文件绑定多个标签：

```bash
# 绑定第二个标签
curl -X POST http://localhost:9100/v1/disk/tags/bind \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "fileId": 5,
    "tagName": "技术方案"
  }'
```

绑定成功后，文件的 `tags` 字段会自动更新为逗号分隔的标签列表。

## 解绑标签

### 通过 Web 界面

1. 打开文件的标签管理面板
2. 点击标签旁的 **移除** 按钮
3. 确认解绑

### 通过 API

```bash
curl -X POST http://localhost:9100/v1/disk/tags/unbind \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "fileId": 5,
    "tagName": "重要"
  }'
```

解绑成功后，该标签与文件的关联关系被移除。如果标签不再关联任何文件，标签字典条目仍然保留，可继续使用。

## 标签搜索

### 通过 Web 界面

1. 在文件管理器的搜索功能中选择 **标签搜索**
2. 输入或选择要搜索的标签
3. 支持选择多个标签进行组合搜索
4. 搜索结果展示同时包含所选标签的文件

### 通过 API

**搜索单个标签：**

```bash
curl "http://localhost:9100/v1/disk/tags/search?tags=重要" \
  -H "Authorization: Bearer $TOKEN"
```

**搜索多个标签（组合搜索）：**

```bash
curl "http://localhost:9100/v1/disk/tags/search?tags=重要,技术方案" \
  -H "Authorization: Bearer $TOKEN"
```

多标签搜索为 **AND** 逻辑，即返回同时包含所有指定标签的文件。

返回示例：

```json
{
  "code": 0,
  "message": "success",
  "data": [
    {
      "id": 5,
      "fileName": "系统架构设计.md",
      "fileSize": 8192,
      "fileType": "md",
      "folderId": 1,
      "tags": "重要,技术方案",
      "createdAt": "2026-01-10T08:00:00Z"
    },
    {
      "id": 12,
      "fileName": "API 设计文档.md",
      "fileSize": 4096,
      "fileType": "md",
      "folderId": 1,
      "tags": "重要,技术方案,API",
      "createdAt": "2026-01-12T14:30:00Z"
    }
  ]
}
```

## 标签数据模型

AgentDisk 的标签系统使用两张表实现：

### 标签字典表（disk_tag）

| 字段 | 说明 |
|------|------|
| `id` | 标签 ID |
| `userId` | 所属用户 |
| `tagName` | 标签名称 |
| `createdAt` | 创建时间 |

### 标签-文件关联表（disk_tag_relation）

| 字段 | 说明 |
|------|------|
| `id` | 关联 ID |
| `tagId` | 标签 ID |
| `fileId` | 文件 ID |
| `createdAt` | 绑定时间 |

## 典型使用场景

### 项目文件分类

为不同项目的文件绑定项目名标签，方便跨目录查找同一项目的所有文件：

```
项目A/需求文档.docx   → 标签: "项目A", "需求"
项目A/技术方案.md     → 标签: "项目A", "技术方案"
项目B/需求文档.docx   → 标签: "项目B", "需求"
共享资料/API文档.md   → 标签: "项目A", "项目B", "API"
```

搜索 `项目A` 标签可以快速找到所有相关文件，不受目录结构限制。

### 文件优先级管理

使用标签标识文件的重要程度和状态：

```
紧急bug修复方案.md  → 标签: "紧急", "bug"
月度报告.pdf        → 标签: "定期", "报告"
会议纪要.md         → 标签: "会议", "2026Q1"
```

### 智能体产物分类

为不同智能体生成的文件添加标签：

```
数据分析报告.csv  → 标签: "智能体产物", "数据分析"
代码审查结果.md   → 标签: "智能体产物", "代码审查"
```

## 使用建议

- **标签命名规范**：团队内统一标签命名规范，如使用项目名、文件类型、优先级等级别
- **避免过度标签**：每个文件建议 3-5 个标签，过多标签反而降低搜索效率
- **定期清理**：定期检查和清理不再使用的标签
- **组合搜索**：善用多标签组合搜索，精确定位目标文件
