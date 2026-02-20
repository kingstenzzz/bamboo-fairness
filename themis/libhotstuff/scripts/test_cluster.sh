#!/bin/bash

# 简化版 HotStuff 集群测试脚本
# 基于 run_demo.sh 和 run_demo_client.sh

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
LIBHOTSTUFF_DIR="$SCRIPT_DIR/.."

# 参数设置
NUM_NODES=${1:-4}  # 默认4个节点
TEST_ITERATIONS=${2:-1000}  # 默认1000次迭代
MAX_ASYNC=${3:-100}  # 默认100并发

# 颜色输出
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
BLUE='\033[0;34m'
NC='\033[0m'

echo_info() { echo -e "${GREEN}[INFO]${NC} $1"; }
echo_warn() { echo -e "${YELLOW}[WARN]${NC} $1"; }
echo_error() { echo -e "${RED}[ERROR]${NC} $1"; }
echo_step() { echo -e "${BLUE}[STEP]${NC} $1"; }

# 清理函数
cleanup() {
    echo_step "清理进程..."
    pkill -f "hotstuff-app" 2>/dev/null || true
    pkill -f "hotstuff-client" 2>/dev/null || true
    sleep 2
}

# 检查依赖
check_dependencies() {
    echo_step "检查依赖..."
    
    if [ ! -f "$LIBHOTSTUFF_DIR/examples/hotstuff-app" ]; then
        echo_error "hotstuff-app 不存在，请先编译"
        exit 1
    fi
    
    if [ ! -f "$LIBHOTSTUFF_DIR/examples/hotstuff-client" ]; then
        echo_error "hotstuff-client 不存在，请先编译"
        exit 1
    fi
    
    # 检查配置文件
    for i in $(seq 0 $((NUM_NODES-1))); do
        if [ ! -f "$LIBHOTSTUFF_DIR/hotstuff-sec${i}.conf" ]; then
            echo_error "配置文件 hotstuff-sec${i}.conf 不存在"
            exit 1
        fi
    done
    
    echo_info "✓ 依赖检查通过"
}

# 启动节点
start_nodes() {
    echo_step "启动 $NUM_NODES 个节点..."
    
    cd "$LIBHOTSTUFF_DIR"
    # 在 macOS 上可能需要 sudo 权限来设置 ulimit
    ulimit -s unlimited 2>/dev/null || echo_warn "无法设置无限栈大小，继续执行"
    
    # 创建日志目录
    mkdir -p logs
    
    # 启动节点
    for i in $(seq 0 $((NUM_NODES-1))); do
        echo_info "启动节点 $i"
        nohup ./examples/hotstuff-app --conf ./hotstuff-sec${i}.conf > logs/node${i}.log 2>&1 &
        sleep 0.5
    done
    
    # 等待启动
    echo_info "等待节点启动..."
    sleep 5
    
    # 检查进程
    local running_count=0
    for i in $(seq 0 $((NUM_NODES-1))); do
        if pgrep -f "hotstuff-app.*hotstuff-sec${i}.conf" > /dev/null; then
            echo_info "✓ 节点 $i 运行中"
            ((running_count++))
        else
            echo_error "✗ 节点 $i 启动失败"
        fi
    done
    
    if [ $running_count -ne $NUM_NODES ]; then
        echo_error "节点启动不完整 ($running_count/$NUM_NODES)"
        exit 1
    fi
}

# 运行测试
run_test() {
    echo_step "运行压力测试..."
    echo "测试配置:"
    echo "  节点数: $NUM_NODES"
    echo "  迭代次数: $TEST_ITERATIONS"
    echo "  最大并发: $MAX_ASYNC"
    echo ""
    
    cd "$LIBHOTSTUFF_DIR"
    
    # 执行测试
    local start_time=$(date +%s)
    ./examples/hotstuff-client --idx 0 --iter $TEST_ITERATIONS --max-async $MAX_ASYNC
    local exit_code=$?
    local end_time=$(date +%s)
    
    local duration=$((end_time - start_time))
    
    echo ""
    echo_step "测试完成"
    echo "耗时: ${duration} 秒"
    echo "结果: $([ $exit_code -eq 0 ] && echo "成功" || echo "失败")"
    
    return $exit_code
}

# 主函数
main() {
    echo ""
    echo "================================"
    echo "  HotStuff 集群测试"
    echo "================================"
    echo "节点数: $NUM_NODES"
    echo "迭代数: $TEST_ITERATIONS"
    echo "并发数: $MAX_ASYNC"
    echo ""
    
    # 设置清理陷阱
    trap cleanup EXIT INT TERM
    
    # 执行步骤
    check_dependencies
    start_nodes
    run_test
    
    echo ""
    echo_info "🎉 测试完成！"
}

# 运行主函数
main "$@"