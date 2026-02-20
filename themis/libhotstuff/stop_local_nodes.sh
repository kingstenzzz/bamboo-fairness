#!/bin/bash

# 停止所有本地 HotStuff 节点
# 删除编译部分后的版本

GREEN='\033[0;32m'
NC='\033[0m'

echo_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

echo ""
echo "停止所有 HotStuff 节点..."
echo ""

# 查找并停止所有 hotstuff 进程
PIDS=$(pgrep -f "hotstuff --conf" || true)

if [ -z "$PIDS" ]; then
    echo_info "没有运行中的 HotStuff 节点"
else
    echo "找到以下进程:"
    ps aux | grep "[h]otstuff --conf"
    echo ""
    
    pkill -f "hotstuff --conf"
    sleep 1
    
    # 检查是否还有残留进程
    if pgrep -f "hotstuff --conf" > /dev/null; then
        echo_info "强制停止残留进程..."
        pkill -9 -f "hotstuff --conf"
    fi
    
    echo_info "✓ 所有 HotStuff 节点已停止"
fi

echo ""
