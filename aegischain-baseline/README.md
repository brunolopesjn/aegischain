# aegischain-baseline

Centralized reference implementation of the AegisChain access control
(ownership, consent, access check): a Go REST API (Gin) on PostgreSQL. It
exists to answer one question in the paper: what does the same consent logic
cost **without** a blockchain? Its latency and throughput are the comparison
point for the Fabric-based system (paper Section VIII-B,
Table 9).

It is a measurement baseline, not a production service. There is **no
authentication**: caller identity is taken from request headers. Do not
expose it to an untrusted network.

## Layout

```
main.go                     router (Gin)
handlers/                   ownership and consent endpoints
models/                     patient, institution, ownership, consent, log entry
db/                         PostgreSQL connection
Dockerfile                  API image
docker-compose.yml          postgres + api
docker-compose-local.yml    postgres + adminer only (run the API with `go run`)
```

## Run

```bash
docker compose up -d --build              # postgres + api on :8080
```

or, with the API on the host:

```bash
docker compose -f docker-compose-local.yml up -d   # postgres on 127.0.0.1:5432, adminer on 127.0.0.1:8081
go run .
```

| Variable | Meaning | Default |
|----------|---------|---------|
| `DATABASE_DSN` | PostgreSQL DSN | `host=localhost user=aegis ... port=5432` |
| `SEED_PATIENT_COUNT` | Patients seeded at startup | `5000` in `docker-compose.yml` |

The database credentials in the compose files (`aegis`/`aegis`) are for local
benchmarking only; Postgres is published on the loopback interface.

## API

| Method | Path | Headers | Purpose |
|--------|------|---------|---------|
| POST | `/ownership` | `X-MSP-ID`, `X-Invoker-ID` | Register ownership of a study |
| POST | `/consent/request` | `X-MSP-ID`, `X-Invoker-ID` (the provider) | A provider requests access |
| PUT | `/consent/{patientId}/{studyUid}/{providerId}` | `X-Invoker-ID` (the patient) | The patient approves or denies, with an expiry |
| GET | `/consent/{patientId}/{studyUid}/{providerId}/check` | none | Access check |
| GET | `/healthz` | none | Liveness |

Caller identity comes from request headers and is not authenticated.

The proxy in [`aegischain-proxy`](../aegischain-proxy) can use this API as its
backend (`BACKEND_MODE=baseline`).

## Benchmark

Load driver: [`aegischain-benchmark`](../aegischain-benchmark).

## License

MIT, see the repository root.
