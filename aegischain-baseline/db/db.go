package db

import (
	"aegischain-baseline/models"
	"fmt"
	"log"
	"os"
	"strconv"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"gorm.io/gorm/logger"
)

var DB *gorm.DB

func Connect() {
	dsn := os.Getenv("DATABASE_DSN")
	if dsn == "" {
		dsn = "host=localhost user=aegis password=aegis dbname=aegischain port=5432 sslmode=disable"
	}

	var err error
	DB, err = gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Warn),
	})
	if err != nil {
		log.Fatalf("failed to connect to database: %v", err)
	}

	sqlDB, err := DB.DB()
	if err != nil {
		log.Fatalf("failed to get underlying sql.DB: %v", err)
	}
	sqlDB.SetMaxOpenConns(25)
	sqlDB.SetMaxIdleConns(25)
	sqlDB.SetConnMaxLifetime(5 * time.Minute)

	if err := DB.AutoMigrate(
		&models.OwnershipRecord{},
		&models.ConsentRecord{},
		&models.LogEntryRecord{},
		&models.InstitutionRecord{},
		&models.PatientRecord{},
	); err != nil {
		log.Fatalf("failed to run auto migration: %v", err)
	}

	// Populate database with institutions once
	SeedInstitutions()

	seedCount := 10
	if v := os.Getenv("SEED_PATIENT_COUNT"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			seedCount = n
		}
	}
	SeedPatients(seedCount)

	fmt.Println("database connected and migrated successfully")

}

func SeedInstitutions() {
	institutions := []models.InstitutionRecord{
		{MSPID: "HospitalAMSP", Name: "Hospital A"},
		{MSPID: "HospitalBMSP", Name: "Hospital B"},
		{MSPID: "HospitalCMSP", Name: "Hospital C"},
		{MSPID: "Clinic1MSP", Name: "Clinic 1"},
		{MSPID: "Clinic2MSP", Name: "Clinic 2"},
	}

	for _, inst := range institutions {
		DB.FirstOrCreate(&inst, models.InstitutionRecord{MSPID: inst.MSPID})
	}
}

func SeedPatients(count int) {
	patients := make([]models.PatientRecord, 0, count)
	for i := 0; i < count; i++ {
		patients = append(patients, models.PatientRecord{
			InvokerID: fmt.Sprintf("patient-owner-%d", i),
			Name:      fmt.Sprintf("Patient %d", i),
		})
	}
	DB.Clauses(clause.OnConflict{DoNothing: true}).CreateInBatches(patients, 100)
}
