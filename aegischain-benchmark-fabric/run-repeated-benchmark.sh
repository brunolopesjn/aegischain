#!/bin/bash
# ==========================================================================================
# run-repeated-benchmark.sh
#
# Orquestra N repetições completas do benchmark Fabric concorrente:
#   para cada repetição -> reseta a rede (./aegis-network.sh start) -> roda o
#   benchmark (com seed novo, -skip-sequential) -> salva log bruto + extrai
#   p50/p95/throughput para um CSV consolidado.
#
# Uso:
#   ./run-repeated-benchmark.sh [N]
#
# Variáveis de ambiente esperadas (mesmas já usadas manualmente):
#   FABRIC_SAMPLES_PATH        - ex: ~/fabric-samples
#   AEGISCHAIN_NETWORK_DIR     - ex: $AEGISCHAIN_REPO/aegischain-network/multihost-2org
#   AEGISCHAIN_BENCHMARK_DIR   - ex: $AEGISCHAIN_REPO/aegischain-benchmark-fabric
#   ORG1                       - ex: $FABRIC_SAMPLES_PATH/test-network/organizations/peerOrganizations/org1.example.com
# ==========================================================================================

set -euo pipefail

N=${1:-15}

: "${FABRIC_SAMPLES_PATH:?defina FABRIC_SAMPLES_PATH}"
: "${AEGISCHAIN_NETWORK_DIR:?defina AEGISCHAIN_NETWORK_DIR}"
: "${AEGISCHAIN_BENCHMARK_DIR:?defina AEGISCHAIN_BENCHMARK_DIR}"
: "${ORG1:?defina ORG1}"

RESULTS_DIR="$AEGISCHAIN_BENCHMARK_DIR/results"
CSV_FILE="$RESULTS_DIR/summary.csv"

mkdir -p "$RESULTS_DIR"

# Cabeçalho do CSV, só na primeira vez (permite retomar/estender sem perder dados antigos)
if [ ! -f "$CSV_FILE" ]; then
    echo "run,operation,p50_ms,p95_ms,throughput_tx_s" > "$CSV_FILE"
fi

# Converte string de duração do Go ("297.423096ms" ou "2.097429324s") para milissegundos.
duration_to_ms() {
    local val="$1"
    if [[ "$val" == *ms ]]; then
        echo "${val%ms}"
    elif [[ "$val" == *s ]]; then
        awk -v v="${val%s}" 'BEGIN { printf "%.3f", v * 1000 }'
    else
        echo "$val"
    fi
}

echo "== Rodando $N repetições completas =="

for i in $(seq 1 "$N"); do
    echo
    echo "############################################"
    echo "## Repetição $i/$N"
    echo "############################################"

    # 1. Reset completo da rede
    ( cd "$AEGISCHAIN_NETWORK_DIR" && ./aegis-network.sh start )

    # 2. Roda o benchmark (seed novo + concorrente, sem sequencial)
    LOG_FILE="$RESULTS_DIR/run-$i.log"
    (
        cd "$AEGISCHAIN_BENCHMARK_DIR"
        ./aegischain-benchmark-fabric \
            -cert "$ORG1/users/User1@org1.example.com/msp/signcerts/cert.pem" \
            -keystore "$ORG1/users/User1@org1.example.com/msp/keystore" \
            -tls-cert "$ORG1/peers/peer0.org1.example.com/tls/ca.crt" \
            -skip-sequential
    ) | tee "$LOG_FILE"

    # 3. Extrai p50/p95/throughput das três operações concorrentes e acrescenta ao CSV
    grep '^\[concurrent\]' "$LOG_FILE" | while IFS= read -r line; do
        op=$(echo "$line" | sed -E 's/^\[concurrent\]\s+([A-Za-z]+).*/\1/')
        p50_raw=$(echo "$line" | grep -oP 'p50=\K[0-9.]+m?s')
        p95_raw=$(echo "$line" | grep -oP 'p95=\K[0-9.]+m?s')
        tput=$(echo "$line" | grep -oP 'throughput=\K[0-9.]+')

        p50_ms=$(duration_to_ms "$p50_raw")
        p95_ms=$(duration_to_ms "$p95_raw")

        echo "$i,$op,$p50_ms,$p95_ms,$tput" >> "$CSV_FILE"
    done

    echo "-- Repetição $i concluída, resultados em $LOG_FILE --"
done

echo
echo "== Todas as $N repetições concluídas =="
echo "CSV consolidado: $CSV_FILE"