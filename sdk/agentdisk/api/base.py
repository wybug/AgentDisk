"""Base API class with auth injection and response parsing."""

from __future__ import annotations

from typing import TYPE_CHECKING, Any

from ..exceptions import raise_for_response

if TYPE_CHECKING:
    import httpx

_PREFIX = "/v1/disk"


class BaseAPI:
    """Shared HTTP helper for all API sub-clients."""

    def __init__(self, client: httpx.Client, token: str):
        self._client = client
        self._token = token

    def _headers(self) -> dict[str, str]:
        return {"Authorization": f"Bearer {self._token}"}

    def _request(
        self,
        method: str,
        path: str,
        *,
        params: dict | None = None,
        json: dict | None = None,
        files: dict | None = None,
        data: dict | None = None,
    ) -> Any:
        url = f"{_PREFIX}{path}"
        resp = self._client.request(
            method,
            url,
            headers=self._headers(),
            params=_drop_none(params or {}),
            json=json,
            files=files,
            data=data,
        )
        body = resp.json()
        raise_for_response(resp.status_code, body)
        return body.get("data")


class AsyncBaseAPI:
    """Async version of BaseAPI."""

    def __init__(self, client: httpx.AsyncClient, token: str):
        self._client = client
        self._token = token

    def _headers(self) -> dict[str, str]:
        return {"Authorization": f"Bearer {self._token}"}

    async def _request(
        self,
        method: str,
        path: str,
        *,
        params: dict | None = None,
        json: dict | None = None,
        files: dict | None = None,
        data: dict | None = None,
    ) -> Any:
        url = f"{_PREFIX}{path}"
        resp = await self._client.request(
            method,
            url,
            headers=self._headers(),
            params=_drop_none(params or {}),
            json=json,
            files=files,
            data=data,
        )
        body = resp.json()
        raise_for_response(resp.status_code, body)
        return body.get("data")


def _drop_none(d: dict) -> dict:
    return {k: v for k, v in d.items() if v is not None}
