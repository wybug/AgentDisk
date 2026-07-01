"""OKF v0.1 wiki API.

Exposes the OKF reader endpoints (``/v1/disk/okf/*``) plus the markdown
writer endpoint (``/v1/disk/public-directories/:id/files/content``) as
Pythonic methods. The reader routes accept either JWT or API Key auth; the
writer route requires an API Key. Both flavors are wired automatically by
``BaseAPI._headers`` based on what the parent client was constructed with.
"""

from __future__ import annotations

from typing import TYPE_CHECKING

from ..models.wiki import (
    BrokenLinksPage,
    BundleStats,
    IndexRegenResult,
    NeighborsResult,
    OkfBundle,
    OkfNode,
    ScanReport,
    SearchPage,
    ShortestPathResult,
    SubgraphResult,
    TypeCount,
)
from .base import AsyncBaseAPI, BaseAPI

if TYPE_CHECKING:
    import builtins


class _WikiAPI(BaseAPI):
    """Sync OKF reader + writer client."""

    # --- Bundle lifecycle ---

    def register_bundle(self, public_dir_id: int) -> OkfBundle:
        """POST /okf/bundles/register — register a public dir as a bundle."""
        data = self._request(
            "POST",
            "/okf/bundles/register",
            json={"publicDirectoryId": public_dir_id},
        )
        return OkfBundle.from_dict(data)

    def list_bundles(self) -> builtins.list[OkfBundle]:
        """GET /okf/bundles — bundles visible to the caller."""
        data = self._request("GET", "/okf/bundles")
        return [OkfBundle.from_dict(d) for d in (data or [])]

    def get_bundle(self, bundle_id: int) -> OkfBundle:
        """GET /okf/bundles/:id — bundle detail + metadata."""
        data = self._request("GET", f"/okf/bundles/{bundle_id}")
        return OkfBundle.from_dict(data)

    def refresh_bundle(self, bundle_id: int) -> OkfBundle:
        """POST /okf/bundles/:id/refresh — re-scan node index from OSS."""
        data = self._request("POST", f"/okf/bundles/{bundle_id}/refresh")
        return OkfBundle.from_dict(data)

    def unregister_bundle(self, bundle_id: int) -> None:
        """DELETE /okf/bundles/:id — drop registration (keeps files)."""
        self._request("DELETE", f"/okf/bundles/{bundle_id}")

    # --- Node reader ---

    def list_nodes(
        self,
        bundle_id: int,
        type_filter: str = "",
        tag: str = "",
    ) -> builtins.list[OkfNode]:
        """GET /okf/bundles/:id/nodes?type=&tag= — filtered node list."""
        data = self._request(
            "GET",
            f"/okf/bundles/{bundle_id}/nodes",
            params={"type": type_filter or None, "tag": tag or None},
        )
        rows = (data or {}).get("nodes") or []
        return [OkfNode.from_dict(r) for r in rows]

    def aggregate_types(self) -> builtins.list[TypeCount]:
        """GET /okf/types — global type→count rollup across visible bundles."""
        data = self._request("GET", "/okf/types")
        return [TypeCount.from_dict(d) for d in (data or [])]

    # --- Search ---

    def search(
        self,
        query: str,
        bundle_id: int = 0,
        type_filter: str = "",
        limit: int = 0,
        cursor: int = 0,
    ) -> SearchPage:
        """POST /okf/search — full-text search over visible bundles."""
        data = self._request(
            "POST",
            "/okf/search",
            json={
                "query": query,
                "bundleId": bundle_id or None,
                "type": type_filter or None,
                "limit": limit or None,
                "cursor": cursor or None,
            },
        )
        return SearchPage.from_dict(data or {})

    # --- Graph queries ---

    def neighbors(
        self,
        node_id: int,
        direction: str = "out",
        type_filter: str = "",
    ) -> NeighborsResult:
        """GET /okf/nodes/:id/neighbors?dir=&type= — 1-hop walk."""
        data = self._request(
            "GET",
            f"/okf/nodes/{node_id}/neighbors",
            params={"dir": direction or None, "type": type_filter or None},
        )
        return NeighborsResult.from_dict(data or {})

    def reachable(
        self,
        node_id: int,
        depth: int,
        types: builtins.list[str] | None = None,
        max_nodes: int = 0,
        direction: str = "",
    ) -> builtins.list[OkfNode]:
        """POST /okf/nodes/:id/reachable — N-hop BFS, returns reachable nodes."""
        data = self._request(
            "POST",
            f"/okf/nodes/{node_id}/reachable",
            json={
                "depth": depth,
                "types": types,
                "maxNodes": max_nodes or None,
                "direction": direction or None,
            },
        )
        rows = (data or {}).get("nodes") or []
        return [OkfNode.from_dict(r) for r in rows]

    def shortest_path(
        self,
        src: int,
        dst: int,
        max_depth: int = 0,
    ) -> ShortestPathResult:
        """POST /okf/paths/shortest — bidirectional BFS path search."""
        data = self._request(
            "POST",
            "/okf/paths/shortest",
            json={"src": src, "dst": dst, "maxDepth": max_depth or None},
        )
        return ShortestPathResult.from_dict(data or {})

    def subgraph(
        self,
        bundle_id: int,
        types: builtins.list[str] | None = None,
        max_nodes: int = 0,
    ) -> SubgraphResult:
        """POST /okf/subgraph — type-filtered bulk node + edge extract."""
        data = self._request(
            "POST",
            "/okf/subgraph",
            json={
                "bundleId": bundle_id,
                "types": types,
                "maxNodes": max_nodes or None,
            },
        )
        return SubgraphResult.from_dict(data or {})

    def stats(self, bundle_id: int) -> BundleStats:
        """GET /okf/bundles/:id/stats — node + edge counts and type rollup."""
        data = self._request("GET", f"/okf/bundles/{bundle_id}/stats")
        return BundleStats.from_dict(data or {})

    # --- Maintenance (P2) ---

    def scan_bundle(self, bundle_id: int) -> ScanReport:
        """POST /okf/bundles/:id/scan — dead-link scan."""
        data = self._request("POST", f"/okf/bundles/{bundle_id}/scan")
        return ScanReport.from_dict(data or {})

    def list_broken_links(
        self,
        bundle_id: int,
        cursor: int = 0,
        limit: int = 50,
    ) -> BrokenLinksPage:
        """GET /okf/bundles/:id/broken-links?cursor=&limit= — cursor paged."""
        data = self._request(
            "GET",
            f"/okf/bundles/{bundle_id}/broken-links",
            params={"cursor": cursor or None, "limit": limit or None},
        )
        return BrokenLinksPage.from_dict(data or {})

    def regenerate_index(self, bundle_id: int) -> IndexRegenResult:
        """POST /okf/bundles/:id/regenerate-index — rebuild root index.md."""
        data = self._request("POST", f"/okf/bundles/{bundle_id}/regenerate-index")
        return IndexRegenResult.from_dict(data or {})

    # --- Writer (API Key only) ---

    def write_markdown(
        self,
        public_dir_id: int,
        rel_path: str,
        content: str,
        content_type: str = "",
    ) -> OkfNode:
        """POST /public-directories/:id/files/content — write a markdown file.

        Triggers OKF materialization (node + edges) for the bundle registered
        against the directory. Requires API Key auth (set on the parent
        client).
        """
        data = self._request(
            "POST",
            f"/public-directories/{public_dir_id}/files/content",
            json={
                "relPath": rel_path,
                "content": content,
                "contentType": content_type or None,
            },
        )
        return OkfNode.from_dict(data)


class _AsyncWikiAPI(AsyncBaseAPI):
    """Async OKF reader + writer client. Mirrors _WikiAPI."""

    # --- Bundle lifecycle ---

    async def register_bundle(self, public_dir_id: int) -> OkfBundle:
        data = await self._request(
            "POST",
            "/okf/bundles/register",
            json={"publicDirectoryId": public_dir_id},
        )
        return OkfBundle.from_dict(data)

    async def list_bundles(self) -> builtins.list[OkfBundle]:
        data = await self._request("GET", "/okf/bundles")
        return [OkfBundle.from_dict(d) for d in (data or [])]

    async def get_bundle(self, bundle_id: int) -> OkfBundle:
        data = await self._request("GET", f"/okf/bundles/{bundle_id}")
        return OkfBundle.from_dict(data)

    async def refresh_bundle(self, bundle_id: int) -> OkfBundle:
        data = await self._request("POST", f"/okf/bundles/{bundle_id}/refresh")
        return OkfBundle.from_dict(data)

    async def unregister_bundle(self, bundle_id: int) -> None:
        await self._request("DELETE", f"/okf/bundles/{bundle_id}")

    # --- Node reader ---

    async def list_nodes(
        self,
        bundle_id: int,
        type_filter: str = "",
        tag: str = "",
    ) -> builtins.list[OkfNode]:
        data = await self._request(
            "GET",
            f"/okf/bundles/{bundle_id}/nodes",
            params={"type": type_filter or None, "tag": tag or None},
        )
        rows = (data or {}).get("nodes") or []
        return [OkfNode.from_dict(r) for r in rows]

    async def aggregate_types(self) -> builtins.list[TypeCount]:
        data = await self._request("GET", "/okf/types")
        return [TypeCount.from_dict(d) for d in (data or [])]

    # --- Search ---

    async def search(
        self,
        query: str,
        bundle_id: int = 0,
        type_filter: str = "",
        limit: int = 0,
        cursor: int = 0,
    ) -> SearchPage:
        data = await self._request(
            "POST",
            "/okf/search",
            json={
                "query": query,
                "bundleId": bundle_id or None,
                "type": type_filter or None,
                "limit": limit or None,
                "cursor": cursor or None,
            },
        )
        return SearchPage.from_dict(data or {})

    # --- Graph queries ---

    async def neighbors(
        self,
        node_id: int,
        direction: str = "out",
        type_filter: str = "",
    ) -> NeighborsResult:
        data = await self._request(
            "GET",
            f"/okf/nodes/{node_id}/neighbors",
            params={"dir": direction or None, "type": type_filter or None},
        )
        return NeighborsResult.from_dict(data or {})

    async def reachable(
        self,
        node_id: int,
        depth: int,
        types: builtins.list[str] | None = None,
        max_nodes: int = 0,
        direction: str = "",
    ) -> builtins.list[OkfNode]:
        data = await self._request(
            "POST",
            f"/okf/nodes/{node_id}/reachable",
            json={
                "depth": depth,
                "types": types,
                "maxNodes": max_nodes or None,
                "direction": direction or None,
            },
        )
        rows = (data or {}).get("nodes") or []
        return [OkfNode.from_dict(r) for r in rows]

    async def shortest_path(
        self,
        src: int,
        dst: int,
        max_depth: int = 0,
    ) -> ShortestPathResult:
        data = await self._request(
            "POST",
            "/okf/paths/shortest",
            json={"src": src, "dst": dst, "maxDepth": max_depth or None},
        )
        return ShortestPathResult.from_dict(data or {})

    async def subgraph(
        self,
        bundle_id: int,
        types: builtins.list[str] | None = None,
        max_nodes: int = 0,
    ) -> SubgraphResult:
        data = await self._request(
            "POST",
            "/okf/subgraph",
            json={
                "bundleId": bundle_id,
                "types": types,
                "maxNodes": max_nodes or None,
            },
        )
        return SubgraphResult.from_dict(data or {})

    async def stats(self, bundle_id: int) -> BundleStats:
        data = await self._request("GET", f"/okf/bundles/{bundle_id}/stats")
        return BundleStats.from_dict(data or {})

    # --- Maintenance (P2) ---

    async def scan_bundle(self, bundle_id: int) -> ScanReport:
        data = await self._request("POST", f"/okf/bundles/{bundle_id}/scan")
        return ScanReport.from_dict(data or {})

    async def list_broken_links(
        self,
        bundle_id: int,
        cursor: int = 0,
        limit: int = 50,
    ) -> BrokenLinksPage:
        data = await self._request(
            "GET",
            f"/okf/bundles/{bundle_id}/broken-links",
            params={"cursor": cursor or None, "limit": limit or None},
        )
        return BrokenLinksPage.from_dict(data or {})

    async def regenerate_index(self, bundle_id: int) -> IndexRegenResult:
        data = await self._request("POST", f"/okf/bundles/{bundle_id}/regenerate-index")
        return IndexRegenResult.from_dict(data or {})

    # --- Writer (API Key only) ---

    async def write_markdown(
        self,
        public_dir_id: int,
        rel_path: str,
        content: str,
        content_type: str = "",
    ) -> OkfNode:
        data = await self._request(
            "POST",
            f"/public-directories/{public_dir_id}/files/content",
            json={
                "relPath": rel_path,
                "content": content,
                "contentType": content_type or None,
            },
        )
        return OkfNode.from_dict(data)
