"""Unit tests for the SDK wrapper layer in :mod:`adk_kb_bot.kb_client`.

The SDK's own test suite covers the wire protocol; here we only verify
the bot's wrapper logic:

* ``get_client()`` reads env vars lazily and caches the singleton.
* ``default_bundle_id()`` / ``default_public_dir_name()`` validate env.
* ``read_node_body()`` builds the right path and GETs the presigned URL.
"""

from __future__ import annotations

from unittest.mock import MagicMock, patch

import pytest

from adk_kb_bot import kb_client


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
# default_public_dir_name
# ----------------------------------------------------------------------
def test_default_public_dir_name_strips_slashes(configured_env: None) -> None:
    # configured_env sets it to "test"; verify plain passthrough.
    assert kb_client.default_public_dir_name() == "test"


def test_default_public_dir_name_trims_leading_slash(
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    monkeypatch.setenv("KB_BOT_PUBLIC_DIRECTORY_NAME", "/public/test/")
    assert kb_client.default_public_dir_name() == "public/test"


def test_default_public_dir_name_missing(monkeypatch: pytest.MonkeyPatch) -> None:
    monkeypatch.delenv("KB_BOT_PUBLIC_DIRECTORY_NAME", raising=False)
    with pytest.raises(RuntimeError, match="KB_BOT_PUBLIC_DIRECTORY_NAME"):
        kb_client.default_public_dir_name()


# ----------------------------------------------------------------------
# read_node_body
# ----------------------------------------------------------------------
def test_read_node_body_builds_path_and_fetches(
    configured_env: None,
) -> None:
    """read_node_body should:
    1. Prefix the rel_path with the public directory name.
    2. Call client.download_file(<pd>/<rel>) on the SDK.
    3. GET the returned download_url via the SDK's internal httpx client
       (which has base_url configured, so relative URLs work).
    """
    fake_client = MagicMock()
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
    fake_client.download_file.return_value.download_url = "http://cdn/x"
    fake_resp = MagicMock()
    fake_resp.text = "body"
    fake_resp.raise_for_status.return_value = None
    fake_client._http.get.return_value = fake_resp

    with patch.object(kb_client, "get_client", return_value=fake_client):
        kb_client.read_node_body("/concepts/foo.md")

    fake_client.download_file.assert_called_once_with("test/concepts/foo.md")


def test_read_node_body_missing_pd_name(monkeypatch: pytest.MonkeyPatch) -> None:
    """Without KB_BOT_PUBLIC_DIRECTORY_NAME, the call fails fast."""
    monkeypatch.setenv("AGENTDISK_BASE_URL", "http://x")
    monkeypatch.setenv("AGENTDISK_API_KEY", "k")
    monkeypatch.setenv("KB_BOT_BUNDLE_ID", "1")
    monkeypatch.delenv("KB_BOT_PUBLIC_DIRECTORY_NAME", raising=False)
    with pytest.raises(RuntimeError, match="KB_BOT_PUBLIC_DIRECTORY_NAME"):
        kb_client.read_node_body("foo.md")
