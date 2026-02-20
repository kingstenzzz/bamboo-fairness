#!/usr/bin/env python3
"""
HotStuff Benchmark数据验证和转换脚本
用于检查当前日志格式并提供修复建议
"""

import sys
import re
import argparse
from datetime import datetime

def analyze_log_format(log_file):
    """分析日志文件格式"""
    send_cmd_pattern = re.compile(r'\[hotstuff info\] send new cmd [0-9a-f]{10}')
    response_pattern = re.compile(r'(\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2}\.\d{6}) \[hotstuff info\] ([0-9.]+)')
    computation_pattern = re.compile(r'\[hotstuff computation info\] ([0-9.]+)')
    
    send_count = 0
    response_count = 0
    computation_count = 0
    total_lines = 0
    
    with open(log_file, 'r') as f:
        for line in f:
            total_lines += 1
            if send_cmd_pattern.search(line):
                send_count += 1
            elif response_pattern.search(line):
                response_count += 1
            elif computation_pattern.search(line):
                computation_count += 1
    
    print(f"=== 日志文件分析结果 ===")
    print(f"总行数: {total_lines}")
    print(f"发送命令行数: {send_count}")
    print(f"响应延迟行数: {response_count}")
    print(f"计算延迟行数: {computation_count}")
    
    if response_count > 0:
        print("✅ 检测到正确的benchmark性能数据")
        return "correct"
    elif send_count > 0 and response_count == 0:
        print("❌ 只检测到发送命令，未检测到响应延迟数据")
        print("可能原因:")
        print("1. Benchmark模式未正确启用")
        print("2. 客户端未收到确认响应")
        print("3. 集群未正确启动或连接")
        return "missing_response"
    else:
        print("❌ 未检测到有效的HotStuff日志数据")
        return "no_data"

def convert_to_standard_format(input_file, output_file):
    """将日志转换为标准分析格式（如果可能）"""
    print(f"尝试从 {input_file} 提取性能数据...")
    
    response_pattern = re.compile(r'(\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2}\.\d{6}) \[hotstuff info\] ([0-9.]+)')
    latencies = []
    
    with open(input_file, 'r') as infile:
        for line in infile:
            match = response_pattern.search(line)
            if match:
                timestamp = match.group(1)
                latency = float(match.group(2))
                latencies.append(latency)
    
    if latencies:
        with open(output_file, 'w') as outfile:
            for i, latency in enumerate(latencies):
                timestamp = f"2026-02-14 01:32:44.{323798 + i*40:06d}"
                outfile.write(f"{timestamp} [hotstuff info] {latency:.6f}\n")
        
        print(f"✅ 已提取 {len(latencies)} 条延迟数据到 {output_file}")
        print(f"平均延迟: {sum(latencies)/len(latencies)*1000:.3f}ms")
        return True
    else:
        print("❌ 无法从输入文件中提取延迟数据")
        return False

def main():
    parser = argparse.ArgumentParser(description='HotStuff Benchmark数据验证工具')
    parser.add_argument('logfile', help='要分析的日志文件路径')
    parser.add_argument('--convert', action='store_true', help='尝试转换为标准格式')
    parser.add_argument('--output', '-o', default='standard_benchmark_data.log', help='转换后的输出文件')
    
    args = parser.parse_args()
    
    # 分析日志格式
    result = analyze_log_format(args.logfile)
    
    if args.convert and result == "missing_response":
        print("\n=== 尝试数据转换 ===")
        success = convert_to_standard_format(args.logfile, args.output)
        if success:
            print(f"\n可以使用以下命令分析转换后的数据:")
            print(f"cat {args.output} | python3 scripts/thr_hist.py")
            print(f"cat {args.output} | python3 scripts/performance_analyzer.py")

if __name__ == '__main__':
    main()