"""Test tag API."""


def test_tag_lifecycle(client):
    f = client.files.upload_bytes("tag-test.txt", b"tag test", folder_id=0)
    try:
        client.tags.bind(f.id, "sdk-test-tag")

        results = client.tags.search(["sdk-test-tag"])
        assert any(rf.id == f.id for rf in results)

        client.tags.unbind(f.id, "sdk-test-tag")

        results2 = client.tags.search(["sdk-test-tag"])
        assert not any(rf.id == f.id for rf in results2)
    finally:
        client.files.delete(f.id)
