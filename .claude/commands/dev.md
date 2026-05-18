---
name: dev
description: 启动/停止 AgentDisk 本地开发环境（后端 + 网关 + 前端）
---

请执行以下操作：

1. 运行 `bash scripts/dev.sh $ARGUMENTS` 管理本地开发服务

支持的子命令：
- `dev` 或 `dev start` — 先停止所有服务，再启动后端(9100) + 网关(3000) + 前端(5173)
- `dev stop` — 停止所有服务
- `dev restart` — 重启所有服务
- `dev status` — 查看各服务运行状态
- `dev logs [backend|gateway|web]` — 查看日志

启动后请报告各服务的状态（端口是否就绪）。
如果某个服务启动失败，查看 `.dev-logs/` 下对应的日志文件并报告错误信息。

$ARGUMENTS
