"""Space model."""

from dataclasses import dataclass


@dataclass
class UserDisk:
    id: int
    user_id: str
    total_quota: int
    used_quota: int
    root_folder: str
    created_at: str
    updated_at: str

    @classmethod
    def from_dict(cls, d: dict) -> "UserDisk":
        return cls(
            id=d["id"],
            user_id=d["userId"],
            total_quota=d["totalQuota"],
            used_quota=d["usedQuota"],
            root_folder=d.get("rootFolder", ""),
            created_at=d["createdAt"],
            updated_at=d["updatedAt"],
        )
