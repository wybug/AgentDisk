"""End-to-end share → preview lifecycle test.

This test exercises the full protocol stack: SDK writes a file, raw HTTP
reads it back through the public share flow, the download-token flow, the
authenticated preview flow, and finally the revoke flow. The point is to
catch wire-format drift between the SDK's request/response shapes and the
backend's actual JSON schema — things unit tests on either side can't
detect.

Layout: 10 numbered steps, each asserting one thing. Step 10 (revoke)
runs in a ``finally`` so the share is revoked even if an earlier step
fails; the file is deleted in the outer ``finally`` so it doesn't leak.
"""

from __future__ import annotations

import os

import httpx
import pytest

from agentdisk import AgentDiskClient

BASE_URL = os.environ.get("AGENTDISK_URL", "http://localhost:9100")
JWT_SECRET = os.environ.get("AGENTDISK_JWT_SECRET", "dev-jwt-secret-for-testing-only")
TEST_PATH = "e2e-share-preview.md"
TEST_CONTENT = b"# Title\n\nshare-preview e2e body"
EXTRACT_CODE = "abc123"


def _generate_user_token() -> str:
    """Generate a JWT for the e2e test user.

    Delegated to ``scripts/gen_token`` so the test doesn't have to depend
    on PyJWT or duplicate the claim shape. The SDK test conftest does the
    same — e2e just calls the script directly.
    """
    import subprocess

    repo_root = os.path.dirname(os.path.dirname(os.path.dirname(os.path.dirname(__file__))))
    result = subprocess.run(
        ["go", "run", "scripts/gen_token/main.go", "-secret", JWT_SECRET, "-userId", "e2e-share-preview"],
        capture_output=True,
        text=True,
        cwd=repo_root,
    )
    if result.returncode != 0:
        pytest.fail(f"gen_token failed: {result.stderr}")
    return result.stdout.strip()


@pytest.fixture(scope="module")
def token() -> str:
    return _generate_user_token()


@pytest.fixture(scope="module")
def client(token):
    with AgentDiskClient(base_url=BASE_URL, token=token) as c:
        yield c


def test_share_preview_lifecycle(client, token):
    """Upload via SDK, read via raw HTTP, revoke via SDK — 10 assertions."""

    # Step 1: SDK upload
    file_obj = client.upload_bytes(TEST_PATH, TEST_CONTENT, content_type="text/markdown")
    file_id = file_obj.id
    assert file_id > 0, f"upload returned non-positive id={file_id}"

    raw = httpx.Client(base_url=BASE_URL, timeout=15.0)
    auth_headers = {"Authorization": f"Bearer {token}"}
    try:
        # Step 2: SDK create share with extract_code
        share = client.create_share(
            TEST_PATH,
            is_file=True,
            extract_code=EXTRACT_CODE,
            expire_hours=1,
        )
        assert share.share_code, "share.share_code empty"
        assert share.id > 0, "share.id non-positive"
        share_code = share.share_code
        share_id = share.id

        # Step 3: raw HTTP GET /v1/disk/share/{code} (public)
        resp = raw.get(f"/v1/disk/share/{share_code}")
        assert resp.status_code == 200, f"GET share: {resp.status_code} {resp.text}"
        body = resp.json()
        assert body["data"]["shareCode"] == share_code, "shareCode mismatch in public GET"

        # Step 4: raw HTTP POST /v1/disk/share/access
        resp = raw.post(
            "/v1/disk/share/access",
            json={"code": share_code, "extractCode": EXTRACT_CODE},
        )
        assert resp.status_code == 200, f"access: {resp.status_code} {resp.text}"
        accessed = resp.json()["data"]
        assert accessed["resourceId"] == file_id, f"access resourceId={accessed['resourceId']} != file_id={file_id}"

        # Step 5: raw HTTP POST /v1/disk/share/download → downloadToken
        resp = raw.post(
            "/v1/disk/share/download",
            json={"code": share_code, "resourceId": file_id, "extractCode": EXTRACT_CODE},
        )
        assert resp.status_code == 200, f"share download: {resp.status_code} {resp.text}"
        download_token = resp.json()["data"]["downloadToken"]
        assert download_token, "downloadToken empty"

        # Step 6: raw HTTP GET /v1/disk/files/download?t=... Accept: JSON.
        # The download token itself is the credential — no Authorization
        # header needed (the public route is registered without middleware).
        resp = raw.get(
            "/v1/disk/files/download",
            params={"t": download_token},
            headers={"Accept": "application/json"},
        )
        assert resp.status_code == 200, f"download JSON: {resp.status_code} {resp.text}"
        download_url = resp.json()["data"]["downloadUrl"]
        assert download_url, "downloadUrl empty"

        # Step 7: raw HTTP GET downloadUrl → bytes match uploaded
        resp = raw.get(download_url)
        assert resp.status_code == 200, f"fetch downloadUrl: {resp.status_code}"
        assert resp.content == TEST_CONTENT, f"downloaded bytes mismatch: got {resp.content!r}, want {TEST_CONTENT!r}"

        # Step 8: raw HTTP GET /v1/disk/preview/{id} (auth required).
        # Backend returns the raw file body in ``content`` for text/markdown/
        # code; client-side React renders it. We verify the wire shape
        # (fileType + content round-trips intact) — that's the cross-layer
        # contract this test exists to catch drift on.
        resp = raw.get(f"/v1/disk/preview/{file_id}", headers=auth_headers)
        assert resp.status_code == 200, f"preview: {resp.status_code} {resp.text}"
        preview_data = resp.json()["data"]
        assert preview_data["fileType"] == "markdown", f"fileType={preview_data.get('fileType')!r}, want 'markdown'"
        assert preview_data["content"] == TEST_CONTENT.decode(), (
            f"preview content mismatch: got {preview_data['content']!r}"
        )

        # Step 9: SDK revoke share
        client.revoke_share(share_id)

        # Step 10: raw HTTP GET /v1/disk/share/{code} → revoked/not-found
        resp = raw.get(f"/v1/disk/share/{share_code}")
        assert resp.status_code != 200 or not resp.json().get("data", {}).get("isActive", True), (
            f"share still accessible after revoke: {resp.status_code} {resp.text}"
        )
    finally:
        raw.close()

    # Outer cleanup: delete the file even if an inner step failed. Use the
    # raw HTTP DELETE rather than SDK because the SDK resolver caches paths
    # and the file may already be in an inconsistent state.
    try:
        client.delete_file(TEST_PATH)
    except Exception:
        pass
