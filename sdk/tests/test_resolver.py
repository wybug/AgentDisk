"""Unit tests for PathCache and _PathResolver."""

from __future__ import annotations

import time
from unittest.mock import MagicMock

from agentdisk._resolver import PathCache, _PathResolver
from agentdisk.exceptions import NotFoundError
from agentdisk.models.file import DiskFile
from agentdisk.models.folder import DiskFolder


def _make_folder(id: int, name: str, parent_id: int = 0) -> DiskFolder:
    return DiskFolder(
        id=id,
        user_id="u1",
        parent_id=parent_id,
        folder_name=name,
        full_path=name,
        sort_order=0,
        is_deleted=False,
        created_at="",
        updated_at="",
    )


def _make_file(id: int, name: str, folder_id: int = 0) -> DiskFile:
    return DiskFile(
        id=id,
        user_id="u1",
        folder_id=folder_id,
        file_name=name,
        file_size=0,
        file_type="txt",
        oss_key="",
        md5="",
        version=1,
        is_deleted=False,
        source_agent="",
        source_agent_group="",
        is_artifact=False,
        tags="",
        created_at="",
        updated_at="",
    )


class TestPathCache:
    def test_put_and_get(self):
        cache = PathCache(ttl=60.0)
        cache.put("docs/reports", 42)
        assert cache.get("docs/reports") == 42

    def test_get_miss(self):
        cache = PathCache(ttl=60.0)
        assert cache.get("nonexistent") is None

    def test_normalize_slash(self):
        cache = PathCache(ttl=60.0)
        cache.put("/docs/reports/", 42)
        assert cache.get("docs/reports") == 42

    def test_ttl_expiry(self):
        cache = PathCache(ttl=0.0)
        cache.put("docs", 1)
        time.sleep(0.01)
        assert cache.get("docs") is None

    def test_invalidate_specific(self):
        cache = PathCache(ttl=60.0)
        cache.put("docs", 1)
        cache.put("docs/reports", 2)
        cache.put("docs/reports/2026", 3)
        cache.put("other", 4)
        cache.invalidate("docs/reports")
        assert cache.get("docs") == 1
        assert cache.get("docs/reports") is None
        assert cache.get("docs/reports/2026") is None
        assert cache.get("other") == 4

    def test_clear(self):
        cache = PathCache(ttl=60.0)
        cache.put("a", 1)
        cache.put("b", 2)
        cache.clear()
        assert cache.get("a") is None
        assert cache.get("b") is None

    def test_max_size_eviction(self):
        cache = PathCache(ttl=60.0, max_size=2)
        cache.put("a", 1)
        cache.put("b", 2)
        cache.put("c", 3)
        assert cache.get("a") is None
        assert cache.get("b") is not None
        assert cache.get("c") is not None

    def test_root_path_not_cached(self):
        cache = PathCache(ttl=60.0)
        cache.put("", 0)
        cache.put("/", 0)
        assert cache.get("") is None
        assert cache.get("/") is None


class TestPathResolver:
    def _setup_resolver(self, folder_tree: dict, file_tree: dict):
        folders_api = MagicMock()
        files_api = MagicMock()

        folder_map: dict[int, DiskFolder] = {}
        file_map: dict[int, list[DiskFile]] = {}

        for fid, (name, pid) in folder_tree.items():
            folder_map[fid] = _make_folder(fid, name, pid)

        def list_folders(parent_id: int = 0):
            return [f for f in folder_map.values() if f.parent_id == parent_id]

        def get_folder(fid: int):
            return folder_map[fid]

        def create_folder(name: str, parent_id: int = 0):
            new_id = max(folder_map.keys(), default=0) + 1
            f = _make_folder(new_id, name, parent_id)
            folder_map[new_id] = f
            return f

        folders_api.list.side_effect = list_folders
        folders_api.get.side_effect = get_folder
        folders_api.create.side_effect = create_folder

        for fid, files in file_tree.items():
            file_map[fid] = [_make_file(i, n, fid) for i, n in files.items()]

        def list_files(folder_id: int):
            return file_map.get(folder_id, [])

        files_api.list.side_effect = list_files

        return _PathResolver(folders_api, files_api)

    def test_resolve_root_id(self):
        r = self._setup_resolver({}, {})
        assert r.resolve_folder_id("/") == 0
        assert r.resolve_folder_id("") == 0

    def test_resolve_folder(self):
        r = self._setup_resolver(
            {1: ("docs", 0), 2: ("reports", 1)},
            {},
        )
        folder = r.resolve_folder("docs/reports")
        assert folder.id == 2
        assert folder.folder_name == "reports"

    def test_resolve_folder_not_found(self):
        r = self._setup_resolver({1: ("docs", 0)}, {})
        try:
            r.resolve_folder("docs/nonexistent")
            raise AssertionError("Should raise NotFoundError")
        except NotFoundError:
            pass

    def test_resolve_file(self):
        r = self._setup_resolver(
            {1: ("docs", 0)},
            {1: {10: "report.md"}},
        )
        f = r.resolve_file("docs/report.md")
        assert f.id == 10
        assert f.file_name == "report.md"

    def test_resolve_file_not_found(self):
        r = self._setup_resolver(
            {1: ("docs", 0)},
            {},
        )
        try:
            r.resolve_file("docs/missing.md")
            raise AssertionError("Should raise NotFoundError")
        except NotFoundError:
            pass

    def test_resolve_file_at_root(self):
        r = self._setup_resolver(
            {},
            {0: {10: "readme.md"}},
        )
        f = r.resolve_file("readme.md")
        assert f.id == 10

    def test_mkdir_creates_all(self):
        r = self._setup_resolver({}, {})
        folder = r.mkdir("a/b/c")
        assert folder.folder_name == "c"

    def test_mkdir_exist_ok(self):
        r = self._setup_resolver({1: ("docs", 0)}, {})
        folder = r.mkdir("docs", exist_ok=True)
        assert folder.id == 1

    def test_cache_populated_on_resolve(self):
        r = self._setup_resolver(
            {1: ("docs", 0), 2: ("reports", 1)},
            {},
        )
        r.resolve_folder("docs/reports")
        assert r._cache.get("docs") == 1
        assert r._cache.get("docs/reports") == 2

    def test_cache_used_on_second_resolve(self):
        r = self._setup_resolver(
            {1: ("docs", 0), 2: ("reports", 1)},
            {},
        )
        r.resolve_folder("docs/reports")
        r._folders.list.reset_mock()
        r.resolve_folder("docs/reports")
        r._folders.list.assert_not_called()

    def test_invalidate_cache(self):
        r = self._setup_resolver(
            {1: ("docs", 0), 2: ("reports", 1)},
            {},
        )
        r.resolve_folder("docs/reports")
        r.invalidate_cache("docs")
        assert r._cache.get("docs") is None
        assert r._cache.get("docs/reports") is None
