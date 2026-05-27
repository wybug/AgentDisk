"""Test file API."""

import os
import tempfile

import pytest


@pytest.fixture(scope="module")
def folder(client):
    f = client.create_folder("sdk-file-test")
    yield f
    try:
        client.delete_folder("sdk-file-test")
    except Exception:
        pass


@pytest.fixture(scope="module")
def uploaded_file(client, folder):
    with tempfile.NamedTemporaryFile(suffix=".txt", delete=False, mode="w") as f:
        f.write("sdk test content")
        path = f.name
    try:
        result = client.upload_file("sdk-file-test/test-upload.txt", path)
        yield result
    finally:
        os.unlink(path)
        try:
            client.delete_file("sdk-file-test/test-upload.txt")
        except Exception:
            pass


def test_upload_file(uploaded_file):
    assert uploaded_file.id > 0
    assert uploaded_file.file_name.endswith(".txt")


def test_list_files(client, folder, uploaded_file):
    files = client.list_files("sdk-file-test")
    assert any(f.id == uploaded_file.id for f in files)


def test_get_file(client, uploaded_file):
    detail = client.get_file("sdk-file-test/test-upload.txt")
    assert detail.file.id == uploaded_file.id
    assert detail.url != ""


def test_upload_bytes(client, folder):
    result = client.upload_bytes("sdk-file-test/test-bytes.txt", b"hello from bytes")
    assert result.id > 0
    client.delete_file("sdk-file-test/test-bytes.txt")


def test_update_file(client, folder, uploaded_file):
    with tempfile.NamedTemporaryFile(suffix=".txt", delete=False, mode="w") as f:
        f.write("updated content v2")
        path = f.name
    try:
        result = client.update_file("sdk-file-test/test-upload.txt", path)
        assert result.version == 2
    finally:
        os.unlink(path)


def test_download_file(client, folder):
    client.upload_bytes("sdk-file-test/test-download.txt", b"download test content")
    try:
        result = client.download_file("sdk-file-test/test-download.txt")
        assert result.download_url != ""
        assert result.file.file_name == "test-download.txt"
    finally:
        client.delete_file("sdk-file-test/test-download.txt")


def test_download_file_to(client, folder, tmp_path):
    client.upload_bytes("sdk-file-test/test-download-to.txt", b"save to disk content")
    try:
        local = str(tmp_path / "downloaded.txt")
        returned = client.download_file_to("sdk-file-test/test-download-to.txt", local)
        assert returned == local
        with open(local) as f:
            assert f.read() == "save to disk content"
    finally:
        client.delete_file("sdk-file-test/test-download-to.txt")
