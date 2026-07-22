P5c — 协议联调（全景规划 + P5c.1 详细）                                                             
                                                                                                     
 Context                                                                                             
                                                                                                     
 P4 系列已完成 OKF 知识库的核心能力（模型、UI、分享）。P5c 是 T6 协议联调智能体的交付，目标是把 SDK  
 ↔ backend 协议补全、灰度开关机制化、BFS 性能验证、e2e 测试覆盖。这 4 项是 plan                      
 中规划的下一个里程碑，也是协议层最后一块拼图。                                                      

 由于范围很大（侦察估计 2000+ 行改动），用户决定拆 4 个子 PR                                         
 独立交付，每个可单独验收、单独回滚。本轮 plan 给出 4 个子 PR 的全景概要，并对 P5c.1 协议字段        
 做详细实施步骤。后续 P5c.2/3/4 在 P5c.1 交付后单独规划。                                            
                                                                                                     
 本轮范围（P5c.1）：给协议加 cloudDisk 字段（诊断/元数据用途，不影响路由）。SDK → backend 单向：SDK  
 上报，backend 接收并存储到 disk_file.cloud_disk，响应不返回。旧 SDK 不带该字段时 backend            
 落空值不报错，保证向下兼容。

 P5c 全景概要

 ┌───────┬────────────────┬───────────────────────────────────────────────────────────┬──────────┐
 │ 子 PR │      范围      │                        关键交付物                         │ 预计行数 │
 ├───────┼────────────────┼───────────────────────────────────────────────────────────┼──────────┤
 │ P5c.1 │ 协议字段扩展   │ SDK 加 cloudDisk 参数，backend 落盘到                     │ ~400     │
 │       │                │ disk_file.cloud_disk                                      │          │
 ├───────┼────────────────┼───────────────────────────────────────────────────────────┼──────────┤
 │       │                │ config.go 加 FieldFlag                                    │          │
 │ P5c.2 │ 字段级灰度开关 │ 机制，热加载（fsnotify）；/v1/disk/admin/features         │ ~600     │
 │       │                │ 管理端点                                                  │          │
 ├───────┼────────────────┼───────────────────────────────────────────────────────────┼──────────┤
 │ P5c.3 │ BFS 压测       │ 10万节点 seed 脚本；Go bench 测试；性能基线文档；P95 <    │ ~800     │
 │       │                │ 500ms 验证                                                │          │
 ├───────┼────────────────┼───────────────────────────────────────────────────────────┼──────────┤
 │ P5c.4 │ 协议级 e2e     │ SDK → gateway → backend → 落盘 → 分享 → 预览 全链路       │ ~500     │
 │       │                │ e2e；pytest + go test 双向                                │          │
 └───────┴────────────────┴───────────────────────────────────────────────────────────┴──────────┘

 依赖：P5c.2 不依赖 P5c.1（独立机制）；P5c.3 不依赖前两个（独立压测）；P5c.4 依赖
 P5c.1（要测新协议字段）。可并行启动 P5c.2/3，P5c.4 排最后。

 ---
 P5c.1 详细实施

 现状盘点

 SDK 侧（Python）：
 - sdk/src/agentdisk/client.py:62-85 — AgentDiskClient 构造函数，现有参数 base_url、token、api_key
 - sdk/src/agentdisk/api/file.py:21-53 — 上传 API，现有 agent_id 通过 data
 参数条件传：**({"agentId": agent_id} if agent_id else {})
 - sdk/src/agentdisk/client.py:131-179 — upload_file() 和 upload_bytes()，都接受 agent_id 参数

 Backend 侧（Go）：
 - internal/middleware/auth_hybrid.go:22-97 — HybridAuth，从 JWT claims 提取 userId/agentId（35-36
 行），39-41 行有 form data fallback 机制
 - internal/handler/file.go:30-59 — 私有目录 UploadFile
 - internal/handler/public_directory.go:171-191 — 公共目录 UploadFile
 - internal/service/file.go:49-93 — UploadFileWithGroup，62-73 行给 DiskFile 赋值，已有
 SourceAgent、SourceAgentGroup
 - internal/model/file.go:6-26 — DiskFile 表，SourceAgent(17行)、SourceAgentGroup(18行)
 是同类诊断字段

 向下兼容参考：
 - internal/model/okf.go:53,58 — omitempty + GORM size tag 模式

 用户已确认决策

 ┌────────────────┬─────────────────────────────────────────────────────────────────┐
 │     决策点     │                            选定方案                             │
 ├────────────────┼─────────────────────────────────────────────────────────────────┤
 │ cloudDisk 用途 │ 诊断/元数据字段（不影响路由、不做权限判断）                     │
 ├────────────────┼─────────────────────────────────────────────────────────────────┤
 │ 流向           │ SDK → backend 单向（响应不返回，避免 SDK 启动时验证逻辑复杂化） │
 ├────────────────┼─────────────────────────────────────────────────────────────────┤
 │ 存储           │ disk_file 表加 cloud_disk 列（与 SourceAgent 同类）             │
 ├────────────────┼─────────────────────────────────────────────────────────────────┤
 │ 提取方式       │ Form data fallback（旧 SDK 无 JWT 重发，参考 agentId 模式）     │
 ├────────────────┼─────────────────────────────────────────────────────────────────┤
 │ 兼容           │ 空值允许，旧 SDK 不传时落 ""，不报错                            │
 └────────────────┴─────────────────────────────────────────────────────────────────┘

 实施步骤

 Step 1 — 新 worktree

 git -C /Users/wangyun/Documents/work/gitlab/agent-disk worktree add \
   .claude/worktrees/okf-p5c1-protocol -b feature/okf-p5c1-protocol feature/okf-integration

 EnterWorktree 进入。

 Step 2 — SDK 加 cloud_disk 参数

 sdk/src/agentdisk/client.py（构造函数 + upload 方法）：
 - 构造函数加 cloud_disk: str = "" 参数（可选，默认空字符串）
 - 存到 self._cloud_disk，在 upload_file / upload_bytes 里透传
 - 不强制要求 — 旧调用方式保持工作

 sdk/src/agentdisk/api/file.py（API 层）：
 - upload_file() 和 upload_bytes() 加 cloud_disk: str = "" 参数
 - 在 data dict 里条件加：**({"cloudDisk": cloud_disk} if cloud_disk else {})

 sdk/src/agentdisk/models/file.py（响应模型，可选）：
 - DiskFile 加 cloudDisk: str = "" 字段（response 模型仍会接收 backend 返回值，但 backend
 不主动塞，所以默认空）

 Step 3 — Backend model 加字段

 internal/model/file.go（DiskFile 表）：
 - 加字段：CloudDisk string gorm:"size:64" json:"cloudDisk,omitempty"``
 - 放在 SourceAgent、SourceAgentGroup 附近，标注同类诊断字段

 Step 4 — HybridAuth 提取 cloudDisk

 internal/middleware/auth_hybrid.go（22-97 行附近）：
 - 参考 agentId 的 fallback 模式（39-41 行）
 - 优先从 JWT claims 提取 cloudDisk（如果 JWT 有）
 - 否则从 form data 提取（c.PostForm("cloudDisk")）
 - 设置到 c.Set("cloudDisk", cloudDisk)，handler 用 c.MustGet("cloudDisk") 或
 c.GetString("cloudDisk") 取

 Step 5 — Handler 接收并传给 service

 internal/handler/file.go:30-59（私有 UploadFile）：
 - 取 cloudDisk := c.GetString("cloudDisk")
 - 传给 fileSvc.UploadFileWithGroup(...) 调用（加一个参数，或封装成 struct）

 internal/handler/public_directory.go:171-191（公共 UploadFile）：
 - 同上

 Step 6 — Service 落盘

 internal/service/file.go:49-93（UploadFileWithGroup）：
 - 函数签名加 cloudDisk string 参数
 - 62-73 行附近赋值：file.CloudDisk = cloudDisk
 - 在 SourceAgent、SourceAgentGroup 旁边

 Step 7 — 数据库 schema

 sql/schema.sql（disk_file 表）：
 - 加列：cloud_disk VARCHAR(64) DEFAULT '' COMMENT '云盘实例标识（诊断）'

 自动迁移：
 - GORM AutoMigrate（internal/repository/migrate.go）会自动加列，不需要手工 SQL；schema.sql
 仅作为参考

 Step 8 — 测试

 SDK 测试（sdk/tests/）：
 - 加 test_cloud_disk.py：构造带 cloud_disk 的 client，上传文件，断言 form data 里有 cloudDisk 字段

 Backend 测试：
 - internal/middleware/auth_hybrid_test.go：JWT 带 cloudDisk claim / form data 带 cloudDisk / 都没有
  — 3 种场景
 - internal/service/file_test.go：UploadFileWithGroup 落盘后 file.CloudDisk == "test-instance"
 - internal/handler/file_test.go：端到端验证 — 上传请求带 cloudDisk → 落盘正确

 Step 9 — Lint + 构建

 cd sdk && ruff check . && ruff format --check . && mypy agentdisk tests
 go test ./... -count=1
 make lint

 Step 10 — 浏览器测试 t24（新）

 test/browser/tests/t24-cloud-disk-field.js（新建）：
 1. 用 admin 登录
 2. 通过 admin API 直接创建一个带 cloudDisk 字段的上传（fetch + FormData，手动塞 cloudDisk 字段）
 3. 列出文件，验证响应里包含 cloudDisk 字段
 4. cleanup

 注意：浏览器测试是 UI 集成测试，主要验证字段在端到端链路里没丢。SDK 真实测试在 sdk/tests/ 里。

 Step 11 — 提交、合并、推送

 # commit
 git add sdk/src/agentdisk/client.py sdk/src/agentdisk/api/file.py \
         sdk/src/agentdisk/models/file.py \
         sdk/tests/test_cloud_disk.py \
         internal/model/file.go internal/middleware/auth_hybrid.go \
         internal/middleware/auth_hybrid_test.go \
         internal/handler/file.go internal/handler/file_test.go \
         internal/handler/public_directory.go \
         internal/service/file.go internal/service/file_test.go \
         sql/schema.sql test/browser/tests/t24-cloud-disk-field.js

 git commit -m "feat(protocol): P5c.1 add cloudDisk diagnostic field to SDK and disk_file"

 # merge + push from main worktree
 git -C /Users/wangyun/Documents/work/gitlab/agent-disk merge --no-ff feature/okf-p5c1-protocol \
   -m "Merge PR-P5c.1: protocol cloudDisk field"
 git -C /Users/wangyun/Documents/work/gitlab/agent-disk push origin feature/okf-integration

 关键文件清单

 修改：
 - sdk/src/agentdisk/client.py（构造函数 + upload 方法，+10 行）
 - sdk/src/agentdisk/api/file.py（API 层 cloud_disk 参数，+8 行）
 - sdk/src/agentdisk/models/file.py（DiskFile 模型加字段，+1 行）
 - internal/model/file.go（DiskFile 加 CloudDisk 字段，+1 行）
 - internal/middleware/auth_hybrid.go（提取 cloudDisk，+10 行）
 - internal/handler/file.go（私有 UploadFile 接收，+3 行）
 - internal/handler/public_directory.go（公共 UploadFile 接收，+3 行）
 - internal/service/file.go（落盘，+3 行）
 - sql/schema.sql（schema 参考，+1 行）

 新建：
 - sdk/tests/test_cloud_disk.py（SDK 单测，~50 行）
 - internal/middleware/auth_hybrid_test.go（如不存在则新建；3 个场景，~80 行）
 - internal/service/file_test.go 扩展（cloudDisk 落盘测试，+30 行）
 - internal/handler/file_test.go 扩展（端到端，+30 行）
 - test/browser/tests/t24-cloud-disk-field.js（UI 集成，~80 行）

 DoD

 - cd sdk && ruff check . && ruff format --check . && mypy agentdisk tests 全通过
 - go test ./... -count=1 全通过
 - make lint 0 issue
 - cd test/browser && node runner.js t24 通过
 - cd test/browser && node runner.js t11 无回归（文件分享依赖 disk_file，验证字段改动未破坏）
 - 合并到 feature/okf-integration 并推送

 风险与备注

 - AutoMigrate 安全性：加列是向后兼容的，GORM 会自动处理。生产环境首次启动会自动加列，无需手工 DDL。
 - JWT claims 不动：本期 cloudDisk 只走 form data，不改 JWT 签发逻辑。若后续 P5c.2 灰度字段涉及
 JWT，可再扩展。
 - form data 仅限 multipart 上传：JSON body 的写接口（如 content write）本期不传
 cloudDisk，留待后续按需扩展。
 - 浏览器测试局限：浏览器测的是「字段穿过 form data 不丢」，不测 SDK 真实行为；SDK 行为由 sdk/tests/
  保证。
     - go test ./... -count=1 全通过                                 
     - make lint 0 issue
     - cd test/browser && node runner.js t24 通过
     - cd test/browser && node runner.js t11 无回归（文件分享依赖 disk_file，验证字段改动未破坏）
     - 合并到 feature/okf-integration 并推送
                                             
     风险与备注
                     
     - AutoMigrate 安全性：加列是向后兼容的，GORM 会自动处理。生产环境首次启动会自动加列，无需手工
     DDL。
     - JWT claims 不动：本期 cloudDisk 只走 form data，不改 JWT 签发逻辑。若后续 P5c.2 灰度字段涉及
     JWT，可再扩展。                                                                              
     - form data 仅限 multipart 上传：JSON body 的写接口（如 content write）本期不传
     cloudDisk，留待后续按需扩展。
     - 浏览器测试局限：浏览器测的是「字段穿过 form data 不丢」，不测 SDK 真实行为；SDK 行为由
     sdk/tests/ 保证。
     - 回滚策略：纯加字段，回滚只需 git revert；DB 列保留为空不影响业务。
                            
     验证方式                                               

     # 1. SDK 单测
     cd sdk && pytest tests/test_cloud_disk.py -v

     # 2. SDK lint + 类型
     cd sdk && ruff check . && ruff format --check . && mypy agentdisk tests
                                                                     
     # 3. Backend 测试                                                                   
     go test ./internal/middleware/... ./internal/handler/... ./internal/service/... -run CloudDisk
     -v                                                                            
                              
     # 4. Backend lint
     make lint 

     # 5. 浏览器回归                                                   
     bash scripts/dev.sh restart                       
     cd test/browser && node runner.js t24 && node runner.js t11                                  
                                                                                        
     ---               
     P5c.2 / P5c.3 / P5c.4 概要（本轮不实施）
                 
     P5c.2 字段级灰度开关
                                                                 
     - 目标：把现有 cfg.Okf.Enabled 升级成字段级 feature flag，支持热加载
     - 关键文件：config/config.go 加 FeatureFlags struct；internal/middleware/feature_flag.go
     新建；/v1/disk/admin/features 管理端点                                                      
     - 机制：fsnotify 监听 config.yaml，原子 swap；HybridAuth 之外加 FeatureGate 中间件
     - 依赖：无（可与 P5c.1 并行）           
                                                                                                     
     P5c.3 BFS 压测
                                                                                 
     - 目标：seed 10万节点 + ~50万边，验证 Subgraph / Reachable P95 < 500ms
     - 关键文件：scripts/seed_okf_large.py（新建，扩展 seed
     脚本）；internal/service/okf_bfs_bench_test.go（Go
     bench）；docs/perf/bfs-baseline.md（基线文档）
     - 关注点：Redis 缓存命中率（okf_graph_cache.go）；GORM 查询 N+1；邻接表预计算 vs 实时查
     - 依赖：无（独立压测）
                                                                
     P5c.4 协议级 e2e

     - 目标：覆盖 SDK → gateway → backend → 落盘 → 分享 → 预览 全链路
     - 关键文件：sdk/tests/e2e/test_full_flow.py（pytest，真实起
     backend）；test/browser/tests/t25-protocol-e2e.js（UI 验证）
     - 场景：SDK 写文件 → browser 看到文件 → SDK 创建分享 → browser 访问分享 → SDK 撤销分享 → browser
      验证已失效
     - 依赖：P5c.1（要测 cloudDisk 字段穿过链路）
