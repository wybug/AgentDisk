"""Folder API."""

from __future__ import annotations

from typing import List

from ..models.folder import DiskFolder
from .base import AsyncBaseAPI, BaseAPI


class FolderAPI(BaseAPI):

    def create(self, folder_name: str, parent_id: int = 0) -> DiskFolder:
        data = self._request(
            "POST", "/folders", json={"folderName": folder_name, "parentId": parent_id}
        )
        return DiskFolder.from_dict(data)

    def list(self, parent_id: int = 0) -> List[DiskFolder]:
        data = self._request("GET", "/folders", params={"parentId": parent_id})
        return [DiskFolder.from_dict(d) for d in (data or [])]

    def get(self, id: int) -> DiskFolder:
        data = self._request("GET", f"/folders/{id}")
        return DiskFolder.from_dict(data)

    def ancestors(self, id: int) -> List[DiskFolder]:
        data = self._request("GET", f"/folders/{id}/ancestors")
        return [DiskFolder.from_dict(d) for d in (data or [])]

    def rename(self, id: int, folder_name: str) -> DiskFolder:
        data = self._request("PUT", f"/folders/{id}", json={"folderName": folder_name})
        return DiskFolder.from_dict(data)

    def delete(self, id: int) -> None:
        self._request("DELETE", f"/folders/{id}")


class AsyncFolderAPI(AsyncBaseAPI):

    async def create(self, folder_name: str, parent_id: int = 0) -> DiskFolder:
        data = await self._request(
            "POST", "/folders", json={"folderName": folder_name, "parentId": parent_id}
        )
        return DiskFolder.from_dict(data)

    async def list(self, parent_id: int = 0) -> List[DiskFolder]:
        data = await self._request("GET", "/folders", params={"parentId": parent_id})
        return [DiskFolder.from_dict(d) for d in (data or [])]

    async def get(self, id: int) -> DiskFolder:
        data = await self._request("GET", f"/folders/{id}")
        return DiskFolder.from_dict(data)

    async def ancestors(self, id: int) -> List[DiskFolder]:
        data = await self._request("GET", f"/folders/{id}/ancestors")
        return [DiskFolder.from_dict(d) for d in (data or [])]

    async def rename(self, id: int, folder_name: str) -> DiskFolder:
        data = await self._request("PUT", f"/folders/{id}", json={"folderName": folder_name})
        return DiskFolder.from_dict(data)

    async def delete(self, id: int) -> None:
        await self._request("DELETE", f"/folders/{id}")
