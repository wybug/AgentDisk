"""Internal path resolver - converts path strings to resource IDs."""

from __future__ import annotations

import threading
import time
from typing import TYPE_CHECKING

from .exceptions import NotFoundError

if TYPE_CHECKING:
    from .api.file import _AsyncFileAPI, _FileAPI
    from .api.folder import _AsyncFolderAPI, _FolderAPI
    from .models.file import DiskFile
    from .models.folder import DiskFolder


class PathCache:
    """TTL-based LRU cache for path → folder_id mappings."""

    def __init__(self, ttl: float = 60.0, max_size: int = 512) -> None:
        self._ttl = ttl
        self._max_size = max_size
        self._store: dict[str, tuple[int, float]] = {}
        self._lock = threading.Lock()

    def get(self, path: str) -> int | None:
        normalized = _normalize(path)
        with self._lock:
            entry = self._store.get(normalized)
            if entry is None:
                return None
            folder_id, ts = entry
            if time.monotonic() - ts > self._ttl:
                del self._store[normalized]
                return None
            return folder_id

    def put(self, path: str, folder_id: int) -> None:
        normalized = _normalize(path)
        if not normalized:
            return
        with self._lock:
            self._store[normalized] = (folder_id, time.monotonic())
            self._evict()

    def invalidate(self, path: str) -> None:
        normalized = _normalize(path)
        with self._lock:
            keys_to_remove = [k for k in self._store if k == normalized or k.startswith(normalized + "/")]
            for k in keys_to_remove:
                del self._store[k]

    def clear(self) -> None:
        with self._lock:
            self._store.clear()

    def _evict(self) -> None:
        now = time.monotonic()
        expired = [k for k, (_, ts) in self._store.items() if now - ts > self._ttl]
        for k in expired:
            del self._store[k]
        if len(self._store) > self._max_size:
            sorted_keys = sorted(self._store, key=lambda k: self._store[k][1])
            for k in sorted_keys[: len(self._store) - self._max_size]:
                del self._store[k]


def _normalize(path: str) -> str:
    return path.strip("/")


class _PathResolver:
    """Sync path resolver - delegates to internal ID-based APIs."""

    def __init__(
        self,
        folders: _FolderAPI,
        files: _FileAPI,
        cache_ttl: float = 60.0,
    ) -> None:
        self._folders = folders
        self._files = files
        self._cache = PathCache(ttl=cache_ttl)

    def resolve_folder_id(self, path: str) -> int:
        if not _normalize(path):
            return 0
        cached = self._cache.get(path)
        if cached is not None:
            return cached
        folder = self._resolve_folder(path)
        return folder.id

    def resolve_folder(self, path: str) -> DiskFolder:
        if not _normalize(path):
            raise NotFoundError(code=404, message="Root path does not represent a folder object")
        return self._resolve_folder(path)

    def resolve_file(self, path: str) -> DiskFile:
        normalized = _normalize(path)
        if not normalized:
            raise NotFoundError(code=404, message="Empty path")
        segments = normalized.split("/")
        folder_segments = segments[:-1]
        file_name = segments[-1]
        folder_id = self.resolve_folder_id("/".join(folder_segments))
        files = self._files.list(folder_id)
        for f in files:
            if f.file_name == file_name:
                return f
        raise NotFoundError(code=404, message=f"File not found: {path}")

    def mkdir(self, path: str, *, exist_ok: bool = False) -> DiskFolder:
        normalized = _normalize(path)
        if not normalized:
            raise NotFoundError(code=400, message="Cannot create root folder")
        segments = normalized.split("/")
        parent_id = 0
        created_segments: list[str] = []
        for i, segment in enumerate(segments):
            created_segments.append(segment)
            children = self._folders.list(parent_id)
            found: DiskFolder | None = None
            for child in children:
                if child.folder_name == segment:
                    found = child
                    break
            if found is not None:
                parent_id = found.id
                self._cache.put("/".join(created_segments), found.id)
            else:
                if i < len(segments) - 1 or exist_ok:
                    folder = self._folders.create(segment, parent_id)
                    self._cache.put("/".join(created_segments), folder.id)
                    parent_id = folder.id
                else:
                    folder = self._folders.create(segment, parent_id)
                    self._cache.put("/".join(created_segments), folder.id)
                    return folder
        if not exist_ok:
            raise NotFoundError(code=409, message=f"Folder already exists: {path}")
        return self._folders.get(parent_id)

    def invalidate_cache(self, path: str = "") -> None:
        if path:
            self._cache.invalidate(path)
        else:
            self._cache.clear()

    def clear_cache(self) -> None:
        self._cache.clear()

    def _resolve_folder(self, path: str) -> DiskFolder:
        normalized = _normalize(path)
        if not normalized:
            raise NotFoundError(code=404, message="Root path does not represent a folder object")
        segments = normalized.split("/")
        parent_id = 0
        folder: DiskFolder | None = None
        built_segments: list[str] = []
        for segment in segments:
            built_segments.append(segment)
            cached = self._cache.get("/".join(built_segments))
            if cached is not None:
                parent_id = cached
                folder = None
                continue
            children = self._folders.list(parent_id)
            found: DiskFolder | None = None
            for child in children:
                if child.folder_name == segment:
                    found = child
                    break
            if found is None:
                raise NotFoundError(code=404, message=f"Folder not found: {path}")
            self._cache.put("/".join(built_segments), found.id)
            parent_id = found.id
            folder = found
        if folder is None:
            folder = self._folders.get(parent_id)
        return folder


class _AsyncPathResolver:
    """Async path resolver - delegates to internal ID-based APIs."""

    def __init__(
        self,
        folders: _AsyncFolderAPI,
        files: _AsyncFileAPI,
        cache_ttl: float = 60.0,
    ) -> None:
        self._folders = folders
        self._files = files
        self._cache = PathCache(ttl=cache_ttl)

    async def resolve_folder_id(self, path: str) -> int:
        if not _normalize(path):
            return 0
        cached = self._cache.get(path)
        if cached is not None:
            return cached
        folder = await self._resolve_folder(path)
        return folder.id

    async def resolve_folder(self, path: str) -> DiskFolder:
        if not _normalize(path):
            raise NotFoundError(code=404, message="Root path does not represent a folder object")
        return await self._resolve_folder(path)

    async def resolve_file(self, path: str) -> DiskFile:
        normalized = _normalize(path)
        if not normalized:
            raise NotFoundError(code=404, message="Empty path")
        segments = normalized.split("/")
        folder_segments = segments[:-1]
        file_name = segments[-1]
        folder_id = await self.resolve_folder_id("/".join(folder_segments))
        files = await self._files.list(folder_id)
        for f in files:
            if f.file_name == file_name:
                return f
        raise NotFoundError(code=404, message=f"File not found: {path}")

    async def mkdir(self, path: str, *, exist_ok: bool = False) -> DiskFolder:
        normalized = _normalize(path)
        if not normalized:
            raise NotFoundError(code=400, message="Cannot create root folder")
        segments = normalized.split("/")
        parent_id = 0
        created_segments: list[str] = []
        for i, segment in enumerate(segments):
            created_segments.append(segment)
            children = await self._folders.list(parent_id)
            found: DiskFolder | None = None
            for child in children:
                if child.folder_name == segment:
                    found = child
                    break
            if found is not None:
                parent_id = found.id
                self._cache.put("/".join(created_segments), found.id)
            else:
                if i < len(segments) - 1 or exist_ok:
                    folder = await self._folders.create(segment, parent_id)
                    self._cache.put("/".join(created_segments), folder.id)
                    parent_id = folder.id
                else:
                    folder = await self._folders.create(segment, parent_id)
                    self._cache.put("/".join(created_segments), folder.id)
                    return folder
        if not exist_ok:
            raise NotFoundError(code=409, message=f"Folder already exists: {path}")
        return await self._folders.get(parent_id)

    def invalidate_cache(self, path: str = "") -> None:
        if path:
            self._cache.invalidate(path)
        else:
            self._cache.clear()

    def clear_cache(self) -> None:
        self._cache.clear()

    async def _resolve_folder(self, path: str) -> DiskFolder:
        normalized = _normalize(path)
        if not normalized:
            raise NotFoundError(code=404, message="Root path does not represent a folder object")
        segments = normalized.split("/")
        parent_id = 0
        folder: DiskFolder | None = None
        built_segments: list[str] = []
        for segment in segments:
            built_segments.append(segment)
            cached = self._cache.get("/".join(built_segments))
            if cached is not None:
                parent_id = cached
                folder = None
                continue
            children = await self._folders.list(parent_id)
            found: DiskFolder | None = None
            for child in children:
                if child.folder_name == segment:
                    found = child
                    break
            if found is None:
                raise NotFoundError(code=404, message=f"Folder not found: {path}")
            self._cache.put("/".join(built_segments), found.id)
            parent_id = found.id
            folder = found
        if folder is None:
            folder = await self._folders.get(parent_id)
        return folder
