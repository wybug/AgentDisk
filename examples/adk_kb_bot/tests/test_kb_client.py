"""Unit tests for the SDK wrapper layer in :mod:`adk_kb_bot.kb_client`.

The SDK's own test suite covers the wire protocol; here we only verify
the bot's wrapper logic:

* ``get_client()`` reads env vars lazily and caches the singleton.
* ``default_bundle_id()`` validates env.
* ``default_public_directory_id()`` reads the numeric PD id (writer-same
  config); legacy ``KB_BOT_PUBLIC_DIRECTORY_NAME`` falls back through
  ``list_public_directories()``.
* ``read_node_body()`` builds the right path and GETs the presigned URL.
"""

from __future__ import annotations

from dataclasses import dataclass
from unittest.mock import MagicMock, patch

import pytest

from adk_kb_bot import kb_client


@dataclass
class _FakePD:
    id: int
    display_name: str
    fixed_path: str


# ----------------------------------------------------------------------
# get_client
# ----------------------------------------------------------------------
def test_get_client_builds_from_env(configured_env: None) -> None:
    """Client is built lazily from BASE_URL + API_KEY env vars."""
    client = kb_client.get_client()
    assert client is not None
    # Same call returns the cached instance.
    assert kb_client.get_client() is client


def test_get_client_missing_base_url(monkeypatch: pytest.MonkeyPatch) -> None:
    monkeypatch.setenv("AGENTDISK_API_KEY", "adk_x")
    monkeypatch.delenv("AGENTDISK_BASE_URL", raising=False)
    with pytest.raises(RuntimeError, match="AGENTDISK_BASE_URL"):
        kb_client.get_client()


def test_get_client_missing_api_key(monkeypatch: pytest.MonkeyPatch) -> None:
    monkeypatch.setenv("AGENTDISK_BASE_URL", "http://x")
    monkeypatch.delenv("AGENTDISK_API_KEY", raising=False)
    with pytest.raises(RuntimeError, match="AGENTDISK_API_KEY"):
        kb_client.get_client()


def test_get_client_cache_clear(monkeypatch: pytest.MonkeyPatch) -> None:
    """After cache_clear, a new env value is picked up."""
    monkeypatch.setenv("AGENTDISK_BASE_URL", "http://first")
    monkeypatch.setenv("AGENTDISK_API_KEY", "k1")
    first = kb_client.get_client()
    monkeypatch.setenv("AGENTDISK_BASE_URL", "http://second")
    kb_client.reset_client_cache()
    second = kb_client.get_client()
    assert first is not second


# ----------------------------------------------------------------------
# default_bundle_id
# ----------------------------------------------------------------------
def test_default_bundle_id_reads_env(configured_env: None) -> None:
    assert kb_client.default_bundle_id() == 42


def test_default_bundle_id_missing(monkeypatch: pytest.MonkeyPatch) -> None:
    monkeypatch.delenv("KB_BOT_BUNDLE_ID", raising=False)
    with pytest.raises(RuntimeError, match="KB_BOT_BUNDLE_ID"):
        kb_client.default_bundle_id()


@pytest.mark.parametrize("bad", ["", "0", "-1", "abc", "12.5"])
def test_default_bundle_id_rejects_invalid(
    monkeypatch: pytest.MonkeyPatch, bad: str
) -> None:
    monkeypatch.setenv("KB_BOT_BUNDLE_ID", bad)
    with pytest.raises(RuntimeError):
        kb_client.default_bundle_id()


# ----------------------------------------------------------------------
# default_public_directory_id
# ----------------------------------------------------------------------
def test_default_pd_id_reads_numeric_env(configured_env: None) -> None:
    """AGENTDISK_PUBLIC_DIRECTORY_ID (numeric) is the preferred config —
    mirrors the writer's .env.example."""
    assert kb_client.default_public_directory_id() == 7


def test_default_pd_id_missing(monkeypatch: pytest.MonkeyPatch) -> None:
    """Without either var, fail with a message that points at the writer."""
    monkeypatch.delenv("AGENTDISK_PUBLIC_DIRECTORY_ID", raising=False)
    monkeypatch.delenv("KB_BOT_PUBLIC_DIRECTORY_NAME", raising=False)
    with pytest.raises(RuntimeError, match="AGENTDISK_PUBLIC_DIRECTORY_ID"):
        kb_client.default_public_directory_id()


@pytest.mark.parametrize("bad", ["", "0", "-1", "abc", "1.5"])
def test_default_pd_id_rejects_invalid(
    monkeypatch: pytest.MonkeyPatch, bad: str
) -> None:
    monkeypatch.setenv("AGENTDISK_PUBLIC_DIRECTORY_ID", bad)
    with pytest.raises(RuntimeError):
        kb_client.default_public_directory_id()


def test_default_pd_id_legacy_name_falls_back(
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    """KB_BOT_PUBLIC_DIRECTORY_NAME still works — resolved to id via SDK."""
    monkeypatch.setenv("AGENTDISK_BASE_URL", "http://x")
    monkeypatch.setenv("AGENTDISK_API_KEY", "k")
    monkeypatch.setenv("KB_BOT_BUNDLE_ID", "1")
    monkeypatch.delenv("AGENTDISK_PUBLIC_DIRECTORY_ID", raising=False)
    monkeypatch.setenv("KB_BOT_PUBLIC_DIRECTORY_NAME", "OKF Demo")

    fake_client = MagicMock()
    fake_client.list_public_directories.return_value = [
        _FakePD(id=66, display_name="OKF Demo", fixed_path="/public/okf-demo"),
        _FakePD(id=77, display_name="other", fixed_path="/public/other"),
    ]
    with patch.object(kb_client, "get_client", return_value=fake_client):
        pd_id = kb_client.default_public_directory_id()
    assert pd_id == 66


def test_default_pd_id_legacy_name_unmatched(
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    """Legacy name that matches no visible PD surfaces a clear error."""
    monkeypatch.setenv("AGENTDISK_BASE_URL", "http://x")
    monkeypatch.setenv("AGENTDISK_API_KEY", "k")
    monkeypatch.setenv("KB_BOT_BUNDLE_ID", "1")
    monkeypatch.delenv("AGENTDISK_PUBLIC_DIRECTORY_ID", raising=False)
    monkeypatch.setenv("KB_BOT_PUBLIC_DIRECTORY_NAME", "ghost")

    fake_client = MagicMock()
    fake_client.list_public_directories.return_value = [
        _FakePD(id=66, display_name="other", fixed_path="/public/other"),
    ]
    with patch.object(kb_client, "get_client", return_value=fake_client):
        with pytest.raises(RuntimeError, match="ghost"):
            kb_client.default_public_directory_id()


# ----------------------------------------------------------------------
# default_public_dir_name
# ----------------------------------------------------------------------
def test_default_pd_name_resolves_display_name(configured_env: None) -> None:
    """displayName is resolved from the SDK by PD id and cached."""
    fake_client = MagicMock()
    fake_client.list_public_directories.return_value = [
        _FakePD(id=7, display_name="test", fixed_path="/public/test"),
        _FakePD(id=8, display_name="other", fixed_path="/public/other"),
    ]
    with patch.object(kb_client, "get_client", return_value=fake_client):
        assert kb_client.default_public_dir_name() == "test"


def test_default_pd_name_unknown_id(configured_env: None) -> None:
    """PD id that isn't visible surfaces a clear error."""
    fake_client = MagicMock()
    fake_client.list_public_directories.return_value = [
        _FakePD(id=99, display_name="other", fixed_path="/public/other"),
    ]
    with patch.object(kb_client, "get_client", return_value=fake_client):
        with pytest.raises(RuntimeError, match="id=7"):
            kb_client.default_public_dir_name()


def test_default_pd_name_cached(configured_env: None) -> None:
    """Resolving twice hits the SDK only once — the cache is shared with
    the singleton client."""
    fake_client = MagicMock()
    fake_client.list_public_directories.return_value = [
        _FakePD(id=7, display_name="test", fixed_path="/public/test"),
    ]
    with patch.object(kb_client, "get_client", return_value=fake_client):
        kb_client.default_public_dir_name()
        kb_client.default_public_dir_name()
    assert fake_client.list_public_directories.call_count == 1


# ----------------------------------------------------------------------
# read_node_body
# ----------------------------------------------------------------------
def test_read_node_body_builds_path_and_fetches(
    configured_env: None,
) -> None:
    """read_node_body should:
    1. Resolve displayName from AGENTDISK_PUBLIC_DIRECTORY_ID via SDK.
    2. Prefix the rel_path with the displayName.
    3. Call client.download_file(<name>/<rel>) on the SDK.
    4. GET the returned download_url via the SDK's internal httpx client
       (which has base_url configured, so relative URLs work).
    """
    fake_client = MagicMock()
    fake_client.list_public_directories.return_value = [
        _FakePD(id=7, display_name="test", fixed_path="/public/test"),
    ]
    fake_client.download_file.return_value.download_url = "http://cdn/presigned"
    fake_resp = MagicMock()
    fake_resp.text = "---\ntype: law\n---\n\n# Body\n"
    fake_resp.raise_for_status.return_value = None
    fake_client._http.get.return_value = fake_resp

    with patch.object(kb_client, "get_client", return_value=fake_client):
        body = kb_client.read_node_body("机构监管/反洗钱法.md")

    assert "type: law" in body
    fake_client.download_file.assert_called_once_with("test/机构监管/反洗钱法.md")
    fake_client._http.get.assert_called_once_with(
        "http://cdn/presigned", timeout=30.0
    )


def test_read_node_body_handles_relative_download_url(
    configured_env: None,
) -> None:
    """SDK may return a relative download_url (e.g. /v1/disk/local-storage/...).

    The SDK's httpx client has base_url configured, so we route the GET
    through ``client._http`` instead of a bare ``httpx.get`` to handle this.
    """
    fake_client = MagicMock()
    fake_client.list_public_directories.return_value = [
        _FakePD(id=7, display_name="test", fixed_path="/public/test"),
    ]
    fake_client.download_file.return_value.download_url = "/v1/disk/local-storage/foo?sig=x"
    fake_resp = MagicMock()
    fake_resp.text = "body content"
    fake_resp.raise_for_status.return_value = None
    fake_client._http.get.return_value = fake_resp

    with patch.object(kb_client, "get_client", return_value=fake_client):
        body = kb_client.read_node_body("concepts/foo.md")

    assert body == "body content"
    fake_client._http.get.assert_called_once_with(
        "/v1/disk/local-storage/foo?sig=x", timeout=30.0
    )


def test_read_node_body_strips_leading_slash(
    configured_env: None,
) -> None:
    """A rel_path with a leading slash should not create a double slash."""
    fake_client = MagicMock()
    fake_client.list_public_directories.return_value = [
        _FakePD(id=7, display_name="test", fixed_path="/public/test"),
    ]
    fake_client.download_file.return_value.download_url = "http://cdn/x"
    fake_resp = MagicMock()
    fake_resp.text = "body"
    fake_resp.raise_for_status.return_value = None
    fake_client._http.get.return_value = fake_resp

    with patch.object(kb_client, "get_client", return_value=fake_client):
        kb_client.read_node_body("/concepts/foo.md")

    fake_client.download_file.assert_called_once_with("test/concepts/foo.md")


def test_read_node_body_missing_pd_id(monkeypatch: pytest.MonkeyPatch) -> None:
    """Without AGENTDISK_PUBLIC_DIRECTORY_ID, the call fails fast."""
    monkeypatch.setenv("AGENTDISK_BASE_URL", "http://x")
    monkeypatch.setenv("AGENTDISK_API_KEY", "k")
    monkeypatch.setenv("KB_BOT_BUNDLE_ID", "1")
    monkeypatch.delenv("AGENTDISK_PUBLIC_DIRECTORY_ID", raising=False)
    monkeypatch.delenv("KB_BOT_PUBLIC_DIRECTORY_NAME", raising=False)
    with pytest.raises(RuntimeError, match="AGENTDISK_PUBLIC_DIRECTORY_ID"):
        kb_client.read_node_body("foo.md")
