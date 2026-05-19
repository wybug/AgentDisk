"""API sub-clients."""

from .file import AsyncFileAPI, FileAPI
from .folder import AsyncFolderAPI, FolderAPI
from .permission import AsyncPermissionAPI, PermissionAPI
from .preview import AsyncPreviewAPI, PreviewAPI
from .recycle import AsyncRecycleAPI, RecycleAPI
from .share import AsyncShareAPI, ShareAPI
from .space import AsyncSpaceAPI, SpaceAPI
from .tag import AsyncTagAPI, TagAPI
from .version import AsyncVersionAPI, VersionAPI

__all__ = [
    "AsyncFileAPI",
    "AsyncFolderAPI",
    "AsyncPermissionAPI",
    "AsyncPreviewAPI",
    "AsyncRecycleAPI",
    "AsyncShareAPI",
    "AsyncSpaceAPI",
    "AsyncTagAPI",
    "AsyncVersionAPI",
    "FileAPI",
    "FolderAPI",
    "PermissionAPI",
    "PreviewAPI",
    "RecycleAPI",
    "ShareAPI",
    "SpaceAPI",
    "TagAPI",
    "VersionAPI",
]
