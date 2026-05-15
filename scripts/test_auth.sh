#!/usr/bin/env bash
# AgentDisk 认证集成测试脚本
# 前置: redis-server 已启动, AgentDisk 运行在 localhost:8080
# 用法: bash scripts/test_auth.sh

set -euo pipefail

BASE_URL="${BASE_URL:-http://localhost:8080}"
JWT_SECRET="${JWT_SECRET:-test_secret_key}"
DL_SECRET="${DL_SECRET:-dl_secret_key}"
PASS=0
FAIL=0

# ── 工具函数 ──
log_pass() { PASS=$((PASS+1)); echo "PASS: $1"; }
log_fail() { FAIL=$((FAIL+1)); echo "FAIL: $1"; }
assert_eq() { [ "$1" = "$2" ] && log_pass "$3" || log_fail "$3 (expected=$2, got=$1)"; }
assert_contains() { echo "$1" | grep -q "$2" && log_pass "$3" || log_fail "$3"; }

# ── 0. 健康检查 ──
echo "=== 健康检查 ==="
STATUS=$(curl -s -o /dev/null -w "%{http_code}" "$BASE_URL/health")
assert_eq "$STATUS" "200" "Health check"

# ── 路径1: JWT 内部服务认证 ──
echo ""
echo "=== 路径1: JWT 认证 ==="

TOKEN=$(go run scripts/gen_token/main.go -secret "$JWT_SECRET" -userId "user_test_001" -agentId "agent_test_001")

# 1.1 无 Token 访问 → 401
STATUS=$(curl -s -o /dev/null -w "%{http_code}" "$BASE_URL/v1/disk/space")
assert_eq "$STATUS" "401" "无 Token 拒绝访问"

# 1.2 有效 JWT 访问 → 200
STATUS=$(curl -s -o /dev/null -w "%{http_code}" -H "Authorization: Bearer $TOKEN" "$BASE_URL/v1/disk/space")
assert_eq "$STATUS" "200" "有效 JWT 访问成功"

# 1.3 错误 Token → 401
STATUS=$(curl -s -o /dev/null -w "%{http_code}" -H "Authorization: Bearer invalid_token" "$BASE_URL/v1/disk/space")
assert_eq "$STATUS" "401" "无效 Token 拒绝访问"

# 1.4 用户隔离: user_A 上传 → user_B 无法访问
TOKEN_A=$(go run scripts/gen_token/main.go -secret "$JWT_SECRET" -userId "user_a" -agentId "")
TOKEN_B=$(go run scripts/gen_token/main.go -secret "$JWT_SECRET" -userId "user_b" -agentId "")

# user_A 创建文件夹
RESP=$(curl -s -X POST "$BASE_URL/v1/disk/folders" \
  -H "Authorization: Bearer $TOKEN_A" \
  -H "Content-Type: application/json" \
  -d '{"folderName":"test_private","parentId":0}')
FOLDER_ID=$(echo "$RESP" | jq -r '.data.id // empty')

if [ -n "$FOLDER_ID" ] && [ "$FOLDER_ID" != "null" ]; then
  # user_A 上传文件
  echo "test content for user_a" > /tmp/test_file_a.txt
  RESP=$(curl -s -X POST "$BASE_URL/v1/disk/files/upload" \
    -H "Authorization: Bearer $TOKEN_A" \
    -F "file=@/tmp/test_file_a.txt" \
    -F "folderId=$FOLDER_ID")
  FILE_ID=$(echo "$RESP" | jq -r '.data.id // empty')

  if [ -n "$FILE_ID" ] && [ "$FILE_ID" != "null" ]; then
    # user_B 尝试访问 user_A 的文件 → 应该 403/500
    STATUS=$(curl -s -o /dev/null -w "%{http_code}" \
      -H "Authorization: Bearer $TOKEN_B" \
      "$BASE_URL/v1/disk/files/$FILE_ID")
    [ "$STATUS" != "200" ] && log_pass "用户隔离: user_B 无法访问 user_A 的文件" || log_fail "用户隔离: user_B 可以访问 user_A 的文件"
  fi
fi

# ── 路径2: OAuth2 Web 认证（需要 mock server） ──
echo ""
echo "=== 路径2: OAuth2 Web 认证 ==="

# 2.1 登录跳转（标准模式，无 prompt=none）
RESP_HEADERS=$(curl -s -D - -o /dev/null "$BASE_URL/auth/login" 2>/dev/null || true)
if echo "$RESP_HEADERS" | grep -q "302\|303"; then
  log_pass "OAuth2 登录跳转到授权页"
else
  log_fail "OAuth2 登录跳转 (需要启用 oauth2.enabled=true 并配置端点)"
fi

# 2.2 无效回调 code → 拒绝
STATUS=$(curl -s -o /dev/null -w "%{http_code}" "$BASE_URL/auth/callback?code=invalid_code&state=invalid" 2>/dev/null || echo "000")
if [ "$STATUS" != "200" ]; then
  log_pass "OAuth2 无效 code 拒绝"
else
  log_fail "OAuth2 无效 code 应拒绝"
fi

# ── 路径2b: 网关无感跳转（SSO 自动授权） ──
echo ""
echo "=== 路径2b: 网关无感跳转 SSO ==="

# 2b.1 从网关跳转（带 from=gateway 参数）应使用 prompt=none
RESP_HEADERS=$(curl -s -D - -o /dev/null "$BASE_URL/auth/login?from=gateway" 2>/dev/null || true)
if echo "$RESP_HEADERS" | grep -q "prompt=none"; then
  log_pass "网关跳转: OAuth2 authorize URL 包含 prompt=none"
else
  log_fail "网关跳转: OAuth2 authorize URL 应包含 prompt=none (需要启用 oauth2)"
fi

# ── 路径3: 下载令牌 ──
echo ""
echo "=== 路径3: 下载令牌 ==="

# 3.1 用 JWT 换取下载令牌
if [ -n "${FILE_ID:-}" ] && [ "$FILE_ID" != "null" ] && [ -n "$FILE_ID" ]; then
  RESP=$(curl -s -X POST "$BASE_URL/v1/disk/files/$FILE_ID/download-token" \
    -H "Authorization: Bearer $TOKEN_A")
  DL_TOKEN=$(echo "$RESP" | jq -r '.data.downloadToken // empty')

  if [ -n "$DL_TOKEN" ]; then
    # 下载令牌有效 → 200
    STATUS=$(curl -s -o /dev/null -w "%{http_code}" \
      "$BASE_URL/v1/disk/files/download?t=$DL_TOKEN")
    assert_eq "$STATUS" "200" "下载令牌有效, 文件获取成功"
  else
    log_fail "下载令牌生成失败"
  fi
fi

# 3.2 无效下载令牌 → 拒绝
STATUS=$(curl -s -o /dev/null -w "%{http_code}" "$BASE_URL/v1/disk/files/download?t=invalid_token")
assert_eq "$STATUS" "401" "无效下载令牌拒绝"

# 3.3 过期下载令牌（生成一个已过期的）
EXPIRED_TOKEN=$(go run scripts/gen_dl_token/main.go -secret "$DL_SECRET" -userId "user_a" -fileId "1" -expired)
STATUS=$(curl -s -o /dev/null -w "%{http_code}" "$BASE_URL/v1/disk/files/download?t=$EXPIRED_TOKEN")
assert_eq "$STATUS" "401" "过期下载令牌拒绝"

# ── 清理 ──
rm -f /tmp/test_file_a.txt /tmp/downloaded_a.txt

# ── 汇总 ──
echo ""
echo "=============================="
echo "  通过: $PASS  失败: $FAIL"
echo "=============================="
[ "$FAIL" -eq 0 ] && echo "全部通过!" || echo "存在失败用例"
exit $FAIL
