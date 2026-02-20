#!/usr/bin/env bash
# 简化版HotStuff测试脚本

set -euo pipefail

cd /Users/kingsten/code/themis-src-anon-main/Aequitas-hotstuff/libhotstuff

echo "=== HotStuff 简化测试 ==="

# 清理
pkill -f "hotstuff-app" 2>/dev/null || true
pkill -f "hotstuff-client" 2>/dev/null || true
sleep 2

# 检查文件
if [ ! -f "examples/hotstuff-app" ] || [ ! -f "examples/hotstuff-client" ]; then
    echo "❌ 缺少可执行文件"
    exit 1
fi

# 检查配置
for i in {0..3}; do
    if [ ! -f "hotstuff-sec${i}.conf" ]; then
        echo "❌ 缺少配置文件 hotstuff-sec${i}.conf"
        exit 1
    fi
done

echo "✅ 文件检查通过"

# 启动单个节点测试
echo "→ 启动节点 0 进行测试..."
./examples/hotstuff-app --conf ./hotstuff-sec0.conf > logs/test_node0.log 2>&1 &

NODE_PID=$!

echo "节点PID: $NODE_PID"
sleep 5

# 检查是否还在运行
if kill -0 $NODE_PID 2>/dev/null; then
    echo "✅ 节点运行正常"
    echo "查看日志: tail -f logs/test_node0.log"
else
    echo "❌ 节点启动失败"
    echo "错误日志:"
    cat logs/test_node0.log
fi

# 清理
pkill -f "hotstuff-app" 2>/dev/null || true