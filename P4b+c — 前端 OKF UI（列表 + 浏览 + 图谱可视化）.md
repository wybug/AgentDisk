P4b+c — 前端 OKF UI（列表 + 浏览 + 图谱可视化）                                                     

 Context

 OKF v0.1 后端 18 端点（P1–P3）、Python SDK（P5a）、OpenAPI + VitePress 文档（P5b）已全部合并到
 feature/okf-integration 并推送。但前端零覆盖：

 - grep -ri okf web/src/ 在 web/src/api/、web/src/pages/、web/src/components/ 都返回空。
 - AppSidebar.tsx 没有 OKF 入口；router/index.tsx 没有 /okf/* 路由。
 - web/package.json 没有 cytoscape / d3 / vis-network 等图谱可视化库。
 - 用户除了用 Python SDK / curl，没有任何图形化方式浏览 OKF bundle。

 P4b+c 给前端补全 OKF UI：bundle 列表页、bundle 详情页（4 个 tab：节点 / 图谱 / 统计 /
 死链）、frontmatter 侧滑面板、Cytoscape 图谱可视化。完成后用户可以在浏览器里浏览知识图谱、查看节点
 frontmatter、跑图谱查询、做死链维护。

 范围裁剪：
 - 不做 bundle 分享 UI（依赖 P4a 后端 bundle 分享能力，等 P4a 落地后再加一个"分享"按钮）
 - 不做 写入 UI（writer 端点要 API Key，浏览器 UI 走 JWT；写入仍走 SDK / curl）
 - 不做 admin rebuild-graph 按钮（需要 admin JWT，与浏览器 JWT 不兼容；admin 操作继续走管理后台）

 用户已确认决策

 ┌─────────────────────┬─────────────────────────────────────────────────────────────────────────┐
 │       决策点        │                                选定方案                                 │
 ├─────────────────────┼─────────────────────────────────────────────────────────────────────────┤
 │ 子 PR 范围          │ P4b+c 合并（列表 + 浏览 + 图谱）                                        │
 ├─────────────────────┼─────────────────────────────────────────────────────────────────────────┤
 │ Worktree 策略       │ 新 worktree okf-p4-web-ui off feature/okf-integration（CLAUDE.md §4.1） │
 ├─────────────────────┼─────────────────────────────────────────────────────────────────────────┤
 │ 图谱库              │ Cytoscape.js（plan 默认；轻量、React 兼容好、内置 cose 力导向布局）     │
 ├─────────────────────┼─────────────────────────────────────────────────────────────────────────┤
 │ 写入 UI             │ 不做（writer 要 API Key；JWT 用户走 SDK 写入）                          │
 ├─────────────────────┼─────────────────────────────────────────────────────────────────────────┤
 │ 分享 UI             │ 不做（P4a 未做，无后端可调）                                            │
 ├─────────────────────┼─────────────────────────────────────────────────────────────────────────┤
 │ Admin rebuild-graph │ 不做（admin JWT 与用户 JWT 不兼容）                                     │
 ├─────────────────────┼─────────────────────────────────────────────────────────────────────────┤
 │ 节点 markdown 预览  │ 复用现有 /v1/disk/preview/:fileId（OkfNode 有 fileId 字段，无需新端点） │
 └─────────────────────┴─────────────────────────────────────────────────────────────────────────┘

 现状盘点

 前端栈（Explore agent 确认）：
 - React 19 + TypeScript 6 + Vite 8
 - 状态：Zustand 5 + React Query 5
 - 路由：React Router DOM 7（lazy load + Suspense）
 - UI：Ant Design 6 + @ant-design/icons 6
 - Markdown 渲染：react-markdown 10 + remark-gfm 4（已用于 FilePreview.tsx）
 - HTTP：axios apiClient，cookie 携带凭据（withCredentials: true）
 - 类型：web/src/api/types.ts 集中所有后端 model 类型

 关键文件：
 - web/src/components/layout/AppSidebar.tsx:12-21 — sidebar 菜单硬编码数组
 - web/src/router/index.tsx:49-216 — createBrowserRouter 配置
 - web/src/api/client.ts — axios 实例
 - web/src/api/publicDirectory.ts — API client 模板（10 行）
 - web/src/api/types.ts — 类型集中处
 - web/src/pages/PublicDirectoriesPage.tsx — 列表 → 详情的页面模式参考
 - web/src/pages/ExplorerPage.tsx — 使用 React Query + 子组件组合的页面模式参考
 - web/src/components/file/FilePreview.tsx:94-98 — ReactMarkdown 用法参考
 - web/package.json:12-26 — 依赖清单（无 cytoscape）

 OKF 端点盘点（来自 P5b 文档 docs/site/api/okf.md）：

 ┌─────────────────────────────────────────────────┬─────────────────────────────────────────┐
 │                      端点                       │                 UI 用途                 │
 ├─────────────────────────────────────────────────┼─────────────────────────────────────────┤
 │ GET /v1/disk/okf/bundles                        │ 列表页                                  │
 ├─────────────────────────────────────────────────┼─────────────────────────────────────────┤
 │ POST /v1/disk/okf/bundles/register              │ 列表页"注册"按钮（选 public directory） │
 ├─────────────────────────────────────────────────┼─────────────────────────────────────────┤
 │ GET /v1/disk/okf/bundles/:id                    │ 详情页头                                │
 ├─────────────────────────────────────────────────┼─────────────────────────────────────────┤
 │ POST /v1/disk/okf/bundles/:id/refresh           │ 详情页"刷新"按钮                        │
 ├─────────────────────────────────────────────────┼─────────────────────────────────────────┤
 │ DELETE /v1/disk/okf/bundles/:id                 │ 详情页"注销"按钮（带确认）              │
 ├─────────────────────────────────────────────────┼─────────────────────────────────────────┤
 │ GET /v1/disk/okf/bundles/:id/nodes?type=&tag=   │ 节点 tab                                │
 ├─────────────────────────────────────────────────┼─────────────────────────────────────────┤
 │ GET /v1/disk/okf/types                          │ 列表页/详情页统计 tile                  │
 ├─────────────────────────────────────────────────┼─────────────────────────────────────────┤
 │ POST /v1/disk/okf/search                        │ 全文检索（详情页头部搜索框）            │
 ├─────────────────────────────────────────────────┼─────────────────────────────────────────┤
 │ GET /v1/disk/okf/nodes/:id/neighbors?dir=&type= │ 图谱 tab：点击节点加载邻居              │
 ├─────────────────────────────────────────────────┼─────────────────────────────────────────┤
 │ POST /v1/disk/okf/subgraph                      │ 图谱 tab：初始加载整图（带 maxNodes）   │
 ├─────────────────────────────────────────────────┼─────────────────────────────────────────┤
 │ GET /v1/disk/okf/bundles/:id/stats              │ 统计 tab                                │
 ├─────────────────────────────────────────────────┼─────────────────────────────────────────┤
 │ POST /v1/disk/okf/bundles/:id/scan              │ 死链 tab："扫描"按钮                    │
 ├─────────────────────────────────────────────────┼─────────────────────────────────────────┤
 │ GET /v1/disk/okf/bundles/:id/broken-links       │ 死链 tab：列表分页                      │
 ├─────────────────────────────────────────────────┼─────────────────────────────────────────┤
 │ POST /v1/disk/okf/bundles/:id/regenerate-index  │ 详情页"重建索引"按钮                    │
 └─────────────────────────────────────────────────┴─────────────────────────────────────────┘

 Reachable / shortest-path：图谱 tab 的高级查询面板（可选，UI 用 modal 实现）。

 实施步骤

 Step 1 — 新 worktree

 git -C /Users/wangyun/Documents/work/gitlab/agent-disk worktree add \
   .claude/worktrees/okf-p4-web-ui -b feature/okf-p4-web-ui feature/okf-integration

 EnterWorktree 进入 .claude/worktrees/okf-p4-web-ui。

 Step 2 — 加 Cytoscape 依赖

 web/package.json 的 dependencies 追加：

 "cytoscape": "^3.30.4",
 "cytoscape-cose-bilkent": "^4.1.0"

 devDependencies 追加：

 "@types/cytoscape": "^3.21.7"

 cytoscape-cose-bilkent 提供比内置 cose 更好的力导向布局（节点不挤成一团）。

 cd web && npm install --no-audit --no-fund

 Step 3 — OKF 类型定义

 web/src/api/types.ts 追加：

 // OKF v0.1
 export interface OkfBundle {
   bundleId: number;
   publicDirectoryId: number;
   okfVersion: string;
   rootIndexFileId: number;
   title: string;
   description: string;
   status: 'active' | 'archived';
   nodeCount: number;
   edgeCount: number;
   createdAt: string;
   updatedAt: string;
 }

 export interface OkfNode {
   nodeId: number;
   bundleId: number;
   fileId: number;
   relPath: string;
   type: string;
   title: string;
   description: string;
   tags: string[];
   timestamp: string;
   hasBrokenLink: boolean;
   extra: Record<string, unknown>;
   contentHash: string;
   createdAt: string;
   updatedAt: string;
 }

 export interface OkfEdge {
   edgeId: number;
   srcNodeId: number;
   dstNodeId: number;
   dstRelPath: string;
   linkText: string;
   srcLine: number;
   linkKind: 'bundle' | 'external' | 'anchor';
   dstExists: boolean;
 }

 export interface OkfTypeCount { type: string; count: number; }
 export interface OkfBrokenLink {
   srcNodeId: number; srcRelPath: string; dstRelPath: string;
   srcLine: number; linkText: string;
   linkKind: 'bundle' | 'external' | 'anchor'; reason: string;
 }
 export interface OkfBundleStats {
   nodeCount: number; edgeTotal: number;
   edgeLive: number; edgeBroken: number;
   types: OkfTypeCount[];
 }
 export interface OkfSearchPage { nodes: OkfNode[]; nextCursor: number; }
 export interface OkfRegisterBundleRequest { publicDirectoryId: number; }
 export interface OkfSearchRequest {
   query: string; bundleId?: number; type?: string;
   limit?: number; cursor?: number;
 }
 export interface OkfSubgraphRequest {
   bundleId: number; types?: string[]; maxNodes?: number;
 }

 Step 4 — OKF API client

 新建 web/src/api/okf.ts：

 import apiClient from './client';
 import type { ApiResponse } from './types';

 // 约定：apiClient 返回 axios response，res.data 是 ApiResponse<T>，
 // 这里统一展开成 T 直接返回（与 publicDirectory.ts 风格一致）。
 function unwrap<T>(p: Promise<{ data: ApiResponse<T> }>): Promise<T> {
   return p.then(r => r.data.data);
 }

 export const okfApi = {
   // Bundle lifecycle
   listBundles: () => unwrap<OkfBundle[]>(apiClient.get('/v1/disk/okf/bundles')),
   getBundle: (id: number) => unwrap<OkfBundle>(apiClient.get(`/v1/disk/okf/bundles/${id}`)),
   registerBundle: (publicDirectoryId: number) =>
     unwrap<OkfBundle>(apiClient.post('/v1/disk/okf/bundles/register', { publicDirectoryId })),
   refreshBundle: (id: number) =>
 unwrap<OkfBundle>(apiClient.post(`/v1/disk/okf/bundles/${id}/refresh`)),
   unregisterBundle: (id: number) =>
 unwrap<unknown>(apiClient.delete(`/v1/disk/okf/bundles/${id}`)),

   // Nodes
   listNodes: (bundleId: number, type?: string, tag?: string) =>
     unwrap<{ nodes: OkfNode[] }>(
       apiClient.get(`/v1/disk/okf/bundles/${bundleId}/nodes`, { params: { type, tag } })),
   aggregateTypes: () => unwrap<OkfTypeCount[]>(apiClient.get('/v1/disk/okf/types')),
   search: (req: OkfSearchRequest) =>
     unwrap<OkfSearchPage>(apiClient.post('/v1/disk/okf/search', req)),

   // Graph
   neighbors: (nodeId: number, dir: 'out' | 'in' | 'both' = 'out', type?: string) =>
     unwrap<{ nodes: OkfNode[]; edges: OkfEdge[] }>(
       apiClient.get(`/v1/disk/okf/nodes/${nodeId}/neighbors`, { params: { dir, type } })),
   subgraph: (req: OkfSubgraphRequest) =>
     unwrap<{ nodes: OkfNode[]; edges: OkfEdge[] }>(apiClient.post('/v1/disk/okf/subgraph', req)),
   stats: (bundleId: number) =>
     unwrap<OkfBundleStats>(apiClient.get(`/v1/disk/okf/bundles/${bundleId}/stats`)),

   // Maintenance
   scanBundle: (bundleId: number) =>
     unwrap<{ scannedNodes: number; brokenCount: number; scannedAt: string }>(
       apiClient.post(`/v1/disk/okf/bundles/${bundleId}/scan`)),
   listBrokenLinks: (bundleId: number, cursor = 0, limit = 50) =>
     unwrap<{ brokenLinks: OkfBrokenLink[]; nextCursor: number }>(
       apiClient.get(`/v1/disk/okf/bundles/${bundleId}/broken-links`, { params: { cursor, limit }
 })),
   regenerateIndex: (bundleId: number) =>
     unwrap<{ indexVersion: number; regeneratedAt: string }>(
       apiClient.post(`/v1/disk/okf/bundles/${bundleId}/regenerate-index`)),
 };

 类型 import 用 import type { OkfBundle, OkfNode, ... } from './types'，避免运行时引入。

 Step 5 — Bundle 列表页 OkfBundlesPage

 web/src/pages/OkfBundlesPage.tsx：

 - React Query useQuery(['okf-bundles'], okfApi.listBundles)
 - 顶部 Card + 标题 "OKF 知识库"
 - 顶部工具栏：[刷新] [注册 Bundle] 按钮
 - AntD <List> grid 模式渲染 bundle 卡片：
   - 标题：bundle.title
   - 描述：bundle.description（截断）
   - 标签：<Tag>{status}</Tag> + <Tag>{okfVersion}</Tag> + 节点数 {nodeCount} + 边数 {edgeCount}
   - 点击卡片：navigate(/okf/${bundle.bundleId})
 - 注册 Bundle 用 <Modal>，下拉选择可见的 public directory（复用 publicDirectoryApi.listVisible()）
 - 刷新按钮：queryClient.invalidateQueries(['okf-bundles'])

 Step 6 — Bundle 详情页 OkfBundleDetailPage

 web/src/pages/OkfBundleDetailPage.tsx：

 - useParams<{ bundleId: string }>()
 - 顶部：<BreadcrumbNav> + bundle 头部信息（title, description, nodeCount, edgeCount）+
 操作按钮：[刷新 Bundle] [重建索引] [注销 Bundle]
 - 操作按钮的确认逻辑：
   - 刷新：直接调 okfApi.refreshBundle，loading state on button
   - 重建索引：弹 <Modal.confirm> 确认 → okfApi.regenerateIndex
   - 注销：弹 <Modal.confirm> 警告 → okfApi.unregisterBundle → navigate 回 /okf
 - AntD <Tabs> 4 个 tab：

 Tab 1: 节点（默认）

 - 工具栏：type 下拉（从 okfApi.aggregateTypes 取）、tag 输入框、[搜索] 按钮
 - <Table> 列：title / type / tags / hasBrokenLink（红角标）/ updatedAt
 - 行点击：打开 frontmatter drawer（见 Step 8）
 - 分页：客户端分页（数据量预期 < 1000）

 Tab 2: 图谱

 - 工具栏：[加载整图] / [清空] / type 多选过滤 / maxNodes 输入
 - 默认进入：调用 okfApi.subgraph({ bundleId, maxNodes: 200 })，渲染初始图
 - Cytoscape 容器 div，高度 600px
 - 节点点击：调 okfApi.neighbors(nodeId)，把新节点 + 边 merge 进图
 - 节点双击：打开 frontmatter drawer
 - 配色：节点按 type 分配调色板（10 色循环），hasBrokenLink=true 加红色边框；边按 dstExists
 区分实/虚线
 - 布局：cose-bilkent，animate: false, idealEdgeLength: 100

 Tab 3: 统计

 - 调 okfApi.stats(bundleId) 一次
 - 4 个 <Statistic> tile：nodeCount / edgeTotal / edgeLive / edgeBroken
 - type 分布用 AntD <Table> 或简单条形图（用 <Progress> 横条模拟，避免引入图表库）

 Tab 4: 死链

 - 顶部：[扫描] 按钮（调 okfApi.scanBundle，扫描后刷新列表）
 - <Table> 列：srcRelPath / linkText / dstRelPath / srcLine / linkKind / reason
 - 分页：cursor 分页（nextCursor → 下一页）

 Step 7 — Cytoscape 图谱组件 OkfGraphView

 web/src/components/okf/OkfGraphView.tsx（最复杂的组件，单独说明）：

 interface Props {
   bundleId: number;
   onNodeDoubleClick: (node: OkfNode) => void;
 }

 实现要点：
 - useRef<Cytoscape.Core> 持有 cytoscape 实例
 - useEffect 在 mount 时初始化 cytoscape，卸载时 destroy
 - 内部 state：nodes: OkfNode[], edges: OkfEdge[]，存原始数据用于回调
 - loadSubgraph(types?, maxNodes?)：调 API，把结果 merge 进 state + cytoscape
 - expandNeighbors(nodeId)：调 API，merge
 - 节点样式：
 const style: Stylesheet[] = [
   { selector: 'node', style: { 'label': 'data(label)', 'background-color': 'data(color)', 'width':
 40, 'height': 40 } },
   { selector: 'node[broken="true"]', style: { 'border-color': '#ff4d4f', 'border-width': 3 } },
   { selector: 'edge', style: { 'width': 2, 'line-color': '#888', 'target-arrow-color': '#888',
 'target-arrow-shape': 'triangle' } },
   { selector: 'edge[broken="true"]', style: { 'line-color': '#ff4d4f', 'line-style': 'dashed' } },
 ];
 - 颜色映射：基于 type 字符串 hash → HSL hue，10 色调色板循环
 - 节点 label 用 title（截断到 20 字符）
 - 事件绑定：cy.on('tap', 'node', ...) → expandNeighbors；cy.on('dbltap', 'node', ...) →
 onNodeDoubleClick
 - 工具栏按钮（暴露给详情页 tab）：load-all / clear / filter-by-type

 Step 8 — Frontmatter Drawer OkfFrontmatterDrawer

 web/src/components/okf/OkfFrontmatterDrawer.tsx：

 - AntD <Drawer> placement="right" width={480}
 - 属性区（Descriptions）：
   - title / type / relPath / timestamp / contentHash
   - tags：<Tag> 数组
   - hasBrokenLink：红色 Tag
 - extra（折叠）：<Typography.Text code>{JSON.stringify(extra)}</Typography.Text>
 - 底部按钮：[预览 markdown]（复用 /v1/disk/preview/:fileId 跳转 PreviewPage；或直接打开 <Modal> 用
 <ReactMarkdown> 渲染）

 Step 9 — 路由 + sidebar 接入

 web/src/router/index.tsx：
 - 顶部追加 const OkfBundlesPage = lazy(...)
 - const OkfBundleDetailPage = lazy(...)
 - AppLayout children 追加：
 { path: 'okf', element: <Suspense fallback={<Loading />}><OkfBundlesPage /></Suspense> },
 { path: 'okf/:bundleId', element: <Suspense fallback={<Loading />}><OkfBundleDetailPage 
 /></Suspense> },

 web/src/components/layout/AppSidebar.tsx：
 - icon: ClusterOutlined（从 @ant-design/icons 导入；与 Public 隔一行）
 - 入口：
 { key: '/okf', icon: <ClusterOutlined />, label: 'OKF 知识库' },
 - 位置：/public 之后、/recycle 之前

 Step 10 — 浏览器端验证（DevTools + 人工 + browser tests）

 # 启动栈
 bash scripts/dev.sh start

 # 浏览器手动验证
 # 1. 登录 → 看到 sidebar 新增 "OKF 知识库" 入口
 # 2. 点击 → /okf 列表页加载（无 bundle 时显示空状态）
 # 3. 用 Python SDK 先种一个 bundle + 几个节点（参考 sdk/tests/test_wiki.py）
 # 4. 列表页刷新 → 看到 bundle 卡片
 # 5. 点击 → /okf/:id 详情页
 # 6. 节点 tab：看到节点表格，type 过滤、tag 过滤都工作
 # 7. 行点击 → frontmatter drawer 弹出，字段正确
 # 8. 图谱 tab：默认加载整图，节点按 type 着色，死链节点红框
 # 9. 单击节点 → 邻居 expand，新节点 merge 进图
 # 10. 双击节点 → frontmatter drawer 弹出
 # 11. 统计 tab：4 个数字 + type 分布
 # 12. 死链 tab：扫描 → 看到列表
 # 13. 头部操作按钮：刷新 / 重建索引 / 注销 都工作（注销后回 /okf）

 # 回归：跑现有 browser test suite
 cd test/browser && node runner.js

 Step 11 — Lint + 构建

 cd web && npm run lint
 cd web && npm run build  # tsc -b && vite build

 Step 12 — 提交、合并、推送

 git add web/package.json web/package-lock.json \
         web/src/api/types.ts \
         web/src/api/okf.ts \
         web/src/pages/OkfBundlesPage.tsx \
         web/src/pages/OkfBundleDetailPage.tsx \
         web/src/components/okf/ \
         web/src/components/layout/AppSidebar.tsx \
         web/src/router/index.tsx
 git commit -m "feat(okf): P4b+c frontend UI — bundle list, node browser, Cytoscape graph"

 # 主 worktree 合并
 git -C /Users/wangyun/Documents/work/gitlab/agent-disk merge --no-ff feature/okf-p4-web-ui \
   -m "Merge PR-P4b+c: OKF frontend UI"
 git -C /Users/wangyun/Documents/work/gitlab/agent-disk push origin feature/okf-integration

 关键文件清单

 新建：
 - web/src/api/okf.ts（OKF API client，~80 行）
 - web/src/pages/OkfBundlesPage.tsx（列表页，~150 行）
 - web/src/pages/OkfBundleDetailPage.tsx（详情页 + 4 tab，~250 行）
 - web/src/components/okf/OkfNodeList.tsx（节点 tab 子组件，~100 行）
 - web/src/components/okf/OkfGraphView.tsx（Cytoscape 容器，~200 行）
 - web/src/components/okf/OkfStatsPanel.tsx（统计 tab，~80 行）
 - web/src/components/okf/OkfBrokenLinksPanel.tsx（死链 tab，~120 行）
 - web/src/components/okf/OkfFrontmatterDrawer.tsx（节点详情侧滑，~150 行）
 - web/src/components/okf/RegisterBundleModal.tsx（注册 modal，~100 行）

 修改：
 - web/package.json + web/package-lock.json（加 cytoscape / cytoscape-cose-bilkent /
 @types/cytoscape）
 - web/src/api/types.ts（追加 11 个 OKF 类型）
 - web/src/components/layout/AppSidebar.tsx（加 1 个菜单项 + 1 个图标 import）
 - web/src/router/index.tsx（加 2 个 lazy import + 2 个 route）

 不动：
 - 后端代码（OKF 后端已完整）
 - SDK 代码
 - 文档（P5b 已覆盖；如有需要可后续小 PR 加 VitePress guide）

 DoD

 - cd web && npm run lint 0 错误
 - cd web && npm run build 0 错误（TS + Vite 通过）
 - 浏览器手动验证 13 步全过（见 Step 10）
 - cd test/browser && node runner.js 不引入 OKF 相关回归（已有用例不破）
 - 合并到 feature/okf-integration 并推送

 后续子 PR 概要（不在本轮范围）

 - P4a：后端 bundle 分享（DiskShare 加 bundle resType + share-code 中间件）→ 落地后给 OKF UI
 加"分享"按钮
 - P5c：协议联调 + 灰度开关 + 10 万节点 BFS 压测 + e2e
 - 写入 UI（可选）：API Key 输入框 + markdown 编辑器，让用户在 UI 里写知识（需要 SDK api-key 中转）

 验证方式（汇总）

 # 1. 依赖
 cd web && npm install

 # 2. Lint + 类型 + 构建
 cd web && npm run lint
 cd web && npm run build

 # 3. 浏览器手动（需要先 bash scripts/dev.sh start + 用 SDK 种子数据）
 # 13 步见 Step 10

 # 4. Browser 回归
 cd test/browser && node runner.js

 # 5. Go / SDK 不受影响（无后端改动）
 go test -tags fts5 ./internal/...
 cd sdk && pytest -v
╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌
