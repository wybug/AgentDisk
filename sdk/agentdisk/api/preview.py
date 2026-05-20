"""Preview API."""

from __future__ import annotations

from ..models.preview import PreviewResult
from .base import AsyncBaseAPI, BaseAPI


class PreviewAPI(BaseAPI):
    def file(self, id: int) -> PreviewResult:
        data = self._request("GET", f"/preview/{id}")
        return PreviewResult.from_dict(data)


class AsyncPreviewAPI(AsyncBaseAPI):
    async def file(self, id: int) -> PreviewResult:
        data = await self._request("GET", f"/preview/{id}")
        return PreviewResult.from_dict(data)
