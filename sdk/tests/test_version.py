"""Test version API."""

import tempfile
import os


def test_list_versions(client):
    f = client.files.upload_bytes("ver-test.txt", b"version 1", folder_id=0)
    try:
        client.files.update_bytes(f.id, "ver-test.txt", b"version 2")

        versions = client.versions.list(f.id)
        assert len(versions) >= 1
        assert versions[0].version == 1

        client.versions.rollback(f.id, 1)
        detail = client.files.get(f.id)
        assert detail.file.version == 1
    finally:
        client.files.delete(f.id)
