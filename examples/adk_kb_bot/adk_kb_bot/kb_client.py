"""agentdisk SDK wrapper for the kb_bot agent.

This is the only place in the package that talks to the AgentDisk Python
SDK (``agentdisk.AgentDiskClient``). Tools in :mod:`adk_kb_bot.tools` go
through here so the SDK is the single source of truth for OKF read
access — no hand-rolled HTTP, in contrast to
``adk_writer_agent/agentdisk_client.py``.

The wrapper exposes:

* :func:`get_client` — process-wide singleton built lazily from env vars.
* :func:`default_bundle_id` — resolve the ``KB_BOT_BUNDLE_ID`` env var
  (raises with a helpful message if unset; tools catch and surface as a
  tool-error dict so the LLM gets actionable feedback).
* :func:`default_public_directory_id` — resolve
  ``AGENTDISK_PUBLIC_DIRECTORY_ID`` (numeric, same env var the writer
  uses). Falls back to ``KB_BOT_PUBLIC_DIRECTORY_NAME`` resolved to an
  id via :meth:`AgentDiskClient.list_public_directories` for backward
  compatibility with older .env files.
* :func:`default_public_dir_name` — the displayName of the configured
  PD. Used by :func:`read_node_body` to build the SDK download path
  ``"<name>/<rel_path>"`` because the SDK's path resolver keys off the
  leading segment matching a visible PD's displayName.
* :func:`read_node_body` — download an OKF node's markdown body via the
  SDK (OKF read APIs return frontmatter only; the body has to be
  fetched as a file download).

Configuration is read lazily so unit tests can ``monkeypatch.setenv``
before the first call. The singleton is cached via
:func:`functools.lru_cache` — calling ``get_client.cache_clear()`` in
tests wipes it after env changes.
"""

from __future__ import annotations

import os
from functools import lru_cache

from agentdisk import AgentDiskClient, AgentDiskError

__all__ = [
    "AgentDiskError",
    "default_bundle_id",
    "default_public_dir_name",
    "default_public_directory_id",
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
    _resolved_pd_cache_clear()


def default_bundle_id() -> int:
    """Return the bundle id configured via ``KB_BOT_BUNDLE_ID``.

    Tools that take an optional ``bundle_id`` argument fall back to this
    when the LLM omits it (the common case — the bot is configured
    against a single bundle).

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


def default_public_directory_id() -> int:
    """Return the PD id configured via ``AGENTDISK_PUBLIC_DIRECTORY_ID``.

    Mirrors the writer's config (see
    ``adk_writer_agent/adk_writer_agent/agentdisk_client.py``) so the
    two examples can share the same .env block. Falls back to
    ``KB_BOT_PUBLIC_DIRECTORY_NAME`` resolved through
    :meth:`AgentDiskClient.list_public_directories` for callers that
    haven't migrated yet.

    Raises:
        RuntimeError: if neither env var is set, the value isn't a
            positive int, or the name can't be matched to a visible PD.
    """
    raw = os.environ.get("AGENTDISK_PUBLIC_DIRECTORY_ID", "").strip()
    if raw:
        try:
            pd_id = int(raw)
        except ValueError as exc:
            raise RuntimeError(
                f"AGENTDISK_PUBLIC_DIRECTORY_ID must be an int, got {raw!r}"
            ) from exc
        if pd_id <= 0:
            raise RuntimeError(
                f"AGENTDISK_PUBLIC_DIRECTORY_ID must be positive, got {pd_id}"
            )
        return pd_id

    # Legacy fallback: resolve displayName → id via the SDK. This still
    # works for older .env files but produces an extra round-trip on
    # the first call (cached after that).
    name = os.environ.get("KB_BOT_PUBLIC_DIRECTORY_NAME", "").strip().strip("/")
    if not name:
        raise RuntimeError(
            "AGENTDISK_PUBLIC_DIRECTORY_ID is not set (or legacy "
            "KB_BOT_PUBLIC_DIRECTORY_NAME). Set the PD id in .env — "
            "same value the writer uses."
        )
    for pd in get_client().list_public_directories():
        if pd.display_name == name or pd.fixed_path.rstrip("/") == f"/public/{name}":
            return int(pd.id)
    raise RuntimeError(
        f"No visible public directory matches displayName/fixedPath "
        f"{name!r}. Check AGENTDISK_API_KEY scopes or set "
        f"AGENTDISK_PUBLIC_DIRECTORY_ID directly."
    )


@lru_cache(maxsize=1)
def _resolved_pd() -> tuple[int, str]:
    """Look up the configured PD once and cache (id, displayName).

    Both :func:`default_public_dir_name` and :func:`read_node_body` need
    the displayName, but resolving it costs a ``list_public_directories``
    round-trip. Cache it alongside the singleton client; cleared by
    :func:`reset_client_cache`.
    """
    pd_id = default_public_directory_id()
    for pd in get_client().list_public_directories():
        if pd.id == pd_id:
            return pd_id, pd.display_name
    raise RuntimeError(
        f"No visible public directory with id={pd_id}. Check "
        f"AGENTDISK_PUBLIC_DIRECTORY_ID and the API key's grants."
    )


def _resolved_pd_cache_clear() -> None:
    _resolved_pd.cache_clear()


def default_public_dir_name() -> str:
    """Return the displayName of the configured public directory.

    Used by :func:`read_node_body` to build the download path
    ``"<name>/<rel_path>"``. The SDK's path resolver treats a leading
    segment matching a visible public directory's displayName as the PD
    marker and routes the request through the PD-aware code path.

    Raises:
        RuntimeError: if the PD can't be resolved (see
            :func:`default_public_directory_id`).
    """
    return _resolved_pd()[1]


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

    Note:
        SDK resolver limitation — for subfolder paths (more than one
        segment under the PD) the resolver falls back to a private
        ``/v1/disk/files`` route that ``RequireNonAPIKey`` blocks for
        API keys. PD-root files work; subfolder reads may 403 until the
        backend exposes a PD-scoped listing-by-folderId route.
    """
    pd_name = default_public_dir_name()
    clean = rel_path.lstrip("/")
    path = f"{pd_name}/{clean}"

    client = get_client()
    token = client.download_file(path)
    # download_url may be relative (e.g. "/v1/disk/local-storage/..."). The
    # SDK's internal httpx.Client has base_url configured, so reusing it
    # handles both relative and absolute URLs uniformly.
    resp = client._http.get(token.download_url, timeout=30.0)  # noqa: SLF001
    resp.raise_for_status()
    return str(resp.text)
