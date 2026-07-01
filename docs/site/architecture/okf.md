# OKF 架构

OKF（Open Knowledge Format）v0.1 是 AgentDisk 内置的「知识图谱层」：把一组 markdown 文件注册成一个 bundle，自动派生出可全文检索 + 可图谱遍历的节点 / 边模型。本文档说明 OKF 的数据模型、物化机制、图谱算法、鉴权模型与性能特性。

## OKF v0.1 是什么

### 一句话定义

OKF 是一个**寄生在 public directory 之上**的知识表示协议：

- 一个 bundle = 一个 public directory
- 一个节点 = bundle 内的一个 markdown 文件
- 一条边 = 源节点 markdown 中指向目标节点的 wikilink / 链接

bundle 把「文件夹 + 一堆 md」重新组织成可查询的知识图：全文检索、按 type 聚合、邻居遍历、最短路径、死链扫描都在 bundle 上原生支持。

### 设计目标

- **最小耦合**：OKF 完全构建在已有的 public directory + OSS 之上，不引入新存储后端
- **前向兼容**：未知 frontmatter 字段写入 `extra`，未知 type 容忍读，schema 演进不破坏老数据（OKF §9）
- **读多写少**：物化视图 + Redis 邻接缓存让 BFS / 全文检索读路径几乎不落 DB
- **ACL 透明继承**：bundle 可见性 = underlying public directory 可见性，不另设权限模型

### 与 public directory 的关系

| 项 | public directory | OKF bundle |
|----|-----------------|-----------|
| 存储实体 | 文件夹 + 文件（OSS） | bundle 元数据 + 节点 / 边索引（DB） |
| 一对一 | `disk_folder` 行 | `disk_okf_bundle` 行 |
| 鉴权 | HybridAuth + ACL | 继承 public directory |
| 写入方式 | 上传接口 / 内容写入 | 内容写入（自动物化） |

public directory 是「物理存储」；bundle 是「逻辑视图」。一个 public directory 可以单独存在而不注册为 bundle，反之不行。

## 数据模型

三张表，单向依赖：

```
disk_okf_bundle (1) ──< disk_okf_node (N) ──< disk_okf_edge (N)
```

### disk_okf_bundle

| 字段 | 说明 |
|------|------|
| `id` | 主键，对外暴露为 `bundleId` |
| `public_directory_id` | 唯一外键，对应 `disk_public_directory.id` |
| `okf_version` | 取自根 `index.md` frontmatter，当前固定 `0.1` |
| `root_index_file_id` | 根 `index.md` 对应的 `disk_file.id`；首次刷新前为 0 |
| `title` / `description` | 取自根 `index.md` frontmatter |
| `status` | `active` / `archived`，当前默认 `active` |
| `node_count` / `edge_count` | 冗余计数，避免每次列表查询走 `COUNT(*)` |
| `created_at` / `updated_at` | 时间戳 |

### disk_okf_node

| 字段 | 说明 |
|------|------|
| `id` | 主键，对外 `nodeId` |
| `bundle_id` | 所属 bundle |
| `file_id` | underlying `disk_file` 行 |
| `rel_path` | bundle 内相对路径（如 `concepts/gemma.md`）；唯一索引 `(bundle_id, rel_path)` |
| `type` | frontmatter `type`，必填 |
| `title` / `description` | 标量字段，提升为列以便索引 |
| `tags_json` | JSON 列存 `tags` 数组 |
| `extra_json` | JSON 列存未知 frontmatter 字段（前向兼容） |
| `has_broken_link` | 死链扫描后更新的标志位 |
| `content_hash` | 文件内容 SHA-256，用于变更检测 |
| `created_at` / `updated_at` | 时间戳 |

### disk_okf_edge

| 字段 | 说明 |
|------|------|
| `id` | 主键 |
| `public_dir_id` | 冗余字段，便于按 public directory 切分 |
| `src_node_id` | 源节点 |
| `dst_node_id` | 目标节点；`dst_exists=false` 时为 0 |
| `dst_rel_path` | 目标相对路径（可能不存在） |
| `dst_exists` | 死链标志 |
| `link_text` / `src_line` / `link_kind` | 链接元数据 |
| 唯一索引 | `(src_node_id, dst_rel_path, src_line)` —— 重写同一 .md 产生相同边集合，幂等 |

`link_kind` 三种取值：

- `bundle`：bundle 内 wikilink，如 `[[concepts/gemma]]` 或 `[Gemma](./gemma.md)`
- `external`：绝对 URL（`https://...`），通常不参与图谱遍历
- `anchor`：同页锚点（`#section`）

## Frontmatter 规范

OKF 通过 YAML frontmatter 给 markdown 加结构化元数据。规则：

### 必填字段

| 文件 | 必填字段 | 备注 |
|------|---------|------|
| `index.md`（根） | `okf_version` | 注册 bundle 时校验，缺失即拒 |
| 除 `log.md` 外的 .md | `type` | 写入时校验（`ErrOkfMissingType`） |

### 保留文件

| 文件 | 角色 | Frontmatter |
|------|------|-------------|
| `index.md` | bundle 根索引、入口节点 | **必须** `okf_version`；可携带任意 OKF 字段 |
| `log.md` | writer 写操作追加日志 | **禁止** frontmatter |
| 其他 `.md` | 普通节点 | 必须有 `type` |

### 推荐字段

| 字段 | 类型 | 用途 |
|------|------|------|
| `title` | string | 节点标题，索引列 |
| `description` | string | 节点描述，索引列 + FTS5 全文检索目标 |
| `tags` | string[] | 标签过滤（`GET /nodes?tag=...`） |
| `timestamp` | string | 自由格式时间，原样存储 |
| `extra` | map | 兜底所有未知字段，原样存进 `extra_json` |

### 写入示例

```markdown
---
type: concept
title: Gemma
description: Open LLM from DeepMind.
tags: [model, google]
timestamp: "2026-06-01"
extra:
  context_window: 8192
---

# Gemma

See [[persons/deepmind]] for the team.
```

## 节点物化（materialization）

`POST /v1/disk/public-directories/:id/files/content` 是 OKF 的写路径。一次写入触发一连串事务化操作：

### 写入流程

```
1. 校验 contentType（仅 markdown）
2. 解析 frontmatter + strict 校验
   ├─ index.md 必须有 okf_version
   ├─ 非 index.md 必须有 type
   └─ log.md 禁止 frontmatter
3. OSS 落盘 markdown（auto-create 嵌套文件夹）
4. DB 事务：
   ├─ upsert disk_file（或创建）
   ├─ upsert disk_okf_node（含 tags / extra JSON）
   ├─ 删除该节点的所有旧 edge，重新解析出链并插入
   ├─ 更新 bundle.node_count / edge_count
   └─ 追加 log.md 一行（写类型 / 时间 / relPath）
5. 异步：scheduleIndexRegen（默认开启，可关）
   └─ 重写根 index.md，列出全部节点
6. 返回新物化的 OkfNode
```

### 为什么不直接 lazy 解析

OKF 的核心性能优势是**读路径不需要解析 markdown**：

- 全文检索 → 直接查 FTS5 虚拟表
- 邻居 / BFS → 直接查 `disk_okf_edge`
- type 聚合 → 直接 `GROUP BY type`

代价是写路径需要解析 + 物化。这个权衡在「读多写少」的知识库场景里非常划算。

### 边的目标解析规则

写入源节点 A 时，对 A 中每条 `bundle` 类型链接：

1. 计算 `dst_rel_path`（相对 A 所在目录）
2. 查 `disk_okf_node WHERE bundle_id=A.bundle_id AND rel_path=dst_rel_path`
3. 找到 → `dst_exists=true, dst_node_id=...`
4. 找不到 → `dst_exists=false, dst_node_id=0`

**关键约束**：目标必须在源写入时已经存在。先写 A 引用 B、再写 B，A 的那条边仍然是 `dst_exists=false`（不会自动回填）。要修正请运行 `POST /bundles/:id/scan` 或 `POST /bundles/:id/refresh`。

### SQLite FTS5 全文检索

`disk_okf_node_search` 是 SQLite FTS5 虚拟表，包含 `title` 和 `description` 两列：

```sql
CREATE VIRTUAL TABLE disk_okf_node_search
USING fts5(title, description, content='disk_okf_node', content_rowid='id');
```

通过触发器同步 INSERT / UPDATE / DELETE。WAL 模式 + `busy_timeout=5000ms` 让读不阻塞写。

> 后端启动必须带 `-tags fts5`（见 `scripts/dev.sh`），否则编译失败。MySQL 部署改用 `FULLTEXT INDEX ... WITH PARSER ngram`，由 `internal/repository/okf_search.go` 里的方言分支处理。

## 图谱查询

### BFS 算法

`Reachable`（多跳可达）和 `ShortestPath`（最短路径）都基于 BFS：

- **Reachable**：单向 BFS，深度上限 `MaxBFSDepth=3`
- **ShortestPath**：双向 BFS，src 从前往后、dst 从后往前，碰头即终止

> 为什么 `MaxBFSDepth=3`？大数据量 BFS 在「最大跳数 = 4」时扇出会爆炸。3 跳在 10 万节点的 bundle 上稳定 P50 < 100ms（P5c 会出基准）。深度参数超过上限会被静默钳到 3，不报错。

### Redis 邻接缓存

每跳 BFS 都需要拿一个节点的邻居列表。直接查 DB 会让 BFS 变成 `O(depth × frontier_size)` 次 SQL。

**`RedisGraphCache`**（`internal/service/okf_graph_cache.go`）：

- key：`okf:adj:{bundleId}:{nodeId}`
- value：JSON 编码的出边 `dst_node_id` 列表
- 仅缓存 `dst_exists=true` 的 live 边（死链不参与遍历）
- TTL 默认 5 分钟
- 「空邻居」也缓存（sentinel `[]`），避免反复回查 DB
- 任何 Redis 错误都退化为 cache miss（best-effort）

**失效策略**：

- 写入节点 A 时，调用 `Invalidate(A.id)` 失效 A 的邻接
- `RefreshBundle` / `RebuildGraph` 会清空整个 bundle 的缓存（按节点逐个 DEL，pipeline 批量）

### 查询语义对比

| 端点 | 起点 | 算法 | 返回 |
|------|------|------|------|
| `GET /nodes/:id/neighbors` | 单节点 | 直接查（无 BFS） | 1 跳 nodes + edges |
| `POST /nodes/:id/reachable` | 单节点 | 单向 BFS | N 跳 nodes（不含起点） |
| `POST /paths/shortest` | 双节点 | 双向 BFS | 有序 path；找不到时 `found=false` |
| `POST /subgraph` | Bundle | 全量过滤 | type 过滤后的 nodes + edges |
| `GET /bundles/:id/stats` | Bundle | 聚合 | 计数 + type 分布 |

## 死链管理

### has_broken_link 标志位

每个节点有一个布尔列 `has_broken_link`。死链扫描后，至少有一条 `dst_exists=false` 出边的节点被标为 true。读者可以廉价地按这个字段过滤（例如只列出无死链的节点）。

### 扫描流程

`POST /bundles/:id/scan`：

1. 遍历 bundle 内所有节点
2. 对每条 `bundle` 类型边，重新查 `disk_okf_node` 看 `dst_rel_path` 是否存在
3. 更新边的 `dst_exists`，更新节点的 `has_broken_link`
4. 返回 `scannedNodes` + `brokenCount`

### 死链列表（分页）

`GET /bundles/:id/broken-links` 返回所有 `dst_exists=false` 的边，按 `src_node_id` 游标分页（每页最多 50 条）。

### 索引重建

`POST /bundles/:id/regenerate-index` 用当前节点集合重新生成根 `index.md`：

- 模板按 type 分组列出所有节点
- 通过 `replaceFileContent` 写入 OSS，bump 版本号
- 返回新版本号 + 时间戳

### 重建图谱（admin）

`POST /admin/okf/bundles/:id/rebuild-graph` 调用 `RefreshBundle`，从 OSS 重新解析所有 markdown，重建 node + edge 表。该端点只对管理员 JWT 开放（见下节鉴权模型）。

## 鉴权模型

OKF 端点有三种鉴权模式，由 router 中间件决定：

### 三种中间件

| 中间件 | 允许 | 用于 |
|--------|------|------|
| `HybridAuth` | JWT **或** API Key **或** OAuth2 Session | 所有 `/v1/disk/okf/*` 读 / 维护端点 |
| `pdWrite` | **仅** API Key | `/v1/disk/public-directories/:id/files/content` |
| `AdminAuth` + `AdminOnly` | 管理员 JWT | `/v1/disk/admin/okf/bundles/:id/rebuild-graph` |

### 为什么 writer 独立中间件

OKF writer 是「写 OSS + 写 DB 索引 + 触发异步索引重建」的复合操作。设计上希望：

- **应用 Agent（API Key）可以批量推知识，不需要管理员介入**
- **JWT 用户不能直接改知识库**（避免前端误操作污染图谱）

`pdWrite` 强制只接受 `X-API-Key` 请求头，JWT 调用直接 403。

### 为什么 rebuild-graph 独立路径

`rebuild-graph` 是 O(N) 的重算操作（N = bundle 内文件数），不应该被普通调用方频繁触发：

- 普通用户用 `POST /bundles/:id/refresh`（HybridAuth）就够 —— 行为相同
- 管理员在排查问题、修复破损索引时才走 `rebuild-graph`

`AdminJWT` 与 `BearerAuth` 共用 `Authorization` 头但 claims 不同（`is_admin=true`），由路由器在中间件链中区分。

### ACL 继承

OKF 不维护自己的 ACL 表。bundle 可见性来自 underlying public directory：

- public directory scope = `global` → 所有认证用户可见
- public directory scope = `department` → 仅同部门用户可见
- public directory 单独授予 → 仅授权用户可见

服务层 `ListBundles(userID, department)` 调用 public directory 的 `ListVisible` 复用可见性算法，OKF 不重写一遍。

## 性能与限制

### 数据规模假设

OKF v0.1 针对的 workload：

- 单 bundle 节点数：100 – 100,000
- 单 bundle 边数：100 – 500,000
- 平均每节点字节数：1 – 10 KB
- 写入 QPS：1 – 50
- 读取 QPS：100 – 5,000

P5c 会跑 10 万节点 BFS 基准（详见后续 PR）。

### 性能特性

| 路径 | 性能特性 |
|------|---------|
| `GET /bundles/:id/nodes?type=...` | type 索引，10 万节点 P50 < 30ms |
| `POST /search` | FTS5 倒排，P50 < 20ms（短查询） |
| `GET /nodes/:id/neighbors` | 单跳直接查边表，P50 < 10ms |
| `POST /nodes/:id/reachable?depth=3` | BFS + Redis 缓存命中，P50 < 100ms |
| `POST /paths/shortest` | 双向 BFS + 缓存，P50 < 80ms |
| `POST /files/content`（writer） | OSS 写 + DB 事务，P50 < 200ms |

### 限制

- BFS 深度上限 `MaxBFSDepth = 3`（钳制，不报错）
- 死链分页 `limit ≤ 50`
- 搜索分页 `limit ≤ 100`，默认 20
- `log.md` 行无上限（按写入累积；过大时手动清理或重建 bundle）

### SQLite vs MySQL

`internal/repository/okf_search.go` 按方言分支：

| 后端 | 全文检索 |
|------|---------|
| SQLite | FTS5 虚拟表 + 触发器 |
| MySQL | `FULLTEXT INDEX` + `WITH PARSER ngram`（支持中文） |

启动时按 `config.yaml` 的 `db.driver` 自动选择。SDK 和上层服务对底层无感。

## 安全考量

### 跨用户读

OKF 不暴露独立 ACL：所有读端点都先解析「调用者能看到的 bundle 列表」，所有查询都按 `bundle_id IN (...)` 过滤。**一个用户不可能通过 OKF 读到他看不到的 public directory 内容。**

测试覆盖：

- `internal/handler/okf_test.go` 的 ACL 用例
- `sdk/tests/test_wiki.py` 的跨用户隔离用例

### Writer 只 API Key

`pdWrite` 中间件确保只有 API Key 能写知识库：

- 应用 Agent 拿 API Key → 可推知识
- 普通用户 JWT → 不能写知识库（防止前端误改）
- 管理员 JWT → 也不能直接写（必须签发 API Key 再用）

API Key 通过 `POST /v1/disk/admin/api-keys` 创建，绑定到具体 public directory。

### 死链扫描的副作用

扫描会修改 `has_broken_link` 列和 `disk_okf_edge.dst_exists` 列。扫描在事务内原子完成，不会半路崩溃留下中间状态。

### Rebuild-graph 的资源开销

`rebuild-graph` 是 O(N) 操作：

- 锁住 bundle（`ErrOkfLockHeld` 时返回 409）
- 遍历 OSS 内所有 .md
- 全量重建 node + edge 表

> **不要把它当 API 用** —— 它是排障工具。日常用 `refresh` 即可。

## 与 SDK 的对应

Python SDK 的 `agentdisk.api.wiki` 模块覆盖了 OKF v0.1 的全部读 / 写接口：

- 同步 / 异步双客户端
- `register_bundle` / `list_bundles` / `get_bundle` / `refresh_bundle` / `unregister_bundle`
- `write_markdown`（writer）
- `list_nodes` / `aggregate_types` / `search`
- `neighbors` / `reachable` / `shortest_path` / `subgraph` / `stats`
- `scan_bundle` / `list_broken_links` / `regenerate_index`

SDK **不**暴露 `rebuild-graph`：那是管理员专用工具，超出 SDK auth 模型。运维请通过 `curl` + admin JWT 调用。

详见 `sdk/src/agentdisk/api/wiki.py` 和 `docs/site/api/okf.md`。
