"""OKF v0.1 wiki models.

These dataclasses mirror the JSON wire shape emitted by the OKF reader +
writer handlers under /v1/disk/okf/* and /v1/disk/public-directories/:id/
files/content. camelCase JSON keys map to snake_case Python fields inside
each ``from_dict`` classmethod, matching the convention used by the rest of
the SDK (see ``models/public_directory.py``).
"""

from __future__ import annotations

from dataclasses import dataclass, field
from typing import Any


@dataclass
class OkfBundle:
    """An OKF bundle registered against a public directory."""

    bundle_id: int
    public_directory_id: int
    okf_version: str
    root_index_file_id: int
    title: str
    description: str
    status: str
    node_count: int
    edge_count: int
    created_at: str
    updated_at: str

    @classmethod
    def from_dict(cls, d: dict) -> OkfBundle:
        return cls(
            bundle_id=d.get("bundleId", 0),
            public_directory_id=d.get("publicDirectoryId", 0),
            okf_version=d.get("okfVersion", ""),
            root_index_file_id=d.get("rootIndexFileId", 0),
            title=d.get("title", ""),
            description=d.get("description", ""),
            status=d.get("status", ""),
            node_count=d.get("nodeCount", 0),
            edge_count=d.get("edgeCount", 0),
            created_at=d.get("createdAt", ""),
            updated_at=d.get("updatedAt", ""),
        )


@dataclass
class OkfNode:
    """A materialized OKF node (one markdown file inside a bundle)."""

    node_id: int
    bundle_id: int
    file_id: int
    rel_path: str
    type: str
    title: str
    description: str
    tags: list[str]
    timestamp: str
    has_broken_link: bool
    extra: dict[str, Any]
    content_hash: str
    created_at: str
    updated_at: str

    @classmethod
    def from_dict(cls, d: dict) -> OkfNode:
        tags = d.get("tags") or []
        extra = d.get("extra") or {}
        return cls(
            node_id=d.get("nodeId", 0),
            bundle_id=d.get("bundleId", 0),
            file_id=d.get("fileId", 0),
            rel_path=d.get("relPath", ""),
            type=d.get("type", ""),
            title=d.get("title", ""),
            description=d.get("description", ""),
            tags=list(tags),
            timestamp=d.get("timestamp", ""),
            has_broken_link=d.get("hasBrokenLink", False),
            extra=dict(extra),
            content_hash=d.get("contentHash", ""),
            created_at=d.get("createdAt", ""),
            updated_at=d.get("updatedAt", ""),
        )


@dataclass
class OkfEdge:
    """A directed edge between two OKF nodes.

    ``dst_exists`` is False for dead links (dst_node_id will be 0).
    """

    edge_id: int
    src_node_id: int
    dst_node_id: int
    dst_rel_path: str
    link_text: str
    src_line: int
    link_kind: str
    dst_exists: bool

    @classmethod
    def from_dict(cls, d: dict) -> OkfEdge:
        return cls(
            edge_id=d.get("edgeId", 0),
            src_node_id=d.get("srcNodeId", 0),
            dst_node_id=d.get("dstNodeId", 0),
            dst_rel_path=d.get("dstRelPath", ""),
            link_text=d.get("linkText", ""),
            src_line=d.get("srcLine", 0),
            link_kind=d.get("linkKind", ""),
            dst_exists=d.get("dstExists", False),
        )


@dataclass
class TypeCount:
    """A single row in the per-bundle or global type rollup."""

    type: str
    count: int

    @classmethod
    def from_dict(cls, d: dict) -> TypeCount:
        return cls(type=d.get("type", ""), count=d.get("count", 0))


@dataclass
class BrokenLink:
    """A dead bundle-relative link discovered during a scan."""

    src_node_id: int
    src_rel_path: str
    dst_rel_path: str
    src_line: int
    link_text: str
    link_kind: str
    reason: str

    @classmethod
    def from_dict(cls, d: dict) -> BrokenLink:
        return cls(
            src_node_id=d.get("srcNodeId", 0),
            src_rel_path=d.get("srcRelPath", ""),
            dst_rel_path=d.get("dstRelPath", ""),
            src_line=d.get("srcLine", 0),
            link_text=d.get("linkText", ""),
            link_kind=d.get("linkKind", ""),
            reason=d.get("reason", ""),
        )


@dataclass
class ScanReport:
    """Result of POST /okf/bundles/:id/scan."""

    scanned_nodes: int
    broken_count: int
    scanned_at: str

    @classmethod
    def from_dict(cls, d: dict) -> ScanReport:
        return cls(
            scanned_nodes=d.get("scannedNodes", 0),
            broken_count=d.get("brokenCount", 0),
            scanned_at=d.get("scannedAt", ""),
        )


@dataclass
class IndexRegenResult:
    """Result of POST /okf/bundles/:id/regenerate-index."""

    index_version: int
    regenerated_at: str

    @classmethod
    def from_dict(cls, d: dict) -> IndexRegenResult:
        return cls(
            index_version=d.get("indexVersion", 0),
            regenerated_at=d.get("regeneratedAt", ""),
        )


@dataclass
class BrokenLinksPage:
    """One page of GET /okf/bundles/:id/broken-links.

    ``next_cursor`` is 0 when the bundle has been fully enumerated.
    """

    links: list[BrokenLink] = field(default_factory=list)
    next_cursor: int = 0

    @classmethod
    def from_dict(cls, d: dict) -> BrokenLinksPage:
        rows = d.get("links") or d.get("brokenLinks") or []
        return cls(
            links=[BrokenLink.from_dict(r) for r in rows],
            next_cursor=d.get("nextCursor", 0),
        )


@dataclass
class SearchPage:
    """One page of POST /okf/search."""

    nodes: list[OkfNode] = field(default_factory=list)
    next_cursor: int = 0

    @classmethod
    def from_dict(cls, d: dict) -> SearchPage:
        rows = d.get("nodes") or []
        return cls(
            nodes=[OkfNode.from_dict(r) for r in rows],
            next_cursor=d.get("nextCursor", 0),
        )


@dataclass
class NeighborsResult:
    """Result of GET /okf/nodes/:id/neighbors."""

    nodes: list[OkfNode] = field(default_factory=list)
    edges: list[OkfEdge] = field(default_factory=list)

    @classmethod
    def from_dict(cls, d: dict) -> NeighborsResult:
        return cls(
            nodes=[OkfNode.from_dict(r) for r in (d.get("nodes") or [])],
            edges=[OkfEdge.from_dict(r) for r in (d.get("edges") or [])],
        )


@dataclass
class ShortestPathResult:
    """Result of POST /okf/paths/shortest.

    ``found`` is False (and ``path`` is empty) when no route exists.
    """

    path: list[OkfNode] = field(default_factory=list)
    found: bool = False

    @classmethod
    def from_dict(cls, d: dict) -> ShortestPathResult:
        return cls(
            path=[OkfNode.from_dict(r) for r in (d.get("path") or [])],
            found=d.get("found", False),
        )


@dataclass
class SubgraphResult:
    """Result of POST /okf/subgraph."""

    nodes: list[OkfNode] = field(default_factory=list)
    edges: list[OkfEdge] = field(default_factory=list)

    @classmethod
    def from_dict(cls, d: dict) -> SubgraphResult:
        return cls(
            nodes=[OkfNode.from_dict(r) for r in (d.get("nodes") or [])],
            edges=[OkfEdge.from_dict(r) for r in (d.get("edges") or [])],
        )


@dataclass
class BundleStats:
    """Per-bundle graph summary returned by GET /okf/bundles/:id/stats."""

    node_count: int
    edge_total: int
    edge_live: int
    edge_broken: int
    types: list[TypeCount] = field(default_factory=list)

    @classmethod
    def from_dict(cls, d: dict) -> BundleStats:
        return cls(
            node_count=d.get("nodeCount", 0),
            edge_total=d.get("edgeTotal", 0),
            edge_live=d.get("edgeLive", 0),
            edge_broken=d.get("edgeBroken", 0),
            types=[TypeCount.from_dict(r) for r in (d.get("types") or [])],
        )
