"""Path-traversal and missing-config tests for raw-file tools.

The tools MUST reject:
* Absolute paths (POSIX ``/etc/...`` and any absolute form)
* Any path containing ``..`` segments
* Operations when AGENTDISK_RAW_ROOT is unset AND no default package
  raw/ directory exists

These are the security boundary — failures here could expose arbitrary
local files to the LLM.
"""

from __future__ import annotations

from pathlib import Path

import pytest

from adk_writer_agent.tools import list_raw_files, read_raw_file


def test_list_raw_files_rejects_dotdot(raw_root: Path) -> None:
    result = list_raw_files(dir="../../../etc")
    assert result["ok"] is False
    assert "traversal" in result["error"].lower() or ".." in result["error"]


def test_list_raw_files_rejects_absolute_path(raw_root: Path) -> None:
    result = list_raw_files(dir="/etc/passwd")
    assert result["ok"] is False
    assert "absolute" in result["error"].lower()


def test_read_raw_file_rejects_dotdot(raw_root: Path) -> None:
    result = read_raw_file(path="../secret.md")
    assert result["ok"] is False
    assert "traversal" in result["error"].lower() or ".." in result["error"]


def test_raw_root_unset_returns_error(
    unset_raw_root: None,
    tmp_path: Path,
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    # Change cwd so the package-default raw/ fallback also doesn't resolve
    # to the real package raw/ directory (which would otherwise let the
    # tool succeed silently).
    monkeypatch.chdir(tmp_path)

    # Patch __file__ resolution path by pointing module at a fake location
    # is overkill — instead just confirm the env-unset error is reported
    # when the default also can't be found. We simulate "no default" by
    # importing _raw_root and pointing it at a non-existent raw subdir.
    from adk_writer_agent import tools as tools_mod

    original_file = tools_mod.__file__
    fake_file = tmp_path / "fake_pkg" / "tools.py"
    fake_file.parent.mkdir(parents=True)
    fake_file.write_text("")
    monkeypatch.setattr(tools_mod, "__file__", str(fake_file))

    result = list_raw_files()
    assert result["ok"] is False
    assert "AGENTDISK_RAW_ROOT" in result["error"]

    result2 = read_raw_file(path="anything.md")
    assert result2["ok"] is False
    assert "AGENTDISK_RAW_ROOT" in result2["error"]

    # Restore so other tests in the session see the real __file__.
    monkeypatch.setattr(tools_mod, "__file__", original_file)
