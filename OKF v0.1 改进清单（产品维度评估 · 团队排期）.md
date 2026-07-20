# OKF v0.1 改进清单（产品维度评估 · 团队排期用）

> 来源：基于对 `feature/okf-integration` 的产品维度深挖（后端 / 前端 / SDK·文档·质量 三路并行分析，约 60 条发现）。
> 用途：合并/演进前的改进 backlog，已按 **AgentDisk 团队拆分（CLAUDE.md §2：T0–T6 + SDK + 文档）** 标注归属，供 T0 主控排期、子智能体领取。
> 完整分析见本地 `~/.claude/plans/okf-compressed-valiant.md`。每条均含 `file:line` 证据。成本：S(<1d) / M(1–3d) / L(跨周)。
> **红线（CLAUDE.md §6）**：单智能体不得跨模块包揽；下表「归属」即责任域，跨域项标注「协同」。

---

## ✅ 完成状态（2026-07-18，feature/okf-integration）

**Tier 1 必修缺陷 —— 全部完成**（5 模块提交）：T1.1 Redis 缓存接线、T1.2 SDK 凭证透传、T1.3/T1.4 runner 重置+多 filter、T1.5 文档方法对齐、T1.7 锁 409、T1.8 markdown 样式、T1.9 CreateShareModal、T1.10 死链 total、T1.11 断言修正、T1.12 ignoreDeadLinks、T1.13 fileId、T1.14 markdown 消毒。（T1.6 InternalError 细节保留并入 Tier 3 可观测性。）

**Tier 2 高价值缺口 —— 主要完成**（10 项）：搜索全链路（UI 接入 + 正文入索引 + bm25 相关性排序 + offset 分页）、perf 三件套（WriteMarkdown N+1 批量化 / RefreshBundle 单遍读 / ListBrokenLinks 物化表）、节点列表分页（加载更多）、bundle 列表搜索/排序、OKF 终端指南 + SDK 分页迭代器、图谱边标签 + type 色图例。

**门禁**：`go test -race ./...`（13 包）、`make lint`（0）、`make sdk-check`（53 文件）、`make web-check`、全套 29 浏览器用例（28 + t25 搜索）、SDK pytest 97 全绿。

**Tier 3 进展（2026-07-20 更新）**：
- ✅ 可观测性地基三片：①InternalError 落盘真实详情 + 请求 ID 关联 ②静默吞掉的后台失败（AppendLogEntry/scheduleIndexRegen/bundle.Update/edges.DeleteByBundle 等）现经 logBestEffort 可见。
- ✅ 跨 bundle 边隔离读侧守卫：BFS 结果按起点 bundle 过滤 + 日志（防 writer bug/DB 篡改导致的跨 bundle 泄露）。
- ✅ bundle 导出（zip）：`GET /okf/bundles/:id/export` 流式 + 前端「导出 ZIP」按钮。

**剩余（新会话推进）**：结构化全局审计、批量导入 API、分享访问分析、webhooks/事件、frontmatter 行内编辑、i18n、a11y、可观测性（指标 p99/缓存命中率 + tracing）、跨 bundle 边**写侧**不变量（schema：public_dir_id 入唯一键）、MySQL 搜索相关性排序（待 MySQL 测试环境）、图谱聚类/增量布局。

> **交接指引**：Tier 3 剩余项在**新会话**推进；每项按模块拆分（CLAUDE.md §2 归属），完成后**合并到 `feature/okf-integration`**。当前分支 0 behind origin，全门禁绿，可随时并入主干。

---

## 0. 归属矩阵（速查）

| ID | 改进项 | 归属 | Tier | 成本 |
|----|--------|------|------|------|
| T1.1 | Redis 图谱缓存接线 | T2（配置字段协同 T1） | 1 | S |
| T1.2 | SDK 凭证 setter 透传 `_wiki` | SDK | 1 | S |
| T1.3 | runner 用例间重置驱动 | T6 | 1 | S–M |
| T1.4 | runner 支持多 filter | T6 | 1 | S |
| T1.5 | `sdk-python.md` 方法对齐 / 挂 `_PreviewAPI` | 文档（协同 SDK） | 1 | S |
| T1.6 | `InternalError` 保留+记录详情 | T2（可观测性起点） | 1 | M |
| T1.7 | 锁 409 修正 + Retry-After | T2 | 1 | S |
| T1.8 | 分享 markdown 样式 | T5 | 1 | S |
| T1.9 | CreateShareModal 修补 | T5 | 1 | S |
| T1.10 | 死链分页 total 真值 | T2（后端）+ T5（前端） | 1 | S–M |
| T1.11 | T22.10 图谱挂载断言恒真 | T6 | 1 | S |
| T1.12 | `ignoreDeadLinks:false` + 链路审计 | 文档 | 1 | S |
| T1.13 | 分享 `onPreview` 用 fileId | T5 | 1 | S |
| T1.14 | markdown 显式消毒 | T3（安全）+ T5（落地） | 1 | S |
| — | AggregateTypes 单 IN 查询 | T2 | 2 | S |
| — | WriteMarkdown 边物化 N+1 批量化 | T2 | 2 | M |
| — | Scan/ListBrokenLinks 用物化表 | T2 | 2 | M |
| — | RefreshBundle OSS 单遍读 | T2 | 2 | M |
| — | bundle 锁看门狗续期 | T2 | 2 | M |
| — | 搜索相关性排序 + 正文入索引 + highlight | T2（协同 T4 检索域） | 2 | M–L |
| — | BFS 截断 `truncated` 标记 + 游标 | T2 | 2 | M |
| — | 搜索 UI 接入（后端已就绪） | T5 | 2 | M |
| — | 节点列表分页（含契约改 nextCursor） | T2（契约）+ T5（UI） | 2 | L |
| — | bundle 列表搜索/排序/分页 | T5（契约协同 T2） | 2 | M |
| — | 图谱：边标签/图例/聚类/增量布局 | T5 | 2 | L |
| — | 分享视图 OG/SEO + 过期展示 + 下载/打印 + 统计 tab | T5 | 2 | M–L |
| — | 死链面板操作（跳源/忽略/批量） | T5 | 2 | M |
| — | feature-flag 门控 UI（优雅降级） | T5 | 2 | M |
| — | OKF 终端用户指南 + SDK OKF 章节 | 文档 | 2 | M |
| — | SDK 分页迭代器/流式 | SDK | 2 | M |
| — | SDK path→ID 解析器 + 收敛两套 client | SDK（协同 demo agent） | 2 | M |
| — | eval phase-2 链接构建覆盖 | T6 | 2 | M |
| — | share reader ACL 显式化 | T3 | 2 | S |
| — | OpenAPI 响应 wrapper 命名 schema | 文档（协同 SDK） | 2 | S |
| T3.x | bundle 版本化/导出/diff | T4 | 3 | L |
| T3.x | 批量导入 API | T2 | 3 | L |
| T3.x | 分享访问分析 | T5 | 3 | M |
| T3.x | 结构化全局审计 | T0/T3 | 3 | M |
| T3.x | webhooks/事件 | T2 | 3 | L |
| T3.x | 跨 bundle 边隔离不变量 | T3（schema）+ T2 | 3 | M |
| T3.x | frontmatter 行内编辑 | T5（writer 暴露协同 T3） | 3 | L |
| T3.x | i18n | T5 | 3 | L |
| T3.x | 可访问性（ARIA/键盘） | T5 | 3 | M–L |
| T3.x | 可观测性体系（指标/tracing） | T2 | 3 | L |

---

## 1. Tier 1 · 必修缺陷（建议作为「硬化小 PR」首批落地）

### T2 · 后端
- **[T1.1] 接线 Redis 图谱缓存** — 现状 `SetGraphCache` 从未调用（`internal/router/router.go:140-153`），实现闲置（`okf_graph_cache.go:48-136`），`OkfConfig` 缺 `AdjTTLSeconds`（`config/config.go:100-109`）。验收：RedisAddr 配置时 BFS 走缓存、Invalidate 在 WriteMarkdown 生效；BFS 基准 warm p99 显著下降。依赖：T1 加配置字段。成本 S。
- **[T1.7] 锁 409 修正** — `respondOkfError` 无 `ErrOkfLockHeld` 分支（`okf.go:213-228`），409 漏成 500；scan 路径 flat 409 无 `Retry-After`（`okf_scan.go:132-133`）。验收：并发写败者收 409 + `Retry-After: <TTL>`。成本 S。
- **[T1.6] InternalError 保留+记录详情** — `pkg/response/response.go:63-65` 忽略入参；OKF handler 传入的 `err.Error()` 被丢弃，且无日志。验收：对外仍只回 `"internal error"`，但服务端结构化日志记录详情；不泄露堆栈给客户端。成本 M。
- **[Tier2·性能] WriteMarkdown N+1 批量化** — 每条 bundle 链一次 `GetByBundleAndRelPath`，`ExtractLinks` 跑两遍（`okf.go:542-553,660`；`okf_graph.go:39,103-108`）。验收：单次 `ExtractLinks` + 一次批量 `ListByBundleAndRelPaths`。成本 M。
- **[Tier2·性能] Scan/ListBrokenLinks 用物化表** — 每节点一次 OSS 读、忽略已物化的 `has_broken_link` 列与 `edges.ListBrokenByBundle`（`okf_link.go:242-272,383-405`；`okf.go:584-598`）。验收：从 `disk_okf_edge` 读死链、`OnFileDeleted` 节流。成本 M。
- **[Tier2·性能] RefreshBundle OSS 单遍读** — 每个 markdown 读两遍、文件夹列两遍（`okf.go:854-976`）。验收：body 在两 pass 间传递或融合为单 pass。成本 M。
- **[Tier2·性能] bundle 锁看门狗续期** — TTL 默认 5s 无续期，大事务可中途过期（`okf_lock.go:66-93`；`config.go:190-200`）。验收：事务期间后台续期或版本 fence。成本 M。
- **[Tier2] AggregateTypes 单 IN 查询** — handler 逐 bundle 调聚合（`okf.go:142-152`）。验收：一次 `WHERE bundle_id IN (...)`。成本 S。

### SDK
- **[T1.2] 凭证 setter 透传 `_wiki`** — `client.py:87-119` / `async_client.py` 的 setter 不更新 `_wiki`。验收：JWT→API-Key 切换后 `write_markdown` 成功；新增 setter 透传单测（先 JWT 再切 Key）。成本 S。

### T6 · 测试
- **[T1.3] runner 用例间重置驱动** — 循环内无 `close --all`（`runner.js:96,126`），长跑级联 ETIMEDOUT。验收：`node runner.js`（无 filter）一次跑完全部 28 个不挂。成本 S–M。
- **[T1.4] runner 多 filter** — `args.find` 只取首参（`runner.js:52`）。验收：`node runner.js t22 t23` 两用例都跑。成本 S。
- **[T1.11] 断言修正** — `cyMounted.length > 0` 作用在字符串恒真（`t22-okf-bundle.js:271-274`，同类 t23）。验收：改为对元素计数断言。成本 S。
- **（同 T1.3）runner 死三元 + 超时清理** — `runner.js:111` ternary 两支同值；超时后未清孤立 chromium。验收：record 用例给更长 timeout；超时后显式 close + pkill。成本 S。

### 文档（协同 SDK）
- **[T1.5] sdk-python.md 方法对齐** — 文档调 `client.preview()`/`get_public_directory()` 但 client 上不存在（`sdk-python.md:248,273`；`api/preview.py:5` 类未挂载 `client.py:10-21,402`）。验收：挂 `_PreviewAPI` 到两个 client 并 re-export，或删除文档示例；示例可复制运行。成本 S。
- **[T1.12] ignoreDeadLinks 链路审计** — `config.ts:7` 为 true 掩盖腐烂。验收：清理后设 false 并入 docs-build。成本 S。

### T5 · 前端
- **[T1.8] 分享 markdown 样式** — `.markdown-body` 无任何 CSS（`OkfShareMarkdownModal.tsx:44`；`index.css` 仅 15 行）。验收：引入 github-markdown-css 或加 `.markdown-body` 规则；目视标题/表格/代码正常。成本 S。
- **[T1.9] CreateShareModal 修补** — footer 三元两支同值、用废弃 `message`（`CreateShareModal.tsx:58,2`）。验收：删失效三元、改 `App.useApp().message`、加「管理分享」深链。成本 S。
- **[T1.13] onPreview 用 fileId** — `void fileId;` 丢弃传参（`OkfShareBundleView.tsx:104-107`）。验收：用传入 fileId 加载 body。成本 S。
- **[Tier2] 死链 total 真值（前端）** — 去掉 `OkfBrokenLinksPanel.tsx:123-135` 启发式，用后端 total。依赖 T2 后端先加 total 字段。成本 S。

### T3 · 安全（协同 T5）
- **[T1.14] markdown 显式消毒** — 无 `rehype-sanitize`、无 URL scheme 白名单（`OkfShareMarkdownModal.tsx:45-68`、`FilePreview.tsx:96`）。验收：加 `rehype-sanitize` + `urlTransform` 白名单；`javascript:` 链接不可达；代码块加复制按钮。成本 S。

---

## 2. Tier 2 · 下版本高价值产品缺口（按主题，见矩阵归属）

- **🔍 搜索**：后端搜索 UI 未接入（`okfApi.search` 零调用，`okf.ts:74-75`）→ T5；后端无相关性排序/正文未入索引/无 highlight（`okf.go:265-277,307-321`）→ T2+T4。
- **📦 分页/大规模**：节点列表整包加载无 `nextCursor`（`OkfNodeList.tsx:46-59`、`types.ts:237-239`）→ T2 契约 + T5 UI；bundle 列表无搜索/排序/分页 → T5；BFS 截断无 `truncated`/游标（`okf_bfs.go:34-44,233-235,419-421`）→ T2。
- **🕸 图谱**：边标签不渲染、无图例、无聚类、500 节点无虚拟化、邻居展开重跑全量布局（`OkfGraphView.tsx:126-136,183-221`）→ T5。
- **🔗 分享/预览**：无 OG/twitter 卡片、过期/次数未展示、无下载 zip/打印、统计 tab 未挂（`OkfShareBundleView.tsx:73-99`、`ShareAccessPage` 未用 `expireAt/maxVisit/visitCount`）→ T5；死链面板零操作 → T5。
- **🧑‍💻 DX**：无 OKF 终端指南/SDK 章节（侧栏无 OKF，`sdk-python.md` 零 OKF）→ 文档；SDK 无分页迭代器 → SDK；缺 path→ID 解析器致 demo agent 另起一套手写 client（命名分歧 `refresh_bundle` vs `refresh_index`、错误属性 `http_status` vs `status_code`）→ SDK。
- **🧪 测试**：前端无 feature-flag 门控（关 flag 时 UI 运行时报错/403）→ T5；eval 无 phase-2 链接构建覆盖 → T6。
- **🔐 安全**：share reader `vis=nil` 隐式绕过 ACL（`okf_share.go:97,123`；`okf.go:761-763`）→ T3。

---

## 3. Tier 3 · 中长期战略项（见矩阵归属）

bundle 版本化/导出/diff（T4）、批量导入 API（T2）、分享访问分析（T5）、结构化全局审计（T0/T3）、webhooks/事件（T2）、跨 bundle 边隔离不变量（T3+T2）、frontmatter 行内编辑（T5+T3）、i18n（T5）、a11y（T5）、可观测性体系指标/tracing（T2）。

---

## 4. 现状优势（不建议改，作为基线保留）

错误→状态码 `errors.Is` sentinel 紧凑；BFS 环处理 + `dedupAdj`；边 diff 最小化回链抖动 + clamp；bundle 锁 Lua compare-and-delete；游标分页一致；`api/okf.md` + `architecture/okf.md` 为全项目最强文档；OpenAPI 18 接口齐全；SDK async/sync 完全对等；`seed_okf_demo.py` 真幂等；eval README 如实标注严格性失败。

---

## 5. 排期建议（供 T0）

1. **首批硬化小 PR（1–2 天，多智能体并行）**：T1.1（T2）、T1.2（SDK）、T1.3/T1.4/T1.11（T6）、T1.5/T1.12（文档）、T1.8/T1.9/T1.13（T5）、T1.14（T3+T5）、T1.7（T2）、AggregateTypes（T2）。独立、互不阻塞、风险低。
2. **下版本（1–2 周）**：搜索（T2 排序+索引 → T5 UI）、分页契约（T2→T5）、图谱体验（T5）、DX 文档与 SDK 迭代器（文档+SDK）、性能批量化（T2）、feature-flag 门控（T5）、eval phase-2（T6）。
3. **路线图（按需）**：Tier 3 按业务优先级排。

---

## 6. 验收门禁（所有改动须过）

`go test -race -count=1 ./...`、`make lint`、`make sdk-check`、`make web-check`、`make okf-eval`、`cd test/browser && node runner.js`（用例间重置后）均绿（CLAUDE.md §4.2/§4.3）。
