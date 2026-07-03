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

## Project layout

```
examples/adk_writer_agent/
├── pyproject.toml             # build + ruff + mypy config
├── .env.example               # config template
├── README.md                  # this file
├── adk_writer_agent/
│   ├── __init__.py            # exports root_agent
│   ├── agent.py               # LlmAgent definition + instruction
│   ├── tools.py               # 6 FunctionTool wrappers
│   └── agentdisk_client.py    # OKF HTTP client (httpx)
└── scenarios/
    └── bootstrap_bundle.py    # end-to-end smoke test
```

## Caveats

- This demo is excluded from `make lint`, `go test ./...`, and the SDK/web CI
  pipelines — it has its own `ruff check . && mypy .` gate run from this folder.
- It does NOT depend on the AgentDisk Python SDK (`sdk/`); all HTTP is direct.
- The agent's `instruction` is tuned for OKF v0.1 frontmatter rules; if the
  schema evolves in Worktree B, update the instruction accordingly.
