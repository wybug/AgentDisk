"""Test public directory API."""


def test_list_public_directories(client):
    dirs = client.list_public_directories()
    assert isinstance(dirs, list)
    if not dirs:
        return
    d = client.get_public_directory(dirs[0].display_name)
    assert d.id == dirs[0].id
    folders = client.list_public_directory_folders(dirs[0].display_name)
    assert isinstance(folders, list)
    files = client.list_public_directory_files(dirs[0].display_name)
    assert isinstance(files, list)
