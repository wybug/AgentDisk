"""File models."""

from dataclasses import dataclass


@dataclass
class DiskFile:
    id: int
    user_id: str
    folder_id: int
    file_name: str
    file_size: int
    file_type: str
    oss_key: str
    md5: str
    version: int
    is_deleted: bool
    source_agent: str
    source_agent_group: str
    is_artifact: bool
    tags: str
    created_at: str
    updated_at: str

    @classmethod
    def from_dict(cls, d: dict) -> "DiskFile":
        return cls(
            id=d["id"],
            user_id=d["userId"],
            folder_id=d.get("folderId", 0),
            file_name=d["fileName"],
            file_size=d.get("fileSize", 0),
            file_type=d.get("fileType", ""),
            oss_key=d.get("ossKey", ""),
            md5=d.get("md5", ""),
            version=d.get("version", 1),
            is_deleted=d.get("isDeleted", False),
            source_agent=d.get("sourceAgent", ""),
            source_agent_group=d.get("sourceAgentGroup", ""),
            is_artifact=d.get("isArtifact", False),
            tags=d.get("tags", ""),
            created_at=d["createdAt"],
            updated_at=d["updatedAt"],
        )


@dataclass
class FileDetailResponse:
    file: DiskFile
    url: str

    @classmethod
    def from_dict(cls, d: dict) -> "FileDetailResponse":
        return cls(
            file=DiskFile.from_dict(d["file"]),
            url=d.get("url", ""),
        )


@dataclass
class DownloadTokenResponse:
    download_token: str
    expires_in: int

    @classmethod
    def from_dict(cls, d: dict) -> "DownloadTokenResponse":
        return cls(
            download_token=d["downloadToken"],
            expires_in=d["expiresIn"],
        )


@dataclass
class DownloadByTokenResponse:
    file: DiskFile
    download_url: str

    @classmethod
    def from_dict(cls, d: dict) -> "DownloadByTokenResponse":
        return cls(
            file=DiskFile.from_dict(d["file"]),
            download_url=d.get("downloadUrl", ""),
        )
