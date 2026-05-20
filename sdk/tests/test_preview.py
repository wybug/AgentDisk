"""Test preview API."""


def test_preview_file(client):
    client.upload_bytes("preview-test.md", b"# Hello\npreview test")
    try:
        result = client.preview("preview-test.md")
        assert result.file_type != ""
        assert result.url != ""
    finally:
        client.delete_file("preview-test.md")
