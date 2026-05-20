"""Test file API."""

import os
import tempfile

import pytest


@pytest.fixture(scope="module")
def folder(client):
    f = client.folders.create("sdk-file-test")
    yield f
    try:
        client.folders.delete(f.id)
    except Exception:
        pass


@pytest.fixture(scope="module")
def uploaded_file(client, folder):
    with tempfile.NamedTemporaryFile(suffix=".txt", delete=False, mode="w") as f:
        f.write("sdk test content")
        path = f.name
    try:
        result = client.files.upload(path, folder_id=folder.id)
        yield result
    finally:
        os.unlink(path)
        try:
            client.files.delete(result.id)
        except Exception:
            pass


def test_upload_file(uploaded_file):
    assert uploaded_file.id > 0
    assert uploaded_file.file_name.endswith(".txt")


def test_list_files(client, folder, uploaded_file):
    files = client.files.list(folder.id)
    assert any(f.id == uploaded_file.id for f in files)


def test_get_file(client, uploaded_file):
    detail = client.files.get(uploaded_file.id)
    assert detail.file.id == uploaded_file.id
    assert detail.url != ""


def test_upload_bytes(client, folder):
    result = client.files.upload_bytes("test-bytes.txt", b"hello from bytes", folder_id=folder.id)
    assert result.id > 0
    client.files.delete(result.id)


def test_update_file(client, folder, uploaded_file):
    with tempfile.NamedTemporaryFile(suffix=".txt", delete=False, mode="w") as f:
        f.write("updated content v2")
        path = f.name
    try:
        result = client.files.update(uploaded_file.id, path)
        assert result.version == 2
    finally:
        os.unlink(path)


def test_download_token(client, uploaded_file):
    token_resp = client.files.create_download_token(uploaded_file.id)
    assert token_resp.download_token != ""
    assert token_resp.expires_in > 0

    dl = client.files.download_by_token(token_resp.download_token)
    assert dl.file.id == uploaded_file.id
    assert dl.download_url != ""
