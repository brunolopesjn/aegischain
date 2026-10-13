# aegischain-logging-chaincode

Logging Chaincode (LC) of AegisChain: a Hyperledger Fabric chaincode (Go,
`contractapi`) that stores an append-only audit trail of access events, so a
patient can see who accessed what and when.
Entries are written by the consent chaincode through a cross-chaincode call to
`CreateLogEntry`. An entry holds the transaction ID, patient ID, provider ID,
study UID, timestamp, a status and an optional message.

## Contract functions

| Function | Purpose |
|----------|---------|
| `CreateLogEntry` | Append an access-log entry |
| `QueryLogsByPatient` | List the entries of a patient |

## Build and run

External service (Chaincode-as-a-Service):

| Variable | Meaning |
|----------|---------|
| `CHAINCODE_ID` | Package ID assigned by the peer after installation |
| `CHAINCODE_SERVER_ADDRESS` | gRPC listen address (port 9992 in `aegis-network.sh`) |

```bash
go build -o aegis-logging-bin .
```

Deployed under the name `aegis-logging` by the scripts in
[`aegischain-network`](../aegischain-network). A `Dockerfile` is provided.

## License

MIT, see the repository root.
