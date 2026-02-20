#!/usr/bin/env bash

# 定义函数：停止进程
stop_process() {
    local pid_file="$1"
    local name="$2"

    if [ ! -f "${pid_file}" ]; then
        echo "${name} is not running."
        return
    fi

    local has_pid=false
    while read pid; do
        if [ -z "${pid}" ]; then
            continue
        fi
        has_pid=true
        if kill -15 "${pid}" 2>/dev/null; then
            echo "${name} with PID ${pid} shutdown."
        else
            echo "Failed to shutdown ${name} with PID ${pid}."
        fi
    done < "${pid_file}"

    if ! $has_pid; then
        echo "No ${name} is running."
    fi

    rm -f "${pid_file}"
}

# 定义函数：删除日志文件
delete_logs() {
    local log_pattern="$1"
    if ls ${log_pattern} 1> /dev/null 2>&1; then
        echo "Deleting logs matching pattern: ${log_pattern}"
        rm -f ${log_pattern}
    else
        echo "No logs found matching pattern: ${log_pattern}"
    fi
}

# 运行 closeClient.sh 脚本
if [ -f "closeClient.sh" ]; then
    echo "Running closeClient.sh..."
    bash closeClient.sh
else
    echo "closeClient.sh not found."
fi

# 停止 server 和 master 进程
stop_process "server.pid" "Server"
stop_process "master.pid" "Master"

# 删除指定的日志文件
rm  ./clientLog/*.log
killall -9 server

rm ./serverLog/*.log


