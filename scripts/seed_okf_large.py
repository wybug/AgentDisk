#!/usr/bin/env python3
"""Seed an OKF bundle with a parameterized graph for perf validation.

Generates N markdown files with controlled edge topology, writes them via the
API-key writer endpoint, then registers / refreshes the bundle so the OKF
materializer rebuilds the node + edge tables.

Supported distributions:
    uniform   — each file links to E random other files (E = --edges-per-node)
    powerlaw  — each file links to E files sampled from a Zipf; a few hubs
                receive most in-links (closer to real OKF topology)
    chain     — linear chain file[i] → file[i+1] + (E-1) random shortcuts

The seed is fixed (rand seed 42) so two runs at the same params produce
identical graphs — required for benchstat to compare across backend changes
without graph-shape noise.

Idempotent: looks for an existing bundle with the target name and reuses it;
overwrites files in place. Rerun with the same args to refresh state after a
backend change.

Usage:
    python3 scripts/seed_okf_large.py --nodes 1000
    python3 scripts/seed_okf_large.py --nodes 5000 --edges-per-node 10 --dist powerlaw
    python3 scripts/seed_okf_large.py --bundle-name "Bench 1K powerlaw" \\
        --nodes 1000 --dist powerlaw

Prerequisites:
    - Backend running on :9100 (bash scripts/dev.sh start)
    - Admin credentials (default admin / admin123)
    - agentdisk SDK importable (cd sdk && pip install -e .)
"""

from __future__ import annotations

import argparse
import math
import os
import random
import sys
import time
from typing import Any

import httpx

try:
    from agentdisk import AgentDiskClient
except ImportError:
    sys.path.insert(0, os.path.join(os.path.dirname(__file__), "..", "sdk", "src"))
    from agentdisk import AgentDiskClient  # type: ignore

BASE_URL = os.environ.get("AGENTDISK_URL", "http://localhost:9100")
ADMIN_USER = os.environ.get("AGENTDISK_ADMIN_USER", "admin")
ADMIN_PASS = os.environ.get("AGENTDISK_ADMIN_PASS", "admin123")
DEFAULT_SCOPE = "global"
SEED = 42


def _http_msg(resp: httpx.Response, default: str) -> str:
    try:
        body = resp.json()
        return str(body.get("message") or body.get("data") or default)
    except Exception:
        return default


def admin_login() -> str:
    resp = httpx.post(
        f"{BASE_URL}/v1/disk/admin/login",
        json={"username": ADMIN_USER, "password": ADMIN_PASS},
        timeout=10,
    )
    if resp.status_code != 200:
        raise RuntimeError(f"admin login failed: {resp.status_code} {_http_msg(resp, 'login error')}")
    return resp.json()["data"]["token"]


def ensure_public_directory(admin_token: str, name: str) -> dict[str, Any]:
    resp = httpx.get(
        f"{BASE_URL}/v1/disk/admin/public-directories",
        headers={"Authorization": f"Bearer {admin_token}"},
        timeout=10,
    )
    resp.raise_for_status()
    for d in resp.json()["data"] or []:
        if d.get("displayName") == name:
            return d
    resp = httpx.post(
        f"{BASE_URL}/v1/disk/admin/public-directories",
        headers={"Authorization": f"Bearer {admin_token}"},
        json={"displayName": name, "scope": DEFAULT_SCOPE},
        timeout=10,
    )
    if resp.status_code not in (200, 201):
        raise RuntimeError(f"create public dir failed: {_http_msg(resp, 'error')}")
    return resp.json()["data"]


def ensure_api_key(admin_token: str, label: str) -> str:
    resp = httpx.get(
        f"{BASE_URL}/v1/disk/admin/api-keys",
        headers={"Authorization": f"Bearer {admin_token}"},
        timeout=10,
    )
    resp.raise_for_status()
    for k in resp.json()["data"] or []:
        if k.get("keyName") == label:
            httpx.delete(
                f"{BASE_URL}/v1/disk/admin/api-keys/{k['id']}",
                headers={"Authorization": f"Bearer {admin_token}"},
                timeout=10,
            ).raise_for_status()
            break
    resp = httpx.post(
        f"{BASE_URL}/v1/disk/admin/api-keys",
        headers={"Authorization": f"Bearer {admin_token}"},
        json={"name": label},
        timeout=10,
    )
    if resp.status_code not in (200, 201):
        raise RuntimeError(f"create api key failed: {_http_msg(resp, 'error')}")
    return resp.json()["data"]["key"]


# --- Graph generation -------------------------------------------------------

def pick_targets_uniform(rng: random.Random, n: int, k: int, self_id: int) -> list[int]:
    """Sample k distinct targets from 1..n, excluding self_id."""
    targets: set[int] = set()
    if k >= n - 1:
        # Asking for more unique targets than the population allows — return
        # the whole population minus self.
        return [i for i in range(1, n + 1) if i != self_id]
    while len(targets) < k:
        t = rng.randint(1, n)
        if t == self_id:
            continue
        targets.add(t)
    return sorted(targets)


def pick_targets_powerlaw(rng: random.Random, n: int, k: int, self_id: int) -> list[int]:
    """Sample k targets from a Zipf-skewed distribution.

    We approximate Zipf by repeatedly rolling a randint and applying a 1/rank
    accept probability — the math is approximate but the shape (a few hubs
    dominate) is what matters for the perf benchmark.
    """
    targets: set[int] = set()
    attempts = 0
    max_attempts = k * 20
    while len(targets) < k and attempts < max_attempts:
        attempts += 1
        rank = rng.randint(1, n)
        if rank == self_id:
            continue
        accept_p = 1.0 / math.sqrt(rank)
        if rng.random() < accept_p:
            targets.add(rank)
    while len(targets) < k:
        t = rng.randint(1, n)
        if t != self_id:
            targets.add(t)
    return sorted(targets)


def pick_targets_chain(rng: random.Random, n: int, k: int, self_id: int) -> list[int]:
    """Linear chain i → i+1 plus (k-1) random shortcuts."""
    targets: list[int] = []
    if self_id < n:
        targets.append(self_id + 1)
    while len(targets) < k:
        t = rng.randint(1, n)
        if t == self_id or t in targets:
            continue
        targets.append(t)
    return sorted(set(targets))


def gen_file_contents(file_id: int, targets: list[int]) -> tuple[str, str]:
    """Return (rel_path, markdown body) for one node."""
    rel_path = f"nodes/n{file_id:06d}.md"
    target_links = "\n".join(f"- [n{t}](n{t:06d}.md)" for t in targets)
    body = f"""---
type: concept
title: n{file_id}
description: bench node {file_id}
tags: [bench, n{file_id}]
---

# n{file_id}

Bench-generated node. Outgoing links:

{target_links}
"""
    return rel_path, body


def gen_index_md(node_count: int, dist: str, edges_per_node: int) -> str:
    return f"""---
okf_version: "0.1"
type: index
title: Bench Bundle ({node_count} nodes, {dist}, E={edges_per_node})
description: Auto-generated benchmark bundle
---

# Bench Bundle

This bundle was created by `scripts/seed_okf_large.py` for BFS perf validation.

- Nodes: {node_count}
- Distribution: {dist}
- Edges per node: {edges_per_node}
"""


# --- Main -------------------------------------------------------------------

def main() -> int:
    parser = argparse.ArgumentParser(description="Seed a parameterized OKF bundle for perf bench")
    parser.add_argument("--nodes", type=int, default=1000, help="node count (default 1000)")
    parser.add_argument("--edges-per-node", type=int, default=5, help="out-edge count per node (default 5)")
    parser.add_argument(
        "--dist",
        choices=["uniform", "powerlaw", "chain"],
        default="uniform",
        help="edge distribution (default uniform)",
    )
    parser.add_argument("--bundle-name", default=None, help="bundle + public dir name (default auto)")
    parser.add_argument("--seed", type=int, default=SEED, help=f"RNG seed (default {SEED})")
    args = parser.parse_args()

    if args.nodes < 2:
        parser.error("--nodes must be >= 2")

    name = args.bundle_name or f"Bench {args.nodes} {args.dist} (E={args.edges_per_node})"

    print(f"OKF large seed → {BASE_URL}")
    print(f"  bundle name   : {name}")
    print(f"  nodes         : {args.nodes}")
    print(f"  edges per node: {args.edges_per_node}")
    print(f"  dist          : {args.dist}")
    print(f"  rng seed      : {args.seed}")

    print("  logging in as admin...")
    admin_token = admin_login()

    print("  ensuring public directory...")
    pd = ensure_public_directory(admin_token, name)
    print(f"    public dir #{pd['id']}")

    print("  ensuring API key...")
    api_key = ensure_api_key(admin_token, "okf-bench-seed")

    rng = random.Random(args.seed)
    pick_fn = {
        "uniform": pick_targets_uniform,
        "powerlaw": pick_targets_powerlaw,
        "chain": pick_targets_chain,
    }[args.dist]

    started = time.monotonic()
    with AgentDiskClient(base_url=BASE_URL, api_key=api_key) as client:
        # Write index first so the bundle is registrable.
        print(f"  writing index.md + {args.nodes} node files...")
        client.write_markdown(
            public_dir_id=pd["id"],
            rel_path="index.md",
            content=gen_index_md(args.nodes, args.dist, args.edges_per_node),
        )
        report_every = max(1, args.nodes // 10)
        for i in range(1, args.nodes + 1):
            targets = pick_fn(rng, args.nodes, args.edges_per_node, i)
            rel_path, body = gen_file_contents(i, targets)
            client.write_markdown(public_dir_id=pd["id"], rel_path=rel_path, content=body)
            if i % report_every == 0 or i == args.nodes:
                elapsed = time.monotonic() - started
                rate = i / elapsed if elapsed > 0 else 0
                print(f"    {i}/{args.nodes} ({rate:.1f} files/s)")

        print("  ensuring bundle registered / refreshed...")
        bundles = client.list_bundles()
        bundle = None
        for b in bundles:
            if b.public_directory_id == pd["id"]:
                bundle = client.refresh_bundle(b.bundle_id)
                break
        if bundle is None:
            bundle = client.register_bundle(public_dir_id=pd["id"])

        elapsed = time.monotonic() - started
        print("\n──────── OKF large seed complete ────────")
        print(f"  public dir id : {pd['id']}")
        print(f"  bundle id     : {bundle.bundle_id}")
        print(f"  node count    : {bundle.node_count}")
        print(f"  edge count    : {bundle.edge_count}")
        print(f"  elapsed       : {elapsed:.1f}s")
        print()
        print("Open in browser:")
        print(f"  http://localhost:9101/okf/{bundle.bundle_id}")
        return 0


if __name__ == "__main__":
    sys.exit(main())
