"""Public directory model."""

from __future__ import annotations

from dataclasses import dataclass


@dataclass
class DiskPublicDirectory:
    """A public directory visible to authenticated users."""

    id: int
    folder_id: int
    scope: str
    department: str
    display_name: str
    fixed_path: str
    is_active: bool
    created_by: str


def from_dict(data: dict) -> DiskPublicDirectory:
    return DiskPublicDirectory(
        id=data.get("id", 0),
        folder_id=data.get("folderId", 0),
        scope=data.get("scope", ""),
        department=data.get("department", ""),
        display_name=data.get("displayName", ""),
        fixed_path=data.get("fixedPath", ""),
        is_active=data.get("isActive", True),
        created_by=data.get("createdBy", ""),
    )
