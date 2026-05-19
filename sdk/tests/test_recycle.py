"""Test recycle bin API."""


def test_recycle_lifecycle(client):
    f = client.files.upload_bytes("recycle-test.txt", b"to recycle", folder_id=0)
    client.files.delete(f.id)

    items = client.recycle.list()
    target = [i for i in items if i.resource_id == f.id]
    assert len(target) >= 1

    recycle_item = target[0]
    client.recycle.restore(recycle_item.id)

    items_after = client.recycle.list()
    assert not any(i.resource_id == f.id for i in items_after)

    client.files.delete(f.id)
    items2 = client.recycle.list()
    target2 = [i for i in items2 if i.resource_id == f.id]
    if target2:
        client.recycle.delete_permanent(target2[0].id)
