"""AgentDisk knowledge-base maintenance agent (Google ADK 2.0).

Exposes ``root_agent`` — the entry point ADK discovers via
``from adk_writer_agent.agent import root_agent``. The agent has 6 tools
covering the OKF v0.1 maintenance lifecycle: register / create_folder /
write_markdown / list_nodes / aggregate_types / refresh_index.

``reset_data`` is the ADK eval hook (see
:func:`google.adk.cli.cli_eval.try_get_reset_func`) called before each eval
case. It cleans bundles + PD files so cases don't leak state into each
other — defined here (rather than in ``evals/``) because ADK looks it up
as ``agent_module.agent.reset_data``. The implementation lives in
``evals/eval_setup.py`` and is imported lazily so production users of the
agent don't pay the httpx import cost unless they run evals.
"""

from __future__ import annotations

import logging
import os
from typing import Any

from google.adk.agents import LlmAgent

from .tools import (
    aggregate_types_tool,
    create_folder_tool,
    list_nodes_tool,
    refresh_index_tool,
    register_bundle_tool,
    write_markdown_tool,
)

logger = logging.getLogger("adk_writer_agent.agent")

INSTRUCTION = """\
你是 AgentDisk 知识库维护 Agent。

职责：把用户提供的领域知识结构化为 OKF v0.1 格式，并写入指定 bundle。

工作流程：
1. 仅当 bundle 尚未注册（用户没有在 prompt 中给出 bundle_id）时，才调用
   register_bundle(public_directory_id) 获取 bundle_id。
2. 仅当用户明确要求写入某个子路径（如 concepts/X.md）且对应目录可能不存在时，
   才调用 create_folder 创建该子目录。不要在 register_bundle 之后或没有写
   入请求时主动 create_folder。
3. 写入 .md 文件用 write_markdown(rel_path, content)。content 严格按
   用户在 prompt 中明确给出的字段构造，**用户给出的字段就是完整规格，
   直接按规格写出最小 frontmatter + 正文，不要追问用户确认或要求更多
   细节**。字段映射规则：
   - 用户给出 type=X → frontmatter 必须有 `type: X`
   - 用户给出"标题 X"或"title X" → frontmatter 加 `title: X`。如果用户
     同时明确指定了正文内容（如"正文只有 xxx"、"写一行 xxx"），正文严格
     按用户指定；否则正文起一行一级标题 `# X`。
   - 用户给出"标签 a、b"或"tags=[a,b]" → frontmatter 加 `tags: [a, b]`，
     list 形式
   - 用户没明确要求的字段（description / timestamp 等推荐字段）→ **不要
     主动添加**。"推荐字段"只是格式参考，不是默认全部写进去。
   - 用户没给出标题 → 不要加 `# X` 正文标题。
   - 用户给出正文内容（如"正文：xxx"、"引用 [text](url)"）→ 严格按用户
     给出的内容写，不要展开、不要修饰。
   - 链接文本使用最简形式（如 `[gemma](./gemma.md)`），不要加"详见/参
     考/See"等修饰词。
4. 仅当用户明确要求"列出节点"/"查看分布"/"核对"时，才调用 list_nodes 或
   aggregate_types。不要在写完文件后主动追加核对调用。
5. 仅当用户明确说"重建索引"/"索引漂移"或刚发生了直接 OSS 写入时，才调用
   refresh_index。常规写入流程不要追加 refresh。

frontmatter 规则：
- 每个 .md 必须有 YAML frontmatter，至少包含 type 字段
- type 是自由字符串，推荐小写英文：concept / playbook / reference / llm / person / place / event
- 推荐字段：title / description / tags(list) / timestamp(ISO 8601)
- index.md 必须包含 okf_version: "0.1"

链接与命名：
- 内部链接用 bundle-relative：/concepts/gemma.md 或 ./gemma.md
- 保留文件名：index.md（仅 bundle 根）、log.md（更新历史）

容错与幂等：
- write_markdown 返回 HTTP 400 时，根据 error message 修正 frontmatter 后重试一次
- 重复 create_folder 不算错误（视为幂等）
- 网络错误重试一次；连续两次失败则停止并报告
- 同一个工具在一次对话中不要重复调用同一个参数两次。如果已经收到成功响应，
  不要再次调用同一个工具"核对"。

工具调用参数规约：
- 可选参数（如 create_folder 的 parent_id、write_markdown 的 content_type）
  在用户没明确要求时**必须省略**，不要写成 null。例如创建顶层目录时
  create_folder 应只传 folder_name 和 public_directory_id 两个参数。
- 必填参数（如 create_folder 的 folder_name、write_markdown 的 rel_path
  和 content）必须显式传值，不能省略。

不要：
- 不要把密钥、OSS 路径写进 markdown
- 不要捏造 nodeId；只有服务器返回的 nodeId 才是真的
- 不要主动调用工具做用户没要求的事（注册后建目录、写完后核对、aggregate
  重复调用等）
"""

MODEL = os.environ.get("WRITER_AGENT_MODEL", "deepseek/deepseek-chat")

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

__all__ = ["root_agent", "reset_data"]


def reset_data(*args: Any, **kwargs: Any) -> dict[str, int]:
    """ADK eval hook — wipe OKF bundles + PD files before each eval case.

    ADK's ``cli_eval.try_get_reset_func`` looks up ``agent.reset_data`` on the
    agent submodule. The function takes no args (we accept ``*args`` for
    forward-compat in case ADK starts passing a config object).

    The actual cleanup lives in ``evals/eval_setup.py`` so production
    deployments of the agent don't need ``evals/`` on the Python path — we
    import lazily here, and any failure is logged + swallowed so the eval
    run continues (a dirty state will surface as test failures with clear
    diagnostics, which is preferable to aborting the whole eval).
    """
    try:
        # When the agent package is pip-installed, ``evals/`` is excluded
        # from the install. We sys.path-mutate to add the parent dir so
        # the import works during `adk eval` (which runs from the source
        # tree).
        import sys
        from pathlib import Path

        _pkg_root = Path(__file__).resolve().parent.parent
        if str(_pkg_root) not in sys.path:
            sys.path.insert(0, str(_pkg_root))
        from evals.eval_setup import clean_okf_state  # type: ignore[import-not-found]
    except ImportError as exc:
        logger.warning("reset_data: evals.eval_setup not importable: %s", exc)
        return {"bundles_deleted": 0, "files_deleted": 0}
    return clean_okf_state()
