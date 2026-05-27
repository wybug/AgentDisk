---
name: sdk-publish
description: 发布 AgentDisk Python SDK 到 PyPI。当用户要求"发布 SDK"、"publish to PyPI"、"发布到 PyPI"、"更新 PyPI 包"、"sdk publish"时触发。处理版本升级、元数据补全、构建检查、打包上传的完整流程。
---

# SDK Publish

将 `sdk/` 目录下的 AgentDisk Python SDK 发布到 PyPI。

## 前置条件

### 1. 环境变量认证

检查 `TWINE_USERNAME` 和 `TWINE_PASSWORD` 是否已设置：

```bash
echo "${TWINE_USERNAME:+SET}" && echo "${TWINE_PASSWORD:+SET}"
```

若任一未设置，**停止并通知用户**：

> 发布需要环境变量 `TWINE_USERNAME` 和 `TWINE_PASSWORD`。请先配置：
> ```bash
> export TWINE_USERNAME="__token__"
> export TWINE_PASSWORD="pypi-..."
> ```
> 也可添加到 shell 配置文件（`~/.zshrc`）或 `.env` 中持久化。

### 2. 安装构建工具

确保已安装 `build` 和 `twine`：

```bash
pip install build twine
```

## 版本号升级规则（SemVer）

分析本次 SDK 变更内容，按以下规则确定版本号：

| 变更类型 | 示例 | 1.0.0 前 | 1.0.0 后 |
|----------|------|----------|----------|
| **兼容** | 新增方法、新增可选参数、bugfix | patch (0.1.0→0.1.1) | patch (1.0.0→1.0.1) |
| **不兼容** | 删除方法、修改签名、破坏性改动 | minor (0.1.0→0.2.0) | major (1.0.0→2.0.0) |

**1.0.0 之前特殊规则**：所有变更默认视为不兼容（升 minor），除非明确判定为兼容变更才升 patch。

分析步骤：
1. 读取当前版本：`grep 'version' sdk/pyproject.toml`
2. 分析变更：`git diff <上次tag或commit> -- sdk/agentdisk/ --stat` 以及详细 diff
3. 根据变更内容推荐版本号，说明理由
4. 展示推荐版本号给用户确认后再执行

## 发布流程

### Step 1: 补全 pyproject.toml 元数据

确认 `sdk/pyproject.toml` 包含以下字段，缺失则补全：

```toml
authors = [{name = "wangyun"}]
readme = "README.md"

classifiers = [
    "Development Status :: 3 - Alpha",
    "Intended Audience :: Developers",
    "License :: OSI Approved :: Apache Software License",
    "Programming Language :: Python :: 3",
    "Programming Language :: Python :: 3.9",
    "Programming Language :: Python :: 3.10",
    "Programming Language :: Python :: 3.11",
    "Programming Language :: Python :: 3.12",
]

[project.urls]
Homepage = "https://github.com/wybug/AgentDisk"
Repository = "https://github.com/wybug/AgentDisk"
```

### Step 2: 更新版本号

同步修改两个文件中的版本号：

- `sdk/pyproject.toml` → `version = "<new_version>"`
- `sdk/agentdisk/__init__.py` → `__version__ = "<new_version>"`

### Step 3: 更新 README.md

检查 `sdk/README.md` 是否覆盖了新增的公开方法。参考 `sdk/agentdisk/client.py` 和 `sdk/agentdisk/async_client.py` 中所有公开方法，确保 README 的 API Overview 表格完整。

### Step 4: 运行 SDK 检查

```bash
make sdk-check
```

**必须全部通过**，任何失败则停止并修复。

### Step 5: 清理旧构建 & 重新打包

```bash
cd sdk && rm -rf dist/ build/ *.egg-info agentdisk.egg-info
cd sdk && python -m build
```

### Step 6: 验证包格式

```bash
cd sdk && twine check dist/*
```

**必须通过**，有警告也需处理。

### Step 7: 上传到 PyPI

```bash
cd sdk && twine upload dist/*
```

使用环境变量中的 `TWINE_USERNAME` / `TWINE_PASSWORD` 自动认证。

### Step 8: 验证安装

```bash
pip install --upgrade agentdisk==<new_version>
python -c "import agentdisk; print(agentdisk.__version__)"
```

## 规则

- 未配置环境变量时禁止继续，必须通知用户配置后再执行
- `make sdk-check` 未通过禁止上传
- `twine check` 有 error 禁止上传
- 每次发布前必须分析变更内容并推荐版本号，不可跳过确认
- 版本号必须同时在 `pyproject.toml` 和 `__init__.py` 中同步更新
