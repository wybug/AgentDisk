"""Test recycle bin API."""


def test_recycle_lifecycle(client):
    f = client.upload_bytes("recycle-test.txt", b"to recycle")
    file_id = f.id
    client.delete_file("recycle-test.txt")
    client.clear_cache()

    items = client.list_recycle()
    target = [i for i in items if i.resource_id == file_id]
    assert len(target) >= 1

    recycle_item = target[0]
    client.restore(recycle_item.id)

    items_after = client.list_recycle()
    assert not any(i.resource_id == file_id for i in items_after)

    client.delete_file("recycle-test.txt")
    items2 = client.list_recycle()
    target2 = [i for i in items2 if i.resource_id == file_id]
    if target2:
        client.delete_permanent(target2[0].id)
