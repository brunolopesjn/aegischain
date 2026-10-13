# aegischain-benchmark-fabric

Write-path benchmark of AegisChain on the **single-host** Fabric network
(paper Section VII-B, Table 6). It submits
`RequestAccess` and `UpdateConsentStatus` transactions through the Fabric
Gateway SDK, sequentially and with many transactions in flight, and reports
p50, p95 and throughput.

## Prerequisites

- The network running (`aegischain-network/multihost-2org/aegis-network.sh start`)
- Go (see `go.mod`)
- The `fabric-samples` test-network identity (Org1 `User1` or `Admin`)

## Build and run

```bash
go build -o aegischain-benchmark-fabric .

ORG1=$FABRIC_SAMPLES/test-network/organizations/peerOrganizations/org1.example.com
./aegischain-benchmark-fabric \
  -cert     $ORG1/users/User1@org1.example.com/msp/signcerts/cert.pem \
  -keystore $ORG1/users/User1@org1.example.com/msp/keystore \
  -tls-cert $ORG1/peers/peer0.org1.example.com/tls/ca.crt
```

| Flag | Default | Meaning |
|------|---------|---------|
| `-msp-id` | `Org1MSP` | MSP of the client identity |
| `-cert`, `-keystore`, `-tls-cert` | (required) | Client certificate file, private-key directory, peer TLS CA |
| `-peer-endpoint` | `localhost:7051` | Peer gRPC address |
| `-gateway-peer` | `peer0.org1.example.com` | TLS server name |
| `-channel` | `aegischain` | Channel |
| `-chaincode`, `-ownership-chaincode` | `aegis-consent`, `aegis-ownership` | Chaincode names |
| `-concurrency` | `200` | Maximum transactions in flight |
| `-skip-seed` | `false` | Reuse the `patient-i`/`study-i` records already in the ledger |
| `-skip-sequential` | `false` | Skip the sequential blocks |
| `-run-tag` | Unix time | Avoids key collisions when repeating without resetting the network |
| `-diagnose N` | `0` | Decompose N transactions into endorse, submit and commit phases (`diagnose.go`) |
| `-conn-pool` | `1` | Independent gRPC connections in `-diagnose` mode |

## Repeated runs

`run-repeated-benchmark.sh [N]` (default 15) resets the network, runs the
benchmark with a fresh seed (`-skip-sequential`), stores the raw log and
appends p50, p95 and throughput to `results/summary.csv`. It needs the
variables `FABRIC_SAMPLES_PATH`, `AEGISCHAIN_NETWORK_DIR` (the
`multihost-2org` folder), `AEGISCHAIN_BENCHMARK_DIR` and `ORG1`.

`monitor-stats.sh` samples `docker stats` of the peer, orderer and chaincode
containers during a run.

Raw logs of the 15 runs reported in the paper: `results/write-run-final-N.log`.

## License

MIT, see the repository root.
