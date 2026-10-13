package models

type InstitutionRecord struct {
	MSPID string `gorm:"primaryKey;column:msp_id"`
	Name  string `gorm:"primaryKey;column:name;not null"`
}

func(InstitutionRecord) TableName() string {
	return "institution_record"
}
