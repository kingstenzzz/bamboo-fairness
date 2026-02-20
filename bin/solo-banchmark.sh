#!/usr/bin/env bash

N=$(awk 'END {print NR}' ips.txt)
set -e   # 任一命令失败即退出
Z=$(echo "scale=2; sqrt($N)" | bc)

CONSENSUS=${1:-Default}       # 默认使用默认 HotStuff 排序
DELAY=${2:-5}              # 默认 5ms 延迟
CONFIG_JSON="./config.json"

CLIENT_COUNTS=(2 4 6 8)

# ---------- 选择共识和排序方法 ----------
case "$CONSENSUS" in
    Themis)
        echo ">>> 启用 Themis 公平排序算法"
        START_SCRIPT="bash ./solo_deploy.sh"
        # 启用 Themis，禁用 HyperG
        jq '.themis.enabled = true | .hyperg.enabled = false' config.json > tmp.json && mv tmp.json config.json
        ;;
    HyperG)
        echo ">>> 启用 HyperG 超图排序算法"
        START_SCRIPT="bash ./solo_deploy.sh"
        # 启用 HyperG，禁用 Themis
        jq '.themis.enabled = false | .hyperg.enabled = true' config.json > tmp.json && mv tmp.json config.json
        ;;
    Default)
        echo ">>> 使用默认 HotStuff 排序"
        START_SCRIPT="bash ./solo_deploy.sh"
        # 禁用所有特殊排序算法
        jq '.themis.enabled = false | .hyperg.enabled = false' config.json > tmp.json && mv tmp.json config.json
        ;;
    Both)
        echo ">>> 同时启用 Themis 和 HyperG 排序算法"
        START_SCRIPT="bash ./solo_deploy.sh"
        # 同时启用两种排序算法
        jq '.themis.enabled = true | .hyperg.enabled = true' config.json > tmp.json && mv tmp.json config.json
        ;;
    Kauri)
        echo ">>> Kauri 共识算法"
        START_SCRIPT="bash ./solo_deploy_kauri.sh"
        ;;
    *)
        echo "Unknown sorting method: $CONSENSUS"
        echo "Available options: Themis, HyperG, Default, Both, Kauri"
        exit 1
        ;;
esac
# 可用的排序方法:
# Themis:      启用 Themis 公平排序算法
# HyperG:      启用 HyperG 超图排序算法  
# Default:     使用默认 HotStuff 排序
# Both:        同时启用 Themis 和 HyperG
# Kauri:       Kauri 共识算法

# ---------- 初始化 ----------
echo ">>> Start."
$START_SCRIPT
echo ">>> Environment ready."
sleep 5
# ---------- 按 delay 设置 ----------
echo ">>> Setting delay $DELAY ..."
#./dockerDelay.sh "$DELAY"
echo "Number of IPs in ips.txt: $N"
# 读取 config.json 文件中的 T 字段
T=$(jq '.benchmark.T' config.json)
C=$((T / 10))
# 检查 T 是否为有效数字
if ! [[ "$T" =~ ^[0-9]+$ ]]; then
    echo "Error: T is not a valid number in config.json."
    exit 1
fi
$START_SCRIPT
PID_FILE=client.pid
C=10
j=1
if [ -z "${PID}" ]; then
    echo "Process id for clients is written to location: ${PID_FILE}"
      while (( $j<=$C ))
        do

        echo "run ${c} client"
         #while (( $j<=$c ))
         # do

              ./client -log_level=info -log_dir=./clientLog &
              echo $! >> ${PID_FILE}
              let j=j+1
              echo "$j"

         # done
        echo "get data"
        sleep 10  # 每隔20秒启动一个客户端
        curl "http://127.0.0.1:8070/query" | sed "1s/^/client:$j /" >> tmp.txt
        sleep 10  # 每隔20秒启动一个客户端
        curl "http://127.0.0.1:8070/query" | sed "1s/^/client:$j/" >> tmp.txt
    done

else
    echo "Clients are already started in this folder."
fi

REMOTE_DIR1="./result/0912"
REMOTE_DIR2="./tmp/0912"

mkdir -p "$REMOTE_DIR1"
mkdir -p "$REMOTE_DIR2"

grep -E 'Throughput:|Latency is' tmp.txt > "$REMOTE_DIR1/${N}-${CONSENSUS}-${DELAY}ms.txt"
cp tmp.txt $REMOTE_DIR2/${N}-${CONSENSUS}-${DELAY}ms-tmp.txt
rm tmp.txt
./stop.sh



