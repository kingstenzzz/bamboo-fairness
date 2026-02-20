# HotStuff Benchmark测试使用说明

## 概述
这个脚本整合了完整的HotStuff性能测试功能，包括benchmark模式支持和多种分析工具。

## 主要特性
- ✅ 自动启用benchmark模式
- ✅ 多种性能分析工具集成
- ✅ 详细的测试统计信息
- ✅ 自动化的集群部署和清理
- ✅ macOS兼容性优化

## 使用方法

### 基本使用
```bash
# 进入脚本目录
cd /Users/kingsten/code/themis-src-anon-main/Aequitas-hotstuff/libhotstuff/scripts

# 执行标准测试（4节点，1000迭代，100并发）
./test_cluster_improved.sh

# 自定义参数
./test_cluster_improved.sh 6 2000 150
```

### 编译要求
确保使用benchmark标志编译：
```bash
cd /Users/kingsten/code/themis-src-anon-main/Aequitas-hotstuff/libhotstuff
make clean
make CXXFLAGS="-DHOTSTUFF_ENABLE_BENCHMARK"
```

## 输出示例

### 成功执行的输出
```
================================
  HotStuff 集群压力测试
================================
节点数: 4
迭代数: 1000
并发数: 100

[STEP] 检查依赖...
[INFO] ✓ 依赖检查通过
[STEP] 检查benchmark支持...
[INFO] ✓ 检测到benchmark支持已启用

[STEP] 启动 4 个节点...
[INFO] 启动节点 0 (端口: 10000, 20000)
[WARN] 🚨 注意：macOS防火墙可能会弹窗，请准备好点击'允许'
[INFO] 等待5秒后启动下一个节点...
...

[STEP] 📊 Benchmark性能分析结果
[INFO] ✅ 检测到 1000 条benchmark数据

标准格式输出:
[349669, 367520, 371855, 370391, 366159, 367565, 365957, 322690] lat = 6.955ms # mean end-to-end latency
after removing outliers [349669, 367520, 371855, 370391, 366159, 367565, 365957, 322690] lat = 6.970ms # mean end-to-end latency
tps = 1250.50 # average transactions per second

[STEP] 📈 测试统计信息
成功处理命令数: 1000
预期命令数: 1000
成功率: 100%
```

## 性能指标说明

### 延迟指标
- **Mean Latency**: 平均端到端延迟
- **Median Latency**: 中位数延迟
- **95th Percentile**: 95%请求的延迟
- **99th Percentile**: 99%请求的延迟

### 吞吐量指标
- **TPS**: 每秒事务数
- **Peak TPS**: 瞬时峰值吞吐量
- **Average TPS**: 平均吞吐量

### 成功率指标
- **命令成功率**: 实际完成 vs 预期命令数的比例

## 故障排除

### 常见问题
1. **缺少benchmark支持**
   ```
   ⚠️ 未检测到benchmark支持
   建议重新编译: cd libhotstuff && make clean && make CXXFLAGS='-DHOTSTUFF_ENABLE_BENCHMARK'
   ```

2. **端口被占用**
   ```bash
   # 检查端口占用
   lsof -i :10000
   lsof -i :20000
   
   # 清理进程
   pkill -f hotstuff-app
   ```

3. **防火墙弹窗**
   - 首次运行时点击"允许"
   - 或预先添加防火墙例外

### 日志文件位置
- 节点日志: `libhotstuff/logs/node*.log`
- 测试日志: `libhotstuff/logs/test_stderr_*.log`

## 高级用法

### 参数调优建议
```bash
# 小规模测试
./test_cluster_improved.sh 4 500 50

# 标准测试  
./test_cluster_improved.sh 4 1000 100

# 高负载测试
./test_cluster_improved.sh 4 5000 200
```

### 环境变量设置
```bash
# 设置更大的ulimit（如果需要）
ulimit -n 65536

# 设置更长的超时时间
export HOTSTUFF_CLIENT_TIMEOUT=30
```

这个整合版本提供了完整的HotStuff性能测试解决方案，从集群部署到结果分析的一站式体验。