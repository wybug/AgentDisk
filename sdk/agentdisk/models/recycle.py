"""Recycle bin models."""

from dataclasses import dataclass


@dataclass
class DiskRecycleBin:
    id: int
    user_id: str
    resource_id: int
    res_type: str
    res_name: str
    original_path: str
    deleted_by: str
    expire_at: str
    created_at: str

    @classmethod
    def from_dict(cls, d: dict) -> "DiskRecycleBin":
        return cls(
            id=d["id"],
            user_id=d["userId"],
            resource_id=d["resourceId"],
            res_type=d["resType"],
            res_name=d.get("resName", ""),
            original_path=d.get("originalPath", ""),
            deleted_by=d.get("deletedBy", ""),
            expire_at=d.get("expireAt", ""),
            created_at=d["createdAt"],
        )
