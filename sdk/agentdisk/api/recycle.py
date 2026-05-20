"""Recycle bin API."""

from __future__ import annotations

from typing import TYPE_CHECKING

from ..models.recycle import DiskRecycleBin
from .base import AsyncBaseAPI, BaseAPI

if TYPE_CHECKING:
    import builtins


class RecycleAPI(BaseAPI):
    def list(self) -> builtins.list[DiskRecycleBin]:
        data = self._request("GET", "/recycle")
        return [DiskRecycleBin.from_dict(d) for d in (data or [])]

    def restore(self, recycle_id: int) -> None:
        self._request("POST", "/recycle/restore", json={"recycleId": recycle_id})

    def delete_permanent(self, recycle_id: int) -> None:
        self._request("DELETE", "/recycle", json={"recycleId": recycle_id})


class AsyncRecycleAPI(AsyncBaseAPI):
    async def list(self) -> builtins.list[DiskRecycleBin]:
        data = await self._request("GET", "/recycle")
        return [DiskRecycleBin.from_dict(d) for d in (data or [])]

    async def restore(self, recycle_id: int) -> None:
        await self._request("POST", "/recycle/restore", json={"recycleId": recycle_id})

    async def delete_permanent(self, recycle_id: int) -> None:
        await self._request("DELETE", "/recycle", json={"recycleId": recycle_id})
