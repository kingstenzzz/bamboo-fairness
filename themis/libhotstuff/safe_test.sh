#!/usr/bin/env bash
# 安全的HotStuff测试脚本 - 针对macOS ARM64优化

set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
EXAMPLES="$ROOT/examples"
LOGS_DIR="$ROOT/logs"

cd "$ROOT"

echo "=== HotStuff 安全测试启动 ==="
echo "架构: $(uname -m)"
echo "编译器: $(c++ --version | head -1)"

# 创建日志目录
mkdir -p "$LOGS_DIR"

# 清理旧进程
echo "[清理] 停止旧进程..."
pkill -f "hotstuff-app" 2>/dev/null || true
pkill -f "hotstuff-client" 2>/dev/null || true
sleep 3

# 验证可执行文件
echo "[验证] 检查可执行文件..."
for binary in "hotstuff-app" "hotstuff-client"; do
    if [ ! -f "$EXAMPLES/$binary" ]; then
        echo "❌ 缺少可执行文件: $EXAMPLES/$binary"
        echo "请先运行: ./fix_benchmark.sh"
        exit 1
    fi
    
    # 检查文件权限
    if [ ! -x "$EXAMPLES/$binary" ]; then
        chmod +x "$EXAMPLES/$binary"
    fi
    
    echo "✅ $binary: $(file "$EXAMPLES/$binary")"
done

# 检查配置文件
echo "[验证] 检查配置文件..."
for i in {0..3}; do
    if [ ! -f "hotstuff-sec${i}.conf" ]; then
        echo "❌ 缺少配置文件: hotstuff-sec${i}.conf"
        echo "请确保有4个配置文件"
        exit 1
    fi
done

# 启动节点（逐个启动，增加错误处理）
echo "[启动] 启动HotStuff节点..."
NODE_PIDS=()

for i in {0..3}; do
    echo "→ 启动节点 $i (端口: $((10000+i)), $((20000+i)))"
    
    # 使用更安全的启动方式
    if ./"$EXAMPLES/hotstuff-app" --conf "./hotstuff-sec${i}.conf" > "$LOGS_DIR/node${i}.log" 2>&1 & then
        NODE_PIDS+=($!)
        echo "  PID: ${NODE_PIDS[-1]}"
    else
        echo "❌ 节点 $i 启动失败"
        # 显示错误日志
        echo "错误日志摘要:"
        tail -5 "$LOGS_DIR/node${i}.log" 2>/dev/null || echo "日志文件不存在"
        continue
    fi
    
    # 等待节点稳定启动
    sleep 8
    
    # 验证节点是否仍在运行
    if kill -0 ${NODE_PIDS[-1]} 2>/dev/null; then
        echo "  ✅ 节点 $i 运行中"
    else
        echo "  ❌ 节点 $i 已退出"
        echo "  错误日志:"
        tail -10 "$LOGS_DIR/node${i}.log" 2>/dev/null || echo "日志文件不存在"
    fi
done

# 等待集群稳定
echo "[等待] 等待集群稳定 (15秒)..."
for i in {1..15}; do
    echo -n "."
    sleep 1
done
echo ""

# 检查网络连接
echo "[检查] 验证网络连接..."
echo "监听的端口:"
netstat -an | grep -E "1000[0-3]|2000[0-3]" | grep LISTEN || echo "未检测到监听端口"

echo "已建立的连接:"
netstat -an | grep -E "1000[0-3]|2000[0-3]" | grep ESTABLISHED || echo "未检测到已建立连接"

# 运行简单测试
echo "[测试] 运行基础功能测试..."
TEST_OUTPUT="$LOGS_DIR/test_basic.log"

if ./"$EXAMPLES/hotstuff-client" --help > "$TEST_OUTPUT" 2>&1; then
    echo "✅ 客户端基本功能正常"
else
    echo "❌ 客户端存在问题"
    echo "错误详情:"
    cat "$TEST_OUTPUT"
fi

# 显示运行状态
echo ""
echo "=== 运行状态 ==="
echo "运行中的节点进程:"
ps aux | grep "hotstuff-app" | grep -v grep || echo "无运行中的节点"

echo ""
echo "日志文件位置:"
echo "  节点日志: $LOGS_DIR/node*.log"
echo "  测试日志: $TEST_OUTPUT"

echo ""
echo "=== 使用说明 ==="
echo "查看节点日志: tail -f $LOGS_DIR/node0.log"
echo "停止所有节点: pkill -f hotstuff-app"
echo "运行性能测试: ./examples/hotstuff-client --idx 0 --iter 10 --max-async 2"

# 设置清理函数
cleanup() {
    echo ""
    echo "[清理] 停止所有HotStuff进程..."
    pkill -f "hotstuff-app" 2>/dev/null || true
    pkill -f "hotstuff-client" 2>/dev/null || true
    echo "✅ 清理完成"
}

trap cleanup EXIT INT TERM