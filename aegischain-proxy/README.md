# aegischain-proxy

The enforcement point that sits in front of an institution's PACS. A request
for an image is served only if the ledger says the requesting provider has an
active consent from the patient.

```
client ──► NGINX :9080 ── auth_request ──► authcheck :9000 ──► backend
              │                                                  (Fabric CheckAccess
              │ 2xx: allow                                        or baseline REST API)
              ▼
           Orthanc :8042 (DICOMweb)
```

## Components

| Component | Role |
|-----------|------|
| **NGINX** (`nginx/aegis-proxy.conf`) | Listens on `9080`. Every request to `/dicom-web/` triggers an `auth_request` sub-request to `/internal/authcheck`; if it answers 2xx the request is proxied to Orthanc, otherwise it is rejected. |
| **authcheck** (`authcheck/`, Go) | Listens on `9000`, endpoint `/check`. Reads `X-Original-URI` (the study being requested), `X-Patient-ID` and `X-Invoker-ID`, asks the backend whether access is granted, and answers allow or deny. |
| **Orthanc** | DICOM server with the DICOMweb and Stone Web Viewer plugins, authentication disabled. Reachable only through the proxy. |
| **dicom-viewer** (`nginx/dicom-viewer.conf`) | Demo-only NGINX on `8043` that exposes the Stone viewer **without** consent checks. Do not deploy it publicly. |

`authcheck` has two backends, chosen with `BACKEND_MODE`:

| Mode | Backend | Configuration |
|------|---------|---------------|
| `baseline` (default) | The centralized REST API of [`aegischain-baseline`](../aegischain-baseline) | `BASELINE_API_URL` |
| `fabric` | `CheckAccess` of the consent chaincode through the Fabric Gateway | `FABRIC_MSP_ID`, `FABRIC_CERT_PATH`, `FABRIC_KEYSTORE_DIR`, `FABRIC_TLS_CERT_PATH`, `FABRIC_PEER_ENDPOINT`, `FABRIC_GATEWAY_PEER`, `FABRIC_CHANNEL_NAME`, `FABRIC_CHAINCODE_NAME` |

## Compose files

| File | Use |
|------|-----|
| `docker-compose-local.yml` | Laptop setup: bridge networking, `baseline` backend at `host.docker.internal:8080`, Orthanc without published ports. |
| `docker-compose.yml` | Server setup (Clínica 1 in the paper): host networking, `fabric` backend. Expects a client identity under `/opt/aegis-proxy-identity/` (`signcerts/`, `keystore/`, `tls/ca.crt`) and the local peer at `127.0.0.1:7051`. Set `FABRIC_GATEWAY_PEER` to the name or IP in the peer certificate's SAN (the file contains a documentation placeholder). |

## Run (baseline backend, local)

```bash
# 1. the baseline API on :8080
(cd ../aegischain-baseline && docker compose up -d --build)

# 2. the proxy
docker compose -f docker-compose-local.yml up -d --build

# 3. load a study into Orthanc (see below), then request it
curl -i -H 'X-Patient-ID: patient-1' -H 'X-Invoker-ID: dr-silva' \
  http://localhost:9080/dicom-web/studies/<StudyInstanceUID>/series
```
`authcheck` takes the study from the `/studies/<StudyInstanceUID>` part of the
URL. The request is allowed only if the backend holds a `GRANTED` consent for
that patient, study and provider; create one first (see
[`aegischain-baseline`](../aegischain-baseline) for the REST API).

## Run (Fabric backend)

1. Bring up a network ([`aegischain-network`](../aegischain-network)).
2. Place a client identity and the peer's TLS CA in `/opt/aegis-proxy-identity/`.
3. Adjust `FABRIC_*` in `docker-compose.yml`, then `docker compose up -d --build`.

## Loading DICOM data

```bash
ORTHANC_URL=http://localhost:8042 ./scripts/import-dicom.sh <folder-with-dcm> [more folders]
```

The script uploads every `.dcm` file below the given folders to Orthanc and
prints a summary. The study used in the experiments is from
the public QIN-BREAST-02 collection of The Cancer Imaging Archive (DOI 10.7937/TCIA.2019.4cfm06rr).

## Security notes

- `X-Patient-ID` and `X-Invoker-ID` are **client-supplied and not
  authenticated** in this prototype. A real deployment must authenticate the
  caller (mutual TLS or signed tokens) in front of `authcheck`.
- With host networking (`docker-compose.yml`) Orthanc listens on the host.
  Allow only port `9080` through the firewall.
- The images are referenced as `latest`. For exact reproduction, pin the
  versions you tested with.

## License

MIT, see the repository root.
