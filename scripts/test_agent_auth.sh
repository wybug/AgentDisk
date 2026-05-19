#!/usr/bin/env bash
# Agent 授权验证示例 — agentId + agentGroupId
# 前置: 网关运行在 localhost:3100, Agent 服务运行在 localhost:8090
# 用法: bash scripts/test_agent_auth.sh

set -euo pipefail

GATEWAY="http://localhost:3100"
COOKIE_A="/tmp/agent-auth-cookies-a.txt"
COOKIE_B="/tmp/agent-auth-cookies-b.txt"
PASS=0
FAIL=0

log_pass() { PASS=$((PASS+1)); echo "  PASS: $1"; }
log_fail() { FAIL=$((FAIL+1)); echo "  FAIL: $1"; }

cleanup() { rm -f "$COOKIE_A" "$COOKIE_B"; }
trap cleanup EXIT

echo "=========================================="
echo "  Agent 授权验证 (agentId + agentGroupId)"
echo "=========================================="

# ── 1. 登录两个用户 ──
echo ""
echo "=== 1. 用户登录 ==="

rm -f "$COOKIE_A"
LOGIN=$(curl -sf -X POST "$GATEWAY/api/login" \
  -H "Content-Type: application/json" \
  -d '{"userId":"user001","password":"test123"}' \
  -c "$COOKIE_A")
echo "$LOGIN" | grep -q '"success":true' && log_pass "user001 登录" || log_fail "user001 登录"

rm -f "$COOKIE_B"
LOGIN=$(curl -sf -X POST "$GATEWAY/api/login" \
  -H "Content-Type: application/json" \
  -d '{"userId":"user002","password":"test123"}' \
  -c "$COOKIE_B")
echo "$LOGIN" | grep -q '"success":true' && log_pass "user002 登录" || log_fail "user002 登录"

# ── 2. 注册 Agent（同组 + 独立） ──
echo ""
echo "=== 2. 注册 Agent ==="

REG=$(curl -sf -X POST "$GATEWAY/api/agents" \
  -b "$COOKIE_A" -H "Content-Type: application/json" \
  -d '{"agentId":"writer-01","agentName":"写作助手","agentGroupId":"team-content"}')
echo "$REG" | grep -q '"success":true' && log_pass "注册 writer-01 (team-content)" || log_fail "注册 writer-01"

REG=$(curl -sf -X POST "$GATEWAY/api/agents" \
  -b "$COOKIE_A" -H "Content-Type: application/json" \
  -d '{"agentId":"reviewer-01","agentName":"审核助手","agentGroupId":"team-content"}')
echo "$REG" | grep -q '"success":true' && log_pass "注册 reviewer-01 (team-content)" || log_fail "注册 reviewer-01"

REG=$(curl -sf -X POST "$GATEWAY/api/agents" \
  -b "$COOKIE_A" -H "Content-Type: application/json" \
  -d '{"agentId":"coder-01","agentName":"代码助手","agentGroupId":""}')
echo "$REG" | grep -q '"success":true' && log_pass "注册 coder-01 (无组)" || log_fail "注册 coder-01"

# ── 3. 验证 Agent 归属 ──
echo ""
echo "=== 3. 验证 Agent 归属 ==="

AGENTS=$(curl -sf "$GATEWAY/api/agents" -b "$COOKIE_A")
COUNT=$(echo "$AGENTS" | python3 -c "import sys,json; print(len(json.load(sys.stdin)))" 2>/dev/null || echo "0")
[ "$COUNT" = "3" ] && log_pass "user001 查到 3 个 Agent" || log_fail "user001 应有 3 个 Agent, 实际 $COUNT"

AGENTS_B=$(curl -sf "$GATEWAY/api/agents" -b "$COOKIE_B")
COUNT_B=$(echo "$AGENTS_B" | python3 -c "import sys,json; print(len(json.load(sys.stdin)))" 2>/dev/null || echo "0")
[ "$COUNT_B" = "0" ] && log_pass "user002 查到 0 个 Agent (隔离正确)" || log_fail "user002 不应有 Agent, 实际 $COUNT_B"

# ── 4. 通过 /process 验证 JWT 授权 ──
echo ""
echo "=== 4. /process JWT 授权 ==="

# 4a. 合法 agentId → 成功 (SSE)
echo "  --- 4a. 合法 agentId 请求 (writer-01) ---"
RESP=$(curl -s -X POST "$GATEWAY/process" \
  -b "$COOKIE_A" -H "Content-Type: application/json" \
  -d '{"agentId":"writer-01","input":[{"role":"user","content":[{"type":"text","text":"只回答 ok"}]}]}' \
  --max-time 30 2>&1)
DATA_COUNT=$(echo "$RESP" | grep -c "^data:" || echo "0")
[ "$DATA_COUNT" -gt 0 ] && log_pass "writer-01 代理成功, 收到 $DATA_COUNT 条 SSE" || log_fail "writer-01 代理失败"

# 4b. 未注册 agentId → 403
STATUS=$(curl -s -o /dev/null -w "%{http_code}" -X POST "$GATEWAY/process" \
  -b "$COOKIE_A" -H "Content-Type: application/json" \
  -d '{"agentId":"unknown-agent","input":[{"role":"user","content":[{"type":"text","text":"test"}]}]}' \
  --max-time 5)
[ "$STATUS" = "403" ] && log_pass "未注册 Agent 拒绝 (403)" || log_fail "未注册 Agent 应 403, 实际 $STATUS"

# 4c. 跨用户访问 Agent → 403
STATUS=$(curl -s -o /dev/null -w "%{http_code}" -X POST "$GATEWAY/process" \
  -b "$COOKIE_B" -H "Content-Type: application/json" \
  -d '{"agentId":"writer-01","input":[{"role":"user","content":[{"type":"text","text":"test"}]}]}' \
  --max-time 5)
[ "$STATUS" = "403" ] && log_pass "跨用户 Agent 拒绝 (403)" || log_fail "跨用户 Agent 应 403, 实际 $STATUS"

# 4d. 无 agentId → 用户身份请求
RESP=$(curl -s -X POST "$GATEWAY/process" \
  -b "$COOKIE_A" -H "Content-Type: application/json" \
  -d '{"input":[{"role":"user","content":[{"type":"text","text":"只回答 ok"}]}]}' \
  --max-time 30 2>&1)
DATA_COUNT=$(echo "$RESP" | grep -c "^data:" || echo "0")
[ "$DATA_COUNT" -gt 0 ] && log_pass "无 agentId 用户请求成功" || log_fail "无 agentId 用户请求失败"

# 4e. 未登录 → 401
STATUS=$(curl -s -o /dev/null -w "%{http_code}" -X POST "$GATEWAY/process" \
  -H "Content-Type: application/json" \
  -d '{"input":[{"role":"user","content":[{"type":"text","text":"test"}]}]}' \
  --max-time 5)
[ "$STATUS" = "401" ] && log_pass "未登录拒绝 (401)" || log_fail "未登录应 401, 实际 $STATUS"

# ── 5. 删除 Agent 验证 ──
echo ""
echo "=== 5. Agent 删除与权限回收 ==="

# 5a. 跨用户删除 → 403
STATUS=$(curl -s -o /dev/null -w "%{http_code}" -X DELETE "$GATEWAY/api/agents/writer-01" \
  -b "$COOKIE_B")
[ "$STATUS" = "403" ] && log_pass "跨用户删除 Agent 拒绝 (403)" || log_fail "跨用户删除应 403, 实际 $STATUS"

# 5b. 所有者删除 → 成功
RESP=$(curl -sf -X DELETE "$GATEWAY/api/agents/writer-01" -b "$COOKIE_A")
echo "$RESP" | grep -q '"success":true' && log_pass "所有者删除 writer-01" || log_fail "所有者删除失败"

# 5c. 删除后访问 → 403
STATUS=$(curl -s -o /dev/null -w "%{http_code}" -X POST "$GATEWAY/process" \
  -b "$COOKIE_A" -H "Content-Type: application/json" \
  -d '{"agentId":"writer-01","input":[{"role":"user","content":[{"type":"text","text":"test"}]}]}' \
  --max-time 5)
[ "$STATUS" = "403" ] && log_pass "删除后 Agent 拒绝 (403)" || log_fail "删除后应 403, 实际 $STATUS"

# ── 清理剩余 Agent ──
curl -sf -X DELETE "$GATEWAY/api/agents/reviewer-01" -b "$COOKIE_A" > /dev/null 2>&1 || true
curl -sf -X DELETE "$GATEWAY/api/agents/coder-01" -b "$COOKIE_A" > /dev/null 2>&1 || true

# ── 汇总 ──
echo ""
echo "=============================="
echo "  通过: $PASS  失败: $FAIL"
echo "=============================="
[ "$FAIL" -eq 0 ] && echo "全部通过!" || echo "存在失败用例"
exit $FAIL
