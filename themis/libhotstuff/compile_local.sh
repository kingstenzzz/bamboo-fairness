#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
EXAMPLES="$ROOT/examples"
HOMEBREW_PREFIX="/opt/homebrew"

cd "$ROOT"

# 1) Build core static libs via CMake/Make
make hotstuff_static salticidae_static libsecp256k1

# 2) Compile example objects with explicit include paths and benchmark support (with debug log)
c++ -std=c++14 -DHOTSTUFF_ENABLE_BENCHMARK -DHOTSTUFF_DEBUG_LOG -I"$ROOT/include" \
  -I"$ROOT/salticidae/include" \
  -I"$HOMEBREW_PREFIX/include" \
  -I"$ROOT/secp256k1/include" \
  -c "$EXAMPLES/hotstuff_app.cpp" -o "$EXAMPLES/hotstuff_app.o"

c++ -std=c++14 -DHOTSTUFF_ENABLE_BENCHMARK -DHOTSTUFF_DEBUG_LOG -I"$ROOT/include" \
  -I"$ROOT/salticidae/include" \
  -I"$HOMEBREW_PREFIX/include" \
  -I"$ROOT/secp256k1/include" \
  -c "$EXAMPLES/hotstuff_client.cpp" -o "$EXAMPLES/hotstuff_client.o"

# 3) Link executables with explicit lib paths
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

echo "[OK] Built: $EXAMPLES/hotstuff-app"
echo "[OK] Built: $EXAMPLES/hotstuff-client"