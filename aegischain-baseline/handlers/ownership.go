package handlers

import (
	"aegischain-baseline/db"
	"aegischain-baseline/models"
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

var (
	errUnauthorizedInstitutionalActor = errors.New("unauthorized institutional actor")
	errOwnershipRecordAlreadyExists   = errors.New("record already exists")
	errPatientNotFound                = errors.New("patient not registered")
)

type createOwnershipRequest struct {
	PatientID string `json:"patientId" binding:"required"`
	StudyUID  string `json:"studyUid" binding:"required"`
	ProxyURL  string `json:"proxyUrl" binding:"required"`
}

func CreateOwnership(c *gin.Context) {
	mspID := c.GetHeader("X-MSP-ID")
	invokerID := c.GetHeader("X-Invoker-ID")
	if mspID == "" || invokerID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "Unauthorized institutional actor",
		})
		return
	}

	var req createOwnershipRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	err := db.DB.Transaction(func(tx *gorm.DB) error {
		var institutionExists int64
		tx.Model(&models.InstitutionRecord{}).
			Where("msp_id = ?", mspID).
			Count(&institutionExists)
		if institutionExists == 0 {
			return errUnauthorizedInstitutionalActor
		}

		var patientExists int64
		tx.Model(&models.PatientRecord{}).
			Where("invoker_id = ?", invokerID).
			Count(&patientExists)
		if patientExists == 0 {
			return errPatientNotFound
		}

		var exists int64
		tx.Model(&models.OwnershipRecord{}).
			Where("patient_id = ? AND study_uid = ?", req.PatientID, req.StudyUID).
			Count(&exists)
		if exists > 0 {
			return errOwnershipRecordAlreadyExists
		}

		record := models.OwnershipRecord{
			PatientID:      req.PatientID,
			StudyUID:       req.StudyUID,
			ProxyURL:       req.ProxyURL,
			HospitalID:     mspID,
			OwnerInvokerID: invokerID,
		}

		return tx.Create(&record).Error
	})

	switch {
	case err == nil:
		c.JSON(http.StatusCreated, gin.H{"status": "success"})
	case errors.Is(err, errUnauthorizedInstitutionalActor):
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized institutional actor"})
	case errors.Is(err, errPatientNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": "Patient not registered"})
	case errors.Is(err, errOwnershipRecordAlreadyExists):
		c.JSON(http.StatusConflict, gin.H{"error": "Record already exists"})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create record"})
	}

}
