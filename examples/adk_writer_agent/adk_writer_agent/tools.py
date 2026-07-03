"""ADK FunctionTool wrappers around the AgentDisk OKF HTTP client.

Each tool here mirrors one OKF API and is registered on the root LlmAgent in
:mod:`adk_writer_agent.agent`. Tools are intentionally thin: parameter
validation and persistence live in :mod:`adk_writer_agent.agentdisk_client`,
while these wrappers translate client errors into ADK-friendly dicts.
"""

from __future__ import annotations

import os
import re
from functools import lru_cache
from pathlib import Path
from typing import Any

import yaml
from google.adk.tools.function_tool import FunctionTool

from .agentdisk_client import AgentDiskClient, AgentDiskError


# ----------------------------------------------------------------------
# Client lifecycle
# ----------------------------------------------------------------------
@lru_cache(maxsize=1)
def _get_client() -> AgentDiskClient:
    """Return a process-wide :class:`AgentDiskClient` built from env vars.

    The ADK runtime instantiates tools as module-level singletons, so a module
    global is fine here. Configuration is read lazily so unit tests can
    monkeypatch ``os.environ`` before the first call.
    """
    base_url = os.environ.get("AGENTDISK_BASE_URL")
    api_key = os.environ.get("AGENTDISK_API_KEY")
    if not base_url:
        raise RuntimeError("AGENTDISK_BASE_URL is not set")
    if not api_key:
        raise RuntimeError("AGENTDISK_API_KEY is not set")
    return AgentDiskClient(base_url=base_url, api_key=api_key)


def _err_payload(exc: AgentDiskError) -> dict[str, Any]:
    """Convert :class:`AgentDiskError` into a serializable dict for the LLM."""
    return {
        "ok": False,
        "error": exc.message,
        "httpStatus": exc.status_code,
        "code": exc.code,
    }


def _ok_payload(data: dict[str, Any]) -> dict[str, Any]:
    return {"ok": True, "data": data}


# ----------------------------------------------------------------------
# Tool functions (docstrings are read by the LLM)
# ----------------------------------------------------------------------
def register_bundle(public_directory_id: int) -> dict[str, Any]:
    """Register a public directory as an OKF v0.1 bundle.

    Call this ONCE per public directory. Re-registering an already-registered
    directory returns the existing bundle id (idempotent on the server side).

    Args:
        public_directory_id: ID of the public directory that should back the
            bundle. Must already exist.

    Returns:
        ``{"ok": True, "data": {"bundleId": int, ...}}`` on success, or
        ``{"ok": False, "error": str, ...}`` on failure.
    """
    try:
        data = _get_client().register_bundle(public_directory_id)
    except AgentDiskError as exc:
        return _err_payload(exc)
    return _ok_payload(data)


def create_folder(
    folder_name: str,
    parent_id: int | None = None,
    public_directory_id: int | None = None,
) -> dict[str, Any]:
    """Create a folder inside the bundle's public directory.

    Use this before writing files into a sub-tree (e.g. ``concepts/``). If the
    folder already exists the server returns an error; treat that as success
    for idempotent bootstrap.

    Args:
        folder_name: Folder name, no slashes (e.g. ``"concepts"``).
        parent_id: Optional parent folder id. If omitted, creates at the
            bundle root.
        public_directory_id: Public directory id. If omitted, reads
            ``AGENTDISK_PUBLIC_DIRECTORY_ID`` from env.

    Returns:
        ``{"ok": True, "data": {"folderId": int, "name": str}}`` or error.
    """
    pd_id = public_directory_id or _default_pd_id()
    try:
        data = _get_client().create_folder(pd_id, folder_name, parent_id)
    except AgentDiskError as exc:
        return _err_payload(exc)
    return _ok_payload(data)


def write_markdown(
    rel_path: str,
    content: str,
    public_directory_id: int | None = None,
) -> dict[str, Any]:
    """Write a markdown file into the bundle. The core OKF write primitive.

    The file MUST start with YAML frontmatter delimited by ``---`` lines. At
    minimum include a ``type`` field (free-form lowercase string, e.g.
    ``concept``/``playbook``/``reference``/``llm``). Recommended fields:
    ``title``, ``description``, ``tags`` (list), ``timestamp`` (ISO 8601).
    Bundle-relative links use paths like ``/concepts/gemma.md`` or
    ``./gemma.md``. Special files: ``index.md`` (bundle root, requires
    ``okf_version: "0.1"``) and ``log.md`` (update history).

    If the server rejects the write (HTTP 400), inspect the returned error and
    fix the frontmatter, then retry once.

    Args:
        rel_path: Bundle-relative path, e.g. ``"concepts/gemma.md"`` or
            ``"index.md"``. No leading slash.
        content: Full file content including YAML frontmatter.
        public_directory_id: Public directory id. If omitted, reads
            ``AGENTDISK_PUBLIC_DIRECTORY_ID`` from env.

    Returns:
        ``{"ok": True, "data": {"nodeId": int, "relPath": str}}`` or error.
    """
    pd_id = public_directory_id or _default_pd_id()
    try:
        data = _get_client().write_markdown(pd_id, rel_path, content)
    except AgentDiskError as exc:
        return _err_payload(exc)
    return _ok_payload(data)


def list_nodes(
    bundle_id: int,
    type: str | None = None,
    tag: str | None = None,
) -> dict[str, Any]:
    """List nodes inside an OKF bundle, optionally filtered.

    Args:
        bundle_id: OKF bundle id (from :func:`register_bundle`).
        type: Optional node-type filter (matches frontmatter ``type``).
        tag: Optional tag filter (single tag, exact match).

    Returns:
        ``{"ok": True, "data": {"nodes": [{"nodeId": int, ...}]}}`` or error.
    """
    try:
        data = _get_client().list_nodes(bundle_id, type=type, tag=tag)
    except AgentDiskError as exc:
        return _err_payload(exc)
    return _ok_payload(data)


def aggregate_types() -> dict[str, Any]:
    """Aggregate node counts per ``type`` across all bundles the caller owns.

    No parameters. Useful for a quick "what's in the knowledge base" summary.

    Returns:
        ``{"ok": True, "data": {"types": [{"type": str, "count": int}, ...]}}``
        or error.
    """
    try:
        data = _get_client().aggregate_types()
    except AgentDiskError as exc:
        return _err_payload(exc)
    return _ok_payload(data)


def refresh_index(bundle_id: int) -> dict[str, Any]:
    """Force-rebuild the in-memory index of an OKF bundle from on-disk files.

    Call after a batch of direct writes (outside the HTTP API), or to recover
    from index drift. Reads the public directory recursively; can be slow on
    large bundles.

    Args:
        bundle_id: OKF bundle id.

    Returns:
        ``{"ok": True, "data": {"bundleId": int, "nodes": int}}`` or error.
    """
    try:
        data = _get_client().refresh_index(bundle_id)
    except AgentDiskError as exc:
        return _err_payload(exc)
    return _ok_payload(data)


def _default_pd_id() -> int:
    """Resolve the default public directory id from the environment.

    Accepts either:

    * ``AGENTDISK_PUBLIC_DIRECTORY_ID`` (preferred when set — direct int)
    * ``AGENTDISK_PUBLIC_DIRECTORY_PATH`` (fallback — resolved via the
      ``/v1/disk/public-directories`` listing; the UI surfaces paths, not
      ids, so this is the friendlier option for ad-hoc eval runs)

    The PATH form pays one HTTP call on first use, then caches the result
    on the module-level ``_PD_ID_CACHE`` so subsequent tool calls are
    cheap. Both env vars being unset is a hard error — the agent has no
    way to operate without a target PD.
    """
    raw_id = os.environ.get("AGENTDISK_PUBLIC_DIRECTORY_ID")
    if raw_id and raw_id.strip():
        return int(raw_id)

    raw_path = os.environ.get("AGENTDISK_PUBLIC_DIRECTORY_PATH")
    if not raw_path or not raw_path.strip():
        raise RuntimeError(
            "AGENTDISK_PUBLIC_DIRECTORY_ID or AGENTDISK_PUBLIC_DIRECTORY_PATH must be set"
        )

    cached = _PD_ID_CACHE.get(raw_path.strip())
    if cached is not None:
        return cached

    pd = _get_client().find_public_directory_by_path(raw_path.strip())
    if pd is None:
        raise RuntimeError(
            f"no public directory matches path {raw_path!r} "
            f"(visible PDs can be listed via GET /v1/disk/public-directories)"
        )
    for key in ("id", "Id", "ID"):
        if isinstance(pd.get(key), int):
            _PD_ID_CACHE[raw_path.strip()] = pd[key]
            return int(pd[key])
    raise RuntimeError(f"public directory for path {raw_path!r} has no usable id: {pd!r}")


# Cache for path→id resolution. Keyed by path string so two different
# paths in the same process don't collide. Lives at module scope so the
# first tool call in a eval case pays the lookup cost, subsequent calls
# (and calls in later cases) don't.
_PD_ID_CACHE: dict[str, int] = {}


# ----------------------------------------------------------------------
# Local raw-file tools (for "initialize KB from local files" workflow)
# ----------------------------------------------------------------------
# Accepted text-file extensions (lowercase, with leading dot).
_TEXT_EXTS: frozenset[str] = frozenset({".md", ".markdown", ".txt", ".rst", ".org"})

# Directory names that are always skipped during recursive scan.
_SKIP_DIRS: frozenset[str] = frozenset(
    {".git", "node_modules", "__pycache__", "dist", "build", ".venv", "venv"}
)

# File-name suffix → source-site hint. Raw archives scraped from public
# sites carry a suffix like "_waizi" / "_webtax"; surfacing the source in
# the hint lets the LLM record provenance in frontmatter.
_SOURCE_SUFFIXES: tuple[tuple[str, str], ...] = (
    ("_waizi", "waizi"),
    ("_webtax", "webtax"),
    ("_esnai", "esnai"),
    ("_mpaypass", "mpaypass"),
    ("_lawlib", "lawlib"),
    ("_gov", "gov"),
    ("_shfgw", "shfgw"),
    ("_cnafc", "cnafc"),
)

# Bytes of head to read for hint extraction (first_line, h2_count).
_HINT_BYTES = 4096

# Hard cap on a single read_raw_file non-chunked return. Beyond this the
# caller must use chunk=True. Keeps LLM context windows bounded.
_MAX_FILE_BYTES = 200 * 1024

# Max files returned by list_raw_files. Beyond this, truncated=True.
_MAX_FILES = 500


def _raw_root() -> Path:
    """Return the resolved AGENTDISK_RAW_ROOT path.

    Reads the env var; if unset, falls back to the package-included
    ``raw/`` directory (``examples/adk_writer_agent/raw/``). The fallback
    makes development zero-config; production deployments should set the
    env var to point at their own raw data directory.

    Raises:
        RuntimeError: if the env var is unset AND the default ``raw/``
            directory does not exist.
    """
    env_root = os.environ.get("AGENTDISK_RAW_ROOT")
    if env_root and env_root.strip():
        return Path(env_root).expanduser().resolve()

    # Default: package-included raw/ (parent of adk_writer_agent/ package)
    package_root = Path(__file__).resolve().parent.parent
    default_root = package_root / "raw"
    if not default_root.is_dir():
        raise RuntimeError(
            "AGENTDISK_RAW_ROOT is not set and the default package raw/ "
            f"directory ({default_root}) does not exist"
        )
    return default_root.resolve()


def _resolve_safe(raw_root: Path, sub: str) -> Path:
    """Resolve ``<raw_root>/<sub>`` with path-traversal protection.

    Args:
        raw_root: Absolute, resolved root directory.
        sub: Relative sub-path (file or directory). Empty string means
            the root itself.

    Raises:
        ValueError: if ``sub`` is absolute, contains ``..`` segments,
            or resolves outside ``raw_root``.
    """
    if not sub or not sub.strip():
        return raw_root.resolve()

    sub_stripped = sub.strip()
    # Reject absolute paths BEFORE stripping leading "/" so "/etc/passwd"
    # does not silently become "etc/passwd" and bypass the check.
    if sub_stripped.startswith("/") or Path(sub_stripped).is_absolute():
        raise ValueError(f"absolute path not allowed: {sub!r}")

    sub_clean = sub_stripped.lstrip("/")
    if not sub_clean:
        return raw_root.resolve()

    parts = Path(sub_clean).parts
    if ".." in parts:
        raise ValueError(f"path traversal (..) not allowed: {sub!r}")

    resolved = (raw_root / sub_clean).resolve()
    root_resolved = raw_root.resolve()
    try:
        resolved.relative_to(root_resolved)
    except ValueError as exc:
        raise ValueError(f"path escapes RAW_ROOT: {sub!r}") from exc
    return resolved


def _detect_source(stem: str) -> str | None:
    """Return the source-site name for a known suffix, else None."""
    lower = stem.lower()
    for suffix, name in _SOURCE_SUFFIXES:
        if lower.endswith(suffix):
            return name
    return None


def _build_file_info(entry: Path, raw_root: Path) -> dict[str, Any]:
    """Build the per-file info dict for list_raw_files.

    Reads only the first ``_HINT_BYTES`` to extract ``first_line`` and
    ``h2_count``; the full content is fetched separately via
    :func:`read_raw_file`.
    """
    size = entry.stat().st_size
    rel_path = str(entry.relative_to(raw_root))

    try:
        with entry.open("rb") as fh:
            head_bytes = fh.read(_HINT_BYTES)
        head = head_bytes.decode("utf-8", errors="replace")
    except OSError:
        head = ""

    # Skip a leading YAML frontmatter block when extracting hints so the
    # first_line is the actual title-bearing line, not the frontmatter
    # delimiter.
    body_head = head
    if body_head.startswith("---\n"):
        end = body_head.find("\n---\n", 4)
        if end != -1:
            body_head = body_head[end + len("\n---\n") :]

    first_line = ""
    for raw_line in body_head.splitlines():
        line = raw_line.strip()
        if not line:
            continue
        # Strip a leading markdown H1 (only the very first one is treated
        # as the title-bearing heading).
        if line.startswith("# "):
            line = line[2:].strip()
        first_line = line[:80]
        break

    h2_count = sum(
        1
        for line in body_head.splitlines()
        if line.startswith("## ") and not line.startswith("### ")
    )

    if size < 8 * 1024:
        bucket = "small"
    elif size < _MAX_FILE_BYTES:
        bucket = "medium"
    else:
        bucket = "large"

    hint: dict[str, Any] = {"first_line": first_line, "h2_count": h2_count}
    source = _detect_source(entry.stem)
    if source is not None:
        hint["source"] = source

    return {
        "rel_path": rel_path,
        "size": size,
        "ext": entry.suffix.lower(),
        "size_bucket": bucket,
        "hint": hint,
    }


def list_raw_files(dir: str = "") -> dict[str, Any]:
    """List raw text files under ``AGENTDISK_RAW_ROOT/<dir>`` recursively.

    Used in the scan phase of "initialize KB from local files". Returns
    per-file hints (first line, H2 count, source) so the LLM can infer
    type/title/tags and identify cross-file relations without calling
    :func:`read_raw_file` on every file.

    Args:
        dir: Sub-directory relative to ``AGENTDISK_RAW_ROOT``. Empty string
            means ``RAW_ROOT`` itself. Absolute paths and any path
            containing ``..`` are rejected.

    Returns:
        ``{"ok": True, "data": {"root", "dir", "files", "skipped",
        "truncated"}}`` on success, or ``{"ok": False, "error": str}``
        on failure.
    """
    try:
        raw_root = _raw_root()
    except RuntimeError as exc:
        return {"ok": False, "error": str(exc)}

    try:
        target_dir = _resolve_safe(raw_root, dir)
    except ValueError as exc:
        return {"ok": False, "error": str(exc)}

    if not target_dir.is_dir():
        return {"ok": False, "error": f"not a directory: {dir!r}"}

    files: list[dict[str, Any]] = []
    skipped: list[str] = []
    truncated = False

    for entry in sorted(target_dir.rglob("*")):
        rel_parts = entry.relative_to(raw_root).parts

        # Skip if any parent dir is in the skip list or hidden (".foo").
        skip_for_parent = False
        for part in rel_parts[:-1]:
            if part in _SKIP_DIRS or part.startswith("."):
                skip_for_parent = True
                break

        if skip_for_parent:
            if entry.is_file():
                skipped.append(str(entry.relative_to(raw_root)))
            continue

        if entry.is_dir():
            continue

        # Skip hidden files.
        if entry.name.startswith("."):
            skipped.append(str(entry.relative_to(raw_root)))
            continue

        # Filter by extension.
        if entry.suffix.lower() not in _TEXT_EXTS:
            skipped.append(str(entry.relative_to(raw_root)))
            continue

        if len(files) >= _MAX_FILES:
            truncated = True
            continue

        files.append(_build_file_info(entry, raw_root))

    return _ok_payload(
        {
            "root": str(raw_root),
            "dir": dir,
            "files": files,
            "skipped": sorted(skipped),
            "truncated": truncated,
        }
    )


def _parse_frontmatter(content: str) -> dict[str, Any] | None:
    """Parse YAML frontmatter if present at the top of ``content``.

    Returns the parsed dict, or ``None`` if no frontmatter is present or
    YAML parsing fails. The caller should treat ``None`` as "no usable
    frontmatter; let the LLM regenerate one".
    """
    if not content.startswith("---\n"):
        return None
    end = content.find("\n---\n", 4)
    if end == -1:
        return None
    yaml_body = content[4:end]
    try:
        parsed = yaml.safe_load(yaml_body)
    except yaml.YAMLError:
        return None
    if not isinstance(parsed, dict):
        return None
    return parsed


def _strip_frontmatter(content: str) -> str:
    """Return ``content`` with leading YAML frontmatter removed (if any)."""
    if not content.startswith("---\n"):
        return content
    end = content.find("\n---\n", 4)
    if end == -1:
        return content
    return content[end + len("\n---\n") :]


def _split_by_paragraph(
    text: str, start_index: int, default_heading: str
) -> list[dict[str, Any]]:
    """Split ``text`` by blank-line paragraphs, each chunk ≤ _MAX_FILE_BYTES."""
    paragraphs = re.split(r"\n\s*\n", text)
    chunks: list[dict[str, Any]] = []
    current = ""
    idx = start_index
    for para in paragraphs:
        candidate = f"{current}\n\n{para}" if current else para
        if (
            len(candidate.encode("utf-8")) > _MAX_FILE_BYTES
            and current
        ):
            size_kb = len(current.encode("utf-8")) / 1024
            chunks.append(
                {
                    "index": idx,
                    "heading": default_heading,
                    "approx_size_kb": round(size_kb, 1),
                    "content": current.rstrip() + "\n",
                }
            )
            idx += 1
            current = para
        else:
            current = candidate
    if current.strip():
        size_kb = len(current.encode("utf-8")) / 1024
        chunks.append(
            {
                "index": idx,
                "heading": default_heading,
                "approx_size_kb": round(size_kb, 1),
                "content": current.rstrip() + "\n",
            }
        )
        idx += 1
    return chunks


def _split_into_chunks(content: str) -> list[dict[str, Any]]:
    """Split content into chunks ≤ _MAX_FILE_BYTES by markdown H2 headings.

    Falls back to paragraph-level splitting when an H2 section is itself
    larger than the cap. Frontmatter is stripped before splitting.
    """
    body = _strip_frontmatter(content)

    h2_pattern = re.compile(r"^## ", re.MULTILINE)
    matches = list(h2_pattern.finditer(body))

    chunks: list[dict[str, Any]] = []

    if not matches:
        return _split_by_paragraph(body, start_index=1, default_heading="")

    # Preamble (text before the first H2): emit as chunks if non-trivial.
    preamble = body[: matches[0].start()].strip()
    if preamble and len(preamble.encode("utf-8")) > 500:
        chunks.extend(_split_by_paragraph(preamble, len(chunks) + 1, "引言"))

    for i, match in enumerate(matches):
        section_start = match.start()
        section_end = matches[i + 1].start() if i + 1 < len(matches) else len(body)
        section = body[section_start:section_end]
        heading = section.split("\n", 1)[0].lstrip("# ").strip()[:80]

        if len(section.encode("utf-8")) > _MAX_FILE_BYTES:
            chunks.extend(
                _split_by_paragraph(section, len(chunks) + 1, heading)
            )
        else:
            size_kb = len(section.encode("utf-8")) / 1024
            chunks.append(
                {
                    "index": len(chunks) + 1,
                    "heading": heading,
                    "approx_size_kb": round(size_kb, 1),
                    "content": section.rstrip() + "\n",
                }
            )

    return chunks


def read_raw_file(path: str, chunk: bool = False) -> dict[str, Any]:
    """Read a raw text file under ``AGENTDISK_RAW_ROOT``.

    Args:
        path: Relative path under ``AGENTDISK_RAW_ROOT``. Empty, absolute,
            and ``..``-containing paths are rejected.
        chunk: When True and the file exceeds ``_MAX_FILE_BYTES`` (200 KB),
            split by markdown H2 headings (falling back to paragraph
            splitting) so each chunk fits within the cap. When False,
            oversized files are returned truncated to the first 200 KB
            with ``truncated=True``.

    Returns:
        Non-chunked: ``{"ok": True, "data": {"path", "size", "truncated",
        "frontmatter", "content"}}``.
        Chunked: ``{"ok": True, "data": {"path", "size", "chunked": true,
        "frontmatter", "chunks": [...]}}`` where each chunk has
        ``index``, ``heading``, ``approx_size_kb``, ``content``.
        Error: ``{"ok": False, "error": str}``.
    """
    try:
        raw_root = _raw_root()
    except RuntimeError as exc:
        return {"ok": False, "error": str(exc)}

    try:
        target = _resolve_safe(raw_root, path)
    except ValueError as exc:
        return {"ok": False, "error": str(exc)}

    if not target.is_file():
        return {"ok": False, "error": f"not a file: {path!r}"}

    size = target.stat().st_size
    try:
        content = target.read_text(encoding="utf-8", errors="replace")
    except OSError as exc:
        return {"ok": False, "error": f"read failed: {exc}"}

    frontmatter = _parse_frontmatter(content)

    if chunk and size > _MAX_FILE_BYTES:
        chunks = _split_into_chunks(content)
        return _ok_payload(
            {
                "path": path,
                "size": size,
                "chunked": True,
                "frontmatter": frontmatter,
                "chunks": chunks,
            }
        )

    truncated = size > _MAX_FILE_BYTES
    returned = content[:_MAX_FILE_BYTES] if truncated else content
    return _ok_payload(
        {
            "path": path,
            "size": size,
            "truncated": truncated,
            "frontmatter": frontmatter,
            "content": returned,
        }
    )


# ----------------------------------------------------------------------
# FunctionTool singletons
# ----------------------------------------------------------------------
register_bundle_tool = FunctionTool(func=register_bundle)
create_folder_tool = FunctionTool(func=create_folder)
write_markdown_tool = FunctionTool(func=write_markdown)
list_nodes_tool = FunctionTool(func=list_nodes)
aggregate_types_tool = FunctionTool(func=aggregate_types)
refresh_index_tool = FunctionTool(func=refresh_index)
list_raw_files_tool = FunctionTool(func=list_raw_files)
read_raw_file_tool = FunctionTool(func=read_raw_file)

__all__ = [
    "aggregate_types_tool",
    "create_folder_tool",
    "list_nodes_tool",
    "list_raw_files_tool",
    "read_raw_file_tool",
    "refresh_index_tool",
    "register_bundle_tool",
    "write_markdown_tool",
]
