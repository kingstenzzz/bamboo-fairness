#!/usr/bin/env bash
# HotStuff Benchmark验证和修复脚本

set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
EXAMPLES="$ROOT/examples"

cd "$ROOT"

echo "=== HotStuff Benchmark 验证和修复 ==="

# 1. 检查当前状态
echo "[检查] 当前benchmark支持状态..."
if [ -f "$EXAMPLES/hotstuff-client" ]; then
    echo "可执行文件大小: $(stat -f "%z" "$EXAMPLES/hotstuff-client" 2>/dev/null || stat -c "%s" "$EXAMPLES/hotstuff-client" 2>/dev/null || echo "unknown") 字节"
    
    if strings "$EXAMPLES/hotstuff-client" | grep -q "HOTSTUFF_ENABLE_BENCHMARK"; then
        echo "✅ Benchmark支持已启用"
        exit 0
    else
        echo "❌ Benchmark支持未启用"
    fi
else
    echo "❌ hotstuff-client 可执行文件不存在"
fi

# 2. 执行修复编译
echo ""
echo "[修复] 重新编译启用benchmark支持..."

# 清理
echo "→ 清理旧构建..."
make clean 2>/dev/null || true

# 构建核心库
echo "→ 构建核心库..."
make hotstuff_static salticidae_static libsecp256k1

# 编译带benchmark支持
echo "→ 编译示例程序..."
export CXXFLAGS="-DHOTSTUFF_ENABLE_BENCHMARK"

c++ -std=c++14 $CXXFLAGS \
  -I"$ROOT/include" \
  -I"$ROOT/salticidae/include" \
  -I"/opt/homebrew/include" \
  -I"$ROOT/secp256k1/include" \
  -c "$EXAMPLES/hotstuff_app.cpp" -o "$EXAMPLES/hotstuff_app.o"

c++ -std=c++14 $CXXFLAGS \
  -I"$ROOT/include" \
  -I"$ROOT/salticidae/include" \
  -I"/opt/homebrew/include" \
  -I"$ROOT/secp256k1/include" \
  -c "$EXAMPLES/hotstuff_client.cpp" -o "$EXAMPLES/hotstuff_client.o"

# 链接
echo "→ 链接可执行文件..."
c++ -std=c++14 -o "$EXAMPLES/hotstuff-app" \
  "$EXAMPLES/hotstuff_app.o" \
  "$ROOT/libhotstuff.a" \
  "$ROOT/salticidae/libsalticidae.a" \
  "$ROOT/secp256k1/.libs/libsecp256k1.a" \
  -L"/opt/homebrew/lib" -luv -lssl -lcrypto

c++ -std=c++14 -o "$EXAMPLES/hotstuff-client" \
  "$EXAMPLES/hotstuff_client.o" \
  "$ROOT/libhotstuff.a" \
  "$ROOT/salticidae/libsalticidae.a" \
  "$ROOT/secp256k1/.libs/libsecp256k1.a" \
  -L"/opt/homebrew/lib" -luv -lssl -lcrypto

# 3. 验证结果
echo ""
echo "[验证] 检查编译结果..."
if [ -f "$EXAMPLES/hotstuff-client" ]; then
    echo "新可执行文件大小: $(stat -f "%z" "$EXAMPLES/hotstuff-client" 2>/dev/null || stat -c "%s" "$EXAMPLES/hotstuff-client" 2>/dev/null || echo "unknown") 字节"
    
    if strings "$EXAMPLES/hotstuff-client" | grep -q "HOTSTUFF_ENABLE_BENCHMARK"; then
        echo "✅ SUCCESS: Benchmark支持已成功启用！"
        echo ""
        echo "现在可以运行性能测试："
        echo "  ./scripts/test_cluster_improved.sh -n 4 -i 100 -a 20 --benchmark"
        echo ""
        echo "或者手动测试："
        echo "  ./examples/hotstuff-client --idx 0 --iter 50 --max-async 10 2> test_output.log"
        echo "  cat test_output.log | python3 scripts/thr_hist.py"
    else
        echo "❌ FAILED: Benchmark支持仍未启用"
        echo "请检查编译过程中是否有错误信息"
    fi
else
    echo "❌ 编译失败：可执行文件未生成"
fi