# OKF Integration 工作总结（2026-07-14）

## 本次会话完成

### 1. kb_client 配置对齐 writer（commit `fe92ac2`）

- 替换 `KB_BOT_PUBLIC_DIRECTORY_NAME`（字符串，空格值易碎）→ `AGENTDISK_PUBLIC_DIRECTORY_ID`（数字，与 writer 共用）
- 新增 `default_public_directory_id()` + `_resolved_pd()` lru_cache（id → displayName 经 SDK `list_public_directories()` 解析一次）
- 老 env var 保留为回退（list → 匹配 displayName），向后兼容
- 改动 8 文件 +252/-55：
  - `adk_kb_bot/adk_kb_bot/kb_client.py`
  - `adk_kb_bot/tests/{conftest,test_kb_client,test_tools}.py`
  - `adk_kb_bot/scenarios/ask_business.py`
  - `adk_kb_bot/evals/run_evals.py`
  - `adk_kb_bot/.env.example`
  - `adk_kb_bot/README.md`
- 验证：45/45 测试通过、ruff clean、mypy clean

### 2. 合并 main（merge `05bf466`）

- 拉入 PR #29：SDK `client.py` + `async_client.py` 新增 dynamic token/api_key setter
- ort 策略自动合并，零冲突

## 当前分支状态

- 分支：`feature/okf-integration`，**领先 origin 5 个 commit 未推送**
- 最新 5 个 commit：
  - `05bf466` Merge origin/main
  - `fe92ac2` kb_client 配置对齐
  - `b4eec4c` read_node_body + dotenv 修复
  - `5e3b613` eval harness 接 ADK rubric
  - `45e084d` adk_kb_bot 初始版本

## 未解决（下回继续）

### SDK resolver bug — cite_source_when_answered 用例失败根因

- `sdk/src/agentdisk/_resolver.py:140` 对 PD 子目录文件走私有 `/v1/disk/files` 路由
- `RequireNonAPIKey` middleware 对 API Key 返回 403
- 只有 PD 根文件（如 `index.md`）能读，子目录文件全失败
- eval 当前通过率：3/4（仅 `cite_source_when_answered` 失败）

**修复路径（任选其一）**：

1. **后端加路由**（推荐）：`GET /v1/disk/public-directories/:id/files?folderId=X` —— PD-scoped，API Key 可用，最小侵入
2. **SDK 改 resolver**：`_resolver.resolve_file` 对 PD 子目录改走 PD-scoped 列表 + 本地路径遍历（需要先有 #1 的后端路由）
3. **bot 端妥协**：收紧 INSTRUCTION，让 agent 用 frontmatter（title/description/tags）做引用，不强求读正文

### 其他待办

- **未推送**：本地 5 个 commit 没推 origin（用户未要求 push，等明确指示）
- **工作树噪声**（不影响合并）：
  - `examples/adk_writer_agent/scripts/phase2_scan.{json,md}` 是早先 writer 跑出来的产物
  - 一堆 untracked 的 `.md` 文档和 `dump.rdb`

## 关键环境状态（本地）

- 后端运行中，bundle 24（87 nodes / 85 edges，金融监管语料）已就绪
- PD 66 "OKF Bot Eval Corpus"
- API Key `adk_9218...`（kb-bot-eval scope）
- KB_BOT_MODEL=`deepseek/deepseek-chat`，DEEPSEEK_API_KEY 已配

## 关键文件路径速查

- kb_client 主体：`examples/adk_kb_bot/adk_kb_bot/kb_client.py`
- 工具集：`examples/adk_kb_bot/adk_kb_bot/tools.py`
- Agent 指令：`examples/adk_kb_bot/adk_kb_bot/agent.py`
- eval 入口：`examples/adk_kb_bot/evals/run_evals.py`
- eval 配置生成：`examples/adk_kb_bot/evals/build_eval_config.py`
- eval 用例集：`examples/adk_kb_bot/evals/kb_bot_eval_set.json`
- SDK resolver（bug 点）：`sdk/src/agentdisk/_resolver.py:130-154`
- writer 配置参考：`examples/adk_writer_agent/adk_writer_agent/agentdisk_client.py`
