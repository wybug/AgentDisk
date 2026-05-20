"""Version API."""

from __future__ import annotations

from typing import TYPE_CHECKING

from ..models.version import DiskFileVersion
from .base import AsyncBaseAPI, BaseAPI

if TYPE_CHECKING:
    import builtins


class VersionAPI(BaseAPI):
    def list(self, file_id: int) -> builtins.list[DiskFileVersion]:
        data = self._request("GET", "/versions", params={"fileId": file_id})
        return [DiskFileVersion.from_dict(d) for d in (data or [])]

    def rollback(self, file_id: int, version: int) -> None:
        self._request("POST", "/versions/rollback", json={"fileId": file_id, "version": version})


class AsyncVersionAPI(AsyncBaseAPI):
    async def list(self, file_id: int) -> builtins.list[DiskFileVersion]:
        data = await self._request("GET", "/versions", params={"fileId": file_id})
        return [DiskFileVersion.from_dict(d) for d in (data or [])]

    async def rollback(self, file_id: int, version: int) -> None:
        await self._request("POST", "/versions/rollback", json={"fileId": file_id, "version": version})
