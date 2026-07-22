P5b — OKF 文档补全（OpenAPI + VitePress）                 

 Context

 P1–P3 已落地 OKF v0.1 全部 server 侧能力（18 端点：bundle lifecycle、节点检索、type
 聚合、死链扫描、全文检索、图谱查询、admin rebuild-graph、markdown writer），P5a 把这些端点封装进了
 Python SDK。

 但文档侧零覆盖：
 - grep -ri okf docs/ 在 docs/openapi.yaml 和 docs/site/ 都返回空。docs/openapi.yaml 30KB 只覆盖
 folders/files/shares/permissions/tags/versions/recycle/admin，OKF 一个 path 都没有。
 - VitePress 站点（docs/site/）同样零命中，sidebar 没有入口，落地页 index.md 不提 OKF。
 - 应用 Agent 接入 OKF 时，除了看 SDK 源码 + 直接读 internal/handler/okf*.go，没有任何参考。

 P5b 纯文档增量：补全 OpenAPI paths + schemas、新增 VitePress API 参考 + 架构页、把 OKF 加进 sidebar
  和落地页。不动任何代码、配置、schema。完成后外部 Agent / 集成方可以直接读 docs/openapi.yaml
 生成客户端、或浏览 VitePress 站点学习 OKF。

 用户已确认决策（本轮沿用 P5a）

 ┌───────────────────┬──────────────────────────────────────────────────────────────────────────┐
 │      决策点       │                                 选定方案                                 │
 ├───────────────────┼──────────────────────────────────────────────────────────────────────────┤
 │ 子 PR 顺序        │ P5b 文档补全（最自包含、纯文档、不动代码）                               │
 ├───────────────────┼──────────────────────────────────────────────────────────────────────────┤
 │ Worktree 策略     │ 新 worktree okf-p5-docs off feature/okf-integration（CLAUDE.md §4.1）    │
 ├───────────────────┼──────────────────────────────────────────────────────────────────────────┤
 │ Admin             │ 覆盖（OpenAPI 标 AdminJWT security、VitePress 文档说明"超出 SDK auth     │
 │ rebuild-graph     │ 模型"）                                                                  │
 ├───────────────────┼──────────────────────────────────────────────────────────────────────────┤
 │ 文档拆分粒度      │ 单个 okf.md + 单个 architecture/okf.md（参照 public-directory            │
 │                   │ 模式，避免文件爆炸）                                                     │
 └───────────────────┴──────────────────────────────────────────────────────────────────────────┘

 现状盘点（已通过 Explore agent 确认）

 OpenAPI 现状（docs/openapi.yaml）：
 - 30KB，OpenAPI 3.0 风格，全局 BearerAuth 在 root，path 级用 security: [] 覆盖。
 - Response 信封统一 {"code": int, "message": string, "data": any}，通过 allOf + $ref 复用
 ResponseBase。
 - Path 命名 /v1/disk/{resource}/...，复数名词，camelCase JSON 字段。
 - 现有 path：/auth/login、/folders/*、/files/*、/shares/*、/tags/*、/permissions/*、/admin/*。

 VitePress 现状（docs/site/）：
 - 配置 docs/site/.vitepress/config.ts，sidebar 按 /guide/、/api/、/integration/、/architecture/
 四组分。
 - docs/site/api/*.md 一文件一资源（folders.md、files.md、shares.md、permissions.md 等），每个 H2
 一个端点，含「认证方式 / 请求体 / 响应示例 / curl / 错误场景」五块。
 - docs/site/api/index.md 是 API 总览，列所有 API 文件链接。
 - docs/site/guide/public-directories.md + docs/site/architecture/public-directory.md 是 public dir
 的双层文档（用户指南 + 架构深度），OKF 完全可以复用这个模式。
 - docs/site/index.md 是 VitePress home，有 hero + features 卡片网格。

 OKF 端点盘点（18 条，来自 internal/router/router.go:236,319-355）：

 ┌─────┬────────────────────────────────────────────┬──────────────────────────────┬───────────┐
 │  #  │               Method + Path                │           Handler            │   Auth    │
 ├─────┼────────────────────────────────────────────┼──────────────────────────────┼───────────┤
 │ 1   │ POST /v1/disk/okf/bundles/register         │ okf.go:52 RegisterBundle     │ HybridAut │
 │     │                                            │                              │ h         │
 ├─────┼────────────────────────────────────────────┼──────────────────────────────┼───────────┤
 │ 2   │ GET /v1/disk/okf/bundles                   │ okf.go:70 ListBundles        │ HybridAut │
 │     │                                            │                              │ h         │
 ├─────┼────────────────────────────────────────────┼──────────────────────────────┼───────────┤
 │ 3   │ GET /v1/disk/okf/bundles/:id               │ okf.go:85 GetBundle          │ HybridAut │
 │     │                                            │                              │ h         │
 ├─────┼────────────────────────────────────────────┼──────────────────────────────┼───────────┤
 │ 4   │ GET /v1/disk/okf/bundles/:id/nodes         │ okf.go:109 ListNodes         │ HybridAut │
 │     │                                            │                              │ h         │
 ├─────┼────────────────────────────────────────────┼──────────────────────────────┼───────────┤
 │ 5   │ GET /v1/disk/okf/types                     │ okf.go:129 AggregateTypes    │ HybridAut │
 │     │                                            │                              │ h         │
 ├─────┼────────────────────────────────────────────┼──────────────────────────────┼───────────┤
 │ 6   │ POST /v1/disk/okf/bundles/:id/refresh      │ okf.go:177 RefreshBundle     │ HybridAut │
 │     │                                            │                              │ h         │
 ├─────┼────────────────────────────────────────────┼──────────────────────────────┼───────────┤
 │ 7   │ DELETE /v1/disk/okf/bundles/:id            │ okf.go:194 UnregisterBundle  │ HybridAut │
 │     │                                            │                              │ h         │
 ├─────┼────────────────────────────────────────────┼──────────────────────────────┼───────────┤
 │ 8   │ POST /v1/disk/okf/search                   │ okf_search.go:19 Search      │ HybridAut │
 │     │                                            │                              │ h         │
 ├─────┼────────────────────────────────────────────┼──────────────────────────────┼───────────┤
 │ 9   │ GET /v1/disk/okf/nodes/:id/neighbors       │ okf_graph.go:37 Neighbors    │ HybridAut │
 │     │                                            │                              │ h         │
 ├─────┼────────────────────────────────────────────┼──────────────────────────────┼───────────┤
 │ 10  │ POST /v1/disk/okf/nodes/:id/reachable      │ okf_graph.go:65 Reachable    │ HybridAut │
 │     │                                            │                              │ h         │
 ├─────┼────────────────────────────────────────────┼──────────────────────────────┼───────────┤
 │ 11  │ POST /v1/disk/okf/paths/shortest           │ okf_graph.go:96 ShortestPath │ HybridAut │
 │     │                                            │                              │ h         │
 ├─────┼────────────────────────────────────────────┼──────────────────────────────┼───────────┤
 │ 12  │ POST /v1/disk/okf/subgraph                 │ okf_graph.go:125 Subgraph    │ HybridAut │
 │     │                                            │                              │ h         │
 ├─────┼────────────────────────────────────────────┼──────────────────────────────┼───────────┤
 │ 13  │ GET /v1/disk/okf/bundles/:id/stats         │ okf_graph.go:153 Stats       │ HybridAut │
 │     │                                            │                              │ h         │
 ├─────┼────────────────────────────────────────────┼──────────────────────────────┼───────────┤
 │ 14  │ POST /v1/disk/okf/bundles/:id/scan         │ okf_scan.go:35 ScanBundle    │ HybridAut │
 │     │                                            │                              │ h         │
 ├─────┼────────────────────────────────────────────┼──────────────────────────────┼───────────┤
 │ 15  │ GET /v1/disk/okf/bundles/:id/broken-links  │ okf_scan.go:64               │ HybridAut │
 │     │                                            │ ListBrokenLinks              │ h         │
 ├─────┼────────────────────────────────────────────┼──────────────────────────────┼───────────┤
 │ 16  │ POST                                       │ okf_scan.go:95               │ HybridAut │
 │     │ /v1/disk/okf/bundles/:id/regenerate-index  │ RegenerateIndex              │ h         │
 ├─────┼────────────────────────────────────────────┼──────────────────────────────┼───────────┤
 │ 17  │ POST /v1/disk/admin/okf/bundles/:id/rebuil │ okf_graph.go:188             │ AdminJWT  │
 │     │ d-graph                                    │ RebuildGraph                 │           │
 ├─────┼────────────────────────────────────────────┼──────────────────────────────┼───────────┤
 │ 18  │ POST /v1/disk/public-directories/:id/files │ public_directory_content.go: │ APIKey    │
 │     │ /content                                   │ 48 WriteContent              │           │
 └─────┴────────────────────────────────────────────┴──────────────────────────────┴───────────┘

 关键响应形状（来自 P5a SDK 实现 sdk/src/agentdisk/api/wiki.py）：
 - list_nodes 返回 {"nodes": [...]}，不是裸数组。
 - aggregate_types 返回裸数组 [{"type": "...", "count": N}]。
 - search 返回 {"nodes": [...], "nextCursor": N}。
 - shortest_path 返回 {"path": [...], "found": bool}。
 - reachable 返回 {"nodes": [...]}。
 - stats 返回 {"nodeCount": N, "edgeTotal": N, "edgeLive": N, "edgeBroken": N, "types": [...]}。
 - broken-links 返回 {"links": [...], "nextCursor": N}。

 实施步骤

 Step 1 — 新 worktree

 git -C /Users/wangyun/Documents/work/gitlab/agent-disk worktree add \
   .claude/worktrees/okf-p5-docs -b feature/okf-p5-docs feature/okf-integration

 EnterWorktree 进入 .claude/worktrees/okf-p5-docs。

 Step 2 — OpenAPI 补全（docs/openapi.yaml）

 新增 schemas（追加到 components.schemas）：

 OkfBundle:          # bundleId, publicDirectoryId, okfVersion, rootIndexFileId,
                     # title, description, status, nodeCount, edgeCount,
                     # createdAt, updatedAt
 OkfNode:            # nodeId, bundleId, fileId, relPath, type, title, description,
                     # tags[], timestamp, hasBrokenLink, extra{}, contentHash,
                     # linkCount, createdAt, updatedAt
 OkfEdge:            # edgeId, srcNodeId, dstNodeId, dstRelPath, linkText,
                     # srcLine, linkKind, dstExists
 OkfTypeCount:       # type, count
 OkfBrokenLink:      # srcNodeId, srcRelPath, dstRelPath, linkText, srcLine,
                     # linkKind, reason
 OkfScanReport:      # scannedNodes, brokenCount, scannedAt
 OkfIndexRegenResult:# indexVersion, regeneratedAt
 OkfBundleStats:     # nodeCount, edgeTotal, edgeLive, edgeBroken, types[]
 OkfNeighborsResult: # nodes[], edges[]
 OkfShortestPathResult: # path[], found
 OkfSubgraphResult:  # nodes[], edges[]
 OkfSearchRequest:   # query, bundleId, type, limit, cursor
 OkfSearchPage:      # nodes[], nextCursor
 OkfReachableRequest:# depth, types[], maxNodes, direction
 OkfSubgraphRequest: # bundleId, types[], maxNodes
 OkfShortestRequest: # src, dst, maxDepth
 WriteContentRequest:# relPath, content, contentType

 新增 paths（追加到 paths），每条 path 都要：
 1. 用 $ref: '#/components/schemas/ResponseBase' + allOf 包裹 data 字段。
 2. 在 security 字段声明对应鉴权（HybridAuth = BearerAuth 或 ApiKeyAuth；Admin-only =
 AdminJWT；API-key only = ApiKeyAuth）。
 3. 标注错误码（400/403/404/500）。
 4. 给每个 request/response 加 example。

 Security schemes 补全（components.securitySchemes）：
 - BearerAuth: type: http, scheme: bearer（已存在）
 - ApiKeyAuth: type: apiKey, in: header, name: X-API-Key（新增，writer + HybridAuth 都用）
 - AdminJWT: type: http, scheme: bearer, description: "Admin-only JWT, obtained via
 /v1/disk/admin/login"（新增，rebuild-graph 用）

 参照已有 components.schemas.DiskFolder / paths./v1/disk/folders 的格式。

 Step 3 — VitePress API 参考（docs/site/api/okf.md）

 单文件，参照 docs/site/api/shares.md 风格，~600 行。结构：

 # OKF Wiki 接口

 > Open Knowledge Format v0.1 — bundle lifecycle, node index, graph queries.
 > OKF bundles live inside public directories; register one before writing nodes.

 ## 数据模型
 （OkfBundle / OkfNode / OkfEdge 字段表 + 关系图）

 ## Bundle 生命周期

 ### 注册 Bundle
 POST /v1/disk/okf/bundles/register
 ...

 ### 列出 Bundle
 GET /v1/disk/okf/bundles
 ...

 ### 获取 Bundle 详情
 GET /v1/disk/okf/bundles/:id
 ...

 ### 刷新 Bundle
 POST /v1/disk/okf/bundles/:id/refresh
 ...

 ### 注销 Bundle
 DELETE /v1/disk/okf/bundles/:id
 ...

 ## 节点读写

 ### 写入 Markdown（writer）
 POST /v1/disk/public-directories/:id/files/content
 > ⚠ 仅 API Key 可调用（pdWrite 中间件）

 ### 列出节点
 GET /v1/disk/okf/bundles/:id/nodes
 ...

 ### 类型聚合
 GET /v1/disk/okf/types
 ...

 ### 全文检索
 POST /v1/disk/okf/search
 ...

 ## 图谱查询

 ### 邻居
 GET /v1/disk/okf/nodes/:id/neighbors
 ...

 ### 多跳可达
 POST /v1/disk/okf/nodes/:id/reachable
 ...

 ### 最短路径
 POST /v1/disk/okf/paths/shortest
 ...

 ### 子图
 POST /v1/disk/okf/subgraph
 ...

 ### 统计
 GET /v1/disk/okf/bundles/:id/stats
 ...

 ## 维护

 ### 扫描死链
 POST /v1/disk/okf/bundles/:id/scan
 ...

 ### 死链列表
 GET /v1/disk/okf/bundles/:id/broken-links
 ...

 ### 重建索引
 POST /v1/disk/okf/bundles/:id/regenerate-index
 ...

 ### 重建图谱（admin-only）
 POST /v1/disk/admin/okf/bundles/:id/rebuild-graph
 > ⚠ 仅管理员 JWT 可调用；SDK 不暴露此方法（admin 模型超出 SDK auth 范围）

 ## 错误码汇总
 （400/403/404/409/500 → 哪些端点会返回 → 通用处理建议）

 每个端点块包含 5 子段（参照 shares.md）：
 - HTTP 方法 + 路径（code fence）
 认证方式（JWT / API Key / Admin JWT / 都可）

 请求体或参数（字段表）

 响应示例（JSON）

 curl 示例（完整可执行）


 Step 4 — VitePress 架构页（docs/site/architecture/okf.md）

 参照 docs/site/architecture/public-directory.md 风格，~400 行：

 # OKF 架构

 ## OKF v0.1 是什么
 （Open Knowledge Format 起源、bundle / node / edge 三层模型、与 public directory 的关系）

 ## 数据模型
 （OkfBundle 1—N OkfNode 1—N OkfEdge 的 ER 图 + 字段语义）

 ## Frontmatter 规范
 （type 必填、index.md 必须 okf_version、log.md 禁止 frontmatter、tags/extra 字段）

 ## 节点物化（materialization）
 （WriteMarkdown → 解析 frontmatter → OSS 落盘 → 事务内 upsert node + edges + bundle counts）
 （FTS5 虚拟表 + 触发器同步，WAL 模式下读不阻塞写）

 ## 图谱查询
 （BFS / 双向 BFS、Redis adjacency cache、TTL、缓存失效策略）
 （邻居 / 可达 / 最短路径 / 子图分别的算法选择）

 ## 死链管理
 （has_broken_link 标志位、扫描任务、broken-links 分页、regenerate-index 重建根 index.md）

 ## 鉴权模型
 （HybridAuth vs pdWrite vs AdminJWT 三种中间件，分别覆盖哪些端点）
 （API Key 获取：管理员通过 /admin/api-keys 创建，SDK 通过 api_key= 参数传）

 ## 性能与限制
 （10万节点 BFS 性能基准 → P5c 跑；busy_timeout=5000ms、WAL、Redis 缓存）
 （FTS5 vs MySQL FULLTEXT ngram 的差异）

 ## 安全考量
 （bundle 可见性 = underlying public directory 可见性，不能跨用户读）
 （admin rebuild-graph 为何独立路径：避免普通 HybridAuth 调用方触发昂贵重算）

 Step 5 — 接入 VitePress sidebar + landing

 docs/site/.vitepress/config.ts：
 - /api/ sidebar 数组追加两项：
 { text: 'OKF Wiki', items: [
   { text: 'API 参考', link: '/api/okf' },
 ]},
 - /architecture/ sidebar 数组追加：
 { text: 'OKF 架构', link: '/architecture/okf' },

 docs/site/api/index.md（API 总览）：追加 OKF Wiki 卡片链接。

 docs/site/index.md（landing page）：features 数组追加：
 - title: OKF Wiki
   details: Open Knowledge Format v0.1 — 把公共目录注册成知识图谱，支持 bundle / node / edge
 查询、全文检索、死链扫描、图谱 BFS。
   link: /architecture/okf

 Step 6 — 验证

 # 1. OpenAPI 语法校验（用 redocly 或 swagger-cli；本地用 pythonyaml + 引用完整性即可）
 python3 -c "
 import yaml
 with open('docs/openapi.yaml') as f:
     spec = yaml.safe_load(f)
 # 所有 \$ref 都指向已存在的 schema
 import re
 def check(obj, root):
     if isinstance(obj, dict):
         for k, v in obj.items():
             if k == '\$ref' and v.startswith('#/components/schemas/'):
                 name = v.split('/')[-1]
                 assert name in root['components']['schemas'], f'missing schema: {name}'
             else:
                 check(v, root)
     elif isinstance(obj, list):
         for x in obj: check(x, root)
 check(spec, spec)
 print('openapi.yaml: OK')
 "

 # 2. VitePress build（确保新文档能渲染、无死链）
 cd docs/site && npm run build

 # 3. （可选）本地预览
 cd docs/site && npx vitepress dev --port 9102
 # 浏览器打开 http://localhost:9102/api/okf、/architecture/okf，确认页面正常

 # 4. SDK 测试不受影响（无代码改动）
 cd sdk && pytest -v

 # 5. Go 测试不受影响
 cd .. && go test -tags fts5 ./...

 Step 7 — 提交、合并、推送

 # worktree 内
 git add docs/openapi.yaml \
         docs/site/api/okf.md \
         docs/site/architecture/okf.md \
         docs/site/.vitepress/config.ts \
         docs/site/api/index.md \
         docs/site/index.md
 git commit -m "docs(okf): P5b OpenAPI + VitePress coverage"

 # 主 worktree 合并
 git -C /Users/wangyun/Documents/work/gitlab/agent-disk merge --no-ff feature/okf-p5-docs \
   -m "Merge PR-P5b: OKF documentation (OpenAPI + VitePress)"
 git -C /Users/wangyun/Documents/work/gitlab/agent-disk push origin feature/okf-integration

 关键文件清单

 新建：
 - docs/site/api/okf.md（~600 行，18 端点 API 参考）
 - docs/site/architecture/okf.md（~400 行，OKF 架构深度）

 修改：
 - docs/openapi.yaml（追加 18 paths + ~17 schemas + 2 security schemes）
 - docs/site/.vitepress/config.ts（sidebar 加 2 项）
 - docs/site/api/index.md（API 总览加 OKF 卡片）
 - docs/site/index.md（landing 加 OKF feature）

 不动：
 - 任何 Go 代码、SDK 代码、配置、schema（纯文档增量）
 - docs/site/guide/（用户指南，留给后续 PR 或不需要——架构 + API 参考已够）

 DoD

 - docs/openapi.yaml 校验通过（yaml 合法 + 所有 $ref 指向存在的 schema）
 - cd docs/site && npm run build 0 错误
 - 浏览器打开 http://localhost:9102/api/okf 和 /architecture/okf 页面正常渲染
 - 18 个 OKF 端点在 OpenAPI 都有完整 path + request/response schema + example
 - 18 个端点在 docs/site/api/okf.md 都有「认证 / 请求 / 响应 / curl」四段
 - cd sdk && pytest -v 仍全过（无回归）
 - go test -tags fts5 ./... 仍全过（无回归）
 - 合并到 feature/okf-integration 并推送

 后续子 PR 概要（不在本轮范围）

 - P5c：协议联调 + 灰度开关验证（config.okf.enabled=false）+ 压测脚本（10万节点 BFS 基准）+ 端到端
 e2e
 - P4a：后端 bundle 分享（DiskShare 加 bundle resType + share-code 中间件 + bundle-scoped 下载
 token）
 - P4b+c：前端 OKF UI（OkfBundle 列表页 + 目录树 + frontmatter 面板 + Cytoscape 图谱可视化）

 验证方式（汇总）

 # 1. OpenAPI 校验
 python3 -c "import yaml; yaml.safe_load(open('docs/openapi.yaml'))" && echo "yaml OK"

 # 2. VitePress 构建
 cd docs/site && npm run build

 # 3. （可选）本地预览
 cd docs/site && npx vitepress dev --port 9102
 # → 浏览器打开 /api/okf、/architecture/okf

 # 4. 回归确认
 cd ../.. && cd sdk && pytest -v
 cd .. && go test -tags fts5 ./...

 