package handlers

import (
	"net/http"

	"netcontrol-containers/services"

	"github.com/gin-gonic/gin"
)

func GetWireGuardStatus(c *gin.Context) {
	wg := services.GetWireGuardService()
	status, err := wg.GetStatus()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, status)
}

func GetWireGuardConfig(c *gin.Context) {
	wg := services.GetWireGuardService()
	config, err := wg.GetConfig()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"config": config})
}

func SaveWireGuardConfig(c *gin.Context) {
	var req struct {
		Config string `json:"config" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	wg := services.GetWireGuardService()
	if err := wg.SaveConfig(req.Config); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Configuration saved"})
}

func ConnectWireGuard(c *gin.Context) {
	wg := services.GetWireGuardService()
	if err := wg.Connect(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Connected"})
}

func DisconnectWireGuard(c *gin.Context) {
	wg := services.GetWireGuardService()
	if err := wg.Disconnect(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Disconnected"})
}
