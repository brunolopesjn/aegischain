#!/bin/bash
set -e

FABRIC_SAMPLES_PATH=${FABRIC_SAMPLES_PATH:-$HOME/fabric-samples}
CERT=$FABRIC_SAMPLES_PATH/test-network/organizations/peerOrganizations/org1.example.com/users/Admin@org1.example.com/msp/signcerts
KEYSTORE=$FABRIC_SAMPLES_PATH/test-network/organizations/peerOrganizations/org1.example.com/users/Admin@org1.example.com/msp/keystore
TLS=$FABRIC_SAMPLES_PATH/test-network/organizations/peerOrganizations/org1.example.com/peers/peer0.org1.example.com/tls/ca.crt

cd "$(dirname "$0")"
LOGDIR=${LOGDIR:-$HOME/scalability-results}
mkdir -p "$LOGDIR"

run_measurements() {
  local experiment=$1
  local level=$2
  local tag=$3
  for i in $(seq 1 15); do
    echo "[$(date '+%H:%M:%S')] $experiment level=$level rep=$i/15"
    ./aegischain-benchmark-scalability \
      -experiment "$experiment" -level "$level" -skip-seed \
      -cert-dir "$CERT" -keystore-dir "$KEYSTORE" -tls-cert "$TLS" \
      > "$LOGDIR/${experiment}-${tag}-rep${i}.log" 2>&1
  done
}

seed_to() {
  local experiment=$1
  local from=$2
  local to=$3
  local tag=$4
  echo "[$(date '+%H:%M:%S')] Seed $experiment: $from -> $to"
  ./aegischain-benchmark-scalability -experiment "$experiment" -level "$to" -seed-start "$from" -concurrency 200 \
    -cert-dir "$CERT" -keystore-dir "$KEYSTORE" -tls-cert "$TLS" \
    > "$LOGDIR/seed-${experiment}-${tag}.log" 2>&1
}

echo "=========================================="
echo "EXPERIMENTO A: CheckAccess vs tamanho total"
echo "=========================================="

seed_to checkaccess 0 100 100
run_measurements checkaccess 100 100

seed_to checkaccess 100 1000 1k
run_measurements checkaccess 1000 1k

seed_to checkaccess 1000 10000 10k
run_measurements checkaccess 10000 10k

seed_to checkaccess 10000 100000 100k
run_measurements checkaccess 100000 100k

seed_to checkaccess 100000 1000000 1m
run_measurements checkaccess 1000000 1m

echo "=========================================="
echo "EXPERIMENTO B: QueryConsentsByPatient vs registros por paciente"
echo "=========================================="

seed_to discovery 0 10 10
run_measurements discovery 10 10

seed_to discovery 10 100 100
run_measurements discovery 100 100

seed_to discovery 100 1000 1k
run_measurements discovery 1000 1k

seed_to discovery 1000 10000 10k
run_measurements discovery 10000 10k

seed_to discovery 10000 100000 100k
run_measurements discovery 100000 100k

seed_to discovery 100000 1000000 1m
run_measurements discovery 1000000 1m

echo "[$(date '+%H:%M:%S')] TUDO CONCLUÍDO"
