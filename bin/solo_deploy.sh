#!/usr/bin/env zsh


SERVER_PID_FILE=server.pid
MAXPEERNUM=$(awk 'END {print NR}' ips.txt)

SERVER_PID=$(cat "${SERVER_PID_FILE}");
jq '.zoned = false' config.json > tmp.json && mv tmp.json config.json
if [ -z "${SERVER_PID}" ]; then
    echo "Process id for servers is written to location: {$SERVER_PID_FILE}"
    go build ../server/
    int=1
    while (( $int<=$MAXPEERNUM ))
    do
	    ./server -id $int -log_dir=./serverLog -log_level=info -algorithm=hotstuff &
	    echo $! >> ${SERVER_PID_FILE}
	    let "int++"
    done
else
    echo "Servers are already started in this folder."
    exit 0
fi
