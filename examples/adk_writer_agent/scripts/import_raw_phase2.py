"""Phase 2: scan bundle 21 for cross-directory citation candidates + broken-link cleanup.

Two responsibilities, run in DRY mode by default:

1. **Citation candidates** — for each raw file, regex-scan body for `《XX》`,
   `依据XX`, `参见《XX》`, `相关法规：XX` patterns. Fuzzy-match XX against the
   bundle's title index (built from `list_nodes`). High-confidence matches
   (>= MATCH_THRESHOLD) become "apply" candidates; the rest are surfaced for
   human review.

2. **Broken-link inventory** — pull all broken links from
   `/okf/bundles/21/broken-links` and group by source file. Phase-2 APPLY mode
   will strip these when rewriting. Most are scraped nav / image residue that
   `_JUNK_LINK_RE` didn't catch in phase 1 (e.g. `news.esnai.com/...` paths).

Output files (written next to this script):

- `phase2_scan.json` — machine-readable: full candidate list, decisions, broken links
- `phase2_scan.md` — human-readable: per-file summary tables

Run:
    python scripts/import_raw_phase2.py            # scan only
    python scripts/import_raw_phase2.py --summary  # scan + print compact summary
    APPLY=1 python scripts/import_raw_phase2.py    # apply accepted candidates
"""
from __future__ import annotations

import argparse
import json
import os
import re
import sys
from collections import defaultdict
from difflib import SequenceMatcher
from pathlib import Path

HERE = Path(__file__).resolve().parent.parent
os.environ.setdefault("AGENTDISK_RAW_ROOT", str(HERE / "raw"))
sys.path.insert(0, str(HERE))

from adk_writer_agent.tools import (  # noqa: E402
    _get_client,
    list_nodes,
    read_raw_file,
    write_markdown,
)

# Import phase-1 helpers so the apply-mode rewrite stays byte-identical to
# phase-1's output PLUS the new citation links. Diverging here would silently
# re-introduce formatting differences that pollute diffs.
from scripts.import_raw_phase1 import (  # noqa: E402
    SKIP_DIRS,
    TEXT_EXTS,
    _JUNK_LINK_RE,
    _LINK_RE,
    _link_text_url,
    build_markdown_content,
    dedupe_same_line_links,
    strip_frontmatter,
)

BUNDLE_ID = int(os.environ.get("OKF_BUNDLE_ID", "21"))
MATCH_THRESHOLD = 0.78  # (0,1] — SequenceMatcher.ratio(); 0.78 catches 法/办法/规定 variants
REVIEW_THRESHOLD = 0.55  # below MATCH_THRESHOLD but worth showing in the review table

# Citation patterns. Order matters — the `依据/参见/相关法规` lookbehind captures
# the more specific case before the bare `《XX》` falls through.
CITATION_RES = [
    # 依据/按照/根据/参照 + optional 《》
    re.compile(r"(?:依据|按照|根据|参照|参见|执行|贯彻|落实|符合|遵守)\s*[《\[「]([^\]》」]{2,40})[》\]」]"),
    # 相关法规：/参考资料：/依据： + comma-or-、separated list of titles
    re.compile(r"(?:相关法规|参考资料|依据|参考|引用)\s*[:：]\s*([^\n]{4,200})"),
    # Bare 《XX》 — most common in Chinese legal text
    re.compile(r"[《\[「]([^\]》」]{2,40})[》\]」]"),
]

# Markdown link forms that point to bundle-relative targets. Used by the
# broken-link stripper to find residue from phase-1's incomplete cleanup.
_BUNDLE_LINK_RE = re.compile(
    r"!\[([^\]]*)\]\(([^)]+)\)|\[([^\]]+)\]\(([^)]+)\)"
)

# Title normalization: drop common suffixes so "中华人民共和国反洗钱法" matches
# "反洗钱法" and "中华人民共和国反洗钱法（修订）".
_TITLE_NOISE = re.compile(
    r"(中华人民共和国|中国政府|国务院|中国人民银行|中国银行保险监督管理委员会|国家金融监督管理总局|中国证监会|中国银保监会)"
    r"|（[^）]*）|\([^)]*\)"
    r"|(关于|进一步|加强|规范|推动|做好|深化|推进|完善)"
    r"|(实施|试行|暂行|修订|修正)"
    r"|(的|了)"
)
_TITLE_SUFFIX = re.compile(r"(法|办法|规定|条例|实施细则|意见|通知|公告|规范|标准|要求|指引|制度|细则)$")


def _normalize_title(s: str) -> str:
    """Strip country/admin prefixes and version/year markers from a title for matching."""
    s = s.strip()
    s = _TITLE_NOISE.sub("", s)
    s = _TITLE_NOISE.sub("", s)  # second pass for residual compound noise
    s = _TITLE_SUFFIX.sub("", s) if len(s) > 6 else s
    return s.strip()


def _ratio(a: str, b: str) -> float:
    """SequenceMatcher ratio on normalized titles. (0,1]."""
    return SequenceMatcher(None, _normalize_title(a), _normalize_title(b)).ratio()


def build_title_index(nodes: list[dict]) -> dict[str, dict]:
    """rel_path → node. Also builds a (title → node) lookup used by the matcher."""
    index: dict[str, dict] = {}
    by_title: dict[str, list[dict]] = defaultdict(list)
    for n in nodes:
        rel = n.get("relPath", "")
        if not rel or rel == "index.md":
            continue
        index[rel] = n
        title = (n.get("title") or "").strip()
        if title:
            by_title[title].append(n)
    index["__by_title__"] = dict(by_title)  # type: ignore[assignment]
    return index


def match_citation(text: str, by_title: dict[str, list[dict]], src_rel_path: str = "") -> list[dict]:
    """Fuzzy-match citation text against bundle titles. Returns ranked candidates.

    Drops self-matches (where the only candidate is the source file itself) —
    a file citing its own name in its own body shouldn't produce a self-loop.
    """
    candidates = []
    for title, nodes in by_title.items():
        score = _ratio(text, title)
        if score >= REVIEW_THRESHOLD:
            # Best-scoring node wins (in rare cases two nodes share a title)
            best = max(nodes, key=lambda n: len(n.get("relPath", "")))
            candidates.append({"title": title, "score": round(score, 3), "rel_path": best["relPath"]})
    candidates.sort(key=lambda c: c["score"], reverse=True)
    # Drop self-matches: a citation whose only candidate is the source file
    # would create a self-loop. Keep other candidates if they exist.
    if src_rel_path:
        non_self = [c for c in candidates if c["rel_path"] != src_rel_path]
        if non_self or any(c["rel_path"] == src_rel_path for c in candidates):
            candidates = non_self
    return candidates[:5]


def find_citations(body: str) -> list[dict]:
    """Find citation candidates in a file body. Returns list of {text, line, candidates}."""
    out = []
    seen = set()  # dedupe (text, line) — same title cited multiple times on the same line is one edge
    for line_no, line in enumerate(body.splitlines(), 1):
        for rx in CITATION_RES:
            for m in rx.finditer(line):
                text = m.group(1).strip()
                if len(text) < 3 or text.isdigit():
                    continue
                # `相关法规：` pattern yields a comma/、/; separated list
                for part in re.split(r"[,，、;；]", text):
                    part = part.strip().rstrip(".。")
                    if len(part) < 3 or part.isdigit():
                        continue
                    key = (part, line_no)
                    if key in seen:
                        continue
                    seen.add(key)
                    out.append({"text": part, "line": line_no, "context": line.strip()[:100]})
    return out


def find_broken_links_in_body(body: str, live_paths: set[str], src_rel_path: str) -> list[dict]:
    """Find bundle-relative links whose targets aren't in live_paths. For apply-mode strip."""
    out = []
    src_dir = str(Path(src_rel_path).parent)
    for line_no, line in enumerate(body.splitlines(), 1):
        for m in _BUNDLE_LINK_RE.finditer(line):
            text, url = _link_text_url(m)
            if not url:
                continue
            # Classify: external (http, mailto, etc.) and pure anchors are fine
            if url.startswith(("http://", "https://", "mailto:", "tel:", "ftp:", "#")):
                continue
            if url.startswith("javascript:"):
                continue  # phase-1 already handled
            # Bundle-relative resolution
            if url.startswith("./"):
                target = str(Path(src_dir) / url[2:])
            elif url.startswith("/"):
                target = url.lstrip("/")
            else:
                target = str(Path(src_dir) / url)
            target = target.replace("\\", "/")
            if target not in live_paths:
                out.append({
                    "line": line_no,
                    "text": text[:50],
                    "url": url[:100],
                    "resolved": target[:120],
                    "raw": m.group(0),
                })
    return out


def fetch_broken_links(bundle_id: int) -> list[dict]:
    """Page through /okf/bundles/:id/broken-links via SDK. Returns full list."""
    client = _get_client()
    all_links, cursor = [], None
    while True:
        path = f"/okf/bundles/{bundle_id}/broken-links"
        params = {"cursor": cursor} if cursor else None
        data = client._get(path, params=params)
        all_links.extend(data.get("brokenLinks", []))
        cursor = data.get("nextCursor")
        if not cursor:
            break
    return all_links


def scan(
    bundle_id: int,
    raw_root: Path,
    live_paths: set[str],
    title_index: dict[str, dict],
) -> dict:
    """Walk raw/, find citations + broken links per file. Returns the scan result."""
    by_title = title_index["__by_title__"]  # type: ignore[index]
    files_out = []

    top_dirs = sorted(d.name for d in raw_root.iterdir() if d.is_dir() and not d.name.startswith("."))
    for d in top_dirs:
        if d in SKIP_DIRS:
            continue
        listed = os.path.exists(raw_root / d)
        if not listed:
            continue
        for sub in sorted((raw_root / d).rglob("*")):
            if not sub.is_file() or sub.suffix.lower() not in TEXT_EXTS:
                continue
            rel_path = str(sub.relative_to(raw_root)).replace("\\", "/")
            read = read_raw_file(path=rel_path)
            if not read.get("ok"):
                continue
            body = strip_frontmatter(read["data"]["content"])
            body = _JUNK_LINK_RE.sub(lambda m: m.group("text"), body)
            body = dedupe_same_line_links(body)

            citations = find_citations(body)
            for c in citations:
                c["candidates"] = match_citation(c["text"], by_title, src_rel_path=rel_path)
                c["decision"] = (
                    "accept" if c["candidates"] and c["candidates"][0]["score"] >= MATCH_THRESHOLD
                    else "review" if c["candidates"] and c["candidates"][0]["score"] >= REVIEW_THRESHOLD
                    else "no_match"
                )

            broken = find_broken_links_in_body(body, live_paths, rel_path)

            files_out.append({
                "rel_path": rel_path,
                "citations": citations,
                "broken_links": broken,
            })

    return {
        "bundle_id": bundle_id,
        "match_threshold": MATCH_THRESHOLD,
        "review_threshold": REVIEW_THRESHOLD,
        "files": files_out,
    }


def render_markdown(scan_result: dict) -> str:
    """Render a human-readable summary."""
    lines = []
    lines.append(f"# Phase-2 Scan — Bundle {scan_result['bundle_id']}\n")
    lines.append(f"- match threshold: `{scan_result['match_threshold']}` (auto-apply)")
    lines.append(f"- review threshold: `{scan_result['review_threshold']}` (surface for review)")
    lines.append("")

    n_accept = sum(
        1 for f in scan_result["files"] for c in f["citations"] if c["decision"] == "accept"
    )
    n_review = sum(
        1 for f in scan_result["files"] for c in f["citations"] if c["decision"] == "review"
    )
    n_no_match = sum(
        1 for f in scan_result["files"] for c in f["citations"] if c["decision"] == "no_match"
    )
    n_broken = sum(len(f["broken_links"]) for f in scan_result["files"])
    n_files_with_work = sum(
        1 for f in scan_result["files"] if f["citations"] or f["broken_links"]
    )

    lines.append("## Summary\n")
    lines.append(f"- files scanned: {len(scan_result['files'])}")
    lines.append(f"- files with citations or broken links: {n_files_with_work}")
    lines.append(f"- citations auto-accepted: **{n_accept}**")
    lines.append(f"- citations needing review: **{n_review}**")
    lines.append(f"- citations with no match: {n_no_match}")
    lines.append(f"- broken links to strip: {n_broken}")
    lines.append("")

    # Per-file detail — only files with action
    lines.append("## Per-file detail\n")
    for f in scan_result["files"]:
        accepts = [c for c in f["citations"] if c["decision"] == "accept"]
        reviews = [c for c in f["citations"] if c["decision"] == "review"]
        broken = f["broken_links"]
        if not (accepts or reviews or broken):
            continue
        lines.append(f"### `{f['rel_path']}`\n")
        if accepts:
            lines.append(f"**Auto-apply citations ({len(accepts)}):**\n")
            lines.append("| Text | Best match | Score | Dst |")
            lines.append("|---|---|---|---|")
            for c in accepts[:10]:
                top = c["candidates"][0]
                lines.append(f"| `{c['text'][:30]}` | {top['title']} | {top['score']} | `{top['rel_path']}` |")
            if len(accepts) > 10:
                lines.append(f"\n_…and {len(accepts) - 10} more_\n")
            lines.append("")
        if reviews:
            lines.append(f"**Needs review ({len(reviews)}):**\n")
            lines.append("| Text | Best match | Score | Dst |")
            lines.append("|---|---|---|---|")
            for c in reviews[:10]:
                top = c["candidates"][0]
                lines.append(f"| `{c['text'][:30]}` | {top['title']} | {top['score']} | `{top['rel_path']}` |")
            lines.append("")
        if broken:
            lines.append(f"**Broken links to strip ({len(broken)}):**\n")
            for b in broken[:5]:
                lines.append(f"- L{b['line']}: `[{b['text']}]({b['url']})` → resolves to `{b['resolved']}`")
            if len(broken) > 5:
                lines.append(f"\n_…and {len(broken) - 5} more_\n")
            lines.append("")

    return "\n".join(lines) + "\n"


def main() -> int:
    p = argparse.ArgumentParser()
    p.add_argument("--summary", action="store_true", help="print compact summary after scan")
    p.add_argument("--apply", action="store_true", help="apply accepted candidates (rewrites files)")
    args = p.parse_args()

    raw_root = Path(os.environ["AGENTDISK_RAW_ROOT"])

    # 1. Build title index + live-path set from bundle nodes
    r = list_nodes(bundle_id=BUNDLE_ID)
    if not r.get("ok"):
        print(f"list_nodes failed: {r.get('error')}", file=sys.stderr)
        return 1
    nodes = r["data"]["nodes"]
    title_index = build_title_index(nodes)
    live_paths = {n["relPath"] for n in nodes if n.get("relPath")}
    print(f"[scan] bundle {BUNDLE_ID}: {len(nodes)} nodes, {len(live_paths)} live paths", file=sys.stderr)

    # 2. Scan
    result = scan(BUNDLE_ID, raw_root, live_paths, title_index)

    # 3. Dump JSON + MD
    out_json = Path(__file__).parent / "phase2_scan.json"
    out_md = Path(__file__).parent / "phase2_scan.md"
    out_json.write_text(json.dumps(result, ensure_ascii=False, indent=2))
    out_md.write_text(render_markdown(result))
    print(f"[scan] wrote {out_json.relative_to(HERE)} + {out_md.relative_to(HERE)}", file=sys.stderr)

    if args.summary:
        n_accept = sum(1 for f in result["files"] for c in f["citations"] if c["decision"] == "accept")
        n_review = sum(1 for f in result["files"] for c in f["citations"] if c["decision"] == "review")
        n_broken = sum(len(f["broken_links"]) for f in result["files"])
        print(f"[summary] auto-apply: {n_accept}  review: {n_review}  broken-links: {n_broken}")

    if not args.apply:
        print("[hint] re-run with --apply to write accepted candidates", file=sys.stderr)
        return 0

    # 4. Apply mode — rewrite each file with verified links + stripped broken links
    n_written = 0
    n_skipped = 0
    n_total_links_added = 0
    n_total_broken_stripped = 0
    for f in result["files"]:
        # Dedupe accepts by citation text — multiple citations of the same title
        # in the same file all point to the same target, so one link is enough.
        # Also drop captures containing `《` (regex over-captured into nested
        # brackets; the reconstructed `《text》` won't appear verbatim in body).
        accepts: list[dict] = []
        seen_text: set[str] = set()
        for c in f["citations"]:
            if c["decision"] != "accept":
                continue
            t = c["text"]
            if "《" in t or "]" in t or t in seen_text:
                continue
            seen_text.add(t)
            accepts.append(c)
        broken = f["broken_links"]
        if not accepts and not broken:
            n_skipped += 1
            continue

        read = read_raw_file(path=f["rel_path"])
        if not read.get("ok"):
            print(f"  ✗ read  {f['rel_path']}: {read.get('error')}", file=sys.stderr)
            continue
        body = strip_frontmatter(read["data"]["content"])
        body = _JUNK_LINK_RE.sub(lambda m: m.group("text"), body)
        body = dedupe_same_line_links(body)

        # Strip broken bundle-relative links (replace `[text](url)` with `text`).
        # Many broken links are scraped image residue where `text` is `![` or
        # empty — for those we just drop the whole markdown node.
        broken_stripped = 0
        for b in broken:
            if b["raw"] in body:
                replacement = b["text"] if b["text"] and not b["text"].startswith("![") else ""
                body = body.replace(b["raw"], replacement, 1)
                broken_stripped += 1

        # Inject verified citation links. We use a sentinel-aware replace: walk
        # occurrences of `《text》` and skip any already wrapped in `[...](...)`
        # (from an earlier accept in this loop). Naive `body.replace()` would
        # match `《text》` inside `[《text》](url)` and corrupt it.
        links_added = 0
        for c in accepts:
            top = c["candidates"][0]
            target_rel = top["rel_path"]
            src_dir = str(Path(f["rel_path"]).parent)
            link_rel = os.path.relpath(target_rel, src_dir).replace("\\", "/")
            if not link_rel.startswith(("./", "../", "/")):
                link_rel = "./" + link_rel
            needle = f"《{c['text']}》"
            new_md = f"[{needle}]({link_rel})"
            # Find first occurrence of needle NOT preceded by `[` (which would
            # mean it's already inside a link's text). This is more robust than
            # a regex because the body may have whitespace between `[` and `《`.
            idx = 0
            while idx < len(body):
                pos = body.find(needle, idx)
                if pos < 0:
                    break
                # Look back up to 5 chars for a `[`
                pre = body[max(0, pos - 5):pos]
                if "[" in pre and pre.rindex("[") > pre.rindex("]") if "]" in pre else "[" in pre:
                    idx = pos + len(needle)
                    continue
                body = body[:pos] + new_md + body[pos + len(needle):]
                links_added += 1
                idx = pos + len(new_md)
                break  # one link per unique citation text per file

        if not links_added and not broken_stripped:
            n_skipped += 1
            continue

        new_content = build_markdown_content(body, Path(f["rel_path"]).name)
        wrote = write_markdown(rel_path=f["rel_path"], content=new_content)
        if wrote.get("ok"):
            n_written += 1
            n_total_links_added += links_added
            n_total_broken_stripped += broken_stripped
            print(f"  ✓ {f['rel_path']}  (+{links_added} links, -{broken_stripped} broken)")
        else:
            print(f"  ✗ write {f['rel_path']}: {str(wrote.get('error', ''))[:120]}")

    print(f"\n[apply] files written: {n_written}  skipped: {n_skipped}")
    print(f"[apply] total citation links added: {n_total_links_added}")
    print(f"[apply] total broken-link nodes stripped: {n_total_broken_stripped}")
    return 0


if __name__ == "__main__":
    sys.exit(main())
