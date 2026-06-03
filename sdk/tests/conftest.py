"""Pytest fixtures for SDK tests."""

import os
import subprocess

import pytest

from agentdisk import AgentDiskClient, AsyncAgentDiskClient

BASE_URL = os.environ.get("AGENTDISK_URL", "http://localhost:9100")
JWT_SECRET = os.environ.get("AGENTDISK_JWT_SECRET", "dev-jwt-secret-for-testing-only")
DL_SECRET = os.environ.get("AGENTDISK_DL_SECRET", "dev-dl-token-secret-for-testing")


def _generate_jwt(user_id: str, agent_id: str = "", agent_group_id: str = "") -> str:
    args = ["go", "run", "scripts/gen_token/main.go", "-secret", JWT_SECRET, "-userId", user_id]
    if agent_id:
        args += ["-agentId", agent_id]
    if agent_group_id:
        args += ["-agentGroupId", agent_group_id]
    result = subprocess.run(
        args, capture_output=True, text=True, cwd=os.path.dirname(os.path.dirname(os.path.dirname(__file__)))
    )
    return result.stdout.strip()


@pytest.fixture(scope="session")
def user_token():
    return _generate_jwt("sdk-test-user")


@pytest.fixture(scope="session")
def agent_token():
    return _generate_jwt("sdk-test-user", "sdk-test-agent", "sdk-test-group")


def _cleanup(client: AgentDiskClient) -> None:
    """Force-clean all data for the SDK test user."""
    # Revoke all shares
    for s in client.list_shares():
        try:
            client.revoke_share(s.id)
        except Exception:
            pass

    # Revoke all permissions
    for p in client.list_permissions():
        try:
            client._permissions.revoke(p.agent_id, p.resource_id, p.res_type)
        except Exception:
            pass

    # Delete all files in root and subfolders
    for folder in client.list_folders("/"):
        _delete_folder_recursive(client, folder)

    # Delete root-level files
    for f in client.list_files("/"):
        try:
            client._files.delete(f.id)
        except Exception:
            pass

    # Clear recycle bin
    for item in client.list_recycle():
        try:
            client.delete_permanent(item.id)
        except Exception:
            pass

    client.clear_cache()


def _delete_folder_recursive(client: AgentDiskClient, folder) -> None:
    """Recursively delete a folder and all its contents."""
    # Delete files in this folder
    for f in client._files.list(folder.id):
        try:
            client._files.delete(f.id)
        except Exception:
            pass

    # Recurse into subfolders
    for sub in client._folders.list(folder.id):
        _delete_folder_recursive(client, sub)

    # Delete the folder itself
    try:
        client._folders.delete(folder.id)
    except Exception:
        pass


@pytest.fixture(scope="session", autouse=True)
def cleanup(user_token):
    """Run forced cleanup before and after all tests."""
    with AgentDiskClient(base_url=BASE_URL, token=user_token) as c:
        _cleanup(c)
    yield
    with AgentDiskClient(base_url=BASE_URL, token=user_token) as c:
        _cleanup(c)


@pytest.fixture(scope="session")
def client(user_token):
    with AgentDiskClient(base_url=BASE_URL, token=user_token) as c:
        yield c


@pytest.fixture(scope="session")
def agent_client(agent_token):
    with AgentDiskClient(base_url=BASE_URL, token=agent_token) as c:
        yield c


@pytest.fixture
def async_client(user_token):
    c = AsyncAgentDiskClient(base_url=BASE_URL, token=user_token)
    yield c
    import asyncio

    asyncio.get_event_loop().run_until_complete(c.close())
