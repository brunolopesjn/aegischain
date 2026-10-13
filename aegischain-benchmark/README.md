# aegischain-benchmark

HTTP load driver for [`aegischain-baseline`](../aegischain-baseline). It
produces the baseline side of the comparison in the paper (Table
9): latency percentiles (p50, p95) and throughput for write
operations (consent creation, status update) and for the read-only access
check, first sequentially and then with many requests in flight.

## Build and run

Requires Go (see `go.mod`) and a running baseline
(`docker compose up -d --build` in `../aegischain-baseline`).

```bash
go build -o aegischain-benchmark .
./aegischain-benchmark -url http://localhost:8080
```

| Flag | Default | Meaning |
|------|---------|---------|
| `-url` | `http://localhost:8080` | Base URL of the API |
| `-concurrency` | `200` | Maximum requests in flight in the concurrent mode |
| `-run-tag` | Unix time | Tag embedded in provider IDs, so repeated runs do not collide without resetting the database |
| `-skip-seed` | `false` | Reuse the `patient-i`/`study-i` records already in the database |
| `-skip-sequential` | `false` | Skip the three sequential blocks |

Each run prints one line per operation with p50, p95 and throughput.
The paper reports 15 independent repetitions; the raw output of each is in
[`results/`](results) (`baseline-run-N.log`). As in the paper, the concurrent block keeps 200
requests in flight (the `-concurrency` default), against a baseline seeded with
5,000 patients (`SEED_PATIENT_COUNT` in `aegischain-baseline/docker-compose.yml`);
the flags themselves are not recorded in the logs.

## License

MIT, see the repository root.
