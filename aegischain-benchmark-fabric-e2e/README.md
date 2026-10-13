# aegischain-benchmark-fabric-e2e

End-to-end benchmark: HTTP `GET` of a DICOM resource (WADO-RS) **through the
Aegis proxy**, so each request pays for the NGINX `auth_request`, the
`authcheck` service, a Fabric `CheckAccess` and the Orthanc response (paper
Section VII-D, Table 7). Each run sends 5,000 requests
and reports p50, p95 and throughput.

## Prerequisites

- The proxy stack of [`aegischain-proxy`](../aegischain-proxy) running with
  `BACKEND_MODE=fabric`
- The study loaded in Orthanc (`aegischain-proxy/scripts/import-dicom.sh`)
- A consent in the ledger authorizing `patient-id` / `provider-id` for that
  study. The drivers in this repository do not create it: register the study's
  ownership and have the patient grant the consent (`RequestAccess`, then
  `UpdateConsentStatus`) before running.

## Build and run

```bash
go build -o aegischain-benchmark-fabric-e2e .
./aegischain-benchmark-fabric-e2e -url http://<proxy-host>:9080 -skip-sequential -timeout 90
```

| Flag | Default | Meaning |
|------|---------|---------|
| `-url` | placeholder | Base URL of the Aegis proxy; replace with yours |
| `-study-uid`, `-patient-id`, `-provider-id` | TCIA study, `patient-e2e-1`, `dr-silva` | Resource and identity already authorized |
| `-concurrency` | `200` | Maximum requests in flight |
| `-timeout` | `30` | Per-request timeout in seconds |
| `-skip-sequential` | `false` | Run only the concurrent block |

`run-repeated-e2e.sh [N]` repeats the concurrent run N times.

Raw logs of the paired runs reported in the paper, end to end and
Fabric-direct, are in `results/paired-e2e-run-N.log` and
`results/paired-fabric-direct-run-N.log`. Run N of each kind forms one pair;
the paper describes the 15 pairs as temporally interleaved (Section VII-D).

The study used is from the public QIN-BREAST-02 collection of The Cancer Imaging Archive (DOI 10.7937/TCIA.2019.4cfm06rr).

## License

MIT, see the repository root.
