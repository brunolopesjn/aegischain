# aegischain-benchmark-fabric-cloud

Write-path benchmark of AegisChain on the **distributed four-server** testbed
(paper Section VII-D, Table 7). Same operations as
[`aegischain-benchmark-fabric`](../aegischain-benchmark-fabric), but with a
pool of independent gRPC connections, because a single connection to a remote
peer is limited by HTTP/2 `MaxConcurrentStreams`.

## Prerequisites

- The four-server network of
  [`aegischain-network/multihost-4org`](../aegischain-network/multihost-4org)
- A client identity registered in one of the organizations
- Network access from the machine running the benchmark to the chosen peer

## Build and run

```bash
go build -o aegischain-benchmark-fabric-cloud .
./aegischain-benchmark-fabric-cloud \
  -msp-id HospitalAMSP \
  -cert <signcerts-dir> -keystore <keystore-dir> -tls-cert <peer-tls-ca.crt> \
  -peer-endpoint <peer-ip>:7051 -gateway-peer <peer-ip>
```
`-cert` and `-keystore` take **directories**; the first file in each is used.

| Flag | Default | Meaning |
|------|---------|---------|
| `-msp-id` | `HospitalAMSP` | MSP of the client identity |
| `-cert`, `-keystore`, `-tls-cert` | (required) | Identity and peer TLS CA |
| `-peer-endpoint` | `127.0.0.1:7051` | Peer gRPC address |
| `-gateway-peer` | (required) | Name or IP used for TLS validation; must match a SAN of the peer certificate |
| `-channel` | `aegischain` | Channel |
| `-chaincode`, `-ownership-chaincode` | `aegis-consent`, `aegis-ownership` | Chaincode names |
| `-concurrency` | `200` | Maximum transactions in flight |
| `-pool-size` | `20` | Independent gRPC connections |
| `-skip-seed`, `-skip-sequential`, `-run-tag` | | As in the single-host driver |

To repeat a measurement without re-creating the records, run again with
`-skip-seed -run-tag <new-tag>`.

## License

MIT, see the repository root.
