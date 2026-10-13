# aegischain-network

Scripts and configuration to bring up the Hyperledger Fabric networks used in
the AegisChain evaluation. Two testbeds are provided:

| Directory | Testbed | Used for (paper) |
|-----------|---------|------------------|
| [`multihost-2org/`](multihost-2org/) | One machine, two organizations, Raft ordering, CouchDB, chaincodes deployed as external services (CCaaS) | Single-host write benchmarks (Table 6), scalability (Table 5) |
| [`multihost-4org/`](multihost-4org/) | Four Linux servers: three hospitals (orderer + peer each) and one clinic (peer only) | Distributed benchmarks (Table 7), end-to-end WADO-RS |

The directory name `multihost-2org` is historical: its script runs on a single host.

## Versions

Hyperledger Fabric 2.5.16, Fabric CA 1.5.17, channel `aegischain`.
`install-fabric.sh` (repository root) installs the matching binaries and
images.

## Single-host testbed (`multihost-2org`)

`aegis-network.sh` starts the Fabric `test-network` from `fabric-samples`,
creates the `aegischain` channel and deploys the three chaincodes as external
services (Chaincode-as-a-Service).

### Prerequisites

- Docker and Docker Compose v2
- Go (see the chaincodes' `go.mod`)
- A clone of [`hyperledger/fabric-samples`](https://github.com/hyperledger/fabric-samples)
  with the Fabric binaries installed (not included in this repository)

### Usage

```bash
cd aegischain-network/multihost-2org
./aegis-network.sh start -fp /path/to/fabric-samples -ap /path/to/aegischain
./aegis-network.sh stop -fp /path/to/fabric-samples
```
`-fp` can be replaced by `FABRIC_SAMPLES_PATH` and `-ap` (the directory that
contains the chaincode folders, here the repository root) by
`AEGISCHAIN_CHAINCODES_PATH`.

What `start` does:

1. Stops any previous network and kills orphan chaincode processes on ports
   9991 to 9993.
2. Brings up two organizations, one Raft orderer and CouchDB state databases.
3. Creates the `aegischain` channel and joins both peers.
4. Packages, installs, approves and commits the Ownership, Consent and Logging
   chaincodes as CCaaS. They run as local processes whose PIDs are written to
   `/tmp/aegis-ccaas-pids`.
5. Makes the host reachable from the peer containers as `host.docker.internal`.

`ccaas-builder/` holds the external builder scripts (`detect`, `build`,
`release`) and `ccaas-package/` the connection and metadata files of each
chaincode.

## Distributed testbed (`multihost-4org`)

A native (non-Docker) deployment on four servers:

| Organization | MSP | Orderer | Peer |
|--------------|-----|---------|------|
| Hospital A | `HospitalAMSP` | yes | yes |
| Hospital B | `HospitalBMSP` | yes | yes |
| Hospital C | `HospitalCMSP` | yes | yes |
| Clínica 1 | `Clinica1MSP` | no | yes |

The three orderers form a Raft cluster. The step-by-step procedure (servers,
firewall, crypto material, channel creation, chaincode lifecycle,
verification) is in
[`fabric-4vps-nativo.md`](multihost-4org/fabric-4vps-nativo.md) (in
Portuguese). It uses two shell variables you must define first:
`AEGISCHAIN_REPO` (this repository) and `FABRIC_SAMPLES`.

IP addresses in this repository are documentation placeholders
(`203.0.113.11` to `203.0.113.14`, in the order of the table above). Replace
them with your own servers' addresses in `configtx.yaml`, `crypto-config.yaml`
and the guide before following it.

### Generating the crypto material

Certificates, private keys and channel blocks are **not** versioned; every
deployment generates its own, as described in section 2 of the guide using
`crypto-config.yaml` and `configtx.yaml`.

Never commit `crypto-config/`, `organizations/` or `channel-artifacts/`
(they are in `.gitignore`).

## Next steps

With a network running, use one of the benchmark drivers:
[`aegischain-benchmark-fabric`](../aegischain-benchmark-fabric) (single host),
[`aegischain-benchmark-fabric-cloud`](../aegischain-benchmark-fabric-cloud) and
[`aegischain-benchmark-fabric-native`](../aegischain-benchmark-fabric-native)
(distributed), [`aegischain-benchmark-scalability`](../aegischain-benchmark-scalability).

## License

MIT, see the repository root.
