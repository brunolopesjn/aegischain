# aegischain-ownership-chaincode

Ownership Chaincode (OC) of AegisChain: a Hyperledger Fabric chaincode (Go,
`contractapi`) that records which patient owns each imaging study and where
the study can be retrieved (the URL of the institution's Aegis proxy). It is
the root of trust for consent: only the owner of a study can grant access to
it. The consent chaincode checks ownership with a cross-chaincode call.

## Contract functions

| Function | Purpose |
|----------|---------|
| `CreateOwnershipRecord` | Register a study for a patient |
| `GetOwnershipRecord` | Read one ownership record |
| `QueryOwnershipRecordsByPatient` | List a patient's records |

## Build and run

The chaincode runs as an external service (Chaincode-as-a-Service) and reads:

| Variable | Meaning |
|----------|---------|
| `CHAINCODE_ID` | Package ID assigned by the peer after installation |
| `CHAINCODE_SERVER_ADDRESS` | gRPC listen address, e.g. `0.0.0.0:9991` (port 9991 in `aegis-network.sh`) |

```bash
go build -o aegis-ownership-bin .
```

You normally do not run it by hand. `aegischain-network/multihost-2org/aegis-network.sh`
(single host) and `aegischain-network/multihost-4org/` (four servers) package,
install and start it under the chaincode name `aegis-ownership`. A
`Dockerfile` is provided to run it as a container.

## Related

- [`aegischain-consent-chaincode`](../aegischain-consent-chaincode)
- [`aegischain-logging-chaincode`](../aegischain-logging-chaincode)

## License

MIT, see the repository root.
