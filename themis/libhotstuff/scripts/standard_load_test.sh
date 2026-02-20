#!/bin/bash

# 标准HotStuff压力测试脚本
# 符合官方测试格式和输出标准

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
LIBHOTSTUFF_DIR="$SCRIPT_DIR/.."

# 参数设置
NUM_NODES=${1:-4}
TEST_ITERATIONS=${2:-1000}
MAX_ASYNC=${3:-100}

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

# 启动节点集群
start_cluster() {
    echo_step "启动 $NUM_NODES 个节点集群..."
    
    cd "$LIBHOTSTUFF_DIR"
    
    # 设置ulimit
    if ! ulimit -s unlimited 2>/dev/null; then
        echo_warn "无法设置无限栈大小，继续执行"
    fi
    
    # 创建日志目录
    mkdir -p logs
    
    # 启动所有节点
    echo_info "启动所有节点..."
    for i in $(seq 0 $((NUM_NODES-1))); do
        echo_info "启动节点 $i"
         ./examples/hotstuff-app --conf ./hotstuff-sec${i}.conf > logs/node${i}.log 2>&1 &
        sleep 1  # 节点间启动间隔
    done
    
    # 等待集群稳定
    echo_info "等待集群稳定 (约10秒)..."
    sleep 10
    
    # 验证节点运行状态
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
        echo_error "集群启动不完整 ($running_count/$NUM_NODES)"
        exit 1
    fi
    
    echo_info "✓ 集群启动完成 ($running_count/$NUM_NODES 节点运行中)"
}

# 执行压力测试
run_load_test() {
    echo_step "执行压力测试..."
    echo "测试参数:"
    echo "  节点数量: $NUM_NODES"
    echo "  迭代次数: $TEST_ITERATIONS"
    echo "  最大并发: $MAX_ASYNC"
    echo ""
    
    cd "$LIBHOTSTUFF_DIR"
    
    # 执行测试并捕获stderr输出用于分析
    local test_log="logs/test_$(date +%Y%m%d_%H%M%S).log"
    local start_time=$(date +%s)
    
    echo_info "开始执行压力测试..."
    echo_info "测试日志将保存到: $test_log"
    
    # 执行测试，重定向stderr到日志文件以便后续分析
    if ./examples/hotstuff-client --idx 0 --iter $TEST_ITERATIONS --max-async $MAX_ASYNC 2>"$test_log"; then
        local exit_code=0
        echo_info "✓ 压力测试执行完成"
    else
        local exit_code=$?
        echo_error "✗ 压力测试执行失败 (退出码: $exit_code)"
    fi
    
    local end_time=$(date +%s)
    local duration=$((end_time - start_time))
    
    echo ""
    echo_step "测试统计"
    echo "执行时间: ${duration} 秒"
    echo "测试结果: $([ $exit_code -eq 0 ] && echo "成功" || echo "失败")"
    echo "日志文件: $test_log"
    
    # 分析测试结果
    if [ -f "$test_log" ] && [ $exit_code -eq 0 ]; then
        echo ""
        echo_step "性能分析结果"
        echo "原始数据和统计:"
        cat "$test_log" | python3 "$SCRIPT_DIR/thr_hist.py"
    fi
    
    return $exit_code
}

# 显示使用说明
show_usage() {
    echo "HotStuff 标准压力测试脚本"
    echo ""
    echo "用法: $0 [节点数] [迭代次数] [最大并发数]"
    echo ""
    echo "参数:"
    echo "  节点数      - 节点数量 (默认: 4)"
    echo "  迭代次数    - 测试迭代次数 (默认: 1000)"
    echo "  最大并发数  - 客户端最大并发请求数 (默认: 100)"
    echo ""
    echo "示例:"
    echo "  $0                    # 默认参数测试"
    echo "  $0 6 2000 150        # 6节点，2000次迭代，150并发"
    echo ""
    echo "输出格式示例:"
    echo "[349669, 367520, 371855, 370391, 366159, 367565, 365957, 322690]"
    echo "lat = 6.955ms # mean end-to-end latency"
    echo "lat = 6.970ms # after removing outliers"
}

# 主函数
main() {
    # 检查帮助参数
    if [[ "$1" == "-h" ]] || [[ "$1" == "--help" ]]; then
        show_usage
        exit 0
    fi
    
    echo ""
    echo "================================"
    echo "  HotStuff 标准压力测试"
    echo "================================"
    echo "节点数: $NUM_NODES"
    echo "迭代数: $TEST_ITERATIONS"
    echo "并发数: $MAX_ASYNC"
    echo "工作目录: $LIBHOTSTUFF_DIR"
    echo ""
    
    # 设置清理陷阱
    trap cleanup EXIT INT TERM
    
    # 执行测试流程
    check_dependencies
    start_cluster
    run_load_test
    
    echo ""
    echo_info "🎉 标准压力测试完成！"
    echo_info "如需重新测试，请先执行: pkill -f hotstuff-app"
}

# 运行主函数
main "$@"