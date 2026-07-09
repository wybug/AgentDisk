"""ADK FunctionTool wrappers around the agentdisk SDK's OKF read APIs.

Each tool here mirrors one SDK method on
:class:`agentdisk.AgentDiskClient` and is registered on the root LlmAgent
in :mod:`adk_kb_bot.agent`. Tools are intentionally thin: SDK calls +
dataclass → dict coercion live here, while the SDK itself owns wire
protocols and path resolution.

All tools return a uniform envelope so the LLM can pattern-match:

* Success: ``{"ok": True, "data": ...}``
* Failure: ``{"ok": False, "error": str, "code": Optional[int], "httpStatus": Optional[int]}``

The dataclass → dict coercion drops internal fields the LLM doesn't need
(``content_hash``, ``created_at``, ``file_id``) and keeps the load-bearing
ones (``nodeId``, ``relPath``, ``title``, ``tags``). The principle is:
every field in the dict is something the LLM might cite or reason about.
"""

from __future__ import annotations

from typing import Any

from agentdisk import AgentDiskError
from agentdisk.models.wiki import (
    BundleStats,
    NeighborsResult,
    OkfBundle,
    OkfEdge,
    OkfNode,
    ShortestPathResult,
    SubgraphResult,
    TypeCount,
)
from google.adk.tools.function_tool import FunctionTool

from .kb_client import default_bundle_id, get_client, read_node_body

__all__ = [
    "bundle_stats_tool",
    "get_bundle_tool",
    "list_bundles_tool",
    "list_nodes_tool",
    "neighbors_tool",
    "read_node_content_tool",
    "reachable_tool",
    "search_knowledge_tool",
    "shortest_path_tool",
    "subgraph_tool",
]


# ----------------------------------------------------------------------
# Payload helpers
# ----------------------------------------------------------------------
def _err_payload(exc: AgentDiskError) -> dict[str, Any]:
    """Convert an :class:`AgentDiskError` into a serializable error dict."""
    payload: dict[str, Any] = {
        "ok": False,
        "error": str(exc),
    }
    code = getattr(exc, "code", None)
    if code is not None:
        payload["code"] = code
    http_status = getattr(exc, "http_status", None)
    if http_status is not None:
        payload["httpStatus"] = http_status
    return payload


def _runtime_err(exc: RuntimeError) -> dict[str, Any]:
    """Config errors (missing env, etc.) — surfaced as plain error strings."""
    return {"ok": False, "error": str(exc)}


def _ok(data: Any) -> dict[str, Any]:
    return {"ok": True, "data": data}


def _node_to_dict(node: OkfNode) -> dict[str, Any]:
    """Coerce an :class:`OkfNode` to an LLM-friendly dict.

    Drops internal bookkeeping (``content_hash`` / ``file_id`` / timestamps)
    and keeps only fields the LLM might cite or reason about. ``relPath`` is
    the key the LLM passes back to :func:`read_node_content` to fetch the
    full body.
    """
    return {
        "nodeId": node.node_id,
        "bundleId": node.bundle_id,
        "relPath": node.rel_path,
        "type": node.type,
        "title": node.title,
        "description": node.description,
        "tags": list(node.tags),
        "hasBrokenLink": node.has_broken_link,
    }


def _bundle_to_dict(bundle: OkfBundle) -> dict[str, Any]:
    return {
        "bundleId": bundle.bundle_id,
        "publicDirectoryId": bundle.public_directory_id,
        "title": bundle.title,
        "description": bundle.description,
        "okfVersion": bundle.okf_version,
        "status": bundle.status,
        "nodeCount": bundle.node_count,
        "edgeCount": bundle.edge_count,
    }


def _edge_to_dict(edge: OkfEdge) -> dict[str, Any]:
    return {
        "srcNodeId": edge.src_node_id,
        "dstNodeId": edge.dst_node_id,
        "dstRelPath": edge.dst_rel_path,
        "linkText": edge.link_text,
        "linkKind": edge.link_kind,
        "dstExists": edge.dst_exists,
    }


def _type_count_to_dict(tc: TypeCount) -> dict[str, Any]:
    return {"type": tc.type, "count": tc.count}


def _neighbors_to_dict(result: NeighborsResult) -> dict[str, Any]:
    return {
        "nodes": [_node_to_dict(n) for n in result.nodes],
        "edges": [_edge_to_dict(e) for e in result.edges],
    }


def _path_to_dict(result: ShortestPathResult) -> dict[str, Any]:
    return {
        "found": result.found,
        "path": [_node_to_dict(n) for n in result.path],
    }


def _subgraph_to_dict(result: SubgraphResult) -> dict[str, Any]:
    return {
        "nodes": [_node_to_dict(n) for n in result.nodes],
        "edges": [_edge_to_dict(e) for e in result.edges],
    }


def _stats_to_dict(stats: BundleStats) -> dict[str, Any]:
    return {
        "nodeCount": stats.node_count,
        "edgeTotal": stats.edge_total,
        "edgeLive": stats.edge_live,
        "edgeBroken": stats.edge_broken,
        "types": [_type_count_to_dict(t) for t in stats.types],
    }


# ----------------------------------------------------------------------
# Tool functions (docstrings are read by the LLM)
# ----------------------------------------------------------------------
def list_bundles() -> dict[str, Any]:
    """List every OKF bundle the caller can read.

    Use this first when you don't know which ``bundleId`` to query. The
    bot is typically configured against one bundle via ``KB_BOT_BUNDLE_ID``
    so this tool is mainly for discovery / sanity-check.

    Returns:
        ``{"ok": True, "data": {"bundles": [{"bundleId", "title",
        "nodeCount", "edgeCount", ...}, ...]}}`` or error.
    """
    try:
        bundles = get_client().list_bundles()
    except AgentDiskError as exc:
        return _err_payload(exc)
    except RuntimeError as exc:
        return _runtime_err(exc)
    return _ok({"bundles": [_bundle_to_dict(b) for b in bundles]})


def get_bundle(bundle_id: int | None = None) -> dict[str, Any]:
    """Get metadata for one OKF bundle.

    Args:
        bundle_id: OKF bundle id. If omitted, falls back to
            ``KB_BOT_BUNDLE_ID`` from the env (the common case — the bot
            is configured against one bundle).

    Returns:
        ``{"ok": True, "data": {"bundleId", "title", "description",
        "nodeCount", "edgeCount", "status", ...}}`` or error.
    """
    try:
        bid = bundle_id if bundle_id is not None else default_bundle_id()
        bundle = get_client().get_bundle(bid)
    except AgentDiskError as exc:
        return _err_payload(exc)
    except RuntimeError as exc:
        return _runtime_err(exc)
    return _ok(_bundle_to_dict(bundle))


def bundle_stats(bundle_id: int | None = None) -> dict[str, Any]:
    """Get graph statistics for an OKF bundle.

    Use this to assess coverage before answering — e.g. "is there any
    node of type=law in this bundle?" or "are there many broken links
    hinting at stale data?".

    Args:
        bundle_id: OKF bundle id. Defaults to ``KB_BOT_BUNDLE_ID``.

    Returns:
        ``{"ok": True, "data": {"nodeCount", "edgeTotal", "edgeLive",
        "edgeBroken", "types": [{"type", "count"}, ...]}}`` or error.
    """
    try:
        bid = bundle_id if bundle_id is not None else default_bundle_id()
        stats = get_client().bundle_stats(bid)
    except AgentDiskError as exc:
        return _err_payload(exc)
    except RuntimeError as exc:
        return _runtime_err(exc)
    return _ok(_stats_to_dict(stats))


def search_knowledge(
    query: str,
    type_filter: str = "",
    limit: int = 10,
    bundle_id: int | None = None,
) -> dict[str, Any]:
    """Full-text search across OKF nodes — the primary Q&A entry point.

    Always try this FIRST for a business question. If results look thin,
    expand via :func:`neighbors` / :func:`reachable` on a promising node.

    Args:
        query: Natural-language or keyword query.中文 OK. Matches against
            node titles, descriptions, tags, and indexed body content.
        type_filter: Optional node-type filter (e.g. ``"law"``,
            ``"regulation"``, ``"concept"``). Empty = no filter.
        limit: Max results. Default 10; raise for broad queries, lower
            for narrow ones.
        bundle_id: OKF bundle id. Defaults to ``KB_BOT_BUNDLE_ID``.

    Returns:
        ``{"ok": True, "data": {"nodes": [{nodeId, relPath, type, title,
        description, tags, hasBrokenLink}, ...], "nextCursor": int}}``
        or error. ``nextCursor=0`` means no more pages.
    """
    try:
        bid = bundle_id if bundle_id is not None else default_bundle_id()
        page = get_client().search_okf(
            query,
            bundle_id=bid,
            type_filter=type_filter,
            limit=limit,
        )
    except AgentDiskError as exc:
        return _err_payload(exc)
    except RuntimeError as exc:
        return _runtime_err(exc)
    return _ok(
        {
            "nodes": [_node_to_dict(n) for n in page.nodes],
            "nextCursor": page.next_cursor,
        }
    )


def list_nodes(
    bundle_id: int | None = None,
    type_filter: str = "",
    tag: str = "",
) -> dict[str, Any]:
    """List nodes in a bundle, optionally filtered by type or tag.

    Use this for browsing ("what's in this KB?") rather than searching
    — for content lookup, :func:`search_knowledge` is faster and
    rank-ordered.

    Args:
        bundle_id: OKF bundle id. Defaults to ``KB_BOT_BUNDLE_ID``.
        type_filter: Optional node-type filter (exact match).
        tag: Optional tag filter (exact match, single tag).

    Returns:
        ``{"ok": True, "data": {"nodes": [{nodeId, relPath, ...}]}}``
        or error.
    """
    try:
        bid = bundle_id if bundle_id is not None else default_bundle_id()
        nodes = get_client().list_okf_nodes(bid, type_filter=type_filter, tag=tag)
    except AgentDiskError as exc:
        return _err_payload(exc)
    except RuntimeError as exc:
        return _runtime_err(exc)
    return _ok({"nodes": [_node_to_dict(n) for n in nodes]})


def neighbors(
    node_id: int,
    direction: str = "both",
    type_filter: str = "",
) -> dict[str, Any]:
    """Get the 1-hop neighborhood of a node — use to expand a seed result.

    After :func:`search_knowledge` returns a promising node, call this to
    see what's directly connected. The ``linkKind`` on each edge tells
    you the relationship type (citation / version-of / implements / etc.).

    Args:
        node_id: The seed node id (from search/list output).
        direction: ``"out"`` (what this node points at), ``"in"`` (what
            points at this node), or ``"both"`` (default — most useful
            for Q&A since you usually want all adjacent context).
        type_filter: Optional node-type filter on the returned neighbors.

    Returns:
        ``{"ok": True, "data": {"nodes": [...], "edges": [{srcNodeId,
        dstNodeId, linkText, linkKind, dstExists}, ...]}}`` or error.
    """
    try:
        result = get_client().neighbors(node_id, direction=direction, type_filter=type_filter)
    except AgentDiskError as exc:
        return _err_payload(exc)
    except RuntimeError as exc:
        return _runtime_err(exc)
    return _ok(_neighbors_to_dict(result))


def reachable(
    node_id: int,
    depth: int,
    types: list[str] | None = None,
    max_nodes: int = 50,
    direction: str = "both",
) -> dict[str, Any]:
    """Get all nodes reachable from ``node_id`` within ``depth`` hops.

    Use this when :func:`neighbors` is too shallow (e.g. "what regs
    derive from this law, transitively?"). ``depth=2`` or ``3`` is
    usually enough; deeper gets noisy fast.

    Args:
        node_id: Seed node id.
        depth: Max hops (1 = same as neighbors, 2-3 typical, >5 rarely
            useful).
        types: Optional list of node types to keep (e.g. ``["law",
            "regulation"]`` drops noise like ``notice`` / ``toc``).
            ``None`` = no filter.
        max_nodes: Cap on returned nodes. Default 50; raise for broad
            traversals.
        direction: ``"out"`` / ``"in"`` / ``"both"`` (default).

    Returns:
        ``{"ok": True, "data": {"nodes": [...]}}`` or error. Edges are
        NOT returned — use :func:`neighbors` or :func:`subgraph` if you
        need the link structure.
    """
    try:
        nodes = get_client().reachable(
            node_id,
            depth,
            types=types,
            max_nodes=max_nodes,
            direction=direction,
        )
    except AgentDiskError as exc:
        return _err_payload(exc)
    except RuntimeError as exc:
        return _runtime_err(exc)
    return _ok({"nodes": [_node_to_dict(n) for n in nodes]})


def shortest_path(
    src_node_id: int,
    dst_node_id: int,
    max_depth: int = 0,
) -> dict[str, Any]:
    """Find the shortest graph path between two OKF nodes.

    Use this for relational questions like "how does A depend on B?" or
    "is there a citation chain from X to Y?". If ``found=False``, there's
    no path within ``max_depth`` — report that as "no relation in KB"
    rather than guessing.

    Args:
        src_node_id: Source node id.
        dst_node_id: Destination node id.
        max_depth: Search depth cap. ``0`` = server default (usually 5).
            Raise if you suspect a long indirect chain.

    Returns:
        ``{"ok": True, "data": {"found": bool, "path": [{nodeId, relPath,
        title, ...}, ...]}}`` or error. ``path[0]`` is ``src``, ``path[-1]``
        is ``dst``.
    """
    try:
        result = get_client().shortest_path(
            src_node_id,
            dst_node_id,
            max_depth=max_depth,
        )
    except AgentDiskError as exc:
        return _err_payload(exc)
    except RuntimeError as exc:
        return _runtime_err(exc)
    return _ok(_path_to_dict(result))


def subgraph(
    bundle_id: int | None = None,
    types: list[str] | None = None,
    max_nodes: int = 50,
) -> dict[str, Any]:
    """Extract a subgraph from a bundle — nodes plus the edges among them.

    Use this when you need the actual link structure, not just node lists.
    Typical use: "show me all laws and regulations and how they reference
    each other".

    Args:
        bundle_id: OKF bundle id. Defaults to ``KB_BOT_BUNDLE_ID``.
        types: Optional node-type whitelist (e.g. ``["law", "regulation"]``).
            ``None`` = all types.
        max_nodes: Cap. Default 50; the result stays LLM-digestible.

    Returns:
        ``{"ok": True, "data": {"nodes": [...], "edges": [...]}}``
        or error.
    """
    try:
        bid = bundle_id if bundle_id is not None else default_bundle_id()
        result = get_client().subgraph(bid, types=types, max_nodes=max_nodes)
    except AgentDiskError as exc:
        return _err_payload(exc)
    except RuntimeError as exc:
        return _runtime_err(exc)
    return _ok(_subgraph_to_dict(result))


def read_node_content(rel_path: str) -> dict[str, Any]:
    """Fetch the full markdown body of an OKF node.

    The other read tools (search / list / neighbors) return
    **frontmatter only** — :class:`OkfNode` carries title / tags / type
    but not the body. To quote chapter and verse from a node, call this
    with the node's ``relPath`` (e.g. ``"机构监管/反洗钱法.md"``).

    Args:
        rel_path: Bundle-relative path as returned by other tools.
            Leading slashes are stripped. NO ``..`` or absolute paths.

    Returns:
        ``{"ok": True, "data": {"relPath", "content", "size"}}`` where
        ``content`` is the raw markdown (UTF-8, including frontmatter).
        Error payload otherwise.
    """
    try:
        body = read_node_body(rel_path)
    except AgentDiskError as exc:
        return _err_payload(exc)
    except RuntimeError as exc:
        return _runtime_err(exc)
    return _ok(
        {
            "relPath": rel_path,
            "content": body,
            "size": len(body.encode("utf-8")),
        }
    )


# ----------------------------------------------------------------------
# FunctionTool singletons
# ----------------------------------------------------------------------
list_bundles_tool = FunctionTool(func=list_bundles)
get_bundle_tool = FunctionTool(func=get_bundle)
bundle_stats_tool = FunctionTool(func=bundle_stats)
search_knowledge_tool = FunctionTool(func=search_knowledge)
list_nodes_tool = FunctionTool(func=list_nodes)
neighbors_tool = FunctionTool(func=neighbors)
reachable_tool = FunctionTool(func=reachable)
shortest_path_tool = FunctionTool(func=shortest_path)
subgraph_tool = FunctionTool(func=subgraph)
read_node_content_tool = FunctionTool(func=read_node_content)
