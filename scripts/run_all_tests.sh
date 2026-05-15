#!/usr/bin/env bash
set -euo pipefail

echo "=== 启动 Redis ==="
redis-server --daemonize yes --port 6379 2>/dev/null || true
trap "redis-cli -p 6379 shutdown 2>/dev/null || true" EXIT

echo "=== 单元测试 ==="
go test ./... -v -count=1

echo "=== 启动 AgentDisk ==="
# 使用测试配置启动（需要设置必要的环境变量）
export JWT_SECRET="${JWT_SECRET:-test_secret_key}"
export DL_TOKEN_SECRET="${DL_TOKEN_SECRET:-dl_secret_key}"
export DB_PASSWORD="${DB_PASSWORD:-}"
export OSS_ACCESS_KEY="${OSS_ACCESS_KEY:-minioadmin}"
export OSS_SECRET_KEY="${OSS_SECRET_KEY:-minioadmin}"

# 如果有测试配置则使用，否则使用默认
CONFIG_FILE="config.yaml"
if [ -f "config.test.yaml" ]; then
  CONFIG_FILE="config.test.yaml"
fi

AGENTDISK_TEST=1 go run main.go &
SERVER_PID=$!
trap "kill $SERVER_PID 2>/dev/null; redis-cli -p 6379 shutdown 2>/dev/null || true" EXIT
sleep 3

echo "=== 认证集成测试 ==="
bash scripts/test_auth.sh

echo "=== 停止服务 ==="
kill $SERVER_PID 2>/dev/null || true
