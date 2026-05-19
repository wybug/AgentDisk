"""Test space API."""

from agentdisk.models import UserDisk


def test_get_space(client):
    space = client.space.get()
    assert isinstance(space, UserDisk)
    assert space.user_id == "sdk-test-user"
    assert space.total_quota >= 0
