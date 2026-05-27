"""Share API."""

from __future__ import annotations

from typing import TYPE_CHECKING

from ..models.file import DownloadTokenResponse
from ..models.share import DiskShare
from .base import AsyncBaseAPI, BaseAPI

if TYPE_CHECKING:
    import builtins


class _ShareAPI(BaseAPI):
    def create(
        self,
        resource_id: int,
        res_type: str,
        extract_code: str = "",
        max_visit: int = -1,
        expire_hours: int = 72,
    ) -> DiskShare:
        data = self._request(
            "POST",
            "/shares",
            json={
                "resourceId": resource_id,
                "resType": res_type,
                "extractCode": extract_code,
                "maxVisit": max_visit,
                "expireHours": expire_hours,
            },
        )
        return DiskShare.from_dict(data)

    def list(self) -> builtins.list[DiskShare]:
        data = self._request("GET", "/shares")
        return [DiskShare.from_dict(d) for d in (data or [])]

    def revoke(self, share_id: int) -> None:
        self._request("DELETE", "/shares", json={"shareId": share_id})

    def get_by_code(self, code: str) -> DiskShare:
        data = self._request("GET", f"/share/{code}")
        return DiskShare.from_dict(data)

    def access(self, code: str, extract_code: str = "") -> DiskShare:
        data = self._request("POST", "/share/access", json={"code": code, "extractCode": extract_code})
        return DiskShare.from_dict(data)

    def download(self, code: str, resource_id: int, extract_code: str = "") -> DownloadTokenResponse:
        data = self._request(
            "POST",
            "/share/download",
            json={"code": code, "extractCode": extract_code, "resourceId": resource_id},
        )
        return DownloadTokenResponse.from_dict(data)


class _AsyncShareAPI(AsyncBaseAPI):
    async def create(
        self,
        resource_id: int,
        res_type: str,
        extract_code: str = "",
        max_visit: int = -1,
        expire_hours: int = 72,
    ) -> DiskShare:
        data = await self._request(
            "POST",
            "/shares",
            json={
                "resourceId": resource_id,
                "resType": res_type,
                "extractCode": extract_code,
                "maxVisit": max_visit,
                "expireHours": expire_hours,
            },
        )
        return DiskShare.from_dict(data)

    async def list(self) -> builtins.list[DiskShare]:
        data = await self._request("GET", "/shares")
        return [DiskShare.from_dict(d) for d in (data or [])]

    async def revoke(self, share_id: int) -> None:
        await self._request("DELETE", "/shares", json={"shareId": share_id})

    async def get_by_code(self, code: str) -> DiskShare:
        data = await self._request("GET", f"/share/{code}")
        return DiskShare.from_dict(data)

    async def access(self, code: str, extract_code: str = "") -> DiskShare:
        data = await self._request("POST", "/share/access", json={"code": code, "extractCode": extract_code})
        return DiskShare.from_dict(data)

    async def download(self, code: str, resource_id: int, extract_code: str = "") -> DownloadTokenResponse:
        data = await self._request(
            "POST",
            "/share/download",
            json={"code": code, "extractCode": extract_code, "resourceId": resource_id},
        )
        return DownloadTokenResponse.from_dict(data)
