 OKF Writer Agent 评估套件（ADK eval 模式）                

 Context

 examples/adk_writer_agent/ 是 OKF v0.1 的 LLM 维护 agent demo，但目前完全没有评估覆盖：
 - 改 prompt（adk_writer_agent/agent.py:23-53 的 INSTRUCTION）没有回归保护
 - 加新工具没人测是不是真的被正确调用
 - LLM 输出不确定性让普通 unit test 没法捕获"agent 决策"层面的退化

 用户要求用 ADK 内置 eval 框架（adk eval）建立 OKF 知识库的评估集，跑出
 baseline，并找出可改进的地方。这比单元测试高一层 —— 它评估的是 agent
 的工具轨迹和最终回复质量，而不是单个函数的输入输出。

 Goal

 1. 5–8 个 eval case 覆盖 OKF writer 主要决策点（register / write / refresh / 容错）
 2. 可独立运行：make okf-eval 一键跑，输出分数 + 改进建议
 3. Baseline 报告：跑一次 DeepSeek V4，记录每条 case 的 trajectory 分数、response 分数，标出 <
 阈值的 case 作为"改进点"
 4. 每个 case 独立 OKF 状态：eval 之间互不污染（每 case 前清 bundle、注册新 pd）

 Scope

 做：
 - 在 examples/adk_writer_agent/evals/ 新建评估套件（与 demo 同包，独立目录）
 - 写 eval set JSON（ADK 原生格式）
 - 写 Python harness：每 case 前清状态、注入 deterministic public directory
 - 写 baseline 报告 + 改进点清单
 - Makefile 加 okf-eval target

 不做：
 - 不动后端 OKF 代码
 - 不动 agent.py 的 prompt（评估完后再决定改不改）
 - 不做 CI 集成（eval 需要 LLM key + 真后端，先手动跑）

 文件清单

 新建：
 - examples/adk_writer_agent/evals/__init__.py（空）
 - examples/adk_writer_agent/evals/okf_writer_eval_set.json —— ADK 原生 eval set，5-8 个 case
 - examples/adk_writer_agent/evals/conversation_scenarios.json —— 每个 case 的 user 对话脚本
 - examples/adk_writer_agent/evals/session_inputs.json —— 每个 case 的初始 session
 状态（public_directory_id 等）
 - examples/adk_writer_agent/evals/conftest.py —— pytest hooks，每 case 前后清 OKF 状态
 - examples/adk_writer_agent/evals/run_evals.py —— 调用 adk eval 的 wrapper，收集结果生成 markdown
 报告
 - examples/adk_writer_agent/evals/baseline_results.md —— 首次跑出的分数 + 改进点
 - examples/adk_writer_agent/evals/README.md —— 怎么跑、怎么看结果、怎么加新 case

 修改：
 - Makefile（根目录）—— 加 okf-eval 和 okf-eval-install target
 - examples/adk_writer_agent/pyproject.toml —— optional-dependencies 加 eval = [pytest, ...]
 - examples/adk_writer_agent/.env.example —— 注释里指向 evals/README.md

 5–8 个 Eval Cases 设计

 每个 case 是一组 (initial_state, user_message, expected_tool_trajectory,
 expected_response_keywords)。

 ┌─────┬────────────────────────────┬─────────────────┬─────────────────────────┬───────────────┐
 │  #  │          Case 名           │    触发输入     │  期望工具轨迹（按序）   │  改进点假设   │
 ├─────┼────────────────────────────┼─────────────────┼─────────────────────────┼───────────────┤
 │     │                            │ "请注册 bundle  │ register_bundle →       │ agent 是否能  │
 │ 1   │ register_when_missing      │ （pd_id=X）"    │ list_nodes              │ 正确判断      │
 │     │                            │                 │                         │ bundle 未注册 │
 ├─────┼────────────────────────────┼─────────────────┼─────────────────────────┼───────────────┤
 │     │                            │ bundle          │ list_nodes（无          │ agent 是否会  │
 │ 2   │ skip_register_when_exists  │ 已存在时同样的  │ register）              │ 重复注册      │
 │     │                            │ prompt          │                         │               │
 ├─────┼────────────────────────────┼─────────────────┼─────────────────────────┼───────────────┤
 │     │                            │ "写一个         │                         │               │
 │ 3   │ write_concept_with_frontma │ type=concept 的 │ write_markdown（含      │ frontmatter   │
 │     │ tter                       │  markdown 关于  │ frontmatter）           │ 完整性        │
 │     │                            │ X"              │                         │               │
 ├─────┼────────────────────────────┼─────────────────┼─────────────────────────┼───────────────┤
 │     │ write_invalid_frontmatter_ │ "写一个         │ write_markdown → 失败 → │               │
 │ 4   │ recovery                   │ markdown，type  │  重试                   │ 容错路径      │
 │     │                            │ 字段空着"       │                         │               │
 ├─────┼────────────────────────────┼─────────────────┼─────────────────────────┼───────────────┤
 │ 5   │ refresh_after_writes       │ "写完 3 个节点  │ 3× write_markdown →     │ 是否记得      │
 │     │                            │ 后刷新索引"     │ refresh_index           │ refresh       │
 ├─────┼────────────────────────────┼─────────────────┼─────────────────────────┼───────────────┤
 │ 6   │ multi_folder_organization  │ "写 concepts/X  │ create_folder × 2 →     │ 子目录组织是  │
 │     │                            │ 和 playbooks/Y" │ write_markdown × 2      │ 否合理        │
 ├─────┼────────────────────────────┼─────────────────┼─────────────────────────┼───────────────┤
 │     │                            │ "写一个引用     │ write_markdown（链接格  │ bundle-relati │
 │ 7   │ internal_link_format       │ ./gemma.md 的   │ 式正确）                │ ve 链接       │
 │     │                            │ markdown"       │                         │               │
 ├─────┼────────────────────────────┼─────────────────┼─────────────────────────┼───────────────┤
 │ 8   │ noop_clarification         │ "你好"（无明确  │ 无工具调用，回复询问    │ 是否会乱调工  │
 │     │                            │ 指令）          │                         │ 具            │
 └─────┴────────────────────────────┴─────────────────┴─────────────────────────┴───────────────┘

 每个 case 的 expected trajectory 用 ADK 的 genai_types.FunctionCall 格式描述，response
 用关键词列表（不强求精确匹配，含关键词即算通过）。

 实施步骤

 1. 准备 eval 目录结构

 mkdir -p examples/adk_writer_agent/evals
 cd examples/adk_writer_agent && pip install -e ".[eval]"

 2. 写 conversation_scenarios.json

 ADK 格式（参考 google.adk.evaluation.conversation_scenarios.ConversationScenario）：

 {
   "scenarios": [
     {
       "name": "register_when_missing",
       "conversation": [
         {"author": "user", "content": {"parts": [{"text": "请注册
 bundle（public_directory_id=1）"}]}}
       ]
     },
     ...
   ]
 }

 3. 写 expected tool trajectory

 每个 case 在 okf_writer_eval_set.json 里描述期望：

 {
   "eval_set_id": "okf_writer_baseline",
   "cases": [
     {
       "eval_id": "register_when_missing",
       "conversation_scenario_id": "register_when_missing",
       "initial_state": {"session_input": {...}},
       "expected_intermediate_data": {
         "tool_uses": [
           {"name": "register_bundle", "args": {"public_directory_id": "1"}},
           {"name": "list_nodes"}
         ]
       },
       "rubrics": [
         {"criteria": "回复提到 bundle_id 或注册成功", "score": 1.0}
       ]
     }
   ]
 }

 4. conftest.py 处理 OKF 状态隔离

 每个 eval case 跑前：
 - 调 /v1/disk/admin/okf/bundles/:id 删除现有 bundle（如有）
 - 重新 register 一个干净的 bundle
 - 把新的 bundle_id 注入 session state

 跑后：
 - 清理 bundle
 - 删除测试期间写入的 markdown 文件

 通过 ADK 的 reset_data hook 触发（参考 cli_eval.py:107 的 try_get_reset_func）。在
 adk_writer_agent/agent.py 加：

 def reset_data():
     """ADK eval 在每 case 前调用。"""
     from scenarios.eval_setup import clean_okf_state
     clean_okf_state()

 5. run_evals.py wrapper

 """Run ADK eval and emit a markdown report."""
 # 1. 调 `adk eval examples/adk_writer_agent evals/okf_writer_eval_set.json`
 # 2. 解析输出 JSON，每个 case 取 tool_trajectory_avg_score 和 response_match_score
 # 3. 生成 baseline_results.md：
 #    - 表格：case | trajectory | response | 通过？
 #    - 列出 < 0.8 分的 case 为"改进点"
 #    - 抓取 agent 实际 trajectory vs expected 的 diff，标出错误步骤

 6. 跑一次 baseline

 bash scripts/dev.sh start                 # 后端 + gateway + web
 cd examples/adk_writer_agent
 export WRITER_AGENT_MODEL=deepseek-v4
 export DEEPSEEK_API_KEY=...               # 用户提供
 make okf-eval                             # → 生成 baseline_results.md

 7. 写 baseline_results.md

 跑完后人工 + LLM 辅助填写：
 - 8 个 case 各自分数
 - 失败 case 的根因分析（prompt 不清？工具签名模糊？LLM 能力不足？）
 - 给出 3-5 个可操作的改进建议（如：在 INSTRUCTION 里加"先检查 bundle 是否存在再 register"、把
 register_bundle 的 description 写得更明确等）

 8. Makefile target

 okf-eval-install:
        cd examples/adk_writer_agent && pip install -e ".[eval]"

 okf-eval: okf-eval-install
        cd examples/adk_writer_agent && python evals/run_evals.py

 Verification

 # 1. 评估套件能跑通（端到端）
 make okf-eval

 # 2. baseline_results.md 自动生成
 cat examples/adk_writer_agent/evals/baseline_results.md

 # 3. 单独跑某个 case 验证
 cd examples/adk_writer_agent
 adk eval . evals/okf_writer_eval_set.json:register_when_missing

 # 4. 改 INSTRUCTION 后回归
 # (改 agent.py:23-53)
 make okf-eval    # 比较 baseline_results.md 变化

 DoD

 - make okf-eval 在配了 DEEPSEEK_API_KEY 的本地环境能完整跑通
 - 5–8 个 case 全部跑出分数（不是 0，说明 eval 框架正确接入）
 - baseline_results.md 自动生成，含：
   - 每个 case 的 trajectory 分数 + response 分数
   - 失败 case 的 diff（expected tool calls vs actual）
   - 至少 3 条可操作的改进建议
 - reset_data hook 正确触发：连续跑两次 make okf-eval 第二次结果与第一次接近（说明状态隔离生效）
 - evals/README.md 让另一个工程师能 30 分钟内加一个新 case
 - examples/adk_writer_agent/pyproject.toml 的 [eval] extra 不影响现有 pip install -e .
 - 不破坏现有 make lint / make sdk-check

 风险

 - LLM 非确定性导致 baseline 漂移：DeepSeek V4 同一 case 跑 3 次可能 3 个分。Mitigation：每个 case
 跑 3 次，取中位数写进 baseline；改进建议基于"是否稳定 < 0.8"而非单次跑分。
 - adk eval 输出 schema 没文档化：从 cli_eval.py 源码看输出是
 EvalCaseResult，需要实测确认字段名。如果输出 schema 变了，run_evals.py 解析会坏。Mitigation：用
 --log_level debug 抓原始 JSON，写 defensive parser（result.get('xxx') or result.get('yyy')）。
 - OKF 状态隔离失效：如果 reset_data 没正确清干净，第二个 case 看到第一个 case 的状态，所有 case
 都会假阳性通过。Mitigation：第一个 case 故意写入一个独特 marker（如 timestamp），第二个 case 前验证
  marker 已不存在。
 - 真实 LLM 调用慢 + 贵：8 case × 3 次 × DeepSeek V4 调用 ≈ 几十次 LLM 请求。Mitigation：开发循环用
 WRITER_AGENT_MODEL=gemini-2.5-flash（便宜快），baseline 正式版跑 DeepSeek V4。
 - ADK eval 框架本身可能 buggy： google-adk 2.0 还是新东西（代码里 import 都是 2026
 年）。Mitigation：先用最简 case（case 1 register_when_missing）端到端跑通，确认框架能用，再扩到 8
 个。
 - DeepSeek V4 model id 不对：实际 id 可能是 deepseek-chat 或 deepseek/deepseek-v4
 等。Mitigation：写一个 scripts/probe_llm.py 先验证 model id，再写进 baseline。
╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌