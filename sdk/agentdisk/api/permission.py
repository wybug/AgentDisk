"""Permission API."""

from __future__ import annotations

from typing import TYPE_CHECKING

from ..models.permission import DiskPermission, PermissionCheckResponse
from .base import AsyncBaseAPI, BaseAPI

if TYPE_CHECKING:
    import builtins


class PermissionAPI(BaseAPI):
    def grant(self, agent_id: str, resource_id: int, res_type: str, permission: str) -> None:
        self._request(
            "POST",
            "/permissions",
            json={
                "agentId": agent_id,
                "resourceId": resource_id,
                "resType": res_type,
                "permission": permission,
            },
        )

    def list(self) -> builtins.list[DiskPermission]:
        data = self._request("GET", "/permissions")
        return [DiskPermission.from_dict(d) for d in (data or [])]

    def check(self, resource_id: int, res_type: str, permission: str, agent_id: str = "") -> bool:
        data = self._request(
            "GET",
            "/permissions/check",
            params={
                "resourceId": resource_id,
                "resType": res_type,
                "permission": permission,
                **({"agentId": agent_id} if agent_id else {}),
            },
        )
        return PermissionCheckResponse.from_dict(data).allowed

    def revoke(self, agent_id: str, resource_id: int, res_type: str) -> None:
        self._request(
            "DELETE",
            "/permissions",
            json={"agentId": agent_id, "resourceId": resource_id, "resType": res_type},
        )


class AsyncPermissionAPI(AsyncBaseAPI):
    async def grant(self, agent_id: str, resource_id: int, res_type: str, permission: str) -> None:
        await self._request(
            "POST",
            "/permissions",
            json={
                "agentId": agent_id,
                "resourceId": resource_id,
                "resType": res_type,
                "permission": permission,
            },
        )

    async def list(self) -> builtins.list[DiskPermission]:
        data = await self._request("GET", "/permissions")
        return [DiskPermission.from_dict(d) for d in (data or [])]

    async def check(self, resource_id: int, res_type: str, permission: str, agent_id: str = "") -> bool:
        data = await self._request(
            "GET",
            "/permissions/check",
            params={
                "resourceId": resource_id,
                "resType": res_type,
                "permission": permission,
                **({"agentId": agent_id} if agent_id else {}),
            },
        )
        return PermissionCheckResponse.from_dict(data).allowed

    async def revoke(self, agent_id: str, resource_id: int, res_type: str) -> None:
        await self._request(
            "DELETE",
            "/permissions",
            json={"agentId": agent_id, "resourceId": resource_id, "resType": res_type},
        )
