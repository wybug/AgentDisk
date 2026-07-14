# AgentDisk KB Bot (Google ADK 2.0 Demo)

A Google ADK consumer agent that answers **business questions** by reading the
OKF v0.1 knowledge base through the `agentdisk` Python SDK.

It is the read-side counterpart to `examples/adk_writer_agent`:

```
raw notes ──▶ adk_writer_agent ──▶ OKF bundle ──▶ adk_kb_bot ──▶ grounded answer
            (producer, writes)                  (consumer, reads + cites)
```

The writer maintains the knowledge base; this bot consumes it. The two agents
are decoupled — they share the OKF bundle id but never call each other.

## Why this exists

1. **Close the loop on the OKF example set.** The writer proves the API is
   writable from ADK; the bot proves it is readable.
2. **Validate the `agentdisk` SDK in a real ADK agent.** Unlike the writer
   (which calls REST directly via `httpx`), the bot uses the SDK for every
   OKF operation — `search_okf`, `neighbors`, `reachable`, `shortest_path`,
   `subgraph`, `bundle_stats`, `list_okf_nodes`, `download_file`. The SDK is
   the contract; the bot is its first ADK-shaped customer.
3. **Provide a measurable "with-KB vs without-KB" eval.** `make kb-bot-eval`
   emits a baseline report so prompt / tool / model drift shows up as diffs.

## Prerequisites

1. AgentDisk backend running: `make dev-start` from repo root.
2. A bundle with at least one node. Easiest path — run the writer's bootstrap:
   ```bash
   cd examples/adk_writer_agent
   python -m scenarios.bootstrap_bundle
   ```
   Note the printed `bundleId` and the **public directory name** (the path
   segment, e.g. `test` if the PD is `/public/test`).
3. An **API key** with `okf-read` scope (created in the admin console). The
   writer's `okf-write` key also works.

## Install

```bash
cd examples/adk_kb_bot
pip install -e ".[dev]"
```

For evals add the `eval` extra (pulls `google-adk[eval]` + `litellm`):

```bash
pip install -e ".[eval]"
```

## Configure

```bash
cp .env.example .env
# then edit .env and fill in real values
```

| Var | Meaning |
|---|---|
| `AGENTDISK_BASE_URL` | Backend base URL, e.g. `http://localhost:8080` |
| `AGENTDISK_API_KEY` | API key (read scope is sufficient) |
| `KB_BOT_BUNDLE_ID` | Numeric id of the bundle to consume |
| `AGENTDISK_PUBLIC_DIRECTORY_ID` | Numeric id of the public directory backing the bundle (same var as `adk_writer_agent`; resolved to displayName for SDK download paths) |
| `KB_BOT_MODEL` | LiteLLM model id (default `deepseek/deepseek-chat`) |
| `DEEPSEEK_API_KEY` / `GEMINI_API_KEY` / `ANTHROPIC_API_KEY` | Provider key for the chosen model |

## Run

### Interactive web UI

```bash
adk web
# open http://localhost:8000 and pick "kb_bot"
```

### CLI

```bash
adk run . --input "反洗钱法对客户身份识别有什么要求？"
```

Expected trajectory: `search_knowledge` → `neighbors` (expand multi-hop) →
`read_node_content` (fetch the relevant article) → grounded answer with
`[nodeId=N, relPath=..., title=...]` citations.

### SDK smoke (no LLM)

```bash
python -m scenarios.ask_business
```

Walks `list_bundles → bundle_stats → search_okf → neighbors → read_node_body`
directly against the SDK. Prints elapsed ms per step — a quick connectivity
sanity check.

## Tool set

Ten `FunctionTool`s exposed to the LLM. Each returns
`{"ok": True, "data": ...}` on success or `{"ok": False, "error": ..., "code": ..., "httpStatus": ...}` on failure.

| Tool | Wraps | Use when |
|---|---|---|
| `list_bundles` | `client.list_bundles()` | Enumerate available knowledge bases |
| `get_bundle` | `client.get_bundle(id)` | Inspect a bundle's metadata |
| `bundle_stats` | `client.bundle_stats(id)` | Get type / edge counts |
| `search_knowledge` | `client.search_okf(query, ...)` | **Primary entry point** for any business question |
| `list_nodes` | `client.list_okf_nodes(...)` | Browse by type / tag |
| `neighbors` | `client.neighbors(node_id, ...)` | One-hop expansion |
| `reachable` | `client.reachable(node_id, ...)` | N-hop subgraph |
| `shortest_path` | `client.shortest_path(src, dst, ...)` | "Does A derive from B?" |
| `subgraph` | `client.subgraph(bundle_id, ...)` | Sample the graph |
| `read_node_content` | `client.download_file("<pd>/<rel>")` + httpx | Fetch the markdown body (frontmatter alone is not enough for grounded answers) |

The agent's `INSTRUCTION` (in `adk_kb_bot/agent.py`) mandates the workflow:
search first, expand via graph traversal when thin, read the body before
quoting, cite every factual claim, and **explicitly refuse when the KB doesn't
cover the question** (highest-priority rule — no fabrication from training
memory).

## Switching models

Same scheme as the writer. `KB_BOT_MODEL` is read at agent construction time:

| Model id | Required env |
|---|---|
| `deepseek/deepseek-chat` (default) | `DEEPSEEK_API_KEY` |
| `deepseek/deepseek-reasoner` | `DEEPSEEK_API_KEY` |
| `gemini/gemini-2.5-flash` | `GEMINI_API_KEY` (or `GOOGLE_API_KEY`) |
| `claude/claude-haiku-4-5-20251001` | `ANTHROPIC_API_KEY` |

## Evaluate

```bash
make kb-bot-eval                # from repo root
# or
python evals/run_evals.py
```

Four rubric-based cases:

| Case | Asserts |
|---|---|
| `search_first_for_business_question` | Calls `search_knowledge` before any other OKF tool |
| `cite_source_when_answered` | Final response contains at least one `[nodeId=N, relPath=...]` citation; no invented article numbers |
| `refuse_when_kb_uncovered` | Out-of-scope question → explicit "知识库未覆盖" + no fabricated statute |
| `expand_via_neighbors_when_thin` | Multi-hop question → `neighbors` / `reachable` after search |

Rubric scoring (not trajectory matching) — LLM phrasing varies, the contract
is structural (cite, refuse, expand). See `evals/README.md`.

`evals/baseline_results.md` is regenerated on every run; commit it to track
drift.

## Repository layout

```
adk_kb_bot/
├── __init__.py            # exports root_agent
├── agent.py               # LlmAgent + INSTRUCTION
├── tools.py               # 10 FunctionTools
└── kb_client.py           # agentdisk SDK singleton + read_node_body helper
scenarios/
└── ask_business.py        # SDK smoke (no LLM)
tests/
├── conftest.py
├── test_kb_client.py      # client singleton + read_node_body
└── test_tools.py          # tool success/error payloads
evals/
├── README.md
├── run_evals.py
├── eval_setup.py
├── kb_bot_eval_set.json   # 4 rubric cases
└── baseline_results.md    # regenerated per run
```

## Relationship to `adk_writer_agent`

| Aspect | writer | kb_bot |
|---|---|---|
| Role | Producer — writes notes | Consumer — answers questions |
| HTTP layer | `httpx` direct | `agentdisk` SDK |
| Auth scope | `okf-write` | `okf-read` (write key also works) |
| State mutation | Yes — creates nodes / edges | No — purely read-only |
| Eval suite | Trajectory-heavy (writes are deterministic) | Rubric-heavy (reads vary) |

They never talk to each other; they share only the OKF bundle id.
