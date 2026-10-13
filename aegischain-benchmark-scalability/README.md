# aegischain-benchmark-scalability

Scalability experiments of the consent chaincode as the ledger grows (paper
Section VII-C, Table 5):

- **Experiment A, `checkaccess`**: `CheckAccess` latency and throughput versus
  the number of patients and consents already in the ledger.
- **Experiment B, `discovery`**: `QueryConsentsByPatient` latency versus the
  number of consent records of a single patient (partial composite-key scan).

## Prerequisites

- The single-host network running
  (`aegischain-network/multihost-2org/aegis-network.sh start`)
- Go (see `go.mod`)
- Time: the full experiment seeds up to 10^6 records and repeats each level 15
  times, so expect many hours.

## Run one measurement

```bash
go build -o aegischain-benchmark-scalability .
./aegischain-benchmark-scalability -experiment checkaccess -level 1000 \
  -cert-dir <signcerts> -keystore-dir <keystore> -tls-cert <peer-tls-ca.crt>
```

| Flag | Default | Meaning |
|------|---------|---------|
| `-experiment` | (required) | `checkaccess` or `discovery` |
| `-level` | `0` | Scale level. `checkaccess`: cumulative patients that must exist. `discovery`: cumulative providers for the fixed patient |
| `-seed-start` | `0` | Start index of the incremental seed (first `patient-scale-N`, or providers already present) |
| `-skip-seed` | `false` | Measure only; reuse what is already populated |
| `-msp-id` | `Org1MSP` | MSP of the client identity |
| `-cert-dir`, `-keystore-dir`, `-tls-cert` | (required) | Identity and peer TLS CA |
| `-peer-endpoint`, `-gateway-peer` | `localhost:7051`, `peer0.org1.example.com` | Peer and TLS name |
| `-channel`, `-chaincode`, `-ownership-chaincode` | `aegischain`, `aegis-consent`, `aegis-ownership` | |
| `-concurrency`, `-pool-size` | `50`, `10` | Calls in flight, gRPC connections |

## Run the whole experiment

`run-scalability-full.sh` seeds the ledger level by level (100, 1k, 10k, 100k,
1M for Experiment A; 10 to 1M for Experiment B) and takes 15 measurements per
level:

```bash
export FABRIC_SAMPLES_PATH=~/fabric-samples      # default: $HOME/fabric-samples
export LOGDIR=./logs                             # default: a timestamped directory under $HOME
./run-scalability-full.sh
```

Raw logs: `<experiment>-<level>-repN.log` and `seed-<experiment>-<level>.log`.

Build the summary table from the raw logs:

```bash
python3 make-summary.py <log-dir> > results/summary.csv
```

`make-summary.py` writes one CSV row per valid measurement log and lists on
stderr any log without a result line (exit status 1), so an incomplete result
set is not published by accident.

## Results

`results/summary.csv`: p50, p95 and throughput per level and repetition
(135 measurements, 15 repetitions each: Experiment A at 100 to 1,000,000
patients; Experiment B at 10 to 10,000 records for one patient).
`results/raw-measurements.tar.gz` holds the corresponding raw logs.

`results/probes/` holds single-run probes at lower concurrency
(`-concurrency 1, 2, 5, 10`), used to separate queueing from per-call cost, and
the memory log of `couchdb0` during Experiment B at 10,000 records.

Notes:

- Experiment B stops at 10,000 records per patient. The 100,000 and 1,000,000
  levels were not re-measured in the final campaign (paper, Table 5, note a).
- During the seeding of Experiment A, patient indices 203299 and 822768 had
  transient endorsement failures; the measurements do not use them.
- `couchdb0` was killed by the kernel (out of memory) during Experiment B
  at 10,000 records. It was restarted, the ledger was verified, and that level
  was measured again from scratch.
- Repetitions within a level reuse the same seeded ledger.

## License

MIT, see the repository root.
