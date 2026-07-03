"""Pytest fixtures for adk_writer_agent raw-file tools.

Builds a deterministic virtual RAW_ROOT under ``tmp_path`` so tests can
assert scan / read / chunk behavior without touching real package data.
The fixture tree covers the cases the tools must handle:

* ``note.md`` (root-level small markdown)
* ``sub/concept.md`` (nested normal markdown)
* ``.hidden.md`` (hidden — should land in skipped)
* ``node_modules/x.md`` (noise dir — should land in skipped)
* ``binary.png`` (non-text ext — should land in skipped)
* ``with_front.md`` (markdown with YAML frontmatter to exercise the parser)
* ``big.md`` (250 KB body — exercises non-chunked truncation)
* ``pci_dss.md`` (250 KB body with multiple H2 — exercises chunk splitting)
* ``with_steps.md`` (small markdown with ``## 步骤`` — exercises h2 hint)
"""

from __future__ import annotations

from pathlib import Path
from typing import Any

import pytest


def _write(path: Path, content: str) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(content, encoding="utf-8")


@pytest.fixture()
def raw_root(tmp_path: Path, monkeypatch: pytest.MonkeyPatch) -> Path:
    """Create a virtual RAW_ROOT and point AGENTDISK_RAW_ROOT at it."""
    root = tmp_path / "raw"
    root.mkdir()

    _write(root / "note.md", "# Note\n\nA small note.\n")
    _write(root / "sub" / "concept.md", "# Concept\n\nBody.\n")
    _write(root / ".hidden.md", "# Hidden\n\nshould be skipped\n")
    _write(root / "node_modules" / "x.md", "# NodeModules\n\nskipped\n")
    _write(root / "binary.png", "\x89PNG\r\n\x1a\n fake png bytes")

    _write(
        root / "with_front.md",
        "---\ntitle: With Front\ntype: concept\ntags: [a, b]\n---\n\n# With Front\n\nbody.\n",
    )

    _write(
        root / "with_steps.md",
        "# Playbook\n\n## 步骤\n\n1. first\n2. second\n\n## Reference\n\nfoo\n",
    )

    # 250 KB normal body (no H2): truncation in non-chunked mode.
    _write(root / "big.md", "# Big\n\n" + ("x" * (250 * 1024)))

    # 250 KB body with multiple H2 headings: chunked mode splits into chunks.
    h2_section = "## Section {i}\n\n" + ("y" * (30 * 1024)) + "\n\n"
    sections = [h2_section.format(i=i) for i in range(8)]
    _write(root / "pci_diss.md", "# PCI DSS\n\n" + "".join(sections))

    monkeypatch.setenv("AGENTDISK_RAW_ROOT", str(root))
    return root


@pytest.fixture()
def unset_raw_root(monkeypatch: pytest.MonkeyPatch) -> None:
    """Ensure AGENTDISK_RAW_ROOT is not set — used by "unset" tests."""
    monkeypatch.delenv("AGENTDISK_RAW_ROOT", raising=False)


def make_file_info(rel_path: str, **overrides: Any) -> dict[str, Any]:
    """Helper for tests to build expected file-info dicts without repeating
    boilerplate. Override only the fields that matter for the assertion."""
    base: dict[str, Any] = {
        "rel_path": rel_path,
        "size": 0,
        "ext": ".md",
        "size_bucket": "small",
        "hint": {"first_line": "", "h2_count": 0},
    }
    base.update(overrides)
    return base
