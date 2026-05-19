"""Test preview API."""


def test_preview_file(client):
    f = client.files.upload_bytes("preview-test.md", b"# Hello\npreview test", folder_id=0)
    try:
        result = client.preview.file(f.id)
        assert result.file_type != ""
        assert result.url != ""
    finally:
        client.files.delete(f.id)
