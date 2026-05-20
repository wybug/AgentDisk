"""Test async client."""

import pytest

from agentdisk.models import UserDisk


@pytest.mark.asyncio
async def test_async_space(async_client):
    async with async_client:
        space = await async_client.get_space()
        assert isinstance(space, UserDisk)


@pytest.mark.asyncio
async def test_async_folder_crud(async_client):
    async with async_client:
        f = await async_client.create_folder("async-test-folder")
        assert f.id > 0

        folders = await async_client.list_folders("/")
        assert any(fld.id == f.id for fld in folders)

        got = await async_client.get_folder("async-test-folder")
        assert got.id == f.id

        await async_client.delete_folder("async-test-folder")


@pytest.mark.asyncio
async def test_async_file_upload(async_client):
    async with async_client:
        f = await async_client.upload_bytes("async-test.txt", b"async content")
        assert f.id > 0

        detail = await async_client.get_file("async-test.txt")
        assert detail.file.id == f.id

        await async_client.delete_file("async-test.txt")
