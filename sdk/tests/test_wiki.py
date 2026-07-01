"""Integration tests for the OKF wiki module.

Hits a live server (AGENTDISK_URL, default http://localhost:9100) using the
api_key_client fixture from conftest.py — the writer endpoint requires API
Key auth, so we use api_key_client for everything to keep the matrix simple.
Each test writes into its own rel_path so they don't collide on the shared
session bundle.
"""

from __future__ import annotations

from typing import TYPE_CHECKING

import pytest

from agentdisk.exceptions import BadRequestError

if TYPE_CHECKING:
    from collections.abc import Iterator

    from agentdisk import AgentDiskClient, AsyncAgentDiskClient
    from agentdisk.models.wiki import OkfBundle


# --- Session fixtures ---


@pytest.fixture(scope="session")
def test_bundle(api_key_client: AgentDiskClient, test_public_dir: dict) -> Iterator[OkfBundle]:
    """Register the session's public directory as an OKF bundle.

    Writes index.md first (the bundle root), then calls register. Tears down
    by unregistering at session end; the public directory itself is dropped
    by test_public_dir's own teardown.
    """
    pd_id = test_public_dir["id"]
    api_key_client.write_markdown(
        public_dir_id=pd_id,
        rel_path="index.md",
        content='---\nokf_version: "0.1"\ntype: index\ntitle: SDK Test Bundle\n---\n# SDK Test Bundle',
    )
    bundle = api_key_client.register_bundle(public_dir_id=pd_id)
    yield bundle
    try:
        api_key_client.unregister_bundle(bundle.bundle_id)
    except Exception:
        pass


# Helper for unique rel paths per test.
def _path(name: str) -> str:
    return f"{name}.md"


def _node(body: str, type_: str = "concept", title: str = "") -> str:
    title_line = f"title: {title}\n" if title else ""
    return f"---\ntype: {type_}\n{title_line}---\n{body}"


# --- Writer tests ---


class TestWikiWriter:
    """POST /public-directories/:id/files/content via write_markdown."""

    def test_write_markdown_creates_node(
        self, api_key_client: AgentDiskClient, test_public_dir: dict, test_bundle: OkfBundle
    ) -> None:
        node = api_key_client.write_markdown(
            public_dir_id=test_public_dir["id"],
            rel_path=_path("test_write_creates"),
            content=_node("hello world", title="Hello"),
        )
        assert node.node_id > 0
        assert node.type == "concept"
        assert node.title == "Hello"
        assert node.bundle_id == test_bundle.bundle_id

    def test_write_markdown_missing_type_returns_400(
        self, api_key_client: AgentDiskClient, test_public_dir: dict
    ) -> None:
        # No frontmatter at all — server must reject with 400 (missing type).
        with pytest.raises(BadRequestError):
            api_key_client.write_markdown(
                public_dir_id=test_public_dir["id"],
                rel_path=_path("test_write_missing_type"),
                content="# no frontmatter",
            )

    def test_write_markdown_nested_path_autocreates(
        self, api_key_client: AgentDiskClient, test_public_dir: dict, test_bundle: OkfBundle
    ) -> None:
        # A nested rel_path must succeed; the OKF writer auto-creates folders.
        node = api_key_client.write_markdown(
            public_dir_id=test_public_dir["id"],
            rel_path="nested/deep/test_write_nested.md",
            content=_node("nested"),
        )
        assert node.node_id > 0
        assert node.rel_path == "nested/deep/test_write_nested.md"


# --- Bundle lifecycle ---


class TestWikiBundleCRUD:
    """register / list / get / unregister."""

    def test_register_returns_active_bundle(self, test_bundle: OkfBundle, test_public_dir: dict) -> None:
        assert test_bundle.bundle_id > 0
        assert test_bundle.public_directory_id == test_public_dir["id"]
        assert test_bundle.okf_version == "0.1"
        assert test_bundle.status == "active"

    def test_list_bundles_contains_created(self, api_key_client: AgentDiskClient, test_bundle: OkfBundle) -> None:
        bundles = api_key_client.list_bundles()
        ids = [b.bundle_id for b in bundles]
        assert test_bundle.bundle_id in ids

    def test_get_bundle_returns_same(self, api_key_client: AgentDiskClient, test_bundle: OkfBundle) -> None:
        fresh = api_key_client.get_bundle(test_bundle.bundle_id)
        assert fresh.bundle_id == test_bundle.bundle_id
        assert fresh.okf_version == test_bundle.okf_version

    def test_refresh_bundle_preserves_id(self, api_key_client: AgentDiskClient, test_bundle: OkfBundle) -> None:
        refreshed = api_key_client.refresh_bundle(test_bundle.bundle_id)
        assert refreshed.bundle_id == test_bundle.bundle_id


# --- Node reader ---


class TestWikiNodes:
    """list_okf_nodes + aggregate_okf_types."""

    def test_list_nodes_unfiltered(
        self, api_key_client: AgentDiskClient, test_public_dir: dict, test_bundle: OkfBundle
    ) -> None:
        # Write a unique node, then list — must appear.
        api_key_client.write_markdown(
            public_dir_id=test_public_dir["id"],
            rel_path=_path("test_list_nodes_unfiltered"),
            content=_node("body", type_="guide"),
        )
        nodes = api_key_client.list_okf_nodes(test_bundle.bundle_id)
        paths = [n.rel_path for n in nodes]
        assert "test_list_nodes_unfiltered.md" in paths

    def test_list_nodes_filtered_by_type(
        self, api_key_client: AgentDiskClient, test_public_dir: dict, test_bundle: OkfBundle
    ) -> None:
        api_key_client.write_markdown(
            public_dir_id=test_public_dir["id"],
            rel_path=_path("test_list_nodes_by_type"),
            content=_node("body", type_="rare-type"),
        )
        nodes = api_key_client.list_okf_nodes(test_bundle.bundle_id, type_filter="rare-type")
        assert all(n.type == "rare-type" for n in nodes)
        assert any(n.rel_path == "test_list_nodes_by_type.md" for n in nodes)

    def test_aggregate_types_includes_written(self, api_key_client: AgentDiskClient, test_public_dir: dict) -> None:
        # index.md has type=index; the rollup should include it.
        api_key_client.write_markdown(
            public_dir_id=test_public_dir["id"],
            rel_path=_path("test_aggregate"),
            content=_node("body", type_="aggregate-marker"),
        )
        counts = api_key_client.aggregate_okf_types()
        types = {c.type for c in counts}
        assert "aggregate-marker" in types


# --- Search ---


class TestWikiSearch:
    """POST /okf/search."""

    def test_search_finds_title(
        self, api_key_client: AgentDiskClient, test_public_dir: dict, test_bundle: OkfBundle
    ) -> None:
        api_key_client.write_markdown(
            public_dir_id=test_public_dir["id"],
            rel_path=_path("test_search_title"),
            content=_node("body", title="UniqueGrepMarker"),
        )
        page = api_key_client.search_okf("UniqueGrepMarker", bundle_id=test_bundle.bundle_id)
        assert any(n.title == "UniqueGrepMarker" for n in page.nodes)

    def test_search_no_match_returns_empty(self, api_key_client: AgentDiskClient, test_bundle: OkfBundle) -> None:
        page = api_key_client.search_okf(
            "ThisStringDoesNotExistAnywhere12345",
            bundle_id=test_bundle.bundle_id,
        )
        assert page.nodes == []


# --- Graph queries ---


class TestWikiGraph:
    """neighbors / shortest_path / subgraph / stats."""

    def test_neighbors_out(
        self, api_key_client: AgentDiskClient, test_public_dir: dict, test_bundle: OkfBundle
    ) -> None:
        # Write target first, then source. Edge materialization runs on the
        # source's write and only resolves the target if it already exists —
        # writing source first would leave a broken edge that never auto-heals.
        api_key_client.write_markdown(
            public_dir_id=test_public_dir["id"],
            rel_path=_path("graph_b"),
            content=_node("target"),
        )
        api_key_client.write_markdown(
            public_dir_id=test_public_dir["id"],
            rel_path=_path("graph_a"),
            content=_node("link to [b](./graph_b.md)"),
        )
        a_nodes = api_key_client.list_okf_nodes(test_bundle.bundle_id, type_filter="concept")
        a_node = next(n for n in a_nodes if n.rel_path == "graph_a.md")

        result = api_key_client.neighbors(a_node.node_id, direction="out")
        # The neighbor's title comes from frontmatter; since graph_b has no
        # title, it falls back to the file basename. Just assert non-empty.
        assert len(result.nodes) >= 1
        # Edges should also be present.
        assert any(e.src_node_id == a_node.node_id for e in result.edges)

    def test_shortest_path_direct_link(
        self, api_key_client: AgentDiskClient, test_public_dir: dict, test_bundle: OkfBundle
    ) -> None:
        # Write dst first so the src→dst edge materializes as live on src's write.
        api_key_client.write_markdown(
            public_dir_id=test_public_dir["id"],
            rel_path=_path("sp_dst"),
            content=_node("target"),
        )
        api_key_client.write_markdown(
            public_dir_id=test_public_dir["id"],
            rel_path=_path("sp_src"),
            content=_node("links to [dst](./sp_dst.md)"),
        )
        nodes = api_key_client.list_okf_nodes(test_bundle.bundle_id, type_filter="concept")
        src = next(n for n in nodes if n.rel_path == "sp_src.md")
        dst = next(n for n in nodes if n.rel_path == "sp_dst.md")

        result = api_key_client.shortest_path(src.node_id, dst.node_id)
        assert result.found is True
        assert len(result.path) == 2
        assert result.path[0].node_id == src.node_id
        assert result.path[1].node_id == dst.node_id

    def test_subgraph_returns_nodes_and_edges(self, api_key_client: AgentDiskClient, test_bundle: OkfBundle) -> None:
        result = api_key_client.subgraph(test_bundle.bundle_id)
        # Bundle has at least the index.md node from the session fixture.
        assert len(result.nodes) >= 1
        # Edges may be empty if no links; just check the field exists.
        assert isinstance(result.edges, list)

    def test_stats_returns_counts(self, api_key_client: AgentDiskClient, test_bundle: OkfBundle) -> None:
        s = api_key_client.bundle_stats(test_bundle.bundle_id)
        assert s.node_count >= 1
        # edge_total = live + broken; both are non-negative.
        assert s.edge_total == s.edge_live + s.edge_broken
        assert isinstance(s.types, list)


# --- Maintenance (P2) ---


class TestWikiMaintenance:
    """scan_bundle / list_broken_links / regenerate_index."""

    def test_scan_bundle_returns_report(self, api_key_client: AgentDiskClient, test_bundle: OkfBundle) -> None:
        report = api_key_client.scan_bundle(test_bundle.bundle_id)
        assert report.scanned_nodes >= 1
        assert report.scanned_at  # ISO timestamp
        assert report.broken_count >= 0

    def test_list_broken_links_after_dead_link(
        self, api_key_client: AgentDiskClient, test_public_dir: dict, test_bundle: OkfBundle
    ) -> None:
        # Write a node pointing to a non-existent target → dead link.
        api_key_client.write_markdown(
            public_dir_id=test_public_dir["id"],
            rel_path=_path("dead_link_src"),
            content=_node("links to [ghost](./does_not_exist.md)"),
        )
        # Trigger a scan so broken links are computed.
        api_key_client.scan_bundle(test_bundle.bundle_id)

        page = api_key_client.list_broken_links(test_bundle.bundle_id)
        assert isinstance(page.links, list)
        # At least the dead link we just wrote should be present.
        assert any(link.dst_rel_path == "does_not_exist.md" for link in page.links)

    def test_regenerate_index(self, api_key_client: AgentDiskClient, test_bundle: OkfBundle) -> None:
        result = api_key_client.regenerate_index(test_bundle.bundle_id)
        assert result.index_version >= 1
        assert result.regenerated_at


# --- Async smoke ---


class TestWikiAsync:
    """Async mirror smoke test — verifies the _AsyncWikiAPI path works."""

    async def test_async_list_bundles(self, async_client: AsyncAgentDiskClient, test_bundle: OkfBundle) -> None:
        bundles = await async_client.list_bundles()
        ids = [b.bundle_id for b in bundles]
        assert test_bundle.bundle_id in ids

    async def test_async_write_markdown(
        self,
        async_api_key_client: AsyncAgentDiskClient,
        test_public_dir: dict,
        test_bundle: OkfBundle,
    ) -> None:
        node = await async_api_key_client.write_markdown(
            public_dir_id=test_public_dir["id"],
            rel_path=_path("async_write"),
            content=_node("async body"),
        )
        assert node.node_id > 0
        assert node.bundle_id == test_bundle.bundle_id
