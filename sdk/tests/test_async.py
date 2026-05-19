"""Test async client."""

import pytest
from agentdisk import AsyncAgentDiskClient
from agentdisk.models import UserDisk


@pytest.mark.asyncio
async def test_async_space(async_client):
    async with async_client:
        space = await async_client.space.get()
        assert isinstance(space, UserDisk)


@pytest.mark.asyncio
async def test_async_folder_crud(async_client):
    async with async_client:
        f = await async_client.folders.create("async-test-folder")
        assert f.id > 0

        folders = await async_client.folders.list(0)
        assert any(fld.id == f.id for fld in folders)

        got = await async_client.folders.get(f.id)
        assert got.id == f.id

        await async_client.folders.delete(f.id)


@pytest.mark.asyncio
async def test_async_file_upload(async_client):
    async with async_client:
        f = await async_client.files.upload_bytes("async-test.txt", b"async content", folder_id=0)
        assert f.id > 0

        detail = await async_client.files.get(f.id)
        assert detail.file.id == f.id

        await async_client.files.delete(f.id)
