#!/bin/bash

# 本地多节点 HotStuff 启动脚本
# 在本地启动多个 HotStuff 副本节点，使用不同端口

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
LIBHOTSTUFF_DIR="$SCRIPT_DIR"

# 颜色输出
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
BLUE='\033[0;34m'
NC='\033[0m'

echo_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

echo_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

echo_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

echo_step() {
    echo -e "${BLUE}[STEP]${NC} $1"
}

# 配置参数
NUM_REPLICAS=4
BASE_PEER_PORT=10000
BASE_CLIENT_PORT=20000
LOG_DIR="$LIBHOTSTUFF_DIR/logs"

echo ""
echo "========================================"
echo "  本地多节点 HotStuff 启动"
echo "========================================"
echo ""
echo "配置:"
echo "  副本数量: $NUM_REPLICAS"
echo "  副本通信端口: $BASE_PEER_PORT-$((BASE_PEER_PORT+NUM_REPLICAS-1))"
echo "  客户端通信端口: $BASE_CLIENT_PORT-$((BASE_CLIENT_PORT+NUM_REPLICAS-1))"
echo "  工作目录: $LIBHOTSTUFF_DIR"
echo ""

# 步骤 1: 检查可执行文件
echo_step "1. 检查可执行文件"

if [ ! -f "$LIBHOTSTUFF_DIR/hotstuff" ]; then
    echo_error "hotstuff 可执行文件不存在，请先编译项目"
    echo ""
    echo "编译步骤:"
    echo "  cd $LIBHOTSTUFF_DIR"
    echo "  cmake -DCMAKE_BUILD_TYPE=Release -DBUILD_SHARED=ON ."
    echo "  make -j$(sysctl -n hw.ncpu)"
    echo ""
    exit 1
fi

echo_info "✓ 找到可执行文件: $LIBHOTSTUFF_DIR/hotstuff"

# 步骤 2: 检查配置文件
echo ""
echo_step "2. 检查配置文件"

if [ ! -f "$LIBHOTSTUFF_DIR/hotstuff.conf" ]; then
    echo_error "配置文件 hotstuff.conf 不存在"
    echo_info "请先生成配置文件"
    exit 1
fi

for i in $(seq 0 $((NUM_REPLICAS-1))); do
    if [ ! -f "$LIBHOTSTUFF_DIR/hotstuff-sec$i.conf" ]; then
        echo_error "配置文件 hotstuff-sec$i.conf 不存在"
        exit 1
    fi
done

echo_info "✓ 所有配置文件已就绪"

# 步骤 3: 创建日志目录
echo ""
echo_step "3. 准备日志目录"
mkdir -p "$LOG_DIR"
echo_info "✓ 日志目录: $LOG_DIR"

# 步骤 4: 停止旧的进程
echo ""
echo_step "4. 清理旧进程"
pkill -f "hotstuff --conf" 2>/dev/null && echo_info "✓ 已停止旧的 hotstuff 进程" || echo_info "没有运行中的进程"

# 步骤 5: 启动所有副本
echo ""
echo_step "5. 启动副本节点"

PIDS=()

for i in $(seq 0 $((NUM_REPLICAS-1))); do
    PEER_PORT=$((BASE_PEER_PORT + i))
    CLIENT_PORT=$((BASE_CLIENT_PORT + i))
    LOG_FILE="$LOG_DIR/replica$i.log"
    
    echo_info "启动副本 $i (端口: $PEER_PORT, $CLIENT_PORT)"
    
    cd "$LIBHOTSTUFF_DIR"
    nohup ./hotstuff \
        --conf "$LIBHOTSTUFF_DIR/hotstuff.conf" \
        --conf "$LIBHOTSTUFF_DIR/hotstuff-sec$i.conf" \
        > "$LOG_FILE" 2>&1 &
    
    PID=$!
    PIDS+=($PID)
    echo_info "  进程 ID: $PID, 日志: $LOG_FILE"
    
    sleep 0.5
done

# 步骤 6: 等待启动
echo ""
echo_step "6. 等待节点启动"
sleep 3

# 步骤 7: 检查进程状态
echo ""
echo_step "7. 检查进程状态"

ALL_RUNNING=true
for i in $(seq 0 $((NUM_REPLICAS-1))); do
    PID=${PIDS[$i]}
    if ps -p $PID > /dev/null 2>&1; then
        echo_info "✓ 副本 $i (PID: $PID) 正在运行"
    else
        echo_error "✗ 副本 $i (PID: $PID) 启动失败"
        echo_warn "  查看日志: tail -50 $LOG_DIR/replica$i.log"
        ALL_RUNNING=false
    fi
done

echo ""
if [ "$ALL_RUNNING" = true ]; then
    echo_info "========================================"
    echo_info "✓ 所有 $NUM_REPLICAS 个副本节点启动成功！"
    echo_info "========================================"
    echo ""
    echo "管理命令:"
    echo "  查看日志:   tail -f $LOG_DIR/replica0.log"
    echo "  查看所有:   tail -f $LOG_DIR/*.log"
    echo "  停止所有:   pkill -f 'hotstuff --conf'"
    echo "  查看进程:   ps aux | grep hotstuff"
    echo ""
    echo "端口分配:"
    for i in $(seq 0 $((NUM_REPLICAS-1))); do
        echo "  副本 $i: 127.0.0.1:$((BASE_PEER_PORT+i)) (副本通信), :$((BASE_CLIENT_PORT+i)) (客户端)"
    done
    echo ""
else
    echo_error "========================================"
    echo_error "部分节点启动失败"
    echo_error "========================================"
    echo ""
    echo "故障排查:"
    echo "  1. 查看日志文件了解错误原因"
    echo "  2. 检查端口是否被占用: lsof -i :10000"
    echo "  3. 停止所有进程: pkill -f 'hotstuff --conf'"
    echo ""
    exit 1
fi
