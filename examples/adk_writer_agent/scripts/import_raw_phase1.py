"""Batch-import raw/ files into OKF bundle 21 — phase 1 (no links).

Walks AGENTDISK_RAW_ROOT, infers frontmatter from filename + first heading,
and calls write_markdown for each .md/.markdown/.txt/.rst/.org file.

Skips raw/历史文件/ (already imported in the prior smoke test) and any
binary files (.pdf/.docx/.xlsx — list_raw_files reports them as skipped).

Type/title inference mirrors the agent's INSTRUCTION heuristics so the
output is consistent with what the agent would have produced.
"""
from __future__ import annotations

import os
import re
import sys
from pathlib import Path

HERE = Path(__file__).resolve().parent.parent
os.environ.setdefault("AGENTDISK_RAW_ROOT", str(HERE / "raw"))
sys.path.insert(0, str(HERE))

from adk_writer_agent.tools import list_raw_files, read_raw_file, write_markdown  # noqa: E402

SKIP_DIRS = {"历史文件"}
TEXT_EXTS = {".md", ".markdown", ".txt", ".rst", ".org"}


def infer_type(filename: str) -> str:
    if "主席令" in filename:
        return "law"
    if any(k in filename for k in ("条例", "办法", "规定", "实施细则")):
        return "regulation"
    if any(k in filename for k in ("GB", "GBT", "JRT", "PCI")):
        return "standard"
    if any(k in filename for k in ("通知", "公告", "银发", "银办发")):
        return "notice"
    if "规范" in filename:
        return "spec"
    if any(k in filename for k in ("行动方案", "规划")):
        return "plan"
    if any(k in filename for k in ("JL-", "WI-", "RD-")):
        return "policy"
    if "法" in filename:
        return "law"
    return "notice"


def strip_frontmatter(content: str) -> str:
    if not content.startswith("---\n"):
        return content
    end = content.find("\n---\n", 4)
    if end == -1:
        return content
    return content[end + 5 :]


# Match the BACKEND's link extractor exactly (internal/service/okf_link.go:60):
# `!\[([^\]]*)\]\(([^)]+)\)|\[([^\]]+)\]\(([^)]+)\)`
# The backend stops at the first `)` — escaped parens inside URLs are NOT
# handled — so two `[收藏](javascript:Void\(\);)` on the same line both
# capture url=`javascript:Void\(` and trip the unique constraint.
# We mirror that here so dedupe sees what the backend sees.
_LINK_RE = re.compile(r"!\[([^\]]*)\]\(([^)]+)\)|\[([^\]]+)\]\(([^)]+)\)")
# Same regex restricted to junk URLs so we strip navigation residue BEFORE
# dedupe runs. Covers three families:
# - javascript: / mailto: — script / mail nav residue
# - "/" and "/#fragment" — site-absolute nav URLs whose path becomes "" after
#   the backend's stripFragment(); three of them on one line trip the
#   (src_node_id, dst_rel_path, src_line) UNIQUE constraint because all
#   three edges get dst_rel_path="".
# Legitimate bundle-relative paths like /foo.md or /my/ are NOT matched
# (they have real path content after the slash).
_JUNK_LINK_RE = re.compile(
    r"\[(?P<text>[^\]]*)\]\((?P<url>javascript:[^)]*|mailto:[^)]*|/(?:#[^)]*)?)\)",
    re.IGNORECASE,
)


def _link_text_url(m: re.Match) -> tuple[str, str]:
    """Pick (text, url) from a _LINK_RE match — image vs link branch."""
    if m.group(1) is not None or m.group(2) is not None:  # image branch
        return m.group(1) or "", m.group(2) or ""
    return m.group(3) or "", m.group(4) or ""


def dedupe_same_line_links(body: str) -> str:
    out_lines = []
    for line in body.splitlines():
        matches = list(_LINK_RE.finditer(line))
        if len(matches) < 2:
            out_lines.append(line)
            continue
        seen_urls: dict[str, int] = {}
        chunks: list[str] = []
        cursor = 0
        for m in matches:
            _, url = _link_text_url(m)
            seen_urls[url] = seen_urls.get(url, 0) + 1
            chunks.append(line[cursor : m.start()])
            if seen_urls[url] == 1:
                chunks.append(m.group(0))
            else:
                chunks.append(_link_text_url(m)[0])
            cursor = m.end()
        chunks.append(line[cursor:])
        out_lines.append("".join(chunks))
    return "\n".join(out_lines)


# Match a line that is purely markdown image/link residue — `[![alt](url)](url)`
# or `[text](url)` — so infer_title can skip them. Scraped law sites prepend
# navigation bars whose first non-empty line is a logo image, and that raw
# markdown makes a poor YAML title (square brackets break flow-sequence
# parsing).
_MARKDOWN_LINK_ONLY_RE = re.compile(
    r"^(?:!\[[^\]]*\]\([^)]*\)|\[[^\]]*\]\([^)]*\))+$"
)


def _clean_title(s: str) -> str:
    """Strip markdown formatting from a title candidate.

    `# X` headings → `X`; `[text](url)` → `text`; `![alt](url)` → ``.
    Returns empty string when the candidate was pure image/link residue —
    caller decides whether to skip or fall back to filename.
    """
    out = s
    if out.startswith("# "):
        out = out[2:]
    # Drop images first (alt text only confuses titles), then keep link text.
    out = re.sub(r"!\[[^\]]*\]\([^)]*\)", "", out)
    out = re.sub(r"\[([^\]]*)\]\([^)]*\)", r"\1", out)
    return out.strip()


def infer_title(body: str, filename: str) -> str:
    for line in body.splitlines():
        s = line.strip()
        if not s:
            continue
        cleaned = _clean_title(s)
        if cleaned and not _MARKDOWN_LINK_ONLY_RE.match(cleaned):
            return cleaned[:60]
    return Path(filename).stem


def _yaml_double_quoted(s: str) -> str:
    """Serialize ``s`` as a YAML double-quoted scalar.

    The OKF writer constructs frontmatter as a plain string template, so a
    title containing ``:`` / ``#`` / ``[`` / unicode quotes would otherwise
    parse as something else (mapping / comment / flow sequence). Double
    quoting with ``"``-escaping + backslash escapes for control chars keeps
    every title YAML-safe without pulling in PyYAML at script runtime.
    """
    out = ['"']
    for ch in s:
        if ch == "\\":
            out.append("\\\\")
        elif ch == '"':
            out.append('\\"')
        elif ch == "\n":
            out.append("\\n")
        elif ch == "\t":
            out.append("\\t")
        else:
            out.append(ch)
    out.append('"')
    return "".join(out)


def build_markdown_content(body: str, filename: str) -> str:
    """Wrap a cleaned body with safe YAML frontmatter.

    Used by both the bulk directory walk and the retry path so they stay in
    sync — a divergence here would silently re-break any file we retried.
    """
    title = infer_title(body, filename)
    type_ = infer_type(filename)
    return (
        "---\n"
        f"type: {type_}\n"
        f"title: {_yaml_double_quoted(title)}\n"
        "---\n\n"
        f"{body.strip()}\n"
    )


def process_dir(dir_name: str, stats: dict[str, int]) -> None:
    print(f"\n=== {dir_name}/ ===")
    listed = list_raw_files(dir=dir_name)
    if not listed.get("ok"):
        print(f"  list failed: {listed.get('error')}")
        return
    files = listed["data"]["files"]
    skipped = listed["data"].get("skipped", [])
    print(f"  {len(files)} text files, {len(skipped)} skipped")

    for f in files:
        rel_path = f["rel_path"]
        if Path(rel_path).suffix.lower() not in TEXT_EXTS:
            stats["skipped_ext"] += 1
            continue

        read = read_raw_file(path=rel_path)
        if not read.get("ok"):
            print(f"  ✗ read  {rel_path}: {read.get('error')}")
            stats["read_fail"] += 1
            continue
        body = strip_frontmatter(read["data"]["content"])
        body = _JUNK_LINK_RE.sub(lambda m: m.group("text"), body)
        body = dedupe_same_line_links(body)

        filename = Path(rel_path).name
        new_content = build_markdown_content(body, filename)

        wrote = write_markdown(rel_path=rel_path, content=new_content)
        if wrote.get("ok"):
            stats["written"] += 1
            print(f"  ✓ {rel_path}")
        else:
            err = str(wrote.get("error", ""))
            stats["write_fail"] += 1
            print(f"  ✗ write {rel_path}: {err[:120]}")


def main() -> int:
    stats = {"written": 0, "read_fail": 0, "write_fail": 0, "skipped_ext": 0}
    raw_root = Path(os.environ["AGENTDISK_RAW_ROOT"])
    retry_list = os.environ.get("RETRY_PATHS", "")
    retry_paths = {p.strip() for p in retry_list.split("|") if p.strip()}

    if retry_paths:
        print(f"=== Retry mode: {len(retry_paths)} files ===")
        for rel_path in sorted(retry_paths):
            if Path(rel_path).suffix.lower() not in TEXT_EXTS:
                stats["skipped_ext"] += 1
                continue
            read = read_raw_file(path=rel_path)
            if not read.get("ok"):
                print(f"  ✗ read  {rel_path}: {read.get('error')}")
                stats["read_fail"] += 1
                continue
            body = strip_frontmatter(read["data"]["content"])
            body = _JUNK_LINK_RE.sub(lambda m: m.group("text"), body)
            body = dedupe_same_line_links(body)
            filename = Path(rel_path).name
            content = build_markdown_content(body, filename)
            wrote = write_markdown(rel_path=rel_path, content=content)
            if wrote.get("ok"):
                stats["written"] += 1
                print(f"  ✓ {rel_path}")
            else:
                stats["write_fail"] += 1
                print(f"  ✗ write {rel_path}: {str(wrote.get('error', ''))[:120]}")
        print(
            f"\n=== Retry Summary ===\n"
            f"  written: {stats['written']}  failed: {stats['write_fail']}"
        )
        return 0 if stats["write_fail"] == 0 else 1

    top_dirs = sorted(d.name for d in raw_root.iterdir() if d.is_dir() and not d.name.startswith("."))
    for d in top_dirs:
        if d in SKIP_DIRS:
            print(f"\n=== skip {d}/ (already imported) ===")
            continue
        process_dir(d, stats)

    print(
        f"\n=== Summary ===\n"
        f"  written:      {stats['written']}\n"
        f"  read_fail:    {stats['read_fail']}\n"
        f"  write_fail:   {stats['write_fail']}\n"
        f"  skipped_ext:  {stats['skipped_ext']}"
    )
    return 0 if stats["write_fail"] == 0 and stats["read_fail"] == 0 else 1


if __name__ == "__main__":
    sys.exit(main())
