#!/bin/bash
# 模拟标准部署测试流程的本地版本

set -e
echo_info() { echo -e "${GREEN}[INFO]${NC} $1"; }
echo_warn() { echo -e "${YELLOW}[WARN]${NC} $1"; }
echo_error() { echo -e "${RED}[ERROR]${NC} $1"; }
echo_step() { echo -e "${BLUE}[STEP]${NC} $1"; }
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
LIBHOTSTUFF_DIR="$SCRIPT_DIR/.."

echo "=== HotStuff 标准性能测试模拟 ==="
echo ""

# 1. 检查环境
echo "步骤1: 检查测试环境"
if [ ! -f "$LIBHOTSTUFF_DIR/examples/hotstuff-app" ]; then
    echo "❌ hotstuff-app 未找到，请先编译"
    exit 1
fi

if [ ! -f "$LIBHOTSTUFF_DIR/examples/hotstuff-client" ]; then
    echo "❌ hotstuff-client 未找到，请先编译"
    exit 1
fi

echo "✅ 可执行文件检查通过"

# 2. 准备配置（模拟hotstuff.gen.conf和clients.yml的配置）
echo ""
echo "步骤2: 准备测试配置"
TEST_ITERATIONS=1000
MAX_ASYNC=100
NUM_NODES=4

echo "配置参数:"
echo "  迭代次数: $TEST_ITERATIONS"
echo "  最大并发: $MAX_ASYNC"
echo "  节点数量: $NUM_NODES"

# 3. 启动Replica进程（模拟 ./run.sh new myrun1）
echo ""
echo "步骤3: 启动Replica集群"
cd "$LIBHOTSTUFF_DIR"
mkdir -p logs

# 清理旧进程
pkill -f "hotstuff-app" 2>/dev/null || true
sleep 2

# 启动节点
for i in $(seq 0 $((NUM_NODES-1))); do
    echo "启动节点 $i"
          echo_info "启动节点 $i (端口: $((10000+i)), $((20000+i)))"
          echo_warn "🚨 注意：macOS防火墙可能会弹窗，请准备好点击'允许'"
         ./examples/hotstuff-app --conf ./hotstuff-sec${i}.conf > logs/node${i}.log 2>&1 &
        pids+=($!)
        echo_info "等待5秒后启动下一个节点..."
    sleep 1
done

echo "等待集群稳定 (10秒)..."
sleep 10

# 4. 验证节点运行状态
echo ""
echo "步骤4: 验证集群状态"
running_nodes=0
for i in $(seq 0 $((NUM_NODES-1))); do
    if pgrep -f "hotstuff-app.*hotstuff-sec${i}.conf" > /dev/null; then
        echo "✅ 节点 $i 运行中"
        ((running_nodes++))
    else
        echo "❌ 节点 $i 启动失败"
    fi
done

if [ $running_nodes -ne $NUM_NODES ]; then
    echo "❌ 集群启动不完整 ($running_nodes/$NUM_NODES)"
    exit 1
fi

# 5. 运行客户端测试（模拟 ./run_cli.sh new myrun1_cli）
echo ""
echo "步骤5: 执行性能测试"
echo "发送 $TEST_ITERATIONS 条命令，最大并发 $MAX_ASYNC"

# 启用benchmark模式并执行测试
export HOTSTUFF_ENABLE_BENCHMARK=1
TEST_OUTPUT_FILE="logs/client_test_$(date +%Y%m%d_%H%M%S).log"

echo "测试开始时间: $(date)"
./examples/hotstuff-client --idx 0 --iter $TEST_ITERATIONS --max-async $MAX_ASYNC 2>"$TEST_OUTPUT_FILE"
TEST_EXIT_CODE=$?

echo "测试结束时间: $(date)"
echo "测试退出码: $TEST_EXIT_CODE"

# 6. 分析结果（模拟 cat myrun1_cli/remote/*/log/stderr | python ../thr_hist.py）
echo ""
echo "步骤6: 分析性能结果"

if [ -f "$TEST_OUTPUT_FILE" ]; then
    echo "原始测试输出行数: $(wc -l < "$TEST_OUTPUT_FILE")"
    echo ""
    
    # 使用thr_hist.py分析
    echo "=== thr_hist.py 分析结果 ==="
    python3 "$SCRIPT_DIR/thr_hist.py" < "$TEST_OUTPUT_FILE"
    
    echo ""
    echo "=== 详细统计信息 ==="
    echo "成功处理的命令数: $(grep -c "got.*cmd" "$TEST_OUTPUT_FILE" 2>/dev/null || echo 0)"
    echo "平均延迟: $(awk '/lat =/ {sum+=$3; count++} END {if(count>0) printf "%.3fms\n", sum/count}' "$TEST_OUTPUT_FILE" 2>/dev/null || echo "N/A")"
else
    echo "❌ 未找到测试输出文件"
fi

# 7. 清理（模拟 ./run.sh stop myrun1）
echo ""
echo "步骤7: 清理测试环境"
pkill -f "hotstuff-app" 2>/dev/null || true
pkill -f "hotstuff-client" 2>/dev/null || true

echo ""
echo "=== 测试完成 ==="
echo "测试日志位置: $TEST_OUTPUT_FILE"
echo "节点日志位置: $LIBHOTSTUFF_DIR/logs/"

if [ $TEST_EXIT_CODE -eq 0 ]; then
    echo "🎉 性能测试成功完成"
else
    echo "⚠️  测试过程中出现错误"
fi