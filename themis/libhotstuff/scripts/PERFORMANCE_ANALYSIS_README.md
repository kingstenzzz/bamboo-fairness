# HotStuff 性能测试和分析工具

## 概述

这套工具提供了完整的HotStuff共识算法性能测试和分析解决方案，包括：

1. **自动化测试脚本** - 一键启动集群并执行压力测试
2. **性能分析工具** - 计算延迟、吞吐量等关键指标
3. **标准输出格式** - 符合业界标准的性能报告格式

## 工具组成

### 1. test_cluster_improved.sh
主要的自动化测试脚本，功能包括：
- 自动启动指定数量的HotStuff节点
- 执行压力测试
- 集成性能分析
- 详细的执行状态反馈

### 2. performance_analyzer.py
专业的性能分析工具，提供：
- 延迟统计（平均值、中位数、百分位数）
- 吞吐量计算（TPS）
- 异常值检测和过滤
- 标准输出格式

### 3. thr_hist.py
传统的直方图分析工具

## 使用方法

### 基本测试
```bash
# 进入脚本目录
cd /Users/kingsten/code/themis-src-anon-main/Aequitas-hotstuff/libhotstuff/scripts

# 执行标准测试（4节点，1000迭代，100并发）
./test_cluster_improved.sh

# 自定义参数测试
./test_cluster_improved.sh 6 2000 150
```

### 单独使用分析工具
```bash
# 分析现有日志文件
python3 performance_analyzer.py --input your_log_file.txt

# 从stdin读取并分析
cat log_file.txt | python3 performance_analyzer.py

# 详细输出模式
python3 performance_analyzer.py --input log.txt --verbose
```

## 输出格式说明

### 标准延迟输出
```
[349669, 367520, 371855, 370391, 366159, 367565, 365957, 322690] lat = 6.955ms # mean end-to-end latency
[349669, 367520, 371855, 370391, 366159, 367565, 365957, 322690] lat = 6.970ms # after removing outliers
```

### 吞吐量统计
```
tps = 1250.50 # average transactions per second
max_tps = 1350 # peak transactions per second
min_tps = 1100 # minimum transactions per second
```

### 百分位数统计
```
95th_percentile = 7.234ms
99th_percentile = 8.123ms
```

## 性能指标解释

### 延迟 (Latency)
- **Mean Latency**: 平均端到端延迟
- **Median Latency**: 中位数延迟（更能反映典型性能）
- **95th Percentile**: 95%请求的延迟不超过此值
- **99th Percentile**: 99%请求的延迟不超过此值

### 吞吐量 (Throughput)
- **TPS (Transactions Per Second)**: 每秒处理的交易数
- **Average TPS**: 测试期间的平均吞吐量
- **Peak TPS**: 瞬时最高吞吐量
- **Min TPS**: 最低吞吐量

### 异常值处理
使用四分位距(IQR)方法识别和移除异常值，确保统计结果的准确性。

## 时间估算

完整的测试流程大约需要：
- **节点启动**: 15-20秒
- **集群稳定**: 10-15秒  
- **压力测试**: 根据参数变化（500迭代≈5-10秒）
- **结果分析**: 2-3秒
- **总计**: 35-50秒（典型配置）

## 故障排除

### 常见问题
1. **端口占用**: 使用`lsof -i :10000`检查端口
2. **防火墙弹窗**: 首次运行时点击"允许"
3. **节点启动失败**: 检查日志文件`logs/node*.log`
4. **性能数据为空**: 确保测试成功完成且有足够样本

### 日志位置
- 节点日志: `libhotstuff/logs/node*.log`
- 测试stderr: `libhotstuff/logs/test_stderr_*.log`

## 最佳实践

1. **预热**: 首次测试建议使用较小参数
2. **多次测试**: 进行多次测试取平均值
3. **环境稳定**: 确保测试期间系统负载稳定
4. **参数调优**: 根据硬件配置调整并发数和迭代次数

## 示例输出
```
================================
  HotStuff 集群压力测试
================================
节点数: 4
迭代数: 1000
并发数: 100

[STEP] 检查依赖...
[INFO] ✓ 依赖检查通过

[STEP] 启动 4 个节点...
[INFO] 启动节点 0 (端口: 10000, 20000)
[WARN] 🚨 注意：macOS防火墙可能会弹窗，请准备好点击'允许'
[INFO] 等待5秒后启动下一个节点...
...

[STEP] 性能分析结果
标准格式输出:
[349669, 367520, 371855, 370391, 366159, 367565, 365957, 322690] lat = 6.955ms # mean end-to-end latency
after removing outliers [349669, 367520, 371855, 370391, 366159, 367565, 365957, 322690] lat = 6.970ms # mean end-to-end latency
tps = 1250.50 # average transactions per second
max_tps = 1350 # peak transactions per second
min_tps = 1100 # minimum transactions per second
95th_percentile = 7.234ms
99th_percentile = 8.123ms
```