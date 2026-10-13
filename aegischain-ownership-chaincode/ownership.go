package main

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/hyperledger/fabric-contract-api-go/contractapi"
)

type AegisOwnershipContract struct {
	contractapi.Contract
}

type OwnershipRecord struct {
	ObjectType          string    `json:"docType"`
	PatientID           string    `json:"patientId"`
	StudyInstanceUID    string    `json:"studyUid"`
	SourceProxyEndpoint string    `json:"sourceProxyEndpoint"`
	CreatingHospitalID  string    `json:"creatingHospitalId"`
	OwnerInvokerID      string    `json:"ownerInvokerId"`
	CreationTimestamp   time.Time `json:"creationTimestamp"`
}

const (
	ObjectType     string = "ownershipRecord"
	CompositeKeyID string = "ownershipRecord"
)


func (ac *AegisOwnershipContract) CreateOwnershipRecord(
	ctx contractapi.TransactionContextInterface,
	patientID string,
	studyUID string,
	sourceProxyEndpoint string,
	ownerInvokerID string) error {

	if patientID == "" || studyUID == "" || sourceProxyEndpoint == "" || ownerInvokerID == "" {
		return fmt.Errorf("patientID, studyUID, sourceProxyEndpoint and ownerInvokerID must not be empty")
	}

	creatingHospitalID, err := ctx.GetClientIdentity().GetMSPID()
	if err != nil {
		return fmt.Errorf("failed to get client MSP ID: %w", err)
	}

	compositeKey, err := ctx.GetStub().CreateCompositeKey(CompositeKeyID, []string{patientID, studyUID})
	if err != nil {
		return fmt.Errorf("failed to create composite key: %w", err)
	}

	existingRecord, err := ctx.GetStub().GetState(compositeKey)
	if err != nil {
		return fmt.Errorf("failed to read from world state: %w", err)
	}
	if existingRecord != nil {
		return fmt.Errorf("an ownership record for this patient and study already exists with key: %s", compositeKey)
	}

	txTimestamp, err := ctx.GetStub().GetTxTimestamp()
	if err != nil {
		return fmt.Errorf("failed to get transaction timestamp: %w", err)
	}
	creationTime := txTimestamp.AsTime() // BUGFIX: era time.Parse(RFC3339, txTimestamp.String())

	record := OwnershipRecord{
		ObjectType:          ObjectType,
		PatientID:           patientID,
		StudyInstanceUID:    studyUID,
		SourceProxyEndpoint: sourceProxyEndpoint,
		CreatingHospitalID:  creatingHospitalID,
		OwnerInvokerID:      ownerInvokerID,
		CreationTimestamp:   creationTime,
	}

	recordJSON, err := json.Marshal(record)
	if err != nil {
		return fmt.Errorf("failed to marshal ownership record: %w", err)
	}

	if err := ctx.GetStub().PutState(compositeKey, recordJSON); err != nil {
		return fmt.Errorf("failed to put state on the ledger: %w", err)
	}

	if err := ctx.GetStub().SetEvent("OwnershipCreated", recordJSON); err != nil {
		fmt.Printf("Warning: failed to set event OwnershipCreated: %v\n", err)
	}

	fmt.Printf("Ownership record successfully created. Key: %s\n", compositeKey)
	return nil
}

func (ac *AegisOwnershipContract) GetOwnershipRecord(
	ctx contractapi.TransactionContextInterface,
	patientID string,
	studyUID string) (*OwnershipRecord, error) {

	compositeKey, err := ctx.GetStub().CreateCompositeKey(CompositeKeyID, []string{patientID, studyUID})
	if err != nil {
		return nil, fmt.Errorf("failed to create composite key: %w", err)
	}

	recordJSON, err := ctx.GetStub().GetState(compositeKey)
	if err != nil {
		return nil, fmt.Errorf("failed to read from world state: %w", err)
	}
	if recordJSON == nil {
		return nil, fmt.Errorf("ownership record not found for key: %s", compositeKey)
	}

	var record OwnershipRecord
	if err := json.Unmarshal(recordJSON, &record); err != nil {
		return nil, fmt.Errorf("failed to unmarshal ownership record: %w", err)
	}

	return &record, nil
}

func (ac *AegisOwnershipContract) QueryOwnershipRecordsByPatient(
	ctx contractapi.TransactionContextInterface,
	patientID string) ([]*OwnershipRecord, error) {

	resultsIterator, err := ctx.GetStub().GetStateByPartialCompositeKey(CompositeKeyID, []string{patientID})
	if err != nil {
		return nil, fmt.Errorf("failed to get records by partial composite key: %w", err)
	}
	defer resultsIterator.Close()

	var records []*OwnershipRecord
	for resultsIterator.HasNext() {
		queryResponse, err := resultsIterator.Next()
		if err != nil {
			return nil, fmt.Errorf("failed to iterate over results: %w", err)
		}

		var record OwnershipRecord
		if err := json.Unmarshal(queryResponse.Value, &record); err != nil {
			fmt.Printf("Warning: failed to unmarshal record, skipping: %v\n", err)
			continue
		}
		records = append(records, &record)
	}

	return records, nil
}