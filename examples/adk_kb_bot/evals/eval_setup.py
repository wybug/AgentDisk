"""Minimal eval precondition checks for kb_bot.

Unlike ``adk_writer_agent/evals/eval_setup.py``, this is **read-only** —
kb_bot consumes an existing bundle and never mutates OKF state. So there's
no per-case state wipe. What we DO need:

1. Verify the env is configured (BASE_URL / API_KEY / BUNDLE_ID).
2. Verify the bundle exists and has at least one node (otherwise every
   eval case trivially fails with "no results").

The check is best-effort: a failure is logged but doesn't abort — the
caller surfaces the actual eval failures, which are more actionable.
"""

from __future__ import annotations

import logging
import os

logger = logging.getLogger("kb_bot_eval_setup")


def check_preconditions() -> dict[str, object]:
    """Verify env + bundle; return a small report dict.

    Returns:
        ``{"ok": bool, "bundle_id": Optional[int], "node_count": int,
        "error": Optional[str]}``

    Never raises — failures are surfaced via the ``error`` field so the
    eval runner can decide whether to abort or proceed.
    """
    missing = [
        name
        for name in (
            "AGENTDISK_BASE_URL",
            "AGENTDISK_API_KEY",
            "KB_BOT_BUNDLE_ID",
        )
        if not os.environ.get(name, "").strip()
    ]
    if missing:
        return {
            "ok": False,
            "bundle_id": None,
            "node_count": 0,
            "error": f"missing env vars: {', '.join(missing)}",
        }

    try:
        bid = int(os.environ["KB_BOT_BUNDLE_ID"])
    except ValueError:
        return {
            "ok": False,
            "bundle_id": None,
            "node_count": 0,
            "error": f"KB_BOT_BUNDLE_ID not an int: {os.environ['KB_BOT_BUNDLE_ID']!r}",
        }

    # Lazy import so unit tests don't pay the SDK import cost.
    try:
        from agentdisk import AgentDiskClient, AgentDiskError
    except ImportError as exc:
        return {
            "ok": False,
            "bundle_id": bid,
            "node_count": 0,
            "error": f"agentdisk SDK not importable: {exc}",
        }

    try:
        client = AgentDiskClient(
            base_url=os.environ["AGENTDISK_BASE_URL"],
            api_key=os.environ["AGENTDISK_API_KEY"],
        )
        stats = client.bundle_stats(bid)
        client.close()
    except AgentDiskError as exc:
        return {
            "ok": False,
            "bundle_id": bid,
            "node_count": 0,
            "error": f"bundle_stats failed: {exc}",
        }

    return {
        "ok": True,
        "bundle_id": bid,
        "node_count": stats.node_count,
        "error": None,
    }
