#!/usr/bin/env bash
# usage: ./gen_ips.sh <node_cnt>

set -e

node_cnt=${1:?please give node_cnt}

# 生成节点 IP
printf '' > ips_"$node_cnt".txt           # 覆盖写
for ((i=0; i<node_cnt; i++)); do

    printf "127.0.0.1\n"
done > ips_"$node_cnt".txt

# 统一复制
cp -f ips_"$node_cnt".txt ips.txt
cp -f ips_"$node_cnt".txt public_ips.txt