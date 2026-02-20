#!/bin/bash

# HotStuff配置和启动诊断脚本

echo "🔍 HotStuff配置诊断"
echo "=================="

# 检查配置文件
echo "📋 配置文件检查:"
for i in {0..3}; do
    echo "检查 hotstuff-sec${i}.conf:"
    if [ -f "hotstuff-sec${i}.conf" ]; then
        echo "  ✓ 文件存在"
        echo "  内容预览:"
        head -5 "hotstuff-sec${i}.conf" | sed 's/^/    /'
        echo "  replica行检查:"
        if grep -q "^replica =" "hotstuff-sec${i}.conf"; then
            echo "    ✓ 包含replica配置"
            grep "^replica =" "hotstuff-sec${i}.conf" | sed 's/^/      /'
        else
            echo "    ❌ 缺少replica配置"
        fi
        echo ""
    else
        echo "  ❌ 文件不存在"
    fi
done

# 检查可执行文件
echo "🔧 可执行文件检查:"
if [ -f "./examples/hotstuff-app" ]; then
    echo "  ✓ hotstuff-app 存在"
    file ./examples/hotstuff-app
else
    echo "  ❌ hotstuff-app 不存在"
fi

if [ -f "./examples/hotstuff-client" ]; then
    echo "  ✓ hotstuff-client 存在"
    file ./examples/hotstuff-client
else
    echo "  ❌ hotstuff-client 不存在"
fi

# 检查端口占用
echo ""
echo "🔌 端口占用检查:"
for port in 10000 10001 10002 10003 20000; do
    if lsof -Pi :$port -sTCP:LISTEN -t >/dev/null 2>&1; then
        echo "  端口 $port: 被占用"
        lsof -Pi :$port -sTCP:LISTEN | tail -1 | awk '{print "    " $1 " (" $2 ")"}'
    else
        echo "  端口 $port: 可用"
    fi
done

# 尝试手动启动一个节点进行测试
echo ""
echo "🧪 单节点启动测试:"
echo "启动节点0进行测试..."

# 清理可能存在的进程
pkill -f "hotstuff-app" 2>/dev/null || true
sleep 2

# 启动单个节点
timeout 10s ./examples/hotstuff-app --conf ./hotstuff-sec0.conf > logs/diag_node0.log 2>&1 &
NODE_PID=$!

echo "节点0 PID: $NODE_PID"
sleep 3

# 检查进程状态
if kill -0 $NODE_PID 2>/dev/null; then
    echo "✓ 节点0仍在运行"
    echo "最近日志:"
    tail -5 logs/diag_node0.log | sed 's/^/  /'
else
    echo "❌ 节点0已退出"
    echo "完整日志:"
    cat logs/diag_node0.log | sed 's/^/  /'
fi

# 清理
kill $NODE_PID 2>/dev/null || true
pkill -f "hotstuff-app" 2>/dev/null || true

echo ""
echo "诊断完成"