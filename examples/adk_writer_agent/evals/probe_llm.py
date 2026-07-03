"""Quick LLM connectivity probe — verify model id + API key before `adk eval`.

Sends a trivial 1-message chat completion through ADK's LlmAgent so we
catch model-id / auth issues BEFORE spending money on the full eval suite.
Errors here usually mean:

* ``WRITER_AGENT_MODEL`` is malformed (LiteLLM expects ``provider/model``).
* The matching ``*_API_KEY`` env var is missing or wrong.
* ``DEEPSEEK_API_BASE`` (if set) points somewhere unreachable.

Usage::

    python evals/probe_llm.py
    WRITER_AGENT_MODEL=gemini-2.5-flash python evals/probe_llm.py

Exit codes: 0 = ok, 1 = config error, 2 = LLM call failed.
"""

from __future__ import annotations

import os
import sys
import time
from pathlib import Path

_PKG_ROOT = Path(__file__).resolve().parent.parent
if str(_PKG_ROOT) not in sys.path:
    sys.path.insert(0, str(_PKG_ROOT))


def main() -> int:
    model = os.environ.get("WRITER_AGENT_MODEL", "deepseek/deepseek-chat")
    print(f"[probe] model = {model!r}")

    # Detect the matching API key requirement — same logic as run_evals.py.
    model_lower = model.lower()
    if "deepseek" in model_lower:
        api_key_var = "DEEPSEEK_API_KEY"
    elif "gemini" in model_lower:
        api_key_var = "GEMINI_API_KEY" if os.environ.get("GEMINI_API_KEY") else "GOOGLE_API_KEY"
    elif "claude" in model_lower or "anthropic" in model_lower:
        api_key_var = "ANTHROPIC_API_KEY"
    else:
        api_key_var = "(unknown)"

    key = os.environ.get(api_key_var) if api_key_var != "(unknown)" else None
    if not key:
        print(f"[probe] FAIL: {api_key_var} not set", file=sys.stderr)
        return 1
    print(f"[probe] {api_key_var} = {key[:6]}...{key[-4:]} (len={len(key)})")

    # Construct LlmAgent + send a single short prompt. We bypass the full
    # tool set so this works even if the backend isn't running — we just
    # want to verify the LLM provider picks up the model + key correctly.
    try:
        from google.adk.agents import LlmAgent
        from google.adk.runners import Runner
        from google.adk.sessions.in_memory_session_service import (
            InMemorySessionService,
        )
        from google.genai import types as genai_types
    except ImportError as exc:
        print(f"[probe] FAIL: google-adk not installed: {exc}", file=sys.stderr)
        return 1

    agent = LlmAgent(
        name="probe",
        model=model,
        instruction="You are a connectivity probe. Reply with the single word PONG.",
    )

    session_service = InMemorySessionService()
    # create_session is async; create_session_sync is deprecated but works
    # fine for a one-shot probe. We avoid pulling in asyncio.run because
    # Runner.run (below) is a sync generator.
    session = session_service.create_session_sync(  # type: ignore[attr-defined]
        app_name="probe", user_id="probe_user"
    )
    runner = Runner(app_name="probe", agent=agent, session_service=session_service)

    user_msg = genai_types.Content(role="user", parts=[genai_types.Part(text="ping")])

    print("[probe] sending ping...")
    start = time.perf_counter()
    final_text = ""
    try:
        for event in runner.run(user_id="probe_user", session_id=session.id, new_message=user_msg):
            if event.content and event.content.parts:
                for part in event.content.parts:
                    if part.text:
                        final_text += part.text
    except Exception as exc:  # noqa: BLE001 — surface the real error verbatim
        elapsed = time.perf_counter() - start
        print(f"[probe] FAIL after {elapsed:.1f}s: {type(exc).__name__}: {exc}", file=sys.stderr)
        return 2

    elapsed = time.perf_counter() - start
    final_text = final_text.strip() or "(empty)"
    print(f"[probe] ok in {elapsed:.1f}s — response: {final_text!r}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
