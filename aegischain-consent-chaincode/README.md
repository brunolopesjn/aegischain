# aegischain-consent-chaincode

Consent Chaincode (COC) of AegisChain: a Hyperledger Fabric chaincode (Go,
`contractapi`) that implements the consent state machine between patients and
healthcare providers, and answers the access-control query used by the Aegis
proxy.

Design points:

- Consent records use the composite key `patientID~studyUID~providerID`.
- Expiration is **lazy**: nothing runs on a timer; `CheckAccess` compares the
  expiry with the transaction timestamp when it is called.
- The state machine is formally specified and model-checked with TLC in
  [`aegischain-tla`](https://github.com/brunolopesjn/aegischain-tla).
  Consent states: `WAITING_APPROVAL`, `GRANTED`, `REVOKED` and `EXPIRED`; a
  denial by the patient is recorded as `REVOKED`.

## Contract functions

| Function | Purpose |
|----------|---------|
| `RequestAccess` | A provider asks for access to a study |
| `UpdateConsentStatus` | The patient approves, denies or revokes, with an expiry |
| `CheckAccess` | Read-only decision: is this provider currently authorized? |
| `QueryConsentsByPatient` | Lists a patient's consents (partial composite-key scan; cost grows with the number of records per patient) |
| `GetConsentHistory` | Full transaction history of one consent |

For cross-chaincode calls the chaincode uses the channel it is deployed on
(`ctx.GetStub().GetChannelID()`), so the channel name is not hardcoded.

## Build and run

External service (Chaincode-as-a-Service), configured through:

| Variable | Meaning |
|----------|---------|
| `CHAINCODE_ID` | Package ID assigned by the peer after installation |
| `CHAINCODE_SERVER_ADDRESS` | gRPC listen address (port 9993 in `aegis-network.sh`) |

```bash
go build -o aegis-consent-bin .
```

Packaged and started by the scripts in
[`aegischain-network`](../aegischain-network) under the name `aegis-consent`.
A `Dockerfile` is provided.

## License

MIT, see the repository root.
