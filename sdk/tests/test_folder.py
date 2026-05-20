"""Test folder API."""

import pytest


@pytest.fixture(scope="module")
def folder(client):
    f = client.create_folder("sdk-test-folder")
    yield f
    try:
        client.delete_folder("sdk-test-folder")
    except Exception:
        pass


def test_create_folder(folder):
    assert folder.id > 0
    assert folder.folder_name == "sdk-test-folder"
    assert folder.parent_id == 0


def test_list_folders(client, folder):
    folders = client.list_folders("/")
    assert any(f.id == folder.id for f in folders)


def test_get_folder(client, folder):
    f = client.get_folder("sdk-test-folder")
    assert f.id == folder.id
    assert f.folder_name == "sdk-test-folder"


def test_rename_folder(client, folder):
    f = client.rename_folder("sdk-test-folder", "sdk-renamed")
    assert f.folder_name == "sdk-renamed"


def test_create_subfolder(client, folder):
    sub = client.create_folder("sdk-renamed/sdk-subfolder")
    assert sub.parent_id == folder.id
    client.delete_folder("sdk-renamed/sdk-subfolder")
