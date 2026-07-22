P5c — 协议联调（P5c.2 + P5c.3 + P5c.4 详细实施）                                                    
                                                                                                     
 Context                                                                                             

 P5c 是 T6 协议联调智能体的交付。原计划 4 个子 PR，P5c.1（加 cloudDisk
 字段）因语义重复（整个系统就叫 AgentDisk）经用户决定作废，本轮改为实施 P5c.2/3/4 三个独立子
 PR，每个独立 worktree + 独立分支 + 独立合并。

 ┌───────┬────────────────┬───────────────────────────────────────────────────────────┬──────────┐
 │ 子 PR │      范围      │                        关键交付物                         │ 预计行数 │
 ├───────┼────────────────┼───────────────────────────────────────────────────────────┼──────────┤
 │       │                │ internal/feature 包（atomic.Pointer +                     │          │
 │ P5c.2 │ 字段级灰度开关 │ fsnotify）；/v1/disk/admin/features                       │ ~750     │
 │       │                │ 管理端点；RequireFeature 中间件                           │          │
 ├───────┼────────────────┼───────────────────────────────────────────────────────────┼──────────┤
 │ P5c.3 │ BFS 压测       │ Go testing.B 表驱动 bench；参数化 Python seed             │ ~550     │
 │       │                │ 脚本；性能基线文档                                        │          │
 ├───────┼────────────────┼───────────────────────────────────────────────────────────┼──────────┤
 │ P5c.4 │ 协议级 e2e     │ Python sdk/tests/e2e/ 全链路测试；浏览器                  │ ~300     │
 │       │                │ t24-e2e-protocol.js 跨层验证                              │          │
 └───────┴────────────────┴───────────────────────────────────────────────────────────┴──────────┘

 依赖：三者完全独立，可任意顺序交付。建议 P5c.3 → P5c.2 → P5c.4（最复杂先做，最后用 e2e 验证全部）。

 预确认事实：
 - Go 1.25.0（go.mod）— atomic.Pointer[T] 原生支持
 - fsnotify v1.7.0 已在 go.sum（indirect），需在 go.mod 提升为 direct
 - Router 现有 feature gate 点：cfg.Okf.Enabled 在 router.go:244, 325, 380；cfg.WebAuthn.Enabled 在
 router.go:172
 - Preview 双端点：/preview/:id（JSON 信封）+ /preview/:id/html（raw HTML）
 - 现有测试基础设施：SDK 测试用真实 backend (localhost:9100)，浏览器 30 个测试到 t23，t24 是下一个

 ---
 P5c.2 — 字段级灰度开关（feature flags + hot-reload）

 Goal

 把 cfg.Okf.Enabled 升级成运行时可切换的字段级 feature flag，支持 fsnotify 热加载 + admin API
 持久化切换 + 中间件级路由网关。向下兼容：旧 config.yaml 不带 features: 块时按原逻辑工作。

 Scope

 - 新建 internal/feature 包：注册表 + atomic.Pointer 快照 + fsnotify watcher + viper 写回
 - 新建 internal/middleware/feature_gate.go：RequireFeature("name") 中间件
 - 新建 internal/handler/feature.go：GET/PATCH /v1/disk/admin/features
 - 修改 config/config.go：加 Features map[string]bool 字段
 - 修改 internal/router/router.go：实例化 registry，替换 4 处静态检查
 - 初始 5 个 flag：okf.reader、okf.writer、okf.graph_bfs、webauthn、public_directory

 文件清单

 新建：
 - internal/feature/flags.go (~180 行) — Registry struct + atomic.Pointer[Flags]，Get()、Set(name,
 v)、NewRegistry(cfg, path)
 - internal/feature/watcher.go (~90 行) — fsnotify goroutine，200ms debounce，Close() graceful
 shutdown
 - internal/feature/persist.go (~60 行) — viper 读-改-写只更新 features.* 键
 - internal/feature/flags_test.go (~150 行) — 默认值、Set/Get、未知 flag、持久化 round-trip、热加载
 - internal/middleware/feature_gate.go (~40 行) — 单次 atomic load，off 时 403
 - internal/handler/feature.go (~80 行) — List、Update (bind {name, enabled})
 - internal/handler/feature_test.go (~80 行) — 列表、更新、未知 flag 400、admin 鉴权

 修改：
 - config/config.go (line 15-25, 143-174) — 加 Features FeaturesConfig，默认值 seed
 - internal/router/router.go (line 23, 102, 172, 244, 325, 380) — 实例化 registry，替换静态 if
 cfg.*.Enabled，挂载 /features 路由
 - cmd/server/main.go — defer featureReg.Close()
 - go.mod — fsnotify 提升为 direct

 实施步骤

 1. 新 worktree：git worktree add .claude/worktrees/okf-p5c2-features -b feature/okf-p5c2-features
 feature/okf-integration
 2. config.go 加 FeaturesConfig map + 默认值（沿用现有 cfg.Okf.Enabled / cfg.WebAuthn.Enabled 作
 seed）
 3. internal/feature/flags.go — Registry + atomic.Pointer 模式：
 type Flags struct {
     OkfReader, OkfWriter, OkfGraphBFS, WebAuthn, PublicDirectory bool
 }
 type Registry struct {
     cur atomic.Pointer[Flags]
     cfgPath string
     mu sync.Mutex // 仅保护 Set 写持久化
 }
 4. internal/feature/persist.go — viper 读旧 yaml，只覆盖 features: 块，写回（保留注释目前 viper
 不支持，加 README 说明）
 5. internal/feature/watcher.go — fsnotify 监听 config.yaml，200ms debounce 避免抖动，重载失败保留旧
  flag 并 log
 6. internal/middleware/feature_gate.go — 闭包返回 gin.HandlerFunc，按名字查 flag
 7. internal/handler/feature.go — GET 返回 5 个 flag 状态，PATCH 接 {name, enabled} → 调 reg.Set +
 持久化
 8. router.go 替换：
   - line 172 cfg.WebAuthn.Enabled → reg.Get().WebAuthn
   - line 244/325/380 cfg.Okf.Enabled → reg.Get().OkfReader（reader 路由）/ OkfWriter（writer 路由）
   - 挂载：adminAPI.GET("/features", featureH.List) + adminAPI.PATCH("/features", featureH.Update)
 9. 测试：unit (flags_test) + integration (router 用 fake registry 验证路由可达性切换)
 10. lint + build + commit + merge

 Verification

 go test ./internal/feature/... ./internal/middleware/... ./internal/handler/... -count=1
 go test ./... -count=1
 make lint

 # 手动 smoke
 bash scripts/dev.sh restart
 # 1. 默认状态
 curl -H "Authorization: Bearer $ADMIN_JWT" http://localhost:9100/v1/disk/admin/features
 # 2. 关掉 okf.reader
 curl -X PATCH -H "Authorization: Bearer $ADMIN_JWT" -d '{"name":"okf.reader","enabled":false}'
 http://localhost:9100/v1/disk/admin/features
 # 3. 验证 OKF 路由 404
 curl -i http://localhost:9100/v1/disk/okf/bundles
 # 4. 重启后保持关闭（持久化生效）
 bash scripts/dev.sh restart
 curl -H "Authorization: Bearer $ADMIN_JWT" http://localhost:9100/v1/disk/admin/features

 DoD

 - internal/feature 包单测全过，覆盖率 ≥ 80%
 - go test ./... -count=1 0 失败
 - make lint 0 issue
 - PATCH 持久化：重启后 flag 状态保留
 - fsnotify 热加载：< 500ms 生效（手动 smoke）
 - cfg.Okf.Enabled=false 仍能首次启动正确禁用 OKF（向下兼容）
 - atomic.Pointer[Flags] Get 零锁竞争（go test -race 通过）
 - 5 个 flag 全部可切，未知 flag PATCH 返回 400

 风险

 - viper 写回丢注释：viper 的 WriteConfig 不保留 YAML 注释。本期范围接受 — 在 config.yaml 顶部加
 README 注释说明；后续 P5c.x 可换 yaml.v3 保注释。
 - PATCH 触发 fsnotify 自循环：自己写文件会触发 watcher 重新加载，需要 debounce + 状态比较避免循环。
 - router 启动时 flag 离线：注册路由时 flag 必须已知（router
 是静态注册）。解决：所有路由都注册，运行时用中间件 gate，而不是条件注册。

 ---
 P5c.3 — BFS 性能压测

 Goal

 为 OKF BFS 三大查询（Reachable / Subgraph / ShortestPath）建立性能基线，覆盖 1K/10K/100K
 节点规模，验证 P95 < 500ms 目标，定位缓存/N+1 热点。

 Scope

 - 新建 internal/service/okf_bfs_bench_test.go — 表驱动 testing.B
 - 重构 okf_bfs_test.go 把 newBFSService 抽成 *testing.TB 友好版本（bench 复用）
 - 新建 scripts/seed_okf_large.py — 参数化大规模 seed（节点数、边密度、拓扑分布）
 - 新建 docs/perf/bfs-baseline.md — 基线表（P50/P95/P99 + cache 命中率）
 - 新建 scripts/bench-bfs.sh — 一键 bench + 输出收集

 文件清单

 新建：
 - internal/service/okf_bfs_bench_test.go (~280 行) — 4 个 Benchmark 函数：Reachable (depth
 1/2/3)、Subgraph、ShortestPath (found + no-path)、Neighbors；每个表驱动 (N, edgeMult, dist)
 笛卡尔积
 - scripts/seed_okf_large.py (~150 行) — 参数 --nodes、--edges-per-node、--dist
 {uniform,powerlaw}、--bundle-name；幂等
 - docs/perf/bfs-baseline.md (~100 行) — 1K/10K/100K 三档基线 + 与
 docs/site/architecture/okf.md:321-330 既定目标对齐
 - scripts/bench-bfs.sh (~25 行) — go test -bench=... -count=5 -run=^$ ./internal/service/ | tee
 docs/perf/bfs-latest.txt

 修改：
 - internal/service/okf_bfs_test.go (~10 行) — 把 newBFSService 拆成 newBFSServiceRepos(tb) 给 bench
  复用
 - scripts/README.md — 加 seed_okf_large.py 说明

 实施步骤

 1. 新 worktree：git worktree add .claude/worktrees/okf-p5c3-bfs-bench -b feature/okf-p5c3-bfs-bench
  feature/okf-integration
 2. 重构 okf_bfs_test.go:81-90 的 newBFSService：抽成 newBFSServiceRepos(tb testing.TB, nodes
 []model.OkfNode, edges []model.OkfEdge) *OkfService
 3. 实现 seedScaleGraph(b, N, edgeMult, dist)：
   - uniform：每个节点随机选 edgeMult 个目标
   - powerlaw：Zipf 分布（少数 hub 节点高入度），更接近真实 OKF 拓扑
   - Zipf 种子固定（rand.NewSource(42)）保证可复现
 4. 4 个 Benchmark：
   - BenchmarkReachable — 表驱动 (1K/10K/100K, 1x/5x/10x, uniform/powerlaw) × depth 1/2/3
   - BenchmarkSubgraph — 全量拉取，关注 MaxNodes 截断
   - BenchmarkShortestPath — 两种：找到路径（最近 2 跳）+ 无路径（最坏情况全展开）
   - BenchmarkNeighbors — 1 跳基线对照
 5. 每个 bench：b.ReportAllocs() + b.SetBytes(0) + 5 次 count 用 benchstat 比较
 6. cold cache vs warm cache：cold 用 NoOpGraphCache；warm 在 b.ResetTimer() 前预填
 7. MaxNodes: HardBFSMaxNodes (1000) 显式传，测算法不测截断
 8. 跑 bench 收集基线，填 docs/perf/bfs-baseline.md：
   - 1K 节点 / 5x 边：Reachable depth=3 P50 目标 < 50ms
   - 10K 节点 / 5x 边：Reachable depth=3 P50 目标 < 200ms
   - 100K 节点 / 5x 边：Reachable depth=3 P50 目标 < 500ms（与 docs/site/architecture/okf.md:321-330
  一致）
 9. seed_okf_large.py — 复用 seed_okf_demo.py 的认证模式（JWT 生成），用 client.write_markdown
 批量造节点
 10. lint + build + commit + merge

 Verification

 # 单 bench 快速验证
 go test -bench=BenchmarkReachable -benchmem -run=^$ ./internal/service/ -count=3

 # 全套 bench
 bash scripts/bench-bfs.sh

 # 真实 DB 验证（可选）
 bash scripts/dev.sh start
 python3 scripts/seed_okf_large.py --nodes 10000 --edges-per-node 5 --bundle-name bench-10k

 # 跨 run 比较
 go test -bench=. -count=5 -run=^$ ./internal/service/ > old.txt
 # (改代码)
 go test -bench=. -count=5 -run=^$ ./internal/service/ > new.txt
 benchstat old.txt new.txt

 DoD

 - 所有 bench 不依赖外部 DB/Redis（用 fake repos）
 - go test -bench=... -benchmem 全跑通
 - 1K/10K/100K 三档基线录入 docs/perf/bfs-baseline.md
 - Reachable depth=3 P50 在 100K 节点规模 < 500ms（或文档化差距 + 根因）
 - seed_okf_large.py --nodes 1000 在干净 backend 上 < 60s 完成
 - 5 次运行方差 < 15%（用 benchstat 验证）
 - make lint 0 issue，go test ./... -count=1 0 失败

 风险

 - fake repo 不真实：fake 的 map 查询比真 MySQL 快 10-100x，bench 主要测算法不测 DB。要测 DB 端，靠
 seed_okf_large.py + 真实 backend 手测。
 - 100K 节点 bench 内存：fake repo map 占内存，每个节点 ~200B，100K = 20MB，OK。
 - miniredis 依赖：warm cache bench 想用真 Redis 行为，要么加 miniredis
 依赖（go.mod），要么用接口注入 fake cache。决策：用现有 fake cache 接口（okf_graph_cache.go 的
 Cache interface），不引入新依赖。

 ---
 P5c.4 — 协议级端到端测试

 Goal

 补齐 SDK → backend → 落盘 → 分享 → 预览 全链路跨层验证：SDK 写、原始 HTTP
 读、浏览器渲染预览页，验证协议在真实链路里端到端一致。

 Scope

 - 新建 sdk/tests/e2e/ 子包 + 全链路 e2e 测试
 - 新建浏览器测试 t24-e2e-protocol.js — admin fetch 上传 → 验证预览页渲染
 - 复用 sdk/tests/conftest.py 的 user_token / client / admin_token / _cleanup fixtures

 文件清单

 新建：
 - sdk/tests/e2e/__init__.py (空) — 包标记
 - sdk/tests/e2e/conftest.py (~15 行) — pytest_plugins = ("..conftest",)；backend_reachable fixture
 ping /health，失败 pytest.skip()
 - sdk/tests/e2e/test_share_preview_lifecycle.py (~180 行) — 单测函数 10 步断言
 - test/browser/tests/t24-e2e-protocol.js (~120 行) — 浏览器侧跨层验证

 测试场景

 Python e2e (test_share_preview_lifecycle.py)：

 1. SDK: client.upload_bytes("docs/preview-test.md", b"# Title\n\ncontent") → file_id
 2. SDK: client.create_share("docs/preview-test.md", is_file=True, extract_code="abc123") →
 share_code
 3. RAW HTTP: GET /v1/disk/shares/{code} (公开) → 200, share.code == share_code
 4. RAW HTTP: POST /v1/disk/shares/{code}/access (body: {"extract_code": "abc123"}) → 200, file
 列表非空
 5. RAW HTTP: GET /v1/disk/files/{file_id}/download-token → downloadToken
 6. RAW HTTP: GET /v1/disk/files/download?t={token} Accept: application/json → 200, downloadUrl 非空
 7. RAW HTTP: GET downloadUrl → bytes 与 #1 上传的 content 一致
 8. RAW HTTP: GET /v1/disk/files/{file_id}/preview → 200, data.html contains "<h1>Title</h1>"
 9. SDK: client.revoke_share(share_id)
 10. RAW HTTP: GET /v1/disk/shares/{code} → 非 200（已撤销），cleanup finally 块兜底

 Browser e2e (t24-e2e-protocol.js)：

 1. ab.login('user001', 'test123')
 2. fetch('/v1/disk/files/upload', FormData with cloudDisk-test.md content "# Browser E2E")
 3. fetch('/v1/disk/shares', { resourceId, resType: 'file', extractCode: 'abc123' }) → share
 4. ab.navigate('http://localhost:9101/share/{code}') (web 前端分享页)
 5. fill extract code "abc123", submit
 6. fetch preview iframe, assert rendered HTML contains "<h1>Browser E2E</h1>"
 7. fetch('/v1/disk/shares/{id}/revoke', DELETE)
 8. reload share page, assert "已失效"
 9. ab.closeBrowser()

 实施步骤

 1. 新 worktree：git worktree add .claude/worktrees/okf-p5c4-e2e -b feature/okf-p5c4-e2e
 feature/okf-integration
 2. 新建 sdk/tests/e2e/{__init__.py, conftest.py}
 3. test_share_preview_lifecycle.py 单测函数，用 httpx.Client(base_url=BASE_URL) 做原始 HTTP，与 SDK
  client 交叉验证
 4. backend_reachable fixture ping /health，断网时 skip 不 fail
 5. cleanup 在 finally 块 + autouse cleanup fixture 双保险
 6. 浏览器测试 t24：
   - 复用 t11-share.js 的 createShareAPI / revokeShareAPI helper（直接 import 或拷贝）
   - 复用 t06-file-preview.js 的预览断言模式
   - 直接 fetch 上传文件（不依赖 SDK），强调"跨层"
 7. 跑 pytest + 浏览器 runner
 8. lint + build + commit + merge

 Verification

 bash scripts/dev.sh start

 # Python e2e
 cd sdk && python -m pytest tests/e2e/test_share_preview_lifecycle.py -v

 # 浏览器 e2e
 cd test/browser && node runner.js t24-e2e-protocol

 # 回归（确保新测试不破坏 t11/t06）
 cd test/browser && node runner.js t11 t06

 # SDK lint
 cd sdk && ruff check . && ruff format --check . && mypy agentdisk tests

 DoD

 - pytest sdk/tests/e2e/test_share_preview_lifecycle.py -v 全过
 - backend 离线时测试 skip（不 fail）
 - 10 步断言全部触发（用 pytest -v --tb=short 验证每步都跑了）
 - 浏览器 t24-e2e-protocol.js 通过
 - t11-share.js / t06-file-preview.js 无回归
 - cd sdk && ruff check . && mypy agentdisk tests 全过
 - 测试总耗时 < 15s
 - 无数据残留（cleanup finally + autouse 双保险）

 风险

 - 预览端点响应结构：/preview/:id 返回 JSON 信封 ({code, message, data: {html}})，不是直接
 HTML；/preview/:id/html 才是 raw HTML。Python e2e 用 JSON 端点解析 data.html，浏览器测试可选用 raw
 HTML 端点直接 iframe 渲染。
 - share 端点路径：本计划基于 explorer 报告的 /v1/disk/shares/{code} 与 /access 子路径，实施时需
 grep internal/router/router.go 确认实际路径。
 - web 前端分享页 URL：浏览器测试需要 web 前端有分享访问页（/share/:code），需先 grep
 web/src/router/ 确认存在。若不存在，回退到纯 fetch 验证（不 navigate）。

 ---
 全局交付顺序

 按依赖 + 风险排序：

 1. P5c.3 先做（独立压测，零外部影响，最先验证算法基线）
 2. P5c.2 次做（feature flag 框架，影响 router 启动路径，需要回归测试）
 3. P5c.4 最后做（e2e 验证前两个 PR 没破坏协议）

 每个 PR 单独 worktree、单独 review、单独合并到 feature/okf-integration。三个全部合并后 P5c
 视为完工。
╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌