package main

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/hyperledger/fabric-chaincode-go/shim"
	"github.com/hyperledger/fabric-contract-api-go/contractapi"
)

type AegisConsentContract struct {
	contractapi.Contract
}

type ConsentStatus string

const (
	ConsentStatusWaitingApproval ConsentStatus = "WAITING_APPROVAL"
	ConsentStatusGranted         ConsentStatus = "GRANTED"
	ConsentStatusRevoked         ConsentStatus = "REVOKED"
	ConsentStatusExpired         ConsentStatus = "EXPIRED"
)

const (
	ObjectType            string = "consentEntry"
	CompositeKeyID        string = "consent"
	ConsentRequestedEvent string = "ConsentRequestedEvent"
	ConsentExpiredEvent   string = "ConsentExpiredEvent"
	OwnershipChaincodeName string = "aegis-ownership"
	LoggingChaincodeName   string = "aegis-logging"
	ChannelName            string = "aegischain"
)

type Consent struct {
	ObjectType          string        `json:"docType"`
	PatientID           string        `json:"patientId"`
	ProviderID          string        `json:"providerId"`
	StudyInstanceUID    string        `json:"studyUid"`
	Status              ConsentStatus `json:"status"`
	RequestTimestamp    time.Time     `json:"requestTimestamp"`
	GrantTimestamp      time.Time     `json:"grantTimestamp"`
	ExpiryTimestamp     time.Time     `json:"expiryTimestamp"`
	RevocationTimestamp time.Time     `json:"revocationTimestamp"`
}

type ownershipRecordView struct {
	PatientID      string `json:"patientId"`
	OwnerInvokerID string `json:"ownerInvokerId"`
}

type HistoryQueryResult struct {
	Record    *Consent  `json:"record"`
	TxId      string    `json:"txId"`
	Timestamp time.Time `json:"timestamp"`
	IsDelete  bool      `json:"isDelete"`
}

func invokeGetOwnershipRecord(ctx contractapi.TransactionContextInterface, patientID, studyUID string) (*ownershipRecordView, error) {
	args := [][]byte{[]byte("GetOwnershipRecord"), []byte(patientID), []byte(studyUID)}
	response := ctx.GetStub().InvokeChaincode(OwnershipChaincodeName, args, ChannelName)
	if response.Status != shim.OK {
		return nil, fmt.Errorf("study not registered (ownership lookup failed: %s)", response.Message)
	}

	var record ownershipRecordView
	if err := json.Unmarshal(response.Payload, &record); err != nil {
		return nil, fmt.Errorf("failed to unmarshal ownership record: %w", err)
	}
	return &record, nil
}

func invokeCreateLogEntry(ctx contractapi.TransactionContextInterface, patientID, providerID, studyUID, status, message string) error {
	args := [][]byte{
		[]byte("CreateLogEntry"),
		[]byte(patientID),
		[]byte(providerID),
		[]byte(studyUID),
		[]byte(status),
		[]byte(message),
	}
	response := ctx.GetStub().InvokeChaincode(LoggingChaincodeName, args, ChannelName)
	if response.Status != shim.OK {
		return fmt.Errorf("failed to create log entry: %s", response.Message)
	}
	return nil
}

func (ac *AegisConsentContract) RequestAccess(
	ctx contractapi.TransactionContextInterface,
	patientID string,
	providerID string,
	studyUID string) error {

	if patientID == "" || providerID == "" || studyUID == "" {
		return fmt.Errorf("the patientID, providerID and studyUID arguments must not be empty")
	}

	if _, err := ctx.GetClientIdentity().GetMSPID(); err != nil {
		return fmt.Errorf("failed to get client MSP ID: %w", err)
	}

	if _, err := invokeGetOwnershipRecord(ctx, patientID, studyUID); err != nil {
		return err
	}

	compositeKey, err := ctx.GetStub().CreateCompositeKey(CompositeKeyID, []string{patientID, studyUID, providerID})
	if err != nil {
		return fmt.Errorf("failed to create composite key: %w", err)
	}

	existingConsent, err := ctx.GetStub().GetState(compositeKey)
	if err != nil {
		return fmt.Errorf("failed to read from world state: %w", err)
	}
	if existingConsent != nil {
		return fmt.Errorf("a consent for this patient, study and provider already exists with key: %s", compositeKey)
	}

	txTimestamp, err := ctx.GetStub().GetTxTimestamp()
	if err != nil {
		return fmt.Errorf("failed to get transaction timestamp: %w", err)
	}
	requestTime := txTimestamp.AsTime()

	consent := Consent{
		ObjectType:       ObjectType,
		PatientID:        patientID,
		ProviderID:       providerID,
		StudyInstanceUID: studyUID,
		RequestTimestamp: requestTime,
		Status:           ConsentStatusWaitingApproval,
	}

	consentJSON, err := json.Marshal(consent)
	if err != nil {
		return fmt.Errorf("failed to marshal consent to JSON: %w", err)
	}

	if err := ctx.GetStub().PutState(compositeKey, consentJSON); err != nil {
		return fmt.Errorf("failed to put state on the ledger: %w", err)
	}

	if err := ctx.GetStub().SetEvent(ConsentRequestedEvent, consentJSON); err != nil {
		fmt.Printf("Warning: failed to set event %s: %v\n", ConsentRequestedEvent, err)
	}

	fmt.Printf("Consent successfully requested. Key: %s\n", compositeKey)
	return nil
}

func (ac *AegisConsentContract) UpdateConsentStatus(
	ctx contractapi.TransactionContextInterface,
	patientID string,
	providerID string,
	studyUID string,
	invokerID string,
	approve bool,
	expiryTimestampStr string) error {

	compositeKey, err := ctx.GetStub().CreateCompositeKey(CompositeKeyID, []string{patientID, studyUID, providerID})
	if err != nil {
		return fmt.Errorf("failed to create composite key: %w", err)
	}

	consentJSON, err := ctx.GetStub().GetState(compositeKey)
	if err != nil {
		return fmt.Errorf("failed to read from world state: %w", err)
	}
	if consentJSON == nil {
		return fmt.Errorf("consent with key %s does not exist", compositeKey)
	}

	var consent Consent
	if err := json.Unmarshal(consentJSON, &consent); err != nil {
		return fmt.Errorf("failed to unmarshal consent JSON: %w", err)
	}

	ownership, err := invokeGetOwnershipRecord(ctx, patientID, studyUID)
	if err != nil {
		return err
	}
	if ownership.OwnerInvokerID != invokerID {
		return fmt.Errorf("authorization error: invoker is not the study owner")
	}

	txTimestamp, err := ctx.GetStub().GetTxTimestamp()
	if err != nil {
		return fmt.Errorf("failed to get transaction timestamp: %w", err)
	}
	eventTime := txTimestamp.AsTime()

	var eventName string

	if approve {
		if consent.Status != ConsentStatusWaitingApproval &&
			consent.Status != ConsentStatusRevoked &&
			consent.Status != ConsentStatusExpired {
			return fmt.Errorf("cannot grant consent: current status is '%s'", consent.Status)
		}
		consent.Status = ConsentStatusGranted
		consent.GrantTimestamp = eventTime
		eventName = "ConsentGranted"

		if expiryTimestampStr != "" {
			expiryTime, err := time.Parse(time.RFC3339, expiryTimestampStr)
			if err != nil {
				return fmt.Errorf("failed to parse expiryTimestamp string. Please use RFC3339 format (e.g., '2025-12-31T23:59:59Z'): %w", err)
			}
			if expiryTime.Before(eventTime) {
				return fmt.Errorf("expiryTimestamp must be in the future")
			}
			consent.ExpiryTimestamp = expiryTime
		} else {
			consent.ExpiryTimestamp = time.Time{}
		}

	} else {
		// BUGFIX: bloqueia revoke a partir de EXPIRED (não previsto na Table 3;
		// só GRANTED e WAITING_APPROVAL podem transicionar para REVOKED).
		if consent.Status != ConsentStatusWaitingApproval && consent.Status != ConsentStatusGranted {
			return fmt.Errorf("cannot revoke/deny consent: current status is '%s'", consent.Status)
		}
		consent.Status = ConsentStatusRevoked
		consent.RevocationTimestamp = eventTime
		consent.ExpiryTimestamp = time.Time{}
		eventName = "ConsentRevoked"
	}

	updatedConsentJSON, err := json.Marshal(consent)
	if err != nil {
		return fmt.Errorf("failed to marshal updated consent: %w", err)
	}

	if err := ctx.GetStub().PutState(compositeKey, updatedConsentJSON); err != nil {
		return fmt.Errorf("failed to update consent on the ledger: %w", err)
	}

	if err := ctx.GetStub().SetEvent(eventName, updatedConsentJSON); err != nil {
		fmt.Printf("Warning: failed to set event %s: %v\n", eventName, err)
	}

	return nil
}

func (ac *AegisConsentContract) CheckAccess(
	ctx contractapi.TransactionContextInterface,
	patientID string,
	providerID string,
	studyUID string) (bool, error) {

	compositeKey, err := ctx.GetStub().CreateCompositeKey(CompositeKeyID, []string{patientID, studyUID, providerID})
	if err != nil {
		return false, fmt.Errorf("failed to create composite key: %w", err)
	}

	consentJSON, err := ctx.GetStub().GetState(compositeKey)
	if err != nil {
		return false, fmt.Errorf("failed to read from world state: %w", err)
	}

	if consentJSON == nil {
		// Nenhum consent - nada a logar; nega.
		return false, nil
	}

	var consent Consent
	if err := json.Unmarshal(consentJSON, &consent); err != nil {
		return false, fmt.Errorf("failed to unmarshal consent JSON: %w", err)
	}

	if consent.Status != ConsentStatusGranted {
		if err := invokeCreateLogEntry(ctx, patientID, providerID, studyUID, "FAILURE", "consent not granted"); err != nil {
			fmt.Printf("Warning: failed to write audit log: %v\n", err)
		}
		return false, nil
	}

	if !consent.ExpiryTimestamp.IsZero() && time.Now().UTC().After(consent.ExpiryTimestamp) {
		consent.Status = ConsentStatusExpired

		updatedConsentJSON, err := json.Marshal(consent)
		if err != nil {
			return false, fmt.Errorf("failed to marshal expired consent: %w", err)
		}
		if err := ctx.GetStub().PutState(compositeKey, updatedConsentJSON); err != nil {
			return false, fmt.Errorf("failed to update expired consent on the ledger: %w", err)
		}
		if err := ctx.GetStub().SetEvent(ConsentExpiredEvent, updatedConsentJSON); err != nil {
			fmt.Printf("Warning: failed to set event ConsentExpired: %v\n", err)
		}

		if err := invokeCreateLogEntry(ctx, patientID, providerID, studyUID, "FAILURE", "consent expired at check time"); err != nil {
			fmt.Printf("Warning: failed to write audit log: %v\n", err)
		}
		return false, nil
	}

	if err := invokeCreateLogEntry(ctx, patientID, providerID, studyUID, "SUCCESS", "access granted"); err != nil {
		fmt.Printf("Warning: failed to write audit log: %v\n", err)
	}
	return true, nil
}

func (ac *AegisConsentContract) QueryConsentsByPatient(
	ctx contractapi.TransactionContextInterface,
	patientID string) ([]*Consent, error) {

	resultsIterator, err := ctx.GetStub().GetStateByPartialCompositeKey(CompositeKeyID, []string{patientID})
	if err != nil {
		return nil, fmt.Errorf("failed to iterate over results: %w", err)
	}
	defer resultsIterator.Close()

	var consents []*Consent
	for resultsIterator.HasNext() {
		queryResponse, err := resultsIterator.Next()
		if err != nil {
			return nil, fmt.Errorf("failed to iterate over results: %w", err)
		}

		var consent Consent
		if err := json.Unmarshal(queryResponse.Value, &consent); err != nil {
			return nil, fmt.Errorf("failed to unmarshal consent JSON: %w", err)
		}
		consents = append(consents, &consent)
	}

	return consents, nil
}

func (ac *AegisConsentContract) GetConsentHistory(
	ctx contractapi.TransactionContextInterface,
	patientID string,
	providerID string,
	studyUID string) ([]*HistoryQueryResult, error) {

	compositeKey, err := ctx.GetStub().CreateCompositeKey(CompositeKeyID, []string{patientID, studyUID, providerID})
	if err != nil {
		return nil, fmt.Errorf("failed to create composite key: %w", err)
	}

	resultsIterator, err := ctx.GetStub().GetHistoryForKey(compositeKey)
	if err != nil {
		return nil, fmt.Errorf("failed to get asset history: %w", err)
	}
	defer resultsIterator.Close()

	var records []*HistoryQueryResult
	for resultsIterator.HasNext() {
		response, err := resultsIterator.Next()
		if err != nil {
			return nil, fmt.Errorf("failed to iterate over history results: %w", err)
		}

		var consent Consent
		if !response.IsDelete {
			if err := json.Unmarshal(response.Value, &consent); err != nil {
				return nil, fmt.Errorf("failed to unmarshal historical consent value: %w", err)
			}
		}

		historyRecord := HistoryQueryResult{
			TxId:      response.TxId,
			Timestamp: response.Timestamp.AsTime(),
			IsDelete:  response.IsDelete,
			Record:    &consent,
		}
		if response.IsDelete {
			historyRecord.Record = nil
		}

		records = append(records, &historyRecord)
	}

	return records, nil
}