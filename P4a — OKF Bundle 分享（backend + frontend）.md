P4a — OKF Bundle 分享（backend + frontend）                                                         
                                                                                                     
 Context                                                                                             
                                                                                                     
 P4b+c 落地了 OKF UI（列表 / 详情 / 4 tab / 图谱），但用户除了自己浏览，没法把 bundle                
 分享给协作者。现有的 DiskShare 系统（internal/service/share.go）只支持 resType: "file" |
 "folder"，OKF bundle 没有分享入口。

 P4a 把 "bundle" 加成第三种 resType：用户在 bundle 详情页点 [分享]，生成 share
 link，接收者无需登录即可看到 2-tab 简化视图（节点 + 图谱），read-only。点节点行弹出 frontmatter
 drawer，drawer 里点 [预览 markdown] 在 modal 内联渲染 markdown（不复用 /preview/:fileId 路由）。

 范围裁剪：
 - 不做 统计 tab / 死链 tab（维护工具，对只读访问者价值低）
 - 不做 写入 / 注销 / 重建索引（read-only，权限太复杂）
 - 不做 一次性 JWT 交换中间件（per-request 重发 extractCode 更简单）

 用户已确认决策

 ┌──────────────────┬───────────────────────────────────────────────────────────────────────────┐
 │      决策点      │                                 选定方案                                  │
 ├──────────────────┼───────────────────────────────────────────────────────────────────────────┤
 │ 收件人 UX        │ 简化 2-tab 视图（节点 + 图谱），无统计/死链 tab                           │
 ├──────────────────┼───────────────────────────────────────────────────────────────────────────┤
 │ Markdown 预览    │ 复用现有 drawer，drawer 底部按钮开 modal 内联渲染                         │
 ├──────────────────┼───────────────────────────────────────────────────────────────────────────┤
 │ 后端端点结构     │ Option A：新建专用公开端点 /v1/disk/share/:code/...，与 HybridAuth OKF    │
 │                  │ 端点分离                                                                  │
 ├──────────────────┼───────────────────────────────────────────────────────────────────────────┤
 │ Markdown         │ frontmatter API 内联返回 markdown 字段，避免再开 preview 端点             │
 │ 数据来源         │                                                                           │
 ├──────────────────┼───────────────────────────────────────────────────────────────────────────┤
 │ 前端路由         │ 复用 /share/:code，ShareAccessPage 检测 resType 后分支渲染                │
 ├──────────────────┼───────────────────────────────────────────────────────────────────────────┤
 │ Worktree         │ 新 worktree okf-p4a-share off feature/okf-integration                     │
 └──────────────────┴───────────────────────────────────────────────────────────────────────────┘

 现状盘点

 后端：
 - internal/model/share.go:6-18 — DiskShare 表，ResType 16 字符字段（已支持任意字符串）
 - internal/service/share.go:54-83 — CreateShare switch 硬编码 file/folder；SetGrantChecker
 注入模式（line 49）
 - internal/router/router.go:362-364 —
 公开路由：/v1/disk/share/:code、/share/access、/share/download
 - internal/router/router.go:316-352 — OKF 路由走 HybridAuth（JWT/APIKey），无匿名访问
 - internal/repository/okf_bundle.go（待确认）— OkfBundleRepo.GetByID 已存在

 前端：
 - web/src/components/share/CreateShareModal.tsx — 文件专用，form → result 两阶段
 - web/src/api/share.ts — shareApi.create({resType}) 已是 generic
 - web/src/pages/ShareAccessPage.tsx — /share/:code 兑换页，目前只支持下载
 - web/src/pages/OkfBundleDetailPage.tsx:117-125 — 按钮行（刷新/重建/注销）可加 [分享]
 - web/src/components/okf/OkfNodeList.tsx、OkfGraphView.tsx、OkfFrontmatterDrawer.tsx — 复用对象
 - web/src/api/okf.ts — okfApi，目前只走带 cookie 的 apiClient

 实施步骤

 Step 1 — 新 worktree

 git -C /Users/wangyun/Documents/work/gitlab/agent-disk worktree add \
   .claude/worktrees/okf-p4a-share -b feature/okf-p4a-share feature/okf-integration

 EnterWorktree 进入 .claude/worktrees/okf-p4a-share。

 Step 2 — 后端 service 层扩展

 internal/service/share.go：
 - 加 bundleResourceRepo interface（GetByID(id uint64) (*model.OkfBundle, error)）
 - 加 bundleVisibility interface（或复用现有 public-dir visibility checker）
 - ShareService 加字段 bundleRepo、bundleVisibility
 - 加 setter SetBundleRepo、SetBundleVisibility（mirror SetGrantChecker at line 49）
 - 在 CreateShare switch 加 case "bundle":：
 case "bundle":
     b, err := s.bundleRepo.GetByID(resourceID)
     if err != nil { return nil, fmt.Errorf("Bundle 不存在") }
     // 公共目录可见性检查（bundle 归属 public dir）
     visible, err := s.bundleVisibility.IsUserGrantedForBundle(resourceID, userID)
     if err != nil || !visible { return nil, fmt.Errorf("无权分享该 Bundle") }

 新文件 internal/service/okf_share.go — 只读 reader：
 type OkfShareReader struct { /* bundleRepo, nodeRepo, edgeRepo, pdSvc */ }
 func (r *OkfShareReader) GetShareBundle(share, bundleID) (*OkfBundleDTO, error)
 func (r *OkfShareReader) ListShareNodes(share, bundleID, typeFilter, tagFilter) ([]OkfNodeDTO,
 error)
 func (r *OkfShareReader) SubgraphForShare(share, bundleID, types, maxNodes) (graph, error)
 func (r *OkfShareReader) NeighborsForShare(share, nodeID) (graph, error)
 func (r *OkfShareReader) GetShareNode(share, nodeID) (node + markdown, error) // 内联 markdown
 每个方法都断言 share.ResType == "bundle" 且 share.ResourceID == bundleID，否则返回
 ErrShareResTypeMismatch。

 Step 3 — 后端 handler 层

 新文件 internal/handler/okf_share.go：
 - OkfShareHandler 结构体，依赖 shareSvc、okfShareReader
 - 5 个
 handler：GetShareBundle、ListShareNodes、GetShareSubgraph、GetShareNodeNeighbors、GetShareNode
 - 每个 handler：取 :code、:bundleId/:nodeId，调 shareSvc.GetShareByCode 验证 +
 IncrementVisitCount，转发给 reader

 Step 4 — 后端路由接线

 internal/router/router.go（公开路由块 line 361-365 追加）：
 okfShareH := handler.NewOkfShareHandler(shareSvc, okfShareReaderSvc)
 r.GET("/v1/disk/share/:code/bundle", okfShareH.GetShareBundle)
 r.GET("/v1/disk/share/:code/nodes", okfShareH.ListShareNodes)
 r.GET("/v1/disk/share/:code/subgraph", okfShareH.GetShareSubgraph)
 r.GET("/v1/disk/share/:code/nodes/:nodeId/neighbors", okfShareH.GetShareNodeNeighbors)
 r.GET("/v1/disk/share/:code/nodes/:nodeId", okfShareH.GetShareNode)

 注意路由顺序：:code/bundle 是 :code 的 sub-path，与 GET /v1/disk/share/:code 不冲突（Gin
 路由优先具体路径）。

 依赖注入：在 RegisterRoutes 里 shareSvc.SetBundleRepo(okfBundleRepo);
 shareSvc.SetBundleVisibility(publicDirSvc)，构造 okfShareReaderSvc :=
 service.NewOkfShareReader(...)。

 Step 5 — 后端测试

 - internal/service/share_test.go（如不存在则新建）— bundle case：ownership pass / not-owner-fail /
 non-existent-bundle
 - internal/handler/okf_share_test.go（新建）— 5 个端点：code 错误、resType 不匹配、share.ResourceID
  != bundleId、happy path
 - go test ./... 必须 0 fail

 Step 6 — 前端 API client

 web/src/api/okf.ts 追加 okfShareApi 对象（与 okfApi 并列）：
 - 用单独的 publicClient（axios.create，不带 cookie/auth interceptor）
 - 5 个方法：getBundle(code)、listNodes(code, type?, tag?)、subgraph(code, req)、neighbors(code,
 nodeId)、getNode(code, nodeId)

 Step 7 — 重构 OkfNodeList + OkfGraphView 接受 fetcher

 web/src/components/okf/OkfNodeList.tsx：
 - 加可选 prop fetcher?: (bundleId, type?, tag?) => Promise<{nodes: OkfNode[]}>，默认走
 okfApi.listNodes
 - 加可选 prop typesFetcher?: () => Promise<OkfTypeCount[]>，默认走 okfApi.aggregateTypes；share
 模式传 undefined → 自动隐藏 type 过滤下拉
 - 加可选 prop onNodeClickOverride?: (node) => void，share 模式用于打开 markdown modal 而不是 drawer
  的 preview 跳转

 web/src/components/okf/OkfGraphView.tsx：
 - 加可选 prop subgraphFetcher、neighborsFetcher，默认走 okfApi.subgraph / okfApi.neighbors

 跑 node runner.js t22 验证 auth 路径无回归。

 Step 8 — 新组件 OkfShareBundleView + Markdown modal

 web/src/components/okf/OkfShareBundleView.tsx（新建）：
 - props: code: string, bundleId: number, extractCode?: string
 - 内部 useQuery(['share-bundle', code], () => okfShareApi.getBundle(code))
 - AntD <Tabs> 2 个：节点 / 图谱（不渲染统计 / 死链）
 - 把 fetcher 传给 OkfNodeList/OkfGraphView
 - 复用 OkfFrontmatterDrawer，但 drawer 底部 [预览 markdown] 改为打开 OkfShareMarkdownModal

 web/src/components/okf/OkfShareMarkdownModal.tsx（新建）：
 - props: code: string, nodeId: number | null
 - useQuery(['share-node', code, nodeId], () => okfShareApi.getNode(code, nodeId!))
 - <Modal> 里 <ReactMarkdown> 渲染 data.markdown

 Step 9 — ShareAccessPage 分支

 web/src/pages/ShareAccessPage.tsx：
 - access 成功后，如果 shareInfo.resType === 'bundle' → 渲染 <OkfShareBundleView code={code}
 bundleId={shareInfo.resourceId} extractCode={extractCode} />
 - 否则继续现有下载 UI

 Step 10 — 通用化 CreateShareModal + 加按钮

 web/src/components/share/CreateShareModal.tsx：
 - props 从 {file: DiskFile | null} 改为 {resource: {id: number, name: string} | null, resType:
 'file' | 'folder' | 'bundle'}
 - title 显示 resource.name
 - shareApi.create({resourceId: resource.id, resType, ...})

 web/src/pages/OkfBundleDetailPage.tsx：
 - 头部按钮行加 <Button icon={<ShareAltOutlined />} onClick={() => setShareTarget(bundle)}>分享
 Bundle</Button>
 - 底部加 <CreateShareModal resource={shareTarget ? {id: shareTarget.bundleId, name:
 shareTarget.title} : null} resType="bundle" open={!!shareTarget} onClose={() =>
 setShareTarget(null)} />

 Step 11 — 浏览器测试 t23

 test/browser/tests/t23-okf-share.js（新建）：
 1. seed bundle（复用 python3 scripts/seed_okf_demo.py）
 2. login user001 → 进入 /okf/:bundleId → 点 [分享 Bundle] → 创建无 extractCode 的 share
 3. 解析返回的 shareCode
 4. 关闭当前 session（ab.closeAll()），重新打开浏览器无 cookie
 5. 访问 /share/:code → 验证 access 页面渲染
 6. 点 [访问] → 验证 OkfShareBundleView 渲染，2 个 tab
 7. 切到图谱 tab → 验证 cytoscape canvas 存在
 8. 切回节点 tab → 点行 → 验证 drawer 弹出
 9. drawer 点 [预览 markdown] → 验证 modal 渲染 markdown
 10. 验证无刷新 / 重建 / 注销按钮可见
 11. 负向：直接 fetch('/v1/disk/okf/bundles/:id') 无 auth → 401
 12. cleanup：用原 session revoke share

 Step 12 — Lint + 构建

 cd web && npm run lint
 cd web && npm run build
 go test ./...
 make lint  # 后端 golangci-lint

 Step 13 — 提交、合并、推送

 git add internal/service/share.go internal/service/okf_share.go \
         internal/handler/okf_share.go internal/handler/okf_share_test.go \
         internal/router/router.go \
         web/src/api/okf.ts \
         web/src/components/okf/OkfShareBundleView.tsx \
         web/src/components/okf/OkfShareMarkdownModal.tsx \
         web/src/components/okf/OkfNodeList.tsx \
         web/src/components/okf/OkfGraphView.tsx \
         web/src/components/share/CreateShareModal.tsx \
         web/src/pages/ShareAccessPage.tsx \
         web/src/pages/OkfBundleDetailPage.tsx \
         test/browser/tests/t23-okf-share.js
 git commit -m "feat(okf): P4a bundle sharing — backend share resType + simplified read-only
 recipient view"

 # 主 worktree merge + push
 git -C /Users/wangyun/Documents/work/gitlab/agent-disk merge --no-ff feature/okf-p4a-share \
   -m "Merge PR-P4a: OKF bundle sharing"
 git -C /Users/wangyun/Documents/work/gitlab/agent-disk push origin feature/okf-integration

 关键文件清单

 新建：
 - internal/service/okf_share.go（OkfShareReader 服务，~150 行）
 - internal/handler/okf_share.go（5 个公开端点 handler，~120 行）
 - internal/handler/okf_share_test.go（handler 单测，~150 行）
 - web/src/components/okf/OkfShareBundleView.tsx（2-tab 容器，~120 行）
 - web/src/components/okf/OkfShareMarkdownModal.tsx（内联 markdown modal，~50 行）
 - test/browser/tests/t23-okf-share.js（端到端测试，~250 行）

 修改：
 - internal/service/share.go（加 bundle case + visibility 注入，+30 行）
 - internal/router/router.go（加 5 个公开路由 + DI 接线，+20 行）
 - web/src/api/okf.ts（加 okfShareApi 对象，+50 行）
 - web/src/components/okf/OkfNodeList.tsx（fetcher 注入，+15 行）
 - web/src/components/okf/OkfGraphView.tsx（fetcher 注入，+15 行）
 - web/src/components/share/CreateShareModal.tsx（通用化，~30 行改动）
 - web/src/pages/ShareAccessPage.tsx（分支 resType，+20 行）
 - web/src/pages/OkfBundleDetailPage.tsx（加分享按钮，+10 行）

 DoD

 - go test ./... 全部通过（含新 okf_share_test）
 - make lint 0 错误 0 警告
 - cd web && npm run lint 0 错误（已有 MFA 文件的 1 warning 是 pre-existing，不算）
 - cd web && npm run build 0 错误
 - cd test/browser && node runner.js t23 通过
 - cd test/browser && node runner.js 不引入回归（t22 OKF UI 仍通过；t11 文件分享仍通过）
 - 合并到 feature/okf-integration 并推送

 风险与备注

 - extractCode per-request 重发：每次公开 OKF 请求都在 query 里带 ?extractCode=xxx，backend 在
 okf_share.go 里重新校验。简单但需前端在每个 fetcher 里拼 URL。
 - OkfNodeList/OkfGraphView 重构：fetcher 注入必须保持 auth 路径行为不变。t22 是回归防线。
 - Cytoscape type 过滤：share 模式没 aggregateTypes 端点，禁用 type 过滤下拉。
 - markdown 内联：getNodeForShare 返回完整 markdown body。OKF 节点都是 markdown 文档，单文件预期 <
 100KB，安全。若未来超长可改 streaming。
 - 路由冲突：Gin 的 :code/bundle 与 :code 共存需测一遍（实测 Gin 支持，但 t23 必须验证）。

 验证方式

 # 1. 后端测试
 go test ./internal/service/... ./internal/handler/... -run Share -v

 # 2. 后端 lint
 make lint

 # 3. 前端 lint + build
 cd web && npm run lint && npm run build

 # 4. 浏览器手动
 #   - 启栈：bash scripts/dev.sh start
 #   - seed：python3 scripts/seed_okf_demo.py
 #   - 登录 → /okf/:bundleId → 点分享 → 拷贝链接
 #   - 无痕窗口粘贴链接 → 验证 2-tab + drawer + markdown modal

 # 5. 浏览器回归
 cd test/browser && node runner.js

 后续子 PR 概要（不在本轮范围）

 - P5c：协议联调 + 灰度开关 + 10 万节点 BFS 压测 + e2e
 - 写入 UI（可选）：API Key 输入 + markdown 编辑器，UI 内直接写知识
 - bundle 完整 4-tab 分享：如果用户反馈简化视图不够，再加统计 + 死链 tab 到 share view
╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌