package models

import "time"

type ConsentRecord struct {
	PatientID       string     `gorm:"primaryKey;column:patient_id"`
	StudyUID        string     `gorm:"primaryKey;column:study_uid"`
	ProviderID      string     `gorm:"primaryKey;column:provider_id"`
	Status          string     `gorm:"column:status;not null;default:WAITING_APPROVAL"`
	ExpiryTimestamp *time.Time `gorm:"column:expiry_timestamp"`
	UpdatedAt       time.Time  `gorm:"column:updated_at;autoUpdateTime"`
}

func (ConsentRecord) TableName() string {
	return "consent_records"
}