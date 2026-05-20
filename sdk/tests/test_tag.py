"""Test tag API."""


def test_tag_lifecycle(client):
    f = client.upload_bytes("tag-test.txt", b"tag test")
    try:
        client.bind_tag("tag-test.txt", "sdk-test-tag")

        results = client.search_files(["sdk-test-tag"])
        assert any(rf.id == f.id for rf in results)

        client.unbind_tag("tag-test.txt", "sdk-test-tag")

        results2 = client.search_files(["sdk-test-tag"])
        assert not any(rf.id == f.id for rf in results2)
    finally:
        client.delete_file("tag-test.txt")
