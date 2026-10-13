#!/bin/bash
CONTAINERS=$(docker ps --format "{{.Names}}" | grep -E "^peer0\.|^orderer\.example\.com$|^dev-peer0.*aegis-")

while true; do
    echo "=== $(date +%s.%N) ==="
    docker stats --no-stream --format "table {{.Name}}\t{{.CPUPerc}}\t{{.MemUsage}}" $CONTAINERS
    sleep 0.05
done