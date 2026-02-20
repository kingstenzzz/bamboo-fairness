#!/usr/bin/env bash
# 标准HotStuff性能测试编译脚本
# 使用CXXFLAGS方式启用benchmark支持

set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
EXAMPLES="$ROOT/examples"
HOMEBREW_PREFIX="/opt/homebrew"

cd "$ROOT"

echo "=== HotStuff 性能测试编译 ==="
echo "启用benchmark支持: CXXFLAGS='-DHOTSTUFF_ENABLE_BENCHMARK'"

# 1) 清理之前的构建
echo "[STEP] 清理旧构建..."
make clean 2>/dev/null || true

# 2) 构建核心静态库
echo "[STEP] 构建核心库..."
make hotstuff_static salticidae_static libsecp256k1

# 3) 使用CXXFLAGS编译示例程序
echo "[STEP] 编译示例程序（启用benchmark）..."
export CXXFLAGS="-DHOTSTUFF_ENABLE_BENCHMARK"

# 编译hotstuff_app
c++ -std=c++14 $CXXFLAGS \
  -I"$ROOT/include" \
  -I"$ROOT/salticidae/include" \
  -I"$HOMEBREW_PREFIX/include" \
  -I"$ROOT/secp256k1/include" \
  -c "$EXAMPLES/hotstuff_app.cpp" -o "$EXAMPLES/hotstuff_app.o"

# 编译hotstuff_client  
c++ -std=c++14 $CXXFLAGS \
  -I"$ROOT/include" \
  -I"$ROOT/salticidae/include" \
  -I"$HOMEBREW_PREFIX/include" \
  -I"$ROOT/secp256k1/include" \
  -c "$EXAMPLES/hotstuff_client.cpp" -o "$EXAMPLES/hotstuff_client.o"

# 4) 链接可执行文件
echo "[STEP] 链接可执行文件..."
c++ -std=c++14 -o "$EXAMPLES/hotstuff-app" \
  "$EXAMPLES/hotstuff_app.o" \
  "$ROOT/libhotstuff.a" \
  "$ROOT/salticidae/libsalticidae.a" \
  "$ROOT/secp256k1/.libs/libsecp256k1.a" \
  -L"$HOMEBREW_PREFIX/lib" -luv -lssl -lcrypto

c++ -std=c++14 -o "$EXAMPLES/hotstuff-client" \
  "$EXAMPLES/hotstuff_client.o" \
  "$ROOT/libhotstuff.a" \
  "$ROOT/salticidae/libsalticidae.a" \
  "$ROOT/secp256k1/.libs/libsecp256k1.a" \
  -L"$HOMEBREW_PREFIX/lib" -luv -lssl -lcrypto

# 5) 验证benchmark支持
echo "[STEP] 验证benchmark支持..."
if strings "$EXAMPLES/hotstuff-client" | grep -q "HOTSTUFF_ENABLE_BENCHMARK"; then
    echo "✅ Benchmark支持已成功启用"
else
    echo "❌ Benchmark支持未正确启用"
    echo "建议检查编译过程是否有错误"
fi

echo ""
echo "[OK] 构建完成!"
echo "可执行文件位置:"
echo "  $EXAMPLES/hotstuff-app"
echo "  $EXAMPLES/hotstuff-client"
echo ""
echo "现在可以运行性能测试:"
echo "  ./scripts/test_cluster_improved.sh --benchmark"