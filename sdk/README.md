# AgentDisk Python SDK

Python SDK for [AgentDisk](https://github.com/wybug/AgentDisk) — an enterprise-grade cloud disk middleware designed for multi-agent systems.

## Installation

```bash
pip install agentdisk
```

Requires Python 3.9+.

## Quick Start

### Authentication

The SDK supports two authentication methods — JWT token or API Key:

```python
# JWT token (from gateway auth) — full private resource access + public directory read-only
client = AgentDiskClient(base_url="http://localhost:9100", token="<jwt>")

# API Key (from admin panel) — public directory CRUD only, no private access
client = AgentDiskClient(base_url="http://localhost:9100", api_key="<api-key>")
```

### Synchronous Client

```python
from agentdisk import AgentDiskClient

client = AgentDiskClient(
    base_url="http://localhost:9100",
    token="<jwt-from-gateway>",
)

# Folder operations
client.create_folder("docs/reports")
folders = client.list_folders("docs")
ancestors = client.get_folder_ancestors("docs/reports")

# File operations
client.upload_file("docs/reports/summary.md", "/local/summary.md")
client.upload_bytes("docs/notes.txt", b"hello world", auto_mkdir=True)
files = client.list_files("docs/reports")

# File download
result = client.download_file("docs/reports/summary.md")
client.download_file_to("docs/reports/summary.md", "/local/summary.md")

# Share
share = client.create_share("docs/reports", expire_hours=24)

# Download shared file
client.download_shared_file(code="AbC123", resource_id=42)

# Public directory — access by display_name as path
pub_dirs = client.list_public_directories()
client.list_files("shared-project")                   # list files
client.download_file("shared-project/data.csv")       # download
client.create_share("shared-project/data.csv", is_file=True)  # share

client.close()
```

### Asynchronous Client

```python
from agentdisk import AsyncAgentDiskClient

async with AsyncAgentDiskClient(
    base_url="http://localhost:9100",
    token="<jwt-from-gateway>",
) as client:
    await client.create_folder("docs/reports")
    await client.upload_file("docs/reports/summary.md", "/local/summary.md")
    files = await client.list_files("docs/reports")

    # Download
    result = await client.download_file("docs/reports/summary.md")
    await client.download_file_to("docs/reports/summary.md", "/local/summary.md")
```

### Admin Client (API Key only)

```python
from agentdisk import AgentDiskAdminClient

admin = AgentDiskAdminClient(
    base_url="http://localhost:9100",
    api_key="<api-key>",
)

# Grant user access to a public directory
admin.grant_access(public_dir_id=1, user_id="user-123")
users = admin.list_granted_users(public_dir_id=1)

# Revoke access
admin.revoke_access(public_dir_id=1, user_id="user-123")
admin.close()
```

## Authentication Matrix

| Operation | JWT Token | API Key |
|-----------|-----------|---------|
| Private files/folders CRUD | Full access | Blocked |
| Public directory list/browse | Granted users | Full access |
| Public directory download/share | Granted users | Blocked |
| Public directory upload/delete/create folder | Blocked | Full access |
| User authorization management | Blocked | Full access |

## API Overview

All operations use **path-based** API — no need to manage folder/file IDs manually.

Public directories are accessed by their `display_name` as a path segment:

```python
# Private resource
client.upload_file("docs/report.txt", "/local/report.txt")

# Public directory (display_name = "shared-project")
client.list_files("shared-project")
client.download_file("shared-project/data.csv")
```

| Category | Methods |
|----------|---------|
| **Folders** | `create_folder`, `list_folders`, `get_folder`, `rename_folder`, `delete_folder`, `get_folder_ancestors` |
| **Files** | `upload_file`, `upload_bytes`, `list_files`, `get_file`, `update_file`, `update_file_bytes`, `delete_file`, `download_file`, `download_file_to` |
| **Shares** | `create_share`, `list_shares`, `revoke_share`, `get_share_by_code`, `access_share`, `download_shared_file` |
| **Permissions** | `grant_permission`, `list_permissions`, `check_permission`, `revoke_permission` |
| **Tags** | `bind_tag`, `unbind_tag`, `search_files` |
| **Versions** | `list_versions`, `rollback_version` |
| **Recycle Bin** | `list_recycle`, `restore`, `delete_permanent` |
| **Space** | `get_space` |
| **Public Directory** | `list_public_directories` |
| **Cache** | `invalidate_cache`, `clear_cache` |

## License

Apache-2.0
