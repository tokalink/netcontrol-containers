package main

import (
	"context"
	"flag"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"netcontrol-containers/config"
	"netcontrol-containers/database"
	"netcontrol-containers/handlers"
	"netcontrol-containers/middleware"

	"github.com/gin-gonic/gin"
	"github.com/kardianos/service"
)

var logger service.Logger

type program struct {
	srv *http.Server
}

func (p *program) Start(s service.Service) error {
	// Start should not block. Do the actual work async.
	go p.run()
	return nil
}

func (p *program) run() {
	// Ensure we are in the executable directory so relative paths work
	exePath, err := os.Executable()
	if err == nil {
		exeDir := filepath.Dir(exePath)
		os.Chdir(exeDir)
	}

	// Initialize config
	config.Init()
	cfg := config.Get()

	// Initialize database
	if err := database.Init(); err != nil {
		if logger != nil {
			logger.Error(fmt.Sprintf("Failed to initialize database: %v", err))
		}
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
	// Check if running in dev mode (relative path) or installed mode (absolute path might be needed, but for now assume CWD)
	// Ideally we find where the executable is.
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
		api.GET("/docker/system/usage", handlers.GetSystemUsage)
		api.GET("/docker/system/ws", handlers.StreamDockerStats)
		api.GET("/docker/containers", handlers.ListContainers)
		api.GET("/docker/containers/:id/stats", handlers.GetContainerStats)
		api.GET("/docker/containers/:id/logs", handlers.GetContainerLogs)
		api.GET("/docker/containers/:id/inspect", handlers.InspectContainer)
		api.POST("/docker/containers", handlers.CreateContainer)
		api.POST("/docker/containers/:id/start", handlers.StartContainer)
		api.POST("/docker/containers/:id/stop", handlers.StopContainer)
		api.POST("/docker/containers/:id/restart", handlers.RestartContainer)
		api.DELETE("/docker/containers/:id", handlers.RemoveContainer)
		api.GET("/docker/images", handlers.ListImages)
		api.POST("/docker/images/pull", handlers.PullImage)
		api.DELETE("/docker/images/:id", handlers.RemoveImage)

		// Kubernetes
		api.GET("/kubernetes/status", handlers.KubernetesStatus)
		api.GET("/kubernetes/overview", handlers.GetClusterOverview)
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
		api.POST("/installer/unlock", handlers.ForceUnlock)
		api.POST("/installer/docker", handlers.InstallDocker)
		api.POST("/installer/kubernetes", handlers.InstallKubernetes)

		api.POST("/installer/restart/:service", handlers.RestartSoftware)

		// Files
		api.GET("/files", handlers.ListFiles)
		api.GET("/files/drives", handlers.GetDrives)
		api.GET("/files/content", handlers.GetFileContent)
		api.POST("/files/content", handlers.SaveFile)
		api.POST("/files/create", handlers.CreateFile)
		api.DELETE("/files", handlers.DeleteFile)
		api.POST("/files/rename", handlers.RenameFile)
		api.POST("/files/chmod", handlers.ChmodFile)
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

	// WebSocket routes
	r.GET("/ws/terminal", handlers.TerminalWS)
	r.GET("/ws/installer/docker", handlers.InstallDockerWS)
	r.GET("/ws/installer/kubernetes", handlers.InstallKubernetesWS)
	r.GET("/ws/installer/setup-k8s", handlers.SetupKubernetesWS)
	r.GET("/ws/installer/docker/uninstall", handlers.UninstallDockerWS)
	r.GET("/ws/installer/kubernetes/uninstall", handlers.UninstallKubernetesWS)

	// Start server
	addr := fmt.Sprintf(":%d", cfg.Port)
	log.Printf("🚀 NetControl Containers starting on http://localhost%s", addr)

	p.srv = &http.Server{
		Addr:    addr,
		Handler: r,
	}

	if err := p.srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		if logger != nil {
			logger.Error(fmt.Sprintf("Failed to start server: %v", err))
		}
		log.Fatalf("Failed to start server: %v", err)
	}
}

func (p *program) Stop(s service.Service) error {
	// Stop should not block. Return with a few seconds.
	if p.srv != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := p.srv.Shutdown(ctx); err != nil {
			log.Printf("Server forced to shutdown: %v", err)
		}
	}
	return nil
}

func main() {
	svcConfig := &service.Config{
		Name:        "NetControlContainers",
		DisplayName: "NetControl Containers Service",
		Description: "Container Management VPS Panel",
		Arguments:   []string{},
	}

	prg := &program{}
	s, err := service.New(prg, svcConfig)
	if err != nil {
		log.Fatal(err)
	}

	logger, err = s.Logger(nil)
	if err != nil {
		log.Fatal(err)
	}

	// Define flags
	install := flag.Bool("install", false, "Install service")
	uninstall := flag.Bool("uninstall", false, "Uninstall service")
	start := flag.Bool("start", false, "Start service")
	stop := flag.Bool("stop", false, "Stop service")
	restart := flag.Bool("restart", false, "Restart service")

	flag.Parse()

	// Handle simple legacy commands or explicit service commands
	if len(os.Args) > 1 {
		verb := os.Args[1]
		switch verb {
		case "install":
			err = s.Install()
			if err != nil {
				fmt.Printf("Failed to install service: %s\n", err)
				os.Exit(1)
			}
			fmt.Println("Service installed successfully.")
			return
		case "uninstall":
			err = s.Uninstall()
			if err != nil {
				fmt.Printf("Failed to uninstall service: %s\n", err)
				os.Exit(1)
			}
			fmt.Println("Service uninstalled successfully.")
			return
		case "start":
			err = s.Start()
			if err != nil {
				fmt.Printf("Failed to start service: %s\n", err)
				os.Exit(1)
			}
			fmt.Println("Service started.")
			return
		case "stop":
			err = s.Stop()
			if err != nil {
				fmt.Printf("Failed to stop service: %s\n", err)
				os.Exit(1)
			}
			fmt.Println("Service stopped.")
			return
		case "restart":
			err = s.Restart()
			if err != nil {
				fmt.Printf("Failed to restart service: %s\n", err)
				os.Exit(1)
			}
			fmt.Println("Service restarted.")
			return
		}
	}

	// Handle flags if args didn't match
	if *install {
		err = s.Install()
		if err != nil {
			log.Fatal(err)
		}
		fmt.Println("Service installed.")
		return
	}

	if *uninstall {
		err = s.Uninstall()
		if err != nil {
			log.Fatal(err)
		}
		fmt.Println("Service uninstalled.")
		return
	}

	if *start {
		err = s.Start()
		if err != nil {
			log.Fatal(err)
		}
		fmt.Println("Service started.")
		return
	}

	if *stop {
		err = s.Stop()
		if err != nil {
			log.Fatal(err)
		}
		fmt.Println("Service stopped.")
		return
	}

	if *restart {
		err = s.Restart()
		if err != nil {
			log.Fatal(err)
		}
		fmt.Println("Service restarted.")
		return
	}

	// Default: Run the service (foreground or background)
	err = s.Run()
	if err != nil {
		logger.Error(err.Error())
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
