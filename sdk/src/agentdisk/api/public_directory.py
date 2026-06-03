"""Public directory API."""

from __future__ import annotations

from typing import TYPE_CHECKING

from ..models.file import DiskFile
from ..models.folder import DiskFolder
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

    def list_sub_folders(self, public_dir_id: int) -> builtins.list[DiskFolder]:
        data = self._request("GET", f"/public-directories/{public_dir_id}/folders")
        return [DiskFolder.from_dict(d) for d in (data or [])]

    def list_files(self, public_dir_id: int) -> builtins.list[DiskFile]:
        data = self._request("GET", f"/public-directories/{public_dir_id}/files")
        return [DiskFile.from_dict(d) for d in (data or [])]

    def upload_file(
        self,
        public_dir_id: int,
        filename: str,
        content: bytes,
        content_type: str = "application/octet-stream",
    ) -> DiskFile:
        data = self._request(
            "POST",
            f"/public-directories/{public_dir_id}/files/upload",
            files={"file": (filename, content, content_type)},
        )
        return DiskFile.from_dict(data)

    def delete_file(self, public_dir_id: int, file_id: int) -> None:
        self._request("DELETE", f"/public-directories/{public_dir_id}/files/{file_id}")

    def create_download_token(self, public_dir_id: int, file_id: int) -> dict:
        return self._request(  # type: ignore[no-any-return]
            "POST",
            f"/public-directories/{public_dir_id}/download-token",
            data={"fileId": str(file_id)},
        )

    def create_sub_folder(self, public_dir_id: int, folder_name: str) -> DiskFolder:
        data = self._request(
            "POST",
            f"/public-directories/{public_dir_id}/folders",
            json={"folderName": folder_name},
        )
        return DiskFolder.from_dict(data)

    # --- User authorization ---

    def grant_access(self, public_dir_id: int, user_id: str) -> None:
        self._request(
            "POST",
            f"/public-directories/{public_dir_id}/grants",
            json={"userId": user_id},
        )

    def revoke_access(self, public_dir_id: int, user_id: str) -> None:
        self._request("DELETE", f"/public-directories/{public_dir_id}/grants/{user_id}")

    def list_granted_users(self, public_dir_id: int) -> builtins.list[str]:
        data = self._request("GET", f"/public-directories/{public_dir_id}/grants")
        return data or []


class _AsyncPublicDirectoryAPI(AsyncBaseAPI):
    async def list_visible(self) -> builtins.list[DiskPublicDirectory]:
        data = await self._request("GET", "/public-directories")
        if not data:
            return []
        return [from_dict(d) for d in data]

    async def get(self, public_dir_id: int) -> DiskPublicDirectory:
        data = await self._request("GET", f"/public-directories/{public_dir_id}")
        return from_dict(data)

    async def list_sub_folders(self, public_dir_id: int) -> builtins.list[DiskFolder]:
        data = await self._request("GET", f"/public-directories/{public_dir_id}/folders")
        return [DiskFolder.from_dict(d) for d in (data or [])]

    async def list_files(self, public_dir_id: int) -> builtins.list[DiskFile]:
        data = await self._request("GET", f"/public-directories/{public_dir_id}/files")
        return [DiskFile.from_dict(d) for d in (data or [])]

    async def upload_file(
        self,
        public_dir_id: int,
        filename: str,
        content: bytes,
        content_type: str = "application/octet-stream",
    ) -> DiskFile:
        data = await self._request(
            "POST",
            f"/public-directories/{public_dir_id}/files/upload",
            files={"file": (filename, content, content_type)},
        )
        return DiskFile.from_dict(data)

    async def delete_file(self, public_dir_id: int, file_id: int) -> None:
        await self._request("DELETE", f"/public-directories/{public_dir_id}/files/{file_id}")

    async def create_download_token(self, public_dir_id: int, file_id: int) -> dict:
        return await self._request(  # type: ignore[no-any-return]
            "POST",
            f"/public-directories/{public_dir_id}/download-token",
            data={"fileId": str(file_id)},
        )

    async def create_sub_folder(self, public_dir_id: int, folder_name: str) -> DiskFolder:
        data = await self._request(
            "POST",
            f"/public-directories/{public_dir_id}/folders",
            json={"folderName": folder_name},
        )
        return DiskFolder.from_dict(data)

    async def grant_access(self, public_dir_id: int, user_id: str) -> None:
        await self._request(
            "POST",
            f"/public-directories/{public_dir_id}/grants",
            json={"userId": user_id},
        )

    async def revoke_access(self, public_dir_id: int, user_id: str) -> None:
        await self._request("DELETE", f"/public-directories/{public_dir_id}/grants/{user_id}")

    async def list_granted_users(self, public_dir_id: int) -> builtins.list[str]:
        data = await self._request("GET", f"/public-directories/{public_dir_id}/grants")
        return data or []
