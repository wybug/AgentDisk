"""OKF state cleanup for ADK eval case isolation.

ADK's ``reset_data`` hook (see
:func:`google.adk.cli.cli_eval.try_get_reset_func`) is called before each eval
case to bring the agent's environment back to a clean slate. For the OKF
writer agent, "clean slate" means:

1. No registered OKF bundles owned by the test user.
2. No markdown files in the test public directory.
3. Exactly ONE freshly-registered bundle exists, so cases that assert on
   ``bundle_id`` (e.g. ``list_nodes(bundle_id=N)``) can predict N at eval
   set build time. The freshly-registered bundle's id is persisted to
   ``OKF_EVAL_BUNDLE_STATE_FILE`` so ``run_evals.py`` can substitute the
   placeholder ``{{BUNDLE_ID}}`` in the eval set.

Both are necessary because eval cases assert on tool trajectories like
``register_bundle`` and ``write_markdown`` — if state from a previous case
lingers, the agent might skip a required call (false-positive pass) or see
stale responses that change its decision (false-negative fail).

The cleanup is best-effort: any HTTP error is logged and swallowed so an
eval run can complete even if the backend is in a weird state. The
``reset_data`` symbol is intentionally a no-arg callable so ADK can
discover it via ``getattr(agent_module.agent, "reset_data", None)``.
"""

from __future__ import annotations

import json
import logging
import os
import tempfile
from typing import Any

import httpx

logger = logging.getLogger("okf_eval_setup")

# Filesystem location where the freshly-registered bundle id is written
# after each clean. run_evals.py reads this to substitute {{BUNDLE_ID}}
# in the parameterized eval set. Lives in the system tempdir (not the
# worktree) so concurrent eval runs in different worktrees don't collide.
OKF_EVAL_BUNDLE_STATE_FILE = os.path.join(tempfile.gettempdir(), "okf_eval_bundle_state.json")

# Successful business code in AgentDisk's unified response envelope.
_OK_CODES: tuple[int, ...] = (0, 200)


def _env(name: str) -> str | None:
    val = os.environ.get(name)
    if val is None or val.strip() == "":
        return None
    return val


def _unwrap(resp: httpx.Response) -> Any:
    """Validate envelope and return ``data`` — raises on any failure."""
    if resp.status_code >= 400:
        raise RuntimeError(f"HTTP {resp.status_code}: {resp.text[:200]}")
    payload = resp.json()
    if not isinstance(payload, dict):
        raise RuntimeError(f"unexpected payload type {type(payload).__name__}")
    code = payload.get("code")
    if code not in _OK_CODES:
        raise RuntimeError(f"business code={code} message={payload.get('message')}")
    return payload.get("data")


def _list_bundles(client: httpx.Client) -> list[dict[str, Any]]:
    """GET /v1/disk/okf/bundles — list bundles owned by the caller."""
    resp = client.get("/v1/disk/okf/bundles")
    data = _unwrap(resp)
    # Server may return either a bare list or {"bundles": [...]}.
    if isinstance(data, list):
        return [b for b in data if isinstance(b, dict)]
    if isinstance(data, dict):
        bundles = data.get("bundles") or data.get("items") or []
        return [b for b in bundles if isinstance(b, dict)]
    return []


def _list_files(client: httpx.Client, pd_id: int) -> list[dict[str, Any]]:
    """GET /v1/disk/public-directories/:id/files — list files in the PD."""
    resp = client.get(f"/v1/disk/public-directories/{pd_id}/files")
    data = _unwrap(resp)
    if isinstance(data, list):
        return [f for f in data if isinstance(f, dict)]
    if isinstance(data, dict):
        files = data.get("files") or data.get("items") or data.get("list") or []
        return [f for f in files if isinstance(f, dict)]
    return []


def _bundle_id(bundle: dict[str, Any]) -> int | None:
    for key in ("bundleId", "id", "bundleID"):
        raw = bundle.get(key)
        if isinstance(raw, int) and raw > 0:
            return raw
        if isinstance(raw, (str, float)) and str(raw).strip().isdigit():
            return int(raw)
    return None


def _file_id(file_obj: dict[str, Any]) -> int | None:
    for key in ("fileId", "id", "fileID"):
        raw = file_obj.get(key)
        if isinstance(raw, int) and raw > 0:
            return raw
        if isinstance(raw, (str, float)) and str(raw).strip().isdigit():
            return int(raw)
    return None


def _delete_quietly(client: httpx.Client, method: str, path: str, label: str) -> None:
    """Issue a DELETE; log + swallow errors so cleanup continues."""
    try:
        resp = client.request(method, path)
        if resp.status_code >= 400 and resp.status_code != 404:
            logger.warning(
                "cleanup %s failed: HTTP %d %s",
                label,
                resp.status_code,
                resp.text[:200],
            )
    except httpx.HTTPError as exc:
        logger.warning("cleanup %s transport error: %s", label, exc)


def clean_okf_state() -> dict[str, int | None]:
    """Wipe OKF bundles + PD files, ensure ONE bundle exists, persist its id.

    Strategy:

    * Read the persisted bundle id from :data:`OKF_EVAL_BUNDLE_STATE_FILE`.
    * If it exists AND the server still has that bundle, KEEP it (just
      wipe the PD files). This makes ``bundle_id`` STABLE across cases in
      the same eval run — critical because ``run_evals.py`` substitutes
      ``{{BUNDLE_ID}}`` once at eval set build time.
    * Otherwise (state missing, bundle gone, or stale id): delete any
      leftover bundles, register a fresh one, persist its id.

    The "keep if exists" path is what makes the eval set deterministic.
    The first reset_data call registers; subsequent calls reuse.

    Returns a small stats dict so callers can confirm the wipe ran.
    """
    base_url = _env("AGENTDISK_BASE_URL")
    api_key = _env("AGENTDISK_API_KEY")
    pd_id_raw = _env("AGENTDISK_PUBLIC_DIRECTORY_ID")
    pd_path_raw = _env("AGENTDISK_PUBLIC_DIRECTORY_PATH")
    if not base_url or not api_key or (not pd_id_raw and not pd_path_raw):
        logger.warning(
            "clean_okf_state skipped: missing env (base_url=%s api_key=%s pd_id=%s pd_path=%s)",
            bool(base_url),
            bool(api_key),
            bool(pd_id_raw),
            bool(pd_path_raw),
        )
        return {"bundles_deleted": 0, "files_deleted": 0, "bundle_id": None}

    bundles_deleted = 0
    files_deleted = 0
    bundle_id: int | None = None

    with httpx.Client(
        base_url=base_url.rstrip("/"),
        # API key goes via X-API-Key (not Bearer) — see
        # agentdisk_client.AgentDiskClient.__init__ for the rationale.
        headers={"X-API-Key": api_key, "Content-Type": "application/json"},
        timeout=15.0,
    ) as client:
        # Resolve path → id lazily, only when the PATH form is in use.
        pd_id: int | None
        if pd_id_raw:
            try:
                pd_id = int(pd_id_raw)
            except ValueError:
                logger.warning(
                    "clean_okf_state: AGENTDISK_PUBLIC_DIRECTORY_ID not an int: %r",
                    pd_id_raw,
                )
                return {"bundles_deleted": 0, "files_deleted": 0, "bundle_id": None}
        else:
            resolved = _resolve_pd_id_by_path(client, pd_path_raw or "")
            if resolved is None:
                logger.warning(
                    "clean_okf_state: no public directory matches path %r",
                    pd_path_raw,
                )
                return {"bundles_deleted": 0, "files_deleted": 0, "bundle_id": None}
            pd_id = resolved

        # Try to reuse the bundle from the previous reset_data call.
        # Without this, every case would get a NEW bundle id and the
        # {{BUNDLE_ID}} placeholder in the eval set would be stale for
        # cases 2..N.
        cached_bid = _read_cached_bundle_id()

        existing_bundles: set[int] = set()
        try:
            for bundle in _list_bundles(client):
                bid = _bundle_id(bundle)
                if bid is not None:
                    existing_bundles.add(bid)
        except Exception as exc:  # noqa: BLE001 — best-effort cleanup
            logger.warning("bundle list aborted: %s", exc)

        if cached_bid is not None and cached_bid in existing_bundles:
            # Reuse path: delete every OTHER bundle, keep cached_bid.
            for bid in existing_bundles - {cached_bid}:
                _delete_quietly(client, "DELETE", f"/v1/disk/okf/bundles/{bid}", f"bundle {bid}")
                bundles_deleted += 1
            bundle_id = cached_bid
            logger.info("clean_okf_state: reusing cached bundle_id=%d", bundle_id)
        else:
            # Fresh path: wipe everything, write index.md, register one new
            # bundle. The index.md seed is required because the backend
            # rejects bundle registration with "not an OKF bundle root
            # (index.md missing okf_version)" if the PD root is empty.
            for bid in existing_bundles:
                _delete_quietly(client, "DELETE", f"/v1/disk/okf/bundles/{bid}", f"bundle {bid}")
                bundles_deleted += 1
            _ensure_okf_root(client, pd_id)
            try:
                bundle_id = _register_fresh_bundle(client, pd_id)
            except Exception as exc:  # noqa: BLE001 — best-effort cleanup
                logger.warning("pre-register bundle failed: %s", exc)
                bundle_id = None

        # Always wipe PD files so cases don't see each other's writes,
        # then re-seed index.md so the bundle root stays valid for any
        # subsequent register_bundle call (idempotent server path may
        # re-validate on each call).
        try:
            for file_obj in _list_files(client, pd_id):
                fid = _file_id(file_obj)
                if fid is None:
                    continue
                _delete_quietly(
                    client,
                    "DELETE",
                    f"/v1/disk/public-directories/{pd_id}/files/{fid}",
                    f"file {fid}",
                )
                files_deleted += 1
        except Exception as exc:  # noqa: BLE001 — best-effort cleanup
            logger.warning("file cleanup aborted: %s", exc)
        _ensure_okf_root(client, pd_id)

    _persist_bundle_state(pd_id, bundle_id)
    logger.info(
        "clean_okf_state: %d bundles, %d files removed, bundle_id=%s",
        bundles_deleted,
        files_deleted,
        bundle_id,
    )
    return {
        "bundles_deleted": bundles_deleted,
        "files_deleted": files_deleted,
        "bundle_id": bundle_id,
    }


def _read_cached_bundle_id() -> int | None:
    """Return the bundle id from the previous reset_data call, or None."""
    try:
        with open(OKF_EVAL_BUNDLE_STATE_FILE) as f:
            data = json.load(f)
    except (OSError, json.JSONDecodeError):
        return None
    bid = data.get("bundle_id") if isinstance(data, dict) else None
    return int(bid) if isinstance(bid, int) and bid > 0 else None


def _register_fresh_bundle(client: httpx.Client, pd_id: int) -> int | None:
    """POST /v1/disk/okf/bundles/register and return the new bundle id.

    Returns ``None`` if the response shape is unexpected. The server is
    expected to be idempotent — if a bundle already exists for this PD
    (e.g. a prior eval case didn't clean up), the same id is returned.
    """
    resp = client.post(
        "/v1/disk/okf/bundles/register",
        json={"publicDirectoryId": pd_id},
    )
    data = _unwrap(resp)
    if not isinstance(data, dict):
        logger.warning("register bundle returned non-object: %r", data)
        return None
    return _bundle_id(data)


def _ensure_okf_root(client: httpx.Client, pd_id: int) -> None:
    """Write a minimal ``index.md`` so the PD qualifies as an OKF bundle root.

    The backend rejects ``POST /v1/disk/okf/bundles/register`` with
    ``"index.md missing okf_version"`` if the PD root is empty. We seed
    ``index.md`` with the minimum valid frontmatter before registering.

    Errors are logged but not raised — if the write fails, register_bundle
    will fail next, and we surface that as the actionable error.
    """
    body = {
        "relPath": "index.md",
        "content": '---\nokf_version: "0.1"\ntype: index\n---\n',
        "contentType": "text/markdown",
    }
    try:
        resp = client.post(
            f"/v1/disk/public-directories/{pd_id}/files/content",
            json=body,
        )
        _unwrap(resp)
    except Exception as exc:  # noqa: BLE001 — best-effort seed
        logger.warning("could not seed index.md for PD %d: %s", pd_id, exc)


def _persist_bundle_state(pd_id: int | None, bundle_id: int | None) -> None:
    """Write ``{pd_id, bundle_id}`` to the eval state file.

    Best-effort: a write failure is logged but not raised, so cleanup
    itself never fails the eval. ``run_evals.py`` reads this file before
    each eval run to substitute placeholders.
    """
    payload = {"pd_id": pd_id, "bundle_id": bundle_id}
    try:
        with open(OKF_EVAL_BUNDLE_STATE_FILE, "w") as f:
            json.dump(payload, f)
    except OSError as exc:
        logger.warning("could not write eval state file %s: %s", OKF_EVAL_BUNDLE_STATE_FILE, exc)


def _resolve_pd_id_by_path(client: httpx.Client, path: str) -> int | None:
    """Look up a public directory id by fixedPath/displayName.

    The web UI only shows the path; this helper lets eval setups identify
    the target PD without admin tools. Returns ``None`` if no visible PD
    matches.
    """
    target = path.strip().rstrip("/")
    if not target:
        return None
    try:
        resp = client.get("/v1/disk/public-directories")
        data = _unwrap(resp)
    except Exception as exc:  # noqa: BLE001 — surfacing lookup failures as None
        logger.warning("PD path lookup failed: %s", exc)
        return None

    dirs: list[dict[str, Any]] = []
    if isinstance(data, list):
        dirs = [d for d in data if isinstance(d, dict)]
    elif isinstance(data, dict):
        items = data.get("items") or data.get("list") or []
        dirs = [d for d in items if isinstance(d, dict)]

    for d in dirs:
        fixed = str(d.get("fixedPath") or "").strip().rstrip("/")
        display = str(d.get("displayName") or "").strip()
        if fixed == target or display == target:
            # PD's primary key is `id` — same field name as bundle, so the
            # existing _bundle_id helper happens to work, but be explicit
            # here so the code reads as "PD lookup", not "bundle lookup".
            for key in ("id", "Id", "ID"):
                raw = d.get(key)
                if isinstance(raw, int) and raw > 0:
                    return raw
                if isinstance(raw, (str, float)) and str(raw).strip().isdigit():
                    return int(raw)
            return None
    return None
