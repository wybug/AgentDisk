"""AgentDisk async client."""

from __future__ import annotations

import httpx

from .api import (
    AsyncFileAPI,
    AsyncFolderAPI,
    AsyncPermissionAPI,
    AsyncPreviewAPI,
    AsyncRecycleAPI,
    AsyncShareAPI,
    AsyncSpaceAPI,
    AsyncTagAPI,
    AsyncVersionAPI,
)


class AsyncAgentDiskClient:
    """Asynchronous AgentDisk SDK client (gateway mode).

    Usage:
        async with AsyncAgentDiskClient(
            base_url="http://localhost:9100",
            token="<jwt-from-gateway>",
        ) as client:
            space = await client.space.get()
    """

    def __init__(
        self,
        base_url: str,
        token: str,
        timeout: float = 30.0,
    ):
        self._http = httpx.AsyncClient(base_url=base_url, timeout=timeout)
        self._token = token
        self.space = AsyncSpaceAPI(self._http, token)
        self.folders = AsyncFolderAPI(self._http, token)
        self.files = AsyncFileAPI(self._http, token)
        self.permissions = AsyncPermissionAPI(self._http, token)
        self.versions = AsyncVersionAPI(self._http, token)
        self.recycle = AsyncRecycleAPI(self._http, token)
        self.tags = AsyncTagAPI(self._http, token)
        self.shares = AsyncShareAPI(self._http, token)
        self.preview = AsyncPreviewAPI(self._http, token)

    async def close(self) -> None:
        await self._http.aclose()

    async def __aenter__(self) -> AsyncAgentDiskClient:
        return self

    async def __aexit__(self, *_: object) -> None:
        await self.close()
