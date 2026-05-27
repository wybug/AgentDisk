"""Public directory API."""

from __future__ import annotations

from typing import TYPE_CHECKING, Any

from ..models.public_directory import DiskPublicDirectory, from_dict
from .base import AsyncBaseAPI, BaseAPI

if TYPE_CHECKING:
    import builtins


class _PublicDirectoryAPI(BaseAPI):
    def list_visible(self) -> builtins.list[DiskPublicDirectory]:
        data = self._request("GET", "/public-directories")
        if not data:
            return []
        return [from_dict(d) for d in data]

    def get(self, public_dir_id: int) -> DiskPublicDirectory:
        data = self._request("GET", f"/public-directories/{public_dir_id}")
        return from_dict(data)

    def list_sub_folders(self, public_dir_id: int) -> Any:
        return self._request("GET", f"/public-directories/{public_dir_id}/folders")


class _AsyncPublicDirectoryAPI(AsyncBaseAPI):
    async def list_visible(self) -> builtins.list[DiskPublicDirectory]:
        data = await self._request("GET", "/public-directories")
        if not data:
            return []
        return [from_dict(d) for d in data]

    async def get(self, public_dir_id: int) -> DiskPublicDirectory:
        data = await self._request("GET", f"/public-directories/{public_dir_id}")
        return from_dict(data)

    async def list_sub_folders(self, public_dir_id: int) -> Any:
        return await self._request("GET", f"/public-directories/{public_dir_id}/folders")
