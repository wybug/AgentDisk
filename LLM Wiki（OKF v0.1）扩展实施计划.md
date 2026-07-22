 LLM Wiki（OKF v0.1）扩展实施计划                                                                     
                                                                                                      
 Context                                                                                              
                                                                                                      
 AgentDisk 是一个智能体云盘（T0–T6 团队分工），已经具备完整的文件                                     
 CRUD、版本快照、回收站、标签、外链分享、Markdown 预览、RBAC 权限、公共目录 + API Key 机制、Python    
 SDK、React 前端。本计划在 AgentDisk 之上叠加 OKF v0.1（Google 草案规范）知识库能力。                 

 核心目标：扩展 AgentDisk 为"知识库管理与检索服务平台"：
 - 维护 Agent 通过公共目录机制写入知识（writer）
 - AgentDisk 自动物化索引、构建知识图谱
 - 应用 Agent（其他业务系统）通过 OKF 检索 API 查询知识、获取图谱（reader）

 核心约束：
 - 升级零侵入：原数据/原 API/原 SDK/原前端行为全部保持可用，OKF 是纯叠加
 - 写入严格、读取统一宽容：写入做质量门禁（type 必填、YAML 可解析、bundle 内链接目标存在）；读取按
 OKF §9 统一宽容（broken link/unknown type/unknown 字段不阻断）

 架构简化核心：一个公共目录（public directory）= 一个 OKF bundle。复用现有公共目录 + API Key
 鉴权机制，写入侧只需新增一个文本写入接口；读取侧新增独立的 OKF 检索 API。

 OKF v0.1 规范要点（合规边界）

 参考 https://github.com/GoogleCloudPlatform/knowledge-catalog/blob/main/okf/SPEC.md：
 - 唯一必填字段：type（字符串，不强制枚举）
 - 推荐字段：title / description / resource / tags(list) / timestamp(ISO 8601)
 - 保留文件名：index.md（目录索引，禁 frontmatter，仅根 index.md 可加
 okf_version）、log.md（更新历史）
 - 链接：/bundle/relative.md（bundle-relative）或 ./relative.md
 - Conformance §9：消费者 MUST NOT 因缺可选字段、unknown type、unknown 字段、broken link 拒绝 bundle

 用户已确认决策（2026-06-27）

 ┌────────────────────┬───────────────────────────────────────────────────────────────────────────┐
 │       决策点       │                                 选定方案                                  │
 ├────────────────────┼───────────────────────────────────────────────────────────────────────────┤
 │ 整体架构           │ 复用公共目录作为 bundle 容器：维护 Agent 走 pdWrite + API Key             │
 │                    │ 写入，AgentDisk 提供独立 OKF 检索 API 给应用 Agent                        │
 ├────────────────────┼───────────────────────────────────────────────────────────────────────────┤
 │ Bundle 标识        │ 隐式约定：公共目录根 index.md 含 okf_version 即为                         │
 │                    │ bundle；维护预编译索引表，检索不扫 OSS                                    │
 ├────────────────────┼───────────────────────────────────────────────────────────────────────────┤
 │ 内容存储           │ 正文存 OSS（复用版本/回收站/分享链路），MySQL 仅存 frontmatter            │
 │                    │ 抽取的元数据                                                              │
 ├────────────────────┼───────────────────────────────────────────────────────────────────────────┤
 │ 分期方案           │ 同意 5 期拆分，从 P1 MVP 开始                                             │
 ├────────────────────┼───────────────────────────────────────────────────────────────────────────┤
 │ Conformance 边界   │ 写入完全严格、读取统一宽容                                                │
 ├────────────────────┼───────────────────────────────────────────────────────────────────────────┤
 │ Redis 依赖         │ 强依赖（config.yaml 默认 redis.enabled=true），与 CLAUDE.md「必须         │
 │                    │ redis-server」一致                                                        │
 ├────────────────────┼───────────────────────────────────────────────────────────────────────────┤
 │ 跨 bundle 引用     │ 不支持，标 link_kind=external 当死链处理，后续按需开放                    │
 ├────────────────────┼───────────────────────────────────────────────────────────────────────────┤
 │ 图谱重建读取一致性 │ Stale 读 + 提示，重建期间返回旧图谱（响应头 X-Okf-Rebuild:                │
 │                    │ in-progress），重建完成后切换                                             │
 ├────────────────────┼───────────────────────────────────────────────────────────────────────────┤
 │ 前端可视化         │ Cytoscape.js + react-cytoscapejs，>5000 节点启用 lazy rendering           │
 ├────────────────────┼───────────────────────────────────────────────────────────────────────────┤
 │ 维护 Agent 实现    │ Google ADK Python 2.0（pip install google-adk），模型默认 Gemini 2.5      │
 │                    │ Flash，可切 Claude/DeepSeek V4/Ollama                                     │
 └────────────────────┴───────────────────────────────────────────────────────────────────────────┘

 角色与系统边界

 ┌──────────────────┐   API Key + pdWrite    ┌─────────────────────────┐
 │  维护 Agent      │ ─────────────────────> │ 公共目录 = OKF bundle   │
 │ (writer, 内部)   │                        │ (disk_folder + disk_file)│
 └──────────────────┘                        │   + 根 index.md 含      │
                                             │     okf_version: "0.1"  │
                   ┌─────────────────────────┘
                   │ 写入触发（同步物化）
                   ▼
 ┌──────────────────┐                  ┌────────────────────────────┐
 │  AgentDisk       │ <────────────────│ disk_okf_bundle            │
 │  检索/索引服务   │                  │ disk_okf_node (索引)       │
 │                  │                  │ disk_okf_edge (P3 图谱)    │
 └──────────────────┘                  └────────────────────────────┘
         │
         │ API Key + /v1/disk/okf/*
         ▼
 ┌──────────────────┐
 │  应用 Agent      │  调用方：业务系统、外部智能体
 │ (reader, 外部)   │  能力：检索 / 按 type 聚合 / 知识图谱查询
 └──────────────────┘

 - 维护 Agent：用现有 pdWrite 接口 + 1 个新增文本写入接口；走 API Key 鉴权；归属系统用户
 __system_public__
 - 应用 Agent：调用独立 OKF 检索 API；走 API Key 鉴权；只读
 - AgentDisk：内部维护 disk_okf_bundle/disk_okf_node/disk_okf_edge 索引，写入时同步物化，P3
 起后台增量构建图谱

 严格度边界

 - 写入路径（严格）：所有写入 bundle 内 .md 文件时校验
   - frontmatter 必须可解析（YAML 语法错误 → 400）
   - type 字段非空（缺失 → 400）
   - 保留文件名（index.md/log.md）结构合规
   - bundle-relative 链接 /foo.md 的目标在 bundle 内必须存在（断裂 → 400）
   - 不强制 type 枚举、不强制 title/timestamp、不拒绝 unknown frontmatter keys
 - 读取路径（统一宽容）：所有 GET/聚合/图谱/检索接口按 OKF §9
   - 缺可选字段 → 用文件名/空值兜底
   - unknown type → 当作 generic concept
   - broken link → 标记 dangling，图谱中以虚线节点呈现
   - unknown frontmatter keys → 原样保留返回
   - 非本系统写入的外部 bundle → 同上

 升级零侵入原则（贯穿 5 期）

 - 原表零修改：disk_file/disk_folder/disk_public_directory/disk_share
 等现有表只读引用，不改字段、不加列
 - 原接口零变更：现有 /v1/disk/files//folders//shares//public-directories 路由保持请求/响应结构不变
 - 新增路由独立前缀：/v1/disk/okf/*（reader）+ /v1/disk/public-directories/:id/files/content（writer
 扩展点）
 - 功能可降级：config.okf.enabled（默认 true），关闭后系统行为与升级前一致
 - 数据可回滚：新表独立，删除 OKF 相关表 + 路由即可完全回滚
 - 旧客户端兼容：Python SDK / 前端旧版本继续可用
 - bundle 显式注册：现有公共目录不会被自动当作 bundle，必须显式注册（写根 index.md 含 okf_version
 并调用 register 接口）

 知识图谱构建方案（P3 详细设计）

 存储技术选型（已评估）

 推荐方案：MySQL edge 表 + Redis 内存图缓存 + 应用层图算法

 ┌─────────────────────────┬─────────────────────────────────────────────────────────────────────┐
 │          方案           │                                评价                                 │
 ├─────────────────────────┼─────────────────────────────────────────────────────────────────────┤
 │ ✅ A. MySQL edge +      │ 零新依赖、复用现有 GORM/Redis、运维零成本、性能足够（10万节点 3 跳  │
 │ Redis 缓存              │ < 200ms）                                                           │
 ├─────────────────────────┼─────────────────────────────────────────────────────────────────────┤
 │ ❌ B. Neo4j / Memgraph  │ 独立部署 SRE 成本高、与项目极简栈冲突、Cypher 学习曲线陡            │
 ├─────────────────────────┼─────────────────────────────────────────────────────────────────────┤
 │ ❌ C. PostgreSQL +      │ 换主库破坏升级零侵入、AGE 在 PG 17 不稳定                           │
 │ Apache AGE              │                                                                     │
 ├─────────────────────────┼─────────────────────────────────────────────────────────────────────┤
 │ ⚠️  D. 纯 MySQL          │ 作为 A 的降级路径（Redis 不可用时），不应作为主方案                 │
 │ 应用层算法              │                                                                     │
 └─────────────────────────┴─────────────────────────────────────────────────────────────────────┘

 理由：OKF 链接图是稀疏有向图（平均出度 ~5），多跳查询深度 ≤ 3 跳，MySQL 复合索引 + 应用层 BFS +
 Redis 邻接表缓存足够。Neo4j 是 over-engineering。

 数据模型

 disk_okf_node（在 P1 基础上扩展）：
 + link_count       -- 出度（用于全图统计 O(1)、死链过滤）
 + backlink_count   -- 入度（被引用数）
 + content_hash     -- sha256(content)，增量更新幂等

 disk_okf_edge（P3 新建）：
 id BIGINT PK
 public_dir_id BIGINT        -- bundle 隔离，所有查询必带
 src_node_id BIGINT          -- 源节点
 dst_node_id BIGINT NULL     -- 目标节点（死链时 NULL）
 dst_rel_path VARCHAR(1024)  -- 原始链接文本
 dst_exists TINYINT(1)       -- 0=死链 1=活链（冗余，加速死链过滤）
 link_text VARCHAR(512)      -- 锚文本
 src_line INT                -- 行号（便于回链渲染）
 link_kind VARCHAR(16)       -- 'bundle' | 'relative' | 'external' | 'anchor'
 anchor VARCHAR(128)
 created_at, updated_at

 索引：
   uk(public_dir_id, src_node_id, dst_rel_path, src_line)  -- 增量幂等
   idx(public_dir_id, src_node_id)                         -- 正向 BFS
   idx(public_dir_id, dst_node_id)                         -- 反向 BFS
   idx(public_dir_id, dst_exists)                          -- 死链清单
   idx(public_dir_id, dst_rel_path(255))                   -- 路径反查

 跨 bundle 引用：MVP 不支持，link_kind='external' 当死链处理（破坏 bundle 隔离边界，后续按需开放）。

 增量构建算法

 OnUpsertFile(bundle_id, rel_path, content):
   1. parse_frontmatter_and_links(content) → {meta, links[]}     # O(L)
   2. sha256(content) → new_hash
   3. BEGIN TX
        a. upsert disk_file（OSS key 已先写入）
        b. upsert disk_okf_node（rel_path, meta, content_hash, link_count）
        c. SELECT * FROM disk_okf_edge WHERE src_node_id=? FOR UPDATE  # 行锁
        d. diff(old_edges, new_links) by (dst_rel_path, src_line)
             - to_delete / to_insert / unchanged
        e. DELETE/INSERT batch；同步维护相关 node 的 backlink_count
        f. UPDATE disk_okf_node.has_broken_link =
             EXISTS(edge WHERE src_node_id=? AND dst_exists=0)
      COMMIT
   4. Redis 失效：DEL graph:adj:{bundle}:{node_id}
   5. 异步：recompute broken_link flags on affected dst nodes

 - 复杂度 O(L)：L = 文件链接数（通常 < 50），毫秒级
 - 事务一致性：OSS 写入在前；MySQL TX 包裹 b-f；Redis 失效在 TX
 后（失败时下次查询回源重建，最终一致）
 - 失败回滚：GORM Transaction(Func(tx) error)，任意步骤 error 自动 ROLLBACK

 查询能力清单（P3 新增 API）

 ┌────────┬───────────────────────────────────┬─────────────────┬───────────────┬───────────────┐
 │  能力  │               路径                │      入参       │     底层      │ 性能（10万节  │
 │        │                                   │                 │               │     点）      │
 ├────────┼───────────────────────────────────┼─────────────────┼───────────────┼───────────────┤
 │ 1      │ GET /okf/nodes/:id/neighbors?dir= │ nodeId, dir,    │ MySQL 索引    │ < 10ms        │
 │ 跳邻居 │ out|in|both                       │ type 过滤       │               │               │
 ├────────┼───────────────────────────────────┼─────────────────┼───────────────┼───────────────┤
 │        │                                   │ {depth:1-3,     │ Redis 邻接表  │ 1 跳 <10ms，2 │
 │ N 跳   │ POST /okf/nodes/:id/reachable     │ types:[],       │ 优先，miss 回 │  跳 <50ms，3  │
 │ BFS    │                                   │ limit:500}      │  MySQL 批量   │ 跳 <200ms     │
 │        │                                   │                 │ IN            │               │
 ├────────┼───────────────────────────────────┼─────────────────┼───────────────┼───────────────┤
 │ 最短路 │ POST /okf/paths/shortest          │ {src, dst,      │ Redis + 双向  │ 平均 <100ms   │
 │ 径     │                                   │ maxDepth:6}     │ BFS           │               │
 ├────────┼───────────────────────────────────┼─────────────────┼───────────────┼───────────────┤
 │ 按     │                                   │ {types:["concep │ MySQL 全表 +  │               │
 │ type   │ POST /okf/subgraph                │ t"],            │ IN 批量取边   │ ~300ms        │
 │ 子图   │                                   │ maxNodes:1000}  │               │               │
 ├────────┼───────────────────────────────────┼─────────────────┼───────────────┼───────────────┤
 │        │                                   │                 │ Redis         │               │
 │ 全图统 │ GET /okf/bundles/:id/stats        │ bundleId        │ 缓存，TTL     │ < 5ms         │
 │ 计     │                                   │                 │ 60s，async    │               │
 │        │                                   │                 │ refresh       │               │
 ├────────┼───────────────────────────────────┼─────────────────┼───────────────┼───────────────┤
 │ 死链清 │ GET /okf/bundles/:id/broken-links │ cursor 分页     │ idx_broken    │ < 20ms/页     │
 │ 单     │                                   │                 │ 索引扫描      │               │
 ├────────┼───────────────────────────────────┼─────────────────┼───────────────┼───────────────┤
 │ 全量重 │ POST                              │ bundleId（管理  │ 8-worker      │               │
 │ 建     │ /okf/bundles/:id/rebuild-graph    │ 员）            │ 并发解析 +    │ 10万节点 ~90s │
 │        │                                   │                 │ 批量 INSERT   │               │
 └────────┴───────────────────────────────────┴─────────────────┴───────────────┴───────────────┘

 前端可视化（推荐 Cytoscape.js）

 ┌──────────────────────────────────┬────────────────────────────────────────────────────────────┐
 │                库                │                            评价                            │
 ├──────────────────────────────────┼────────────────────────────────────────────────────────────┤
 │ ✅ Cytoscape.js +                │ 专业图布局（cose-bilkent）、性能稳（1万节点流畅）、API     │
 │ react-cytoscapejs                │ 成熟                                                       │
 ├──────────────────────────────────┼────────────────────────────────────────────────────────────┤
 │ ⚠️  React Flow                    │ 适合节点编辑器场景，纯展示 over-engineered，>5000 节点卡顿 │
 ├──────────────────────────────────┼────────────────────────────────────────────────────────────┤
 │ ❌ D3.js                         │ 定制灵活但工程成本高                                       │
 ├──────────────────────────────────┼────────────────────────────────────────────────────────────┤
 │ ❌ Vis.js                        │ API 老旧、TS 集成弱                                        │
 └──────────────────────────────────┴────────────────────────────────────────────────────────────┘

 策略：bundle 图通常 100–5000 节点，Cytoscape 直接渲染；>5000 启用 lazy rendering（只渲染 viewport
 内节点）。

 性能预估

 ┌────────┬──────┬────────┬───────┬────────┬───────────┬──────────┐
 │  规模  │ 节点 │   边   │ 1 跳  │  2 跳  │   3 跳    │ 全图统计 │
 ├────────┼──────┼────────┼───────┼────────┼───────────┼──────────┤
 │ 1 万   │ 10K  │ 5 万   │ <5ms  │ <20ms  │ <80ms     │ <5ms     │
 ├────────┼──────┼────────┼───────┼────────┼───────────┼──────────┤
 │ 10 万  │ 100K │ 50 万  │ <10ms │ <50ms  │ <200ms    │ <5ms     │
 ├────────┼──────┼────────┼───────┼────────┼───────────┼──────────┤
 │ 100 万 │ 1M   │ 500 万 │ <15ms │ <150ms │ >500ms ⚠️  │ <10ms    │
 └────────┴──────┴────────┴───────┴────────┴───────────┴──────────┘

 瓶颈：
 - 主要：MySQL idx_dst 反向查询（BFS 第 2 起跳）→ Redis 邻接表缓存命中率 ≥ 95% 时几乎不回源
 - 次要：Redis 单实例内存（10 万节点约 500MB，100 万节点 5GB，需 maxmemory-policy=allkeys-lru）

 百万节点应对：BFS depth 硬限制 3 跳；子图提取强制 maxNodes 上限；全量重建走离线任务队列；监控 Redis
 memory >70% 告警；>500 万再评估按 public_dir_id 哈希分库。

 团队边界

 - T1（公共底座）：pkg/okf/（frontmatter + link 解析器）、internal/model/okf.go（GORM model，含
 edge）
 - T4（高级能力）：internal/repository/okf_graph.go、internal/service/okf_graph.go、internal/handler/
 okf_graph.go、应用层 BFS/最短路径算法、Redis 缓存层
 - T0 主控：管理员级 rebuild-graph 接口、审计日志

 关键复用清单（已确认存在）

 ┌──────────┬────────────────────────────────────────────────────────────────┬──────────────────┐
 │   能力   │                            复用路径                            │       备注       │
 ├──────────┼────────────────────────────────────────────────────────────────┼──────────────────┤
 │ 公共目录 │ internal/model/public_directory.go、disk_public_directory/disk │ OKF bundle 容器  │
 │ 机制     │ _public_directory_grant                                        │                  │
 ├──────────┼────────────────────────────────────────────────────────────────┼──────────────────┤
 │ API Key  │ internal/middleware/auth_hybrid.go:RequireAPIKey、disk_api_key │ writer + reader  │
 │ 鉴权     │                                                                │                  │
 ├──────────┼────────────────────────────────────────────────────────────────┼──────────────────┤
 │ 文件     │ pdWrite.POST /:id/files/upload、DELETE /:id/files/:fileId      │ multipart 上传   │
 │ CRUD     │                                                                │                  │
 ├──────────┼────────────────────────────────────────────────────────────────┼──────────────────┤
 │ 子目录创 │ pdWrite.POST /:id/folders                                      │ 当前不支持嵌套， │
 │ 建       │                                                                │ 需扩展 parentId  │
 ├──────────┼────────────────────────────────────────────────────────────────┼──────────────────┤
 │ 版本快照 │ internal/service/version.go                                    │ 自动跟随文件     │
 ├──────────┼────────────────────────────────────────────────────────────────┼──────────────────┤
 │ 回收站   │ internal/service/recycle.go                                    │ 自动跟随文件     │
 ├──────────┼────────────────────────────────────────────────────────────────┼──────────────────┤
 │ 标签     │ internal/service/tag.go                                        │ 可作为 OKF tags  │
 │          │                                                                │ 的镜像           │
 ├──────────┼────────────────────────────────────────────────────────────────┼──────────────────┤
 │ Markdown │ internal/handler/preview.go                                    │ bundle           │
 │  预览    │                                                                │ 内文档复用       │
 ├──────────┼────────────────────────────────────────────────────────────────┼──────────────────┤
 │ 外链分享 │ internal/handler/share.go                                      │ bundle 整包分享  │
 │          │                                                                │ P4               │
 ├──────────┼────────────────────────────────────────────────────────────────┼──────────────────┤
 │ 统一响应 │ pkg/response                                                   │ 全部新接口必须用 │
 └──────────┴────────────────────────────────────────────────────────────────┴──────────────────┘

 分期拆分

 期: P1
 目标: MVP：bundle 注册 + 文本写入 + 节点检索 + type 聚合；预留图谱扩展点 + Google ADK 维护 Agent
 demo
 团队: T1+T2+T6
 关键产出: pkg/okf/、新表、pdWrite 扩展（文本写入 + 嵌套目录）、OKF reader
   API、examples/adk_writer_agent/
 ────────────────────────────────────────
 期: P2
 目标: 智能化维护：index.md/log.md 自动生成、死链接扫描、bundle-relative 双向维护、并发锁
 团队: T2
 关键产出: service/okf_index.go、service/okf_link.go、handler/okf_scan.go
 ────────────────────────────────────────
 期: P3（核心目标）
 目标: 自动构建知识图谱 + 全文检索
 团队: T4
 关键产出: disk_okf_edge 落地、增量构建、GET /okf/bundles/:id/graph、POST /okf/search
 ────────────────────────────────────────
 期: P4
 目标: 预览/分享 OKF 化：目录树渲染、bundle 整包分享、frontmatter 面板、图谱可视化
 团队: T5
 关键产出: web/src/pages/OkfBundle/、share.go bundle 模式
 ────────────────────────────────────────
 期: P5
 目标: SDK + 联调灰度：Python SDK wiki 模块、OpenAPI 文档、协议联调、压测
 团队: T6
 关键产出: sdk/src/agentdisk/wiki.py、docs/

 P1 MVP 详细范围（subagent 直接执行清单）

 Worktree A — T1：pkg/okf 公共组件

 新增文件（无 DB/路由改动）：
 - pkg/okf/types.go：Frontmatter{Type, Title, Description, Resource, Tags []string, Timestamp, Extra
 map[string]any}
 - pkg/okf/frontmatter.go：Parse([]byte) (Frontmatter, []byte, error)、Serialize 保留 unknown
 keys（yaml.v3）
 - pkg/okf/validator.go：Validate(fm) error（type 非空）、IsReservedName(name) bool
 - pkg/okf/link.go：ParseBundleRelative(link) (string, bool)、Resolve(rel, currentPath) string
 - pkg/okf/*_test.go：覆盖率 100%（工具类强制）

 依赖：gopkg.in/yaml.v3（如 go.mod 未引入则添加）

 DoD：go test ./pkg/okf/... 全过、make lint 零警告、独立 PR

 Worktree B — T2：bundle 注册 + 物化索引 + writer/reader 接口

 新增表（独立迁移文件，不动原 schema）：
 - sql/schema_v2_okf.sql 或挂到 repository.AutoMigrate
 - 表：disk_okf_bundle、disk_okf_node（见上）

 新增代码：
 - internal/model/okf.go：OkfBundle、OkfNode struct
 - internal/repository/okf.go：CRUD + 按 type/tag/rel_path 聚合
 - internal/service/okf.go：
   - RegisterBundle(publicDirectoryID)：读根 index.md frontmatter，校验 okf_version，写
 disk_okf_bundle
   - WriteMarkdown(publicDirectoryID, relPath, content)：解析 frontmatter（严格）→ 写 OSS（复用现有
 FileService）→ 物化 disk_okf_node
   - ListBundles()、GetBundle(id)、ListNodes(bundleID, filter)、AggregateByType()
 - internal/handler/okf.go：HTTP handler，统一 pkg/response
 - internal/handler/public_directory_content.go：扩展公共目录文本写入

 扩展公共目录 writer（pdWrite，API Key 鉴权）：
 POST   /v1/disk/public-directories/:id/files/content
   # body: { relPath, content, contentType?="text/markdown" }
   # 严格校验 frontmatter，写入 OSS，同步物化 disk_okf_node
 POST   /v1/disk/public-directories/:id/folders
   # 扩展支持 parentId（嵌套目录），向后兼容现有 folderName 单层创建

 新增 OKF reader 路由（独立前缀，API Key 或 JWT）：
 POST   /v1/disk/okf/bundles/register     # body: { publicDirectoryId } 触发 bundle 注册
 GET    /v1/disk/okf/bundles              # 当前可见 bundle 列表
 GET    /v1/disk/okf/bundles/:id          # bundle 详情（目录树 + 元数据聚合）
 GET    /v1/disk/okf/bundles/:id/nodes    # ?type=&tag= 过滤
 GET    /v1/disk/okf/types                # 跨 bundle 按 type 聚合
 POST   /v1/disk/okf/bundles/:id/refresh  # 手动重建 node 索引
 DELETE /v1/disk/okf/bundles/:id          # 注销 bundle（不删公共目录）

 配置开关：
 - config.okf.enabled（默认 true），关闭时 OKF 路由 404、写入触发跳过物化、原 pdWrite 行为不变

 测试：
 - internal/service/okf_test.go：覆盖率 ≥80%
 - internal/handler/okf_test.go：HTTP 层覆盖正常流 + 错误分支（type 缺失 → 400、非 bundle 公共目录 →
 404、跨用户访问 → 403）
 - 测试前必须 redis-server（CLAUDE.md §3.1）
 - 浏览器测试：扩展 test/browser/ 增加 OKF 用例（T20 起，不污染 T01–T19）

 DoD：
 - go test ./... 全过、make lint 零警告
 - 新增表通过 go run ./scripts/clean_admin 兼容
 - 现有 T01–T19 浏览器测试 0 回归
 - 不读 .env、不直接调用 mysql（CLAUDE.md §4.7）

 Worktree C — T6：Google ADK 维护 Agent Demo

 目的：用 Google ADK Python 2.0 实现一个可运行的维护 Agent 示例，端到端验证 P1 Worktree B 的 writer +
  reader 链路；同时作为 P5 Python SDK wiki.py 模块的设计参考。该 demo
 是纯示例代码，不进生产链路，不影响主服务编译。

 目录结构（examples/adk_writer_agent/，独立于 sdk/ 避免耦合）：
 examples/adk_writer_agent/
 ├── README.md                # 运行说明、环境变量、示例对话录屏
 ├── pyproject.toml           # 依赖：google-adk>=2.0.0, httpx>=0.27, pyyaml
 ├── .env.example             # AGENTDISK_BASE_URL / AGENTDISK_API_KEY / WRITER_AGENT_MODEL
 ├── agentdisk_client.py      # 轻量 HTTP 客户端（封装 pdWrite + OKF reader，不走 sdk/)
 ├── tools.py                 # ADK FunctionTool 集合
 ├── agent.py                 # LlmAgent 定义（instruction + tools + 模型切换）
 └── scenarios/
     └── bootstrap_bundle.py  # 端到端示例脚本：建目录 → 写 index.md → 注册 bundle → 写知识

 ADK 工具清单（每个工具对应一个 AgentDisk API，纯 HTTP 调用）：

 ┌─────────────────┬─────────────────────────────────────────────────┬───────────────────────────┐
 │     工具名      │                    对应 API                     │           用途            │
 ├─────────────────┼─────────────────────────────────────────────────┼───────────────────────────┤
 │ register_bundle │ POST /v1/disk/okf/bundles/register              │ 将公共目录注册为 OKF      │
 │                 │                                                 │ bundle                    │
 ├─────────────────┼─────────────────────────────────────────────────┼───────────────────────────┤
 │ create_folder   │ POST /v1/disk/public-directories/:id/folders    │ 创建嵌套子目录（带        │
 │                 │                                                 │ parentId）                │
 ├─────────────────┼─────────────────────────────────────────────────┼───────────────────────────┤
 │ write_markdown  │ POST                                            │ 写入 .md（frontmatter     │
 │                 │ /v1/disk/public-directories/:id/files/content   │ 严格校验）                │
 ├─────────────────┼─────────────────────────────────────────────────┼───────────────────────────┤
 │ list_nodes      │ GET /v1/disk/okf/bundles/:id/nodes?type=&tag=   │ 列出 bundle 内节点        │
 ├─────────────────┼─────────────────────────────────────────────────┼───────────────────────────┤
 │ aggregate_types │ GET /v1/disk/okf/types                          │ 跨 bundle 按 type 聚合    │
 ├─────────────────┼─────────────────────────────────────────────────┼───────────────────────────┤
 │ refresh_index   │ POST /v1/disk/okf/bundles/:id/refresh           │ 手动重建 node 索引        │
 └─────────────────┴─────────────────────────────────────────────────┴───────────────────────────┘

 Agent 核心定义：
 from google.adk.agents import LlmAgent
 from .tools import (register_bundle, create_folder, write_markdown,
                     list_nodes, aggregate_types, refresh_index)

 INSTRUCTION = """
 你是 AgentDisk 知识库维护 Agent。职责：把用户提供的领域知识结构化为 OKF v0.1
 格式并写入指定 bundle。规则：
 - 每个 .md 必须有 YAML frontmatter，至少包含 type 字段（自由字符串，推荐小写英文）
 - 推荐字段：title / description / tags(list) / timestamp(ISO 8601)
 - 链接使用 bundle-relative：/concepts/gemma.md 或 ./gemma.md
 - 保留文件名：index.md（仅 bundle 根，含 okf_version）、log.md（更新历史）
 - 写入失败（400）时根据错误信息修正 frontmatter 后重试
 """

 root_agent = LlmAgent(
     name="agentdisk_writer",
     model="gemini-2.5-flash",   # 可切 claude-sonnet-4 / deepseek-v4 / ollama/llama3
     instruction=INSTRUCTION,
     tools=[register_bundle, create_folder, write_markdown,
            list_nodes, aggregate_types, refresh_index],
 )

 模型切换（通过 WRITER_AGENT_MODEL 环境变量）：
 - gemini-2.5-flash（默认，需 GEMINI_API_KEY）
 - claude-sonnet-4（需 ANTHROPIC_API_KEY，ADK 经 LiteLLM 适配）
 - deepseek-v4（需 DEEPSEEK_API_KEY，经 LiteLLM 走 DeepSeek/OpenAI 兼容接口）
 - ollama/llama3（本地 Ollama）

 端到端运行步骤：
 # 1. 前置：Worktree B 全部 DoD 通过；AgentDisk 启动
 redis-server &  # CLAUDE.md §3.1 强制
 bash scripts/dev.sh start

 # 2. 管理后台创建公共目录 wiki-demo，取得 pdId 与 API Key（scope=okf-write）

 # 3. 配置环境
 cp examples/adk_writer_agent/.env.example examples/adk_writer_agent/.env
 #   AGENTDISK_BASE_URL=http://localhost:8080
 #   AGENTDISK_API_KEY=adk_xxx
 #   GEMINI_API_KEY=xxx

 # 4. 安装依赖并启动 ADK Web UI（交互式）
 cd examples/adk_writer_agent
 pip install -e .
 adk web

 # 5. 或命令行一次性运行
 adk run . --input "把 Gemma 模型简介写入 wiki-demo bundle 的 concepts/gemma.md，type=LLM"

 验收点：
 - ADK Agent 能正确调用 6 个工具，完成"创建目录 → 写入 .md → 注册 bundle → 查询节点"全链路
 - 写入缺 type 的 .md 时，AgentDisk 返回 400，Agent 能识别错误并自我修正后重试
 - 切换 Claude/DeepSeek V4/Ollama 后行为一致（证明 API 协议层与模型无关）
 - 不依赖 Python SDK（直接 HTTP），证明 OKF API 是协议级开放的（任何语言/框架都可对接）

 DoD：
 - cd examples/adk_writer_agent && ruff check . && mypy . 零警告
 - adk run . 能完成 README 中给出的 bootstrap 场景
 - README 含 1 段录屏（gif/mp4）展示完整对话
 - 强依赖 Worktree B 全部 DoD 通过（否则 demo 无法运行）
 - Demo 代码不进 make lint / go test ./... 范围（独立 pyproject）

 关键文件清单（P1）

 新建：
 - /Users/wangyun/Documents/work/gitlab/agent-disk/pkg/okf/{types,frontmatter,validator,link}.go +
 _test.go
 - /Users/wangyun/Documents/work/gitlab/agent-disk/internal/model/okf.go
 - /Users/wangyun/Documents/work/gitlab/agent-disk/internal/repository/okf.go
 - /Users/wangyun/Documents/work/gitlab/agent-disk/internal/service/okf.go
 - /Users/wangyun/Documents/work/gitlab/agent-disk/internal/handler/okf.go
 - /Users/wangyun/Documents/work/gitlab/agent-disk/internal/handler/public_directory_content.go
 - /Users/wangyun/Documents/work/gitlab/agent-disk/sql/schema_v2_okf.sql
 - /Users/wangyun/Documents/work/gitlab/agent-disk/examples/adk_writer_agent/{README.md,pyproject.tom
 l,.env.example,agentdisk_client.py,tools.py,agent.py,scenarios/bootstrap_bundle.py}

 修改：
 - /Users/wangyun/Documents/work/gitlab/agent-disk/internal/router/router.go（注册 OKF 路由 + DI）
 - /Users/wangyun/Documents/work/gitlab/agent-disk/internal/handler/public_directory.go（folders
 接口扩展 parentId，向后兼容）
 - /Users/wangyun/Documents/work/gitlab/agent-disk/internal/repository/repository.go（AutoMigrate
 加新表）
 - /Users/wangyun/Documents/work/gitlab/agent-disk/config/config.go（新增 okf.enabled 字段）
 - /Users/wangyun/Documents/work/gitlab/agent-disk/go.mod（如需 yaml.v3）

 复用：
 - pkg/response、internal/middleware/auth_hybrid.go
 - internal/service/file.go（OSS 写入）、internal/service/public_directory.go（鉴权 + 目录定位）
 - internal/model/file.go、internal/model/folder.go、internal/model/public_directory.go

 P3 实施清单（细化，2026-06-29）

 P3 在 P1+P2 合入 feature/okf-integration 之后启动。鉴于 P3 体量大（schema + 写侧增量 + 读侧图谱算法
 + 全文检索），拆 3 个独立 worktree / PR，逐个合入：

 ┌─────┬───────────────────────────────────────────────────────────────┬──────┬─────────────────┐
 │ 子  │                             范围                              │ 团队 │      依赖       │
 │ PR  │                                                               │      │                 │
 ├─────┼───────────────────────────────────────────────────────────────┼──────┼─────────────────┤
 │ P3a │ 写侧：disk_okf_edge 表 + disk_okf_node 加                     │ T4   │ P1+P2（已就绪） │
 │     │ link_count/backlink_count + WriteMarkdown 增量物化边          │      │                 │
 ├─────┼───────────────────────────────────────────────────────────────┼──────┼─────────────────┤
 │ P3b │ 读侧：图查询 API（1 跳 / N 跳 BFS / 最短路径 / 子图 /         │ T4   │ P3a             │
 │     │ 全图统计）+ Redis 邻接表缓存 + rebuild-graph                  │      │                 │
 ├─────┼───────────────────────────────────────────────────────────────┼──────┼─────────────────┤
 │ P3c │ 全文检索：MySQL FULLTEXT（ngram）+ POST /okf/search           │ T4   │ P3a（独立于     │
 │     │                                                               │      │ P3b）           │
 └─────┴───────────────────────────────────────────────────────────────┴──────┴─────────────────┘

 P3a 是基础（边表落地后 P3b/P3c 才有意义），先做 P3a，然后 P3b/P3c 可并行。本节聚焦 P3a 细化，P3b/P3c
  等 P3a 合入后再展开。

 现状盘点（P1+P2 合入后的真实代码状态）

 schema 现状（internal/model/okf.go）：
 - disk_okf_node 已有：has_broken_link ✓、content_hash ✓（idx_content_hash）
 - disk_okf_node 缺：link_count、backlink_count
 - disk_okf_bundle 已有：edge_count（占位字段，目前没维护）

 写侧钩子（internal/service/okf.go WriteMarkdown）：
 - L358-372：runTx 事务包裹 materializeNode + CountByBundle + bundles.Update
 - L377：postWriteSyncHooks（同步：AppendLogEntry + broken-link 重算）
 - L382-384：scheduleIndexRegen（异步）
 - P3a 钩子点：在 L359 materializeNode 返回 node 后、CountByBundle 之前，调用 materializeEdges(tx,
 bundle, node, content)

 链接抽取已就绪（internal/service/okf_link.go）：
 - ExtractLinks(content, srcRelPath) []LinkInfo 返回 LinkInfo{SrcNodeID, SrcRelPath, DstRelPath,
 SrcLine, LinkText, LinkKind}
 - LinkKind 已分类：bundle / external / anchor
 - ScanBundleLinks 目前只翻 has_broken_link 标志，不写边表
 - P3a 后 ScanBundleLinks 改为读 disk_okf_edge.dst_exists 反查（去重逻辑统一到边表）

 仓库模式：无批量插入 helper，Upsert 是逐行 Save；P3a 需要新增批量替换语义（DELETE + 批量 INSERT
 在事务内）

 P3a 详细范围（worktree: feature/okf-p3-edge-materialize）

 1. Schema（internal/model/okf.go + sql/schema_v2_okf.sql）

 扩展 OkfNode：
 LinkCount       uint32 `gorm:"not null;default:0" json:"linkCount"`
 BacklinkCount   uint32 `gorm:"not null;default:0" json:"backlinkCount"`

 新增 OkfEdge struct（同文件）：
 type OkfEdge struct {
     ID           uint64    `gorm:"primaryKey;autoIncrement"`
     PublicDirID  uint64    `gorm:"not null;index:idx_edge_src,priority:1;index:idx_edge_dst,priority
 :1;index:idx_edge_broken,priority:1;index:idx_edge_relpath,priority:1" json:"publicDirId"`
     SrcNodeID    uint64    `gorm:"not null;index:idx_edge_src,priority:2" json:"srcNodeId"`
     DstNodeID    uint64    `gorm:"index:idx_edge_dst,priority:2" json:"dstNodeId"`         // NULL
 when dst does not exist
     DstRelPath   string    `gorm:"size:1024;not null" json:"dstRelPath"`
     DstExists    bool      `gorm:"not null;default:false;index:idx_edge_broken,priority:2"
 json:"dstExists"`
     LinkText     string    `gorm:"size:512" json:"linkText"`
     SrcLine      int       `gorm:"not null;default:0" json:"srcLine"`
     LinkKind     string    `gorm:"size:16;not null" json:"linkKind"`                       //
 bundle|external|anchor
     Anchor       string    `gorm:"size:128" json:"anchor"`
     CreatedAt    time.Time `gorm:"type:datetime(3);not null" json:"createdAt"`
     UpdatedAt    time.Time `gorm:"type:datetime(3);not null" json:"updatedAt"`
 }
 func (OkfEdge) TableName() string { return "disk_okf_edge" }

 唯一键去重：uniqueIndex:uk_edge_src_line on (src_node_id, dst_rel_path, src_line) ——
 同一源节点同一行同一目标的重复写只留一条。

 迁移：挂到 internal/repository/db.go 的 AutoMigrate（已有 OkfBundle / OkfNode 注册点，line
 ~70-95），加 &model.OkfEdge{}。sql/schema_v2_okf.sql 同步追加 CREATE TABLE 语句保持文档一致。

 2. 仓库层（internal/repository/okf_graph.go 新文件）

 type OkfEdgeRepo struct{ db *gorm.DB }

 func NewOkfEdgeRepo(db *gorm.DB) *OkfEdgeRepo
 // ReplaceForSrc 在事务内删除 src_node_id 的所有旧边，批量插入新边。
 // 批量插入用单条 INSERT 多 VALUES（构造 CASE WHEN 适配 SQLite/MySQL）。
 func (r *OkfEdgeRepo) ReplaceForSrc(tx *gorm.DB, edges []model.OkfEdge) error
 // ListBySrc 返回某节点出边，支持 limit。
 func (r *OkfEdgeRepo) ListBySrc(srcNodeID uint64, limit int) ([]model.OkfEdge, error)
 // ListByDst 返回某节点入边（反向 BFS 用）。
 func (r *OkfEdgeRepo) ListByDst(dstNodeID uint64, limit int) ([]model.OkfEdge, error)
 // ListBrokenByBundle 分页返回某 bundle 内 dst_exists=0 的边（broken-links API）。
 func (r *OkfEdgeRepo) ListBrokenByBundle(bundleID uint64, publicDirID uint64, cursor uint64, limit
 int) ([]model.OkfEdge, uint64, error)
 // CountByBundle 返回 bundle 的总边数（维护 bundle.edge_count）。
 func (r *OkfEdgeRepo) CountByBundle(tx *gorm.DB, bundleID uint64, publicDirID uint64) (uint32,
 error)
 // DeleteByBundle 注销 bundle 时清理（挂在 UnregisterBundle 路径）。
 func (r *OkfEdgeRepo) DeleteByBundle(tx *gorm.DB, publicDirID uint64) error
 // AdjustBacklinks 在事务内对受影响节点增/减 backlink_count。
 // delta = +1 (新增入边) / -1 (删除入边)。受影响节点 ID 由调用方传入。
 func (r *OkfEdgeRepo) AdjustBacklinks(tx *gorm.DB, increment []uint64, decrement []uint64) error

 3. 服务层（internal/service/okf_graph.go 新文件 + internal/service/okf.go 钩入）

 新文件 okf_graph.go：
 // materializeEdges 是 WriteMarkdown 事务内的边物化器。复用 ExtractLinks 抽链接，
 // 把 bundle 类链接解析到 dst_node_id（找不到则 dst_exists=0），其余 kind 直接入表。
 // 输入：tx、bundle、刚 upsert 的 node、刚写入的 content。
 // 副作用：替换 src_node_id 的全部边；调整受影响 dst 节点的 backlink_count；
 //        回写 src_node 的 link_count 和 has_broken_link；累加 bundle.edge_count 增量。
 func (s *OkfService) materializeEdges(ctx context.Context, tx *gorm.DB, bundle *model.OkfBundle,
 node *model.OkfNode, content []byte) error

 算法（事务内）：
 1. links := ExtractLinks(content, node.RelPath)
 2. 对每条 bundle 链接：dst, err := nodes.GetByBundleAndRelPath(bundle.ID, li.DstRelPath)；err=nil 则
  dstExists=true, dstID=dst.ID，否则 dstExists=false, dstID=0
 3. 构造 []model.OkfEdge（每条 link → 一个 edge，SrcNodeID=node.ID,
 PublicDirID=bundle.PublicDirectoryID）
 4. oldEdges := edgeRepo.ListBySrc(node.ID, ...) 拿到旧边
 5. diff：按 (dst_rel_path, src_line) 做集合差。toDelete / toInsert / unchanged。unchanged 不写。
 6. 批量 DELETE toDelete（按主键）+ 批量 INSERT toInsert
 7. 计算 backlink delta：
   - 新增边的 dst 节点（dstExists=true）→ increment 列表
   - 删除边的 dst 节点（dstExists=true 且不在新边中）→ decrement 列表
   - 调 AdjustBacklinks(tx, inc, dec)
 8. 回写 node.LinkCount = len(links)、node.HasBrokenLink = any(dstExists=false in new edges)
 9. 把 node 的更新并入事务（nodes.Upsert(tx, node)）

 okf.go 钩入：WriteMarkdown L358 事务里，materializeNode 后增加 materializeEdges，并在事务结束前更新
 bundle.EdgeCount：
 if err := s.runTx(ctx, func(tx *gorm.DB) error {
     n, mErr := s.materializeNode(...)
     if mErr != nil { return mErr }
     node = n
     if gErr := s.materializeEdges(ctx, tx, bundle, node, req.Content); gErr != nil {
         return fmt.Errorf("materialize edges: %w", gErr)
     }
     count, _ := s.nodes.CountByBundle(tx, bundle.ID)
     bundle.NodeCount = count
     edgeCount, _ := s.edges.CountByBundle(tx, bundle.ID, bundle.PublicDirectoryID)
     bundle.EdgeCount = edgeCount
     return s.bundles.Update(bundle)
 }); err != nil { ... }

 UnregisterBundle 清理（已有 DELETE 路径，加一行）：在 DeleteByBundle 节点删除前，先
 edges.DeleteByBundle(tx, bundle.PublicDirectoryID)。

 P2 兼容：ScanBundleLinks（P2 已实现）在 P3a 后改为从边表读，去重逻辑统一；过渡期保留 has_broken_link
  由 materializeEdges 直接维护（去掉 postWriteSyncHooks 里的旧 broken-link 重算路径）。

 4. 配置 + 路由

 P3a 不新增路由（只写侧），路由变更留给 P3b。config.okf.enabled 不变。

 5. 测试

 仓库层（新文件 internal/repository/okf_graph_test.go）：用 SQLite
 memory（gorm.Open(sqlite.Open(":memory:"))）测 ReplaceForSrc 的 delete+insert
 语义、ListBySrc/Dst、CountByBundle、AdjustBacklinks。这是项目第一个 repo 层测试 —— 参考
 internal/store 已有 SQLite 测试模式。

 服务层（扩展 internal/service/okf_test.go 或新 okf_graph_test.go）：
 - TestMaterializeEdges_NewNode：新写一个 .md 含 3 个 bundle 链接（2 存在 1 不存在）→ 3 条
 edge、link_count=3、has_broken_link=true
 - TestMaterializeEdges_UpdateReuses：同一 .md 二次写，仅修改 1 条链接 → 边表正确
 diff，backlink_count 增量正确
 - TestMaterializeEdges_DeleteAll：把 .md 内容改成空（无链接）→ 边表清空、link_count=0、backlink
 计数回滚
 - TestWriteMarkdown_EdgeMaterialized：端到端集成，写两个 .md 互相引用，验证双向边都落库
 - TestUnregisterBundle_DeletesEdges：注销 bundle 后边表为空

 fake repo 扩展：internal/service/okf_test.go 的 fakeOkfNodeRepo 加 LinkCount/BacklinkCount
 字段读写；新增 fakeOkfEdgeRepo 实现上述接口（in-memory map[uint64][]model.OkfEdge）。

 ACL：P3a 不引入新端点，但 materializeEdges 跑在 WriteMarkdown 事务内，继承 writer 已通过的
 ACL，无需额外检查。

 6. DoD

 - go test ./... -count=1 全过
 - make lint 0 issues
 - OkfEdge 表通过 AutoMigrate 自动建（SQLite + MySQL 双兼容）
 - WriteMarkdown 写入后 disk_okf_edge 行数 = 该 .md 的链接数
 - 二次写入相同内容 → 边表零变化（content_hash 短路？此处不短路，因为 dst 可能从无到有）
 - P2 的 broken-links API（GET /bundles/:id/broken-links）行为不变（从边表读后等价输出）
 - T01–T19 浏览器回归 0 影响（不涉及公共目录 API）

 P3b/P3c 概要（P3a 合入后细化）

 P3b（读侧）：internal/service/okf_bfs.go（应用层 BFS / 双向 BFS / 子图提取）+
 internal/service/okf_graph_cache.go（Redis 邻接表 okf:adj:{bundleID}:{nodeID}，TTL 5min，写时 DEL）+
  internal/handler/okf_graph.go（5 路由：neighbors / reachable / shortest / subgraph / stats）+
 rebuild-graph（管理员，8-worker 并发）。Redis cache miss → MySQL idx_edge_src/dst 批量取 → 缓存。

 P3c（检索）：disk_okf_node 加 FULLTEXT(title, description) WITH PARSER ngram（仅 MySQL；SQLite 退化
 LIKE）+ OkfNodeRepo.Search(query, bundleID, limit) + POST /v1/disk/okf/search body={query,
 bundleId?, type?, limit}。

 验证方式（P3a）

 # 1. Redis + 服务
 redis-server &
 bash scripts/dev.sh start

 # 2. 单元测试
 go test ./internal/repository -count=1 -run OkfGraph
 go test ./internal/service -count=1 -run Okf
 go test ./... -count=1
 make lint

 # 3. 集成验证（手测，需 API Key + 公共目录已注册为 bundle）
 # 3.1 写 a.md 引用 ./b.md
 curl -X POST http://localhost:9100/v1/disk/public-directories/:pdId/files/content \
   -H "Authorization: Bearer $API_KEY" \
   -d '{"relPath":"a.md","content":"---\ntype: doc\n---\n[b](./b.md)"}'
 # 3.2 写 b.md
 curl -X POST .../files/content -d '{"relPath":"b.md","content":"---\ntype: doc\n---\nbody"}'
 # 3.3 触发扫描（旧路径，应等价于从边表读）
 curl -X POST http://localhost:9100/v1/disk/okf/bundles/:id/scan
 # 3.4 broken-links 应为空
 curl http://localhost:9100/v1/disk/okf/bundles/:id/broken-links
 # 3.5 改 a.md 引用 ./c.md（c.md 不存在）
 curl -X POST .../files/content -d '{"relPath":"a.md","content":"---\ntype: doc\n---\n[b](./b.md)
 [c](./c.md)"}'
 # 3.6 broken-links 应含 1 条 c.md，has_broken_link=true
 # 3.7 浏览器回归
 cd test/browser && node runner.js   # T01–T19 0 回归

 1. 升级兼容性验收（每期必查）：
   - 现有 test/browser/runner.js 全部 T01–T19 用例 0 回归
   - 现有 web/ 前端走查核心场景（上传、预览、分享、标签、公共目录）无变化
   - Python SDK 旧版本调用现有 API 行为不变
   - 关闭 config.okf.enabled 后系统与升级前 byte-identical
 2. 公共目录嵌套目录：现有 POST /:id/folders 不支持 parentId，P1 扩展时必须保持 folderName
 单字段调用的向后兼容（旧客户端无感知）
 3. 维护 Agent 多实例并发写入（P2 解决）：bundle 级 Redis advisory lock（SETNX okf:lock:{bundleId} 5s
  TTL）。P1 MVP 不做，先 last-writer-wins
 4. bundle-relative 链接解析（P2 完整实现）：靠 disk_okf_node.rel_path 反查；P1
 只校验目标存在性、不做反向解析
 5. 知识图谱增量构建（P3 核心）：每次 .md 写入后增量更新 disk_okf_edge；全量重建 POST
 /okf/bundles/:id/rebuild-graph
 6. 全文检索引擎（P3 决策）：MySQL FULLTEXT + ngram parser（中文）起步，性能不足时切 MeiliSearch
 7. 应用 Agent 凭证边界：API Key 是否区分 writer/reader scope？建议 P1 沿用现有 API Key scope
 字段（scope=okf-write/scope=okf-read），pdWrite 检查 write、OKF reader 检查 read；P5 灰度时细化

 验证方式（P1）

 # 1. 启动 Redis（CLAUDE.md §3.1 强制）
 redis-server &

 # 2. 启动服务
 bash scripts/dev.sh start

 # 3. 单元测试
 go test ./pkg/okf/... -v
 go test ./internal/service/... -v -run OKF
 go test ./internal/handler/... -v -run OKF

 # 4. Lint
 make lint

 # 5. 集成测试（手测，准备 API Key）
 # 5.1 创建公共目录（管理后台）+ 取得 pdId + API Key (scope=okf-write)
 # 5.2 POST /v1/disk/public-directories/:pdId/files/content
 #     relPath="index.md", content="---\nokf_version: \"0.1\"\n---\n# Bundle Root"
 # 5.3 POST /v1/disk/okf/bundles/register  body: { publicDirectoryId: pdId }
 # 5.4 POST /v1/disk/public-directories/:pdId/files/content
 #     relPath="concepts/gemma.md", content="---\ntype: LLM\n---\n# Gemma"
 # 5.5 写入缺 type 的 .md → 应返回 400
 # 5.6 GET /v1/disk/okf/bundles/:id 验证目录树 + 元数据
 # 5.7 GET /v1/disk/okf/types 验证跨 bundle 聚合
 # 5.8 关闭 config.okf.enabled → /v1/disk/okf/* 应 404，pdWrite 原行为不变

 # 6. ADK 维护 Agent demo（Worktree C，依赖 Worktree B 完成）
 cd examples/adk_writer_agent
 pip install -e .
 ruff check . && mypy .
 adk run . --input "把 Gemma 模型简介写入 wiki-demo bundle 的 concepts/gemma.md，type=LLM"
 # 期望：Agent 自动调用 create_folder + write_markdown + register_bundle，
 #       验证 GET /v1/disk/okf/bundles/:id 能看到新节点

 # 7. 浏览器测试回归
 cd test/browser && node runner.js

 后续 P3–P5 概要

 - P2（T2，✅ 已合入 feature/okf-integration @ a9beaef）：index.md/log.md 自动生成、死链接扫描 POST
 /okf/bundles/:id/scan、bundle-relative 链接双向维护、并发锁、disk_okf_node.has_broken_link
 字段启用、ACL fix
 - P3（T4，当前阶段，细化见上「P3 实施清单」）：拆 P3a/P3b/P3c 三 PR
   - P3a：disk_okf_edge 表 + 写侧增量物化（worktree feature/okf-p3-edge-materialize）
   - P3b：图查询 API（1 跳/N 跳 BFS/最短路径/子图/统计）+ Redis 邻接表缓存 + rebuild-graph
   - P3c：MySQL FULLTEXT（ngram）+ POST /okf/search
 - P4（T5）：React OKF 目录树组件、bundle 整包外链分享、frontmatter 元数据面板、bundle
 内链接跳转预览、图谱可视化（force-directed graph）
 - P5（T6）：Python SDK wiki.py 模块（writer + reader 双客户端）、OpenAPI
 文档补全、协议联调、灰度开关、压测
╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌

