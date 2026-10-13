# AegisChain

Code and experiment artifacts for **AegisChain**, a blockchain-based system
that enforces patient consent on access to medical imaging (DICOM) shared
between healthcare institutions. Consent, ownership and audit records live on
a permissioned Hyperledger Fabric ledger; each institution keeps its images in
its own PACS, behind a proxy that asks the ledger before serving anything.

This repository accompanies the AegisChain paper (submitted to IEEE Access).
The formal TLA+ specification of the consent state machine is in a separate
repository: [`aegischain-tla`](https://github.com/brunolopesjn/aegischain-tla).

> **Research prototype.** It is built to measure the cost of ledger-based
> consent enforcement, not to be deployed as is. See [Security notes](#security-notes).

## Architecture

```
 client ──► NGINX (auth_request) ──► authcheck ──► Fabric (CheckAccess)
              │  allow                                  ▲
              ▼                                         │ ownership / consent / logging
           Orthanc (DICOMweb)                    3 chaincodes on a Raft-ordered
                                                 Fabric network (CouchDB state)
```

## Repository map

| Directory | What it is | Paper |
|-----------|------------|-------|
| [`aegischain-ownership-chaincode`](aegischain-ownership-chaincode) | Ownership chaincode (who owns each study) | design |
| [`aegischain-consent-chaincode`](aegischain-consent-chaincode) | Consent chaincode (state machine, access check) | design |
| [`aegischain-logging-chaincode`](aegischain-logging-chaincode) | Audit-log chaincode | design |
| [`aegischain-network`](aegischain-network) | Scripts to bring up the single-host and four-server Fabric networks | testbeds |
| [`aegischain-proxy`](aegischain-proxy) | NGINX + `authcheck` + Orthanc enforcement point | design, end-to-end |
| [`aegischain-baseline`](aegischain-baseline) | Centralized REST + PostgreSQL implementation | baseline |
| [`aegischain-benchmark`](aegischain-benchmark) | Load driver for the baseline | Table 9 |
| [`aegischain-benchmark-fabric`](aegischain-benchmark-fabric) | Write benchmark, single host | Table 6 |
| [`aegischain-benchmark-fabric-cloud`](aegischain-benchmark-fabric-cloud) | Write benchmark, four servers | Table 7 |
| [`aegischain-benchmark-fabric-native`](aegischain-benchmark-fabric-native) | `CheckAccess` straight on Fabric, four servers | Table 7 |
| [`aegischain-benchmark-fabric-e2e`](aegischain-benchmark-fabric-e2e) | End-to-end WADO-RS through the proxy | Table 7 |
| [`aegischain-benchmark-scalability`](aegischain-benchmark-scalability) | `CheckAccess` and discovery versus ledger size | Table 5 |

Each directory has its own README with build instructions, flags and the
paper section it supports.

## Requirements

- Linux with Docker and Docker Compose v2
- Go (each `go.mod` states its version)
- Hyperledger Fabric 2.5.16 binaries and images, Fabric CA 1.5.17, and a clone
  of [`fabric-samples`](https://github.com/hyperledger/fabric-samples)
  (not included here). `install-fabric.sh` is the official Fabric installer; use
  it with the versions above.
- For the distributed testbed: four servers (Ubuntu 24.04 was used).
  See [`aegischain-network/multihost-4org`](aegischain-network/multihost-4org/fabric-4vps-nativo.md).

The experiments were run on Arch Linux (single host) and Ubuntu 24.04
(servers). The single host has an AMD Ryzen 7 5700X and 32 GB of RAM.

## Quick start (single host)

```bash
git clone https://github.com/brunolopesjn/aegischain.git
cd aegischain
export AEGISCHAIN_REPO=$PWD
export FABRIC_SAMPLES=~/fabric-samples      # clone with Fabric 2.5.16 binaries installed

# 1. Fabric network with the three chaincodes deployed
cd aegischain-network/multihost-2org
./aegis-network.sh start -f $FABRIC_SAMPLES -a $AEGISCHAIN_REPO   # see its README for exact usage
cd ../..

# 2. Write benchmark (see aegischain-benchmark-fabric/README.md)
cd aegischain-benchmark-fabric && go build -o aegischain-benchmark-fabric .
```

```bash
# Baseline comparison
cd aegischain-baseline && docker compose up -d --build
cd ../aegischain-benchmark && go build -o aegischain-benchmark . && ./aegischain-benchmark
```

## Reproducing the paper

| Result | How |
|--------|-----|
| Formal verification of the consent state machine | [`aegischain-tla`](https://github.com/brunolopesjn/aegischain-tla) |
| Single-host write latency/throughput | `aegischain-benchmark-fabric`, 15 repetitions via `run-repeated-benchmark.sh` |
| Distributed writes, reads, end to end | `aegischain-benchmark-fabric-cloud`, `-native`, `-e2e` on the four-server network |
| Centralized baseline | `aegischain-baseline` + `aegischain-benchmark`, 15 repetitions |
| Scalability up to 10^6 records | `aegischain-benchmark-scalability/run-scalability-full.sh` |

Raw logs of the measurements reported in the paper are under each benchmark's
`results/` directory (scalability: `raw-measurements.tar.gz`). The large
seeding logs of the scalability experiment are not versioned.

Absolute numbers depend on hardware and network; the IP addresses in the
configuration files are documentation placeholders (`203.0.113.0/24`) that you
must replace with your own servers.

## Security notes

- The proxy trusts the `X-Patient-ID` and `X-Invoker-ID` request headers. They
  are **not authenticated** in this prototype; a real deployment needs mutual
  TLS or signed tokens in front of it.
- Orthanc runs with authentication disabled and must only be reachable
  through the proxy.
- The baseline REST API has no authentication.
- Database credentials in the compose files (`aegis`/`aegis`) are for local
  benchmarking only.
- No cryptographic material is versioned. Generate your own certificates and
  keys; never commit `crypto-config/`, `organizations/` or `channel-artifacts/`.
- The imaging study used in the experiments comes from the public QIN-BREAST-02 collection of The Cancer Imaging Archive (DOI 10.7937/TCIA.2019.4cfm06rr).
  Follow the TCIA data usage policy if you reuse it.

## Citation

See [`CITATION.cff`](CITATION.cff). Once the paper is published, cite it
together with the Zenodo record of this release.

## License

[MIT](LICENSE).
