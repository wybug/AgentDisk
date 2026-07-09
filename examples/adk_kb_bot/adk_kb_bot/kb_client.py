"""agentdisk SDK wrapper for the kb_bot agent.

This is the only place in the package that talks to the AgentDisk Python SDK
(``agentdisk.AgentDiskClient``). Tools in :mod:`adk_kb_bot.tools` go through
here so the SDK is the single source of truth for OKF read access — no
hand-rolled HTTP, in contrast to ``adk_writer_agent/agentdisk_client.py``.

The wrapper exposes:

* :func:`get_client` — process-wide singleton built lazily from env vars.
* :func:`default_bundle_id` — resolve the ``KB_BOT_BUNDLE_ID`` env var
  (raises with a helpful message if unset; tools catch and surface as a
  tool-error dict so the LLM gets actionable feedback).
* :func:`default_public_dir_name` — same for ``KB_BOT_PUBLIC_DIRECTORY_NAME``.
* :func:`read_node_body` — download an OKF node's markdown body via the SDK
  (OKF read APIs return frontmatter only; the body has to be fetched as a
  file download).

Configuration is read lazily so unit tests can ``monkeypatch.setenv`` before
the first call. The singleton is cached via :func:`functools.lru_cache` —
calling ``get_client.cache_clear()`` in tests wipes it after env changes.
"""

from __future__ import annotations

import os
from functools import lru_cache

import httpx
from agentdisk import AgentDiskClient, AgentDiskError

__all__ = [
    "AgentDiskError",
    "default_bundle_id",
    "default_public_dir_name",
    "get_client",
    "read_node_body",
    "reset_client_cache",
]


@lru_cache(maxsize=1)
def get_client() -> AgentDiskClient:
    """Return the process-wide :class:`AgentDiskClient`.

    Reads ``AGENTDISK_BASE_URL`` and ``AGENTDISK_API_KEY`` lazily so tests
    can set them just before the first call. A future ``token=`` mode is
    a drop-in extension (SDK accepts either), but the writer/bot pair
    stays on API Key for consistency.

    Raises:
        RuntimeError: if either env var is missing.
    """
    base_url = os.environ.get("AGENTDISK_BASE_URL")
    api_key = os.environ.get("AGENTDISK_API_KEY")
    if not base_url:
        raise RuntimeError("AGENTDISK_BASE_URL is not set")
    if not api_key:
        raise RuntimeError("AGENTDISK_API_KEY is not set")
    return AgentDiskClient(base_url=base_url, api_key=api_key)


def reset_client_cache() -> None:
    """Clear the singleton cache. Used by tests after monkeypatching env."""
    get_client.cache_clear()


def default_bundle_id() -> int:
    """Return the bundle id configured via ``KB_BOT_BUNDLE_ID``.

    Tools that take an optional ``bundle_id`` argument fall back to this
    when the LLM omits it (the common case — the bot is configured against
    a single bundle).

    Raises:
        RuntimeError: if the env var is unset or not a positive int.
    """
    raw = os.environ.get("KB_BOT_BUNDLE_ID", "").strip()
    if not raw:
        raise RuntimeError(
            "KB_BOT_BUNDLE_ID is not set; either set it in .env or pass "
            "bundle_id explicitly to the tool"
        )
    try:
        bid = int(raw)
    except ValueError as exc:
        raise RuntimeError(f"KB_BOT_BUNDLE_ID must be an int, got {raw!r}") from exc
    if bid <= 0:
        raise RuntimeError(f"KB_BOT_BUNDLE_ID must be positive, got {bid}")
    return bid


def default_public_dir_name() -> str:
    """Return the public directory ``displayName`` used as a path prefix.

    Used by :func:`read_node_body` to build the download path
    ``"<name>/<rel_path>"``. The SDK's path resolver treats a leading
    segment matching a visible public directory's name as the PD marker
    and routes the request through the PD-aware code path.

    Raises:
        RuntimeError: if the env var is unset.
    """
    name = os.environ.get("KB_BOT_PUBLIC_DIRECTORY_NAME", "").strip().strip("/")
    if not name:
        raise RuntimeError(
            "KB_BOT_PUBLIC_DIRECTORY_NAME is not set; required by read_node_content"
        )
    return name


def read_node_body(rel_path: str) -> str:
    """Fetch the raw markdown body of an OKF node.

    The OKF read APIs (``list_okf_nodes`` / ``search_okf`` / ``neighbors``)
    return :class:`~agentdisk.models.wiki.OkfNode` objects carrying only
    frontmatter fields (``title`` / ``description`` / ``tags`` / ``type``).
    The full markdown body is not in the OKF payload — it has to be
    fetched as a regular file download. This helper papers over that
    two-step dance so the LLM-facing tool can pretend the body is just
    one call away.

    Args:
        rel_path: Bundle-relative path (e.g. ``"机构监管/反洗钱法.md"``).

    Returns:
        The file's full text content (UTF-8).

    Raises:
        AgentDiskError: if the SDK can't resolve or download the file.
        httpx.HTTPError: if the download URL GET fails.
    """
    pd_name = default_public_dir_name()
    clean = rel_path.lstrip("/")
    path = f"{pd_name}/{clean}"

    token = get_client().download_file(path)
    # download_url is presigned — a plain GET yields the bytes.
    resp = httpx.get(token.download_url, timeout=30.0)
    resp.raise_for_status()
    return resp.text
