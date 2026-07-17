"""Unit tests for credential propagation to the OKF sub-client (T1.2) and for
OKF type re-export from the package root.

No live server required — these only exercise property setters and the package's
public surface. Regression guard for the bug where switching token/api_key on the
top-level client left ``_wiki`` holding the OLD credential, so a JWT->API-Key
switch silently kept authorizing reader/writer calls with the stale token.
"""

from __future__ import annotations

import agentdisk
from agentdisk import AgentDiskClient, AsyncAgentDiskClient


def test_sync_setters_propagate_to_wiki_subclient():
    client = AgentDiskClient(base_url="http://localhost:9100", token="jwt-A")
    try:
        # Constructed with a token: the wiki sub-client must already see it.
        assert client._wiki._token == "jwt-A"
        assert client._wiki._api_key == ""

        # Rotating the token propagates to _wiki (previously dropped).
        client.token = "jwt-B"
        assert client._wiki._token == "jwt-B"

        # Switching to an API key (the documented writer-auth pattern) propagates.
        client.api_key = "key-1"
        assert client._wiki._api_key == "key-1"
    finally:
        client.close()


def test_async_setters_propagate_to_wiki_subclient():
    client = AsyncAgentDiskClient(base_url="http://localhost:9100", token="jwt-A")
    assert client._wiki._token == "jwt-A"
    client.token = "jwt-B"
    assert client._wiki._token == "jwt-B"
    client.api_key = "key-1"
    assert client._wiki._api_key == "key-1"


def test_okf_types_reexported_from_package():
    # OKF data models must be importable straight off the package root so
    # `from agentdisk import OkfBundle` and IDE autocomplete work.
    for name in (
        "OkfBundle",
        "OkfNode",
        "SearchPage",
        "TypeCount",
        "BrokenLinksPage",
        "BundleStats",
        "NeighborsResult",
        "SubgraphResult",
        "ShortestPathResult",
        "ScanReport",
        "IndexRegenResult",
    ):
        assert hasattr(agentdisk, name), f"{name} not re-exported from agentdisk"
        assert name in agentdisk.__all__, f"{name} missing from agentdisk.__all__"
