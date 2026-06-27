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
2. A **public directory** created in the AgentDisk web UI (note its numeric ID)
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
| `AGENTDISK_PUBLIC_DIRECTORY_ID` | Numeric ID of the public directory backing the bundle |
| `WRITER_AGENT_MODEL` | LLM model name (see below) |
| `GEMINI_API_KEY` / `ANTHROPIC_API_KEY` / `DEEPSEEK_API_KEY` | Model provider key (pick one matching `WRITER_AGENT_MODEL`) |

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

`WRITER_AGENT_MODEL` is read at agent construction time. Supported values:

| Model | Required env |
|---|---|
| `gemini-2.5-flash` (default) | `GEMINI_API_KEY` |
| `claude-sonnet-4` | `ANTHROPIC_API_KEY` |
| `deepseek-v4` | `DEEPSEEK_API_KEY` |
| `ollama/llama3` | (none; ensure `OLLAMA_BASE_URL=http://localhost:11434`) |

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
