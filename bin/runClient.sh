#!/usr/bin/env bash

PID_FILE=client.pid

if [ -f "${PID_FILE}" ]; then
    PID=$(cat "${PID_FILE}")
else
    PID=""
fi

if [ -z "${PID}" ]; then
    echo "Process id for clients is written to location: {$PID_FILE}"
    go build ../client/
    ./client -log_dir=./ClientLog -log_level=debug&
    echo $! >> ${PID_FILE}
else
    echo "Clients are already started in this folder."
    exit 0
fi
