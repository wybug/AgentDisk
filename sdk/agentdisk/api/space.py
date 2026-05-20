"""Space API."""

from __future__ import annotations

from ..models.space import UserDisk
from .base import AsyncBaseAPI, BaseAPI


class _SpaceAPI(BaseAPI):
    def get(self) -> UserDisk:
        data = self._request("GET", "/space")
        return UserDisk.from_dict(data)


class _AsyncSpaceAPI(AsyncBaseAPI):
    async def get(self) -> UserDisk:
        data = await self._request("GET", "/space")
        return UserDisk.from_dict(data)
