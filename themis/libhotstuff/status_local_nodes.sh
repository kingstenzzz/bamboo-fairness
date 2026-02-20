#!/bin/bash

# 查看本地 HotStuff 节点状态
# 删除编译部分后的版本

GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m'

echo_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

echo_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

echo ""
echo "========================================"
echo "  HotStuff 本地节点状态"
echo "========================================"
echo ""

# 检查进程
PIDS=$(pgrep -f "hotstuff --conf" || true)

if [ -z "$PIDS" ]; then
    echo_warn "没有运行中的 HotStuff 节点"
else
    echo_info "运行中的节点:"
    echo ""
    ps aux | grep "[h]otstuff --conf" | awk '{print "  PID: "$2", CPU: "$3"%, MEM: "$4"%, CMD: "$11" "$12" "$13}'
    echo ""
    
    # 统计数量
    COUNT=$(echo "$PIDS" | wc -w | tr -d ' ')
    echo_info "总共 $COUNT 个节点正在运行"
fi

echo ""

# 检查端口占用
echo_info "端口占用情况:"
echo ""
for port in 10000 10001 10002 10003 20000 20001 20002 20003; do
    if lsof -i :$port > /dev/null 2>&1; then
        PROC=$(lsof -i :$port | tail -1 | awk '{print $1" (PID: "$2")"}')
        echo "  ✓ 端口 $port: $PROC"
    else
        echo "  - 端口 $port: 未使用"
    fi
done

echo ""

# 检查日志目录
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
LOG_DIR="$SCRIPT_DIR/logs"

if [ -d "$LOG_DIR" ]; then
    echo_info "日志文件:"
    echo ""
    for log in "$LOG_DIR"/replica*.log; do
        if [ -f "$log" ]; then
            SIZE=$(ls -lh "$log" | awk '{print $5}')
            LINES=$(wc -l < "$log" | tr -d ' ')
            echo "  $(basename $log): $SIZE, $LINES 行"
        fi
    done
    echo ""
    echo "查看日志: tail -f $LOG_DIR/replica0.log"
else
    echo_warn "日志目录不存在: $LOG_DIR"
fi

echo ""
echo "========================================"
