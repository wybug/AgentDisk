"""Unit tests for the 10 FunctionTools in :mod:`adk_kb_bot.tools`.

Strategy: patch ``adk_kb_bot.tools.get_client`` to return a ``MagicMock``
whose methods return canned dataclasses. Verify each tool:

* Calls the right SDK method with the right args.
* Coerces the SDK's dataclass output into the LLM-facing dict envelope.
* Surfaces ``AgentDiskError`` as ``{ok: False, error: ...}``.
* Falls back to ``KB_BOT_BUNDLE_ID`` when ``bundle_id`` is omitted.
"""

from __future__ import annotations

from unittest.mock import MagicMock, patch

import pytest
from agentdisk import AgentDiskError
from agentdisk.models.wiki import (
    BundleStats,
    NeighborsResult,
    OkfBundle,
    OkfEdge,
    OkfNode,
    SearchPage,
    ShortestPathResult,
    SubgraphResult,
    TypeCount,
)

from adk_kb_bot import tools


# ----------------------------------------------------------------------
# Test fixtures
# ----------------------------------------------------------------------
def _make_bundle(
    *,
    bundle_id: int = 42,
    title: str = "金融监管知识库",
    node_count: int = 96,
    edge_count: int = 250,
) -> OkfBundle:
    return OkfBundle(
        bundle_id=bundle_id,
        public_directory_id=1,
        okf_version="0.1",
        root_index_file_id=10,
        title=title,
        description="test bundle",
        status="active",
        node_count=node_count,
        edge_count=edge_count,
        created_at="2026-01-01T00:00:00Z",
        updated_at="2026-01-02T00:00:00Z",
    )


def _make_node(
    *,
    node_id: int = 100,
    bundle_id: int = 42,
    rel_path: str = "机构监管/反洗钱法.md",
    type_: str = "law",
    title: str = "反洗钱法",
    tags: list[str] | None = None,
) -> OkfNode:
    return OkfNode(
        node_id=node_id,
        bundle_id=bundle_id,
        file_id=500,
        rel_path=rel_path,
        type=type_,
        title=title,
        description="",
        tags=tags or [],
        timestamp="",
        has_broken_link=False,
        extra={},
        content_hash="abc",
        created_at="",
        updated_at="",
    )


@pytest.fixture()
def mock_client(configured_env: None):
    """Patch the SDK singleton with a MagicMock; return the mock."""
    fake = MagicMock()
    patcher = patch.object(tools, "get_client", return_value=fake)
    patcher.start()
    yield fake
    patcher.stop()


# ----------------------------------------------------------------------
# list_bundles
# ----------------------------------------------------------------------
def test_list_bundles_success(mock_client: MagicMock) -> None:
    mock_client.list_bundles.return_value = [_make_bundle(), _make_bundle(bundle_id=43)]
    result = tools.list_bundles()
    assert result["ok"] is True
    bundles = result["data"]["bundles"]
    assert len(bundles) == 2
    assert bundles[0]["bundleId"] == 42
    assert bundles[0]["nodeCount"] == 96
    mock_client.list_bundles.assert_called_once_with()


def test_list_bundles_sdk_error(mock_client: MagicMock) -> None:
    mock_client.list_bundles.side_effect = AgentDiskError(
        code=401, message="bad key", http_status=401
    )
    result = tools.list_bundles()
    assert result["ok"] is False
    assert "bad key" in result["error"]
    assert result["httpStatus"] == 401


def test_list_bundles_config_error(monkeypatch: pytest.MonkeyPatch) -> None:
    """Missing AGENTDISK_BASE_URL → RuntimeError surfaced as error dict."""
    monkeypatch.delenv("AGENTDISK_BASE_URL", raising=False)
    monkeypatch.delenv("AGENTDISK_API_KEY", raising=False)
    # Force a fresh client build so env vars are re-read.
    from adk_kb_bot import kb_client
    kb_client.reset_client_cache()
    result = tools.list_bundles()
    assert result["ok"] is False
    assert "AGENTDISK_BASE_URL" in result["error"]


# ----------------------------------------------------------------------
# get_bundle
# ----------------------------------------------------------------------
def test_get_bundle_default_id(mock_client: MagicMock, configured_env: None) -> None:
    """bundle_id omitted → falls back to KB_BOT_BUNDLE_ID=42."""
    mock_client.get_bundle.return_value = _make_bundle()
    result = tools.get_bundle()
    assert result["ok"] is True
    assert result["data"]["bundleId"] == 42
    mock_client.get_bundle.assert_called_once_with(42)


def test_get_bundle_explicit_id(mock_client: MagicMock) -> None:
    mock_client.get_bundle.return_value = _make_bundle(bundle_id=99)
    result = tools.get_bundle(bundle_id=99)
    assert result["ok"] is True
    assert result["data"]["bundleId"] == 99
    mock_client.get_bundle.assert_called_once_with(99)


def test_get_bundle_no_default_no_arg(
    mock_client: MagicMock, monkeypatch: pytest.MonkeyPatch
) -> None:
    monkeypatch.delenv("KB_BOT_BUNDLE_ID", raising=False)
    result = tools.get_bundle()
    assert result["ok"] is False
    assert "KB_BOT_BUNDLE_ID" in result["error"]
    mock_client.get_bundle.assert_not_called()


# ----------------------------------------------------------------------
# bundle_stats
# ----------------------------------------------------------------------
def test_bundle_stats_success(mock_client: MagicMock) -> None:
    mock_client.bundle_stats.return_value = BundleStats(
        node_count=96,
        edge_total=250,
        edge_live=245,
        edge_broken=5,
        types=[TypeCount(type="law", count=12), TypeCount(type="notice", count=40)],
    )
    result = tools.bundle_stats()
    assert result["ok"] is True
    data = result["data"]
    assert data["nodeCount"] == 96
    assert data["edgeLive"] == 245
    assert data["edgeBroken"] == 5
    assert data["types"] == [
        {"type": "law", "count": 12},
        {"type": "notice", "count": 40},
    ]


# ----------------------------------------------------------------------
# search_knowledge
# ----------------------------------------------------------------------
def test_search_knowledge_success(mock_client: MagicMock) -> None:
    mock_client.search_okf.return_value = SearchPage(
        nodes=[_make_node(node_id=100), _make_node(node_id=101, title="客户身份识别办法")],
        next_cursor=0,
    )
    result = tools.search_knowledge(query="反洗钱")
    assert result["ok"] is True
    data = result["data"]
    assert data["nextCursor"] == 0
    assert len(data["nodes"]) == 2
    assert data["nodes"][0]["nodeId"] == 100
    assert data["nodes"][0]["relPath"] == "机构监管/反洗钱法.md"
    mock_client.search_okf.assert_called_once_with(
        "反洗钱", bundle_id=42, type_filter="", limit=10
    )


def test_search_knowledge_with_filters(mock_client: MagicMock) -> None:
    mock_client.search_okf.return_value = SearchPage(nodes=[], next_cursor=0)
    tools.search_knowledge(query="x", type_filter="law", limit=5, bundle_id=99)
    mock_client.search_okf.assert_called_once_with(
        "x", bundle_id=99, type_filter="law", limit=5
    )


# ----------------------------------------------------------------------
# list_nodes
# ----------------------------------------------------------------------
def test_list_nodes_success(mock_client: MagicMock) -> None:
    mock_client.list_okf_nodes.return_value = [_make_node()]
    result = tools.list_nodes(type_filter="law")
    assert result["ok"] is True
    assert len(result["data"]["nodes"]) == 1
    mock_client.list_okf_nodes.assert_called_once_with(
        42, type_filter="law", tag=""
    )


# ----------------------------------------------------------------------
# neighbors
# ----------------------------------------------------------------------
def test_neighbors_success(mock_client: MagicMock) -> None:
    mock_client.neighbors.return_value = NeighborsResult(
        nodes=[_make_node(node_id=101)],
        edges=[
            OkfEdge(
                edge_id=1,
                src_node_id=100,
                dst_node_id=101,
                dst_rel_path="机构监管/客户身份识别.md",
                link_text="客户身份识别",
                src_line=10,
                link_kind="citation",
                dst_exists=True,
            )
        ],
    )
    result = tools.neighbors(node_id=100, direction="out")
    assert result["ok"] is True
    data = result["data"]
    assert len(data["nodes"]) == 1
    assert data["edges"][0]["linkKind"] == "citation"
    assert data["edges"][0]["dstExists"] is True
    mock_client.neighbors.assert_called_once_with(
        100, direction="out", type_filter=""
    )


# ----------------------------------------------------------------------
# reachable
# ----------------------------------------------------------------------
def test_reachable_success(mock_client: MagicMock) -> None:
    mock_client.reachable.return_value = [
        _make_node(node_id=101),
        _make_node(node_id=102),
    ]
    result = tools.reachable(node_id=100, depth=2, types=["law"])
    assert result["ok"] is True
    data = result["data"]
    assert len(data["nodes"]) == 2
    mock_client.reachable.assert_called_once_with(
        100, 2, types=["law"], max_nodes=50, direction="both"
    )


# ----------------------------------------------------------------------
# shortest_path
# ----------------------------------------------------------------------
def test_shortest_path_found(mock_client: MagicMock) -> None:
    mock_client.shortest_path.return_value = ShortestPathResult(
        path=[_make_node(node_id=100), _make_node(node_id=200)],
        found=True,
    )
    result = tools.shortest_path(src_node_id=100, dst_node_id=200)
    assert result["ok"] is True
    assert result["data"]["found"] is True
    assert len(result["data"]["path"]) == 2


def test_shortest_path_not_found(mock_client: MagicMock) -> None:
    mock_client.shortest_path.return_value = ShortestPathResult(
        path=[], found=False
    )
    result = tools.shortest_path(src_node_id=1, dst_node_id=2)
    assert result["ok"] is True
    assert result["data"]["found"] is False
    assert result["data"]["path"] == []


# ----------------------------------------------------------------------
# subgraph
# ----------------------------------------------------------------------
def test_subgraph_success(mock_client: MagicMock) -> None:
    mock_client.subgraph.return_value = SubgraphResult(
        nodes=[_make_node()],
        edges=[],
    )
    result = tools.subgraph(types=["law"], max_nodes=30)
    assert result["ok"] is True
    assert "nodes" in result["data"]
    assert "edges" in result["data"]
    mock_client.subgraph.assert_called_once_with(
        42, types=["law"], max_nodes=30
    )


# ----------------------------------------------------------------------
# read_node_content
# ----------------------------------------------------------------------
def test_read_node_content_success(
    mock_client: MagicMock, configured_env: None
) -> None:
    """read_node_content wraps read_node_body and shapes the payload."""
    fake_body = "---\ntype: law\n---\n\n# 反洗钱法\n\n正文内容。\n"
    with patch.object(tools, "read_node_body", return_value=fake_body) as mock_body:
        result = tools.read_node_content(rel_path="机构监管/反洗钱法.md")
    assert result["ok"] is True
    data = result["data"]
    assert data["relPath"] == "机构监管/反洗钱法.md"
    assert data["content"] == fake_body
    assert data["size"] == len(fake_body.encode("utf-8"))
    mock_body.assert_called_once_with("机构监管/反洗钱法.md")


def test_read_node_content_sdk_error(
    mock_client: MagicMock, configured_env: None
) -> None:
    with patch.object(
        tools,
        "read_node_body",
        side_effect=AgentDiskError(code=404, message="not found", http_status=404),
    ):
        result = tools.read_node_content(rel_path="missing.md")
    assert result["ok"] is False
    assert "not found" in result["error"]


def test_read_node_content_config_error(
    mock_client: MagicMock, monkeypatch: pytest.MonkeyPatch
) -> None:
    """Missing PD id → RuntimeError surfaces as error dict."""
    monkeypatch.delenv("AGENTDISK_PUBLIC_DIRECTORY_ID", raising=False)
    monkeypatch.delenv("KB_BOT_PUBLIC_DIRECTORY_NAME", raising=False)
    result = tools.read_node_content(rel_path="x.md")
    assert result["ok"] is False
    assert "AGENTDISK_PUBLIC_DIRECTORY_ID" in result["error"]
