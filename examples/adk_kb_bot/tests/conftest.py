"""Pytest fixtures for adk_kb_bot.

Most tests mock the SDK client + httpx, so the fixtures here mainly
ensure env vars are set to deterministic values and the kb_client
singleton cache is wiped between tests (otherwise the first test's
client lingers and the next test's monkeypatched env is ignored).
"""

from __future__ import annotations

import pytest


@pytest.fixture(autouse=True)
def _reset_client_cache() -> None:
    """Clear ``functools.lru_cache`` on ``get_client`` between tests.

    Without this, the first test that builds a real client would
    cache it, and subsequent tests that monkeypatch env vars would
    see the stale client. The teardown runs after the test so the
    cache is also clean for the next test in the same process.
    """
    # Import here so the fixture is robust to import-order quirks.
    from adk_kb_bot import kb_client

    kb_client.reset_client_cache()
    yield
    kb_client.reset_client_cache()


@pytest.fixture()
def configured_env(monkeypatch: pytest.MonkeyPatch) -> None:
    """Set the standard env var set the bot expects in production."""
    monkeypatch.setenv("AGENTDISK_BASE_URL", "http://test.local:8080")
    monkeypatch.setenv("AGENTDISK_API_KEY", "adk_test_key")
    monkeypatch.setenv("KB_BOT_BUNDLE_ID", "42")
    monkeypatch.setenv("AGENTDISK_PUBLIC_DIRECTORY_ID", "7")
