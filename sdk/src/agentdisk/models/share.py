"""Share models."""

from dataclasses import dataclass


@dataclass
class DiskShare:
    id: int
    user_id: str
    resource_id: int
    res_type: str
    share_code: str
    extract_code: str
    max_visit: int
    visit_count: int
    expire_at: str
    is_active: bool
    created_at: str

    @classmethod
    def from_dict(cls, d: dict) -> "DiskShare":
        return cls(
            id=d["id"],
            user_id=d["userId"],
            resource_id=d["resourceId"],
            res_type=d["resType"],
            share_code=d.get("shareCode", ""),
            extract_code=d.get("extractCode", ""),
            max_visit=d.get("maxVisit", -1),
            visit_count=d.get("visitCount", 0),
            expire_at=d.get("expireAt", ""),
            is_active=d.get("isActive", True),
            created_at=d["createdAt"],
        )
