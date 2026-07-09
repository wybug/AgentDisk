"""AgentDisk knowledge-base business-QA agent (Google ADK 2.0).

Importing this package wires up the ``kb_bot`` consumer agent. ADK discovers
the ``root_agent`` symbol when you run ``adk run`` / ``adk web`` from the
parent directory.

The agent is the read-side counterpart to ``adk_writer_agent``: writer
builds the OKF bundle, bot consumes it to answer business questions with
citations. All backend access goes through the ``agentdisk`` Python SDK
(no hand-rolled HTTP).
"""

from __future__ import annotations

from .agent import root_agent

__all__ = ["root_agent"]
