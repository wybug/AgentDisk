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
# JWT token (from gateway auth)
client = AgentDiskClient(base_url="http://localhost:9100", token="<jwt>")

# API Key (from admin panel)
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

# Share & preview
share = client.create_share("docs/reports", expire_hours=24)
result = client.preview("docs/reports/summary.md")

# Download shared file
client.download_shared_file(code="AbC123", resource_id=42)

# Public directory
pub_dirs = client.list_public_directories()
pub_folders = client.list_public_directory_folders("shared-project")
pub_files = client.list_public_directory_files("shared-project/docs")

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

## API Overview

All operations use **path-based** API — no need to manage folder/file IDs manually.

| Category | Methods |
|----------|---------|
| **Folders** | `create_folder`, `list_folders`, `get_folder`, `rename_folder`, `delete_folder`, `get_folder_ancestors` |
| **Files** | `upload_file`, `upload_bytes`, `list_files`, `get_file`, `update_file`, `update_file_bytes`, `delete_file`, `download_file`, `download_file_to` |
| **Shares** | `create_share`, `list_shares`, `revoke_share`, `get_share_by_code`, `access_share`, `download_shared_file` |
| **Permissions** | `grant_permission`, `list_permissions`, `check_permission`, `revoke_permission` |
| **Tags** | `bind_tag`, `unbind_tag`, `search_files` |
| **Versions** | `list_versions`, `rollback_version` |
| **Recycle Bin** | `list_recycle`, `restore`, `delete_permanent` |
| **Preview** | `preview` |
| **Space** | `get_space` |
| **Public Directory** | `list_public_directories`, `get_public_directory`, `list_public_directory_folders`, `list_public_directory_files` |
| **Cache** | `invalidate_cache`, `clear_cache` |

## License

Apache-2.0
