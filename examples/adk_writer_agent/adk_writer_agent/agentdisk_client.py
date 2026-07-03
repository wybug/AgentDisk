"""AgentDisk OKF v0.1 HTTP client.

A lightweight HTTP wrapper around the AgentDisk OKF REST endpoints. Uses httpx
and the unified ``{code, message, data}`` response envelope. Intentionally
does NOT depend on the AgentDisk Python SDK: the goal is to prove the OKF API
is open at the protocol level.
"""

from __future__ import annotations

from typing import Any

import httpx

# Successful business code in AgentDisk's unified response envelope.
# Backend currently uses 0 to indicate success.
_OK_CODES: tuple[int, ...] = (0, 200)


class AgentDiskError(RuntimeError):
    """Raised when AgentDisk returns a non-success business code or HTTP error.

    Attributes:
        message: Human-readable error message from the server.
        status_code: HTTP status code of the failed response (if available).
        code: Business code from the response body (if available).
    """

    def __init__(
        self,
        message: str,
        *,
        status_code: int | None = None,
        code: int | None = None,
    ) -> None:
        super().__init__(message)
        self.message = message
        self.status_code = status_code
        self.code = code

    def __str__(self) -> str:
        parts: list[str] = [self.message]
        if self.status_code is not None:
            parts.append(f"http={self.status_code}")
        if self.code is not None:
            parts.append(f"code={self.code}")
        return " | ".join(parts)


class AgentDiskClient:
    """Typed HTTP client for the 6 OKF maintenance APIs.

    The client keeps a long-lived :class:`httpx.Client` and reuses the bearer
    token across requests. Pass ``base_url`` and ``api_key`` explicitly, or
    read them from environment variables via :meth:`from_env`.
    """

    def __init__(
        self,
        base_url: str,
        api_key: str,
        *,
        timeout: float = 30.0,
    ) -> None:
        self._base_url = base_url.rstrip("/")
        self._api_key = api_key
        # API key rides via X-API-Key because the backend's HybridAuth
        # middleware treats `Authorization: Bearer ...` as JWT-only and
        # falls through to 401 when the JWT parse fails. An API key is
        # never a valid JWT, so sending it as Bearer never authenticated.
        self._client = httpx.Client(
            base_url=self._base_url,
            headers={
                "X-API-Key": api_key,
                "Content-Type": "application/json",
            },
            timeout=timeout,
        )

    def close(self) -> None:
        """Close the underlying HTTP connection pool."""
        self._client.close()

    def __enter__(self) -> AgentDiskClient:
        return self

    def __exit__(self, *exc: object) -> None:
        self.close()

    # ------------------------------------------------------------------
    # Internal helpers
    # ------------------------------------------------------------------
    def _unwrap(self, response: httpx.Response) -> Any:
        """Validate HTTP + business envelope and return ``data``."""
        # Surface transport-level failures first so callers see real status.
        if response.status_code >= 400:
            raise AgentDiskError(
                f"HTTP {response.status_code}: {response.text}",
                status_code=response.status_code,
            )

        payload: Any
        try:
            payload = response.json()
        except ValueError as exc:
            raise AgentDiskError(
                f"non-JSON response: {response.text!r}",
                status_code=response.status_code,
            ) from exc

        # Defensive: every OKF endpoint MUST return the unified envelope.
        if not isinstance(payload, dict):
            raise AgentDiskError(
                f"unexpected payload type {type(payload).__name__}",
                status_code=response.status_code,
            )

        code = payload.get("code")
        message = str(payload.get("message") or "")
        if code not in _OK_CODES:
            raise AgentDiskError(
                message or f"unknown business error (code={code})",
                status_code=response.status_code,
                code=code if isinstance(code, int) else None,
            )
        return payload.get("data")

    def _get(self, path: str, *, params: dict[str, Any] | None = None) -> Any:
        response = self._client.get(path, params=_drop_none(params))
        return self._unwrap(response)

    def _post(self, path: str, *, json: dict[str, Any] | None = None) -> Any:
        response = self._client.post(path, json=_drop_none(json))
        return self._unwrap(response)

    # ------------------------------------------------------------------
    # OKF APIs
    # ------------------------------------------------------------------
    def register_bundle(self, public_directory_id: int) -> dict[str, Any]:
        """POST /v1/disk/okf/bundles/register.

        Args:
            public_directory_id: The public directory that backs this OKF
                bundle. The directory should already exist.

        Returns:
            Server payload, e.g. ``{"bundleId": 12, "publicDirectoryId": 7}``.
        """
        data = self._post(
            "/v1/disk/okf/bundles/register",
            json={"publicDirectoryId": public_directory_id},
        )
        if not isinstance(data, dict):
            raise AgentDiskError("register_bundle: expected object response")
        return data

    def create_folder(
        self,
        pd_id: int,
        folder_name: str,
        parent_id: int | None = None,
    ) -> dict[str, Any]:
        """POST /v1/disk/public-directories/:id/folders.

        Args:
            pd_id: Public directory id.
            folder_name: New folder name (relative, no slashes).
            parent_id: Optional parent folder id; defaults to bundle root.

        Returns:
            Server payload, typically ``{"folderId": int, "name": str}``.
        """
        body: dict[str, Any] = {"folderName": folder_name}
        if parent_id is not None:
            body["parentId"] = parent_id
        data = self._post(
            f"/v1/disk/public-directories/{pd_id}/folders",
            json=body,
        )
        if not isinstance(data, dict):
            raise AgentDiskError("create_folder: expected object response")
        return data

    def write_markdown(
        self,
        pd_id: int,
        rel_path: str,
        content: str,
        content_type: str = "text/markdown",
    ) -> dict[str, Any]:
        """POST /v1/disk/public-directories/:id/files/content.

        Args:
            pd_id: Public directory id.
            rel_path: Bundle-relative path, e.g. ``"concepts/gemma.md"`` or
                ``"index.md"``. Leading slashes are rejected by the server.
            content: File body, MUST include YAML frontmatter (at least a
                ``type`` field) per OKF v0.1.
            content_type: MIME type, defaults to ``text/markdown``.

        Returns:
            Server payload, typically ``{"nodeId": int, "relPath": str}``.
        """
        body: dict[str, Any] = {
            "relPath": rel_path,
            "content": content,
            "contentType": content_type,
        }
        data = self._post(
            f"/v1/disk/public-directories/{pd_id}/files/content",
            json=body,
        )
        if not isinstance(data, dict):
            raise AgentDiskError("write_markdown: expected object response")
        return data

    def list_nodes(
        self,
        bundle_id: int,
        *,
        type: str | None = None,
        tag: str | None = None,
    ) -> dict[str, Any]:
        """GET /v1/disk/okf/bundles/:id/nodes.

        Args:
            bundle_id: OKF bundle id returned by :meth:`register_bundle`.
            type: Optional node-type filter (matches frontmatter ``type``).
            tag: Optional tag filter.

        Returns:
            Server payload, typically
            ``{"nodes": [{"nodeId": int, "relPath": str, ...}]}``.
        """
        params: dict[str, Any] = {}
        if type is not None:
            params["type"] = type
        if tag is not None:
            params["tag"] = tag
        data = self._get(
            f"/v1/disk/okf/bundles/{bundle_id}/nodes",
            params=params,
        )
        if not isinstance(data, dict):
            raise AgentDiskError("list_nodes: expected object response")
        return data

    def aggregate_types(self) -> dict[str, Any]:
        """GET /v1/disk/okf/types.

        The server returns a list of ``{"type": str, "count": int}`` objects
        directly (not wrapped in ``{"types": [...]}``) — see
        ``internal/handler/okf.go:AggregateTypes``. We normalize to the
        wrapped shape so callers don't have to care which form the server
        chose.

        Returns:
            ``{"types": [{"type": str, "count": int}, ...]}``.
        """
        data = self._get("/v1/disk/okf/types")
        if isinstance(data, list):
            return {"types": data}
        if isinstance(data, dict) and "types" not in data:
            # Server returned a bare rollup map (older form). Wrap it.
            return {"types": [{"type": k, "count": v} for k, v in data.items()]}
        if not isinstance(data, dict):
            raise AgentDiskError(
                f"aggregate_types: expected list or object, got {type(data).__name__}"
            )
        return data

    def list_public_directories(self) -> list[dict[str, Any]]:
        """GET /v1/disk/public-directories — list PDs visible to the caller.

        Used by :meth:`find_public_directory_by_path` to resolve a path
        (the only thing the UI shows) to a numeric id (what every other OKF
        endpoint requires).
        """
        data = self._get("/v1/disk/public-directories")
        if isinstance(data, list):
            return [d for d in data if isinstance(d, dict)]
        if isinstance(data, dict):
            items = data.get("items") or data.get("list") or []
            return [d for d in items if isinstance(d, dict)]
        return []

    def find_public_directory_by_path(self, path: str) -> dict[str, Any] | None:
        """Locate a public directory by ``fixedPath`` or ``displayName``.

        The web UI surfaces paths (e.g. ``/public/test``) but the OKF API
        needs the underlying numeric id. This helper bridges that gap.

        Matches ``fixedPath`` exactly first, then falls back to
        ``displayName``. Returns ``None`` if no visible PD matches.
        """
        target = path.strip().rstrip("/")
        if not target:
            return None
        for pd in self.list_public_directories():
            fixed = str(pd.get("fixedPath") or "").strip().rstrip("/")
            display = str(pd.get("displayName") or "").strip()
            if fixed == target or display == target:
                return pd
        return None

    def refresh_index(self, bundle_id: int) -> dict[str, Any]:
        """POST /v1/disk/okf/bundles/:id/refresh.

        Rebuild the bundle's in-memory index from on-disk files. Use after a
        batch of writes outside the HTTP API (e.g. direct OSS writes), or to
        recover from index drift.

        Returns:
            Server payload, typically ``{"bundleId": int, "nodes": int}``.
        """
        data = self._post(f"/v1/disk/okf/bundles/{bundle_id}/refresh")
        if not isinstance(data, dict):
            raise AgentDiskError("refresh_index: expected object response")
        return data


def _drop_none(values: dict[str, Any] | None) -> dict[str, Any]:
    """Strip keys whose value is ``None`` before sending to httpx."""
    if values is None:
        return {}
    return {k: v for k, v in values.items() if v is not None}
