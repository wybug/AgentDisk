"""Scan / filter / hint / truncation tests for ``list_raw_files``."""

from __future__ import annotations

from pathlib import Path
from typing import Any

import pytest

from adk_writer_agent import tools as tools_mod
from adk_writer_agent.tools import list_raw_files


def _rel_paths(payload: dict[str, Any]) -> set[str]:
    return {f["rel_path"] for f in payload["data"]["files"]}


def test_normal_scan_returns_text_files(raw_root: Path) -> None:
    result = list_raw_files()
    assert result["ok"] is True
    rels = _rel_paths(result)
    assert "note.md" in rels
    assert "sub/concept.md" in rels
    assert "with_front.md" in rels
    assert "with_steps.md" in rels
    assert "big.md" in rels
    assert "pci_diss.md" in rels


def test_hidden_files_go_to_skipped(raw_root: Path) -> None:
    result = list_raw_files()
    assert result["ok"] is True
    assert ".hidden.md" in result["data"]["skipped"]
    assert ".hidden.md" not in _rel_paths(result)


def test_node_modules_files_go_to_skipped(raw_root: Path) -> None:
    result = list_raw_files()
    assert result["ok"] is True
    assert "node_modules/x.md" in result["data"]["skipped"]
    assert "node_modules/x.md" not in _rel_paths(result)


def test_non_text_extension_go_to_skipped(raw_root: Path) -> None:
    result = list_raw_files()
    assert result["ok"] is True
    assert "binary.png" in result["data"]["skipped"]
    assert "binary.png" not in _rel_paths(result)


def test_hint_first_line_extracts_title(raw_root: Path) -> None:
    result = list_raw_files()
    by_path = {f["rel_path"]: f for f in result["data"]["files"]}
    assert by_path["note.md"]["hint"]["first_line"] == "Note"
    assert by_path["sub/concept.md"]["hint"]["first_line"] == "Concept"
    assert by_path["with_front.md"]["hint"]["first_line"] == "With Front"


def test_hint_h2_count_counts_top_level_headings(raw_root: Path) -> None:
    result = list_raw_files()
    by_path = {f["rel_path"]: f for f in result["data"]["files"]}
    # with_steps.md has two H2 ("## 步骤", "## Reference")
    assert by_path["with_steps.md"]["hint"]["h2_count"] == 2
    # note.md has zero H2 (only H1)
    assert by_path["note.md"]["hint"]["h2_count"] == 0


def test_size_bucket_classification(raw_root: Path) -> None:
    result = list_raw_files()
    by_path = {f["rel_path"]: f for f in result["data"]["files"]}
    assert by_path["note.md"]["size_bucket"] == "small"  # tiny
    assert by_path["big.md"]["size_bucket"] == "large"  # 250 KB
    assert by_path["pci_diss.md"]["size_bucket"] == "large"


def test_truncated_when_more_than_max_files(
    raw_root: Path, monkeypatch: pytest.MonkeyPatch
) -> None:
    # Lower the cap to 3 to force truncation without creating 500 files.
    monkeypatch.setattr(tools_mod, "_MAX_FILES", 3)

    result = list_raw_files()
    assert result["ok"] is True
    assert result["data"]["truncated"] is True
    assert len(result["data"]["files"]) == 3


def test_subdir_scan_isolated_to_dir(raw_root: Path) -> None:
    result = list_raw_files(dir="sub")
    assert result["ok"] is True
    rels = _rel_paths(result)
    # Only files under sub/ should be returned; the path is reported relative
    # to RAW_ROOT not to the requested dir (matches the docstring: "root",
    # "dir", "files" with rel_path under root).
    assert any(r.startswith("sub/") for r in rels)
    assert "note.md" not in rels  # note.md is at root, not under sub/
