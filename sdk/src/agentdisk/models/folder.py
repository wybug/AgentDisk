"""Folder models."""

from dataclasses import dataclass


@dataclass
class DiskFolder:
    id: int
    user_id: str
    parent_id: int
    folder_name: str
    full_path: str
    sort_order: int
    is_deleted: bool
    created_at: str
    updated_at: str

    @classmethod
    def from_dict(cls, d: dict) -> "DiskFolder":
        return cls(
            id=d["id"],
            user_id=d.get("userId", ""),
            parent_id=d.get("parentId", 0),
            folder_name=d.get("folderName", ""),
            full_path=d.get("fullPath", ""),
            sort_order=d.get("sortOrder", 0),
            is_deleted=d.get("isDeleted", False),
            created_at=d.get("createdAt", ""),
            updated_at=d.get("updatedAt", ""),
        )
