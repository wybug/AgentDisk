"""End-to-end OKF bootstrap smoke test (no LLM involved).

This scenario walks the full writer→reader pipeline directly against the
AgentDisk HTTP API. It exists to verify:

1. The HTTP client can talk to a running AgentDisk service.
2. The 6 OKF endpoints compose into a coherent bundle bootstrap.
3. The unified ``{code, message, data}`` envelope is parsed correctly.

It is INTENTIONALLY not part of the importable ``adk_writer_agent`` package
(excluded from setuptools, ruff, and mypy configs) because it touches a live
network. Run it with:

    cd examples/adk_writer_agent
    python -m scenarios.bootstrap_bundle

Requires the env vars documented in ``.env.example``.
"""
from __future__ import annotations

import os
import sys
import time
from pathlib import Path
from typing import Any

# Allow `python -m scenarios.bootstrap_bundle` to find the adk_writer_agent
# package even when the project is NOT pip-installed. We sys.path-mutate BEFORE
# the import. When the package is installed via `pip install -e .` this is a
# no-op (the import resolves via the installed egg-link).
_PKG_ROOT = Path(__file__).resolve().parent.parent
if str(_PKG_ROOT) not in sys.path:
    sys.path.insert(0, str(_PKG_ROOT))

# python-dotenv is optional at runtime; only used for local dev convenience.
try:
    from dotenv import load_dotenv  # type: ignore[import-not-found]

    load_dotenv(_PKG_ROOT / ".env")
except ImportError:  # pragma: no cover - dev convenience only
    pass

from adk_writer_agent.agentdisk_client import AgentDiskClient, AgentDiskError  # noqa: E402

# Bundle contents — minimal but valid OKF v0.1.
INDEX_MD = """\
---
okf_version: "0.1"
title: Demo Bundle
description: Smoke-test bundle created by scenarios/bootstrap_bundle.py
---

# Demo Bundle

Created by the AgentDisk ADK writer agent demo. See `concepts/gemma.md`.
"""

GEMMA_MD = """\
---
type: llm
title: Gemma
description: Open-weight LLM family from Google
tags: ["google", "open", "open-weights"]
timestamp: 2026-06-27T00:00:00Z
---

# Gemma

Gemma is a family of open-weight LLMs from Google. See
[/concepts/gemma.md](./gemma.md) for self-reference sanity.

## Notes

- Released in 2B / 7B / 9B variants.
- Apache 2.0 license.
"""


def _step(name: str) -> None:
    print(f"\n=== {name} ===", flush=True)


def _elapsed(start: float) -> str:
    return f"{(time.perf_counter() - start) * 1000:.1f}ms"


def _require_env(name: str) -> str:
    value = os.environ.get(name)
    if not value:
        sys.stderr.write(
            f"missing env var {name}. Copy .env.example to .env and fill it in.\n"
        )
        sys.exit(2)
    return value


def main() -> int:
    base_url = _require_env("AGENTDISK_BASE_URL")
    api_key = _require_env("AGENTDISK_API_KEY")
    pd_id_raw = _require_env("AGENTDISK_PUBLIC_DIRECTORY_ID")
    try:
        pd_id = int(pd_id_raw)
    except ValueError:
        sys.stderr.write(
            f"AGENTDISK_PUBLIC_DIRECTORY_ID must be an int, got {pd_id_raw!r}\n"
        )
        return 2

    client = AgentDiskClient(base_url=base_url, api_key=api_key)
    bundle_id: int | None = None
    try:
        # Step 1: ensure the concepts/ folder exists. Idempotent — ignore
        # "already exists" failures.
        _step("create_folder(concepts)")
        t = time.perf_counter()
        try:
            result = client.create_folder(pd_id, "concepts")
            print(f"created in {_elapsed(t)}: {result}")
        except AgentDiskError as exc:
            print(f"skipped ({exc}) in {_elapsed(t)}")

        # Step 2: write index.md BEFORE register so the bundle has a root.
        _step("write_markdown(index.md)")
        t = time.perf_counter()
        idx = client.write_markdown(pd_id, "index.md", INDEX_MD)
        print(f"wrote in {_elapsed(t)}: {idx}")

        # Step 3: register the bundle and capture the bundle id.
        _step("register_bundle")
        t = time.perf_counter()
        bundle = client.register_bundle(pd_id)
        print(f"registered in {_elapsed(t)}: {bundle}")
        bundle_id = _coerce_int(bundle.get("bundleId"))
        if bundle_id is None:
            sys.stderr.write(f"register_bundle returned no bundleId: {bundle}\n")
            return 1

        # Step 4: write a real content file into concepts/.
        _step("write_markdown(concepts/gemma.md)")
        t = time.perf_counter()
        gemma = client.write_markdown(pd_id, "concepts/gemma.md", GEMMA_MD)
        print(f"wrote in {_elapsed(t)}: {gemma}")

        # Step 5: read back the bundle nodes.
        _step("list_nodes")
        t = time.perf_counter()
        nodes = client.list_nodes(bundle_id)
        print(f"listed in {_elapsed(t)}: {nodes}")

        # Step 6: aggregate types across the caller's bundles.
        _step("aggregate_types")
        t = time.perf_counter()
        types = client.aggregate_types()
        print(f"aggregated in {_elapsed(t)}: {types}")

    finally:
        client.close()

    print("\nAll steps completed.")
    return 0


def _coerce_int(value: Any) -> int | None:
    """Coerce JSON numbers (int or float) into a clean int."""
    if value is None:
        return None
    if isinstance(value, bool):
        return None
    if isinstance(value, int):
        return value
    if isinstance(value, float):
        return int(value)
    try:
        return int(str(value))
    except (TypeError, ValueError):
        return None


if __name__ == "__main__":
    raise SystemExit(main())
