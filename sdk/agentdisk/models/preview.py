"""Preview models."""

from dataclasses import dataclass


@dataclass
class PreviewResult:
    file_type: str
    url: str

    @classmethod
    def from_dict(cls, d: dict) -> "PreviewResult":
        return cls(
            file_type=d.get("fileType", ""),
            url=d.get("url", ""),
        )
