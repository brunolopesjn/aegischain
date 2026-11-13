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
| `RequestAccess` | A provider asks for access to a study (`patientID`, `providerID`, `studyUID`) |
| `UpdateConsentStatus` | The study owner approves, denies or revokes, with an expiry (`patientID`, `providerID`, `studyUID`, `approve`, `expiryTimestamp`). The caller is taken from the transaction signer; there is no invoker argument |
| `CheckAccess` | Access decision used by the proxy. Submitted as a transaction because it writes an audit entry; it fails closed if the entry cannot be written |
| `GetCallerID` | Returns the signer's `GetID()`. Used to obtain the value to register as `ownerInvokerID` in the ownership chaincode |
| `QueryConsentsByPatient` | Lists a patient's consents (partial composite-key scan; cost grows with the number of records per patient) |
| `GetConsentHistory` | Full transaction history of one consent |

## Identity and trust model

- `UpdateConsentStatus` authorizes the **transaction signer**
  (`GetClientIdentity().GetID()`), not a caller-supplied string. The call is
  accepted only if the signer equals the `ownerInvokerID` stored in the
  ownership record of the study.
- `ownerInvokerID` is set by the registering institution in
  `CreateOwnershipRecord` (see `aegis-ownership`). It is a registrar-attested
  value: the model trusts the institution that registers the study to bind it to
  the correct identity. To obtain the value for a given identity, call
  `GetCallerID` with that identity.
- `CheckAccess` uses the transaction timestamp (`GetTxTimestamp`) for expiry, so
  all endorsing peers evaluate the same instant. This is the ledger's time, not a
  trusted clock.
- `CheckAccess` writes an audit entry for every decision on an existing consent,
  and returns an error (no decision) if that write fails. A request for which no
  consent exists is denied without an audit entry, to avoid unbounded ledger
  growth from arbitrary probes.
- `providerID` in `RequestAccess` is a claim made by the caller and is not bound
  to the signer's identity.

Known limitations: `CreateLogEntry` and `QueryLogsByPatient` in `aegis-logging`
are not access-controlled, so a client can invoke the logging chaincode directly.

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