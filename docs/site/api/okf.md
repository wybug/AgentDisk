# OKF Wiki 接口

> Open Knowledge Format v0.1 — bundle 生命周期、节点索引、图谱查询与死链维护。
> OKF bundle 寄生在 public directory 之上：先注册 public directory 为 bundle，再通过 writer 写入 markdown 节点。

OKF 共 18 个端点，分为五组：

- **Bundle 生命周期**：注册 / 列出 / 详情 / 刷新 / 注销（5 个）
- **节点读写**：写入 markdown、列出节点、类型聚合、全文检索（4 个）
- **图谱查询**：邻居 / 多跳可达 / 最短路径 / 子图 / 统计（5 个）
- **维护**：死链扫描 / 死链列表 / 重建索引 / 重建图谱（4 个）

## 数据模型

### OkfBundle

Bundle 是一个注册到 public directory 的知识图谱容器。

| 字段 | 类型 | 说明 |
|------|------|------|
| `bundleId` | `uint64` | Bundle ID |
| `publicDirectoryId` | `uint64` | 关联的 public directory ID |
| `okfVersion` | `string` | OKF 协议版本（取自根 `index.md` 的 frontmatter） |
| `rootIndexFileId` | `uint64` | 根 `index.md` 的文件行 ID；首次刷新前为 0 |
| `title` | `string` | 标题，取自根 `index.md` |
| `description` | `string` | 描述，取自根 `index.md` |
| `status` | `string` | `active` / `archived` |
| `nodeCount` | `uint32` | bundle 内节点数（冗余计数） |
| `edgeCount` | `uint32` | bundle 内边数（冗余计数） |
| `createdAt` / `updatedAt` | `string` | 时间戳（RFC 3339） |

### OkfNode

节点对应一个 markdown 文件。

| 字段 | 类型 | 说明 |
|------|------|------|
| `nodeId` | `uint64` | 节点 ID |
| `bundleId` | `uint64` | 所属 bundle |
| `fileId` | `uint64` | 底层 public directory 文件行 |
| `relPath` | `string` | bundle 内相对路径，如 `concepts/gemma.md` |
| `type` | `string` | frontmatter 中的 `type` 字段（必填） |
| `title` | `string` | frontmatter `title` |
| `description` | `string` | frontmatter `description` |
| `tags` | `string[]` | frontmatter `tags`，永不为 null |
| `timestamp` | `string` | frontmatter `timestamp`（自由格式） |
| `hasBrokenLink` | `bool` | 节点存在死链时为 true，可用于过滤 |
| `extra` | `object` | frontmatter 未知字段兜底（OKF §9 前向兼容） |
| `contentHash` | `string` | 文件内容 SHA-256 |
| `createdAt` / `updatedAt` | `string` | 时间戳 |

### OkfEdge

边表示节点间的有向链接。

| 字段 | 类型 | 说明 |
|------|------|------|
| `edgeId` | `uint64` | 边 ID |
| `srcNodeId` | `uint64` | 源节点 |
| `dstNodeId` | `uint64` | 目标节点；`dstExists=false` 时为 0 |
| `dstRelPath` | `string` | 目标相对路径（可能不存在） |
| `linkText` | `string` | 链接显示文字 |
| `srcLine` | `int` | 链接在源文件中的行号（1-based） |
| `linkKind` | `string` | `bundle` / `external` / `anchor` |
| `dstExists` | `bool` | 目标不存在时为 false（死链） |

---

## Bundle 生命周期

### 注册 Bundle

将一个 public directory 注册为 OKF bundle。该 directory 根目录必须已存在 `index.md` 且 frontmatter 中包含 `okf_version` 字段。

```
POST /v1/disk/okf/bundles/register
```

#### 认证方式

JWT Bearer Token 或 API Key（HybridAuth）。

#### 请求体

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `publicDirectoryId` | `uint64` | 是 | 要注册的 public directory ID |

```json
{
  "publicDirectoryId": 42
}
```

#### 响应示例

**HTTP 201 Created**

```json
{
  "code": 0,
  "message": "created",
  "data": {
    "bundleId": 1,
    "publicDirectoryId": 42,
    "okfVersion": "0.1",
    "rootIndexFileId": 1001,
    "title": "Gemma Knowledge Base",
    "description": "Internal wiki for the Gemma project.",
    "status": "active",
    "nodeCount": 1,
    "edgeCount": 0,
    "createdAt": "2026-07-01T08:00:00Z",
    "updatedAt": "2026-07-01T08:00:00Z"
  }
}
```

#### curl 示例

```bash
curl -X POST http://localhost:9100/v1/disk/okf/bundles/register \
  -H "Authorization: Bearer eyJhbGciOiJIUzI1NiIs..." \
  -H "Content-Type: application/json" \
  -d '{"publicDirectoryId": 42}'
```

#### 错误场景

| HTTP 状态码 | 场景说明 |
|------------|---------|
| 400 | 缺少 `publicDirectoryId`，或根目录缺少 `index.md` / frontmatter 缺少 `okf_version` |
| 401 | Token / API Key 无效 |

---

### 列出 Bundle

返回当前调用者可见的所有 bundle（基于 public directory 可见性）。

```
GET /v1/disk/okf/bundles
```

#### 认证方式

JWT Bearer Token 或 API Key（HybridAuth）。

#### 请求参数

无。

#### 响应示例

**HTTP 200 OK**

```json
{
  "code": 0,
  "message": "success",
  "data": [
    {
      "bundleId": 1,
      "publicDirectoryId": 42,
      "okfVersion": "0.1",
      "title": "Gemma Knowledge Base",
      "status": "active",
      "nodeCount": 124,
      "edgeCount": 311
    }
  ]
}
```

#### curl 示例

```bash
curl http://localhost:9100/v1/disk/okf/bundles \
  -H "Authorization: Bearer eyJhbGciOiJIUzI1NiIs..."
```

---

### 获取 Bundle 详情

```
GET /v1/disk/okf/bundles/{id}
```

#### 认证方式

JWT Bearer Token 或 API Key（HybridAuth）。

#### 路径参数

| 参数 | 类型 | 说明 |
|------|------|------|
| `id` | `uint64` | Bundle ID |

#### 响应示例

**HTTP 200 OK**

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "bundleId": 1,
    "publicDirectoryId": 42,
    "okfVersion": "0.1",
    "rootIndexFileId": 1001,
    "title": "Gemma Knowledge Base",
    "description": "Internal wiki.",
    "status": "active",
    "nodeCount": 124,
    "edgeCount": 311,
    "createdAt": "2026-07-01T08:00:00Z",
    "updatedAt": "2026-07-01T09:30:00Z"
  }
}
```

#### curl 示例

```bash
curl http://localhost:9100/v1/disk/okf/bundles/1 \
  -H "Authorization: Bearer eyJhbGciOiJIUzI1NiIs..."
```

#### 错误场景

| HTTP 状态码 | 场景说明 |
|------------|---------|
| 403 | 调用者看不到关联的 public directory |
| 404 | Bundle 不存在 |

---

### 刷新 Bundle

重新扫描 bundle 的 public directory，重建物化节点索引。批量导入 markdown 文件（绕过 OKF writer）后调用。

```
POST /v1/disk/okf/bundles/{id}/refresh
```

#### 认证方式

JWT Bearer Token 或 API Key（HybridAuth）。

#### 响应示例

**HTTP 200 OK**（响应体同 [获取 Bundle 详情](#获取-bundle-详情)）

#### curl 示例

```bash
curl -X POST http://localhost:9100/v1/disk/okf/bundles/1/refresh \
  -H "Authorization: Bearer eyJhbGciOiJIUzI1NiIs..."
```

#### 错误场景

| HTTP 状态码 | 场景说明 |
|------------|---------|
| 403 | 调用者看不到关联的 public directory |
| 404 | Bundle 不存在 |

---

### 注销 Bundle

移除 bundle 注册和节点索引。**底层 public directory 和文件不会被删除**，后续可以重新注册。

```
DELETE /v1/disk/okf/bundles/{id}
```

#### 认证方式

JWT Bearer Token 或 API Key（HybridAuth）。

#### 响应示例

**HTTP 200 OK**

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "message": "bundle unregistered"
  }
}
```

#### curl 示例

```bash
curl -X DELETE http://localhost:9100/v1/disk/okf/bundles/1 \
  -H "Authorization: Bearer eyJhbGciOiJIUzI1NiIs..."
```

#### 错误场景

| HTTP 状态码 | 场景说明 |
|------------|---------|
| 403 | 调用者看不到关联的 public directory |
| 404 | Bundle 不存在 |

---

## 节点读写

### 写入 Markdown（writer）

将 markdown 写入 public directory 并自动更新 OKF 物化索引。

> ⚠ 仅 API Key 可调用。pdWrite 中间件会拒绝 JWT 调用方。

```
POST /v1/disk/public-directories/{id}/files/content
```

#### 认证方式

**仅** API Key（请求头 `X-API-Key`）。

#### 路径参数

| 参数 | 类型 | 说明 |
|------|------|------|
| `id` | `uint64` | Public directory ID |

#### 请求体

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `relPath` | `string` | 是 | bundle 内相对路径，必须以 `.md` 结尾 |
| `content` | `string` | 是 | 原始 markdown 字节（UTF-8） |
| `contentType` | `string` | 否 | 默认 `text/markdown; charset=utf-8`，目前仅接受 markdown |

```json
{
  "relPath": "concepts/gemma.md",
  "content": "---\ntype: concept\ntitle: Gemma\ntags: [model, google]\n---\n\n# Gemma\n\n[[persons/deepmind]] 是开发商。\n",
  "contentType": "text/markdown; charset=utf-8"
}
```

#### 响应示例

**HTTP 201 Created**

返回新物化的 OkfNode（节选）：

```json
{
  "code": 0,
  "message": "created",
  "data": {
    "nodeId": 501,
    "bundleId": 1,
    "fileId": 2048,
    "relPath": "concepts/gemma.md",
    "type": "concept",
    "title": "Gemma",
    "tags": ["model", "google"],
    "hasBrokenLink": true,
    "contentHash": "sha256:...",
    "createdAt": "2026-07-01T10:00:00Z",
    "updatedAt": "2026-07-01T10:00:00Z"
  }
}
```

#### curl 示例

```bash
curl -X POST http://localhost:9100/v1/disk/public-directories/42/files/content \
  -H "X-API-Key: ak_live_xxxxxxxx" \
  -H "Content-Type: application/json" \
  -d '{
    "relPath": "concepts/gemma.md",
    "content": "---\ntype: concept\ntitle: Gemma\n---\n\n# Gemma\n"
  }'
```

#### 错误场景

| HTTP 状态码 | 场景说明 |
|------------|---------|
| 400 | 缺少 `relPath` / `content`；非 markdown contentType；frontmatter 缺少 `type` 字段；使用保留名（`log.md`） |
| 401 | 缺少 `X-API-Key` 请求头 |
| 403 | 调用方使用 JWT（被 pdWrite 拒绝） |

---

### 列出节点

返回 bundle 内的节点，可选按 type 或 tag 过滤。响应是 `{"nodes": [...]}` 对象信封（不是裸数组）。

```
GET /v1/disk/okf/bundles/{id}/nodes
```

#### 认证方式

JWT Bearer Token 或 API Key（HybridAuth）。

#### 查询参数

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `type` | `string` | 否 | 按 OKF type 精确匹配 |
| `tag` | `string` | 否 | 按标签精确匹配（节点必须包含该 tag） |

#### 响应示例

**HTTP 200 OK**

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "nodes": [
      {
        "nodeId": 501,
        "bundleId": 1,
        "relPath": "concepts/gemma.md",
        "type": "concept",
        "title": "Gemma",
        "tags": ["model", "google"],
        "hasBrokenLink": false
      }
    ]
  }
}
```

#### curl 示例

```bash
# 列出 bundle 1 的全部 concept 节点
curl "http://localhost:9100/v1/disk/okf/bundles/1/nodes?type=concept" \
  -H "Authorization: Bearer eyJhbGciOiJIUzI1NiIs..."

# 列出含 google 标签的节点
curl "http://localhost:9100/v1/disk/okf/bundles/1/nodes?tag=google" \
  -H "Authorization: Bearer eyJhbGciOiJIUzI1NiIs..."
```

#### 错误场景

| HTTP 状态码 | 场景说明 |
|------------|---------|
| 403 | 调用者看不到关联的 public directory |
| 404 | Bundle 不存在 |

---

### 类型聚合

跨所有可见 bundle 按 OKF type 汇总节点数，返回 `[{type, count}]` 数组（按 count 降序、type 升序）。

```
GET /v1/disk/okf/types
```

#### 认证方式

JWT Bearer Token 或 API Key（HybridAuth）。

#### 响应示例

**HTTP 200 OK**

```json
{
  "code": 0,
  "message": "success",
  "data": [
    {"type": "concept", "count": 58},
    {"type": "person", "count": 31},
    {"type": "index", "count": 3}
  ]
}
```

#### curl 示例

```bash
curl http://localhost:9100/v1/disk/okf/types \
  -H "Authorization: Bearer eyJhbGciOiJIUzI1NiIs..."
```

---

### 全文检索

对节点标题和描述做全文检索（SQLite FTS5），按游标分页。搜索范围自动受调用者可见性过滤。

```
POST /v1/disk/okf/search
```

#### 认证方式

JWT Bearer Token 或 API Key（HybridAuth）。

#### 请求体

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `query` | `string` | 是 | 全文检索词 |
| `bundleId` | `uint64` | 否 | 限定单个 bundle；省略则搜索所有可见 bundle |
| `type` | `string` | 否 | OKF type 过滤 |
| `limit` | `int` | 否 | 页大小，默认 20，最大 100 |
| `cursor` | `uint64` | 否 | 续页游标，从上次响应的 `nextCursor` 取 |

```json
{
  "query": "transformer attention",
  "bundleId": 1,
  "limit": 20
}
```

#### 响应示例

**HTTP 200 OK**

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "nodes": [
      {
        "nodeId": 501,
        "bundleId": 1,
        "relPath": "concepts/attention.md",
        "type": "concept",
        "title": "Attention Mechanism",
        "description": "Scaled dot-product attention used in transformers."
      }
    ],
    "nextCursor": 0
  }
}
```

> `nextCursor` 为 0 表示已到末页。

#### curl 示例

```bash
curl -X POST http://localhost:9100/v1/disk/okf/search \
  -H "Authorization: Bearer eyJhbGciOiJIUzI1NiIs..." \
  -H "Content-Type: application/json" \
  -d '{"query": "transformer attention"}'
```

#### 错误场景

| HTTP 状态码 | 场景说明 |
|------------|---------|
| 400 | 缺少 `query` |

---

## 图谱查询

### 邻居

返回节点的 1 跳邻居（节点 + 边）。

```
GET /v1/disk/okf/nodes/{id}/neighbors
```

#### 认证方式

JWT Bearer Token 或 API Key（HybridAuth）。

#### 路径参数

| 参数 | 类型 | 说明 |
|------|------|------|
| `id` | `uint64` | 节点 ID |

#### 查询参数

| 参数 | 类型 | 默认 | 说明 |
|------|------|------|------|
| `dir` | `string` | `out` | 方向：`out` / `in` / `both` |
| `type` | `string` | — | 仅返回该 OKF type 的邻居 |

#### 响应示例

**HTTP 200 OK**

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "nodes": [
      {
        "nodeId": 502,
        "bundleId": 1,
        "relPath": "persons/deepmind.md",
        "type": "person",
        "title": "DeepMind"
      }
    ],
    "edges": [
      {
        "edgeId": 9001,
        "srcNodeId": 501,
        "dstNodeId": 502,
        "dstRelPath": "persons/deepmind.md",
        "linkText": "DeepMind",
        "srcLine": 7,
        "linkKind": "bundle",
        "dstExists": true
      }
    ]
  }
}
```

#### curl 示例

```bash
curl "http://localhost:9100/v1/disk/okf/nodes/501/neighbors?dir=out" \
  -H "Authorization: Bearer eyJhbGciOiJIUzI1NiIs..."
```

#### 错误场景

| HTTP 状态码 | 场景说明 |
|------------|---------|
| 400 | `dir` 取值不合法 |
| 404 | 节点不存在 |

---

### 多跳可达

从起始节点跑 N 跳 BFS，返回所有可达节点（不含起点）。

```
POST /v1/disk/okf/nodes/{id}/reachable
```

#### 认证方式

JWT Bearer Token 或 API Key（HybridAuth）。

#### 路径参数

| 参数 | 类型 | 说明 |
|------|------|------|
| `id` | `uint64` | 起始节点 ID |

#### 请求体

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `depth` | `int` | 是 | 最大 BFS 跳数，最小 1；服务端会钳到 `MaxBFSDepth` |
| `types` | `string[]` | 否 | 仅遍历这些 type 的邻居 |
| `maxNodes` | `int` | 否 | 返回节点数上限，0 表示不限制 |
| `direction` | `string` | 否 | `out` / `in` / `both`，默认 `out` |

```json
{
  "depth": 3,
  "direction": "out",
  "types": ["concept", "person"]
}
```

#### 响应示例

**HTTP 200 OK**

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "nodes": [
      {"nodeId": 502, "type": "person", "title": "DeepMind"},
      {"nodeId": 503, "type": "concept", "title": "Transformer"}
    ]
  }
}
```

#### curl 示例

```bash
curl -X POST http://localhost:9100/v1/disk/okf/nodes/501/reachable \
  -H "Authorization: Bearer eyJhbGciOiJIUzI1NiIs..." \
  -H "Content-Type: application/json" \
  -d '{"depth": 3}'
```

#### 错误场景

| HTTP 状态码 | 场景说明 |
|------------|---------|
| 404 | 节点不存在 |

---

### 最短路径

跑双向 BFS，返回 src 到 dst 的有序节点路径。

```
POST /v1/disk/okf/paths/shortest
```

#### 认证方式

JWT Bearer Token 或 API Key（HybridAuth）。

#### 请求体

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `src` | `uint64` | 是 | 源节点 ID |
| `dst` | `uint64` | 是 | 目标节点 ID |
| `maxDepth` | `int` | 否 | 深度预算，默认与上限均为 `MaxBFSDepth` |

```json
{
  "src": 501,
  "dst": 503,
  "maxDepth": 6
}
```

#### 响应示例

**HTTP 200 OK**（找到路径）

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "path": [
      {"nodeId": 501, "title": "Gemma"},
      {"nodeId": 502, "title": "DeepMind"},
      {"nodeId": 503, "title": "Transformer"}
    ],
    "found": true
  }
}
```

**HTTP 200 OK**（深度预算内无路径）

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "path": [],
    "found": false
  }
}
```

> 注意：找不到路径也是 200，不是 404。

#### curl 示例

```bash
curl -X POST http://localhost:9100/v1/disk/okf/paths/shortest \
  -H "Authorization: Bearer eyJhbGciOiJIUzI1NiIs..." \
  -H "Content-Type: application/json" \
  -d '{"src": 501, "dst": 503}'
```

#### 错误场景

| HTTP 状态码 | 场景说明 |
|------------|---------|
| 400 | 缺少 `src` 或 `dst` |

---

### 子图

返回 bundle 的一个切片，按 type 过滤，用于前端图谱可视化。

```
POST /v1/disk/okf/subgraph
```

#### 认证方式

JWT Bearer Token 或 API Key（HybridAuth）。

#### 请求体

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `bundleId` | `uint64` | 是 | Bundle ID |
| `types` | `string[]` | 否 | type 白名单，省略则包含全部 |
| `maxNodes` | `int` | 否 | 返回节点数上限 |

```json
{
  "bundleId": 1,
  "types": ["concept", "person"]
}
```

#### 响应示例

**HTTP 200 OK**

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "nodes": [
      {"nodeId": 501, "type": "concept", "title": "Gemma"},
      {"nodeId": 502, "type": "person", "title": "DeepMind"}
    ],
    "edges": [
      {"edgeId": 9001, "srcNodeId": 501, "dstNodeId": 502, "linkKind": "bundle", "dstExists": true}
    ]
  }
}
```

#### curl 示例

```bash
curl -X POST http://localhost:9100/v1/disk/okf/subgraph \
  -H "Authorization: Bearer eyJhbGciOiJIUzI1NiIs..." \
  -H "Content-Type: application/json" \
  -d '{"bundleId": 1}'
```

#### 错误场景

| HTTP 状态码 | 场景说明 |
|------------|---------|
| 403 | 调用者看不到关联的 public directory |
| 404 | Bundle 不存在 |

---

### 统计

返回 bundle 的图统计：节点总数、边总数（live + broken）、按 type 分组。

```
GET /v1/disk/okf/bundles/{id}/stats
```

#### 认证方式

JWT Bearer Token 或 API Key（HybridAuth）。

#### 响应示例

**HTTP 200 OK**

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "nodeCount": 124,
    "edgeTotal": 320,
    "edgeLive": 311,
    "edgeBroken": 9,
    "types": [
      {"type": "concept", "count": 58},
      {"type": "person", "count": 31}
    ]
  }
}
```

#### curl 示例

```bash
curl http://localhost:9100/v1/disk/okf/bundles/1/stats \
  -H "Authorization: Bearer eyJhbGciOiJIUzI1NiIs..."
```

#### 错误场景

| HTTP 状态码 | 场景说明 |
|------------|---------|
| 403 | 调用者看不到关联的 public directory |
| 404 | Bundle 不存在 |

---

## 维护

### 扫描死链

遍历 bundle 内所有节点的出链，按 bundle 相对路径解析目标，缺失的标记为死链并写入 `has_broken_link`。

```
POST /v1/disk/okf/bundles/{id}/scan
```

#### 认证方式

JWT Bearer Token 或 API Key（HybridAuth）。

#### 响应示例

**HTTP 200 OK**

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "scannedNodes": 124,
    "brokenCount": 9,
    "scannedAt": "2026-07-01T11:00:00Z"
  }
}
```

#### curl 示例

```bash
curl -X POST http://localhost:9100/v1/disk/okf/bundles/1/scan \
  -H "Authorization: Bearer eyJhbGciOiJIUzI1NiIs..."
```

#### 错误场景

| HTTP 状态码 | 场景说明 |
|------------|---------|
| 403 | 调用者看不到关联的 public directory |
| 404 | Bundle 不存在 |

---

### 死链列表

分页返回 bundle 内已记录的死链。`cursor` 是上一页最后一条记录的源节点 ID；`nextCursor=0` 表示末页。

```
GET /v1/disk/okf/bundles/{id}/broken-links
```

#### 认证方式

JWT Bearer Token 或 API Key（HybridAuth）。

#### 查询参数

| 参数 | 类型 | 默认 | 说明 |
|------|------|------|------|
| `cursor` | `uint64` | 0 | 续页游标 |
| `limit` | `int` | 50 | 页大小，最大 50 |

#### 响应示例

**HTTP 200 OK**

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "brokenLinks": [
      {
        "srcNodeId": 501,
        "srcRelPath": "concepts/gemma.md",
        "dstRelPath": "persons/deepmind.md",
        "srcLine": 7,
        "linkText": "DeepMind",
        "linkKind": "bundle",
        "reason": "target missing"
      }
    ],
    "nextCursor": 0
  }
}
```

#### curl 示例

```bash
curl "http://localhost:9100/v1/disk/okf/bundles/1/broken-links?limit=20" \
  -H "Authorization: Bearer eyJhbGciOiJIUzI1NiIs..."
```

#### 错误场景

| HTTP 状态码 | 场景说明 |
|------------|---------|
| 403 | 调用者看不到关联的 public directory |
| 404 | Bundle 不存在 |

---

### 重建索引

按当前节点集合重新生成 bundle 的根 `index.md` 并写入 OSS。

```
POST /v1/disk/okf/bundles/{id}/regenerate-index
```

#### 认证方式

JWT Bearer Token 或 API Key（HybridAuth）。

#### 响应示例

**HTTP 200 OK**

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "indexVersion": 4,
    "regeneratedAt": "2026-07-01T12:00:00Z"
  }
}
```

#### curl 示例

```bash
curl -X POST http://localhost:9100/v1/disk/okf/bundles/1/regenerate-index \
  -H "Authorization: Bearer eyJhbGciOiJIUzI1NiIs..."
```

#### 错误场景

| HTTP 状态码 | 场景说明 |
|------------|---------|
| 403 | 调用者看不到关联的 public directory |
| 404 | Bundle 不存在 |

---

### 重建图谱（admin-only）

运维逃生通道：根据 public directory 当前内容重新派生 bundle 内所有节点和边。

> ⚠ 仅管理员 JWT 可调用。SDK 不暴露此方法（admin 模型超出 SDK auth 范围）。普通 HybridAuth 调用方应使用 [刷新 Bundle](#刷新-bundle)，行为等价但走用户态鉴权。

```
POST /v1/disk/admin/okf/bundles/{id}/rebuild-graph
```

#### 认证方式

管理员 JWT（`AdminJWT`），通过 `POST /v1/disk/admin/login` 获取。

#### 响应示例

**HTTP 200 OK**

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "bundleId": 1,
    "nodeCount": 124,
    "edgeCount": 311
  }
}
```

#### curl 示例

```bash
curl -X POST http://localhost:9100/v1/disk/admin/okf/bundles/1/rebuild-graph \
  -H "Authorization: Bearer <admin-jwt>"
```

#### 错误场景

| HTTP 状态码 | 场景说明 |
|------------|---------|
| 401 | 缺少管理员 JWT 或 JWT 无效 |
| 403 | 调用者不是管理员 |
| 404 | Bundle 不存在 |

---

## 错误码汇总

所有 OKF 端点共用以下错误响应格式：

```json
{
  "code": 4xx,
  "message": "human-readable error",
  "data": null
}
```

| HTTP 状态码 | 触发场景 | 通用处理建议 |
|------------|---------|-------------|
| 400 | 请求体不合法、缺少必填字段、frontmatter 缺少 `type`、`dir` 取值非法 | 客户端修正后重试，不重试相同请求 |
| 401 | Token / API Key 缺失或失效 | 重新登录或更换 API Key |
| 403 | 调用者看不到目标 bundle（受 public directory ACL 约束） | 检查权限，不重试 |
| 404 | Bundle / 节点不存在 | 不重试 |
| 409 | Bundle 锁被其他 writer 持有（写操作） | 退避后重试 |
| 500 | 服务端内部错误（不应泄露详情） | 联系运维，附带 trace ID |

### 鉴权速查

| 端点 | 鉴权 |
|------|------|
| 所有 `/v1/disk/okf/*` reader / 维护端点 | JWT **或** API Key（HybridAuth） |
| `/v1/disk/public-directories/:id/files/content`（writer） | **仅** API Key |
| `/v1/disk/admin/okf/bundles/:id/rebuild-graph` | 管理员 JWT |

API Key 通过 `POST /v1/disk/admin/api-keys` 创建，请求头 `X-API-Key` 传递。
