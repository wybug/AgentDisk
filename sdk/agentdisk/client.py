"""AgentDisk sync client."""

from __future__ import annotations

from typing import TYPE_CHECKING

import httpx

from ._resolver import _PathResolver
from .api import (
    _FileAPI,
    _FolderAPI,
    _PermissionAPI,
    _PreviewAPI,
    _PublicDirectoryAPI,
    _RecycleAPI,
    _ShareAPI,
    _SpaceAPI,
    _TagAPI,
    _VersionAPI,
)

if TYPE_CHECKING:
    import builtins

    from .models.file import DiskFile, DownloadByTokenResponse, FileDetailResponse
    from .models.folder import DiskFolder
    from .models.permission import DiskPermission
    from .models.preview import PreviewResult
    from .models.public_directory import DiskPublicDirectory
    from .models.recycle import DiskRecycleBin
    from .models.share import DiskShare
    from .models.space import UserDisk
    from .models.version import DiskFileVersion


class AgentDiskClient:
    """Synchronous AgentDisk SDK client (path-based public API).

    Usage:
        client = AgentDiskClient(
            base_url="http://localhost:9100",
            token="<jwt-from-gateway>",
        )
        client.create_folder("docs/reports")
        client.upload_file("docs/reports/summary.md", "/local/summary.md")
        client.close()
    """

    def __init__(
        self,
        base_url: str,
        token: str = "",
        api_key: str = "",
        timeout: float = 30.0,
        cache_ttl: float = 60.0,
    ):
        if not token and not api_key:
            raise ValueError("either token or api_key must be provided")
        self._http = httpx.Client(base_url=base_url, timeout=timeout)
        self._token = token
        self._api_key = api_key
        self._folders = _FolderAPI(self._http, token=token, api_key=api_key)
        self._files = _FileAPI(self._http, token=token, api_key=api_key)
        self._permissions = _PermissionAPI(self._http, token=token, api_key=api_key)
        self._versions = _VersionAPI(self._http, token=token, api_key=api_key)
        self._recycle = _RecycleAPI(self._http, token=token, api_key=api_key)
        self._tags = _TagAPI(self._http, token=token, api_key=api_key)
        self._shares = _ShareAPI(self._http, token=token, api_key=api_key)
        self._preview = _PreviewAPI(self._http, token=token, api_key=api_key)
        self._space = _SpaceAPI(self._http, token=token, api_key=api_key)
        self._public_dirs = _PublicDirectoryAPI(self._http, token=token, api_key=api_key)
        self._resolver = _PathResolver(self._folders, self._files, cache_ttl=cache_ttl, public_dirs=self._public_dirs)

    # --- Folder operations ---

    def create_folder(self, path: str, *, exist_ok: bool = False) -> DiskFolder:
        return self._resolver.mkdir(path, exist_ok=exist_ok)

    def list_folders(self, path: str = "/") -> builtins.list[DiskFolder]:
        folder_id = self._resolver.resolve_folder_id(path)
        return self._folders.list(folder_id)

    def get_folder(self, path: str) -> DiskFolder:
        return self._resolver.resolve_folder(path)

    def rename_folder(self, path: str, new_name: str) -> DiskFolder:
        folder = self._resolver.resolve_folder(path)
        result = self._folders.rename(folder.id, new_name)
        self._resolver.invalidate_cache(path)
        return result

    def delete_folder(self, path: str) -> None:
        folder = self._resolver.resolve_folder(path)
        self._folders.delete(folder.id)
        self._resolver.invalidate_cache(path)

    def get_folder_ancestors(self, path: str) -> builtins.list[DiskFolder]:
        folder = self._resolver.resolve_folder(path)
        return self._folders.ancestors(folder.id)

    # --- File operations ---

    def upload_file(
        self,
        path: str,
        local_file: str,
        *,
        auto_mkdir: bool = False,
        agent_id: str = "",
    ) -> DiskFile:
        folder_path, _file_name = _split_file_path(path)
        if auto_mkdir:
            self._resolver.mkdir(folder_path, exist_ok=True)
        folder_id = self._resolver.resolve_folder_id(folder_path)
        return self._files.upload(
            local_file,
            folder_id=folder_id,
            agent_id=agent_id,
        )

    def upload_bytes(
        self,
        path: str,
        content: bytes,
        *,
        auto_mkdir: bool = False,
        agent_id: str = "",
        content_type: str = "application/octet-stream",
    ) -> DiskFile:
        folder_path, file_name = _split_file_path(path)
        if auto_mkdir:
            self._resolver.mkdir(folder_path, exist_ok=True)
        folder_id = self._resolver.resolve_folder_id(folder_path)
        return self._files.upload_bytes(
            file_name,
            content,
            folder_id=folder_id,
            agent_id=agent_id,
            content_type=content_type,
        )

    def list_files(self, path: str = "/") -> builtins.list[DiskFile]:
        folder_id = self._resolver.resolve_folder_id(path)
        return self._files.list(folder_id)

    def get_file(self, path: str) -> FileDetailResponse:
        file_obj = self._resolver.resolve_file(path)
        return self._files.get(file_obj.id)

    def update_file(self, path: str, local_file: str) -> DiskFile:
        file_obj = self._resolver.resolve_file(path)
        return self._files.update(file_obj.id, local_file)

    def update_file_bytes(
        self,
        path: str,
        content: bytes,
        *,
        content_type: str = "application/octet-stream",
    ) -> DiskFile:
        file_obj = self._resolver.resolve_file(path)
        return self._files.update_bytes(file_obj.id, file_obj.file_name, content, content_type=content_type)

    def delete_file(self, path: str) -> None:
        file_obj = self._resolver.resolve_file(path)
        self._files.delete(file_obj.id)

    def download_file(self, path: str) -> DownloadByTokenResponse:
        file_obj = self._resolver.resolve_file(path)
        token_resp = self._files.create_download_token(file_obj.id)
        return self._files.download_by_token(token_resp.download_token)

    def download_file_to(self, path: str, local_path: str) -> str:
        result = self.download_file(path)
        resp = self._http.get(result.download_url)
        resp.raise_for_status()
        with open(local_path, "wb") as f:
            f.write(resp.content)
        return local_path

    # --- Share operations ---

    def create_share(
        self,
        path: str,
        *,
        is_file: bool = False,
        extract_code: str = "",
        max_visit: int = -1,
        expire_hours: int = 72,
    ) -> DiskShare:
        if is_file:
            resource = self._resolver.resolve_file(path)
            resource_id = resource.id
            res_type = "file"
        else:
            folder = self._resolver.resolve_folder(path)
            resource_id = folder.id
            res_type = "folder"
        return self._shares.create(
            resource_id,
            res_type,
            extract_code=extract_code,
            max_visit=max_visit,
            expire_hours=expire_hours,
        )

    def list_shares(self) -> builtins.list[DiskShare]:
        return self._shares.list()

    def revoke_share(self, share_id: int) -> None:
        self._shares.revoke(share_id)

    def get_share_by_code(self, code: str) -> DiskShare:
        return self._shares.get_by_code(code)

    def access_share(self, code: str, extract_code: str = "") -> DiskShare:
        return self._shares.access(code, extract_code)

    def download_shared_file(self, code: str, resource_id: int, extract_code: str = "") -> DownloadByTokenResponse:
        token_resp = self._shares.download(code, resource_id, extract_code=extract_code)
        return self._files.download_by_token(token_resp.download_token)

    # --- Permission operations ---

    def grant_permission(
        self,
        path: str,
        agent_id: str,
        permission: str,
        *,
        is_file: bool = False,
    ) -> None:
        if is_file:
            resource = self._resolver.resolve_file(path)
            resource_id = resource.id
            res_type = "file"
        else:
            folder = self._resolver.resolve_folder(path)
            resource_id = folder.id
            res_type = "folder"
        self._permissions.grant(agent_id, resource_id, res_type, permission)

    def list_permissions(self) -> builtins.list[DiskPermission]:
        return self._permissions.list()

    def check_permission(
        self,
        path: str,
        permission: str,
        *,
        is_file: bool = False,
        agent_id: str = "",
    ) -> bool:
        if is_file:
            resource = self._resolver.resolve_file(path)
            resource_id = resource.id
            res_type = "file"
        else:
            folder = self._resolver.resolve_folder(path)
            resource_id = folder.id
            res_type = "folder"
        return self._permissions.check(resource_id, res_type, permission, agent_id=agent_id)

    def revoke_permission(
        self,
        path: str,
        agent_id: str,
        *,
        is_file: bool = False,
    ) -> None:
        if is_file:
            resource = self._resolver.resolve_file(path)
            resource_id = resource.id
            res_type = "file"
        else:
            folder = self._resolver.resolve_folder(path)
            resource_id = folder.id
            res_type = "folder"
        self._permissions.revoke(agent_id, resource_id, res_type)

    # --- Tag operations ---

    def bind_tag(self, path: str, tag_name: str) -> None:
        file_obj = self._resolver.resolve_file(path)
        self._tags.bind(file_obj.id, tag_name)

    def unbind_tag(self, path: str, tag_name: str) -> None:
        file_obj = self._resolver.resolve_file(path)
        self._tags.unbind(file_obj.id, tag_name)

    def search_files(self, tags: builtins.list[str]) -> builtins.list[DiskFile]:
        return self._tags.search(tags)

    # --- Version operations ---

    def list_versions(self, path: str) -> builtins.list[DiskFileVersion]:
        file_obj = self._resolver.resolve_file(path)
        return self._versions.list(file_obj.id)

    def rollback_version(self, path: str, version: int) -> None:
        file_obj = self._resolver.resolve_file(path)
        self._versions.rollback(file_obj.id, version)

    # --- Recycle bin operations ---

    def list_recycle(self) -> builtins.list[DiskRecycleBin]:
        return self._recycle.list()

    def restore(self, recycle_id: int) -> None:
        self._recycle.restore(recycle_id)

    def delete_permanent(self, recycle_id: int) -> None:
        self._recycle.delete_permanent(recycle_id)

    # --- Preview operations ---

    def preview(self, path: str) -> PreviewResult:
        file_obj = self._resolver.resolve_file(path)
        return self._preview.file(file_obj.id)

    # --- Space operations ---

    def get_space(self) -> UserDisk:
        return self._space.get()

    # --- Public directory operations ---

    def list_public_directories(self) -> builtins.list[DiskPublicDirectory]:
        return self._public_dirs.list_visible()

    def get_public_directory(self, path: str) -> DiskPublicDirectory:
        return self._resolver.resolve_public_directory(path)

    def list_public_directory_folders(self, path: str) -> builtins.list[DiskFolder]:
        _, folder_id = self._resolver.resolve_public_path(path)
        return self._folders.list(folder_id)

    def list_public_directory_files(self, path: str) -> builtins.list[DiskFile]:
        _, folder_id = self._resolver.resolve_public_path(path)
        return self._files.list(folder_id)

    # --- Cache management ---

    def invalidate_cache(self, path: str = "") -> None:
        self._resolver.invalidate_cache(path)

    def clear_cache(self) -> None:
        self._resolver.clear_cache()

    # --- Lifecycle ---

    def close(self) -> None:
        self._http.close()

    def __enter__(self) -> AgentDiskClient:
        return self

    def __exit__(self, *_: object) -> None:
        self.close()


def _split_file_path(path: str) -> tuple[str, str]:
    normalized = path.strip("/")
    if not normalized:
        raise ValueError("File path cannot be empty or root")
    idx = normalized.rfind("/")
    if idx == -1:
        return "", normalized
    return normalized[:idx], normalized[idx + 1 :]
