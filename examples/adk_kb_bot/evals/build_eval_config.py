"""Generate ``eval_config.json`` for the rubric-based metrics.

ADK 2.x rubric-based metrics require ``criterion.rubrics`` to be non-empty
at evaluator-construction time (an assertion in
``RubricBasedEvaluator.__init__``). The framework later merges in the
per-invocation rubrics that live in the eval set, so the criterion only
needs a *placeholder* rubric to satisfy the assertion. Putting the real
rubrics here too causes a duplicate-id conflict during the merge.

This script emits a minimal config: one placeholder rubric per metric,
plus a judge-model override pointing at the same LiteLLM model the agent
uses (so we don't fall back to Gemini, which would need a Google API key).

Usage::

    python evals/build_eval_config.py
"""

from __future__ import annotations

import json
import os
from pathlib import Path

_PKG_ROOT = Path(__file__).resolve().parent.parent
EVAL_CONFIG_PATH = _PKG_ROOT / "evals" / "eval_config.json"

# Default judge model — same env var the agent reads. Falls back to
# deepseek-chat so the auto-rater doesn't silently switch to Gemini
# (Gemini would require GEMINI_API_KEY and we don't ship one).
JUDGE_MODEL = os.environ.get("KB_BOT_MODEL", "deepseek/deepseek-chat")

_PLACEHOLDER_TOOL_USE = {
    "rubricId": "_criterion_placeholder_tool_use",
    "rubricContent": {
        "text_property": (
            "Placeholder rubric; real rubrics are merged in from the"
            " invocation (see local_eval_service._copy_eval_case_rubrics"
            "_to_actual_invocations)."
        )
    },
    "type": "TOOL_USE_QUALITY",
}

_PLACEHOLDER_FINAL_RESPONSE = {
    "rubricId": "_criterion_placeholder_final_response",
    "rubricContent": {
        "text_property": (
            "Placeholder rubric; real rubrics are merged in from the"
            " invocation."
        )
    },
    "type": "FINAL_RESPONSE_QUALITY",
}


def main() -> int:
    config = {
        "criteria": {
            "rubric_based_tool_use_quality_v1": {
                "threshold": 1.0,
                "judge_model_options": {"judge_model": JUDGE_MODEL},
                "rubrics": [_PLACEHOLDER_TOOL_USE],
            },
            "rubric_based_final_response_quality_v1": {
                "threshold": 1.0,
                "judge_model_options": {"judge_model": JUDGE_MODEL},
                "rubrics": [_PLACEHOLDER_FINAL_RESPONSE],
            },
        }
    }
    EVAL_CONFIG_PATH.write_text(
        json.dumps(config, ensure_ascii=False, indent=2) + "\n", encoding="utf-8"
    )
    print(f"[ok] wrote {EVAL_CONFIG_PATH.name} (judge={JUDGE_MODEL})")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())

