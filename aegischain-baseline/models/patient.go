package models

import "time"

type PatientRecord struct {
	InvokerID string    `gorm:"primaryKey;column:invoker_id"`
	Name      string    `gorm:"column:name;not null"`
	CreatedAt time.Time `gorm:"column:created_at;autoCreateTime"`
}

func (PatientRecord) TableName() string {
	return "patients"
}
