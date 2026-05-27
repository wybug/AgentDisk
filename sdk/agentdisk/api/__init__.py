"""Internal API sub-clients (not part of public SDK surface)."""

from .file import _AsyncFileAPI, _FileAPI
from .folder import _AsyncFolderAPI, _FolderAPI
from .permission import _AsyncPermissionAPI, _PermissionAPI
from .preview import _AsyncPreviewAPI, _PreviewAPI
from .public_directory import _AsyncPublicDirectoryAPI, _PublicDirectoryAPI
from .recycle import _AsyncRecycleAPI, _RecycleAPI
from .share import _AsyncShareAPI, _ShareAPI
from .space import _AsyncSpaceAPI, _SpaceAPI
from .tag import _AsyncTagAPI, _TagAPI
from .version import _AsyncVersionAPI, _VersionAPI

__all__ = [
    "_AsyncFileAPI",
    "_AsyncFolderAPI",
    "_AsyncPermissionAPI",
    "_AsyncPreviewAPI",
    "_AsyncPublicDirectoryAPI",
    "_AsyncRecycleAPI",
    "_AsyncShareAPI",
    "_AsyncSpaceAPI",
    "_AsyncTagAPI",
    "_AsyncVersionAPI",
    "_FileAPI",
    "_FolderAPI",
    "_PermissionAPI",
    "_PreviewAPI",
    "_PublicDirectoryAPI",
    "_RecycleAPI",
    "_ShareAPI",
    "_SpaceAPI",
    "_TagAPI",
    "_VersionAPI",
]
