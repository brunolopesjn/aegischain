package main

import (
	"encoding/json"
	"fmt"
	"net/http"
)

type BaselineBackend struct {
	baseURL string
	client  *http.Client
}

func NewBaselineBackend(baseURL string) *BaselineBackend {
	return &BaselineBackend{baseURL: baseURL, client: http.DefaultClient}
}

func (b *BaselineBackend) CheckAccess(patientID, providerID, studyUID string) (bool, error) {
	url := fmt.Sprintf("%s/consent/%s/%s/%s/check", b.baseURL, patientID, studyUID, providerID)
	resp, err := b.client.Get(url)
	if err != nil {
		return false, fmt.Errorf("failed to reach baseline API: %w", err)
	}
	defer resp.Body.Close()

	var result struct {
		AccessPermitted bool `json:"accessPermitted"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return false, fmt.Errorf("invalid response from baseline API: %w", err)
	}
	return result.AccessPermitted, nil
}