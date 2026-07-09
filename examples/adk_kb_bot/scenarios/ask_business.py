"""End-to-end SDK smoke test (no LLM involved).

This scenario walks a read-only pipeline directly against the AgentDisk
SDK + HTTP API. It exists to verify:

1. The ``agentdisk`` Python SDK can talk to a running AgentDisk service
   with API-Key auth (the same auth mode ``adk_kb_bot`` uses in prod).
2. The OKF read APIs compose into a coherent Q&A flow:
   bundle lookup → search → neighbors → content fetch.
3. The bot's expected preconditions (a registered bundle with at least
   one node) are met. If they aren't, the scenario points the user at
   ``adk_writer_agent`` to build the KB first.

It is INTENTIONALLY not part of the importable ``adk_kb_bot`` package
(excluded from setuptools, ruff, and mypy configs) because it touches a
live network. Run it with:

    cd examples/adk_kb_bot
    python -m scenarios.ask_business

Prerequisites:

* AgentDisk backend running (``make dev-start`` from repo root).
* A public directory with at least one OKF bundle registered. Easiest
  path: ``cd ../adk_writer_agent && python -m scenarios.bootstrap_bundle``.
* ``.env`` filled in (see ``.env.example``).
"""
from __future__ import annotations

import os
import sys
import time
from pathlib import Path
from typing import Any

# Allow `python -m scenarios.ask_business` to find the adk_kb_bot
# package even when the project is NOT pip-installed. Same sys.path
# trick as adk_writer_agent/scenarios/bootstrap_bundle.py.
_PKG_ROOT = Path(__file__).resolve().parent.parent
if str(_PKG_ROOT) not in sys.path:
    sys.path.insert(0, str(_PKG_ROOT))

# python-dotenv is optional at runtime; only used for local dev convenience.
try:
    from dotenv import load_dotenv  # type: ignore[import-not-found]

    load_dotenv(_PKG_ROOT / ".env")
except ImportError:  # pragma: no cover - dev convenience only
    pass

from agentdisk import AgentDiskClient, AgentDiskError  # noqa: E402

from adk_kb_bot.kb_client import read_node_body  # noqa: E402


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


def _pick_bundle(client: AgentDiskClient) -> int:
    """Resolve the bundle id to query.

    Order:

    1. ``KB_BOT_BUNDLE_ID`` if set (the bot's normal config).
    2. Otherwise: list bundles, use the first one's id, warn the user
       that they should pin ``KB_BOT_BUNDLE_ID`` for production.
    3. No bundles at all → instruct the user to run
       ``adk_writer_agent/scenarios/bootstrap_bundle.py`` first.
    """
    raw = os.environ.get("KB_BOT_BUNDLE_ID", "").strip()
    if raw:
        try:
            return int(raw)
        except ValueError:
            sys.stderr.write(f"KB_BOT_BUNDLE_ID not an int: {raw!r}\n")
            sys.exit(2)

    bundles = client.list_bundles()
    if not bundles:
        sys.stderr.write(
            "no OKF bundles visible. Run "
            "`cd ../adk_writer_agent && python -m scenarios.bootstrap_bundle` "
            "first to seed one.\n"
        )
        sys.exit(1)
    bid = bundles[0].bundle_id
    print(
        f"[warn] KB_BOT_BUNDLE_ID unset; using first visible bundle "
        f"({bid}, title={bundles[0].title!r}). Pin it in .env for prod."
    )
    return bid


def _pick_search_query() -> str:
    """Pick a search query — env-overridable, sensible default for the
    bundled 金融监管 sample data shipped with adk_writer_agent/raw/."""
    return os.environ.get("KB_BOT_SEARCH_QUERY", "反洗钱").strip() or "反洗钱"


def main() -> int:
    base_url = _require_env("AGENTDISK_BASE_URL")
    api_key = _require_env("AGENTDISK_API_KEY")
    pd_name = os.environ.get("KB_BOT_PUBLIC_DIRECTORY_NAME", "").strip()
    if not pd_name:
        sys.stderr.write(
            "KB_BOT_PUBLIC_DIRECTORY_NAME is not set; required for "
            "read_node_body. Skipping that step.\n"
        )

    client = AgentDiskClient(base_url=base_url, api_key=api_key)
    try:
        # Step 1: pick bundle + show its metadata.
        _step("resolve bundle")
        bundle_id = _pick_bundle(client)

        _step("get_bundle")
        t = time.perf_counter()
        bundle = client.get_bundle(bundle_id)
        print(
            f"got bundle in {_elapsed(t)}: id={bundle.bundle_id} "
            f"title={bundle.title!r} nodes={bundle.node_count} "
            f"edges={bundle.edge_count}"
        )

        # Step 2: coverage stats — type distribution shapes LLM expectations.
        _step("bundle_stats")
        t = time.perf_counter()
        stats = client.bundle_stats(bundle_id)
        type_summary = ", ".join(
            f"{tc.type}={tc.count}" for tc in stats.types[:8]
        )
        print(
            f"stats in {_elapsed(t)}: live={stats.edge_live}/{stats.edge_total} "
            f"broken={stats.edge_broken} types=[{type_summary}]"
        )

        # Step 3: search — primary Q&A entry point.
        query = _pick_search_query()
        _step(f'search_okf("{query}")')
        t = time.perf_counter()
        page = client.search_okf(query, bundle_id=bundle_id, limit=5)
        print(
            f"search in {_elapsed(t)}: {len(page.nodes)} hits "
            f"(nextCursor={page.next_cursor})"
        )
        for n in page.nodes[:3]:
            title = n.title or "(untitled)"
            print(f"  - nodeId={n.node_id} type={n.type} title={title!r}")
            print(f"    relPath={n.rel_path}")

        # Step 4: neighbors of the top hit — typical Q&A expansion.
        top = page.nodes[0] if page.nodes else None
        if top is not None:
            _step(f'neighbors(nodeId={top.node_id})')
            t = time.perf_counter()
            nbr = client.neighbors(top.node_id, direction="both")
            print(
                f"neighbors in {_elapsed(t)}: {len(nbr.nodes)} nodes, "
                f"{len(nbr.edges)} edges"
            )
            for e in nbr.edges[:3]:
                link = e.link_text or "(no text)"
                arrow = "->" if e.src_node_id == top.node_id else "<-"
                print(
                    f"  - {e.src_node_id} {arrow} {e.dst_node_id} "
                    f"[{e.link_kind}] {link!r} "
                    f"(dst_exists={e.dst_exists})"
                )

        # Step 5: read full markdown body of the top hit.
        if top is not None and pd_name:
            _step(f'read_node_body("{top.rel_path}")')
            t = time.perf_counter()
            try:
                body = read_node_body(top.rel_path)
                size_kb = len(body.encode("utf-8")) / 1024
                preview = body[:200].replace("\n", " ")
                print(
                    f"read in {_elapsed(t)}: {size_kb:.1f} KB, "
                    f"preview={preview!r}"
                )
            except AgentDiskError as exc:
                print(f"read failed in {_elapsed(t)}: {exc}")

    finally:
        client.close()

    print("\nAll steps completed.")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
