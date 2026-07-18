# OKF 知识库

OKF（Open Knowledge Format）是 AgentDisk 内置的结构化知识库：把一个公共目录里的
Markdown 文档物化成「节点 + 边」的知识图谱，支持图谱遍历、全文搜索与只读分享。

本文是面向使用者的终端指南。架构与数据模型 internals 见
[OKF 架构](/architecture/okf)；接口字段见 [OKF Wiki 接口](/api/okf)。

## 核心概念

- **Bundle（知识库）**：一个注册过的公共目录。一个 bundle = 一组 Markdown 文档。
- **Node（节点）**：bundle 里的一个 `.md` 文件，解析自 frontmatter（`type` / `title` /
  `description` / `tags`）+ 正文。节点是图谱的顶点。
- **Edge（边）**：节点间的 Markdown 链接物化而成。`[X](./y.md)` 指向同 bundle 内的
  `y.md` 节点；指向不存在目标的链接是「死链」。
- **Frontmatter**：每个 `.md` 顶部的 YAML 块，`type` 字段必填。

```markdown
---
type: concept          # 必填
title: 自注意力机制
description: Self-attention 让序列中每个位置互相打分
tags: [attention, core]
---
# 自注意力机制
正文……可包含 [其他节点](./transformer.md) 的链接。
```

## 两种使用入口

| 入口 | 能做什么 | 鉴权 |
|------|---------|------|
| **Web UI**（`/okf`） | 浏览节点、图谱可视化、全文搜索、统计/死链、只读分享 | 登录用户 |
| **Python SDK / API** | 上述全部 + **写入节点**（write_markdown）、注册/刷新 bundle | 写入需 API Key |

> Web UI 的 frontmatter 抽屉是**只读**的——写入（authoring）只通过 API Key 走 SDK/API，
> 浏览器不暴露写凭证。

## 在 Web UI 中使用

1. 侧边栏「OKF 知识库」→ bundle 列表，进入某个 bundle 详情页。
2. **搜索**：详情页顶部的搜索框对标题/描述/正文做全文检索（按相关性排序）。
3. **节点 tab**：表格浏览全部节点，可按 type / tag 过滤，点行看 frontmatter。
4. **图谱 tab**：Cytoscape 可视化——单击节点展开邻居，双击看 frontmatter。
5. **统计 / 死链 tab**：节点/边计数、类型分布；死链扫描与列表。
6. **分享**：详情页「分享 Bundle」生成外链（可选提取码/有效期/访问次数），接收方在
   `/share/:code` 以只读视图浏览节点 + 图谱 + Markdown 预览。

## 用 Python SDK 写入

写入需要 API Key（在管理后台创建）。下面是一个最小闭环：

```python
from agentdisk import AgentDiskClient

client = AgentDiskClient(
    base_url="http://localhost:9100",
    api_key="<your-api-key>",
)

# 1. 把一个已含 index.md 的公共目录注册为 bundle
bundle = client.register_bundle(public_directory_id=7)

# 2. 写节点（frontmatter 的 type 必填；链接只能指向已存在的同 bundle 节点）
client.write_markdown(
    public_directory_id=7,
    rel_path="concepts/attention.md",
    content=b"""---
type: concept
title: 自注意力机制
---
详见 [Transformer 架构](./transformer.md)。
""",
)

# 3. 搜索（命中标题/描述/正文）
for node in client.iter_search("注意力", bundle_id=bundle.bundle_id):
    print(node.node_id, node.title)

# 4. 图谱查询：从某节点出发的可达集合
reachable = client.reachable(node_id, depth=2)
```

`iter_search` / `iter_broken_links` 是分页迭代器，自动跟随游标翻完所有结果，无需手写
翻页循环。完整方法见 [Python SDK](/integration/sdk-python#okf-知识库)。

## 写作约定（避免常见错误）

- **`index.md` 必须先于 `register_bundle` 存在**，且含 `okf_version: "0.1"` frontmatter。
- **`type` frontmatter 必填**，否则写入被拒（400）。
- **bundle 内链接只能指向已存在的节点**；否则记为死链（不影响写入，但会出现在死链列表）。
- **`log.md` 由系统维护**（追加式变更日志），不要给它加 frontmatter。
