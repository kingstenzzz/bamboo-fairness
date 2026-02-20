#!/usr/bin/env python3
"""
HotStuff性能分析工具
计算延迟(latency)和吞吐量(throughput)指标
支持标准输出格式解析和统计分析
"""

import sys
import re
import argparse
import numpy as np
from datetime import datetime, timedelta
from typing import List, Tuple, Dict

def parse_hotstuff_log(log_content: str) -> Tuple[List[float], List[datetime]]:
    """
    解析HotStuff客户端日志，提取延迟数据
    格式: YYYY-MM-DD HH:MM:SS.ffffff [hotstuff info] latency_value
    """
    latencies = []
    timestamps = []
    
    # 匹配日志格式
    pattern = re.compile(r'(\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2}\.\d+) \[hotstuff info\] ([\d.]+)')
    
    for line in log_content.split('\n'):
        match = pattern.search(line)
        if match:
            timestamp_str = match.group(1)
            latency_str = match.group(2)
            
            try:
                # 解析时间戳
                if '.' in timestamp_str:
                    # 处理微秒部分
                    parts = timestamp_str.split('.')
                    dt = datetime.strptime(parts[0], "%Y-%m-%d %H:%M:%S")
                    microsecond = int(parts[1][:6].ljust(6, '0'))  # 确保6位微秒
                    timestamp = dt.replace(microsecond=microsecond)
                else:
                    timestamp = datetime.strptime(timestamp_str, "%Y-%m-%d %H:%M:%S")
                
                # 解析延迟值（秒）
                latency = float(latency_str)
                
                timestamps.append(timestamp)
                latencies.append(latency)
                
            except (ValueError, IndexError) as e:
                print(f"警告: 无法解析行: {line.strip()}", file=sys.stderr)
                continue
    
    return latencies, timestamps

def remove_outliers(data: List[float], outlier_constant: float = 1.5) -> Tuple[List[float], List[float]]:
    """
    使用四分位距(IQR)方法移除异常值
    """
    if len(data) < 4:
        return data, []
    
    arr = np.array(data)
    q75, q25 = np.percentile(arr, [75, 25])
    iqr = (q75 - q25) * outlier_constant
    lower_bound = q25 - iqr
    upper_bound = q75 + iqr
    
    filtered_data = []
    outliers = []
    
    for value in data:
        if lower_bound <= value <= upper_bound:
            filtered_data.append(value)
        else:
            outliers.append(value)
    
    return filtered_data, outliers

def calculate_throughput(timestamps: List[datetime], interval: float = 1.0) -> List[int]:
    """
    计算吞吐量（每秒交易数）
    """
    if not timestamps:
        return []
    
    timestamps_sorted = sorted(timestamps)
    start_time = timestamps_sorted[0]
    end_time = timestamps_sorted[-1]
    
    # 计算时间窗口数量
    duration = (end_time - start_time).total_seconds()
    num_intervals = int(np.ceil(duration / interval))
    
    throughput = [0] * (num_intervals + 1)
    
    for timestamp in timestamps_sorted:
        time_diff = (timestamp - start_time).total_seconds()
        interval_index = int(time_diff / interval)
        if 0 <= interval_index < len(throughput):
            throughput[interval_index] += 1
    
    return throughput

def format_latency_stats(latencies: List[float], label: str = "") -> str:
    """
    格式化延迟统计信息
    """
    if not latencies:
        return f"{label}lat = N/A ms"
    
    mean_latency_ms = np.mean(latencies) * 1000  # 转换为毫秒
    median_latency_ms = np.median(latencies) * 1000
    std_latency_ms = np.std(latencies) * 1000
    
    result = f"[{', '.join(map(str, [int(x*1000000) for x in latencies[:8]]))}] "
    result += f"lat = {mean_latency_ms:.3f}ms # mean end-to-end latency"
    
    if label:
        result = label + result
    
    return result

def analyze_performance(log_content: str, interval: float = 1.0) -> Dict:
    """
    完整的性能分析
    """
    # 解析日志数据
    latencies, timestamps = parse_hotstuff_log(log_content)
    
    if not latencies:
        raise ValueError("未找到有效的延迟数据")
    
    print(f"解析到 {len(latencies)} 个延迟样本", file=sys.stderr)
    
    # 基础统计
    results = {
        'total_samples': len(latencies),
        'raw_latencies': latencies,
        'timestamps': timestamps
    }
    
    # 计算原始延迟统计
    print(format_latency_stats(latencies))
    
    # 移除异常值后统计
    filtered_latencies, outliers = remove_outliers(latencies)
    if outliers:
        print(f"移除了 {len(outliers)} 个异常值", file=sys.stderr)
        print(format_latency_stats(filtered_latencies, "after removing outliers "))
    else:
        print("lat = {:.3f}ms # after removing outliers".format(np.mean(filtered_latencies) * 1000))
    
    # 计算吞吐量
    if timestamps:
        throughput = calculate_throughput(timestamps, interval)
        avg_throughput = np.mean(throughput)
        max_throughput = max(throughput)
        min_throughput = min(throughput)
        
        results['throughput'] = {
            'values': throughput,
            'average': avg_throughput,
            'max': max_throughput,
            'min': min_throughput,
            'interval': interval
        }
        
        print(f"tps = {avg_throughput:.2f} # average transactions per second")
        print(f"max_tps = {max_throughput} # peak transactions per second")
        print(f"min_tps = {min_throughput} # minimum transactions per second")
    
    # 额外统计信息
    if latencies:
        percentile_95 = np.percentile(latencies, 95) * 1000
        percentile_99 = np.percentile(latencies, 99) * 1000
        print(f"95th_percentile = {percentile_95:.3f}ms")
        print(f"99th_percentile = {percentile_99:.3f}ms")
    
    return results

def main():
    parser = argparse.ArgumentParser(description='HotStuff性能分析工具')
    parser.add_argument('--interval', type=float, default=1.0, 
                       help='吞吐量计算时间间隔（秒）')
    parser.add_argument('--input', type=str, help='输入日志文件路径')
    parser.add_argument('--verbose', action='store_true', help='详细输出')
    
    args = parser.parse_args()
    
    try:
        # 读取输入数据
        if args.input:
            with open(args.input, 'r') as f:
                log_content = f.read()
        else:
            log_content = sys.stdin.read()
        
        if not log_content.strip():
            print("错误: 输入为空", file=sys.stderr)
            sys.exit(1)
        
        # 执行分析
        results = analyze_performance(log_content, args.interval)
        
        if args.verbose:
            print("\n=== 详细统计信息 ===", file=sys.stderr)
            print(f"总样本数: {results['total_samples']}", file=sys.stderr)
            if 'throughput' in results:
                thr = results['throughput']
                print(f"吞吐量统计 (间隔: {thr['interval']}s):", file=sys.stderr)
                print(f"  平均: {thr['average']:.2f} TPS", file=sys.stderr)
                print(f"  最大: {thr['max']} TPS", file=sys.stderr)
                print(f"  最小: {thr['min']} TPS", file=sys.stderr)
        
    except Exception as e:
        print(f"错误: {e}", file=sys.stderr)
        sys.exit(1)

if __name__ == '__main__':
    main()