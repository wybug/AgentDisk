"""ADK FunctionTool wrappers around the AgentDisk OKF HTTP client.

Each tool here mirrors one OKF API and is registered on the root LlmAgent in
:mod:`adk_writer_agent.agent`. Tools are intentionally thin: parameter
validation and persistence live in :mod:`adk_writer_agent.agentdisk_client`,
while these wrappers translate client errors into ADK-friendly dicts.
"""
from __future__ import annotations

import os
from functools import lru_cache
from typing import Any

from google.adk.tools.function_tool import FunctionTool

from .agentdisk_client import AgentDiskClient, AgentDiskError


# ----------------------------------------------------------------------
# Client lifecycle
# ----------------------------------------------------------------------
@lru_cache(maxsize=1)
def _get_client() -> AgentDiskClient:
    """Return a process-wide :class:`AgentDiskClient` built from env vars.

    The ADK runtime instantiates tools as module-level singletons, so a module
    global is fine here. Configuration is read lazily so unit tests can
    monkeypatch ``os.environ`` before the first call.
    """
    base_url = os.environ.get("AGENTDISK_BASE_URL")
    api_key = os.environ.get("AGENTDISK_API_KEY")
    if not base_url:
        raise RuntimeError("AGENTDISK_BASE_URL is not set")
    if not api_key:
        raise RuntimeError("AGENTDISK_API_KEY is not set")
    return AgentDiskClient(base_url=base_url, api_key=api_key)


def _err_payload(exc: AgentDiskError) -> dict[str, Any]:
    """Convert :class:`AgentDiskError` into a serializable dict for the LLM."""
    return {
        "ok": False,
        "error": exc.message,
        "httpStatus": exc.status_code,
        "code": exc.code,
    }


def _ok_payload(data: dict[str, Any]) -> dict[str, Any]:
    return {"ok": True, "data": data}


# ----------------------------------------------------------------------
# Tool functions (docstrings are read by the LLM)
# ----------------------------------------------------------------------
def register_bundle(public_directory_id: int) -> dict[str, Any]:
    """Register a public directory as an OKF v0.1 bundle.

    Call this ONCE per public directory. Re-registering an already-registered
    directory returns the existing bundle id (idempotent on the server side).

    Args:
        public_directory_id: ID of the public directory that should back the
            bundle. Must already exist.

    Returns:
        ``{"ok": True, "data": {"bundleId": int, ...}}`` on success, or
        ``{"ok": False, "error": str, ...}`` on failure.
    """
    try:
        data = _get_client().register_bundle(public_directory_id)
    except AgentDiskError as exc:
        return _err_payload(exc)
    return _ok_payload(data)


def create_folder(
    folder_name: str,
    parent_id: int | None = None,
    public_directory_id: int | None = None,
) -> dict[str, Any]:
    """Create a folder inside the bundle's public directory.

    Use this before writing files into a sub-tree (e.g. ``concepts/``). If the
    folder already exists the server returns an error; treat that as success
    for idempotent bootstrap.

    Args:
        folder_name: Folder name, no slashes (e.g. ``"concepts"``).
        parent_id: Optional parent folder id. If omitted, creates at the
            bundle root.
        public_directory_id: Public directory id. If omitted, reads
            ``AGENTDISK_PUBLIC_DIRECTORY_ID`` from env.

    Returns:
        ``{"ok": True, "data": {"folderId": int, "name": str}}`` or error.
    """
    pd_id = public_directory_id or _default_pd_id()
    try:
        data = _get_client().create_folder(pd_id, folder_name, parent_id)
    except AgentDiskError as exc:
        return _err_payload(exc)
    return _ok_payload(data)


def write_markdown(
    rel_path: str,
    content: str,
    public_directory_id: int | None = None,
) -> dict[str, Any]:
    """Write a markdown file into the bundle. The core OKF write primitive.

    The file MUST start with YAML frontmatter delimited by ``---`` lines. At
    minimum include a ``type`` field (free-form lowercase string, e.g.
    ``concept``/``playbook``/``reference``/``llm``). Recommended fields:
    ``title``, ``description``, ``tags`` (list), ``timestamp`` (ISO 8601).
    Bundle-relative links use paths like ``/concepts/gemma.md`` or
    ``./gemma.md``. Special files: ``index.md`` (bundle root, requires
    ``okf_version: "0.1"``) and ``log.md`` (update history).

    If the server rejects the write (HTTP 400), inspect the returned error and
    fix the frontmatter, then retry once.

    Args:
        rel_path: Bundle-relative path, e.g. ``"concepts/gemma.md"`` or
            ``"index.md"``. No leading slash.
        content: Full file content including YAML frontmatter.
        public_directory_id: Public directory id. If omitted, reads
            ``AGENTDISK_PUBLIC_DIRECTORY_ID`` from env.

    Returns:
        ``{"ok": True, "data": {"nodeId": int, "relPath": str}}`` or error.
    """
    pd_id = public_directory_id or _default_pd_id()
    try:
        data = _get_client().write_markdown(pd_id, rel_path, content)
    except AgentDiskError as exc:
        return _err_payload(exc)
    return _ok_payload(data)


def list_nodes(
    bundle_id: int,
    type: str | None = None,
    tag: str | None = None,
) -> dict[str, Any]:
    """List nodes inside an OKF bundle, optionally filtered.

    Args:
        bundle_id: OKF bundle id (from :func:`register_bundle`).
        type: Optional node-type filter (matches frontmatter ``type``).
        tag: Optional tag filter (single tag, exact match).

    Returns:
        ``{"ok": True, "data": {"nodes": [{"nodeId": int, ...}]}}`` or error.
    """
    try:
        data = _get_client().list_nodes(bundle_id, type=type, tag=tag)
    except AgentDiskError as exc:
        return _err_payload(exc)
    return _ok_payload(data)


def aggregate_types() -> dict[str, Any]:
    """Aggregate node counts per ``type`` across all bundles the caller owns.

    No parameters. Useful for a quick "what's in the knowledge base" summary.

    Returns:
        ``{"ok": True, "data": {"types": [{"type": str, "count": int}, ...]}}``
        or error.
    """
    try:
        data = _get_client().aggregate_types()
    except AgentDiskError as exc:
        return _err_payload(exc)
    return _ok_payload(data)


def refresh_index(bundle_id: int) -> dict[str, Any]:
    """Force-rebuild the in-memory index of an OKF bundle from on-disk files.

    Call after a batch of direct writes (outside the HTTP API), or to recover
    from index drift. Reads the public directory recursively; can be slow on
    large bundles.

    Args:
        bundle_id: OKF bundle id.

    Returns:
        ``{"ok": True, "data": {"bundleId": int, "nodes": int}}`` or error.
    """
    try:
        data = _get_client().refresh_index(bundle_id)
    except AgentDiskError as exc:
        return _err_payload(exc)
    return _ok_payload(data)


def _default_pd_id() -> int:
    """Read the default public directory id from the environment."""
    raw = os.environ.get("AGENTDISK_PUBLIC_DIRECTORY_ID")
    if raw is None or raw == "":
        raise RuntimeError("AGENTDISK_PUBLIC_DIRECTORY_ID is not set")
    return int(raw)


# ----------------------------------------------------------------------
# FunctionTool singletons
# ----------------------------------------------------------------------
register_bundle_tool = FunctionTool(func=register_bundle)
create_folder_tool = FunctionTool(func=create_folder)
write_markdown_tool = FunctionTool(func=write_markdown)
list_nodes_tool = FunctionTool(func=list_nodes)
aggregate_types_tool = FunctionTool(func=aggregate_types)
refresh_index_tool = FunctionTool(func=refresh_index)

__all__ = [
    "aggregate_types_tool",
    "create_folder_tool",
    "list_nodes_tool",
    "refresh_index_tool",
    "register_bundle_tool",
    "write_markdown_tool",
]
