#!/bin/bash

# ==========================================================================================
# Script to manage the AegisChain test network using Hyperledger Fabric
# Author: Bruno L. A. Batista (with AI assistance)
# Usage:
#   ./aegis-network.sh start [-fp /path/to/fabric-samples] [-ap /path/to/chaincodes]
#   ./aegis-network.sh stop [-fp /path/to/fabric-samples]
#
# Environment Variables:
#   FABRIC_SAMPLES_PATH: Path to the 'fabric-samples' directory (overridden by -fp).
#   AEGISCHAIN_CHAINCODES_PATH: Path to your chaincodes directory (overridden by -ap).
#
# CHANGELOG (migração para CCaaS - deploy via chaincode-as-a-service):
#   - Os 3 chaincodes (ownership, logging, consent) agora sao construidos como
#     binarios Go rodando como PROCESSOS DO HOST (nao mais containers Docker
#     legados via `peer lifecycle chaincode install --lang golang`). O peer
#     empacota so metadata.json+connection.json (tipo "ccaas") e se conecta
#     de volta ao processo do host via host.docker.internal:PORTA.
#   - Motivo: o codigo dos 3 chaincodes ja foi migrado para
#     shim.ChaincodeServer (modo CCaaS puro) durante o trabalho na topologia
#     nativa de 4 organizacoes na nuvem, perdendo compatibilidade com o
#     mecanismo legado (que espera o binario rodar via `chaincode -peer.address`,
#     sem servidor gRPC explicito) - sintoma: "chaincode registration failed:
#     container exited with 0" / "ccid must be specified".
#   - host.docker.internal nao resolve nativamente em Docker no Linux (so em
#     Docker Desktop Mac/Windows); em vez de editar o compose-test-net.yaml
#     compartilhado do fabric-samples, injetamos a entrada direto em
#     /etc/hosts dos containers dos peers em runtime (nao-destrutivo, nao
#     afeta outros usos do mesmo fabric-samples).
#   - PIDs dos processos de chaincode sao rastreados em /tmp/aegis-ccaas-pids
#     para serem encerrados corretamente em `stop`.
# ==========================================================================================

set -e # Exit script on any error

# --- Helper Functions and Colors ---
C_RED='\033[0;31m'
C_GREEN='\033[0;32m'
C_YELLOW='\033[0;33m'
C_BLUE='\033[0;34m'
C_NC='\033[0m' # No Color

function print_info() { echo -e "${C_BLUE}INFO: ${1}${C_NC}"; }
function print_success() { echo -e "${C_GREEN}SUCCESS: ${1}${C_NC}"; }
function print_warning() { echo -e "${C_YELLOW}WARNING: ${1}${C_NC}"; }
function print_error() { echo -e "${C_RED}ERROR: ${1}${C_NC}"; exit 1; }

# --- Global Chaincode Definitions ---
CHANNEL_NAME="aegischain"
STATE_DATABASE="couchdb"
DOCKER_NETWORK="fabric_test"
PID_FILE="/tmp/aegis-ccaas-pids"

# --- Function to Print Usage Help ---
function print_usage() {
    echo "Usage:"
    echo "  ./aegis-network.sh <command> [options]"
    echo
    echo "Commands:"
    echo "  start   Starts the network and deploys the chaincodes (CCaaS)."
    echo "  stop    Stops the network, kills chaincode processes, and cleans up."
    echo
    echo "Options:"
    echo "  -f, --fabric-path <path>   Path to the 'fabric-samples' directory. Overrides FABRIC_SAMPLES_PATH env var."
    echo "  -a, --aegis-path <path>    Path to the AegisChain chaincodes directory. Overrides AEGISCHAIN_CHAINCODES_PATH env var."
}

# --- Function to Resolve Critical Paths ---
function resolve_paths() {
    if [ -n "$FABRIC_PATH_ARG" ]; then
        FABRIC_SAMPLES_PATH=$FABRIC_PATH_ARG
    elif [ -n "$FABRIC_SAMPLES_PATH" ]; then
        print_info "Using FABRIC_SAMPLES_PATH environment variable: $FABRIC_SAMPLES_PATH"
    else
        print_error "Fabric samples path not set. Please either set the FABRIC_SAMPLES_PATH environment variable or use the -f/--fabric-path flag."
    fi

    if [ ! -d "$FABRIC_SAMPLES_PATH/test-network" ]; then
        print_error "'test-network' directory not found in the resolved path: '$FABRIC_SAMPLES_PATH'."
    fi

    if [ "$COMMAND" == "start" ]; then
        if [ -n "$AEGIS_PATH_ARG" ]; then
            AEGISCHAIN_CHAINCODES_PATH=$AEGIS_PATH_ARG
        elif [ -n "$AEGISCHAIN_CHAINCODES_PATH" ]; then
            print_info "Using AEGISCHAIN_CHAINCODES_PATH environment variable: $AEGISCHAIN_CHAINCODES_PATH"
        else
            print_error "AegisChain chaincodes path not set. Please either set the AEGISCHAIN_CHAINCODES_PATH environment variable or use the -a/--aegis-path flag."
        fi

        if [ ! -d "$AEGISCHAIN_CHAINCODES_PATH" ]; then
            print_error "AegisChain chaincodes directory not found in the resolved path: '$AEGISCHAIN_CHAINCODES_PATH'."
        fi
    fi
}

# --- Function to inject host.docker.internal into the peer containers ---
# Necessario no Linux (Docker Desktop Mac/Windows ja faz isso automaticamente).
# Nao-destrutivo: edita apenas /etc/hosts dos containers ja rodando, sem
# tocar em nenhum arquivo compartilhado do fabric-samples.
function inject_host_docker_internal() {
    print_info "Configurando host.docker.internal nos peers (necessário para CCaaS)..."
    local gateway_ip
    gateway_ip=$(docker network inspect "$DOCKER_NETWORK" --format '{{range .IPAM.Config}}{{.Gateway}}{{end}}')
    if [ -z "$gateway_ip" ]; then
        print_error "Não foi possível determinar o gateway da rede Docker '$DOCKER_NETWORK'."
    fi
    docker exec -u root peer0.org1.example.com sh -c "grep -q host.docker.internal /etc/hosts || echo '$gateway_ip host.docker.internal' >> /etc/hosts"
    docker exec -u root peer0.org2.example.com sh -c "grep -q host.docker.internal /etc/hosts || echo '$gateway_ip host.docker.internal' >> /etc/hosts"
    print_success "host.docker.internal configurado (gateway: $gateway_ip)"
}

# --- Function to kill any orphaned chaincode processes from previous runs ---
# Protege contra o caso em que `start` é chamado sem um `stop` anterior
# (ex: apos um teste manual, ou apos o script ter sido interrompido no meio),
# deixando processos de chaincode antigos presos nas portas 9991-9993.
function kill_orphaned_chaincode_processes() {
    print_info "Verificando processos de chaincode órfãos de execuções anteriores..."
    for port in 9991 9992 9993; do
        local pid
        pid=$(ss -tlnp 2>/dev/null | grep ":${port} " | grep -oP 'pid=\K[0-9]+' | head -1)
        if [ -n "$pid" ]; then
            print_warning "Matando processo órfão na porta $port (PID $pid)..."
            kill "$pid" 2>/dev/null || true
            sleep 1
        fi
    done
    if [ -f "$PID_FILE" ]; then
        rm -f "$PID_FILE"
    fi
}

# --- Function to Start the Network ---
function start_network() {
    resolve_paths

    print_info "Setting up Fabric environment variables..."
    export PATH=${FABRIC_SAMPLES_PATH}/bin:${PATH}
    export FABRIC_CFG_PATH=${FABRIC_SAMPLES_PATH}/config/

    print_info "Changing to the test-network directory..."
    cd "$FABRIC_SAMPLES_PATH/test-network"

    ./network.sh down
    print_info "Starting the test network (2 orgs, 1 channel, state database: ${STATE_DATABASE})..."
    ./network.sh up createChannel -c ${CHANNEL_NAME} -ca -s ${STATE_DATABASE}

    inject_host_docker_internal
    kill_orphaned_chaincode_processes

    # Limpa PIDs de uma execução anterior, se houver
    : > "$PID_FILE"

    # Ordem: ownership e logging primeiro (aegis-consent os invoca via
    # InvokeChaincode em runtime: RequestAccess/UpdateConsentStatus ->
    # aegis-ownership; CheckAccess -> aegis-logging).
    deploy_ccaas_chaincode "aegis-ownership" "aegischain-ownership-chaincode" 9991
    deploy_ccaas_chaincode "aegis-logging"  "aegischain-logging-chaincode"  9992
    deploy_ccaas_chaincode "aegis-consent"  "aegischain-consent-chaincode"  9993

    print_success "AegisChain network started and all chaincodes have been deployed successfully (CCaaS)!"
    print_info "Network is ready for transactions on channel '${CHANNEL_NAME}'."
}

# --- Function to deploy ANY of the 3 chaincodes via CCaaS ---
# Args: $1 = chaincode name (ex: aegis-ownership)
#       $2 = pasta do codigo-fonte (ex: aegischain-ownership-chaincode)
#       $3 = porta local onde o binario vai escutar (ex: 9991)
function deploy_ccaas_chaincode() {
    local CC_NAME="$1"
    local CC_FOLDER="$2"
    local CC_PORT="$3"
    local CC_LABEL="${CC_NAME}_1.0"
    local CC_SRC_PATH="$AEGISCHAIN_CHAINCODES_PATH/$CC_FOLDER"
    local CC_PKG_DIR="/tmp/aegis-ccaas-pkg/$CC_NAME"
    local CC_PKG_FILE="$CC_PKG_DIR/${CC_NAME}-ccaas.tar.gz"
    local CC_BIN="$CC_SRC_PATH/${CC_NAME}-bin"

    print_info "== Deploy CCaaS: $CC_NAME (porta $CC_PORT) =="

    # 1. Compila o binario do chaincode
    print_info "Compilando binário de $CC_NAME..."
    (cd "$CC_SRC_PATH" && go build -o "$CC_BIN" .)

    # 2. Monta o pacote CCaaS (metadata.json + connection.json - NAO o
    #    codigo-fonte, diferente do modelo legado)
    mkdir -p "$CC_PKG_DIR"
    cat > "$CC_PKG_DIR/metadata.json" << EOF
{"type":"ccaas","label":"${CC_LABEL}"}
EOF
    cat > "$CC_PKG_DIR/connection.json" << EOF
{"address":"host.docker.internal:${CC_PORT}","dial_timeout":"10s","tls_required":false}
EOF
    (cd "$CC_PKG_DIR" && tar cfz code.tar.gz connection.json && tar cfz "$(basename "$CC_PKG_FILE")" metadata.json code.tar.gz)

    # 3. Instala no Org1, obtem o Package ID real
    export CORE_PEER_TLS_ENABLED=true
    export CORE_PEER_LOCALMSPID="Org1MSP"
    export CORE_PEER_TLS_ROOTCERT_FILE=${FABRIC_SAMPLES_PATH}/test-network/organizations/peerOrganizations/org1.example.com/peers/peer0.org1.example.com/tls/ca.crt
    export CORE_PEER_MSPCONFIGPATH=${FABRIC_SAMPLES_PATH}/test-network/organizations/peerOrganizations/org1.example.com/users/Admin@org1.example.com/msp
    export CORE_PEER_ADDRESS=localhost:7051

    print_info "Instalando $CC_NAME no Org1..."
    peer lifecycle chaincode install "$CC_PKG_FILE"
    local CC_PACKAGE_ID
    CC_PACKAGE_ID=$(peer lifecycle chaincode queryinstalled | grep "Label: ${CC_LABEL}" | sed 's/Package ID: \(.*\), Label: .*/\1/' | tail -1)
    print_info "Package ID: $CC_PACKAGE_ID"

    # 4. Sobe o binario do chaincode como processo do host, usando o
    #    Package ID exato como CHAINCODE_ID (precisa vir DEPOIS do install,
    #    ja que o ID depende do conteudo do pacote)
    print_info "Iniciando processo do chaincode $CC_NAME em background (porta $CC_PORT)..."
    CHAINCODE_ID="$CC_PACKAGE_ID" CHAINCODE_SERVER_ADDRESS="0.0.0.0:${CC_PORT}" \
        "$CC_BIN" > "/tmp/aegis-ccaas-pkg/$CC_NAME/server.log" 2>&1 &
    local CC_PID=$!
    echo "$CC_PID" >> "$PID_FILE"
    sleep 2

    if ! kill -0 "$CC_PID" 2>/dev/null; then
        print_error "Processo do chaincode $CC_NAME morreu logo após iniciar. Log: /tmp/aegis-ccaas-pkg/$CC_NAME/server.log"
    fi

    # 5. Instala no Org2
    export CORE_PEER_LOCALMSPID="Org2MSP"
    export CORE_PEER_TLS_ROOTCERT_FILE=${FABRIC_SAMPLES_PATH}/test-network/organizations/peerOrganizations/org2.example.com/peers/peer0.org2.example.com/tls/ca.crt
    export CORE_PEER_MSPCONFIGPATH=${FABRIC_SAMPLES_PATH}/test-network/organizations/peerOrganizations/org2.example.com/users/Admin@org2.example.com/msp
    export CORE_PEER_ADDRESS=localhost:9051

    print_info "Instalando $CC_NAME no Org2..."
    peer lifecycle chaincode install "$CC_PKG_FILE"

    # 6. Aprova nos dois orgs
    local ORDERER_CA="${FABRIC_SAMPLES_PATH}/test-network/organizations/ordererOrganizations/example.com/orderers/orderer.example.com/msp/tlscacerts/tlsca.example.com-cert.pem"

    print_info "Aprovando $CC_NAME no Org2..."
    peer lifecycle chaincode approveformyorg -o localhost:7050 --ordererTLSHostnameOverride orderer.example.com \
        --channelID ${CHANNEL_NAME} --name "$CC_NAME" --version 1.0 --package-id "$CC_PACKAGE_ID" --sequence 1 \
        --tls --cafile "$ORDERER_CA"

    export CORE_PEER_LOCALMSPID="Org1MSP"
    export CORE_PEER_TLS_ROOTCERT_FILE=${FABRIC_SAMPLES_PATH}/test-network/organizations/peerOrganizations/org1.example.com/peers/peer0.org1.example.com/tls/ca.crt
    export CORE_PEER_MSPCONFIGPATH=${FABRIC_SAMPLES_PATH}/test-network/organizations/peerOrganizations/org1.example.com/users/Admin@org1.example.com/msp
    export CORE_PEER_ADDRESS=localhost:7051

    print_info "Aprovando $CC_NAME no Org1..."
    peer lifecycle chaincode approveformyorg -o localhost:7050 --ordererTLSHostnameOverride orderer.example.com \
        --channelID ${CHANNEL_NAME} --name "$CC_NAME" --version 1.0 --package-id "$CC_PACKAGE_ID" --sequence 1 \
        --tls --cafile "$ORDERER_CA"

    # 7. Checa e commita
    peer lifecycle chaincode checkcommitreadiness --channelID ${CHANNEL_NAME} --name "$CC_NAME" --version 1.0 --sequence 1 \
        --tls --cafile "$ORDERER_CA" --output json

    print_info "Commitando $CC_NAME..."
    peer lifecycle chaincode commit -o localhost:7050 --ordererTLSHostnameOverride orderer.example.com \
        --channelID ${CHANNEL_NAME} --name "$CC_NAME" --version 1.0 --sequence 1 --tls --cafile "$ORDERER_CA" \
        --peerAddresses localhost:7051 --tlsRootCertFiles "${FABRIC_SAMPLES_PATH}/test-network/organizations/peerOrganizations/org1.example.com/peers/peer0.org1.example.com/tls/ca.crt" \
        --peerAddresses localhost:9051 --tlsRootCertFiles "${FABRIC_SAMPLES_PATH}/test-network/organizations/peerOrganizations/org2.example.com/peers/peer0.org2.example.com/tls/ca.crt"

    peer lifecycle chaincode querycommitted --channelID ${CHANNEL_NAME} --name "$CC_NAME" --cafile "$ORDERER_CA"

    print_success "$CC_NAME deployed via CCaaS!"
}

# --- Function to Stop the Network ---
function stop_network() {
    resolve_paths

    print_info "Encerrando processos de chaincode CCaaS (host)..."
    if [ -f "$PID_FILE" ]; then
        while read -r pid; do
            if [ -n "$pid" ] && kill -0 "$pid" 2>/dev/null; then
                kill "$pid" 2>/dev/null || true
            fi
        done < "$PID_FILE"
        rm -f "$PID_FILE"
    fi

    cd "$FABRIC_SAMPLES_PATH/test-network"
    print_info "Stopping the test network and removing all containers and volumes..."
    ./network.sh down
    print_success "AegisChain network environment stopped and cleaned up successfully."
}

# --- Main Script Logic ---
if [ "$#" -eq 0 ]; then
    print_usage
    exit 1
fi

COMMAND=$1
shift

PARSED_ARGS=$(getopt -o f:a: --long fabric-path:,aegis-path: -n 'aegis-network.sh' -- "$@") || {
    print_usage
    exit 1
}
eval set -- "$PARSED_ARGS"

while true; do
  case "$1" in
    -f|--fabric-path)
        FABRIC_PATH_ARG="$2"
        shift 2
        ;;
    -a|--aegis-path)
        AEGIS_PATH_ARG="$2"
        shift 2
        ;;
    --)
        shift
        break
        ;;
    *)
        print_error "Invalid option: $1"
        ;;
  esac
done

case "$COMMAND" in
    start)
        start_network
        ;;
    stop)
        stop_network
        ;;
    *)
        print_error "Unknown command: $COMMAND"
        print_usage
        ;;
esac