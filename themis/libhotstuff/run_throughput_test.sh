#!/bin/bash
# HotStuff 吞吐量测试脚本

set -euo pipefail

# 颜色输出
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

echo_info() { echo -e "${BLUE}[INFO]${NC} $1"; }
echo_success() { echo -e "${GREEN}[SUCCESS]${NC} $1"; }
echo_warn() { echo -e "${YELLOW}[WARN]${NC} $1"; }
echo_error() { echo -e "${RED}[ERROR]${NC} $1"; }

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

echo "=== HotStuff 吞吐量测试 ==="
echo ""

# 检查必要文件
echo "检查可执行文件..."
if [ ! -f "examples/hotstuff-app" ]; then
    echo_error "❌ hotstuff-app 未找到，请先编译"
    exit 1
fi

if [ ! -f "examples/hotstuff-client" ]; then
    echo_error "❌ hotstuff-client 未找到，请先编译"
    exit 1
fi

# 检查配置文件
echo "检查配置文件..."
for i in {0..3}; do
    if [ ! -f "hotstuff-sec${i}.conf" ]; then
        echo_error "❌ 缺少配置文件 hotstuff-sec${i}.conf"
        exit 1
    fi
done

echo_success "✅ 所有必需文件检查通过"

# 默认参数
TEST_DURATION=${1:-30}  # 测试持续时间（秒）
CLIENTS=${2:-1}          # 客户端数量
REQ_PER_SEC=${3:-100}    # 每秒请求数

echo ""
echo "测试参数:"
echo "  持续时间: ${TEST_DURATION}s"
echo "  客户端数: $CLIENTS"
echo "  请求速率: ${REQ_PER_SEC}/s"

# 清理旧进程和日志
echo ""
echo_info "清理旧进程和日志..."
pkill -f "hotstuff-app" 2>/dev/null || true
pkill -f "hotstuff-client" 2>/dev/null || true
sleep 2

mkdir -p logs
rm -f logs/node*.log
rm -f logs/client*.log

# 启动节点
echo ""
echo_info "启动 HotStuff 节点..."

# 启动4个节点
for i in {0..3}; do
    echo_info "启动节点 $i (端口: $((10000+i)), $((20000+i)))"
    echo_warn "🚨 注意：macOS防火墙可能会弹窗，请准备好点击'允许'"
    ./examples/hotstuff-app --conf ./hotstuff-sec${i}.conf > logs/node${i}.log 2>&1 &
    sleep 1
done

echo_info "等待集群启动稳定 (10秒)..."
sleep 10

# 检查节点是否都在运行
running_count=0
for i in {0..3}; do
    if pgrep -f "hotstuff-app.*hotstuff-sec${i}.conf" > /dev/null; then
        echo_success "✅ 节点 $i 运行中"
        ((running_count++))
    else
        echo_error "❌ 节点 $i 启动失败"
    fi
done

if [ $running_count -ne 4 ]; then
    echo_error "❌ 集群启动不完整 ($running_count/4)，请检查日志"
    exit 1
fi

echo_success "✅ 集群启动完成"

# 设置环境变量启用benchmark
export HOTSTUFF_ENABLE_BENCHMARK=1

# 记录开始时间
START_TIME=$(date +%s.%N)
echo ""
echo_info "开始吞吐量测试..."

# 启动客户端测试
LOG_SUFFIX=$(date +%Y%m%d_%H%M%S)
CLIENT_LOG="logs/client_${LOG_SUFFIX}.log"

# 计算总请求数
TOTAL_REQUESTS=$((REQ_PER_SEC * TEST_DURATION))

echo_info "发送 $TOTAL_REQUESTS 个请求，持续 $TEST_DURATION 秒..."

# 启动客户端
./examples/hotstuff-client --idx 0 --iter $TOTAL_REQUESTS --max-async $((REQ_PER_SEC/2)) > /dev/null 2>"$CLIENT_LOG" &
CLIENT_PID=$!

# 监控测试进度
echo_info "客户端PID: $CLIENT_PID"
echo_info "测试运行中，将持续 $TEST_DURATION 秒..."

# 等待测试完成或超时
wait $CLIENT_PID
END_TIME=$(date +%s.%N)

echo ""
echo_success "✅ 吞吐量测试完成！"

# 计算实际运行时间
ACTUAL_DURATION=$(echo "$END_TIME - $START_TIME" | bc -l)
echo_info "实际运行时间: ${ACTUAL_DURATION}s"

# 从日志中计算吞吐量
echo ""
echo_info "分析测试结果..."

# 从节点日志中获取吞吐量数据
DELIVERED_COUNT=0
if [ -f "logs/node0.log" ]; then
    # 查找统计部分中delivered的数量
    DELIVERED_LINE=$(grep "delivered:" logs/node0.log | grep -v "avg" | tail -n 1 | awk '{print $NF}')
    if [ -n "$DELIVERED_LINE" ] && [ "$DELIVERED_LINE" -ge 0 ] 2>/dev/null; then
        DELIVERED_COUNT=$DELIVERED_LINE
    fi
fi

# 计算吞吐量
if (( $(echo "$ACTUAL_DURATION > 0" | bc -l) )); then
    THROUGHPUT=$(echo "scale=2; $DELIVERED_COUNT / $ACTUAL_DURATION" | bc -l)
    echo_success "✅ 性能指标:"
    echo "  - 成功处理的请求: $DELIVERED_COUNT"
    echo "  - 实际运行时间: $ACTUAL_DURATION s"
    echo "  - 平均吞吐量: $THROUGHPUT TPS"
else
    echo_error "❌ 无法计算吞吐量，运行时间无效"
fi

# 显示关键统计信息
echo ""
echo_info "节点统计信息 (最后10秒):"
if [ -f "logs/node0.log" ]; then
    echo "节点0最后统计信息:"
    tail -n 50 logs/node0.log | grep -A 20 "begin stats" | grep -E "(delivered|decided|sent|recv|avg\. parent_size)" | tail -n 10
fi

echo ""
echo_info "详细日志保存在 logs/ 目录下"
echo_info "客户端日志: $CLIENT_LOG"