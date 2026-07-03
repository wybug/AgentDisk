"""Read / frontmatter-parse / chunk tests for ``read_raw_file``."""

from __future__ import annotations

from pathlib import Path

from adk_writer_agent.tools import read_raw_file


def test_normal_file_returns_full_content(raw_root: Path) -> None:
    result = read_raw_file(path="note.md")
    assert result["ok"] is True
    data = result["data"]
    assert data["truncated"] is False
    assert data["chunked"] is False if "chunked" in data else True
    assert "# Note" in data["content"]
    assert data["frontmatter"] is None  # note.md has no frontmatter


def test_with_frontmatter_parses_yaml(raw_root: Path) -> None:
    result = read_raw_file(path="with_front.md")
    assert result["ok"] is True
    data = result["data"]
    fm = data["frontmatter"]
    assert fm is not None
    assert fm["title"] == "With Front"
    assert fm["type"] == "concept"
    assert fm["tags"] == ["a", "b"]
    # content still includes the original frontmatter block
    assert data["content"].startswith("---\n")


def test_big_file_non_chunk_truncates_at_cap(raw_root: Path) -> None:
    result = read_raw_file(path="big.md")  # chunk=False default
    assert result["ok"] is True
    data = result["data"]
    assert data["truncated"] is True
    # Content capped at 200 KB.
    assert len(data["content"].encode("utf-8")) <= 200 * 1024
    # Size field reports the true on-disk size, not the truncated length.
    assert data["size"] > 200 * 1024


def test_chunked_split_returns_multiple_chunks(raw_root: Path) -> None:
    result = read_raw_file(path="pci_diss.md", chunk=True)
    assert result["ok"] is True
    data = result["data"]
    assert data.get("chunked") is True
    chunks = data["chunks"]
    assert len(chunks) >= 2  # at least 2 chunks
    # Each chunk has the expected fields.
    for i, chunk in enumerate(chunks, start=1):
        assert chunk["index"] == i
        assert "heading" in chunk
        assert "approx_size_kb" in chunk
        assert "content" in chunk


def test_each_chunk_within_size_cap(raw_root: Path) -> None:
    result = read_raw_file(path="pci_diss.md", chunk=True)
    assert result["ok"] is True
    for chunk in result["data"]["chunks"]:
        chunk_bytes = len(chunk["content"].encode("utf-8"))
        assert chunk_bytes <= 200 * 1024, (
            f"chunk {chunk['index']} exceeds 200 KB: {chunk_bytes} bytes"
        )
