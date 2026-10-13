# aegischain-benchmark-fabric-native

Read-path benchmark of AegisChain on the distributed testbed (paper Section
VII-D, Table 7): latency and throughput of
`CheckAccess` called **directly on Fabric**, without the HTTP proxy. It is the
reference against which the end-to-end WADO-RS measurement
([`aegischain-benchmark-fabric-e2e`](../aegischain-benchmark-fabric-e2e)) is
compared.

## Prerequisites

- The four-server network running
- A consent already granted for the patient, study and provider used below
  (defaults: `patient-e2e-1`, `dr-silva`). The drivers in this repository do not create it: register the study's
  ownership and have the patient grant the consent (`RequestAccess`, then
  `UpdateConsentStatus`) before running.

## Build and run

```bash
go build -o aegischain-benchmark-fabric-native .
./aegischain-benchmark-fabric-native \
  -cert-dir <signcerts> -keystore-dir <keystore> -tls-cert <peer-tls-ca.crt> \
  -peer-endpoint <peer-ip>:7051 -gateway-peer <peer-ip>
```

| Flag | Default | Meaning |
|------|---------|---------|
| `-msp-id` | `HospitalAMSP` | MSP of the client identity |
| `-cert-dir`, `-keystore-dir`, `-tls-cert` | (required) | Identity and peer TLS CA |
| `-peer-endpoint`, `-gateway-peer` | placeholder IP | Peer address and TLS name (SAN); replace with yours |
| `-channel`, `-chaincode` | `aegischain`, `aegis-consent` | Channel and chaincode |
| `-patient-id`, `-provider-id`, `-study-uid` | `patient-e2e-1`, `dr-silva`, a TCIA study | Consent to query; must already be authorized |
| `-concurrency` | `200` | Maximum calls in flight |
| `-pool-size` | `20` | Independent gRPC connections |
| `-skip-sequential` | `false` | Skip the sequential block |

The paired raw logs (`paired-fabric-direct-run-N.log`) are stored with the
end-to-end ones, in
[`../aegischain-benchmark-fabric-e2e/results`](../aegischain-benchmark-fabric-e2e/results).

## License

MIT, see the repository root.
