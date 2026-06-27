"""AgentDisk knowledge-base maintenance agent (Google ADK 2.0).

Exposes ``root_agent`` — the entry point ADK discovers via
``from adk_writer_agent.agent import root_agent``. The agent has 6 tools
covering the OKF v0.1 maintenance lifecycle: register / create_folder /
write_markdown / list_nodes / aggregate_types / refresh_index.
"""
from __future__ import annotations

import os

from google.adk.agents import LlmAgent

from .tools import (
    aggregate_types_tool,
    create_folder_tool,
    list_nodes_tool,
    refresh_index_tool,
    register_bundle_tool,
    write_markdown_tool,
)

INSTRUCTION = """\
你是 AgentDisk 知识库维护 Agent。

职责：把用户提供的领域知识结构化为 OKF v0.1 格式，并写入指定 bundle。

工作流程：
1. 如果 bundle 尚未注册，调用 register_bundle(public_directory_id) 获取 bundle_id
2. 顶层目录若不存在，先 create_folder("concepts") / create_folder("playbooks") 等
3. 每个 .md 文件用 write_markdown(rel_path, content) 写入
4. 全部写完后可选 list_nodes(bundle_id) 或 aggregate_types() 核对
5. 若怀疑索引漂移，调用 refresh_index(bundle_id) 重建

frontmatter 规则：
- 每个 .md 必须有 YAML frontmatter，至少包含 type 字段
- type 是自由字符串，推荐小写英文：concept / playbook / reference / llm / person / place / event
- 推荐字段：title / description / tags(list) / timestamp(ISO 8601)
- index.md 必须包含 okf_version: "0.1"

链接与命名：
- 内部链接用 bundle-relative：/concepts/gemma.md 或 ./gemma.md
- 保留文件名：index.md（仅 bundle 根）、log.md（更新历史）

容错：
- write_markdown 返回 HTTP 400 时，根据 error message 修正 frontmatter 后重试一次
- 重复 create_folder 不算错误（视为幂等）
- 网络错误重试一次；连续两次失败则停止并报告

不要：
- 不要把密钥、OSS 路径写进 markdown
- 不要捏造 nodeId；只有服务器返回的 nodeId 才是真的
"""

MODEL = os.environ.get("WRITER_AGENT_MODEL", "gemini-2.5-flash")

root_agent = LlmAgent(
    name="agentdisk_writer",
    model=MODEL,
    description="AgentDisk OKF v0.1 知识库维护 Agent",
    instruction=INSTRUCTION,
    tools=[
        register_bundle_tool,
        create_folder_tool,
        write_markdown_tool,
        list_nodes_tool,
        aggregate_types_tool,
        refresh_index_tool,
    ],
)

__all__ = ["root_agent"]
