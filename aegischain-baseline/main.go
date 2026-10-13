package main

import (
	"aegischain-baseline/db"
	"aegischain-baseline/handlers"
	"net/http"

	"github.com/gin-gonic/gin"
)

func main() {
	db.Connect()

	router := gin.Default()

	router.POST("/ownership", handlers.CreateOwnership)
	router.POST("/consent/request", handlers.RequestAccess)
	router.PUT("/consent/:patientId/:studyUid/:providerId", handlers.UpdateConsent)
	router.GET("/consent/:patientId/:studyUid/:providerId/check", handlers.CheckAccess)
	router.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	router.Run(":8080")
}
