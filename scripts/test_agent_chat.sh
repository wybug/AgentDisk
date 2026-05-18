#!/usr/bin/env bash
set -euo pipefail

GATEWAY_PORT=3100
AGENT_PORT=8090
COOKIE_JAR="/tmp/agent-chat-test-cookies.txt"

echo "=========================================="
echo "  Agent Chat 端到端测试"
echo "=========================================="

# 1. 检查网关
echo ""
echo "[1/4] 检查网关 ($GATEWAY_PORT) ..."
if ! curl -sf -o /dev/null "http://localhost:$GATEWAY_PORT/chat"; then
  echo "[FAIL] 网关未运行"
  exit 1
fi
echo "  网关运行中 ✓"

# 2. 检查 Agent 服务
echo ""
echo "[2/4] 检查 Agent 服务 ($AGENT_PORT) ..."
if ! curl -s -o /dev/null "http://localhost:$AGENT_PORT/process" -X POST \
  -H "Content-Type: application/json" \
  -d '{"input":[{"role":"user","content":[{"type":"text","text":"ping"}]}]}' \
  --max-time 5 2>/dev/null; then
  echo "[FAIL] Agent 服务 ($AGENT_PORT) 未运行或无响应"
  exit 1
fi
echo "  Agent 服务运行中 ✓"

# 3. 登录网关获取 session
echo ""
echo "[3/4] 登录网关 ..."
rm -f "$COOKIE_JAR"
LOGIN_RESP=$(curl -sf -X POST "http://localhost:$GATEWAY_PORT/api/login" \
  -H "Content-Type: application/json" \
  -d '{"userId":"5001185","password":"123456"}' \
  -c "$COOKIE_JAR" 2>&1)
if echo "$LOGIN_RESP" | grep -q '"success":true'; then
  echo "  登录成功 (user001) ✓"
else
  echo "[FAIL] 登录失败: $LOGIN_RESP"
  exit 1
fi

# 4. 通过网关代理发送测试消息
echo ""
echo "[4/4] 通过网关代理发起 SSE 会话 ..."
RESPONSE=$(curl -s -N -X POST "http://localhost:$GATEWAY_PORT/process" \
  -H "Content-Type: application/json" \
  -b "$COOKIE_JAR" \
  -d '{"input":[{"role":"user","content":[{"type":"text","text":"1+1=?，只回答数字"}]}]}' \
  --max-time 30 2>&1 || true)

DATA_COUNT=$(echo "$RESPONSE" | grep -c "^data:" || echo "0")

if [ "$DATA_COUNT" -gt 0 ]; then
  echo "  SSE 流接收成功，收到 $DATA_COUNT 条 data 事件 ✓"
  echo ""
  echo "  流式内容:"
  echo "$RESPONSE" | grep '^data:.*"delta":true' | sed 's/.*"text":"\(.*\)"/    \1/' || true
  echo ""
  echo "  完整响应 (首尾):"
  echo "$RESPONSE" | grep "^data:" | head -1 | sed 's/^/    /'
  echo "    ..."
  echo "$RESPONSE" | grep "^data:" | tail -1 | sed 's/^/    /'
else
  echo "[FAIL] 未收到 SSE 数据"
  echo "  响应: $(echo "$RESPONSE" | head -5)"
  rm -f "$COOKIE_JAR"
  exit 1
fi

rm -f "$COOKIE_JAR"

echo ""
echo "=========================================="
echo "  全部测试通过 ✓"
echo "=========================================="
echo ""
echo "打开 http://localhost:$GATEWAY_PORT/chat 发送消息验证 UI"
