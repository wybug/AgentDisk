"""AgentDisk writer agent (Google ADK 2.0).

Importing this package wires up the OKF maintenance agent. ADK discovers the
``root_agent`` symbol when you run ``adk run`` / ``adk web`` from the parent
directory.
"""

from __future__ import annotations

from .agent import root_agent

__all__ = ["root_agent"]
