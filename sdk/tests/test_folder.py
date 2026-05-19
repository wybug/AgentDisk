"""Test folder API."""

import pytest


@pytest.fixture(scope="module")
def folder(client):
    f = client.folders.create("sdk-test-folder")
    yield f
    try:
        client.folders.delete(f.id)
    except Exception:
        pass


def test_create_folder(folder):
    assert folder.id > 0
    assert folder.folder_name == "sdk-test-folder"
    assert folder.parent_id == 0


def test_list_folders(client, folder):
    folders = client.folders.list(0)
    assert any(f.id == folder.id for f in folders)


def test_get_folder(client, folder):
    f = client.folders.get(folder.id)
    assert f.id == folder.id
    assert f.folder_name == "sdk-test-folder"


def test_rename_folder(client, folder):
    f = client.folders.rename(folder.id, "sdk-renamed")
    assert f.folder_name == "sdk-renamed"


def test_ancestors(client, folder):
    ancestors = client.folders.ancestors(folder.id)
    assert isinstance(ancestors, list)


def test_create_subfolder(client, folder):
    sub = client.folders.create("sdk-subfolder", parent_id=folder.id)
    assert sub.parent_id == folder.id
    ancestors = client.folders.ancestors(sub.id)
    assert any(a.id == folder.id for a in ancestors)
    client.folders.delete(sub.id)
