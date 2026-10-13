package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type LogEntryRecord struct {
	TxID       uuid.UUID `gorm:"primaryKey;column:tx_id"`
	PatientID  string    `gorm:"column:patient_id;not null"`
	ProviderID string    `gorm:"column:provider_id;not null"`
	StudyUID   string    `gorm:"column:study_uid;not null"`
	Status     string    `gorm:"column:status;not null"`
	Message    string    `gorm:"column:message;not null"`
	EventTime  time.Time `gorm:"column:event_time;autoCreateTime"`
}

func (LogEntryRecord) TableName() string {
	return "log_entry_records"
}

func (l *LogEntryRecord) BeforeCreate(tx *gorm.DB) error {
	if l.TxID == uuid.Nil {
		l.TxID = uuid.New()
	}
	return nil
}
