adk_writer_agent：raw 资料 → OKF 格式化存储 + 图检索

 Context

 examples/adk_writer_agent/ 当前 6 个工具只能"用户在 prompt 中逐个给出文件规格 → 单次
 write_markdown"，无法批量初始化。本次为该 agent 增加 2 个本地读取工具，让它能扫描本地 raw
 目录、读取内容、由 LLM 推断 OKF frontmatter、按 raw 已有目录结构批量写入
 bundle，并在节点间建立丰富的 bundle-relative 链接，让 P3 已实现的图查询 API（neighbors / reachable
 / shortest / subgraph）能直接服务检索需求。

 仓库内已有 96 个金融监管法规文档作为首批初始化目标（examples/adk_writer_agent/raw/，8 一级目录 + 24
  二级目录）。这些文档间天然存在多版本、引用、同主题关联（如"非银行支付机构分类评级管理办法"5
 个版本、"反洗钱法" → "金融机构反洗钱监督管理办法"实施细节），是知识图谱的理想输入。

 核心目标：raw → OKF 格式化 + 节点间合法 bundle-relative 链接 → 自动物化为 disk_okf_edge → P3 图查询
  API 直接可用。

 与 LLM Wiki 实施计划的对齐

 严格遵守 OKF §严格写入约束（参考 LLM Wiki（OKF v0.1）扩展实施计划.md §严格度边界）：

 ┌──────────────────────────────────────────────┬───────────────────────────────────────────────┐
 │                     约束                     │               对本 plan 的影响                │
 ├──────────────────────────────────────────────┼───────────────────────────────────────────────┤
 │ 写入路径严格：frontmatter 必须可解析、type   │ 两阶段写入：阶段一批量写无链接节点 →          │
 │ 必填、保留文件名结构、bundle-relative        │ 阶段二加链接（目标已存在）                    │
 │ 链接目标必须存在                             │                                               │
 ├──────────────────────────────────────────────┼───────────────────────────────────────────────┤
 │                                              │ 大文件分块的索引页用                          │
 │ 仅 bundle 根 index.md 可含 okf_version       │ <basename>.toc.md（type=toc），不与根 index   │
 │                                              │ 冲突                                          │
 ├──────────────────────────────────────────────┼───────────────────────────────────────────────┤
 │ bundle 注册顺序：先写根 index.md → 再        │ INSTRUCTION 明确这个顺序，不能颠倒            │
 │ register_bundle                              │                                               │
 ├──────────────────────────────────────────────┼───────────────────────────────────────────────┤
 │ 读取路径宽容（OKF §9）                       │ 不影响本 plan，本 plan 只做写入               │
 ├──────────────────────────────────────────────┼───────────────────────────────────────────────┤
 │                                              │ 写 markdown 含 ./foo.md 链接 → 服务端         │
 │ Edge 表自动物化                              │ WriteMarkdown 自动调 materializeEdges 落      │
 │                                              │ disk_okf_edge，无需 agent 显式操作            │
 └──────────────────────────────────────────────┴───────────────────────────────────────────────┘

 P3 图查询 API（已实现）：
 - GET /v1/disk/okf/nodes/:id/neighbors?dir=out|in|both
 - POST /v1/disk/okf/nodes/:id/reachable {depth, types, limit}
 - POST /v1/disk/okf/paths/shortest {src, dst, maxDepth}
 - POST /v1/disk/okf/subgraph {types, maxNodes}

 只要节点写入时含合法链接，这些 API 自动可用。

 文件量评估（基于现有 raw/）

 ┌────────────┬──────────────────────────────────────┬───────────────────────────────────────────┐
 │    指标    │                 数值                 │                 设计影响                  │
 ├────────────┼──────────────────────────────────────┼───────────────────────────────────────────┤
 │ 文件总数   │ 96                                   │ 远低于 list 上限 500，单次可完成          │
 ├────────────┼──────────────────────────────────────┼───────────────────────────────────────────┤
 │ 目录层级   │ 3 层（一级 8 / 二级 24）             │ 保留原结构，扁平化会丢失分类              │
 ├────────────┼──────────────────────────────────────┼───────────────────────────────────────────┤
 │ 大小分布   │ 4KB–640KB，>200KB 共 3 个            │ 200KB 阈值覆盖 93/96，3 个 PCI-DSS 走分块 │
 ├────────────┼──────────────────────────────────────┼───────────────────────────────────────────┤
 │ 文件名     │ 全中文 + 〔〕/_/【】                 │ OKF relPath 已支持，无需转义              │
 ├────────────┼──────────────────────────────────────┼───────────────────────────────────────────┤
 │ 多版本同名 │ ~5 组（如分类评级管理办法 5 个版本） │ 图检索的核心场景：横向关联同主题多版本    │
 ├────────────┼──────────────────────────────────────┼───────────────────────────────────────────┤
 │ 引用关系   │ 法规间相互引用（A 法规引用 B 法规）  │ 图检索的第二场景：建立引用边              │
 ├────────────┼──────────────────────────────────────┼───────────────────────────────────────────┤
 │ 上下位关系 │ 法律 → 实施细则 / 部门规章           │ 图检索的第三场景：建立上下位边            │
 └────────────┴──────────────────────────────────────┴───────────────────────────────────────────┘

 已确认决策

 ┌────────────────────────┬──────────────────────────────────────────────────────────────────────┐
 │          维度          │                                 决策                                 │
 ├────────────────────────┼──────────────────────────────────────────────────────────────────────┤
 │ Raw 根目录             │ AGENTDISK_RAW_ROOT，默认                                             │
 │                        │ examples/adk_writer_agent/raw/（包内），支持外部路径覆盖             │
 ├────────────────────────┼──────────────────────────────────────────────────────────────────────┤
 │ 文件格式               │ .md / .markdown / .txt / .rst / .org                                 │
 ├────────────────────────┼──────────────────────────────────────────────────────────────────────┤
 │ 扫描范围               │ 递归，跳过 . 开头、node_modules/、.git/、__pycache__/、dist/、build/ │
 ├────────────────────────┼──────────────────────────────────────────────────────────────────────┤
 │ 子目录组织             │ 保留 raw 原目录结构                                                  │
 ├────────────────────────┼──────────────────────────────────────────────────────────────────────┤
 │ frontmatter            │ LLM 推断；type 用法律领域类型；source 字段记录抓取来源               │
 ├────────────────────────┼──────────────────────────────────────────────────────────────────────┤
 │                        │ read_raw_file(chunk=True) 按 ## H2 切分，单 chunk ≤                  │
 │ 大文件                 │ 200KB；超出按段落细分；写入时拆为 <basename>.part1.md /              │
 │                        │ <basename>.part2.md + <basename>.toc.md 索引页                       │
 ├────────────────────────┼──────────────────────────────────────────────────────────────────────┤
 │ 链接建立（图检索核心） │ 三类链接：(a) 同主题多版本横向链接 (b) 法规引用关系 (c)              │
 │                        │ 上下位实施关系；阶段二批量补链                                       │
 ├────────────────────────┼──────────────────────────────────────────────────────────────────────┤
 │ 原件归档               │ 不做（用户已确认核心需求是格式化 + 图检索，不需要保留 raw 原件到     │
 │                        │ PD）                                                                 │
 ├────────────────────────┼──────────────────────────────────────────────────────────────────────┤
 │ Eval                   │ 本次不增加                                                           │
 └────────────────────────┴──────────────────────────────────────────────────────────────────────┘

 工具签名（追加到 adk_writer_agent/tools.py）

 list_raw_files(dir: str = "") -> dict[str, Any]

 def list_raw_files(dir: str = "") -> dict[str, Any]:
     """List raw text files under AGENTDISK_RAW_ROOT/<dir> recursively.

     Scan phase of "initialize KB from local files". Returns per-file hints
     so the LLM can infer type/title/tags and identify cross-file relations
     (multi-version, citation, hierarchy) without calling read_raw_file on
     every file.

     Args:
         dir: Sub-directory relative to AGENTDISK_RAW_ROOT. Empty = RAW_ROOT.
             Absolute paths and any path containing ".." are rejected.

     Returns:
         {"ok": True, "data": {
             "root": "<abs RAW_ROOT>",
             "dir": "<requested dir>",
             "files": [{"rel_path": "机构监管/反洗钱管理/X.md",
                        "size": 12340,
                        "ext": ".md",
                        "size_bucket": "small|medium|large",  # <8K / 8-200K / >200K
                        "hint": {"first_line": "中华人民共和国反洗钱法",
                                 "h2_count": 12,
                                 "source": "waizi|webtax|gov|..."}}],
             "skipped": [".hidden.md", "node_modules/x.md", "binary.png"],
             "truncated": false
         }} or {"ok": False, "error": "..."}
     """

 实现要点：
 - AGENTDISK_RAW_ROOT 未设置时默认指向包内 examples/adk_writer_agent/raw/（开发态零配置）
 - 共享 _resolve_safe(raw_root, dir)：Path.resolve() + is_relative_to(raw_root.resolve())，绝对路径
 / .. 一律拒绝
 - 扩展名白名单：.md / .markdown / .txt / .rst / .org
 - 单文件仅读首 4KB 提取 hint；source
 通过文件名后缀（_waizi/_webtax/_esnai/_mpaypass/_lawlib/_gov）启发式识别
 - 文件总数 > 500 时 truncated=True 且只返前 500（防 LLM 上下文爆炸）

 read_raw_file(path: str, chunk: bool = False) -> dict[str, Any]

 def read_raw_file(path: str, chunk: bool = False) -> dict[str, Any]:
     """Read a raw text file under AGENTDISK_RAW_ROOT.

     Args:
         path: Relative path. Empty / absolute / ".."-containing paths rejected.
         chunk: When True and file > 200 KB, split by markdown H2 headings
             into multiple chunks (each ≤ 200 KB; H2 chunks still > 200 KB
             are further split by paragraph). When False, files > 200 KB are
             truncated at 200 KB with truncated=True.

     Returns:
         Non-chunked: {"ok": True, "data": {
             "path": "...", "size": 1234, "truncated": false,
             "frontmatter": null | {...}, "content": "原始正文"}}
         Chunked: {"ok": True, "data": {
             "path": "...", "size": 640000, "chunked": true,
             "chunks": [{"index": 1, "heading": "第二章 定义",
                         "approx_size_kb": 180, "content": "..."},
                        ...]}}
         Error: {"ok": False, "error": "..."}
     """

 实现要点：
 - 越界保护同 list_raw_files
 - 非 chunk 模式：> 200 KB 截断到 200 KB
 - chunk 模式：按 ^##  切分；单 H2 仍 > 200 KB 按空行段落二次切；返回 chunks: [{index, heading,
 approx_size_kb, content}]
 - 已有 YAML frontmatter 用 yaml.safe_load 解析到 frontmatter，失败 → None，content 保持原文

 法律领域 type 推荐表（INSTRUCTION 中明确）

 ┌────────────┬───────────────────────────────────┬────────────────────────────────────────┐
 │    type    │               含义                │                典型示例                │
 ├────────────┼───────────────────────────────────┼────────────────────────────────────────┤
 │ law        │ 法律（人大通过）                  │ 消费者权益保护法、反洗钱法、数据安全法 │
 ├────────────┼───────────────────────────────────┼────────────────────────────────────────┤
 │ regulation │ 行政法规 / 部门规章               │ 银发〔2016〕106号、央行令、国务院令    │
 ├────────────┼───────────────────────────────────┼────────────────────────────────────────┤
 │ standard   │ 国家 / 行业标准                   │ GB/T 39412、JRT 0171、PCI-DSS          │
 ├────────────┼───────────────────────────────────┼────────────────────────────────────────┤
 │ notice     │ 规范性文件 / 通知                 │ 银办发〔2017〕21号、央行公告           │
 ├────────────┼───────────────────────────────────┼────────────────────────────────────────┤
 │ spec       │ 业务规范 / 实施细则               │ 条码支付业务规范、检测规范             │
 ├────────────┼───────────────────────────────────┼────────────────────────────────────────┤
 │ policy     │ 内部制度                          │ JL-WI-RD-010 公司内部办法              │
 ├────────────┼───────────────────────────────────┼────────────────────────────────────────┤
 │ plan       │ 行动方案 / 规划                   │ 推动数字金融高质量发展行动方案         │
 ├────────────┼───────────────────────────────────┼────────────────────────────────────────┤
 │ toc        │ 大文件分块索引页（强制）          │ <basename>.toc.md                      │
 ├────────────┼───────────────────────────────────┼────────────────────────────────────────┤
 │ index      │ bundle 根索引（强制 okf_version） │ 仅 bundle 根 index.md                  │
 └────────────┴───────────────────────────────────┴────────────────────────────────────────┘

 兜底：无法判断 → notice（金融监管领域默认）。

 INSTRUCTION 增量段落（中文草稿，追加到 agent.py INSTRUCTION 末尾）

 从本地 raw 文件初始化知识库（仅当用户明确要求"从 raw 初始化"/"导入本地
 文件"/"扫描目录"等时才走此流程）。本流程的最终目标是把 raw 文件格式化
 为 OKF 节点写入 bundle，并在节点间建立合法的 bundle-relative 链接，让
 P3 图查询 API（neighbors / reachable / shortest）能直接服务检索。

 1. 调用 list_raw_files(dir="") 扫描 RAW_ROOT。truncated=True 时提示
    用户缩小范围；skipped 由工具自动处理。
 2. 对每个文件推断 frontmatter（type / title / tags / source），启发式：
    - 首行 `# X` 或首行非空文本 → title = X（截断到 60 字）
    - 文件名含「主席令」「法」→ type = law
    - 文件名含「条例」「办法」「规定」「实施细则」→ type = regulation
    - 文件名含「GB」「GBT」「JRT」「PCI」→ type = standard
    - 文件名含「通知」「公告」「银发」「银办发」→ type = notice
    - 文件名含「规范」「业务规范」→ type = spec
    - 文件名含「行动方案」「规划」→ type = plan
    - 文件名含「JL-」「WI-」「RD-」等内部编码 → type = policy
    - 兜底 → notice
    - 文件名后缀 `_waizi`/`_webtax`/`_esnai`/`_mpaypass`/`_lawlib`/`_gov`
      → source = 对应站点名
    - tags 从正文高频名词取 0-3 个
 3. 大文件处理：文件 > 200 KB 时调用 read_raw_file(path, chunk=True)
    获取分块；为每个 chunk 写一个独立 OKF 节点（rel_path 形如
    `<原目录>/<basename>.part1.md`），并写一个分块索引页
    `<原目录>/<basename>.toc.md`（type=toc，正文列出各 part 链接）。
    **禁止用 index.md 作为子目录索引**（OKF 规范：仅 bundle 根
    index.md 可含 okf_version）。
 4. **bundle 注册顺序**（OKF 严格，不可颠倒）：
    a. 先 write_markdown(rel_path="index.md", content="---\nokf_version:
       '0.1'\ntitle: 金融监管法规知识库\n---\n# 金融监管法规知识库")。
    b. 再 register_bundle(public_directory_id)。
 5. create_folder 建立出现的所有目录层级（按 raw 目录结构透传）：
    如 `机构监管/`、`机构监管/反洗钱管理/`、`业务管理/银行卡收单/`。
    已存在视为幂等成功。
 6. **阶段一（无链接批量写入）**：对每个 raw 文件（含大文件分块）调用
    write_markdown(rel_path="<原目录>/<name>.md", content=<新 frontmatter
    + 正文>)。正文严格不含任何 bundle-relative 链接（`./xxx.md` 或
    `/foo.md`），否则后端校验目标存在性会 HTTP 400。原文中的链接转义
    为反引号代码块（`` `[x](./foo.md)` ``）或保留为纯文本。HTTP(S)
    链接保留不动。
 7. **阶段二（链接建立，图检索核心）**：所有节点写入完成后，调用一次
    list_nodes(bundle_id) 拿到全部节点 rel_path 与 frontmatter。然后
    对需要建链的节点二次 write_markdown 加链接（此时目标已存在）：

    a. **同主题多版本横向链接**：
       - 识别：文件名前缀完全相同 + 后缀不同（`_征求意见稿` /
         `_正式稿` / `_修订` / `_waizi` 等）
       - 按发布时间（文件名中含的年份/日期）排序
       - 在每个版本节点正文末尾追加"## 相关版本"段，按时间倒序链接
         同主题其他版本（用 bundle-relative 路径
         `./<原文件名>.md`）

    b. **法规引用关系链接**：
       - 阶段一读取时 LLM 应记录正文中明确提到的其他法规名（如
         "依据《反洗钱法》第X条"）
       - 阶段二在引用方节点正文相应位置加 `《反洗钱法》→
         [反洗钱法](../机构监管/中华人民共和国反洗钱法_主席令第三十八号_2024修订.md)`
       - 仅建立明确引用关系，不强行猜测

    c. **上下位实施关系链接**：
       - 法律（type=law）→ 实施该法律的部门规章 / 实施细则
         （type=regulation 或 spec）
       - 在下位法节点 frontmatter 后正文首行加"上位法：[X](./X.md)"
       - 在上位法节点正文末尾加"## 实施细则"列出下位法链接

    d. **大文件 toc 索引页链接**：阶段一已写各 part，阶段二把 toc
       索引页的 part 链接填上。

    链接格式：bundle-relative，同目录用 `./foo.md`，跨目录用
    `../<dir>/foo.md`。所有目标必须已通过 list_nodes 验证存在。

 注意：
 - 不要在 raw 流程结束后追加额外的 list_nodes / aggregate_types /
   refresh_index，除非用户明确要求核对。阶段二的 list_nodes 是允许的。
 - 单次 raw 导入建议不超过 200 个文件；超过则提示用户分批。
 - 中文文件名 / 特殊字符（〔〕、【】、_）路径在工具层已正常处理，
   不要手动转义或重命名。
 - **OKF 写入严格约束（必须遵守）**：
   - bundle-relative 链接目标必须存在于 bundle 内，否则 HTTP 400。
     阶段一写入时正文严禁出现 `./xxx.md` 或 `/foo.md`；阶段二加链接
     前必须用 list_nodes 确认目标存在。
   - 仅 bundle 根 index.md 可含 okf_version；子目录索引页用 toc
     类型 + `<basename>.toc.md` 文件名。
   - type 字段必填，值非空；其余字段（title/tags/source/timestamp）
     可选。
 - 链接建立是图检索的命脉：写 markdown 含 `./foo.md` 链接 → 服务端
   WriteMarkdown 自动物化到 disk_okf_edge 表 → P3 neighbors/reachable/
   shortest API 直接可用。**链接越丰富，图检索越好用**。

 复用的现有能力

 - register_bundle / create_folder / write_markdown / list_nodes（已有）— 直接组合
 - AgentDiskClient（agentdisk_client.py）— 不动
 - _err_payload / _ok_payload / _default_pd_id（tools.py 已有）— 新工具复用
 - yaml（已在 pyproject.toml 依赖）— frontmatter 解析无新依赖
 - 服务端 materializeEdges（internal/service/okf_graph.go，P3a 已实现）— agent 写链接后自动落 edge
 表，agent 不需要显式调用任何图相关工具
 - P3 图查询 API（/v1/disk/okf/nodes/:id/neighbors 等，已实现）— 验证阶段直接调用

 关键文件改动清单

 文件: examples/adk_writer_agent/adk_writer_agent/tools.py
 改动类型: 改
 摘要: 追加 list_raw_files / read_raw_file 2 函数 + 共享 _resolve_safe() / _raw_root()
   helper；末尾注册 2 个 FunctionTool；扩展 __all__
 ────────────────────────────────────────
 文件: examples/adk_writer_agent/adk_writer_agent/agent.py
 改动类型: 改
 摘要: 1) import 2 个新 tool；2) INSTRUCTION 末尾追加"从本地 raw 文件初始化知识库"段落（含两阶段写入

   + 三类链接策略）；3) tools=[...] 加入 2 个新 tool；4) 顶部模块 docstring "6 tools" 改 "8 tools"
 ────────────────────────────────────────
 文件: examples/adk_writer_agent/.env.example
 改动类型: 改
 摘要: 在 AGENTDISK_API_KEY 后追加 AGENTDISK_RAW_ROOT=（注释说明默认指向包内 raw/）
 ────────────────────────────────────────
 文件: examples/adk_writer_agent/README.md
 改动类型: 改
 摘要: 新增"从本地 raw 文件建立知识库"小节：默认 raw 目录介绍 + 示例 prompt + 跳过规则 + 大文件分块
 +
   两阶段写入说明 + 图检索验证
 ────────────────────────────────────────
 文件: examples/adk_writer_agent/pyproject.toml
 改动类型: 改
 摘要: extend-exclude / exclude 不加入 tests；packages.find include 维持 adk_writer_agent*（tests
   不打包但参与 lint/type）
 ────────────────────────────────────────
 文件: examples/adk_writer_agent/tests/__init__.py
 改动类型: 新建
 摘要: 空
 ────────────────────────────────────────
 文件: examples/adk_writer_agent/tests/conftest.py
 改动类型: 新建
 摘要: raw_root fixture：tmp_path 构造 note.md / sub/concept.md / .hidden.md / node_modules/x.md /
   binary.png / big.md（250KB）/ with_front.md / pci_dss.md（250KB，多 H2，用于测 chunk）
 ────────────────────────────────────────
 文件: examples/adk_writer_agent/tests/test_raw_path_safety.py
 改动类型: 新建
 摘要: 4 用例：list_raw_files(dir="../../../etc") / 绝对路径 / read_raw_file(path="../secret") /
   RAW_ROOT 未配置
 ────────────────────────────────────────
 文件: examples/adk_writer_agent/tests/test_list_raw_files.py
 改动类型: 新建
 摘要: 7 用例：正常扫描 / .hidden.md 进 skipped / node_modules/x.md 进 skipped / binary.png 进
   skipped（扩展名）/ hint.first_line / hint.h2_count / size_bucket 分类 / >500 时 truncated
 ────────────────────────────────────────
 文件: examples/adk_writer_agent/tests/test_read_raw_file.py
 改动类型: 新建
 摘要: 5 用例：普通文件完整 content / with_front.md frontmatter 解析 / big.md 非 chunk 截断 /
   pci_dss.md chunk=True 返回多块 / chunk 内单块 ≤200KB

 不在本次范围

 - 不增加 eval（保留 evals/ 现状；未来如需 raw 初始化 eval，可在 okf_writer_eval_set.json 新增
 case）
 - 不引入 watch / 增量同步（一次性导入即可）
 - 不做并发 write_markdown（保持顺序、错误可追溯）
 - 不解析非文本格式（PDF / Word / HTML 等）
 - 不自动去除抓取残留（waizi/webtax 等站点的导航 HTML）
 - 不做原件归档（用户已确认核心需求是 OKF 格式化 + 图检索）
 - 不引入显式"建边"工具（OKF 服务端 WriteMarkdown 已自动 materializeEdges）

 验证步骤

 1. 静态检查

 cd examples/adk_writer_agent
 ruff check .          # 含 tests/，零 warning
 mypy adk_writer_agent tests

 2. 单元测试

 cd examples/adk_writer_agent
 pytest tests/ -v
 16 个用例必须 PASS。

 3. 端到端冒烟（需后端运行 + 仓库内 96 个 raw 文件）

 bash scripts/dev.sh start
 cd examples/adk_writer_agent
 adk run . --input "请扫描 RAW_ROOT 下所有 96 个法规文档，按 raw 目录结构初始化 OKF
 bundle（public_directory_id=$AGENTDISK_PUBLIC_DIRECTORY_ID），并在节点间建立多版本、引用、上下位三
 类链接以支持图检索。"

 验证清单（写入正确性）：
 - 后端 GET /v1/disk/okf/bundles/<id>/nodes 应看到 ~100 节点（96 + index.md + 大文件 toc/part 节点）
 - bundle 根 index.md 含 okf_version: "0.1"，且仅有此一个 index.md
 - 节点 relPath 保留中文路径
 - 每个节点 frontmatter 含 type（law/regulation/standard/...）
 - 阶段一写入无 HTTP 400（所有节点正文不含 bundle-relative 链接）
 - 阶段二加链接后 GET /v1/disk/okf/bundles/<id>/broken-links 为空或仅含合法外站链接
 - PCI-DSS 系列分块为 part1/part2 + toc 索引页

 验证清单（图检索 — 核心目标）：
 # 取任一节点 ID 作为起点
 NODE_ID=$(curl -s "http://localhost:8080/v1/disk/okf/bundles/$BUNDLE_ID/nodes?type=regulation" \
   -H "X-API-Key: $AGENTDISK_API_KEY" | jq '.data.nodes[0].nodeId')

 # 1 跳邻居：应返回同主题多版本 + 引用关系 + 上下位节点
 curl "http://localhost:8080/v1/disk/okf/nodes/$NODE_ID/neighbors?dir=both" \
   -H "X-API-Key: $AGENTDISK_API_KEY"

 # 2 跳可达：以反洗钱法为起点，应可达所有反洗钱相关节点
 curl -X POST "http://localhost:8080/v1/disk/okf/nodes/$NODE_ID/reachable" \
   -H "X-API-Key: $AGENTDISK_API_KEY" \
   -d '{"depth": 2, "limit": 100}'

 # 最短路径：反洗钱法 → 数据安全法，应返回 ≤3 跳路径
 curl -X POST "http://localhost:8080/v1/disk/okf/paths/shortest" \
   -H "X-API-Key: $AGENTDISK_API_KEY" \
   -d '{"src": <反洗钱法 nodeId>, "dst": <数据安全法 nodeId>, "maxDepth": 3}'

 # 子图：所有 type=regulation 节点构成的子图
 curl -X POST "http://localhost:8080/v1/disk/okf/subgraph" \
   -H "X-API-Key: $AGENTDISK_API_KEY" \
   -d '{"types": ["regulation"], "maxNodes": 1000}'

 每个 API 都应返回非空结果，证明链接已正确物化为 edge。

 4. Eval 回归（确保未破坏现有 8 个用例）

 cd examples/adk_writer_agent
 make okf-eval
 baseline_results.md 中 8 个用例维持原有通过率（不引入回归）。
╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌

