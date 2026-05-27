---
name: sync-docs
description: 同步更新 VitePress 帮助文档，检测代码变更并更新对应文档页面
---

同步更新 AgentDisk 帮助文档。请按以下步骤执行：

## 1. 检测变更范围

分析当前分支相对于 `main` 分支的代码变更，确定文档需要更新的区域：

- `internal/router/router.go` 或 `internal/handler/*.go` 变更 → 需更新 `docs/site/api/` 对应页面
- `internal/model/*.go` 变更 → 需更新 `docs/site/api/` 和 `docs/site/architecture/` 对应页面
- `internal/middleware/*.go` 变更 → 需更新 `docs/site/integration/auth.md` 和 `docs/site/architecture/security.md`
- `web/src/pages/*.tsx` 或 `web/src/components/**/*.tsx` 变更 → 需更新 `docs/site/guide/` 对应页面
- `config.yaml` 或 `config` 相关代码变更 → 需更新 `docs/site/guide/configuration.md`
- `sdk/` 变更 → 需更新 `docs/site/integration/sdk-python.md` 和 `docs/site/integration/agent.md`
- `scripts/dev.sh` 或部署相关变更 → 需更新 `docs/site/guide/installation.md`

运行以下命令获取变更概览：

```bash
git diff main...HEAD --name-only -- internal/ sdk/ web/src/ config.yaml scripts/
```

## 2. 逐项比对并更新

对每个检测到的变更区域：

1. **读取源码**：读取变更的 Go handler / Model / React 页面 / SDK 代码
2. **读取对应文档**：读取 `docs/site/` 下的现有文档页面
3. **比对差异**：检查以下内容是否一致
   - API 路径、HTTP 方法、请求参数、响应结构
   - 数据模型字段、类型、默认值
   - 认证方式和权限规则
   - 配置项名称和说明
   - SDK 方法定义和参数
4. **更新文档**：将源码中的最新信息同步到文档，保持中文写作风格和 VitePress Markdown 格式

## 3. 处理特殊指令

$ARGUMENTS

如果 `$ARGUMENTS` 指定了特定模块或文件，仅更新对应的文档。例如：
- `/sync-docs api` — 仅更新 API 参考文档
- `/sync-docs guide` — 仅更新使用指南
- `/sync-docs sdk` — 仅更新 SDK 和集成文档
- `/sync-docs permissions` — 仅更新权限相关文档

## 4. 验证构建

更新完成后，运行构建验证：

```bash
cd docs/site && npm run docs:build
```

如果构建失败，修复错误直到构建通过。

## 5. 输出变更摘要

列出所有修改的文档文件和主要更新内容。
