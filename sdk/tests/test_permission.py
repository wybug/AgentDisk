"""Test permission API."""

import pytest


@pytest.fixture(scope="module")
def test_file(client):
    result = client.files.upload_bytes("perm-test.txt", b"perm test", folder_id=0)
    yield result
    try:
        client.files.delete(result.id)
    except Exception:
        pass


def test_grant_permission(client, test_file):
    client.permissions.grant(
        agent_id="sdk-test-agent",
        resource_id=test_file.id,
        res_type="file",
        permission="read",
    )


def test_check_permission(client, test_file):
    allowed = client.permissions.check(
        resource_id=test_file.id,
        res_type="file",
        permission="read",
        agent_id="sdk-test-agent",
    )
    assert allowed is True


def test_list_permissions(client, test_file):
    perms = client.permissions.list()
    assert len(perms) >= 1


def test_revoke_permission(client, test_file):
    client.permissions.revoke(
        agent_id="sdk-test-agent",
        resource_id=test_file.id,
        res_type="file",
    )
