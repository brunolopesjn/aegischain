package handlers

import (
	"aegischain-baseline/db"
	"aegischain-baseline/models"
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var (
	errUnauthorizedProvider = errors.New("unauthorized provider")
	errStudyNotRegistered   = errors.New("study not registered")
	errRequestAlreadyExists = errors.New("request already exists")

	errConsentNotFound = errors.New("no pending request found")
	errInvokerNotOwner = errors.New("invoker is not the study owner")
)

type requestAccessRequest struct {
	PatientID string `json:"patientId" binding:"required"`
	StudyUID  string `json:"studyUid" binding:"required"`
}

type updateConsentRequest struct {
	Approve         bool       `json:"approve"`
	ExpiryTimestamp *time.Time `json:"expiryTimestamp"`
}

func RequestAccess(c *gin.Context) {
	mspID := c.GetHeader("X-MSP-ID")
	providerID := c.GetHeader("X-Invoker-ID")
	if mspID == "" || providerID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized provider"})
		return
	}

	var req requestAccessRequest
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
			return errUnauthorizedProvider
		}

		var ownerExists int64
		tx.Model(&models.OwnershipRecord{}).
			Where("patient_id = ? AND study_uid = ?", req.PatientID, req.StudyUID).
			Count(&ownerExists)
		if ownerExists == 0 {
			return errStudyNotRegistered
		}

		var consentExists int64
		tx.Model(&models.ConsentRecord{}).
			Where("patient_id = ? AND study_uid = ? AND provider_id = ?", req.PatientID, req.StudyUID, providerID).
			Count(&consentExists)
		if consentExists > 0 {
			return errRequestAlreadyExists
		}

		consent := models.ConsentRecord{
			PatientID:  req.PatientID,
			StudyUID:   req.StudyUID,
			ProviderID: providerID,
			Status:     "WAITING_APPROVAL",
		}

		return tx.Create(&consent).Error
	})

	switch {
	case err == nil:
		c.JSON(http.StatusCreated, gin.H{"status": "success"})
	case errors.Is(err, errUnauthorizedProvider):
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized provider"})
	case errors.Is(err, errStudyNotRegistered):
		c.JSON(http.StatusNotFound, gin.H{"error": "Study not registered"})
	case errors.Is(err, errRequestAlreadyExists):
		c.JSON(http.StatusConflict, gin.H{"error": "Request already exists"})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create consent"})
	}

}

func UpdateConsent(c *gin.Context) {
	patientID := c.Param("patientId")
	studyUID := c.Param("studyUid")
	providerID := c.Param("providerId")

	invokerID := c.GetHeader("X-Invoker-ID")
	if invokerID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "missing identity"})
		return
	}

	var req updateConsentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	err := db.DB.Transaction(func(tx *gorm.DB) error {
		var consent models.ConsentRecord
		if err := tx.Where("patient_id = ? AND study_uid = ? AND provider_id = ?", patientID, studyUID, providerID).
			First(&consent).Error; err != nil {
			return errConsentNotFound
		}

		var ownership models.OwnershipRecord
		if err := tx.Where("patient_id = ? AND study_uid = ?", patientID, studyUID).
			First(&ownership).Error; err != nil {
			return errConsentNotFound
		}

		if invokerID != ownership.OwnerInvokerID {
			return errInvokerNotOwner
		}

		if req.Approve {
			consent.Status = "GRANTED"
			consent.ExpiryTimestamp = req.ExpiryTimestamp
		} else {
			consent.Status = "REVOKED"
			consent.ExpiryTimestamp = nil
		}

		return tx.Save(&consent).Error
	})

	switch {
	case err == nil:
		c.JSON(http.StatusOK, gin.H{"status": "success"})
	case errors.Is(err, errConsentNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": "No pending request found"})
	case errors.Is(err, errInvokerNotOwner):
		c.JSON(http.StatusForbidden, gin.H{"error": "Invoker is not the study owner"})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update consent"})
	}
}

func CheckAccess(c *gin.Context) {
	patientID := c.Param("patientId")
	studyUID := c.Param("studyUid")
	providerID := c.Param("providerId")

	var permitted bool

	err := db.DB.Transaction(func (tx *gorm.DB) error  {
		var consent models.ConsentRecord
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("patient_id = ? AND study_uid = ? AND provider_id = ?", patientID, studyUID, providerID).
			First(&consent).Error

		if errors.Is(err, gorm.ErrRecordNotFound) {
			permitted = false
			return nil
		} else if err != nil {
			return err
		}

		if consent.Status != "GRANTED" {
			permitted = false
			return logAccess(tx, patientID, providerID, studyUID, "DENIED", "consent not granted")
		}

		if consent.ExpiryTimestamp != nil && time.Now().After(*consent.ExpiryTimestamp) {
			consent.Status = "EXPIRED"
			if err := tx.Save(&consent).Error; err != nil {
				return err
			}
			permitted = false
			return logAccess(tx, patientID, providerID, studyUID, "EXPIRED", "consent expired at check time")
		}

		permitted = true
		return logAccess(tx, patientID, providerID, studyUID, "PERMITTED", "access granted")
	})

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to check access"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"accessPermitted": permitted})
}

func logAccess(tx *gorm.DB, patientID, providerID, studyUID, status, message string) error {
	entry := models.LogEntryRecord {
		PatientID: patientID,
		ProviderID: providerID,
		StudyUID: studyUID,
		Status: status,
		Message: message,
	}
	return tx.Create(&entry).Error
}