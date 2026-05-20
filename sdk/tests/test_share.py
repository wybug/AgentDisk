"""Test share API."""


def test_share_lifecycle(client):
    client.upload_bytes("share-test.txt", b"share test")
    try:
        share = client.create_share(
            "share-test.txt",
            is_file=True,
            expire_hours=1,
        )
        assert share.id > 0
        assert share.share_code != ""

        shares = client.list_shares()
        assert any(s.id == share.id for s in shares)

        public_share = client.get_share_by_code(share.share_code)
        assert public_share.share_code == share.share_code

        accessed = client.access_share(share.share_code)
        assert accessed.share_code == share.share_code

        client.revoke_share(share.id)
    finally:
        client.delete_file("share-test.txt")
