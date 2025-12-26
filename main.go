package main

import (
	"fmt"
	"html/template"
	"log"
	"net/http"

	"netcontrol-containers/config"
	"netcontrol-containers/database"
	"netcontrol-containers/handlers"
	"netcontrol-containers/middleware"

	"github.com/gin-gonic/gin"
)

func main() {
	// Initialize config
	config.Init()
	cfg := config.Get()

	// Initialize database
	if err := database.Init(); err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}

	// Setup Gin
	if !cfg.DebugMode {
		gin.SetMode(gin.ReleaseMode)
	}

	r := gin.Default()

	// Load templates
	r.SetFuncMap(template.FuncMap{
		"formatBytes": formatBytes,
	})
	r.LoadHTMLGlob("templates/*")

	// Serve static files
	r.Static("/static", "./static")

	// Public routes
	r.GET("/login", func(c *gin.Context) {
		// Check if already logged in AND valid
		if tokenString, err := c.Cookie("token"); err == nil {
			// Validate token
			token, err := middleware.ValidateToken(tokenString)
			if err == nil && token.Valid {
				c.Redirect(http.StatusFound, "/")
				return
			}
		}
		c.HTML(http.StatusOK, "login.html", nil)
	})
	r.POST("/api/login", handlers.Login)
	r.POST("/api/logout", handlers.Logout)

	// Protected page routes
	pages := r.Group("/")
	pages.Use(middleware.AuthPageMiddleware())
	{
		pages.GET("/", func(c *gin.Context) {
			username, _ := c.Get("username")
			c.HTML(http.StatusOK, "dashboard.html", gin.H{"Username": username})
		})
		pages.GET("/docker", func(c *gin.Context) {
			username, _ := c.Get("username")
			c.HTML(http.StatusOK, "docker.html", gin.H{"Username": username})
		})
		pages.GET("/kubernetes", func(c *gin.Context) {
			username, _ := c.Get("username")
			c.HTML(http.StatusOK, "kubernetes.html", gin.H{"Username": username})
		})
		pages.GET("/installer", func(c *gin.Context) {
			username, _ := c.Get("username")
			c.HTML(http.StatusOK, "installer.html", gin.H{"Username": username})
		})
		pages.GET("/files", func(c *gin.Context) {
			username, _ := c.Get("username")
			c.HTML(http.StatusOK, "files.html", gin.H{"Username": username})
		})
		pages.GET("/terminal", func(c *gin.Context) {
			username, _ := c.Get("username")
			c.HTML(http.StatusOK, "terminal.html", gin.H{"Username": username})
		})
		pages.GET("/settings", func(c *gin.Context) {
			username, _ := c.Get("username")
			c.HTML(http.StatusOK, "settings.html", gin.H{"Username": username})
		})
		pages.GET("/wireguard", func(c *gin.Context) {
			username, _ := c.Get("username")
			c.HTML(http.StatusOK, "wireguard.html", gin.H{"Username": username})
		})
	}

	// Protected API routes
	api := r.Group("/api")
	api.Use(middleware.AuthMiddleware())
	{
		// User
		api.GET("/user", handlers.GetCurrentUser)
		api.POST("/user/password", handlers.ChangePassword)

		// Dashboard / System
		api.GET("/system/info", handlers.GetSystemInfo)
		api.GET("/system/stats", handlers.GetQuickStats)
		api.GET("/system/cpu", handlers.GetCPUInfo)
		api.GET("/system/memory", handlers.GetMemoryInfo)
		api.GET("/system/disk", handlers.GetDiskInfo)

		// Docker
		api.GET("/docker/status", handlers.DockerStatus)
		api.GET("/docker/containers", handlers.ListContainers)
		api.GET("/docker/containers/:id/stats", handlers.GetContainerStats)
		api.GET("/docker/containers/:id/logs", handlers.GetContainerLogs)
		api.GET("/docker/containers/:id/inspect", handlers.InspectContainer)
		api.POST("/docker/containers/:id/start", handlers.StartContainer)
		api.POST("/docker/containers/:id/stop", handlers.StopContainer)
		api.POST("/docker/containers/:id/restart", handlers.RestartContainer)
		api.DELETE("/docker/containers/:id", handlers.RemoveContainer)
		api.GET("/docker/images", handlers.ListImages)
		api.POST("/docker/images/pull", handlers.PullImage)
		api.DELETE("/docker/images/:id", handlers.RemoveImage)

		// Kubernetes
		api.GET("/kubernetes/status", handlers.KubernetesStatus)
		api.GET("/kubernetes/namespaces", handlers.ListNamespaces)
		api.GET("/kubernetes/pods", handlers.ListPods)
		api.GET("/kubernetes/pods/:name/logs", handlers.GetPodLogs)
		api.DELETE("/kubernetes/pods/:name", handlers.DeletePod)
		api.GET("/kubernetes/deployments", handlers.ListDeployments)
		api.POST("/kubernetes/deployments/:name/scale", handlers.ScaleDeployment)
		api.POST("/kubernetes/deployments/:name/restart", handlers.RestartDeployment)
		api.GET("/kubernetes/services", handlers.ListK8sServices)

		// Installer
		api.GET("/installer/status", handlers.GetSoftwareStatus)
		api.GET("/installer/progress", handlers.GetInstallStatus)
		api.POST("/installer/docker", handlers.InstallDocker)
		api.POST("/installer/kubernetes", handlers.InstallKubernetes)
		api.DELETE("/installer/docker", handlers.UninstallDocker)
		api.DELETE("/installer/kubernetes", handlers.UninstallKubernetes)
		api.POST("/installer/restart/:service", handlers.RestartSoftware)

		// Files
		api.GET("/files", handlers.ListFiles)
		api.GET("/files/drives", handlers.GetDrives)
		api.GET("/files/content", handlers.GetFileContent)
		api.POST("/files/content", handlers.SaveFile)
		api.POST("/files/create", handlers.CreateFile)
		api.DELETE("/files", handlers.DeleteFile)
		api.POST("/files/rename", handlers.RenameFile)
		api.POST("/files/copy", handlers.CopyFile)
		api.POST("/files/upload", handlers.UploadFile)
		api.GET("/files/download", handlers.DownloadFile)

		// Terminal
		api.GET("/terminal/sessions", handlers.ListTerminalSessions)
		api.POST("/terminal/:session/resize", handlers.TerminalResize)
		api.DELETE("/terminal/:session", handlers.CloseTerminalSession)

		// WireGuard
		api.GET("/wireguard/status", handlers.GetWireGuardStatus)
		api.GET("/wireguard/config", handlers.GetWireGuardConfig)
		api.POST("/wireguard/config", handlers.SaveWireGuardConfig)
		api.POST("/wireguard/connect", handlers.ConnectWireGuard)
		api.POST("/wireguard/disconnect", handlers.DisconnectWireGuard)
	}

	// WebSocket routes (protected via query param token)
	r.GET("/ws/terminal", handlers.TerminalWS)
	r.GET("/ws/installer/docker", handlers.InstallDockerWS)
	r.GET("/ws/installer/kubernetes", handlers.InstallKubernetesWS)
	r.GET("/ws/installer/setup-k8s", handlers.SetupKubernetesWS)

	// Start server
	addr := fmt.Sprintf(":%d", cfg.Port)
	log.Printf("🚀 NetControl Containers starting on http://localhost%s", addr)
	log.Printf("📦 Default login: admin / admin123")

	if err := r.Run(addr); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}

func formatBytes(bytes uint64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := uint64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}
