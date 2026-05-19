"""Permission models."""

from dataclasses import dataclass


@dataclass
class DiskPermission:
    id: int
    user_id: str
    agent_id: str
    resource_id: int
    res_type: str
    permission: str
    created_at: str
    updated_at: str

    @classmethod
    def from_dict(cls, d: dict) -> "DiskPermission":
        return cls(
            id=d["id"],
            user_id=d["userId"],
            agent_id=d["agentId"],
            resource_id=d["resourceId"],
            res_type=d["resType"],
            permission=d["permission"],
            created_at=d["createdAt"],
            updated_at=d["updatedAt"],
        )


@dataclass
class PermissionCheckResponse:
    allowed: bool

    @classmethod
    def from_dict(cls, d: dict) -> "PermissionCheckResponse":
        return cls(allowed=d["allowed"])
