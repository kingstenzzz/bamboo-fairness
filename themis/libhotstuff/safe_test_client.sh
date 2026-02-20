#!/bin/bash

# 安全的HotStuff客户端测试脚本
# 避免Bus error并提供详细的错误诊断

set -euo pipefail

echo "🔍 HotStuff客户端安全测试"
echo "========================"

# 检查基本依赖
check_dependencies() {
    echo "📋 检查依赖..."
    
    # 检查可执行文件
    if [ ! -f "./examples/hotstuff-app" ]; then
        echo "❌ 错误: 找不到 hotstuff-app 可执行文件"
        echo "请确保在正确的目录下运行此脚本"
        exit 1
    fi
    
    if [ ! -f "./examples/hotstuff-client" ]; then
        echo "❌ 错误: 找不到 hotstuff-client 可执行文件"
        echo "请确保在正确的目录下运行此脚本"
        exit 1
    fi
    
    # 检查配置文件
    for i in {0..3}; do
        if [ ! -f "./hotstuff-sec${i}.conf" ]; then
            echo "❌ 错误: 缺少配置文件 hotstuff-sec${i}.conf"
            exit 1
        fi
    done
    
    echo "✅ 依赖检查通过"
}

# 内存诊断函数
diagnose_memory_issues() {
    echo "🧠 内存问题诊断..."
    
    # 检查系统架构
    local arch=$(uname -m)
    echo "系统架构: $arch"
    
    # 检查可执行文件架构
    local app_arch=$(file ./examples/hotstuff-app | cut -d' ' -f3)
    local client_arch=$(file ./examples/hotstuff-client | cut -d' ' -f3)
    echo "hotstuff-app 架构: $app_arch"
    echo "hotstuff-client 架构: $client_arch"
    
    # 检查内存使用情况
    echo "当前内存使用:"
    vm_stat | head -5
    
    # 检查ulimit设置
    echo "文件描述符限制: $(ulimit -n)"
    echo "栈大小限制: $(ulimit -s)"
}

# 安全启动节点
safe_start_nodes() {
    echo "🚀 安全启动节点..."
    
    # 先清理可能存在的进程
    pkill -f "hotstuff-app" 2>/dev/null || true
    sleep 2
    
    # 逐个启动节点，增加延迟避免竞争
    for i in {0..3}; do
        echo "启动节点 $i..."
        ./examples/hotstuff-app --conf ./hotstuff-sec${i}.conf > logs/node${i}.log 2>&1 &
        local pid=$!
        echo "节点 $i PID: $pid"
        sleep 3  # 给每个节点更多启动时间
        
        # 检查进程是否还在运行
        if ! kill -0 $pid 2>/dev/null; then
            echo "❌ 节点 $i 启动失败"
            cat logs/node${i}.log | tail -10
            return 1
        fi
    done
    
    echo "✅ 所有节点启动完成"
    sleep 5  # 额外等待集群稳定
}

# 安全的客户端测试
safe_client_test() {
    echo "🧪 安全客户端测试..."
    
    # 先测试简单的帮助命令
    echo "测试基本功能..."
    if ! ./examples/hotstuff-client --help >/dev/null 2>&1; then
        echo "❌ 客户端基本功能测试失败"
        return 1
    fi
    echo "✅ 基本功能正常"
    
    # 小规模测试
    echo "执行小规模测试..."
    local small_test_log="logs/small_test_$(date +%Y%m%d_%H%M%S).log"
    
    # 使用非常保守的参数
    if timeout 30s ./examples/hotstuff-client --idx 0 --iter 10 --max-async 2 >"$small_test_log" 2>&1; then
        echo "✅ 小规模测试成功"
        echo "测试日志: $small_test_log"
        cat "$small_test_log"
        return 0
    else
        local exit_code=$?
        echo "❌ 小规模测试失败 (退出码: $exit_code)"
        echo "详细日志:"
        cat "$small_test_log"
        return $exit_code
    fi
}

# 主函数
main() {
    echo "开始安全测试流程..."
    
    # 检查依赖
    check_dependencies
    
    # 内存诊断
    diagnose_memory_issues
    
    # 安全启动节点
    if ! safe_start_nodes; then
        echo "❌ 节点启动阶段失败"
        exit 1
    fi
    
    # 安全客户端测试
    if safe_client_test; then
        echo "🎉 安全测试完成！"
        echo "✅ 没有出现Bus error"
    else
        echo "❌ 测试失败"
        echo "建议检查:"
        echo "1. 重新编译程序: make clean && make"
        echo "2. 检查系统内存是否充足"
        echo "3. 查看详细日志文件"
    fi
    
    # 清理
    pkill -f "hotstuff-app" 2>/dev/null || true
}

# 运行主函数
main "$@"