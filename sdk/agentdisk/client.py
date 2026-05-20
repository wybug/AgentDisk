"""AgentDisk sync client."""

from __future__ import annotations

import httpx

from .api import (
    FileAPI,
    FolderAPI,
    PermissionAPI,
    PreviewAPI,
    RecycleAPI,
    ShareAPI,
    SpaceAPI,
    TagAPI,
    VersionAPI,
)


class AgentDiskClient:
    """Synchronous AgentDisk SDK client (gateway mode).

    Usage:
        client = AgentDiskClient(
            base_url="http://localhost:9100",
            token="<jwt-from-gateway>",
        )
        space = client.space.get()
        client.close()
    """

    def __init__(
        self,
        base_url: str,
        token: str,
        timeout: float = 30.0,
    ):
        self._http = httpx.Client(base_url=base_url, timeout=timeout)
        self._token = token
        self.space = SpaceAPI(self._http, token)
        self.folders = FolderAPI(self._http, token)
        self.files = FileAPI(self._http, token)
        self.permissions = PermissionAPI(self._http, token)
        self.versions = VersionAPI(self._http, token)
        self.recycle = RecycleAPI(self._http, token)
        self.tags = TagAPI(self._http, token)
        self.shares = ShareAPI(self._http, token)
        self.preview = PreviewAPI(self._http, token)

    def close(self) -> None:
        self._http.close()

    def __enter__(self) -> AgentDiskClient:
        return self

    def __exit__(self, *_: object) -> None:
        self.close()
