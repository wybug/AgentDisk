"""AgentDisk admin client for public directory management (API Key auth only)."""

from __future__ import annotations

from typing import TYPE_CHECKING

import httpx

from .api.public_directory import _PublicDirectoryAPI

if TYPE_CHECKING:
    import builtins


class AgentDiskAdminClient:
    """Admin client for public directory management (API Key auth only).

    Usage:
        admin = AgentDiskAdminClient(
            base_url="http://localhost:9100",
            api_key="<api-key>",
        )
        admin.grant_access(1, "user-123")
        admin.revoke_access(1, "user-123")
        admin.list_granted_users(1)
        admin.close()
    """

    def __init__(self, base_url: str, api_key: str, timeout: float = 30.0) -> None:
        self._http = httpx.Client(base_url=base_url, timeout=timeout)
        self._public_dirs = _PublicDirectoryAPI(self._http, api_key=api_key)

    def list_public_directories(self) -> builtins.list:
        return self._public_dirs.list_visible()

    def grant_access(self, public_dir_id: int, user_id: str) -> None:
        self._public_dirs.grant_access(public_dir_id, user_id)

    def revoke_access(self, public_dir_id: int, user_id: str) -> None:
        self._public_dirs.revoke_access(public_dir_id, user_id)

    def list_granted_users(self, public_dir_id: int) -> builtins.list[str]:
        return self._public_dirs.list_granted_users(public_dir_id)

    def close(self) -> None:
        self._http.close()

    def __enter__(self) -> AgentDiskAdminClient:
        return self

    def __exit__(self, *_: object) -> None:
        self.close()
