"""Closed-loop tests for public directory file management and authorization."""

from __future__ import annotations

from typing import TYPE_CHECKING

import pytest

from agentdisk.exceptions import PermissionDeniedError

if TYPE_CHECKING:
    from agentdisk import AgentDiskClient

DIR_NAME = "sdk-test-public"


class TestAPIKeyFileCRUD:
    """API Key manages public directory files through AgentDiskClient (display_name path)."""

    def test_upload_file(self, api_key_client: AgentDiskClient, test_public_dir: dict) -> None:
        result = api_key_client.upload_bytes(
            f"{DIR_NAME}/test-upload.txt",
            b"hello public directory",
            content_type="text/plain",
        )
        assert result.id > 0
        assert result.file_name == "test-upload.txt"

    def test_list_files(self, api_key_client: AgentDiskClient, test_public_dir: dict) -> None:
        files = api_key_client.list_files(DIR_NAME)
        assert len(files) >= 1
        names = [f.file_name for f in files]
        assert "test-upload.txt" in names

    def test_download_file(self, api_key_client: AgentDiskClient, test_public_dir: dict) -> None:
        result = api_key_client.download_file(f"{DIR_NAME}/test-upload.txt")
        assert result.download_url

    def test_create_folder(self, api_key_client: AgentDiskClient, test_public_dir: dict) -> None:
        folder = api_key_client.create_folder(f"{DIR_NAME}/subfolder")
        assert folder.id > 0
        assert folder.folder_name == "subfolder"

    def test_upload_to_subfolder(self, api_key_client: AgentDiskClient, test_public_dir: dict) -> None:
        result = api_key_client.upload_bytes(
            f"{DIR_NAME}/subfolder/nested.txt",
            b"nested content",
            content_type="text/plain",
        )
        assert result.id > 0

    def test_delete_file(self, api_key_client: AgentDiskClient, test_public_dir: dict) -> None:
        api_key_client.upload_bytes(
            f"{DIR_NAME}/to-delete.txt",
            b"delete me",
            content_type="text/plain",
        )
        files_before = api_key_client.list_files(DIR_NAME)
        assert "to-delete.txt" in [f.file_name for f in files_before]

        api_key_client.delete_file(f"{DIR_NAME}/to-delete.txt")

        files_after = api_key_client.list_files(DIR_NAME)
        assert "to-delete.txt" not in [f.file_name for f in files_after]

    def test_create_share_blocked(self, api_key_client: AgentDiskClient, test_public_dir: dict) -> None:
        with pytest.raises(PermissionDeniedError):
            api_key_client.create_share(f"{DIR_NAME}/test-upload.txt", is_file=True)


class TestAPIKeyBlockedFromPrivate:
    """API Key cannot access private endpoints."""

    def test_cannot_list_private_files(self, api_key_client: AgentDiskClient) -> None:
        with pytest.raises(PermissionDeniedError):
            api_key_client.list_files("/")

    def test_cannot_create_private_folder(self, api_key_client: AgentDiskClient) -> None:
        with pytest.raises(PermissionDeniedError):
            api_key_client.create_folder("private-folder")

    def test_cannot_upload_private_file(self, api_key_client: AgentDiskClient) -> None:
        with pytest.raises(PermissionDeniedError):
            api_key_client.upload_bytes("private-file.txt", b"should fail", content_type="text/plain")

    def test_cannot_delete_private_file(self, api_key_client: AgentDiskClient) -> None:
        with pytest.raises(PermissionDeniedError):
            api_key_client.delete_file("nonexistent-file.txt")


class TestAdminGrantManagement:
    """AgentDiskAdminClient manages user authorization for public directories."""

    def test_grant_access(self, admin_client, test_public_dir: dict) -> None:
        admin_client.grant_access(test_public_dir["id"], "sdk-test-user")

    def test_list_granted_users(self, admin_client, test_public_dir: dict) -> None:
        users = admin_client.list_granted_users(test_public_dir["id"])
        assert "sdk-test-user" in users

    def test_grant_idempotent(self, admin_client, test_public_dir: dict) -> None:
        admin_client.grant_access(test_public_dir["id"], "sdk-test-user")


class TestJWTUserReadOnly:
    """JWT user with granted access can read but not write public directory files."""

    def test_jwt_can_list_public_directories(self, client: AgentDiskClient) -> None:
        dirs = client.list_public_directories()
        names = [d.display_name for d in dirs]
        assert DIR_NAME in names

    def test_jwt_can_list_files(self, client: AgentDiskClient) -> None:
        files = client.list_files(DIR_NAME)
        assert len(files) >= 1

    def test_jwt_can_download_file(self, client: AgentDiskClient) -> None:
        result = client.download_file(f"{DIR_NAME}/test-upload.txt")
        assert result.download_url

    def test_jwt_can_create_share(self, client: AgentDiskClient) -> None:
        share = client.create_share(f"{DIR_NAME}/test-upload.txt", is_file=True)
        assert share.id > 0
        client.revoke_share(share.id)

    def test_jwt_cannot_upload_file(self, client: AgentDiskClient) -> None:
        with pytest.raises(PermissionDeniedError):
            client.upload_bytes(
                f"{DIR_NAME}/jwt-upload.txt",
                b"should fail",
                content_type="text/plain",
            )

    def test_jwt_cannot_create_folder(self, client: AgentDiskClient) -> None:
        with pytest.raises(PermissionDeniedError):
            client.create_folder(f"{DIR_NAME}/jwt-folder")

    def test_jwt_cannot_delete_file(self, client: AgentDiskClient) -> None:
        with pytest.raises(PermissionDeniedError):
            client.delete_file(f"{DIR_NAME}/test-upload.txt")

    def test_jwt_can_list_folders(self, client: AgentDiskClient) -> None:
        folders = client.list_folders(DIR_NAME)
        assert len(folders) >= 1
        assert "subfolder" in [f.folder_name for f in folders]

    def test_jwt_can_access_subfolder_files(self, client: AgentDiskClient) -> None:
        files = client.list_files(f"{DIR_NAME}/subfolder")
        assert len(files) >= 1


class TestCleanup:
    """Cleanup: revoke grant."""

    def test_revoke_access(self, admin_client, test_public_dir: dict) -> None:
        admin_client.revoke_access(test_public_dir["id"], "sdk-test-user")
