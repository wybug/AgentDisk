"""Test share API."""


def test_share_lifecycle(client):
    f = client.files.upload_bytes("share-test.txt", b"share test", folder_id=0)
    try:
        share = client.shares.create(
            resource_id=f.id,
            res_type="file",
            expire_hours=1,
        )
        assert share.id > 0
        assert share.share_code != ""

        shares = client.shares.list()
        assert any(s.id == share.id for s in shares)

        public_share = client.shares.get_by_code(share.share_code)
        assert public_share.share_code == share.share_code

        accessed = client.shares.access(share.share_code)
        assert accessed.share_code == share.share_code

        client.shares.revoke(share.id)
    finally:
        client.files.delete(f.id)
