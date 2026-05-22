# AgentDisk Python SDK

Python SDK for [AgentDisk](https://github.com/wybug/AgentDisk) — an enterprise-grade cloud disk middleware designed for multi-agent systems.

## Installation

```bash
pip install agentdisk
```

Requires Python 3.9+.

## Quick Start

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

# File operations
client.upload_file("docs/reports/summary.md", "/local/summary.md")
client.upload_bytes("docs/notes.txt", b"hello world", auto_mkdir=True)
files = client.list_files("docs/reports")

# Share & preview
share = client.create_share("docs/reports", expire_hours=24)
result = client.preview("docs/reports/summary.md")

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
```

## API Overview

All operations use **path-based** API — no need to manage folder/file IDs manually.

| Category | Methods |
|----------|---------|
| **Folders** | `create_folder`, `list_folders`, `get_folder`, `rename_folder`, `delete_folder` |
| **Files** | `upload_file`, `upload_bytes`, `list_files`, `get_file`, `update_file`, `update_file_bytes`, `delete_file` |
| **Shares** | `create_share`, `list_shares`, `revoke_share`, `get_share_by_code`, `access_share` |
| **Permissions** | `grant_permission`, `list_permissions`, `check_permission`, `revoke_permission` |
| **Tags** | `bind_tag`, `unbind_tag`, `search_files` |
| **Versions** | `list_versions`, `rollback_version` |
| **Recycle Bin** | `list_recycle`, `restore`, `delete_permanent` |
| **Preview** | `preview` |
| **Space** | `get_space` |
| **Cache** | `invalidate_cache`, `clear_cache` |

## License

Apache-2.0
