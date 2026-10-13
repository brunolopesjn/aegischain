package models

import "time"

type OwnershipRecord struct {
	PatientID      string    `gorm:"primaryKey;column:patient_id"`
	StudyUID       string    `gorm:"primaryKey;column:study_uid"`
	ProxyURL       string    `gorm:"column:proxy_url;not null"`
	HospitalID     string    `gorm:"column:hospital_id;not null"`
	OwnerInvokerID string    `gorm:"column:owner_invoker_id;not null"`
	CreatedAt      time.Time `gorm:"column:created_at;autoCreateTime"`
}

func (OwnershipRecord) TableName() string {
	return "ownership_records"
}
