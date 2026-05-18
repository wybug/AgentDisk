#!/usr/bin/env bash
# AgentDisk 全栈本地开发启动脚本
# 用途：一键启动/停止后端 + 测试网关 + Web 前端
set -euo pipefail

PROJECT_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
LOG_DIR="$PROJECT_ROOT/.dev-logs"
PID_FILE="$PROJECT_ROOT/.dev-pids"

# Load .env if present
if [ -f "$PROJECT_ROOT/.env" ]; then
  set -a
  source "$PROJECT_ROOT/.env"
  set +a
fi

# 颜色
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
NC='\033[0m'

info()  { echo -e "${GREEN}[INFO]${NC} $*"; }
warn()  { echo -e "${YELLOW}[WARN]${NC} $*"; }
error() { echo -e "${RED}[ERROR]${NC} $*"; }

# ---- 停止所有服务 ----
stop_all() {
  info "停止所有服务..."

  if [ -f "$PID_FILE" ]; then
    while IFS=: read -r name pid; do
      if kill -0 "$pid" 2>/dev/null; then
        info "停止 $name (PID $pid)"
        kill "$pid" 2>/dev/null || true
      fi
    done < "$PID_FILE"
    rm -f "$PID_FILE"
  fi

  # 兜底：按端口清理残留进程
  for port in 9100 3000 5173; do
    local pids
    pids=$(lsof -ti:$port 2>/dev/null || true)
    if [ -n "$pids" ]; then
      warn "清理端口 $port 残留进程: $pids"
      echo "$pids" | xargs kill 2>/dev/null || true
    fi
  done

  sleep 1
  info "所有服务已停止"
}

# ---- 启动单个服务 ----
start_service() {
  local name=$1
  local port=$2
  local cmd=$3
  local log_file="$LOG_DIR/${name}.log"

  # 检查端口是否已被占用
  if lsof -ti:$port 2>/dev/null | grep -q .; then
    error "$name 启动失败：端口 $port 已被占用"
    error "请先运行 $0 stop 或手动释放端口"
    return 1
  fi

  info "启动 $name (端口 $port)..."
  (cd "$PROJECT_ROOT" && eval "$cmd" > "$log_file" 2>&1) &
  local pid=$!
  echo "${name}:${pid}" >> "$PID_FILE"

  # 等待端口就绪（最多 15 秒）
  local waited=0
  while [ $waited -lt 15 ]; do
    if lsof -ti:$port 2>/dev/null | grep -q .; then
      info "$name 已启动 (PID $pid, 端口 $port)"
      return 0
    fi
    sleep 1
    waited=$((waited + 1))
  done

  # 检查进程是否还活着
  if kill -0 "$pid" 2>/dev/null; then
    warn "$name 端口未就绪但进程仍在运行，请检查日志: $log_file"
  else
    error "$name 启动失败，请检查日志: $log_file"
    tail -20 "$log_file" 2>/dev/null || true
    return 1
  fi
}

# ---- 启动所有服务 ----
start_all() {
  mkdir -p "$LOG_DIR"
  rm -f "$PID_FILE"

  echo ""
  echo -e "${CYAN}╔══════════════════════════════════════════╗${NC}"
  echo -e "${CYAN}║     AgentDisk 本地开发环境启动           ║${NC}"
  echo -e "${CYAN}╚══════════════════════════════════════════╝${NC}"
  echo ""

  # 1. 后端 API
  start_service "backend" 9100 \
    "go run main.go --config config.yaml" \
    || { error "后端启动失败，终止"; exit 1; }

  # 2. 测试网关
  start_service "gateway" 3000 \
    "cd gateway && node --import tsx src/index.ts" \
    || { warn "网关启动失败，继续启动前端"; }

  # 3. Web 前端
  start_service "web" 5173 \
    "cd web && node_modules/.bin/vite --host" \
    || { warn "前端启动失败"; }

  echo ""
  echo -e "${GREEN}╔══════════════════════════════════════════╗${NC}"
  echo -e "${GREEN}║     所有服务已启动                        ║${NC}"
  echo -e "${GREEN}╠══════════════════════════════════════════╣${NC}"
  echo -e "${GREEN}║  后端 API:   http://localhost:9100       ║${NC}"
  echo -e "${GREEN}║  测试网关:   http://localhost:3000       ║${NC}"
  echo -e "${GREEN}║  Web 前端:   http://localhost:5173       ║${NC}"
  echo -e "${GREEN}╠══════════════════════════════════════════╣${NC}"
  echo -e "${GREEN}║  日志目录: $LOG_DIR/"
  echo -e "${GREEN}║  停止命令: $0 stop"
  echo -e "${GREEN}╚══════════════════════════════════════════╝${NC}"
  echo ""
}

# ---- 查看状态 ----
show_status() {
  echo ""
  echo "AgentDisk 服务状态:"
  echo "─────────────────────────────"
  for svc in "backend:9100" "gateway:3000" "web:5173"; do
    local name="${svc%%:*}"
    local port="${svc##*:}"
    if lsof -ti:$port 2>/dev/null | grep -q .; then
      echo -e "  $name (:$port)  ${GREEN}运行中${NC}"
    else
      echo -e "  $name (:$port)  ${RED}未运行${NC}"
    fi
  done
  echo ""
}

# ---- 查看日志 ----
show_logs() {
  local name=${1:-all}
  if [ "$name" = "all" ]; then
    for f in "$LOG_DIR"/*.log; do
      [ -f "$f" ] || continue
      echo "=== $(basename "$f") (最近 30 行) ==="
      tail -30 "$f"
      echo ""
    done
  else
    local log_file="$LOG_DIR/${name}.log"
    if [ -f "$log_file" ]; then
      tail -f "$log_file"
    else
      error "日志文件不存在: $log_file"
    fi
  fi
}

# ---- 主入口 ----
case "${1:-start}" in
  start)
    stop_all 2>/dev/null || true
    start_all
    ;;
  stop)
    stop_all
    ;;
  restart)
    stop_all
    sleep 2
    start_all
    ;;
  status)
    show_status
    ;;
  logs)
    show_logs "${2:-all}"
    ;;
  *)
    echo "用法: $0 {start|stop|restart|status|logs [service]}"
    echo ""
    echo "  start    启动所有服务（先停止已有服务）"
    echo "  stop     停止所有服务"
    echo "  restart  重启所有服务"
    echo "  status   查看服务状态"
    echo "  logs     查看日志（可指定 backend/gateway/web）"
    exit 1
    ;;
esac
