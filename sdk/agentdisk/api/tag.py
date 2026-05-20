"""Tag API."""

from __future__ import annotations

from ..models.file import DiskFile
from .base import AsyncBaseAPI, BaseAPI


class _TagAPI(BaseAPI):
    def bind(self, file_id: int, tag_name: str) -> None:
        self._request("POST", "/tags/bind", json={"fileId": file_id, "tagName": tag_name})

    def unbind(self, file_id: int, tag_name: str) -> None:
        self._request("POST", "/tags/unbind", json={"fileId": file_id, "tagName": tag_name})

    def search(self, tags: list[str]) -> list[DiskFile]:
        data = self._request("GET", "/tags/search", params={"tags": ",".join(tags)})
        return [DiskFile.from_dict(d) for d in (data or [])]


class _AsyncTagAPI(AsyncBaseAPI):
    async def bind(self, file_id: int, tag_name: str) -> None:
        await self._request("POST", "/tags/bind", json={"fileId": file_id, "tagName": tag_name})

    async def unbind(self, file_id: int, tag_name: str) -> None:
        await self._request("POST", "/tags/unbind", json={"fileId": file_id, "tagName": tag_name})

    async def search(self, tags: list[str]) -> list[DiskFile]:
        data = await self._request("GET", "/tags/search", params={"tags": ",".join(tags)})
        return [DiskFile.from_dict(d) for d in (data or [])]
