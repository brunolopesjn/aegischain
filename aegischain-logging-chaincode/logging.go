package main

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/hyperledger/fabric-contract-api-go/contractapi"
)

type AegisLoggingContract struct {
	contractapi.Contract
}

type querySelector struct {
	DocType   string `json:"docType"`
	PatientID string `json:"patientId"`
}

type LogStatus string

const (
	LogStatusSuccess LogStatus = "SUCCESS"
	LogStatusFailure LogStatus = "FAILURE"
)

const (
	ObjectType string = "logEntry"
)

type LogEntry struct {
	ObjectType       string    `json:"docType"`
	TxID             string    `json:"txId"`
	PatientID        string    `json:"patientId"`
	ProviderID       string    `json:"providerId"`
	StudyInstanceUID string    `json:"studyUid"`
	Timestamp        time.Time `json:"timestamp"`
	Status           LogStatus `json:"status"`
	Message          string    `json:"message,omitempty"`
}

func (ac *AegisLoggingContract) CreateLogEntry(
	ctx contractapi.TransactionContextInterface,
	patientID string,
	providerID string,
	studyUID string,
	status string,
	message string) error {

	txID := ctx.GetStub().GetTxID()
	if txID == "" {
		return fmt.Errorf("failed to get transaction ID from stub")
	}

	txTimestamp, err := ctx.GetStub().GetTxTimestamp()
	if err != nil {
		return fmt.Errorf("failed to get transaction timestamp: %w", err)
	}
	eventTime := txTimestamp.AsTime()

	logStatus := LogStatus(status)
	if logStatus != LogStatusSuccess && logStatus != LogStatusFailure {
		return fmt.Errorf("invalid status '%s': must be SUCCESS or FAILURE", status)
	}

	logEntry := LogEntry{
		ObjectType:       ObjectType,
		TxID:             txID,
		PatientID:        patientID,
		ProviderID:       providerID,
		StudyInstanceUID: studyUID,
		Timestamp:        eventTime,
		Status:           logStatus,
		Message:          message,
	}

	logJSON, err := json.Marshal(logEntry)
	if err != nil {
		return fmt.Errorf("failed to marshal log entry: %w", err)
	}

	if err := ctx.GetStub().PutState(txID, logJSON); err != nil {
		return fmt.Errorf("failed to put log entry on the ledger: %w", err)
	}

	fmt.Printf("Log entry successfully created. TxID: %s\n", txID)
	return nil
}

func (ac *AegisLoggingContract) QueryLogsByPatient(
	ctx contractapi.TransactionContextInterface,
	patientID string) ([]*LogEntry, error) {

	selector := querySelector{
		DocType:   ObjectType,
		PatientID: patientID,
	}

	selectorBytes, err := json.Marshal(map[string]interface{}{"selector": selector})
	if err != nil {
		return nil, fmt.Errorf("failed to marshal query selector: %w", err)
	}
	queryString := string(selectorBytes)

	resultsIterator, err := ctx.GetStub().GetQueryResult(queryString)
	if err != nil {
		return nil, fmt.Errorf("failed to execute rich query: %w", err)
	}
	defer resultsIterator.Close()

	var logs []*LogEntry
	for resultsIterator.HasNext() {
		queryResponse, err := resultsIterator.Next()
		if err != nil {
			return nil, fmt.Errorf("failed to iterate over results: %w", err)
		}

		var logEntry LogEntry
		if err := json.Unmarshal(queryResponse.Value, &logEntry); err != nil {
			fmt.Printf("Warning: failed to unmarshal log entry, skipping: %v\n", err)
			continue
		}
		logs = append(logs, &logEntry)
	}

	return logs, nil
}