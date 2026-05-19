"""Version models."""

from dataclasses import dataclass


@dataclass
class DiskFileVersion:
    id: int
    file_id: int
    user_id: str
    version: int
    oss_key: str
    file_size: int
    md5: str
    snapshot_by: str
    created_at: str

    @classmethod
    def from_dict(cls, d: dict) -> "DiskFileVersion":
        return cls(
            id=d["id"],
            file_id=d["fileId"],
            user_id=d["userId"],
            version=d["version"],
            oss_key=d.get("ossKey", ""),
            file_size=d.get("fileSize", 0),
            md5=d.get("md5", ""),
            snapshot_by=d.get("snapshotBy", ""),
            created_at=d["createdAt"],
        )
