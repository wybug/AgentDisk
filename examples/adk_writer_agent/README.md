# AgentDisk Writer Agent (Google ADK 2.0 Demo)

A Google ADK maintenance agent that drives the AgentDisk **OKF v0.1** knowledge-base
API. The agent takes free-form domain knowledge from the user, structures it as
markdown with YAML frontmatter, and writes it into a bundle via the OKF REST API.

This demo is **purely illustrative** — it does NOT touch the AgentDisk Go backend,
is NOT part of the production deployment, and is excluded from the main Makefile
lint / test targets.

## Why this exists

Worktree B (Go backend) is implementing the OKF v0.1 endpoints in parallel. This
demo proves the OKF API is **open at the protocol level**: any HTTP client —
including a Google ADK agent — can read/write the knowledge base without coupling
to the AgentDisk Python SDK.

## Prerequisites

1. AgentDisk backend running (Worktree B): `bash scripts/dev.sh start`
2. A **public directory** created in the AgentDisk web UI — you can identify
   it by either numeric ID or path (the path is what the UI shows).
3. An **API key** with `okf-write` scope (created in the admin console)

## Install

```bash
cd examples/adk_writer_agent
pip install -e ".[dev]"
```

This installs Google ADK 2.0, httpx, pyyaml, python-dotenv, plus dev deps
(ruff, mypy).

## Configure

```bash
cp .env.example .env
# then edit .env and fill in real values
```

Required env vars:

| Var | Meaning |
|---|---|
| `AGENTDISK_BASE_URL` | Backend base URL, e.g. `http://localhost:8080` |
| `AGENTDISK_API_KEY` | API key with `okf-write` scope |
| `AGENTDISK_PUBLIC_DIRECTORY_ID` | Numeric ID of the public directory backing the bundle (preferred) |
| `AGENTDISK_PUBLIC_DIRECTORY_PATH` | **Alternative to ID** — path of the PD (e.g. `/public/test`), resolved at first use via `GET /v1/disk/public-directories`. Handy when you only have the UI path. |
| `WRITER_AGENT_MODEL` | LLM model id (see below) |
| `DEEPSEEK_API_KEY` / `GEMINI_API_KEY` / `ANTHROPIC_API_KEY` | Model provider key (pick one matching `WRITER_AGENT_MODEL`) |

> **ID vs PATH:** either `AGENTDISK_PUBLIC_DIRECTORY_ID` or
> `AGENTDISK_PUBLIC_DIRECTORY_PATH` must be set. If both are set, ID wins
> (zero lookup cost). The PATH form pays one extra HTTP call on the first
> OKF API invocation, then caches the resolved ID for the rest of the
> process.

## Run

### Interactive web UI (recommended for exploration)

```bash
adk web
# open http://localhost:8000 and pick "agentdisk_writer"
```

### CLI

```bash
adk run . --input "Register the bundle and write a markdown note about Llama 3"
```

### Smoke test (no LLM, hits the real HTTP API)

```bash
python -m scenarios.bootstrap_bundle
```

This walks the full writer→reader pipeline:

1. `create_folder("concepts")` (idempotent)
2. `write_markdown("index.md", ...)` with `okf_version: "0.1"`
3. `register_bundle(pd_id)` → bundle_id
4. `write_markdown("concepts/gemma.md", ...)` with full frontmatter
5. `list_nodes(bundle_id)`
6. `aggregate_types()`

Each step prints elapsed ms and the server payload. Useful as a connectivity
check between this demo and Worktree B.

## Switching models

`WRITER_AGENT_MODEL` is read at agent construction time. Supported values
(LiteLLM model-id form):

| Model id | Required env | Default endpoint |
|---|---|---|
| `deepseek/deepseek-chat` (default) | `DEEPSEEK_API_KEY` | `https://api.deepseek.com/beta` (built into LiteLLM — no URL config needed) |
| `deepseek/deepseek-reasoner` | `DEEPSEEK_API_KEY` | same |
| `gemini-2.5-flash` | `GEMINI_API_KEY` (or `GOOGLE_API_KEY`) | Google AI |
| `claude-sonnet-4` | `ANTHROPIC_API_KEY` | Anthropic API |
| `ollama/llama3` | (none) | `OLLAMA_BASE_URL` (default `http://localhost:11434`) |

**DeepSeek 不需要设置 URL**：LiteLLM 已经内置了 DeepSeek 官方端点
`https://api.deepseek.com/beta`。只有走代理/自建端点时才需要设
`DEEPSEEK_API_BASE`。

## 从本地 raw 文件建立知识库

agent 内置 2 个本地读取工具（`list_raw_files` + `read_raw_file`），可
批量扫描本地 raw 目录、推断 OKF frontmatter、按 raw 目录结构透传写入
bundle，并在节点间建立多版本 / 引用 / 上下位三类链接以支持 P3 图检索。

### 配置

可选环境变量 `AGENTDISK_RAW_ROOT` 指定 raw 根目录；不设时默认指向包内
`examples/adk_writer_agent/raw/`（已包含 ~96 个金融监管法规文档作为示例）：

```bash
# 用包内示例（零配置）
unset AGENTDISK_RAW_ROOT

# 或指向自己的 raw 目录
export AGENTDISK_RAW_ROOT=/path/to/my/notes
```

### 扫描规则

- 递归扫描所有子目录
- 跳过隐藏文件（`.foo`）、`.git/`、`node_modules/`、`__pycache__/`、`dist/`、`build/`、`.venv/`、`venv/`
- 文件扩展名白名单：`.md / .markdown / .txt / .rst / .org`
- 总数 > 500 时返回 `truncated=true`，提示缩小范围

### 大文件分块

单文件 > 200 KB 时调用 `read_raw_file(path, chunk=True)`，按 markdown H2
标题切分（H2 仍超 200 KB 再按段落细分），每个 chunk 写为独立 OKF 节点
（`<basename>.part1.md` / `.part2.md`），并生成 `<basename>.toc.md`
索引页（type=toc）。

### 两阶段写入（OKF 严格约束）

OKF 写入路径强制要求 `bundle-relative` 链接的目标节点必须已存在，否则
HTTP 400。agent 按两个阶段执行：

1. **阶段一**：批量写所有节点，正文不含任何 `./xxx.md` 链接（原文中
   的链接转义为反引号代码块或纯文本）
2. **阶段二**：所有节点落库后，调用 `list_nodes` 拿全集，二次写入添加
   多版本 / 引用 / 上下位 / toc 四类链接（目标已存在，校验通过）

### 示例 prompt

```bash
adk run . --input "请扫描 RAW_ROOT 下所有 96 个法规文档，按 raw 目录结构初始化 OKF bundle（public_directory_id=$AGENTDISK_PUBLIC_DIRECTORY_ID），并在节点间建立多版本、引用、上下位三类链接以支持图检索。"
```

### 图检索验证

写入完成后，节点的 bundle-relative 链接会被服务端 WriteMarkdown 自动
物化到 `disk_okf_edge` 表，P3 图查询 API 直接可用：

```bash
# 1 跳邻居
curl "http://localhost:8080/v1/disk/okf/nodes/$NODE_ID/neighbors?dir=both" \
  -H "X-API-Key: $AGENTDISK_API_KEY"

# N 跳可达
curl -X POST "http://localhost:8080/v1/disk/okf/nodes/$NODE_ID/reachable" \
  -H "X-API-Key: $AGENTDISK_API_KEY" \
  -d '{"depth": 2, "limit": 100}'

# 最短路径
curl -X POST "http://localhost:8080/v1/disk/okf/paths/shortest" \
  -H "X-API-Key: $AGENTDISK_API_KEY" \
  -d '{"src": <反洗钱法 nodeId>, "dst": <数据安全法 nodeId>, "maxDepth": 3}'
```

## Project layout

```
examples/adk_writer_agent/
├── pyproject.toml             # build + ruff + mypy + pytest config
├── .env.example               # config template
├── README.md                  # this file
├── adk_writer_agent/
│   ├── __init__.py            # exports root_agent
│   ├── agent.py               # LlmAgent definition + instruction
│   ├── tools.py               # 8 FunctionTool wrappers
│   └── agentdisk_client.py    # OKF HTTP client (httpx)
├── raw/                       # 默认 raw 根目录（金融监管法规示例）
│   ├── 机构监管/
│   ├── 业务管理/
│   └── ...
├── scenarios/
│   └── bootstrap_bundle.py    # end-to-end smoke test
└── tests/                     # pytest unit tests (raw-file tools)
    ├── conftest.py
    ├── test_raw_path_safety.py
    ├── test_list_raw_files.py
    └── test_read_raw_file.py
```

## Caveats

- This demo is excluded from `make lint`, `go test ./...`, and the SDK/web CI
  pipelines — it has its own `ruff check . && mypy .` gate run from this folder.
- It does NOT depend on the AgentDisk Python SDK (`sdk/`); all HTTP is direct.
- The agent's `instruction` is tuned for OKF v0.1 frontmatter rules; if the
  schema evolves in Worktree B, update the instruction accordingly.
