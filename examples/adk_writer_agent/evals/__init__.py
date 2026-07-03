"""OKF writer agent evaluation suite (ADK eval mode).

This package is intentionally empty — eval cases live as JSON files in
this directory so they can be loaded directly by ``adk eval`` without
needing Python imports. The Python entry points are:

* :mod:`evals.run_evals` — wrapper that invokes ``adk eval`` and emits a
  markdown report.
* :mod:`evals.conftest` — pytest hooks (currently a no-op; ADK's own
  ``reset_data`` hook on the agent module is what does state isolation).
"""
