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
