"""E2E conftest.

Pytest auto-discovers conftest.py from parent directories, so the fixtures
defined in ``sdk/tests/conftest.py`` (``client``, ``user_token``, etc.) are
available here without an explicit import. The only e2e-specific fixture is
``backend_reachable``, which pings ``/health`` once per session and skips
the whole e2e module if the backend is down — e2e tests are inherently
integration tests and shouldn't fail CI when the backend isn't running.
"""

from __future__ import annotations

import os

import httpx
import pytest

BASE_URL = os.environ.get("AGENTDISK_URL", "http://localhost:9100")


@pytest.fixture(scope="session", autouse=True)
def backend_reachable():
    try:
        resp = httpx.get(f"{BASE_URL}/health", timeout=2.0)
    except httpx.HTTPError as exc:
        pytest.skip(f"backend at {BASE_URL} not reachable: {exc}")
        return
    if resp.status_code != 200:
        pytest.skip(f"backend /health returned {resp.status_code}")
        return
    yield
