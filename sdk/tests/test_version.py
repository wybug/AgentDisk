"""Test version API."""


def test_list_versions(client):
    client.upload_bytes("ver-test.txt", b"version 1")
    try:
        client.update_file_bytes("ver-test.txt", b"version 2")

        versions = client.list_versions("ver-test.txt")
        assert len(versions) >= 1
        assert versions[0].version == 1

        client.rollback_version("ver-test.txt", 1)
        detail = client.get_file("ver-test.txt")
        assert detail.file.version == 1
    finally:
        client.delete_file("ver-test.txt")
