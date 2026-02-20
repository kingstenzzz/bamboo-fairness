#!/bin/bash

# 改进版 HotStuff 集群测试脚本
# 基于 run_demo.sh 和 run_demo_client.sh，优化macOS兼容性

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
LIBHOTSTUFF_DIR="$SCRIPT_DIR/.."

# 默认参数设置
NUM_NODES=4
TEST_ITERATIONS=1000
MAX_ASYNC=100
MAX_DURATION=300  # 默认最大运行时间5分钟
VERBOSE=false
CLEAN_START=false

# 解析命令行参数
show_help() {
    echo "HotStuff 集群压力测试脚本"
    echo ""
    echo "用法: $0 [选项]"
    echo ""
    echo "选项:"
    echo "  -n, --nodes NUM        节点数量 (默认: $NUM_NODES)"
    echo "  -i, --iterations NUM   测试迭代次数 (默认: $TEST_ITERATIONS)"
    echo "  -a, --async NUM        最大并发请求数 (默认: $MAX_ASYNC)"
    echo "  -t, --timeout SEC      最大运行时间(秒) (默认: $MAX_DURATION)"
    echo "  -v, --verbose          详细输出模式"
    echo "  -c, --clean            清理之前运行的进程"
    echo "  -h, --help             显示此帮助信息"
    echo ""
    echo "示例:"
    echo "  $0                           # 使用默认参数运行"
    echo "  $0 -n 4 -i 1000 -a 100      # 4节点，1000次迭代，100并发"
    echo "  $0 -t 60 -i 500 -a 50       # 限制运行时间为60秒"
    echo "  $0 --nodes 4 --iterations 2000 --async 150 --timeout 30 --verbose"
    echo ""
    exit 0
}

# 解析参数
while [[ $# -gt 0 ]]; do
    case $1 in
        -n|--nodes)
            NUM_NODES="$2"
            shift 2
            ;;
        -i|--iterations)
            TEST_ITERATIONS="$2"
            shift 2
            ;;
        -a|--async)
            MAX_ASYNC="$2"
            shift 2
            ;;
        -t|--timeout)
            MAX_DURATION="$2"
            shift 2
            ;;
        -v|--verbose)
            VERBOSE=true
            shift
            ;;
        -c|--clean)
            CLEAN_START=true
            shift
            ;;
        -h|--help)
            show_help
            ;;
        *)
            echo "未知选项: $1"
            show_help
            ;;
    esac
done

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

# 检查端口占用
check_ports() {
    echo_step "检查端口占用..."
    local ports_busy=false
    
    for i in $(seq 0 $((NUM_NODES-1))); do
        local peer_port=$((10000 + i))
        local client_port=$((20000 + i))
        
        if lsof -i ":$peer_port" >/dev/null 2>&1; then
            echo_warn "端口 $peer_port 已被占用"
            ports_busy=true
        fi
        
        if lsof -i ":$client_port" >/dev/null 2>&1; then
            echo_warn "端口 $client_port 已被占用"
            ports_busy=true
        fi
    done
    
    if [ "$ports_busy" = true ]; then
        echo_warn "建议先清理占用的端口或使用不同的端口范围"
    else
        echo_info "✓ 端口检查通过"
    fi
}

# 检查依赖
check_dependencies() {
    echo_step "检查依赖..."
    
    if [ ! -f "$LIBHOTSTUFF_DIR/examples/hotstuff-app" ]; then
        echo_error "hotstuff-app 不存在，请先编译"
        echo_info "编译命令: cd $LIBHOTSTUFF_DIR && make"
        exit 1
    fi
    
    if [ ! -f "$LIBHOTSTUFF_DIR/examples/hotstuff-client" ]; then
        echo_error "hotstuff-client 不存在，请先编译"
        echo_info "编译命令: cd $LIBHOTSTUFF_DIR && make"
        exit 1
    fi
    
    # 检查是否启用了benchmark模式
    echo_step "检查benchmark支持..."
    if strings "$LIBHOTSTUFF_DIR/examples/hotstuff-client" | grep -q "HOTSTUFF_ENABLE_BENCHMARK"; then
        echo_info "✓ 检测到benchmark支持已启用"
    else
        echo_warn "⚠️ 未检测到benchmark支持"
        echo_info "建议重新编译: cd $LIBHOTSTUFF_DIR && make clean && make CXXFLAGS='-DHOTSTUFF_ENABLE_BENCHMARK'"
    fi
    
    # 检查配置文件
    local missing_configs=()
    for i in $(seq 0 $((NUM_NODES-1))); do
        if [ ! -f "$LIBHOTSTUFF_DIR/hotstuff-sec${i}.conf" ]; then
            missing_configs+=("hotstuff-sec${i}.conf")
        fi
    done
    
    if [ ${#missing_configs[@]} -gt 0 ]; then
        echo_error "缺少配置文件: ${missing_configs[*]}"
        echo_info "生成配置文件: cd $LIBHOTSTUFF_DIR && python3 scripts/gen_conf.py --prefix hotstuff --iter $NUM_NODES"
        exit 1
    fi
    
    echo_info "✓ 依赖检查通过"
}

# 启动节点
start_nodes() {
    echo_step "启动 $NUM_NODES 个节点..."
    
    cd "$LIBHOTSTUFF_DIR"
    
    # 尝试设置ulimit（macOS可能需要特殊处理）
    if ! ulimit -s unlimited 2>/dev/null; then
        echo_warn "无法设置无限栈大小（macOS限制），继续执行"
    fi
    
    # 创建日志目录
    mkdir -p logs
    
    # 启动节点
    local pids=()
    for i in $(seq 0 $((NUM_NODES-1))); do
        echo_info "启动节点 $i (端口: $((10000+i)), $((20000+i)))"
        echo_warn "🚨 注意：macOS防火墙可能会弹窗，请准备好点击'允许'"

         ./examples/hotstuff-app --conf ./hotstuff-sec${i}.conf > logs/node${i}.log 2>&1 &
        pids+=($!)
        echo_info "等待5秒后启动下一个节点..."
    done
    
    # 等待启动
    echo_info "等待所有节点启动完成..."
    sleep 10
    
    # 检查进程状态
    local running_count=0
    local failed_nodes=()
    
    for i in $(seq 0 $((NUM_NODES-1))); do
        if ps -p ${pids[$i]} > /dev/null 2>&1; then
            echo_info "✓ 节点 $i (PID: ${pids[$i]}) 运行中"
            ((running_count++))
        else
            echo_error "✗ 节点 $i 启动失败"
            failed_nodes+=($i)
        fi
    done
    
    # 显示失败节点的日志片段
    if [ ${#failed_nodes[@]} -gt 0 ]; then
        echo_error "启动失败的节点日志摘要:"
        for node in "${failed_nodes[@]}"; do
            echo_warn "节点 $node 日志 (最后10行):"
            tail -10 "logs/node${node}.log" 2>/dev/null || echo "日志文件不存在"
            echo ""
        done
        echo_error "节点启动不完整 ($running_count/$NUM_NODES)"
        exit 1
    fi
    
    echo_info "✓ 所有节点启动成功 ($running_count/$NUM_NODES)"
}

# 等待集群稳定
wait_for_stable_cluster() {
    echo_step "等待集群稳定..."
    echo_info "预计需要10-15秒让节点建立连接"
    
    # 等待一段时间让节点建立网络连接
    for i in {1..15}; do
        echo -n "."
        sleep 1
    done
    echo ""
    
    # 检查网络连接状态
    local connections_established=true
    for i in $(seq 0 $((NUM_NODES-1))); do
        if ! netstat -an | grep -E ":1000${i}|:2000${i}" | grep ESTABLISHED >/dev/null 2>&1; then
            connections_established=false
        fi
    done
    
    if [ "$connections_established" = true ]; then
        echo_info "✓ 集群网络连接已建立"
    else
        echo_warn "⚠ 集群可能仍在建立连接中"
    fi
}

# 运行测试
run_test() {
    echo_step "运行压力测试..."
    echo "测试配置:"
    echo "  节点数: $NUM_NODES"
    echo "  迭代次数: $TEST_ITERATIONS"
    echo "  最大并发: $MAX_ASYNC"
    echo "  详细模式: $([ "$VERBOSE" = true ] && echo "开启" || echo "关闭")"
    echo ""
    echo_info "💡 macOS用户请注意：首次运行可能会弹出防火墙确认对话框，请点击'允许'"
    echo ""
    
    cd "$LIBHOTSTUFF_DIR"
    
    # 启用benchmark模式
    export HOTSTUFF_ENABLE_BENCHMARK=1
    echo_info "✅ 已启用benchmark模式"
    
    # 验证benchmark支持
    if [ "$VERBOSE" = true ]; then
        echo_info "验证benchmark支持状态..."
        if strings "$LIBHOTSTUFF_DIR/examples/hotstuff-client" | grep -q "HOTSTUFF_ENABLE_BENCHMARK"; then
            echo_info "✓ 编译时benchmark支持已启用"
        else
            echo_warn "⚠ 编译时可能未启用benchmark支持"
            echo_info "建议重新编译: cd $LIBHOTSTUFF_DIR && make clean && make CXXFLAGS='-DHOTSTUFF_ENABLE_BENCHMARK'"
        fi
    fi
    
    # 准备测试命令
    local test_cmd="./examples/hotstuff-client --idx 0 --iter $TEST_ITERATIONS --max-async $MAX_ASYNC"
    local test_stderr="logs/test_stderr_$(date +%Y%m%d_%H%M%S).log"
    
    if [ "$VERBOSE" = true ]; then
        echo_info "执行命令: $test_cmd"
        echo_info "stderr输出: $test_stderr"
    fi
    
    # 执行测试（带超时控制）
    local start_time=$(date +%s)
    echo_info "开始执行测试 (超时: ${MAX_DURATION}秒)..."
    
    # 设置超时监控
    local timeout_pid=0
    (
        sleep $MAX_DURATION
        if ps -p $$ > /dev/null 2>&1; then
            echo_error "⏰ 测试超时 (${MAX_DURATION}秒)，正在终止..."
            pkill -P $$ -f "hotstuff-client" 2>/dev/null || true
            kill -TERM $$ 2>/dev/null || true
        fi
    ) &
    timeout_pid=$!
    
    if [ "$VERBOSE" = true ]; then
        # 详细模式：显示实时输出
        if $test_cmd 2>"$test_stderr"; then
            local exit_code=0
            echo_info "✓ 测试执行完成"
        else
            local exit_code=$?
            echo_error "✗ 测试执行失败 (退出码: $exit_code)"
        fi
    else
        # 静默模式：重定向输出
        if $test_cmd > logs/client.log 2>"$test_stderr"; then
            local exit_code=0
            echo_info "✓ 测试执行完成"
        else
            local exit_code=$?
            echo_error "✗ 测试执行失败 (退出码: $exit_code)"
            if [ -f "$test_stderr" ]; then
                echo_warn "错误日志摘要:"
                tail -10 "$test_stderr"
            fi
        fi
    fi
    
    # 清理超时监控进程
    if [ $timeout_pid -ne 0 ]; then
        kill $timeout_pid 2>/dev/null || true
    fi
    
    local end_time=$(date +%s)
    local duration=$((end_time - start_time))
    
    echo ""
    echo_step "测试统计"
    echo "执行时间: ${duration} 秒"
    echo "测试结果: $([ $exit_code -eq 0 ] && echo "成功" || echo "失败")"
    echo "stderr日志: $test_stderr"
    
    # 如果测试成功，分析性能数据
    if [ $exit_code -eq 0 ] && [ -f "$test_stderr" ]; then
        echo ""
        echo_step "📊 Benchmark性能分析结果"
        
        # 检查是否有benchmark数据
        local has_benchmark_data=$(grep -c "\[hotstuff info\]" "$test_stderr" 2>/dev/null || echo 0)
        if [ $has_benchmark_data -gt 0 ]; then
            echo_info "✅ 检测到 $has_benchmark_data 条benchmark数据"
            
            echo ""
            echo "标准格式输出:"
            cat "$test_stderr" | python3 "$SCRIPT_DIR/performance_analyzer.py"
            
            echo ""
            echo_step "传统统计分析"
            echo "直方图分析:"
            cat "$test_stderr" | python3 "$SCRIPT_DIR/thr_hist.py"
        else
            echo_warn "⚠️ 未检测到benchmark性能数据"
            echo_info "请确保使用-DHOTSTUFF_ENABLE_BENCHMARK编译选项"
            echo "原始日志预览:"
            head -10 "$test_stderr"
        fi
        
        # 显示测试统计信息
        echo ""
        echo_step "📈 测试统计信息"
        local successful_commands=$(grep -c "got.*cmd" "$test_stderr" 2>/dev/null || echo 0)
        echo "成功处理命令数: $successful_commands"
        echo "预期命令数: $TEST_ITERATIONS"
        echo "成功率: $((successful_commands * 100 / TEST_ITERATIONS))%"
    fi
    
    return $exit_code
}

# 显示结果摘要
show_summary() {
    echo ""
    echo "================================"
    echo "  测试结果摘要"
    echo "================================"
    echo "配置: $NUM_NODES 节点, $TEST_ITERATIONS 迭代, $MAX_ASYNC 并发"
    echo "日志位置: $LIBHOTSTUFF_DIR/logs/"
    echo "查看节点日志: tail -f $LIBHOTSTUFF_DIR/logs/node*.log"
    echo "================================"
}

# 主函数
main() {
    echo ""
    echo "================================"
    echo "  HotStuff 集群压力测试"
    echo "================================"
    echo "节点数: $NUM_NODES"
    echo "迭代数: $TEST_ITERATIONS"
    echo "并发数: $MAX_ASYNC"
    echo "超时时间: ${MAX_DURATION}秒"
    echo "详细模式: $([ "$VERBOSE" = true ] && echo "开启" || echo "关闭")"
    echo "清理模式: $([ "$CLEAN_START" = true ] && echo "开启" || echo "关闭")"
    echo "工作目录: $LIBHOTSTUFF_DIR"
    echo ""
    
    # 如果启用清理模式，先清理旧进程
    if [ "$CLEAN_START" = true ]; then
        echo_step "执行清理启动模式..."
        cleanup
        sleep 2
    fi
    
    # 设置清理陷阱
    trap cleanup EXIT INT TERM
    
    # 执行测试流程
    check_ports
    check_dependencies
    start_nodes
    wait_for_stable_cluster
    run_test
    show_summary
    
    echo ""
    echo_info "🎉 压力测试完成！"
    echo_info "如需重新运行，请先执行: pkill -f hotstuff-app"
    echo_info "查看详细日志: tail -f $LIBHOTSTUFF_DIR/logs/node*.log"
}

# 运行主函数
main "$@"