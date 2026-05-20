"""File API."""

from __future__ import annotations

from pathlib import Path
from typing import TYPE_CHECKING

from ..models.file import (
    DiskFile,
    DownloadByTokenResponse,
    DownloadTokenResponse,
    FileDetailResponse,
)
from .base import AsyncBaseAPI, BaseAPI

if TYPE_CHECKING:
    import builtins


class FileAPI(BaseAPI):
    def upload(
        self,
        file_path: str,
        folder_id: int = 0,
        agent_id: str = "",
    ) -> DiskFile:
        p = Path(file_path)
        with open(p, "rb") as f:
            data = self._request(
                "POST",
                "/files/upload",
                files={"file": (p.name, f)},
                data={"folderId": str(folder_id), **({"agentId": agent_id} if agent_id else {})},
            )
        return DiskFile.from_dict(data)

    def upload_bytes(
        self,
        filename: str,
        content: bytes,
        folder_id: int = 0,
        agent_id: str = "",
        content_type: str = "application/octet-stream",
    ) -> DiskFile:
        data = self._request(
            "POST",
            "/files/upload",
            files={"file": (filename, content, content_type)},
            data={"folderId": str(folder_id), **({"agentId": agent_id} if agent_id else {})},
        )
        return DiskFile.from_dict(data)

    def list(self, folder_id: int) -> builtins.list[DiskFile]:
        data = self._request("GET", "/files", params={"folderId": folder_id})
        return [DiskFile.from_dict(d) for d in (data or [])]

    def get(self, id: int) -> FileDetailResponse:
        data = self._request("GET", f"/files/{id}")
        return FileDetailResponse.from_dict(data)

    def update(self, id: int, file_path: str) -> DiskFile:
        p = Path(file_path)
        with open(p, "rb") as f:
            data = self._request(
                "PUT",
                f"/files/{id}",
                files={"file": (p.name, f)},
            )
        return DiskFile.from_dict(data)

    def update_bytes(
        self,
        id: int,
        filename: str,
        content: bytes,
        content_type: str = "application/octet-stream",
    ) -> DiskFile:
        data = self._request(
            "PUT",
            f"/files/{id}",
            files={"file": (filename, content, content_type)},
        )
        return DiskFile.from_dict(data)

    def delete(self, id: int) -> None:
        self._request("DELETE", f"/files/{id}")

    def create_download_token(self, id: int) -> DownloadTokenResponse:
        data = self._request("POST", f"/files/{id}/download-token")
        return DownloadTokenResponse.from_dict(data)

    def download_by_token(self, token: str) -> DownloadByTokenResponse:
        data = self._request("GET", "/files/download", params={"token": token})
        return DownloadByTokenResponse.from_dict(data)


class AsyncFileAPI(AsyncBaseAPI):
    async def upload(
        self,
        file_path: str,
        folder_id: int = 0,
        agent_id: str = "",
    ) -> DiskFile:
        p = Path(file_path)
        with open(p, "rb") as f:
            data = await self._request(
                "POST",
                "/files/upload",
                files={"file": (p.name, f)},
                data={"folderId": str(folder_id), **({"agentId": agent_id} if agent_id else {})},
            )
        return DiskFile.from_dict(data)

    async def upload_bytes(
        self,
        filename: str,
        content: bytes,
        folder_id: int = 0,
        agent_id: str = "",
        content_type: str = "application/octet-stream",
    ) -> DiskFile:
        data = await self._request(
            "POST",
            "/files/upload",
            files={"file": (filename, content, content_type)},
            data={"folderId": str(folder_id), **({"agentId": agent_id} if agent_id else {})},
        )
        return DiskFile.from_dict(data)

    async def list(self, folder_id: int) -> builtins.list[DiskFile]:
        data = await self._request("GET", "/files", params={"folderId": folder_id})
        return [DiskFile.from_dict(d) for d in (data or [])]

    async def get(self, id: int) -> FileDetailResponse:
        data = await self._request("GET", f"/files/{id}")
        return FileDetailResponse.from_dict(data)

    async def update(self, id: int, file_path: str) -> DiskFile:
        p = Path(file_path)
        with open(p, "rb") as f:
            data = await self._request(
                "PUT",
                f"/files/{id}",
                files={"file": (p.name, f)},
            )
        return DiskFile.from_dict(data)

    async def update_bytes(
        self,
        id: int,
        filename: str,
        content: bytes,
        content_type: str = "application/octet-stream",
    ) -> DiskFile:
        data = await self._request(
            "PUT",
            f"/files/{id}",
            files={"file": (filename, content, content_type)},
        )
        return DiskFile.from_dict(data)

    async def delete(self, id: int) -> None:
        await self._request("DELETE", f"/files/{id}")

    async def create_download_token(self, id: int) -> DownloadTokenResponse:
        data = await self._request("POST", f"/files/{id}/download-token")
        return DownloadTokenResponse.from_dict(data)

    async def download_by_token(self, token: str) -> DownloadByTokenResponse:
        data = await self._request("GET", "/files/download", params={"token": token})
        return DownloadByTokenResponse.from_dict(data)
