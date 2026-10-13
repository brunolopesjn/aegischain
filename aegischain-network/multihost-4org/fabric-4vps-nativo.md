# AegisChain — Fabric v2.5.16 nativo em 4 VPS (3 hospitais + 1 clínica)

Documento gerado do zero, sem reaproveitar o `multihost-4org` anterior. Baseado na documentação
oficial do Hyperledger Fabric (Install Fabric and Fabric Samples; Running Chaincode as an External
Service `cc_service.html`; Running External Chaincode Builders `cc_basic.html`; Create a channel
— Channel Participation API; e o exemplo oficial `test-network-nano-bash` do `fabric-samples`, que
é literalmente o tutorial oficial de rede nativa sem Docker para peer/orderer).

> **Revisão 26**: `aegis-logging` replicado com sucesso (mesmo padrão, porta 9993, label
> `aegis-logging_1.0`) — sem bugs novos, já que o `logging.go` não faz `InvokeChaincode` (só é
> chamado). **Marco: os 3 chaincodes fecham de ponta a ponta** — teste final com `CheckAccess` do
> `aegis-consent` (`status:200, payload:false`, esperado pois o consent de teste está em
> `WAITING_APPROVAL`) disparando `InvokeChaincode` → `aegis-logging.CreateLogEntry`, confirmado via
> `QueryLogsByPatient` (rich query CouchDB) retornando o `LogEntry` de `FAILURE` gravado.
>
> **Revisão 25**: CouchDB incorporado ao processo inicial de configuração (Seção 6.1, nova) — em
> vez de deixar como pendência pra migrar depois (o que exige apagar/reconstruir o `statedb`), o
> `docker run` do CouchDB e as variáveis `CORE_LEDGER_STATE_*` agora fazem parte do setup normal
> do peer, incluídas diretamente nos 4 `peer.env`. Migração real em campo documentada como
> referência: `sudo systemctl stop fabric-peer` → apagar só `stateLeveldb` (não o `blkstorage`) →
> reiniciar — o Fabric reconstrói o índice de estado a partir dos blocos já existentes sem precisar
> sair/reentrar do canal.
>
> **Revisão 24**: `aegis-consent` replicado com sucesso (Seções 8.1–8.5, mesmo padrão do
> `aegis-ownership`) — porta 9992, label `aegis-consent_1.0`. Bug real encontrado no código-fonte
> antes do build: `ChannelName string = "mychannel"` (deveria ser `"aegischain"`, o canal real
> desta topologia) — sem essa correção, todo `InvokeChaincode` cross-chaincode
> (`RequestAccess`→ownership, `CheckAccess`→logging) falharia. Corrigido via `sed` no código-fonte
> antes do `tar`/build. `RequestAccess` testado em campo com `status:200`, confirmando a chamada
> cross-chaincode funcionando de verdade (não só o commit).
>
> **Revisão 23**: `checkcommitreadiness`/`commit`/`querycommitted`/teste de invoke real, validados
> em campo (Seção 8.5, nova) — `CreateOwnershipRecord` → `status:200`, `GetOwnershipRecord` de
> volta batendo com os dados enviados. Fecha o ciclo completo do `aegis-ownership` na topologia
> nativa de 4 orgs cross-datacenter.
>
> **Revisão 22**: `approveformyorg` das 4 orgs confirmado `VALID` em campo (Seção 8.4, nova). Bug
> real de firewall descoberto no processo — as regras da Seção 9 só abriam `7051` (peer) da
> Clínica 1 nas 3 VPS de hospital, mas `approveformyorg`/`commit` falam com o **orderer**
> (`7050`); adicionada a regra que faltava nos 3 blocos concretos e uma nota no topo da Seção 9.
> Também documentado `docker run` (Seção 8.3) com o Package ID real de cada VPS.
>
> **Revisão 21**: dois bugs reais, confirmados no Hospital A. (1) `peer lifecycle chaincode package
> --lang external` tem bug conhecido e nunca corrigido no Fabric (issue #3368 upstream) —
> `unknown chaincodeType: EXTERNAL`; substituído por empacotamento manual via `tar`
> (`metadata.json` + `code.tar.gz`), como a doc oficial realmente faz. (2) o builder oficial
> (`ccaasbuilder`, copiado do `fabric-samples`) espera `"type":"ccaas"` no `metadata.json`, não
> `"external"` — com o valor errado, o `detect` rejeita silenciosamente e o peer cai no fallback de
> build via Docker, gerando um erro enganoso de permissão no `docker.sock`. Também movidos todos os
> arquivos de `/home/ubuntu` pra `/tmp` — `/home/ubuntu` tem permissão `750` no Ubuntu 24.04,
> inacessível pro usuário `fabric` (Seção 8.2).
>
> **Revisão 20**: Seção 8 reescrita com o fluxo real, a partir do código-fonte de
> `aegis-ownership-chaincode` (Go, `shim.ChaincodeServer`, env vars `CHAINCODE_ID`/
> `CHAINCODE_SERVER_ADDRESS`, `TLSProps.Disabled: true`) — distribuição do código-fonte, build
> Docker por VPS, distribuição dos TLS CA cross-org (orderer + 4 peers), e os 4 blocos concretos de
> `package`/`install`. `docker run`/`approveformyorg`/`commit` ainda pendentes de documentar após
> validação em campo dos Package IDs.
>
> **Revisão 19**: bug real, confirmado no Hospital A — `ufw` recusa regra com múltiplas portas
> (`7050,7051,9443`) sem `proto tcp` explícito (`ERROR: Must specify 'tcp' or 'udp' with multiple
> ports`). Adicionado `proto tcp` em todas as regras multi-porta, e substituído o exemplo único
> genérico pelos 4 blocos concretos por VPS, com `ufw status numbered` antes do `enable` (Seção 9).
>
> **Revisão 18**: substituído o bloco genérico único de `peer channel join` (que levou a copiar
> valores errados do Hospital A pro Hospital B) pelos 4 blocos concretos, um por VPS, com os
> valores reais de MSP ID/domínio/IP já preenchidos (Seção 7).
>
> **Revisão 17**: bug real — `peer channel join` manual (fora do systemd) não herda `FABRIC_CFG_PATH`
> do `peer.env`; sem ele, `peer` não acha o `core.yaml` (`Config File "core" Not Found in
> "[/home/ubuntu]"`). Adicionado `FABRIC_CFG_PATH=/opt/fabric/config` ao `export` e ao
> `--preserve-env` em todos os comandos `peer channel join`/`list` da Seção 7.
>
> **Revisão 16**: bug real, mesma classe do `CORE_LEDGER_SNAPSHOTS_ROOTDIR` do peer (Revisão 13),
> agora no orderer — `Consensus.SnapDir`/`Consensus.WALDir` (armazenamento Raft) tem default em
> `/var/hyperledger/...`, sem permissão pro usuário `fabric`, e não era coberto por nenhuma
> variável do `orderer.env`. Adicionadas `ORDERER_CONSENSUS_WALDIR`/`ORDERER_CONSENSUS_SNAPDIR`
> (Seção 6) — como só são lidas na inicialização, exige `systemctl stop/start` do
> `fabric-orderer` após adicionar, antes de repetir o `channel join`.
>
> **Revisão 15**: bug real, confirmado no Hospital A —
> `certificate is valid for orderer1.orderer.aegischain.local, orderer1, not localhost`. O SAN dos
> certificados (Seção 2) nunca incluiu `localhost`/`127.0.0.1`, só o hostname e o IP público.
> Trocado `-o localhost:9443` por `-o <IP-público>:9443` no `osnadmin`, e `CORE_PEER_ADDRESS=localhost:7051`
> por `CORE_PEER_ADDRESS=<IP-público>:7051` no `peer channel join` — mesmo problema de verificação
> TLS nos dois casos (Seção 7).
>
> **Revisão 14**: bug real — `osnadmin`/`peer channel join` rodando como `ubuntu` batem em
> `permission denied` ao ler o `server.key` (dono `fabric`, permissão restrita). Adicionado
> `sudo -u fabric` (com `--preserve-env` explícito no caso do `peer`, já que `sudo -u` não propaga
> `export` do shell chamador por padrão) em todos os comandos da Seção 7.
>
> **Revisão 13**: bug real, confirmado via teste seco (`timeout 5 .../peer node start`) no Hospital
> A — `panic: Error in instantiating ledger provider: mkdir /var/hyperledger: permission denied`.
> `CORE_PEER_FILESYSTEMPATH` só cobre a chave `peer.fileSystemPath`; `ledger.snapshots.rootDir` é
> uma chave separada no `core.yaml`, com default `/var/hyperledger/production/snapshots` (fora de
> `/opt/fabric`, sem permissão pro usuário `fabric`). Adicionada `CORE_LEDGER_SNAPSHOTS_ROOTDIR` nos
> 4 `peer.env` (Seção 6).
>
> **Revisão 12**: padronizada toda criação de arquivo do documento no modelo `tee ... <<'EOF'`
> (com `sudo`/`sudo -u fabric` quando o destino exige) — `crypto-config.yaml`, `configtx.yaml`,
> `connection.json`, os 2 `systemd unit` files e os 4 `peer.env` (Seções 2, 3, 5 e 6). O `core.yaml`
> (Seção 4) continua como edição manual — é um arquivo já existente vindo do `fabric-samples`, e
> `tee` sobrescreveria o resto do arquivo que não queremos tocar.
>
> **Revisão 11**: `peer.env` gerado com os 4 blocos reais (não mais 1 exemplo genérico), a partir
> da estrutura de `crypto` confirmada em campo nas 4 VPS (Seção 6).
>
> **Revisão 10**: confirmado, via estrutura real gerada pelo `scp`/`mv` nas 3 VPS de hospital, que
> `orderer.env` é **idêntico** nas 3 (paths locais simétricos, sem IP específico) — removida a nota
> de "ajustar por VPS". Também documentado o bug de subpasta duplicada
> (`orderer2.../orderer3...` dentro de `/opt/fabric/crypto/orderer`) causado pelo mesmo padrão de
> "destino já existia como diretório" visto no `ccaas-builder` (Seção 6).
>
> **Revisão 9**: esclarecido que o `core.yaml` do `fabric-samples` já vem com `externalBuilders`
> apontando para `/opt/hyperledger/ccaas_builder` (path de dentro da imagem Docker oficial, não
> existe em peer nativo) — é pra **substituir** o `path` dessa entrada existente, não adicionar uma
> segunda (Seção 4).
>
> **Revisão 8**: bug real — `cryptogen` não interpreta o prefixo `IP:` nos valores de `SANS`. Ele
> testa se a string é um IP válido (`net.ParseIP`); com o prefixo, a string deixa de ser parseável
> e vira um SAN tipo **DNS** com o texto literal `IP:203.0.113.11` em vez de um SAN tipo **IP
> Address** de verdade (confirmado no output real: `DNS:IP:203.0.113.11`). Removido o prefixo —
> `SANS` agora leva o IP puro (Seção 2).
>
> **Revisão 7**: bug real — `${DOMAIN}` dentro de arquivos YAML (`crypto-config.yaml`,
> `configtx.yaml`) e do `peer.env` (lido pelo `systemd` via `EnvironmentFile=`) **não é expandido**
> por shell; `cryptogen`/`configtxgen` leem o YAML literalmente, e `systemd` não faz expansão de
> variável nesse tipo de arquivo. Substituído por `aegischain.local` literal nesses três lugares.
> `${DOMAIN}` continua correto nos comandos `bash` de fato executados no terminal (`scp`, `openssl`,
> `export CORE_PEER_...`), onde a expansão de shell realmente acontece.
>
> **Revisão 6**: material criptográfico e `configtx.yaml`/`configtxgen` agora ancorados no
> diretório local real (`$AEGISCHAIN_REPO/aegischain-network/multihost-4org`, Arch
> Linux), usando os binários já existentes em `fabric-samples/bin` local via `PATH` — inclui export
> manual das variáveis da Seção 0, já que o `/etc/profile.d` só existe nas 4 VPS (Seção 2 e 3).
>
> **Revisão 5**: adicionado `chown fabric:fabric` em `core.yaml`/`orderer.yaml` logo após o `cp`
> (Seção 1) — o `sudo cp` original deixava os dois com dono `root:root`, inofensivo por permissão
> `644`, mas inconsistente com o resto de `/opt/fabric`.
>
> **Revisão 4**: Docker instalado em modo rootless (não grupo `docker`) — adicionado
> `loginctl enable-linger`, nota sobre `DOCKER_HOST` e consistência de usuário entre quem sobe os
> containers CCaaS e quem instalou o rootless (Seção 1).
>
> **Revisão 3**: corrigido o caminho de instalação dos binários — `install-fabric.sh` coloca tudo
> dentro de `fabric-samples/bin` e `fabric-samples/config`, não em `/tmp/bin`/`/tmp/config` soltos
> (Seção 1, confirmado por teste real numa das VPS).
>
> **Revisão 2**: variáveis de ambiente movidas de `.bashrc` para `/etc/profile.d/` (Seção 0);
> esclarecido que `install-fabric.sh` baixa tudo relativo ao diretório de execução, sem `cd`
> faltando (Seção 1); corrigido `cp -r` do builder CCaaS que gerava subpasta `ccaas/bin` em vez de
> `bin` direto (Seção 1 e 5).

## 0. Topologia e IPs reais

| Papel | Org (MSP) | VPS / IP | Orderer? | Peer? |
|---|---|---|---|---|
| Hospital A | `HospitalAMSP` | 203.0.113.11 | ✅ orderer1 | ✅ peer0 |
| Hospital B | `HospitalBMSP` | 203.0.113.12   | ✅ orderer2 | ✅ peer0 |
| Hospital C | `HospitalCMSP` | 203.0.113.13  | ✅ orderer3 | ✅ peer0 |
| Clínica 1  | `Clinica1MSP`  | 203.0.113.14    | ❌ | ✅ peer0 |

3 orderers na mesma MSP (`OrdererMSP`), Raft com quórum ímpar, fisicamente um por VPS de hospital.

Estado atual confirmado: Ubuntu 24.04 limpo, sem Docker/Go/Fabric em nenhuma das 4. Isso muda o
passo 1 — vamos partir de zero mesmo nos pré-requisitos de SO.

Essas variáveis são iguais nas 4 VPS (descrevem a topologia inteira, não algo por máquina) — cole
o mesmo bloco nas 4. Use `/etc/profile.d/`, não `.bashrc` de um usuário específico: ao longo do
processo você vai rodar comandos como `root`, como seu usuário SSH e como `fabric`, e um arquivo em
`/etc/profile.d/` é carregado por qualquer sessão de login de qualquer usuário.

```bash
sudo tee /etc/profile.d/aegischain-net.sh <<'EOF'
export HOSPITAL_A_IP=203.0.113.11
export HOSPITAL_B_IP=203.0.113.12
export HOSPITAL_C_IP=203.0.113.13
export CLINICA1_IP=203.0.113.14
export FABRIC_VERSION=2.5.16
export CHANNEL_NAME=aegischain
export DOMAIN=aegischain.local
EOF
source /etc/profile.d/aegischain-net.sh   # aplica na sessão atual; novas sessões carregam sozinhas
```

Portas por VPS:
- `7050` orderer (só nas 3 de hospital)
- `7051` peer
- `9443` orderer admin/operations — Channel Participation API (`osnadmin`)
- `9444` peer operations/health (opcional)
- `9991`/`9992`/`9993` containers CCaaS (ownership/consent/logging)

---

## 1. Pré-requisitos de SO (rodar nas 4 VPS — Ubuntu 24.04 limpo)

O peer/orderer nativos **não precisam de Go instalado** — `install-fabric.sh` baixa binários
pré-compilados. Go só seria necessário se você fosse compilar o Fabric a partir do código-fonte,
o que não é o caso aqui. Docker é necessário **só** para os containers CCaaS dos chaincodes — o
processo do peer nunca toca no `docker.sock` nesta arquitetura (é exatamente isso que elimina a
classe de bug da Seção 8.5.4 do seu documento anterior).

```bash
sudo apt update && sudo apt upgrade -y
sudo apt install -y curl wget jq tar git ca-certificates gnupg lsb-release ufw

# Docker (só para os containers CCaaS — não para peer/orderer)
# Instalado em modo ROOTLESS (evita colocar o usuário no grupo docker, equivalente a root):
curl -fsSL https://get.docker.com | sudo sh
sudo apt-get install -y uidmap
dockerd-rootless-setuptool.sh install
sudo loginctl enable-linger $(whoami)   # mantém o dockerd rootless vivo sem sessão SSH ativa
# confirme: `docker context ls` mostra "rootless" como atual; `docker info` sem erro de conexão
# DOCKER_HOST=unix:///run/user/$(id -u)/docker.sock deve estar no seu .bashrc/.profile
#
# Atenção: qualquer script/systemd que rodar `docker run` para os containers CCaaS precisa
# rodar como o MESMO usuário que fez o dockerd-rootless-setuptool.sh (ou ter DOCKER_HOST
# apontando explicitamente para o socket desse usuário) — senão o CLI tenta o socket padrão
# /var/run/docker.sock e falha. A publicação de porta (-p 9991:9991) funciona normalmente
# via rootlesskit, sem mudança no connection.json do CCaaS (Seção 5).

# usuário e diretórios do Fabric nativo
sudo useradd -m -s /bin/bash fabric || true
sudo mkdir -p /opt/fabric/{bin,config,crypto,channel-artifacts,production,ccaas-builder}
sudo chown -R fabric:fabric /opt/fabric
sudo -u fabric mkdir -p /opt/fabric/production/orderer
```

Instalar os binários Fabric (rodar como o próprio usuário, ou ajustando permissões depois). O
script clona `fabric-samples` no diretório atual e instala os binários/config **dentro dela**
(`fabric-samples/bin` e `fabric-samples/config`) — não em `bin`/`config` soltos ao lado. Não falta
nenhum `cd` entre os dois blocos abaixo; o segundo bloco só referencia esses caminhos com o prefixo
absoluto `/tmp/fabric-samples/...`:

```bash
cd /tmp
curl -sSL https://raw.githubusercontent.com/hyperledger/fabric/main/scripts/install-fabric.sh -o install-fabric.sh
chmod +x install-fabric.sh
./install-fabric.sh --fabric-version ${FABRIC_VERSION} binary samples
ls /tmp/fabric-samples/bin /tmp/fabric-samples/builders/ccaas   # confirma que os dois existem antes de seguir

sudo cp /tmp/fabric-samples/bin/* /opt/fabric/bin/
sudo cp /tmp/fabric-samples/config/core.yaml /tmp/fabric-samples/config/orderer.yaml /opt/fabric/config/
sudo chown fabric:fabric /opt/fabric/config/core.yaml /opt/fabric/config/orderer.yaml
sudo ln -sf /opt/fabric/bin/* /usr/local/bin/

peer version    # confirma 2.5.16
orderer version
```

O componente `samples` (clone do `fabric-samples`) é o que traz o builder CCaaS **oficial e já
pronto** (`builders/ccaas`) — é exatamente essa a peça que vamos usar no lugar de escrever
`detect`/`build`/`release` à mão, evitando o bug de path (`chaincode/server/connection.json`) que
consumiu tempo na investigação anterior.

**Atenção ao `cp -r` aqui** — o passo 1 já criou `/opt/fabric/ccaas-builder` vazio (no bloco de
`mkdir -p` acima). Quando o destino de `cp -r origem/pasta destino/` já existe como diretório, o
`cp` copia `pasta` **para dentro** dele em vez de usar o destino como o próprio conteúdo — o
resultado seria `/opt/fabric/ccaas-builder/ccaas/bin/...` (errado) em vez de
`/opt/fabric/ccaas-builder/bin/...` (o que o `core.yaml` do passo 4 espera). O `/.` no final do
caminho de origem evita isso, copiando o *conteúdo* da pasta:

```bash
sudo cp -r /tmp/fabric-samples/builders/ccaas/. /opt/fabric/ccaas-builder/
sudo chown -R fabric:fabric /opt/fabric/ccaas-builder
ls /opt/fabric/ccaas-builder/bin   # detect, build, release — direto aqui, sem subpasta ccaas/
```

---

## 2. Material criptográfico (gerado localmente — Arch Linux, `$AEGISCHAIN_REPO`)

Gerado na estação local, **não** em nenhuma das 4 VPS — motivo: é a máquina que vai reter a chave
privada da CA de cada organização (o material mais sensível de todo o setup), e nenhuma VPS deveria
enxergar o `crypto-config/` de todas as orgs de uma vez, só o material da própria.

Diretório de trabalho (recriado do zero, sem reaproveitar o `multihost-4org` anterior):
```bash
mkdir -p $AEGISCHAIN_REPO/aegischain-network/multihost-4org
cd $AEGISCHAIN_REPO/aegischain-network/multihost-4org

export PATH=$FABRIC_SAMPLES/bin:$PATH
cryptogen version    # confirma que o binário local está acessível

# as variáveis da Seção 0 foram exportadas via /etc/profile.d só nas 4 VPS — na sua máquina local
# exporte as mesmas manualmente (ou salve num .envrc/arquivo próprio do projeto):
export HOSPITAL_A_IP=203.0.113.11
export HOSPITAL_B_IP=203.0.113.12
export HOSPITAL_C_IP=203.0.113.13
export CLINICA1_IP=203.0.113.14
export FABRIC_VERSION=2.5.16
export CHANNEL_NAME=aegischain
export DOMAIN=aegischain.local
```

`crypto-config.yaml`:
```bash
tee crypto-config.yaml <<'EOF'
OrdererOrgs:
  - Name: OrdererOrg
    Domain: orderer.aegischain.local
    EnableNodeOUs: true
    Specs:
      - Hostname: orderer1
        SANS: ["203.0.113.11"]
      - Hostname: orderer2
        SANS: ["203.0.113.12"]
      - Hostname: orderer3
        SANS: ["203.0.113.13"]

PeerOrgs:
  - Name: HospitalA
    Domain: hospitala.aegischain.local
    EnableNodeOUs: true
    Template: {Count: 1, SANS: ["203.0.113.11"]}
    Users: {Count: 1}

  - Name: HospitalB
    Domain: hospitalb.aegischain.local
    EnableNodeOUs: true
    Template: {Count: 1, SANS: ["203.0.113.12"]}
    Users: {Count: 1}

  - Name: HospitalC
    Domain: hospitalc.aegischain.local
    EnableNodeOUs: true
    Template: {Count: 1, SANS: ["203.0.113.13"]}
    Users: {Count: 1}

  - Name: Clinica1
    Domain: clinica1.aegischain.local
    EnableNodeOUs: true
    Template: {Count: 1, SANS: ["203.0.113.14"]}
    Users: {Count: 1}
EOF
```

```bash
cd $AEGISCHAIN_REPO/aegischain-network/multihost-4org
cryptogen generate --config=crypto-config.yaml --output=crypto-config

# confira os SANs antes de distribuir
openssl x509 -in crypto-config/peerOrganizations/hospitala.${DOMAIN}/peers/peer0.hospitala.${DOMAIN}/tls/server.crt -noout -text | grep -A1 "Subject Alternative Name"
```

Distribua via `scp`, a partir de `$AEGISCHAIN_REPO/aegischain-network/multihost-4org`,
só o que cada VPS precisa (org própria + a MSP do orderer, que todas usam para verificar
assinaturas):

```bash
# Hospital A (peer + orderer1)
scp -r crypto-config/peerOrganizations/hospitala.${DOMAIN} fabric@${HOSPITAL_A_IP}:/opt/fabric/crypto/peer
scp -r crypto-config/ordererOrganizations/orderer.${DOMAIN}/orderers/orderer1.orderer.${DOMAIN} fabric@${HOSPITAL_A_IP}:/opt/fabric/crypto/orderer
scp -r crypto-config/ordererOrganizations/orderer.${DOMAIN}/msp fabric@${HOSPITAL_A_IP}:/opt/fabric/crypto/orderer-msp

# análogo para Hospital B → orderer2, Hospital C → orderer3
# Clínica 1 recebe só a pasta de peer (sem orderer):
scp -r crypto-config/peerOrganizations/clinica1.${DOMAIN} fabric@${CLINICA1_IP}:/opt/fabric/crypto/peer
```

Guarde a pasta `crypto-config/` inteira localmente — o `configtxgen` do próximo passo precisa
enxergar todas as 4 orgs + a orderer org de uma vez.

---

## 3. `configtx.yaml` (canal de aplicação, sem system channel)

```bash
tee configtx.yaml <<'EOF'
Organizations:
  - &OrdererOrg
    Name: OrdererOrg
    ID: OrdererMSP
    MSPDir: crypto-config/ordererOrganizations/orderer.aegischain.local/msp
    Policies:
      Readers: {Type: Signature, Rule: "OR('OrdererMSP.member')"}
      Writers: {Type: Signature, Rule: "OR('OrdererMSP.member')"}
      Admins:  {Type: Signature, Rule: "OR('OrdererMSP.admin')"}
    OrdererEndpoints:
      - 203.0.113.11:7050
      - 203.0.113.12:7050
      - 203.0.113.13:7050

  - &HospitalA
    Name: HospitalAMSP
    ID: HospitalAMSP
    MSPDir: crypto-config/peerOrganizations/hospitala.aegischain.local/msp
    Policies:
      Readers: {Type: Signature, Rule: "OR('HospitalAMSP.admin','HospitalAMSP.peer','HospitalAMSP.client')"}
      Writers: {Type: Signature, Rule: "OR('HospitalAMSP.admin','HospitalAMSP.client')"}
      Admins:  {Type: Signature, Rule: "OR('HospitalAMSP.admin')"}
      Endorsement: {Type: Signature, Rule: "OR('HospitalAMSP.peer')"}
    AnchorPeers: [{Host: 203.0.113.11, Port: 7051}]

  - &HospitalB
    Name: HospitalBMSP
    ID: HospitalBMSP
    MSPDir: crypto-config/peerOrganizations/hospitalb.aegischain.local/msp
    Policies:
      Readers: {Type: Signature, Rule: "OR('HospitalBMSP.admin','HospitalBMSP.peer','HospitalBMSP.client')"}
      Writers: {Type: Signature, Rule: "OR('HospitalBMSP.admin','HospitalBMSP.client')"}
      Admins:  {Type: Signature, Rule: "OR('HospitalBMSP.admin')"}
      Endorsement: {Type: Signature, Rule: "OR('HospitalBMSP.peer')"}
    AnchorPeers: [{Host: 203.0.113.12, Port: 7051}]

  - &HospitalC
    Name: HospitalCMSP
    ID: HospitalCMSP
    MSPDir: crypto-config/peerOrganizations/hospitalc.aegischain.local/msp
    Policies:
      Readers: {Type: Signature, Rule: "OR('HospitalCMSP.admin','HospitalCMSP.peer','HospitalCMSP.client')"}
      Writers: {Type: Signature, Rule: "OR('HospitalCMSP.admin','HospitalCMSP.client')"}
      Admins:  {Type: Signature, Rule: "OR('HospitalCMSP.admin')"}
      Endorsement: {Type: Signature, Rule: "OR('HospitalCMSP.peer')"}
    AnchorPeers: [{Host: 203.0.113.13, Port: 7051}]

  - &Clinica1
    Name: Clinica1MSP
    ID: Clinica1MSP
    MSPDir: crypto-config/peerOrganizations/clinica1.aegischain.local/msp
    Policies:
      Readers: {Type: Signature, Rule: "OR('Clinica1MSP.admin','Clinica1MSP.peer','Clinica1MSP.client')"}
      Writers: {Type: Signature, Rule: "OR('Clinica1MSP.admin','Clinica1MSP.client')"}
      Admins:  {Type: Signature, Rule: "OR('Clinica1MSP.admin')"}
      Endorsement: {Type: Signature, Rule: "OR('Clinica1MSP.peer')"}
    AnchorPeers: [{Host: 203.0.113.14, Port: 7051}]

Capabilities:
  Channel: &ChannelCapabilities {V2_0: true}
  Orderer: &OrdererCapabilities {V2_0: true}
  Application: &ApplicationCapabilities {V2_5: true}

Application: &ApplicationDefaults
  Organizations:
  Policies:
    Readers: {Type: ImplicitMeta, Rule: "ANY Readers"}
    Writers: {Type: ImplicitMeta, Rule: "ANY Writers"}
    Admins:  {Type: ImplicitMeta, Rule: "MAJORITY Admins"}
    LifecycleEndorsement: {Type: ImplicitMeta, Rule: "MAJORITY Endorsement"}
    Endorsement: {Type: ImplicitMeta, Rule: "MAJORITY Endorsement"}
  Capabilities: {<<: *ApplicationCapabilities}

Orderer: &OrdererDefaults
  OrdererType: etcdraft
  Addresses:
    - 203.0.113.11:7050
    - 203.0.113.12:7050
    - 203.0.113.13:7050
  EtcdRaft:
    Consenters:
      - Host: 203.0.113.11
        Port: 7050
        ClientTLSCert: crypto-config/ordererOrganizations/orderer.aegischain.local/orderers/orderer1.orderer.aegischain.local/tls/server.crt
        ServerTLSCert: crypto-config/ordererOrganizations/orderer.aegischain.local/orderers/orderer1.orderer.aegischain.local/tls/server.crt
      - Host: 203.0.113.12
        Port: 7050
        ClientTLSCert: crypto-config/ordererOrganizations/orderer.aegischain.local/orderers/orderer2.orderer.aegischain.local/tls/server.crt
        ServerTLSCert: crypto-config/ordererOrganizations/orderer.aegischain.local/orderers/orderer2.orderer.aegischain.local/tls/server.crt
      - Host: 203.0.113.13
        Port: 7050
        ClientTLSCert: crypto-config/ordererOrganizations/orderer.aegischain.local/orderers/orderer3.orderer.aegischain.local/tls/server.crt
        ServerTLSCert: crypto-config/ordererOrganizations/orderer.aegischain.local/orderers/orderer3.orderer.aegischain.local/tls/server.crt
  BatchTimeout: 200ms
  BatchSize:
    MaxMessageCount: 100
    AbsoluteMaxBytes: 10 MB
    PreferredMaxBytes: 2 MB
  Organizations: [*OrdererOrg]
  Policies:
    Readers: {Type: ImplicitMeta, Rule: "ANY Readers"}
    Writers: {Type: ImplicitMeta, Rule: "ANY Writers"}
    Admins:  {Type: ImplicitMeta, Rule: "MAJORITY Admins"}
    BlockValidation: {Type: ImplicitMeta, Rule: "ANY Writers"}
  Capabilities: {<<: *OrdererCapabilities}

Channel: &ChannelDefaults
  Policies:
    Readers: {Type: ImplicitMeta, Rule: "ANY Readers"}
    Writers: {Type: ImplicitMeta, Rule: "ANY Writers"}
    Admins:  {Type: ImplicitMeta, Rule: "MAJORITY Admins"}
  Capabilities: {<<: *ChannelCapabilities}

Profiles:
  AegisApplicationGenesis:
    <<: *ChannelDefaults
    Orderer: {<<: *OrdererDefaults}
    Application:
      <<: *ApplicationDefaults
      Organizations: [*HospitalA, *HospitalB, *HospitalC, *Clinica1]
EOF
```

```bash
cd $AEGISCHAIN_REPO/aegischain-network/multihost-4org
mkdir -p channel-artifacts
export FABRIC_CFG_PATH=$(pwd)
configtxgen -profile AegisApplicationGenesis -channelID ${CHANNEL_NAME} \
  -outputBlock ./channel-artifacts/${CHANNEL_NAME}.block
```

Distribua `${CHANNEL_NAME}.block` só para as 3 VPS de hospital (join do orderer via
`osnadmin`) e depois para as 4 (join do peer via `peer channel join`).

---

## 4. `core.yaml` do peer (ajustar em cada VPS, a partir do `config/core.yaml` baixado)

Edite `/opt/fabric/config/core.yaml` (mantendo a estrutura oficial, só sobrescrevendo os campos
abaixo — a maioria dos operadores prefere sobrescrever via variáveis de ambiente `CORE_*` no
`peer.env`, deixando o `core.yaml` como veio, exceto o `externalBuilders`):

**Atenção**: o `core.yaml` que veio do `fabric-samples` já tem uma entrada `externalBuilders` por
padrão, mas apontando para `/opt/hyperledger/ccaas_builder` — esse é o path de dentro da **imagem
Docker oficial** do peer, não existe no filesystem de uma VPS com peer nativo. Substitua o `path`
dessa entrada (não adicione uma segunda) para o builder que você copiou no passo 1:

```yaml
chaincode:
  externalBuilders:
    - name: ccaas_builder
      path: /opt/fabric/ccaas-builder
      propagateEnvironment:
        - CHAINCODE_AS_A_SERVICE_BUILDER_CONFIG
```

Confirme com `grep -A5 "externalBuilders:" /opt/fabric/config/core.yaml` — deve aparecer só essa
uma entrada, com `path: /opt/fabric/ccaas-builder`.

Todo o resto (`peer.id`, `peer.address`, TLS, MSP, gossip) fica em variáveis de ambiente no
`peer.env` do systemd (passo 6) — é o padrão que o próprio `install-fabric.sh`/`core.yaml` usa,
evita duplicar a mesma informação em dois lugares.

---

## 5. Builder CCaaS — usar o oficial, não escrever na mão

Diferente da tentativa anterior (Seção 8.5.7 do seu documento antigo), aqui **não escrevemos**
`detect`/`build`/`release`: usamos o builder oficial (`fabric-samples/builders/ccaas`) já copiado
para `/opt/fabric/ccaas-builder` no passo 1 (atenção ao `cp -r .../. destino/` — veja a nota lá).
Ele já resolve corretamente o
`connection.json` em `chaincode/server/connection.json` — esse era exatamente o bug custoso da
investigação anterior, e a doc oficial confirma esse é o comportamento correto do `bin/release`.

`connection.json` de cada chaincode, empacotado dentro do pacote CCaaS instalado (`localhost:PORT`,
porque o container Docker do chaincode expõe a porta no host e o peer é processo nativo no mesmo
host, então nunca atravessa rede entre VPS):
```bash
tee connection.json <<'EOF'
{
  "address": "localhost:9991",
  "dial_timeout": "10s",
  "tls_required": false
}
EOF
```

Suba os containers CCaaS normalmente em cada VPS (isso não muda em relação ao que você já validou
nas Seções 8.5.5/8.5.6):
```bash
docker run --rm -d --name aegis-ownership-ccaas -p 9991:9991 \
  -e CHAINCODE_SERVER_ADDRESS=0.0.0.0:9991 \
  -e CORE_CHAINCODE_ID_NAME=<package-id-apos-install> \
  aegis-ownership-ccaas:latest
```

---

## 6. systemd

`/etc/systemd/system/fabric-orderer.service` (só nas 3 VPS de hospital):
```bash
sudo tee /etc/systemd/system/fabric-orderer.service <<'EOF'
[Unit]
Description=Hyperledger Fabric Orderer
After=network.target

[Service]
Type=simple
User=fabric
WorkingDirectory=/opt/fabric
EnvironmentFile=/opt/fabric/orderer.env
ExecStart=/opt/fabric/bin/orderer
Restart=on-failure
RestartSec=5
LimitNOFILE=65536

[Install]
WantedBy=multi-user.target
EOF
```

`/opt/fabric/orderer.env` — **idêntico nas 3 VPS de hospital**, sem adaptação por nó. Isso porque
`ORDERER_GENERAL_LISTENADDRESS=0.0.0.0` não é IP específico, e o material criptográfico de cada
orderer fica sempre no mesmo caminho local (`/opt/fabric/crypto/orderer/{msp,tls}`, "o material do
próprio nó"). A diferenciação entre orderer1/2/3 já está resolvida no `configtx.yaml` (via
`EtcdRaft.Consenters`, que lista os 3 IPs), não neste arquivo:
```bash
sudo -u fabric tee /opt/fabric/orderer.env <<'EOF'
FABRIC_CFG_PATH=/opt/fabric/config
ORDERER_GENERAL_LISTENADDRESS=0.0.0.0
ORDERER_GENERAL_LISTENPORT=7050
ORDERER_GENERAL_LOCALMSPID=OrdererMSP
ORDERER_GENERAL_LOCALMSPDIR=/opt/fabric/crypto/orderer/msp
ORDERER_GENERAL_TLS_ENABLED=true
ORDERER_GENERAL_TLS_PRIVATEKEY=/opt/fabric/crypto/orderer/tls/server.key
ORDERER_GENERAL_TLS_CERTIFICATE=/opt/fabric/crypto/orderer/tls/server.crt
ORDERER_GENERAL_TLS_ROOTCAS=/opt/fabric/crypto/orderer/tls/ca.crt
ORDERER_GENERAL_CLUSTER_CLIENTCERTIFICATE=/opt/fabric/crypto/orderer/tls/server.crt
ORDERER_GENERAL_CLUSTER_CLIENTPRIVATEKEY=/opt/fabric/crypto/orderer/tls/server.key
ORDERER_GENERAL_CLUSTER_ROOTCAS=/opt/fabric/crypto/orderer/tls/ca.crt
ORDERER_GENERAL_BOOTSTRAPMETHOD=none
ORDERER_CHANNELPARTICIPATION_ENABLED=true
ORDERER_ADMIN_TLS_ENABLED=true
ORDERER_ADMIN_TLS_CERTIFICATE=/opt/fabric/crypto/orderer/tls/server.crt
ORDERER_ADMIN_TLS_PRIVATEKEY=/opt/fabric/crypto/orderer/tls/server.key
ORDERER_ADMIN_TLS_ROOTCAS=/opt/fabric/crypto/orderer/tls/ca.crt
ORDERER_ADMIN_TLS_CLIENTROOTCAS=/opt/fabric/crypto/orderer/tls/ca.crt
ORDERER_ADMIN_TLS_CLIENTAUTHREQUIRED=true
ORDERER_ADMIN_LISTENADDRESS=0.0.0.0:9443
ORDERER_OPERATIONS_LISTENADDRESS=0.0.0.0:8443
ORDERER_FILELEDGER_LOCATION=/opt/fabric/production/orderer
ORDERER_CONSENSUS_WALDIR=/opt/fabric/production/orderer/etcdraft/wal
ORDERER_CONSENSUS_SNAPDIR=/opt/fabric/production/orderer/etcdraft/snapshot
EOF
```

### 6.1 CouchDB (nas 4 VPS, antes de subir o peer pela primeira vez)

O `CouchDB` habilita rich queries (`GetQueryResult`/`ExecuteQuery`) — necessário pro
`aegis-logging` (`QueryLogsByPatient`) e recomendado desde o início, para não precisar migrar
depois com o peer já em produção (o que exige apagar e reconstruir o `statedb` a partir dos
blocos). Cada VPS tem sua própria instância de CouchDB isolada — não precisa ser a mesma senha
entre VPS, só precisa ser a mesma senha dentro de cada VPS entre o container e o `peer.env`.

```bash
docker run -d --name couchdb --restart unless-stopped \
  -p 127.0.0.1:5984:5984 \
  -e COUCHDB_USER=admin \
  -e COUCHDB_PASSWORD=<senha-real-desta-VPS> \
  couchdb:3.3

curl -s http://admin:<senha-real-desta-VPS>@127.0.0.1:5984/_up   # confirma {"status":"ok",...}
```

As variáveis `CORE_LEDGER_STATE_*` correspondentes já estão incluídas nos 4 `peer.env` abaixo —
troque `<senha-real-desta-VPS>` pela senha real escolhida em cada VPS, igual no `docker run` acima.

`/etc/systemd/system/fabric-peer.service` (nas 4 VPS):
```bash
sudo tee /etc/systemd/system/fabric-peer.service <<'EOF'
[Unit]
Description=Hyperledger Fabric Peer
After=network.target docker.service

[Service]
Type=simple
User=fabric
WorkingDirectory=/opt/fabric
EnvironmentFile=/opt/fabric/peer.env
ExecStart=/opt/fabric/bin/peer node start
Restart=on-failure
RestartSec=5
LimitNOFILE=65536

[Install]
WantedBy=multi-user.target
EOF
```

`/opt/fabric/peer.env` — muda por VPS (IP próprio, MSP ID da org, hostname). Estrutura de crypto
confirmada em campo: identidade do peer sempre em
`/opt/fabric/crypto/peer/peers/peer0.<org>.aegischain.local/{tls,msp}` nas 4 VPS.

**Hospital A (203.0.113.11):**
```bash
sudo -u fabric tee /opt/fabric/peer.env <<'EOF'
FABRIC_CFG_PATH=/opt/fabric/config
CORE_PEER_ID=peer0.hospitala.aegischain.local
CORE_PEER_ADDRESS=203.0.113.11:7051
CORE_PEER_LISTENADDRESS=0.0.0.0:7051
CORE_PEER_CHAINCODEADDRESS=203.0.113.11:7052
CORE_PEER_CHAINCODELISTENADDRESS=0.0.0.0:7052
CORE_PEER_GOSSIP_BOOTSTRAP=203.0.113.11:7051
CORE_PEER_GOSSIP_ENDPOINT=203.0.113.11:7051
CORE_PEER_GOSSIP_EXTERNALENDPOINT=203.0.113.11:7051
CORE_PEER_LOCALMSPID=HospitalAMSP
CORE_PEER_MSPCONFIGPATH=/opt/fabric/crypto/peer/peers/peer0.hospitala.aegischain.local/msp
CORE_PEER_TLS_ENABLED=true
CORE_PEER_TLS_CERT_FILE=/opt/fabric/crypto/peer/peers/peer0.hospitala.aegischain.local/tls/server.crt
CORE_PEER_TLS_KEY_FILE=/opt/fabric/crypto/peer/peers/peer0.hospitala.aegischain.local/tls/server.key
CORE_PEER_TLS_ROOTCERT_FILE=/opt/fabric/crypto/peer/peers/peer0.hospitala.aegischain.local/tls/ca.crt
CORE_PEER_FILESYSTEMPATH=/opt/fabric/production
CORE_LEDGER_SNAPSHOTS_ROOTDIR=/opt/fabric/production/snapshots
CORE_LEDGER_STATE_STATEDATABASE=CouchDB
CORE_LEDGER_STATE_COUCHDBCONFIG_COUCHDBADDRESS=127.0.0.1:5984
CORE_LEDGER_STATE_COUCHDBCONFIG_USERNAME=admin
CORE_LEDGER_STATE_COUCHDBCONFIG_PASSWORD=<senha-real-desta-VPS>
CORE_OPERATIONS_LISTENADDRESS=0.0.0.0:9444
EOF
```

**Hospital B (203.0.113.12):**
```bash
sudo -u fabric tee /opt/fabric/peer.env <<'EOF'
FABRIC_CFG_PATH=/opt/fabric/config
CORE_PEER_ID=peer0.hospitalb.aegischain.local
CORE_PEER_ADDRESS=203.0.113.12:7051
CORE_PEER_LISTENADDRESS=0.0.0.0:7051
CORE_PEER_CHAINCODEADDRESS=203.0.113.12:7052
CORE_PEER_CHAINCODELISTENADDRESS=0.0.0.0:7052
CORE_PEER_GOSSIP_BOOTSTRAP=203.0.113.12:7051
CORE_PEER_GOSSIP_ENDPOINT=203.0.113.12:7051
CORE_PEER_GOSSIP_EXTERNALENDPOINT=203.0.113.12:7051
CORE_PEER_LOCALMSPID=HospitalBMSP
CORE_PEER_MSPCONFIGPATH=/opt/fabric/crypto/peer/peers/peer0.hospitalb.aegischain.local/msp
CORE_PEER_TLS_ENABLED=true
CORE_PEER_TLS_CERT_FILE=/opt/fabric/crypto/peer/peers/peer0.hospitalb.aegischain.local/tls/server.crt
CORE_PEER_TLS_KEY_FILE=/opt/fabric/crypto/peer/peers/peer0.hospitalb.aegischain.local/tls/server.key
CORE_PEER_TLS_ROOTCERT_FILE=/opt/fabric/crypto/peer/peers/peer0.hospitalb.aegischain.local/tls/ca.crt
CORE_PEER_FILESYSTEMPATH=/opt/fabric/production
CORE_LEDGER_SNAPSHOTS_ROOTDIR=/opt/fabric/production/snapshots
CORE_LEDGER_STATE_STATEDATABASE=CouchDB
CORE_LEDGER_STATE_COUCHDBCONFIG_COUCHDBADDRESS=127.0.0.1:5984
CORE_LEDGER_STATE_COUCHDBCONFIG_USERNAME=admin
CORE_LEDGER_STATE_COUCHDBCONFIG_PASSWORD=<senha-real-desta-VPS>
CORE_OPERATIONS_LISTENADDRESS=0.0.0.0:9444
EOF
```

**Hospital C (203.0.113.13):**
```bash
sudo -u fabric tee /opt/fabric/peer.env <<'EOF'
FABRIC_CFG_PATH=/opt/fabric/config
CORE_PEER_ID=peer0.hospitalc.aegischain.local
CORE_PEER_ADDRESS=203.0.113.13:7051
CORE_PEER_LISTENADDRESS=0.0.0.0:7051
CORE_PEER_CHAINCODEADDRESS=203.0.113.13:7052
CORE_PEER_CHAINCODELISTENADDRESS=0.0.0.0:7052
CORE_PEER_GOSSIP_BOOTSTRAP=203.0.113.13:7051
CORE_PEER_GOSSIP_ENDPOINT=203.0.113.13:7051
CORE_PEER_GOSSIP_EXTERNALENDPOINT=203.0.113.13:7051
CORE_PEER_LOCALMSPID=HospitalCMSP
CORE_PEER_MSPCONFIGPATH=/opt/fabric/crypto/peer/peers/peer0.hospitalc.aegischain.local/msp
CORE_PEER_TLS_ENABLED=true
CORE_PEER_TLS_CERT_FILE=/opt/fabric/crypto/peer/peers/peer0.hospitalc.aegischain.local/tls/server.crt
CORE_PEER_TLS_KEY_FILE=/opt/fabric/crypto/peer/peers/peer0.hospitalc.aegischain.local/tls/server.key
CORE_PEER_TLS_ROOTCERT_FILE=/opt/fabric/crypto/peer/peers/peer0.hospitalc.aegischain.local/tls/ca.crt
CORE_PEER_FILESYSTEMPATH=/opt/fabric/production
CORE_LEDGER_SNAPSHOTS_ROOTDIR=/opt/fabric/production/snapshots
CORE_LEDGER_STATE_STATEDATABASE=CouchDB
CORE_LEDGER_STATE_COUCHDBCONFIG_COUCHDBADDRESS=127.0.0.1:5984
CORE_LEDGER_STATE_COUCHDBCONFIG_USERNAME=admin
CORE_LEDGER_STATE_COUCHDBCONFIG_PASSWORD=<senha-real-desta-VPS>
CORE_OPERATIONS_LISTENADDRESS=0.0.0.0:9444
EOF
```

**Clínica 1 (203.0.113.14):**
```bash
sudo -u fabric tee /opt/fabric/peer.env <<'EOF'
FABRIC_CFG_PATH=/opt/fabric/config
CORE_PEER_ID=peer0.clinica1.aegischain.local
CORE_PEER_ADDRESS=203.0.113.14:7051
CORE_PEER_LISTENADDRESS=0.0.0.0:7051
CORE_PEER_CHAINCODEADDRESS=203.0.113.14:7052
CORE_PEER_CHAINCODELISTENADDRESS=0.0.0.0:7052
CORE_PEER_GOSSIP_BOOTSTRAP=203.0.113.14:7051
CORE_PEER_GOSSIP_ENDPOINT=203.0.113.14:7051
CORE_PEER_GOSSIP_EXTERNALENDPOINT=203.0.113.14:7051
CORE_PEER_LOCALMSPID=Clinica1MSP
CORE_PEER_MSPCONFIGPATH=/opt/fabric/crypto/peer/peers/peer0.clinica1.aegischain.local/msp
CORE_PEER_TLS_ENABLED=true
CORE_PEER_TLS_CERT_FILE=/opt/fabric/crypto/peer/peers/peer0.clinica1.aegischain.local/tls/server.crt
CORE_PEER_TLS_KEY_FILE=/opt/fabric/crypto/peer/peers/peer0.clinica1.aegischain.local/tls/server.key
CORE_PEER_TLS_ROOTCERT_FILE=/opt/fabric/crypto/peer/peers/peer0.clinica1.aegischain.local/tls/ca.crt
CORE_PEER_FILESYSTEMPATH=/opt/fabric/production
CORE_LEDGER_SNAPSHOTS_ROOTDIR=/opt/fabric/production/snapshots
CORE_LEDGER_STATE_STATEDATABASE=CouchDB
CORE_LEDGER_STATE_COUCHDBCONFIG_COUCHDBADDRESS=127.0.0.1:5984
CORE_LEDGER_STATE_COUCHDBCONFIG_USERNAME=admin
CORE_LEDGER_STATE_COUCHDBCONFIG_PASSWORD=<senha-real-desta-VPS>
CORE_OPERATIONS_LISTENADDRESS=0.0.0.0:9444
EOF
```

```bash
sudo systemctl daemon-reload
sudo systemctl enable --now fabric-orderer   # só nas 3 de hospital
sudo systemctl enable --now fabric-peer      # nas 4
journalctl -u fabric-orderer -f
journalctl -u fabric-peer -f
```

---

## 7. Formar o canal (Channel Participation API)

**Rodar como usuário `fabric`, não como `ubuntu`** — o `server.key` (chave privada TLS) tem
permissão restrita ao dono (`fabric`); rodando como `ubuntu` direto, `osnadmin`/`peer` batem em
`permission denied` ao tentar ler a chave. Use `sudo -u fabric` na frente de cada comando abaixo.

**Use o IP público no `-o`, não `localhost`** — o certificado TLS do orderer só tem SAN pro
hostname (`orderer1.orderer.aegischain.local`, `orderer1`) e pro IP público (Seção 2); `localhost`
não está no SAN, então a verificação TLS falha (`certificate is valid for ..., not localhost`)
mesmo rodando na própria máquina.

Em cada uma das 3 VPS de hospital (troque o IP pelo da própria VPS — Hospital A
`203.0.113.11`, Hospital B `203.0.113.12`, Hospital C `203.0.113.13`):
```bash
sudo -u fabric osnadmin channel join \
  --channelID aegischain \
  --config-block /opt/fabric/channel-artifacts/aegischain.block \
  -o 203.0.113.11:9443 \
  --ca-file /opt/fabric/crypto/orderer/tls/ca.crt \
  --client-cert /opt/fabric/crypto/orderer/tls/server.crt \
  --client-key /opt/fabric/crypto/orderer/tls/server.key

sudo -u fabric osnadmin channel list -o 203.0.113.11:9443 \
  --ca-file /opt/fabric/crypto/orderer/tls/ca.crt \
  --client-cert /opt/fabric/crypto/orderer/tls/server.crt \
  --client-key /opt/fabric/crypto/orderer/tls/server.key
```
O canal fica ativo assim que o primeiro orderer entra, mas só fica operacional com quórum (2 dos 3
Raft nodes) — junte os 3 antes de seguir.

Em cada uma das 4 VPS (peer) — usa a identidade do `Admin@`, não a do peer, por isso o
`CORE_PEER_MSPCONFIGPATH` aponta pra `users/Admin@...` (diferente do que está no `peer.env`, que
usa a identidade do próprio nó). **Use o IP público em `CORE_PEER_ADDRESS`, não `localhost`** —
mesmo motivo do `osnadmin` acima, o certificado do peer não tem SAN pra `localhost`. **Inclua
`FABRIC_CFG_PATH`** — o `peer` CLI precisa dele pra achar o `core.yaml`, e essa invocação manual
não herda o `peer.env` do systemd.

**Hospital A (203.0.113.11):**
```bash
export CORE_PEER_TLS_ENABLED=true
export CORE_PEER_LOCALMSPID=HospitalAMSP
export CORE_PEER_MSPCONFIGPATH=/opt/fabric/crypto/peer/users/Admin@hospitala.aegischain.local/msp
export CORE_PEER_ADDRESS=203.0.113.11:7051
export CORE_PEER_TLS_ROOTCERT_FILE=/opt/fabric/crypto/peer/peers/peer0.hospitala.aegischain.local/tls/ca.crt
export FABRIC_CFG_PATH=/opt/fabric/config

sudo -u fabric --preserve-env=CORE_PEER_TLS_ENABLED,CORE_PEER_LOCALMSPID,CORE_PEER_MSPCONFIGPATH,CORE_PEER_ADDRESS,CORE_PEER_TLS_ROOTCERT_FILE,FABRIC_CFG_PATH \
  peer channel join -b /opt/fabric/channel-artifacts/aegischain.block
sudo -u fabric --preserve-env=CORE_PEER_TLS_ENABLED,CORE_PEER_LOCALMSPID,CORE_PEER_MSPCONFIGPATH,CORE_PEER_ADDRESS,CORE_PEER_TLS_ROOTCERT_FILE,FABRIC_CFG_PATH \
  peer channel list
```

**Hospital B (203.0.113.12):**
```bash
export CORE_PEER_TLS_ENABLED=true
export CORE_PEER_LOCALMSPID=HospitalBMSP
export CORE_PEER_MSPCONFIGPATH=/opt/fabric/crypto/peer/users/Admin@hospitalb.aegischain.local/msp
export CORE_PEER_ADDRESS=203.0.113.12:7051
export CORE_PEER_TLS_ROOTCERT_FILE=/opt/fabric/crypto/peer/peers/peer0.hospitalb.aegischain.local/tls/ca.crt
export FABRIC_CFG_PATH=/opt/fabric/config

sudo -u fabric --preserve-env=CORE_PEER_TLS_ENABLED,CORE_PEER_LOCALMSPID,CORE_PEER_MSPCONFIGPATH,CORE_PEER_ADDRESS,CORE_PEER_TLS_ROOTCERT_FILE,FABRIC_CFG_PATH \
  peer channel join -b /opt/fabric/channel-artifacts/aegischain.block
sudo -u fabric --preserve-env=CORE_PEER_TLS_ENABLED,CORE_PEER_LOCALMSPID,CORE_PEER_MSPCONFIGPATH,CORE_PEER_ADDRESS,CORE_PEER_TLS_ROOTCERT_FILE,FABRIC_CFG_PATH \
  peer channel list
```

**Hospital C (203.0.113.13):**
```bash
export CORE_PEER_TLS_ENABLED=true
export CORE_PEER_LOCALMSPID=HospitalCMSP
export CORE_PEER_MSPCONFIGPATH=/opt/fabric/crypto/peer/users/Admin@hospitalc.aegischain.local/msp
export CORE_PEER_ADDRESS=203.0.113.13:7051
export CORE_PEER_TLS_ROOTCERT_FILE=/opt/fabric/crypto/peer/peers/peer0.hospitalc.aegischain.local/tls/ca.crt
export FABRIC_CFG_PATH=/opt/fabric/config

sudo -u fabric --preserve-env=CORE_PEER_TLS_ENABLED,CORE_PEER_LOCALMSPID,CORE_PEER_MSPCONFIGPATH,CORE_PEER_ADDRESS,CORE_PEER_TLS_ROOTCERT_FILE,FABRIC_CFG_PATH \
  peer channel join -b /opt/fabric/channel-artifacts/aegischain.block
sudo -u fabric --preserve-env=CORE_PEER_TLS_ENABLED,CORE_PEER_LOCALMSPID,CORE_PEER_MSPCONFIGPATH,CORE_PEER_ADDRESS,CORE_PEER_TLS_ROOTCERT_FILE,FABRIC_CFG_PATH \
  peer channel list
```

**Clínica 1 (203.0.113.14)** — lembre de copiar o `aegischain.block` pra ela antes (ainda não
recebeu, só as 3 de hospital):
```bash
# da sua máquina local
scp $AEGISCHAIN_REPO/aegischain-network/multihost-4org/channel-artifacts/aegischain.block ubuntu@203.0.113.14:/tmp/
# na Clínica 1
sudo mv /tmp/aegischain.block /opt/fabric/channel-artifacts/aegischain.block
sudo chown fabric:fabric /opt/fabric/channel-artifacts/aegischain.block
```
```bash
export CORE_PEER_TLS_ENABLED=true
export CORE_PEER_LOCALMSPID=Clinica1MSP
export CORE_PEER_MSPCONFIGPATH=/opt/fabric/crypto/peer/users/Admin@clinica1.aegischain.local/msp
export CORE_PEER_ADDRESS=203.0.113.14:7051
export CORE_PEER_TLS_ROOTCERT_FILE=/opt/fabric/crypto/peer/peers/peer0.clinica1.aegischain.local/tls/ca.crt
export FABRIC_CFG_PATH=/opt/fabric/config

sudo -u fabric --preserve-env=CORE_PEER_TLS_ENABLED,CORE_PEER_LOCALMSPID,CORE_PEER_MSPCONFIGPATH,CORE_PEER_ADDRESS,CORE_PEER_TLS_ROOTCERT_FILE,FABRIC_CFG_PATH \
  peer channel join -b /opt/fabric/channel-artifacts/aegischain.block
sudo -u fabric --preserve-env=CORE_PEER_TLS_ENABLED,CORE_PEER_LOCALMSPID,CORE_PEER_MSPCONFIGPATH,CORE_PEER_ADDRESS,CORE_PEER_TLS_ROOTCERT_FILE,FABRIC_CFG_PATH \
  peer channel list
```
`sudo -u fabric` por padrão **não propaga** as variáveis `export`adas no seu shell — por isso o
`--preserve-env` explícito com a lista exata das 6 variáveis usadas aqui.



---

## 8. Deploy dos 3 chaincodes CCaaS

Fluxo real, a partir do código-fonte de `aegis-ownership-chaincode` (Go, `shim.ChaincodeServer` puro,
lê `CHAINCODE_ID`/`CHAINCODE_SERVER_ADDRESS` do ambiente, `TLSProps.Disabled: true`). `aegis-consent`
e `aegis-logging` repetem exatamente o mesmo padrão, trocando porta (9992/9993) e nome da imagem.

### 8.1 Distribuir código-fonte e buildar a imagem (nas 4 VPS)

Na máquina local:
```bash
cd $AEGISCHAIN_REPO
tar czf /tmp/aegis-ownership-chaincode.tar.gz aegischain-ownership-chaincode

for ip in 203.0.113.11 203.0.113.12 203.0.113.13 203.0.113.14; do
  scp /tmp/aegis-ownership-chaincode.tar.gz ubuntu@${ip}:/tmp/
done
```

Nas 4 VPS (como `ubuntu`, Docker rootless já configurado nesse usuário):
```bash
mkdir -p ~/chaincodes
tar xzf /tmp/aegis-ownership-chaincode.tar.gz -C ~/chaincodes
cd ~/chaincodes/aegischain-ownership-chaincode
docker build -t aegis-ownership-ccaas:1.0 .
docker images | grep aegis-ownership
```

### 8.2 `package`/`install` (nas 4 VPS — valores reais por VPS)

Antes de distribuir o TLS CA de cada org (necessário pro `approve`/`commit` cross-org depois),
copie das VPS a CA de cada org a partir da máquina local:
```bash
cd $AEGISCHAIN_REPO/aegischain-network/multihost-4org
for ip in 203.0.113.11 203.0.113.12 203.0.113.13 203.0.113.14; do
  scp crypto-config/ordererOrganizations/orderer.aegischain.local/tlsca/tlsca.orderer.aegischain.local-cert.pem \
      ubuntu@${ip}:/tmp/orderer-tlsca.crt
  scp crypto-config/peerOrganizations/hospitala.aegischain.local/tlsca/tlsca.hospitala.aegischain.local-cert.pem \
      ubuntu@${ip}:/tmp/peer-tlsca-hospitala.crt
  scp crypto-config/peerOrganizations/hospitalb.aegischain.local/tlsca/tlsca.hospitalb.aegischain.local-cert.pem \
      ubuntu@${ip}:/tmp/peer-tlsca-hospitalb.crt
  scp crypto-config/peerOrganizations/hospitalc.aegischain.local/tlsca/tlsca.hospitalc.aegischain.local-cert.pem \
      ubuntu@${ip}:/tmp/peer-tlsca-hospitalc.crt
  scp crypto-config/peerOrganizations/clinica1.aegischain.local/tlsca/tlsca.clinica1.aegischain.local-cert.pem \
      ubuntu@${ip}:/tmp/peer-tlsca-clinica1.crt
done
```
Nas 4 VPS:
```bash
sudo mv /tmp/orderer-tlsca.crt /tmp/peer-tlsca-*.crt /opt/fabric/crypto/
sudo chown fabric:fabric /opt/fabric/crypto/orderer-tlsca.crt /opt/fabric/crypto/peer-tlsca-*.crt
```

**Nota importante**: `peer lifecycle chaincode package --lang external` tem um bug conhecido e nunca
corrigido no Fabric ([issue #3368](https://github.com/hyperledger/fabric/issues/3368)) —
`Error: failed to normalize chaincode path: unknown chaincodeType: EXTERNAL`. A doc oficial de
"Chaincode as an external service" não usa esse subcomando: monta o pacote manualmente via `tar`.
Além disso, o builder oficial que copiamos (`fabric-samples/builders/ccaas`) espera
`"type":"ccaas"` no `metadata.json` — não `"external"` (essa é a convenção própria desse builder
específico, diferente do exemplo genérico da documentação). E os arquivos ficam em `/tmp`, não em
`/home/ubuntu` — o Ubuntu 24.04 cria `/home/ubuntu` com permissão `750` (sem acesso a "outros"), e
o usuário `fabric` não está no grupo `ubuntu`, então nem consegue entrar no diretório.

**Hospital A (203.0.113.11):**
```bash
mkdir -p /tmp/ownership-ccaas-package
cat > /tmp/ownership-ccaas-package/connection.json <<'EOF'
{
  "address": "localhost:9991",
  "dial_timeout": "10s",
  "tls_required": false
}
EOF
cat > /tmp/ownership-ccaas-package/metadata.json <<'EOF'
{"type":"ccaas","label":"aegis-ownership_1.0"}
EOF

cd /tmp/ownership-ccaas-package
tar cfz code.tar.gz connection.json
tar cfz /tmp/aegis-ownership.tar.gz metadata.json code.tar.gz
chmod 644 /tmp/aegis-ownership.tar.gz

export FABRIC_CFG_PATH=/opt/fabric/config
export CORE_PEER_TLS_ENABLED=true

sudo -u fabric --preserve-env=FABRIC_CFG_PATH,CORE_PEER_TLS_ENABLED \
  bash -c '
    export CORE_PEER_MSPCONFIGPATH=/opt/fabric/crypto/peer/users/Admin@hospitala.aegischain.local/msp
    export CORE_PEER_ADDRESS=203.0.113.11:7051
    export CORE_PEER_TLS_ROOTCERT_FILE=/opt/fabric/crypto/peer/peers/peer0.hospitala.aegischain.local/tls/ca.crt
    export CORE_PEER_LOCALMSPID=HospitalAMSP
    peer lifecycle chaincode install /tmp/aegis-ownership.tar.gz
    peer lifecycle chaincode queryinstalled
  '
```

**Hospital B (203.0.113.12):**
```bash
mkdir -p /tmp/ownership-ccaas-package
cat > /tmp/ownership-ccaas-package/connection.json <<'EOF'
{
  "address": "localhost:9991",
  "dial_timeout": "10s",
  "tls_required": false
}
EOF
cat > /tmp/ownership-ccaas-package/metadata.json <<'EOF'
{"type":"ccaas","label":"aegis-ownership_1.0"}
EOF

cd /tmp/ownership-ccaas-package
tar cfz code.tar.gz connection.json
tar cfz /tmp/aegis-ownership.tar.gz metadata.json code.tar.gz
chmod 644 /tmp/aegis-ownership.tar.gz

export FABRIC_CFG_PATH=/opt/fabric/config
export CORE_PEER_TLS_ENABLED=true

sudo -u fabric --preserve-env=FABRIC_CFG_PATH,CORE_PEER_TLS_ENABLED \
  bash -c '
    export CORE_PEER_MSPCONFIGPATH=/opt/fabric/crypto/peer/users/Admin@hospitalb.aegischain.local/msp
    export CORE_PEER_ADDRESS=203.0.113.12:7051
    export CORE_PEER_TLS_ROOTCERT_FILE=/opt/fabric/crypto/peer/peers/peer0.hospitalb.aegischain.local/tls/ca.crt
    export CORE_PEER_LOCALMSPID=HospitalBMSP
    peer lifecycle chaincode install /tmp/aegis-ownership.tar.gz
    peer lifecycle chaincode queryinstalled
  '
```

**Hospital C (203.0.113.13):**
```bash
mkdir -p /tmp/ownership-ccaas-package
cat > /tmp/ownership-ccaas-package/connection.json <<'EOF'
{
  "address": "localhost:9991",
  "dial_timeout": "10s",
  "tls_required": false
}
EOF
cat > /tmp/ownership-ccaas-package/metadata.json <<'EOF'
{"type":"ccaas","label":"aegis-ownership_1.0"}
EOF

cd /tmp/ownership-ccaas-package
tar cfz code.tar.gz connection.json
tar cfz /tmp/aegis-ownership.tar.gz metadata.json code.tar.gz
chmod 644 /tmp/aegis-ownership.tar.gz

export FABRIC_CFG_PATH=/opt/fabric/config
export CORE_PEER_TLS_ENABLED=true

sudo -u fabric --preserve-env=FABRIC_CFG_PATH,CORE_PEER_TLS_ENABLED \
  bash -c '
    export CORE_PEER_MSPCONFIGPATH=/opt/fabric/crypto/peer/users/Admin@hospitalc.aegischain.local/msp
    export CORE_PEER_ADDRESS=203.0.113.13:7051
    export CORE_PEER_TLS_ROOTCERT_FILE=/opt/fabric/crypto/peer/peers/peer0.hospitalc.aegischain.local/tls/ca.crt
    export CORE_PEER_LOCALMSPID=HospitalCMSP
    peer lifecycle chaincode install /tmp/aegis-ownership.tar.gz
    peer lifecycle chaincode queryinstalled
  '
```

**Clínica 1 (203.0.113.14):**
```bash
mkdir -p /tmp/ownership-ccaas-package
cat > /tmp/ownership-ccaas-package/connection.json <<'EOF'
{
  "address": "localhost:9991",
  "dial_timeout": "10s",
  "tls_required": false
}
EOF
cat > /tmp/ownership-ccaas-package/metadata.json <<'EOF'
{"type":"ccaas","label":"aegis-ownership_1.0"}
EOF

cd /tmp/ownership-ccaas-package
tar cfz code.tar.gz connection.json
tar cfz /tmp/aegis-ownership.tar.gz metadata.json code.tar.gz
chmod 644 /tmp/aegis-ownership.tar.gz

export FABRIC_CFG_PATH=/opt/fabric/config
export CORE_PEER_TLS_ENABLED=true

sudo -u fabric --preserve-env=FABRIC_CFG_PATH,CORE_PEER_TLS_ENABLED \
  bash -c '
    export CORE_PEER_MSPCONFIGPATH=/opt/fabric/crypto/peer/users/Admin@clinica1.aegischain.local/msp
    export CORE_PEER_ADDRESS=203.0.113.14:7051
    export CORE_PEER_TLS_ROOTCERT_FILE=/opt/fabric/crypto/peer/peers/peer0.clinica1.aegischain.local/tls/ca.crt
    export CORE_PEER_LOCALMSPID=Clinica1MSP
    peer lifecycle chaincode install /tmp/aegis-ownership.tar.gz
    peer lifecycle chaincode queryinstalled
  '
```

Guarde o **Package ID** (`aegis-ownership_1.0:<hash>`) de cada VPS — o hash difere entre elas (não
é bug; depende do conteúdo exato do pacote e não precisa ser idêntico entre orgs).

### 8.3 Subir o container CCaaS (nas 4 VPS — usa o Package ID como `CHAINCODE_ID`)

O `main.go` do `aegis-ownership-chaincode` lê `CHAINCODE_ID`/`CHAINCODE_SERVER_ADDRESS` do
ambiente (`shim.ChaincodeServer`, `TLSProps.Disabled: true`) — o container precisa dessas duas
variáveis batendo exatamente com o Package ID retornado no `queryinstalled` da própria VPS.

```bash
# exemplo Hospital A — trocar CHAINCODE_ID pelo Package ID de cada VPS
docker run -d --name aegis-ownership-ccaas --restart unless-stopped \
  -p 9991:9991 \
  -e CHAINCODE_ID=aegis-ownership_1.0:<hash-da-própria-VPS> \
  -e CHAINCODE_SERVER_ADDRESS=0.0.0.0:9991 \
  aegis-ownership-ccaas:1.0
docker logs aegis-ownership-ccaas
```
Log esperado: as linhas `[DEBUG]` até `Servidor configurado, chamando Start()...`, sem
`[DEBUG] ERRO ao iniciar` — parar de imprimir depois disso, sem erro, é sinal de que está servindo
(bloqueado esperando conexão do peer).

### 8.4 `approveformyorg` (nas 4 VPS)

**Pré-requisito de firewall**: a Clínica 1 precisa da porta `7050` liberada nas 3 VPS de hospital
(ver nota no topo da Seção 9) — `approveformyorg`/`commit` falam direto com o orderer, não só com
o peer.

```bash
# exemplo Hospital A — trocar MSPID/domínio/IP/CA-file por VPS (Clínica 1 aponta -o pro
# orderer do Hospital A, já que não tem orderer próprio), e --package-id pelo Package ID real
export FABRIC_CFG_PATH=/opt/fabric/config
export CORE_PEER_TLS_ENABLED=true
sudo -u fabric --preserve-env=FABRIC_CFG_PATH,CORE_PEER_TLS_ENABLED \
  bash -c '
    export CORE_PEER_MSPCONFIGPATH=/opt/fabric/crypto/peer/users/Admin@hospitala.aegischain.local/msp
    export CORE_PEER_ADDRESS=203.0.113.11:7051
    export CORE_PEER_TLS_ROOTCERT_FILE=/opt/fabric/crypto/peer/peers/peer0.hospitala.aegischain.local/tls/ca.crt
    export CORE_PEER_LOCALMSPID=HospitalAMSP
    peer lifecycle chaincode approveformyorg \
      -o 203.0.113.11:7050 --tls --cafile /opt/fabric/crypto/orderer-tlsca.crt \
      --channelID aegischain --name aegis-ownership --version 1.0 --sequence 1 \
      --package-id aegis-ownership_1.0:<hash-da-própria-VPS>
  '
```
Repita nas outras 3, trocando os valores pela tabela da Seção 0. Sucesso esperado:
`txid [...] committed with status (VALID)`.

Repita as Seções 8.1–8.4 para `aegis-consent` (porta 9992) e `aegis-logging` (porta 9993) depois
que o fluxo do `aegis-ownership` fechar de ponta a ponta (`checkcommitreadiness`/`commit`/teste de
invoke ainda pendentes de documentar aqui).

### 8.5 `checkcommitreadiness` / `commit` / teste de invoke (validado em campo)

Rodar **numa única VPS** (ex: Hospital A) — `checkcommitreadiness` e `querycommitted` só leem
estado já replicado no canal; `commit` é ação única, repetir em outra VPS geraria erro de
sequence duplicado.

```bash
export FABRIC_CFG_PATH=/opt/fabric/config
export CORE_PEER_TLS_ENABLED=true
sudo -u fabric --preserve-env=FABRIC_CFG_PATH,CORE_PEER_TLS_ENABLED \
  bash -c '
    export CORE_PEER_MSPCONFIGPATH=/opt/fabric/crypto/peer/users/Admin@hospitala.aegischain.local/msp
    export CORE_PEER_ADDRESS=203.0.113.11:7051
    export CORE_PEER_TLS_ROOTCERT_FILE=/opt/fabric/crypto/peer/peers/peer0.hospitala.aegischain.local/tls/ca.crt
    export CORE_PEER_LOCALMSPID=HospitalAMSP
    peer lifecycle chaincode checkcommitreadiness \
      -o 203.0.113.11:7050 --tls --cafile /opt/fabric/crypto/orderer-tlsca.crt \
      --channelID aegischain --name aegis-ownership --version 1.0 --sequence 1 --output json

    peer lifecycle chaincode commit \
      -o 203.0.113.11:7050 --tls --cafile /opt/fabric/crypto/orderer-tlsca.crt \
      --channelID aegischain --name aegis-ownership --version 1.0 --sequence 1 \
      --peerAddresses 203.0.113.11:7051 --tlsRootCertFiles /opt/fabric/crypto/peer-tlsca-hospitala.crt \
      --peerAddresses 203.0.113.12:7051   --tlsRootCertFiles /opt/fabric/crypto/peer-tlsca-hospitalb.crt \
      --peerAddresses 203.0.113.13:7051  --tlsRootCertFiles /opt/fabric/crypto/peer-tlsca-hospitalc.crt \
      --peerAddresses 203.0.113.14:7051    --tlsRootCertFiles /opt/fabric/crypto/peer-tlsca-clinica1.crt

    peer lifecycle chaincode querycommitted --channelID aegischain --name aegis-ownership
  '
```
Resultado real obtido: 4 `txid` idênticos, um por peer (`203.0.113.13`, `203.0.113.11`,
`203.0.113.14`, `203.0.113.12`), todos `committed with status (VALID)`;
`querycommitted` confirmando `Approvals: [Clinica1MSP: true, HospitalAMSP: true, HospitalBMSP:
true, HospitalCMSP: true]`.

**Teste de invoke/query real** (mesma VPS):
```bash
sudo -u fabric --preserve-env=FABRIC_CFG_PATH,CORE_PEER_TLS_ENABLED \
  bash -c '
    export CORE_PEER_MSPCONFIGPATH=/opt/fabric/crypto/peer/users/Admin@hospitala.aegischain.local/msp
    export CORE_PEER_ADDRESS=203.0.113.11:7051
    export CORE_PEER_TLS_ROOTCERT_FILE=/opt/fabric/crypto/peer/peers/peer0.hospitala.aegischain.local/tls/ca.crt
    export CORE_PEER_LOCALMSPID=HospitalAMSP

    peer chaincode invoke \
      -o 203.0.113.11:7050 --tls --cafile /opt/fabric/crypto/orderer-tlsca.crt \
      -C aegischain -n aegis-ownership \
      --peerAddresses 203.0.113.11:7051 --tlsRootCertFiles /opt/fabric/crypto/peer-tlsca-hospitala.crt \
      --peerAddresses 203.0.113.12:7051   --tlsRootCertFiles /opt/fabric/crypto/peer-tlsca-hospitalb.crt \
      --peerAddresses 203.0.113.13:7051  --tlsRootCertFiles /opt/fabric/crypto/peer-tlsca-hospitalc.crt \
      --peerAddresses 203.0.113.14:7051    --tlsRootCertFiles /opt/fabric/crypto/peer-tlsca-clinica1.crt \
      -c "{\"function\":\"CreateOwnershipRecord\",\"Args\":[\"<patientID>\",\"<studyUID>\",\"<proxyEndpoint>\",\"<invokerID>\"]}"

    peer chaincode query -C aegischain -n aegis-ownership \
      -c "{\"function\":\"GetOwnershipRecord\",\"Args\":[\"<patientID>\",\"<studyUID>\"]}"
  '
```
Resultado real obtido: `invoke` → `status:200`; `query` de volta →
`{"docType":"ownershipRecord","patientId":"...","studyUid":"...","sourceProxyEndpoint":"...",
"creatingHospitalId":"HospitalAMSP","ownerInvokerId":"...","creationTimestamp":"..."}` — dados
batendo exatamente com o que foi enviado.

**Isso fecha, pela primeira vez nesta topologia** (Fabric 100% nativo — sem Docker no peer/orderer
— cross-datacenter, 4 organizações reais), uma transação de negócio completa do
`aegis-ownership`. Próximo: replicar Seções 8.1–8.5 para `aegis-consent` (9992) e `aegis-logging`
(9993).

---

## 9. Firewall (aplicar já, não deixar para depois)

**Atenção — regra adicional necessária, descoberta só na Seção 8**: as regras abaixo originalmente
só abriam a porta `7051` (peer) da Clínica 1 nas 3 VPS de hospital. Isso é insuficiente:
`approveformyorg`/`commit` falam direto com o **orderer** (`-o <ip>:7050`), não só com o peer. Sem
abrir `7050` também pra Clínica 1, o `approveformyorg` dela falha. As 3 VPS de hospital já
abriam `7050,7051,9443` reciprocamente entre si, então esse problema só afeta a Clínica 1 → 3
hospitais. Os blocos abaixo já incluem a correção; se você aplicou o firewall antes desta revisão,
rode só a linha extra nas 3 VPS de hospital:
```bash
sudo ufw allow from 203.0.113.14 to any port 7050 proto tcp
```

**`proto tcp` é obrigatório** quando a regra lista mais de uma porta separada por vírgula — sem
isso, `ufw` recusa com `ERROR: Must specify 'tcp' or 'udp' with multiple ports`. Rode `sudo ufw
allow ssh` **antes** do `enable` em todas, e confirme com `sudo ufw status numbered` antes de
ativar — se o `enable` pedir confirmação, só responda `y` depois de checar que a regra de SSH
está lá.

**Hospital A (203.0.113.11):**
```bash
sudo ufw allow ssh
sudo ufw allow from 203.0.113.12   to any port 7050,7051,9443 proto tcp
sudo ufw allow from 203.0.113.13  to any port 7050,7051,9443 proto tcp
sudo ufw allow from 203.0.113.14    to any port 7051 proto tcp
sudo ufw allow from 203.0.113.14    to any port 7050 proto tcp
sudo ufw status numbered
sudo ufw enable
```

**Hospital B (203.0.113.12):**
```bash
sudo ufw allow ssh
sudo ufw allow from 203.0.113.11 to any port 7050,7051,9443 proto tcp
sudo ufw allow from 203.0.113.13  to any port 7050,7051,9443 proto tcp
sudo ufw allow from 203.0.113.14    to any port 7051 proto tcp
sudo ufw allow from 203.0.113.14    to any port 7050 proto tcp
sudo ufw status numbered
sudo ufw enable
```

**Hospital C (203.0.113.13):**
```bash
sudo ufw allow ssh
sudo ufw allow from 203.0.113.11 to any port 7050,7051,9443 proto tcp
sudo ufw allow from 203.0.113.12   to any port 7050,7051,9443 proto tcp
sudo ufw allow from 203.0.113.14    to any port 7051 proto tcp
sudo ufw allow from 203.0.113.14    to any port 7050 proto tcp
sudo ufw status numbered
sudo ufw enable
```

**Clínica 1 (203.0.113.14)** — não tem orderer, não abre `7050`/`9443`:
```bash
sudo ufw allow ssh
sudo ufw allow from 203.0.113.11 to any port 7051 proto tcp
sudo ufw allow from 203.0.113.12   to any port 7051 proto tcp
sudo ufw allow from 203.0.113.13  to any port 7051 proto tcp
sudo ufw status numbered
sudo ufw enable
```

---

## 10. Checklist de validação

- [ ] `peer version` / `orderer version` = 2.5.16 nas 4 VPS
- [ ] Docker instalado e funcional nas 4 (só para CCaaS)
- [ ] `journalctl -u fabric-orderer` sem erro de TLS/Admin nas 3 de hospital
- [ ] `osnadmin channel list` → `active` nas 3
- [ ] `peer channel list` → `aegischain` nas 4
- [ ] anchor peers descobertos via gossip (log `reportMembershipChanges`)
- [ ] builder `ccaas_builder` oficial reconhecendo o pacote (`peer lifecycle chaincode queryinstalled` sem erro de builder)
- [ ] 3 chaincodes instalados/aprovados/commitados nas 4 orgs
- [ ] `CreateOwnershipRecord` → `status:200`
- [ ] `RequestAccess` → `status:200`
- [ ] `UpdateConsentStatus` → `status:200`
- [ ] `CheckAccess` → `status:200, payload:true`
- [ ] firewall aplicado

Depois: CouchDB nos peers (pendência já registrada), reconectar `authcheck`/Aegis Proxy da
Clínica 1 (`BACKEND_MODE=fabric`), benchmark multi-host com a metodologia das Seções 8.3–8.4.
