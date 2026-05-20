"""Test permission API."""

import pytest


@pytest.fixture(scope="module")
def test_file(client):
    result = client.upload_bytes("perm-test.txt", b"perm test")
    yield result
    try:
        client.delete_file("perm-test.txt")
    except Exception:
        pass


def test_grant_permission(client, test_file):
    client.grant_permission(
        "perm-test.txt",
        agent_id="sdk-test-agent",
        permission="read",
        is_file=True,
    )


def test_check_permission(client, test_file):
    allowed = client.check_permission(
        "perm-test.txt",
        "read",
        is_file=True,
        agent_id="sdk-test-agent",
    )
    assert allowed is True


def test_list_permissions(client, test_file):
    perms = client.list_permissions()
    assert len(perms) >= 1


def test_revoke_permission(client, test_file):
    client.revoke_permission(
        "perm-test.txt",
        "sdk-test-agent",
        is_file=True,
    )
