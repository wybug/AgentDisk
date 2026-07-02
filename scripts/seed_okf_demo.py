#!/usr/bin/env python3
"""Seed an OKF demo bundle for browser validation.

Idempotent: safe to run repeatedly. Looks for an existing "OKF Demo" public
directory + bundle and reuses them; overwrites the markdown files in place.

Usage:
    python3 scripts/seed_okf_demo.py
    AGENTDISK_URL=http://localhost:9100 python3 scripts/seed_okf_demo.py

Prerequisites:
    - Backend running on :9100 (bash scripts/dev.sh start)
    - Admin credentials available (defaults: admin / admin123)
    - agentdisk SDK importable (cd sdk && pip install -e .)
"""

from __future__ import annotations

import os
import sys
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
DEMO_DIR_NAME = "OKF Demo Bundle"
DEMO_DIR_SCOPE = "global"


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


def list_public_directories(admin_token: str) -> list[dict[str, Any]]:
    resp = httpx.get(
        f"{BASE_URL}/v1/disk/admin/public-directories",
        headers={"Authorization": f"Bearer {admin_token}"},
        timeout=10,
    )
    resp.raise_for_status()
    return resp.json()["data"] or []


def create_public_directory(admin_token: str, name: str) -> dict[str, Any]:
    resp = httpx.post(
        f"{BASE_URL}/v1/disk/admin/public-directories",
        headers={"Authorization": f"Bearer {admin_token}"},
        json={"displayName": name, "scope": DEMO_DIR_SCOPE},
        timeout=10,
    )
    if resp.status_code not in (200, 201):
        raise RuntimeError(f"create public dir failed: {_http_msg(resp, 'error')}")
    return resp.json()["data"]


def ensure_public_directory(admin_token: str) -> dict[str, Any]:
    dirs = list_public_directories(admin_token)
    for d in dirs:
        if d.get("displayName") == DEMO_DIR_NAME:
            print(f"  reuse public directory #{d['id']} ({DEMO_DIR_NAME})")
            return d
    d = create_public_directory(admin_token, DEMO_DIR_NAME)
    print(f"  created public directory #{d['id']} ({DEMO_DIR_NAME})")
    return d


def ensure_api_key(admin_token: str) -> str:
    # Look for an existing key with our name to avoid piling up keys on reruns.
    resp = httpx.get(
        f"{BASE_URL}/v1/disk/admin/api-keys",
        headers={"Authorization": f"Bearer {admin_token}"},
        timeout=10,
    )
    resp.raise_for_status()
    for k in resp.json()["data"] or []:
        if k.get("keyName") == "okf-demo-seed":
            # We can't recover the raw key after creation; revoke + recreate.
            httpx.delete(
                f"{BASE_URL}/v1/disk/admin/api-keys/{k['id']}",
                headers={"Authorization": f"Bearer {admin_token}"},
                timeout=10,
            ).raise_for_status()
            break
    resp = httpx.post(
        f"{BASE_URL}/v1/disk/admin/api-keys",
        headers={"Authorization": f"Bearer {admin_token}"},
        json={"name": "okf-demo-seed"},
        timeout=10,
    )
    if resp.status_code not in (200, 201):
        raise RuntimeError(f"create api key failed: {_http_msg(resp, 'error')}")
    return resp.json()["data"]["key"]


# --- Markdown content for the demo bundle ----------------------------------

INDEX_MD = """---
okf_version: "0.1"
type: index
title: OKF Demo Bundle
description: 演示用知识图谱，覆盖概念 / 人物 / 组织 / 模型四类节点
tags: [demo, knowledge-graph]
---

# OKF Demo Bundle

这是一个用于前端 UI 验证的演示知识库，包含：

- 跨节点链接（Transformer ↔ Attention ↔ Gemma）
- 多类型节点（concept / person / org / model）
- 多标签节点
- 故意构造的死链（指向不存在的 markdown 文件）

请配合 `/okf/:bundleId` 详情页的「图谱」tab 验证。
"""

TRANSFORMER_MD = """---
type: concept
title: Transformer 架构
description: 基于自注意力的序列建模架构
tags: [architecture, attention, neural-network]
---

# Transformer 架构

Transformer 由 Vaswani 等人在 2017 年提出，是现代大模型的基石。

核心组件：

- [自注意力机制](attention.md)
- 多头注意力
- 位置编码
- 前馈网络

参见：[Google DeepMind](../orgs/deepmind.md)、[Gemma 模型](../models/gemma.md)。

故意死链：[未来方向](future-directions.md)。
"""

ATTENTION_MD = """---
type: concept
title: 自注意力机制
description: Self-attention，让序列中每个位置互相打分
tags: [attention, core-concept]
---

# 自注意力机制

Self-attention 计算序列中每对位置的关联强度。

详见 [Transformer 架构](transformer.md)。也是 [Gemma](../models/gemma.md) 的底层机制。
"""

GEMMA_MD = """---
type: model
title: Gemma
description: Google 的开源轻量大语言模型家族
tags: [llm, google, open-models]
---

# Gemma

Gemma 是 Google DeepMind 推出的开源大语言模型家族，基于 [Transformer 架构](../concepts/transformer.md)。

由 [Google DeepMind](../orgs/deepmind.md) 发布，并提供了 Gemma 2B / 7B 等多个参数规模。

技术博客参见：[Gemma 技术报告](gemma-tech-report.md)。
"""

DEEPMIND_MD = """---
type: org
title: Google DeepMind
description: Google 旗下的人工智能研究实验室
tags: [lab, google]
---

# Google DeepMind

Google DeepMind 是 Google 旗下的人工智能研究实验室，发布了 [Gemma](../models/gemma.md) 等开源模型。

其研究覆盖：强化学习、多模态、基础模型等方向。
"""

HINTON_MD = """---
type: person
title: Geoffrey Hinton
description: 深度学习先驱，2024 年诺贝尔物理学奖得主
tags: [researcher, dl-pioneer]
---

# Geoffrey Hinton

深度学习之父，提出了反向传播、玻尔兹曼机等基础理论，影响了整个 [Transformer 架构](../concepts/transformer.md) 的发展。
"""

# rel_path → markdown content. Files are written via the API-key writer
# endpoint; the OKF materializer rebuilds nodes + edges from frontmatter
# and [[wikilinks]] / [text](relative.md) links.
FILES: dict[str, str] = {
    "index.md": INDEX_MD,
    "concepts/transformer.md": TRANSFORMER_MD,
    "concepts/attention.md": ATTENTION_MD,
    "models/gemma.md": GEMMA_MD,
    "orgs/deepmind.md": DEEPMIND_MD,
    "persons/hinton.md": HINTON_MD,
}


def write_all_files(client: AgentDiskClient, public_dir_id: int) -> None:
    for rel_path, content in FILES.items():
        node = client.write_markdown(public_dir_id=public_dir_id, rel_path=rel_path, content=content)
        broken_flag = " [broken]" if node.has_broken_link else ""
        print(f"    wrote {rel_path:<32} → node #{node.node_id} ({node.type}){broken_flag}")


def ensure_bundle(client: AgentDiskClient, public_dir_id: int) -> Any:
    bundles = client.list_bundles()
    for b in bundles:
        if b.public_directory_id == public_dir_id:
            print(f"  reuse bundle #{b.bundle_id} (status={b.status}, nodes={b.node_count})")
            # Refresh to re-pick up any file changes.
            b = client.refresh_bundle(b.bundle_id)
            print(f"  refreshed → nodes={b.node_count}, edges={b.edge_count}")
            return b
    b = client.register_bundle(public_dir_id=public_dir_id)
    print(f"  registered bundle #{b.bundle_id}")
    return b


def main() -> int:
    print(f"OKF demo seed → {BASE_URL}")
    print("  logging in as admin...")
    admin_token = admin_login()

    print("  ensuring public directory...")
    pd = ensure_public_directory(admin_token)

    print("  ensuring API key...")
    api_key = ensure_api_key(admin_token)

    with AgentDiskClient(base_url=BASE_URL, api_key=api_key) as client:
        print("  writing markdown files (API key auth)...")
        write_all_files(client, pd["id"])

        print("  ensuring bundle registered...")
        bundle = ensure_bundle(client, pd["id"])

        print("  running dead-link scan...")
        report = client.scan_bundle(bundle.bundle_id)
        print(f"    scanned {report.scanned_nodes} nodes, {report.broken_count} broken links")

        print("\n──────── OKF demo seed complete ────────")
        print(f"  public dir id : {pd['id']}")
        print(f"  bundle id     : {bundle.bundle_id}")
        print(f"  bundle title  : {bundle.title}")
        print(f"  node count    : {bundle.node_count}")
        print(f"  edge count    : {bundle.edge_count}")
        print(f"  broken links  : {report.broken_count}")
        print()
        print("Open in browser:")
        print(f"  http://localhost:9101/okf                (bundle list)")
        print(f"  http://localhost:9101/okf/{bundle.bundle_id}   (detail page)")
        return 0


if __name__ == "__main__":
    sys.exit(main())
