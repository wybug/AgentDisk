"""Pytest fixtures for SDK tests."""

import os
import subprocess

import httpx
import pytest

from agentdisk import AgentDiskAdminClient, AgentDiskClient, AsyncAgentDiskClient

BASE_URL = os.environ.get("AGENTDISK_URL", "http://localhost:9100")
JWT_SECRET = os.environ.get("AGENTDISK_JWT_SECRET", "dev-jwt-secret-for-testing-only")
DL_SECRET = os.environ.get("AGENTDISK_DL_SECRET", "dev-dl-token-secret-for-testing")
ADMIN_USERNAME = os.environ.get("AGENTDISK_ADMIN_USER", "admin")
ADMIN_PASSWORD = os.environ.get("AGENTDISK_ADMIN_PASS", "admin123")


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


def _admin_login() -> str:
    resp = httpx.post(
        f"{BASE_URL}/v1/disk/admin/login",
        json={"username": ADMIN_USERNAME, "password": ADMIN_PASSWORD},
    )
    resp.raise_for_status()
    return resp.json()["data"]["token"]


def _create_api_key(admin_token: str) -> str:
    resp = httpx.post(
        f"{BASE_URL}/v1/disk/admin/api-keys",
        headers={"Authorization": f"Bearer {admin_token}"},
        json={"keyName": "sdk-test-key"},
    )
    resp.raise_for_status()
    return resp.json()["data"]["key"]


def _create_public_directory(admin_token: str, name: str) -> dict:
    resp = httpx.post(
        f"{BASE_URL}/v1/disk/admin/public-directories",
        headers={"Authorization": f"Bearer {admin_token}"},
        json={"displayName": name, "scope": "global"},
    )
    resp.raise_for_status()
    return resp.json()["data"]


def _delete_public_directory(admin_token: str, pd_id: int) -> None:
    httpx.delete(
        f"{BASE_URL}/v1/disk/admin/public-directories/{pd_id}",
        headers={"Authorization": f"Bearer {admin_token}"},
    )


def _revoke_api_key(admin_token: str, key_id: int) -> None:
    httpx.delete(
        f"{BASE_URL}/v1/disk/admin/api-keys/{key_id}",
        headers={"Authorization": f"Bearer {admin_token}"},
    )


@pytest.fixture(scope="session")
def user_token():
    return _generate_jwt("sdk-test-user")


@pytest.fixture(scope="session")
def agent_token():
    return _generate_jwt("sdk-test-user", "sdk-test-agent", "sdk-test-group")


def _cleanup(client: AgentDiskClient) -> None:
    """Force-clean all data for the SDK test user."""
    for s in client.list_shares():
        try:
            client.revoke_share(s.id)
        except Exception:
            pass

    for p in client.list_permissions():
        try:
            client._permissions.revoke(p.agent_id, p.resource_id, p.res_type)
        except Exception:
            pass

    for folder in client.list_folders("/"):
        _delete_folder_recursive(client, folder)

    for f in client.list_files("/"):
        try:
            client._files.delete(f.id)
        except Exception:
            pass

    for item in client.list_recycle():
        try:
            client.delete_permanent(item.id)
        except Exception:
            pass

    client.clear_cache()


def _delete_folder_recursive(client: AgentDiskClient, folder) -> None:
    for f in client._files.list(folder.id):
        try:
            client._files.delete(f.id)
        except Exception:
            pass

    for sub in client._folders.list(folder.id):
        _delete_folder_recursive(client, sub)

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
async def async_client(user_token):
    c = AsyncAgentDiskClient(base_url=BASE_URL, token=user_token)
    yield c
    await c.close()


# --- Public directory test fixtures ---


@pytest.fixture(scope="session")
def admin_token():
    return _admin_login()


@pytest.fixture(scope="session")
def admin_http(admin_token):
    return httpx.Client(base_url=BASE_URL, headers={"Authorization": f"Bearer {admin_token}"})


@pytest.fixture(scope="session")
def api_key_info(admin_token):
    """Create API Key and return (raw_key, key_id)."""
    resp = httpx.post(
        f"{BASE_URL}/v1/disk/admin/api-keys",
        headers={"Authorization": f"Bearer {admin_token}"},
        json={"name": "sdk-test-pubdir-key"},
    )
    resp.raise_for_status()
    data = resp.json()["data"]
    return data["key"], data["id"]


@pytest.fixture(scope="session")
def api_key(api_key_info):
    return api_key_info[0]


@pytest.fixture(scope="session")
def api_key_id(api_key_info):
    return api_key_info[1]


@pytest.fixture(scope="session")
def test_public_dir(admin_token):
    """Create a public directory for testing, yield its info, cleanup after."""
    info = _create_public_directory(admin_token, "sdk-test-public")
    yield info
    _delete_public_directory(admin_token, info["id"])


@pytest.fixture(scope="session")
def admin_client(api_key):
    with AgentDiskAdminClient(base_url=BASE_URL, api_key=api_key) as c:
        yield c


@pytest.fixture(scope="session")
def api_key_client(api_key):
    with AgentDiskClient(base_url=BASE_URL, api_key=api_key) as c:
        yield c
