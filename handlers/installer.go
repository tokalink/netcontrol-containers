package handlers

import (
	"net/http"
	"sync"

	"netcontrol-containers/services"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

var installerUpgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

func GetSoftwareStatus(c *gin.Context) {
	installer := services.GetInstallerService()
	status := installer.CheckSoftwareStatus()
	c.JSON(http.StatusOK, status)
}

func GetInstallStatus(c *gin.Context) {
	installer := services.GetInstallerService()
	status := installer.GetStatus()
	c.JSON(http.StatusOK, status)
}

func InstallDocker(c *gin.Context) {
	installer := services.GetInstallerService()

	// Check if already installing
	status := installer.GetStatus()
	if status.IsInstalling {
		c.JSON(http.StatusConflict, gin.H{"error": "Another installation is in progress"})
		return
	}

	// Start installation in background
	go func() {
		installer.InstallDocker(nil)
	}()

	c.JSON(http.StatusOK, gin.H{"message": "Docker installation started"})
}

func InstallDockerWS(c *gin.Context) {
	conn, err := installerUpgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return
	}
	defer conn.Close()

	var mu sync.Mutex
	writeJSON := func(v interface{}) error {
		mu.Lock()
		defer mu.Unlock()
		return conn.WriteJSON(v)
	}

	installer := services.GetInstallerService()
	progressChan := make(chan string, 100)

	go func() {
		for msg := range progressChan {
			writeJSON(gin.H{"message": msg})
		}
	}()

	err = installer.InstallDocker(progressChan)
	close(progressChan)

	// Wait a tiny bit for the channel to drain (optional, but good practice if not using WaitGroup)
	// proper way is WaitGroup but here we just need to ensure thread safety on the socket

	if err != nil {
		writeJSON(gin.H{"error": err.Error(), "complete": true})
	} else {
		writeJSON(gin.H{"message": "Installation complete", "complete": true, "success": true})
	}
}

func InstallKubernetes(c *gin.Context) {
	installer := services.GetInstallerService()

	status := installer.GetStatus()
	if status.IsInstalling {
		c.JSON(http.StatusConflict, gin.H{"error": "Another installation is in progress"})
		return
	}

	go func() {
		installer.InstallKubernetes(nil)
	}()

	c.JSON(http.StatusOK, gin.H{"message": "Kubernetes installation started"})
}

func InstallKubernetesWS(c *gin.Context) {
	conn, err := installerUpgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return
	}
	defer conn.Close()

	var mu sync.Mutex
	writeJSON := func(v interface{}) error {
		mu.Lock()
		defer mu.Unlock()
		return conn.WriteJSON(v)
	}

	installer := services.GetInstallerService()
	progressChan := make(chan string, 100)

	go func() {
		for msg := range progressChan {
			writeJSON(gin.H{"message": msg})
		}
	}()

	err = installer.InstallKubernetes(progressChan)
	close(progressChan)

	if err != nil {
		writeJSON(gin.H{"error": err.Error(), "complete": true})
	} else {
		writeJSON(gin.H{"message": "Installation complete", "complete": true, "success": true})
	}
}

func UninstallDockerWS(c *gin.Context) {
	conn, err := installerUpgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return
	}
	defer conn.Close()

	var mu sync.Mutex
	writeJSON := func(v interface{}) error {
		mu.Lock()
		defer mu.Unlock()
		return conn.WriteJSON(v)
	}

	installer := services.GetInstallerService()
	progressChan := make(chan string, 100)

	go func() {
		for msg := range progressChan {
			writeJSON(gin.H{"message": msg})
		}
	}()

	err = installer.UninstallDocker(progressChan)
	close(progressChan)

	if err != nil {
		writeJSON(gin.H{"error": err.Error(), "complete": true})
	} else {
		writeJSON(gin.H{"message": "Uninstallation complete", "complete": true, "success": true})
	}
}

func UninstallKubernetesWS(c *gin.Context) {
	conn, err := installerUpgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return
	}
	defer conn.Close()

	var mu sync.Mutex
	writeJSON := func(v interface{}) error {
		mu.Lock()
		defer mu.Unlock()
		return conn.WriteJSON(v)
	}

	installer := services.GetInstallerService()
	progressChan := make(chan string, 100)

	go func() {
		for msg := range progressChan {
			writeJSON(gin.H{"message": msg})
		}
	}()

	err = installer.UninstallKubernetes(progressChan)
	close(progressChan)

	if err != nil {
		writeJSON(gin.H{"error": err.Error(), "complete": true})
	} else {
		writeJSON(gin.H{"message": "Uninstallation complete", "complete": true, "success": true})
	}
}

func RestartSoftware(c *gin.Context) {
	serviceName := c.Param("service")
	// Validate service name to prevent command injection
	if serviceName != "docker" && serviceName != "kubelet" && serviceName != "containerd" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid service name"})
		return
	}

	installer := services.GetInstallerService()
	if err := installer.RestartService(serviceName); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": serviceName + " restarted successfully"})
}

func ForceUnlock(c *gin.Context) {
	installer := services.GetInstallerService()
	installer.ResetLock()
	c.JSON(http.StatusOK, gin.H{"message": "Installation lock cleared"})
}

func SetupKubernetesWS(c *gin.Context) {
	conn, err := installerUpgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return
	}
	defer conn.Close()

	var mu sync.Mutex
	writeJSON := func(v interface{}) error {
		mu.Lock()
		defer mu.Unlock()
		return conn.WriteJSON(v)
	}

	installer := services.GetInstallerService()
	progressChan := make(chan string, 100)

	go func() {
		for msg := range progressChan {
			writeJSON(gin.H{"message": msg})
		}
	}()

	err = installer.SetupKubernetes(progressChan)
	close(progressChan)

	if err != nil {
		writeJSON(gin.H{"error": err.Error(), "complete": true})
	} else {
		writeJSON(gin.H{"message": "Setup complete", "complete": true, "success": true})
	}
}
