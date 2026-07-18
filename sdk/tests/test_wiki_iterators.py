"""Unit tests for the OKF pagination iterators (no live server).

Exercises iter_search / iter_broken_links cursor-following without HTTP by
monkeypatching the underlying page methods to return canned pages.
"""

from __future__ import annotations

from types import SimpleNamespace
from typing import Any

import httpx

from agentdisk.api.wiki import _AsyncWikiAPI, _WikiAPI


def test_iter_search_follows_cursor() -> None:
    api = _WikiAPI(httpx.Client(base_url="http://localhost:9100"), token="t")
    n1, n2, n3 = object(), object(), object()
    pages = [
        SimpleNamespace(nodes=[n1, n2], next_cursor=100),
        SimpleNamespace(nodes=[n3], next_cursor=0),
    ]
    seen_cursors: list[int] = []

    def fake_search(
        query: str,
        bundle_id: int = 0,
        type_filter: str = "",
        limit: int = 0,
        cursor: int = 0,
    ) -> Any:
        seen_cursors.append(cursor)
        return pages.pop(0)

    api.search = fake_search  # type: ignore[method-assign]
    out = list(api.iter_search("x"))
    assert out == [n1, n2, n3]
    # The iterator must thread page 1's next_cursor into page 2's cursor.
    assert seen_cursors == [0, 100]


def test_iter_broken_links_follows_cursor() -> None:
    api = _WikiAPI(httpx.Client(base_url="http://localhost:9100"), token="t")
    l1, l2 = object(), object()
    pages = [
        SimpleNamespace(links=[l1], next_cursor=7),
        SimpleNamespace(links=[l2], next_cursor=0),
    ]

    def fake_list(bundle_id: int, cursor: int = 0, limit: int = 50) -> Any:
        return pages.pop(0)

    api.list_broken_links = fake_list  # type: ignore[method-assign]
    out = list(api.iter_broken_links(1))
    assert out == [l1, l2]


async def test_async_iter_search_follows_cursor() -> None:
    api = _AsyncWikiAPI(httpx.AsyncClient(base_url="http://localhost:9100"), token="t")
    n1, n2 = object(), object()
    pages = [
        SimpleNamespace(nodes=[n1], next_cursor=50),
        SimpleNamespace(nodes=[n2], next_cursor=0),
    ]

    async def fake_search(
        query: str,
        bundle_id: int = 0,
        type_filter: str = "",
        limit: int = 0,
        cursor: int = 0,
    ) -> Any:
        return pages.pop(0)

    api.search = fake_search  # type: ignore[method-assign]
    out = [n async for n in api.iter_search("x")]
    assert out == [n1, n2]
